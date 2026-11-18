package examples

import (
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

// scalaPins is the pinned build the Scala examples are compiled against.
// Dependabot has no sbt ecosystem, so bump the versions in
// containers/scala/build.sbt by hand.
//
//go:embed containers/scala/build.sbt containers/scala/project/build.properties
var scalaPins embed.FS

// scalaLanguage compiles the complete Scala blocks of one rule file as one sbt
// build, inside the scala-service fixture's image.
//
// Assembly rules:
//
//  1. A block whose first non-empty line is a "// src/main/scala/<path>.scala"
//     comment is written to that path in the root project; such blocks share
//     one project and may reference each other.
//  2. Any other complete block is written to
//     examples/block_<line>/src/main/scala/Example.scala, in a subproject of
//     its own, so standalone blocks may declare the same names.
//  3. Every project compiles against the pinned build.sbt and
//     project/build.properties under containers/scala/; examples import
//     nothing outside them.
//  4. "Not found" errors (E006) for names a block uses but does not define are
//     ignored, unless they occur on an import line: examples reference
//     application state such as Order, saveOrder, or logger. Every other
//     error fails the block.
type scalaLanguage struct{}

func (scalaLanguage) Name() string    { return "scala" }
func (scalaLanguage) Tags() []string  { return []string{"scala"} }
func (scalaLanguage) Fixture() string { return "scala-service" }

func (scalaLanguage) ManifestFiles() []string {
	return []string{"build.sbt", "project/build.properties"}
}

// Caches mounts the coursier cache, which holds the library downloads CI
// caches, and the sbt directories (launcher boot, global base with the disk
// cache, and Ivy home), which keep local reruns fast.
func (scalaLanguage) Caches() []ContainerCache {
	return []ContainerCache{
		{Name: "packages", Path: "/cache/coursier"},
		{Name: "sbt", Path: "/cache/sbt"},
	}
}

func (scalaLanguage) Env() []string {
	return []string{"COURSIER_CACHE=/cache/coursier", "NO_COLOR=1"}
}

// scalaSbt is the sbt invocation shared by both phases: no thin client and no
// server, every sbt directory on a mount, and a bounded heap.
var scalaSbt = []string{
	"sbt", "--server", "--no-server", "--batch", "--no-colors",
	"--sbt-dir", "/cache/sbt/global",
	"--sbt-boot", "/cache/sbt/boot",
	"--ivy", "/cache/sbt/ivy2",
	"-J-Xmx1g",
}

// FetchCommand resolves the pinned libraries, the Scala compiler, and sbt
// itself; the build declares no sbt plugins.
func (scalaLanguage) FetchCommand() []string {
	return append(append([]string{}, scalaSbt...), "update")
}

func (scalaLanguage) CheckLabel() string { return "sbt compile" }

// scalaFilter reads sbt compile output and exits non-zero when it holds an
// error that rule 4 does not ignore, or when sbt failed without any compiler
// error (a resolution or build-definition failure). A compiler error starts
// with a "[error] -- [E<code>] <kind>: <path>:<line>:<col>" header, followed
// by the "[error] <line> |<source>" line it points at.
const scalaFilter = `
/^\[error\] -- \[E[0-9]+\] / { if (pending) bad++; n++; pending = 1; id = $3; next }
pending && /^\[error\] [0-9]+ \|/ {
  src = $0; sub(/^\[error\] [0-9]+ \|[ \t]*/, "", src)
  if (id != "[E006]" || src ~ /^import[ \t]/) bad++
  pending = 0; next
}
END { if (pending) bad++; if (bad > 0) exit 1; if (code != 0 && n == 0) exit code; exit 0 }`

// CheckCommand compiles every project offline and applies scalaFilter.
func (scalaLanguage) CheckCommand() []string {
	script := `out=$(COURSIER_MODE=offline "$@" 'set ThisBuild / offline := true' compile 2>&1); code=$?
printf '%s\n' "$out"
printf '%s\n' "$out" | awk -v code="$code" '` + scalaFilter + `'`
	return append([]string{"sh", "-c", script, "sbt"}, scalaSbt...)
}

// scalaHeaderRe matches the header of a compiler error, for example
// "[error] -- [E007] Type Mismatch Error: /work/src/main/scala/A.scala:3:5 ---".
var scalaHeaderRe = regexp.MustCompile(`^\[error\] -- \[(E\d+)\] ([^:]+): /work/(\S+):(\d+):(\d+)`)

// scalaSourceRe matches the source line under a header, for example
// "[error] 3 |  val x: Int = \"s\"".
var scalaSourceRe = regexp.MustCompile(`^\[error\] \d+ \|\s*(.*)$`)

// scalaImportLineRe matches source text that is an import clause.
var scalaImportLineRe = regexp.MustCompile(`^import\s`)

// scalaMessageRe matches a message line of an error, for example
// "[error]   |  Found: String".
var scalaMessageRe = regexp.MustCompile(`^\[error\]\s+\|\s*(.*)$`)

func (scalaLanguage) Diagnostics(output string) []Diagnostic {
	var diagnostics []Diagnostic
	var current *Diagnostic
	var code, source string
	var message []string
	flush := func() {
		if current == nil {
			return
		}
		if code != "E006" || scalaImportLineRe.MatchString(source) {
			current.Message = strings.Join(message, " ")
			diagnostics = append(diagnostics, *current)
		}
		current, code, source, message = nil, "", "", nil
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, " \r")
		if m := scalaHeaderRe.FindStringSubmatch(line); m != nil {
			flush()
			lineNo, _ := strconv.Atoi(m[4])
			column, _ := strconv.Atoi(m[5])
			current = &Diagnostic{Path: m[3], Line: lineNo, Column: column}
			code = m[1]
			message = []string{fmt.Sprintf("[%s] %s:", m[1], m[2])}
			continue
		}
		if current == nil {
			continue
		}
		if m := scalaSourceRe.FindStringSubmatch(line); m != nil {
			if source == "" {
				source = m[1]
			}
			continue
		}
		if m := scalaMessageRe.FindStringSubmatch(line); m != nil {
			text := strings.TrimSpace(m[1])
			if text == "" || strings.Trim(text, "^") == "" || strings.HasPrefix(text, "longer explanation available") {
				continue
			}
			message = append(message, text)
			continue
		}
		flush()
	}
	flush()
	return diagnostics
}

// scalaPathCommentRe matches the "// src/main/scala/<path>.scala" first line
// naming a block's file.
var scalaPathCommentRe = regexp.MustCompile(`^//\s*(src/main/scala/[A-Za-z0-9_./-]+\.scala)\s*$`)

// scalaImportRe matches a top-level package clause or import at column 0.
var scalaImportRe = regexp.MustCompile(`(?m)^(package|import)\s+\w`)

// scalaTypeRe matches a top-level object, class, trait, or enum definition at
// column 0, with optional modifiers.
var scalaTypeRe = regexp.MustCompile(`(?m)^((final|sealed|abstract|case|private|open|implicit)\s+)*(object|class|trait|enum)\s+\w`)

// scalaMainRe matches a top-level @main method at column 0.
var scalaMainRe = regexp.MustCompile(`(?m)^@main\s+def\s+\w`)

// scalaBlockPath returns the "// src/main/scala/<path>.scala" path named on
// the block's first non-empty line, or "".
func scalaBlockPath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := scalaPathCommentRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			return m[1]
		}
		return ""
	}
	return ""
}

// isCompleteScala reports whether a Scala block is a compilation unit: it
// names its file with a "// src/main/scala/<path>.scala" first line, declares
// an @main method, or has a top-level package clause or import together with
// a top-level object, class, trait, or enum. sbt build definitions (a first
// line such as "// build.sbt") never match, because they declare no type.
func isCompleteScala(content string) bool {
	if scalaBlockPath(content) != "" || scalaMainRe.MatchString(content) {
		return true
	}
	return scalaImportRe.MatchString(content) && scalaTypeRe.MatchString(content)
}

func (scalaLanguage) Assemble(dir string, code []*Block, _ []*Block) (map[*Block]string, error) {
	paths := map[*Block]string{}
	owner := map[string]*Block{}
	var subprojects []string
	for _, block := range code {
		p := scalaBlockPath(block.Content)
		if p == "" {
			name := fmt.Sprintf("block_%d", block.Line)
			subprojects = append(subprojects, name)
			p = "examples/" + name + "/src/main/scala/Example.scala"
		} else if path.Clean(p) != p || strings.Contains(p, "..") {
			return nil, fmt.Errorf("block at line %d: invalid path %q", block.Line, p)
		}
		if other, taken := owner[p]; taken {
			return nil, fmt.Errorf("%s is claimed by the blocks at lines %d and %d; mark one <!-- eval:fragment -->", p, other.Line, block.Line)
		}
		owner[p] = block
		paths[block] = p
	}
	sort.Strings(subprojects)

	files := map[string]string{}
	for p, block := range owner {
		files[p] = block.Content + "\n"
	}
	files["examples.sbt"] = scalaExamplesBuild(subprojects)
	for _, name := range (scalaLanguage{}).ManifestFiles() {
		data, err := scalaPins.ReadFile("containers/scala/" + name)
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

// scalaExamplesBuild renders examples.sbt: the root project aggregating one
// subproject per standalone block, compiled one task at a time so compiler
// output does not interleave and the heap stays bounded.
func scalaExamplesBuild(subprojects []string) string {
	var b strings.Builder
	b.WriteString("// Generated by evals/custom/examples/scala.go.\n")
	b.WriteString("Global / concurrentRestrictions += Tags.limitAll(1)\n\n")
	b.WriteString("lazy val root = (project in file(\".\"))")
	if len(subprojects) > 0 {
		fmt.Fprintf(&b, "\n  .aggregate(%s)", strings.Join(subprojects, ", "))
	}
	b.WriteString("\n")
	for _, name := range subprojects {
		fmt.Fprintf(&b, "\nlazy val %s = project in file(\"examples/%s\")\n", name, name)
	}
	return b.String()
}
