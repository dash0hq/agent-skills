package examples

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// phpPins is the pinned package set the PHP examples are analysed against.
// Regenerate composer.lock after editing composer.json (see the PHP section
// of evals/custom/README.md).
//
//go:embed containers/php/composer.json containers/php/composer.lock
var phpPins embed.FS

// phpLanguage statically analyses the complete PHP blocks of one rule file as
// one project, with php -l and PHPStan inside the php-service fixture's
// image. No example code is executed.
//
// Assembly rules:
//
//  1. A block whose first non-empty line, or whose line after a bare
//     "<?php" line, is a "// <path>.php" comment is written to that path;
//     any other complete block to examples/block_<line>.php.
//  2. A block that does not open with "<?php" gets "<?php " prepended on its
//     first line, so line numbers stay those of the block.
//  3. The project uses the pinned composer.json and composer.lock under
//     containers/php/; examples use no package outside them. Composer runs
//     in the image the fixture Dockerfile copies its composer binary from,
//     with scripts and plugins disabled.
//  4. Every file must pass php -l; then PHPStan runs at level 2 over the
//     written files. Undefined variables (variable.undefined) and calls to
//     unqualified functions that do not exist (function.notFound on a name
//     without a namespace) are ignored: examples reference application
//     state such as $order or saveOrder(). Every other error fails the
//     block.
type phpLanguage struct{}

func (phpLanguage) Name() string    { return "php" }
func (phpLanguage) Tags() []string  { return []string{"php"} }
func (phpLanguage) Fixture() string { return "php-service" }

func (phpLanguage) ManifestFiles() []string { return []string{"composer.json", "composer.lock"} }

// Caches mounts COMPOSER_HOME, whose cache/ holds the package downloads CI
// caches, and vendor/, which keeps the per-file installs incremental.
func (phpLanguage) Caches() []ContainerCache {
	return []ContainerCache{
		{Name: "packages", Path: "/cache/composer"},
		{Name: "vendor", Path: "/work/vendor"},
	}
}

// Env points COMPOSER_HOME at the mounted cache and fixes the root package
// version, which Composer otherwise tries to read from a VCS checkout.
func (phpLanguage) Env() []string {
	return []string{"COMPOSER_HOME=/cache/composer", "COMPOSER_ROOT_VERSION=1.0.0", "COMPOSER_NO_INTERACTION=1"}
}

// phpComposerCopyRe matches the Dockerfile line copying the composer binary
// from another image, for example
// "COPY --from=composer:2 /usr/bin/composer /usr/bin/composer".
var phpComposerCopyRe = regexp.MustCompile(`(?i)^\s*COPY\s+--from=(\S+)\s+\S*/composer\s+\S+\s*$`)

// FetchImage implements ContainerFetchImager: the php fixture image has no
// composer, so the fetch phase runs in the image the fixture Dockerfile
// copies its composer binary from, and no second pin exists.
func (phpLanguage) FetchImage(dockerfile string) (string, error) {
	file, err := os.Open(dockerfile)
	if err != nil {
		return "", fmt.Errorf("fixture Dockerfile: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if m := phpComposerCopyRe.FindStringSubmatch(scanner.Text()); m != nil {
			return m[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("fixture Dockerfile %s: %w", dockerfile, err)
	}
	return "", fmt.Errorf("fixture Dockerfile %s has no \"COPY --from=<image> .../composer\" line", dockerfile)
}

// FetchCommand installs the pinned packages; Composer scripts and plugins do
// not run.
func (phpLanguage) FetchCommand() []string {
	return []string{"composer", "install", "--no-scripts", "--no-plugins", "--no-interaction", "--no-progress", "--no-ansi", "--prefer-dist"}
}

func (phpLanguage) CheckLabel() string { return "php -l and PHPStan" }

// CheckCommand lints every project file with php -l and, when all parse,
// runs PHPStan with the generated phpstan.neon.
func (phpLanguage) CheckCommand() []string {
	script := `fail=0
for f in $(find . -path ./vendor -prune -o -type f -name '*.php' -print | sed 's|^\./||' | sort); do
  out=$(php -d display_errors=stderr -d log_errors=0 -l "$f" 2>&1) || { printf '%s\n' "$out"; fail=1; }
done
[ "$fail" -eq 0 ] || exit 1
exec php vendor/bin/phpstan analyse --no-progress --no-ansi --error-format=raw --memory-limit=1G -c phpstan.neon`
	return []string{"sh", "-c", script}
}

// phpLintRe matches a php -l parse error, for example
// "PHP Parse error:  syntax error, unexpected token "}" in examples/block_3.php on line 4".
var phpLintRe = regexp.MustCompile(`^(?:PHP )?(?:Parse|Fatal) error:\s+(.*) in (\S+\.php) on line (\d+)$`)

// phpStanRawRe matches a PHPStan "--error-format=raw" line, for example
// "/work/examples/block_3.php:5:Call to an undefined method ...".
var phpStanRawRe = regexp.MustCompile(`^(\S+\.php):(\d+):(.*)$`)

// Diagnostics parses php -l and PHPStan errors. Neither reports a column, so
// every diagnostic sits at column 1.
func (phpLanguage) Diagnostics(output string) []Diagnostic {
	var diagnostics []Diagnostic
	seen := map[Diagnostic]bool{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		var d Diagnostic
		if m := phpLintRe.FindStringSubmatch(line); m != nil {
			lineNo, _ := strconv.Atoi(m[3])
			d = Diagnostic{Path: phpProjectPath(m[2]), Line: lineNo, Column: 1, Message: "php -l: " + m[1]}
		} else if m := phpStanRawRe.FindStringSubmatch(line); m != nil {
			lineNo, _ := strconv.Atoi(m[2])
			d = Diagnostic{Path: phpProjectPath(m[1]), Line: lineNo, Column: 1, Message: "PHPStan: " + strings.TrimSpace(m[3])}
		} else {
			continue
		}
		if !seen[d] {
			seen[d] = true
			diagnostics = append(diagnostics, d)
		}
	}
	return diagnostics
}

// phpProjectPath turns a path reported inside the container into a
// project-relative one.
func phpProjectPath(p string) string {
	return strings.TrimPrefix(strings.TrimPrefix(p, "/work/"), "./")
}

// phpPathCommentRe matches the "// <path>.php" first line naming a block's
// file.
var phpPathCommentRe = regexp.MustCompile(`^//\s*([A-Za-z0-9_./-]+\.php)\s*$`)

// phpOpenTagRe matches a first line opening with the "<?php" tag.
var phpOpenTagRe = regexp.MustCompile(`^<\?php(\s|$)`)

// phpUseRe matches a top-level namespace import at column 0, for example
// "use OpenTelemetry\API\Globals;" or "use function Foo\bar;".
var phpUseRe = regexp.MustCompile(`(?m)^use\s+(?:function\s+|const\s+)?\\?[A-Za-z_][A-Za-z0-9_]*\\`)

// phpLeadingLines returns the first n non-blank lines of content, trimmed.
func phpLeadingLines(content string, n int) []string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
			if len(lines) == n {
				break
			}
		}
	}
	return lines
}

// phpOpensWithTag reports whether the block's first non-empty line opens
// with "<?php".
func phpOpensWithTag(content string) bool {
	lines := phpLeadingLines(content, 1)
	return len(lines) == 1 && phpOpenTagRe.MatchString(lines[0])
}

// phpBlockPath returns the path named by a "// <path>.php" comment on the
// block's first non-empty line, or on the line after a bare "<?php" line,
// or "".
func phpBlockPath(content string) string {
	lines := phpLeadingLines(content, 2)
	if len(lines) == 2 && lines[0] == "<?php" {
		lines = lines[1:]
	}
	if len(lines) > 0 {
		if m := phpPathCommentRe.FindStringSubmatch(lines[0]); m != nil {
			return m[1]
		}
	}
	return ""
}

// isCompletePHP reports whether a PHP block is a self-contained file: it
// opens with "<?php", names its file with a "// <path>.php" comment, or
// imports a namespaced name with a top-level use statement.
func isCompletePHP(content string) bool {
	return phpOpensWithTag(content) || phpBlockPath(content) != "" || phpUseRe.MatchString(content)
}

func (phpLanguage) Assemble(dir string, code []*Block, _ []*Block) (map[*Block]string, error) {
	paths := map[*Block]string{}
	owner := map[string]*Block{}
	for _, block := range code {
		p := phpBlockPath(block.Content)
		if p == "" {
			p = fmt.Sprintf("examples/block_%d.php", block.Line)
		}
		if path.IsAbs(p) || path.Clean(p) != p || strings.HasPrefix(p, "..") || strings.HasPrefix(p, "vendor/") {
			return nil, fmt.Errorf("block at line %d: invalid path %q", block.Line, p)
		}
		if other, taken := owner[p]; taken {
			return nil, fmt.Errorf("%s is claimed by the blocks at lines %d and %d; mark one <!-- eval:fragment -->", p, other.Line, block.Line)
		}
		owner[p] = block
		paths[block] = p
	}

	files := map[string]string{}
	var analysed []string
	for p, block := range owner {
		content := block.Content
		if !phpOpensWithTag(content) {
			// Same line, so PHP line numbers stay the block's.
			content = "<?php " + content
		}
		files[p] = content + "\n"
		analysed = append(analysed, p)
	}
	sort.Strings(analysed)
	files["phpstan.neon"] = phpStanConfig(analysed)
	for _, name := range (phpLanguage{}).ManifestFiles() {
		data, err := phpPins.ReadFile("containers/php/" + name)
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

// phpStanLevel catches unknown classes, functions, constants, methods, and
// properties on every expression (level 2), without the type checks of
// higher levels, which would flag examples whose variables stand for
// application state.
const phpStanLevel = 2

// phpStanConfig renders the phpstan.neon that analyses exactly the given
// files.
func phpStanConfig(files []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "parameters:\n  level: %d\n  tmpDir: /tmp/phpstan\n  reportUnmatchedIgnoredErrors: false\n  paths:\n", phpStanLevel)
	for _, f := range files {
		fmt.Fprintf(&b, "    - %s\n", f)
	}
	b.WriteString(`  ignoreErrors:
    # Variables an example uses without assigning them stand for
    # application state, for example $order.
    - identifier: variable.undefined
    # Unqualified functions an example calls without defining them are
    # application helpers, for example saveOrder(); namespaced ones still fail.
    - identifier: function.notFound
      message: '#^Function [A-Za-z_][A-Za-z0-9_]* not found\.$#'
`)
	return b.String()
}
