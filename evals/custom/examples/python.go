package examples

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// pythonPins is the pinned package set the Python examples are type-checked
// against, mypy included (see containers/python/requirements.txt).
//
//go:embed containers/python/requirements.txt
var pythonPins embed.FS

// pythonLanguage type-checks the complete Python blocks of one rule file as
// one project, with mypy inside the python-service fixture's image.
//
// Assembly rules:
//
//  1. A block whose first non-empty line is a "# <path>.py" comment, with
//     every path segment a Python identifier, is written to that path; any
//     other complete block to examples/block_<line>.py. Module names follow
//     the path, so a block can import another as app.telemetry from
//     "# app/telemetry.py".
//  2. The project uses the pinned requirements.txt under containers/python/;
//     examples import nothing outside it and the standard library.
//  3. mypy runs at default (non-strict) settings plus check_untyped_defs, so
//     the bodies of unannotated functions are checked too, and follows
//     imports into installed packages that ship no py.typed marker.
//  4. Diagnostics for names a block uses but does not define (name-defined)
//     are disabled: examples reference application state such as app,
//     logger, or order. Every other diagnostic fails the block.
type pythonLanguage struct{}

func (pythonLanguage) Name() string    { return "python" }
func (pythonLanguage) Tags() []string  { return []string{"python"} }
func (pythonLanguage) Fixture() string { return "python-service" }

func (pythonLanguage) ManifestFiles() []string { return []string{"requirements.txt"} }

// Caches mounts the pip download cache, which CI caches, and the virtual
// environment, which FetchCommand rebuilds only when the pins or the
// interpreter change.
func (pythonLanguage) Caches() []ContainerCache {
	return []ContainerCache{
		{Name: "packages", Path: "/cache/pip"},
		{Name: "venv", Path: "/venv"},
	}
}

func (pythonLanguage) Env() []string {
	return []string{
		"PIP_CACHE_DIR=/cache/pip",
		"PIP_DISABLE_PIP_VERSION_CHECK=1",
		"PIP_NO_INPUT=1",
		"NO_COLOR=1",
	}
}

// FetchCommand installs the pinned packages into /venv from wheels only, so
// no third-party build script runs. A stamp of the interpreter version and
// the requirements.txt hash decides whether the cached venv is reused.
func (pythonLanguage) FetchCommand() []string {
	script := `set -e
stamp="$(python -VV) $(sha256sum requirements.txt)"
if [ -x /venv/bin/python ] && [ "$(cat /venv/.stamp 2>/dev/null)" = "$stamp" ]; then exit 0; fi
find /venv -mindepth 1 -delete
python -m venv /venv
/venv/bin/pip install --quiet --only-binary=:all: -r requirements.txt
printf '%s' "$stamp" > /venv/.stamp`
	return []string{"sh", "-c", script}
}

// pythonDisabledCodes are the mypy error codes switched off: name-defined
// ("Name \"x\" is not defined") for names an example uses without defining.
var pythonDisabledCodes = []string{"name-defined"}

func (pythonLanguage) CheckLabel() string { return "mypy" }

// CheckCommand runs mypy with the mypy.ini Assemble writes; mypy exits
// non-zero on any reported error.
func (pythonLanguage) CheckCommand() []string {
	return []string{"/venv/bin/mypy", "--config-file", "mypy.ini"}
}

// mypyDiagnosticRe matches a mypy error line with column numbers, for example
// `examples/block_12.py:4:5: error: Module has no attribute "FAILED"  [attr-defined]`.
// The column is absent for some file-level errors.
var mypyDiagnosticRe = regexp.MustCompile(`^(\S+?\.pyi?):(\d+)(?::(\d+))?: (error: .*)$`)

func (pythonLanguage) Diagnostics(output string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, line := range strings.Split(output, "\n") {
		m := mypyDiagnosticRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		lineNo, _ := strconv.Atoi(m[2])
		column := 1
		if m[3] != "" {
			column, _ = strconv.Atoi(m[3])
		}
		diagnostics = append(diagnostics, Diagnostic{Path: m[1], Line: lineNo, Column: column, Message: m[4]})
	}
	return diagnostics
}

// pythonPathCommentRe matches the "# <path>.py" first line naming a block's
// file; every segment must be a Python identifier so it maps to a module.
var pythonPathCommentRe = regexp.MustCompile(`^#\s*((?:[A-Za-z_][A-Za-z0-9_]*/)*[A-Za-z_][A-Za-z0-9_]*\.py)\s*$`)

// pythonBlockPath returns the path named on the block's first non-empty
// line, or "".
func pythonBlockPath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := pythonPathCommentRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			return m[1]
		}
		return ""
	}
	return ""
}

func (pythonLanguage) Assemble(dir string, code []*Block, _ []*Block) (map[*Block]string, error) {
	paths := map[*Block]string{}
	owner := map[string]*Block{}
	for _, block := range code {
		p := pythonBlockPath(block.Content)
		if p == "" {
			p = fmt.Sprintf("examples/block_%d.py", block.Line)
		}
		if other, taken := owner[p]; taken {
			return nil, fmt.Errorf("%s is claimed by the blocks at lines %d and %d; mark one <!-- eval:fragment -->", p, other.Line, block.Line)
		}
		owner[p] = block
		paths[block] = p
	}

	files := map[string]string{}
	var include []string
	for p, block := range owner {
		files[p] = block.Content + "\n"
		include = append(include, p)
	}
	sort.Strings(include)
	files["mypy.ini"] = pythonMypyConfig(include)
	for _, name := range (pythonLanguage{}).ManifestFiles() {
		data, err := pythonPins.ReadFile("containers/python/" + name)
		if err != nil {
			return nil, err
		}
		files[name] = string(data)
	}

	for p, content := range files {
		target := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// pythonMypyConfig renders the mypy.ini that type-checks exactly the given
// files, for the Python version of the image's interpreter.
// explicit_package_bases maps app/telemetry.py to the module
// app.telemetry without __init__.py files.
func pythonMypyConfig(files []string) string {
	return "[mypy]\n" +
		"files = " + strings.Join(files, ", ") + "\n" +
		"explicit_package_bases = True\n" +
		"check_untyped_defs = True\n" +
		"follow_untyped_imports = True\n" +
		"disable_error_code = " + strings.Join(pythonDisabledCodes, ", ") + "\n" +
		"show_column_numbers = True\n" +
		"show_error_codes = True\n" +
		"pretty = False\n" +
		"color_output = False\n" +
		"cache_dir = /tmp/mypy-cache\n"
}
