package examples

import (
	_ "embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// javaPom is the pinned Maven project the Java examples are compiled
// against (see containers/java/pom.xml).
//
//go:embed containers/java/pom.xml
var javaPom []byte

// javaLanguage compiles the complete Java blocks of one rule file as one
// Maven project, with mvn inside the java-service fixture's build image.
//
// Assembly rules:
//
//  1. A block whose first non-empty line is a "// src/main/java/<path>.java"
//     comment is written to that path. Any other complete block is written to
//     src/main/java/<package path>/<Type>.java, where <package path> comes
//     from its package declaration and <Type> is its public top-level type,
//     or its first top-level type when none is public.
//  2. The project uses the pinned pom.xml under containers/java/; examples
//     import nothing outside it and the JDK.
//  3. Dependencies and plugins are resolved with dependency:go-offline in the
//     network phase, so no plugin or annotation processor compiles code
//     there; the compile runs with mvn --offline.
//  4. Every javac error fails the block it occurs in, undefined names
//     included: a complete Java block must declare or import everything it
//     uses.
type javaLanguage struct{}

func (javaLanguage) Name() string    { return "java" }
func (javaLanguage) Tags() []string  { return []string{"java"} }
func (javaLanguage) Fixture() string { return "java-service" }

func (javaLanguage) ManifestFiles() []string { return []string{"pom.xml"} }

// javaRepo is the Maven local repository inside the container.
const javaRepo = "/cache/m2"

// Caches mounts the Maven local repository, which CI caches.
func (javaLanguage) Caches() []ContainerCache {
	return []ContainerCache{{Name: "packages", Path: javaRepo}}
}

// Env bounds the Maven JVM heap and points the image entrypoint's
// MAVEN_CONFIG at the writable HOME, since the container runs as the host
// user rather than root.
func (javaLanguage) Env() []string {
	return []string{"MAVEN_OPTS=-Xmx768m", "MAVEN_CONFIG=/tmp/.m2"}
}

func (javaLanguage) FetchCommand() []string {
	return []string{"mvn", "--batch-mode", "--no-transfer-progress", "-Dmaven.repo.local=" + javaRepo, "dependency:go-offline"}
}

func (javaLanguage) CheckLabel() string { return "mvn compile" }

func (javaLanguage) CheckCommand() []string {
	return []string{"mvn", "--batch-mode", "--no-transfer-progress", "--offline", "-Dmaven.repo.local=" + javaRepo, "compile"}
}

// javaDiagnosticRe matches a maven-compiler-plugin error line, for example
// "[ERROR] /work/src/main/java/OrderService.java:[12,9] cannot find symbol".
var javaDiagnosticRe = regexp.MustCompile(`^\[ERROR\] /work/(\S+\.java):\[(\d+),(\d+)\] (.*)$`)

// javaDetailRe matches the indented "symbol:" and "location:" lines javac
// prints below a diagnostic.
var javaDetailRe = regexp.MustCompile(`^(?:\[ERROR\])?\s+((?:symbol|location):\s.*)$`)

// Diagnostics extracts the javac errors, each with its symbol detail.
// maven-compiler-plugin prints every error twice, in the COMPILATION ERROR
// section and in the build failure summary; duplicates are dropped.
func (javaLanguage) Diagnostics(output string) []Diagnostic {
	var diagnostics []Diagnostic
	seen := map[Diagnostic]bool{}
	var current *Diagnostic
	flush := func() {
		if current != nil && !seen[*current] {
			seen[*current] = true
			diagnostics = append(diagnostics, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if m := javaDiagnosticRe.FindStringSubmatch(line); m != nil {
			flush()
			lineNo, _ := strconv.Atoi(m[2])
			column, _ := strconv.Atoi(m[3])
			current = &Diagnostic{Path: m[1], Line: lineNo, Column: column, Message: strings.TrimSpace(m[4])}
			continue
		}
		if m := javaDetailRe.FindStringSubmatch(line); m != nil && current != nil {
			current.Message += " (" + strings.Join(strings.Fields(m[1]), " ") + ")"
			continue
		}
		flush()
	}
	flush()
	return diagnostics
}

// javaPathCommentRe matches the "// src/main/java/<path>.java" first line
// naming a block's file.
var javaPathCommentRe = regexp.MustCompile(`^//\s*(src/main/java/[A-Za-z0-9_./-]+\.java)\s*$`)

// javaPackageRe matches a package declaration at column 0.
var javaPackageRe = regexp.MustCompile(`(?m)^package\s+([A-Za-z_][\w.]*)\s*;`)

// javaTypeRe matches a top-level class, interface, enum, record, or
// annotation type declaration at column 0, capturing its modifiers and name.
var javaTypeRe = regexp.MustCompile(`(?m)^((?:(?:public|protected|private|abstract|final|sealed|non-sealed|static|strictfp)\s+)*)(?:class|interface|enum|record|@interface)\s+([A-Za-z_]\w*)`)

// javaBlockPath returns the path named on the block's first non-empty line,
// or "".
func javaBlockPath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := javaPathCommentRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			return m[1]
		}
		return ""
	}
	return ""
}

// isCompleteJava reports whether a Java block is a compilation unit: it names
// its file with a "// src/main/java/<path>.java" first line, or declares a
// top-level type at column 0.
func isCompleteJava(content string) bool {
	return javaBlockPath(content) != "" || javaTypeRe.MatchString(content)
}

// javaDerivedPath returns src/main/java/<package path>/<Type>.java for a
// block without a path comment, or "" when it declares no top-level type.
func javaDerivedPath(content string) string {
	var name string
	for _, m := range javaTypeRe.FindAllStringSubmatch(content, -1) {
		if name == "" {
			name = m[2]
		}
		if strings.Contains(" "+m[1], " public ") {
			name = m[2]
			break
		}
	}
	if name == "" {
		return ""
	}
	dir := "src/main/java"
	if m := javaPackageRe.FindStringSubmatch(content); m != nil {
		dir += "/" + strings.ReplaceAll(m[1], ".", "/")
	}
	return dir + "/" + name + ".java"
}

func (javaLanguage) Assemble(dir string, code []*Block, _ []*Block) (map[*Block]string, error) {
	paths := map[*Block]string{}
	owner := map[string]*Block{}
	for _, block := range code {
		p := javaBlockPath(block.Content)
		if p == "" {
			p = javaDerivedPath(block.Content)
		}
		if p == "" {
			return nil, fmt.Errorf("block at line %d declares no top-level type and names no file", block.Line)
		}
		if path.Clean(p) != p || strings.Contains(p, "..") || !strings.HasPrefix(p, "src/main/java/") {
			return nil, fmt.Errorf("block at line %d: invalid path %q", block.Line, p)
		}
		if other, taken := owner[p]; taken {
			return nil, fmt.Errorf("%s is claimed by the blocks at lines %d and %d; name distinct files with a // src/main/java/<path>.java first line, or mark one <!-- eval:fragment -->", p, other.Line, block.Line)
		}
		owner[p] = block
		paths[block] = p
	}

	files := map[string][]byte{"pom.xml": javaPom}
	for p, block := range owner {
		files[p] = []byte(block.Content + "\n")
	}
	for p, content := range files {
		target := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return nil, err
		}
	}
	return paths, nil
}
