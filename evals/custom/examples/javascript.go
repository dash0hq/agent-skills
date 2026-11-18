package examples

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// javascriptPins is the pinned package set the JavaScript and TypeScript
// examples are type-checked against. Regenerate package-lock.json after
// editing package.json (see containers/javascript/package.json).
//
//go:embed containers/javascript/package.json containers/javascript/package-lock.json containers/javascript/.npmrc
var javascriptPins embed.FS

// javascriptLanguage type-checks the complete JavaScript and TypeScript blocks
// of one rule file as one project, with tsc inside the nodejs-service
// fixture's image.
//
// Assembly rules:
//
//  1. A block whose first non-empty line is a "// <path>" comment naming a
//     .js, .mjs, .cjs, .jsx, .ts, .mts, or .tsx file is written to that path;
//     any other complete block to examples/block_<line>.<ext>, where <ext> is
//     js or ts by fence tag, with an x appended when the block contains JSX.
//  2. The project uses the pinned package.json and package-lock.json under
//     containers/javascript/; examples import nothing outside them.
//  3. tsc runs with --noEmit and checkJs at non-strict settings, and the path
//     alias "@/*" resolves to src/* (the Next.js default).
//  4. Diagnostics for names a block uses but does not define (TS2304 and
//     TS2552) are ignored: examples reference application state such as
//     app, logger, or orderId. Every other diagnostic fails the block.
type javascriptLanguage struct{}

func (javascriptLanguage) Name() string    { return "javascript" }
func (javascriptLanguage) Tags() []string  { return []string{"javascript", "typescript"} }
func (javascriptLanguage) Fixture() string { return "nodejs-service" }

func (javascriptLanguage) ManifestFiles() []string {
	return []string{"package.json", "package-lock.json", ".npmrc"}
}

// Caches mounts the npm download cache, which CI caches, and node_modules,
// which keeps the per-file installs incremental.
func (javascriptLanguage) Caches() []ContainerCache {
	return []ContainerCache{
		{Name: "packages", Path: "/cache/npm"},
		{Name: "node_modules", Path: "/work/node_modules"},
	}
}

func (javascriptLanguage) Env() []string {
	return []string{"npm_config_cache=/cache/npm", "NO_COLOR=1"}
}

// FetchCommand installs the pinned packages without running their install
// scripts. npm ci would delete the mounted node_modules, so npm install
// reconciles it against the lockfile instead.
func (javascriptLanguage) FetchCommand() []string {
	return []string{"npm", "install", "--ignore-scripts", "--no-audit", "--no-fund", "--loglevel=error"}
}

// javascriptIgnoredCodes are the tsc diagnostics for names an example uses
// without defining them ("Cannot find name").
var javascriptIgnoredCodes = []string{"2304", "2552"}

func (javascriptLanguage) CheckLabel() string { return "tsc" }

// CheckCommand runs tsc and fails only on diagnostics outside
// javascriptIgnoredCodes, or when tsc exits non-zero without any diagnostic
// (a crash or a configuration error).
func (javascriptLanguage) CheckCommand() []string {
	ignored := "error TS(" + strings.Join(javascriptIgnoredCodes, "|") + "):"
	script := `out=$(node_modules/.bin/tsc -p . --pretty false 2>&1); code=$?
printf '%s\n' "$out"
if printf '%s\n' "$out" | grep 'error TS' | grep -qvE '` + ignored + `'; then exit 1; fi
if [ "$code" -ne 0 ] && ! printf '%s\n' "$out" | grep -q 'error TS'; then exit "$code"; fi
exit 0`
	return []string{"sh", "-c", script}
}

// tscDiagnosticRe matches a "--pretty false" diagnostic, for example
// "examples/block_12.ts(3,5): error TS2339: Property 'x' does not exist".
var tscDiagnosticRe = regexp.MustCompile(`^(\S+)\((\d+),(\d+)\): (error TS(\d+): .*)$`)

func (javascriptLanguage) Diagnostics(output string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, line := range strings.Split(output, "\n") {
		m := tscDiagnosticRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || isIgnoredTSCode(m[5]) {
			continue
		}
		lineNo, _ := strconv.Atoi(m[2])
		column, _ := strconv.Atoi(m[3])
		diagnostics = append(diagnostics, Diagnostic{Path: m[1], Line: lineNo, Column: column, Message: m[4]})
	}
	return diagnostics
}

func isIgnoredTSCode(code string) bool {
	return slices.Contains(javascriptIgnoredCodes, code)
}

// javascriptPathCommentRe matches the "// <path>" first line naming a
// block's file.
var javascriptPathCommentRe = regexp.MustCompile(`^//\s*([A-Za-z0-9_@./-]+\.(?:js|mjs|cjs|jsx|ts|mts|tsx))\s*$`)

// javascriptJSXRe matches a closing or self-closing JSX tag.
var javascriptJSXRe = regexp.MustCompile(`</[A-Za-z][A-Za-z0-9.]*>|/>`)

// javascriptBlockPath returns the path named on the block's first non-empty
// line, or "".
func javascriptBlockPath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := javascriptPathCommentRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			return m[1]
		}
		return ""
	}
	return ""
}

func (javascriptLanguage) Assemble(dir string, code []*Block, _ []*Block) (map[*Block]string, error) {
	paths := map[*Block]string{}
	owner := map[string]*Block{}
	for _, block := range code {
		p := javascriptBlockPath(block.Content)
		if p == "" {
			ext := "js"
			if block.Tag == "typescript" {
				ext = "ts"
			}
			if javascriptJSXRe.MatchString(block.Content) {
				ext += "x"
			}
			p = fmt.Sprintf("examples/block_%d.%s", block.Line, ext)
		}
		if path.IsAbs(p) || path.Clean(p) != p || strings.HasPrefix(p, "..") || strings.HasPrefix(p, "node_modules/") {
			return nil, fmt.Errorf("block at line %d: invalid path %q", block.Line, p)
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
	tsconfig, err := javascriptTSConfig(include)
	if err != nil {
		return nil, err
	}
	files["tsconfig.json"] = tsconfig
	for _, name := range (javascriptLanguage{}).ManifestFiles() {
		data, err := javascriptPins.ReadFile("containers/javascript/" + name)
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

// javascriptTSConfig renders the tsconfig.json that type-checks exactly the
// given files.
func javascriptTSConfig(files []string) (string, error) {
	config := map[string]any{
		"compilerOptions": map[string]any{
			"target":           "es2023",
			"module":           "esnext",
			"moduleResolution": "bundler",
			"lib":              []string{"es2023", "dom", "dom.iterable"},
			"types":            []string{"node"},
			"jsx":              "preserve",
			"allowJs":          true,
			"checkJs":          true,
			"noEmit":           true,
			"strict":           false,
			"skipLibCheck":     true,
			"esModuleInterop":  true,
			"paths":            map[string][]string{"@/*": {"./src/*"}},
		},
		"files": files,
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}
