package examples

import (
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

// rustLanguage compiles the complete Rust blocks of one rule file as one
// crate, checked with cargo inside the rust-service fixture's build image.
//
// Assembly rules:
//
//  1. A block whose first non-empty line is a "// src/<path>.rs" comment is
//     written to that path; any other complete block (one containing
//     "fn main") is written to src/main.rs, and further "fn main" blocks to
//     src/bin/example_<line>.rs.
//  2. Every toml block of the file feeds Cargo.toml: lines before any table
//     header and the bodies of [dependencies], [dev-dependencies], and
//     [build-dependencies] are merged per table; every other table (for
//     example [[bin]]) is copied verbatim, and a [package] table replaces the
//     generated one. A dependency declared twice with different
//     specifications is an assembly error.
//  3. Without src/main.rs or src/lib.rs, a src/lib.rs is generated. Every
//     top-level src/<name>.rs module the crate root does not declare gets a
//     "mod <name>;" appended to the root, so no written file escapes the
//     check.
type rustLanguage struct{}

func (rustLanguage) Tag() string     { return "rust" }
func (rustLanguage) Fixture() string { return "rust-service" }

func (rustLanguage) ManifestFiles() []string { return []string{"Cargo.toml"} }

// Caches mounts CARGO_HOME, whose registry/ holds the crate downloads CI
// caches, and a target dir that keeps warm rebuilds fast locally.
func (rustLanguage) Caches() []ContainerCache {
	return []ContainerCache{
		{Name: "packages", Path: "/cache/cargo-home"},
		{Name: "target", Path: "/cache/target"},
	}
}

func (rustLanguage) Env() []string {
	return []string{
		"CARGO_HOME=/cache/cargo-home",
		"CARGO_TARGET_DIR=/cache/target",
		"CARGO_TERM_COLOR=never",
	}
}

func (rustLanguage) FetchCommand() []string { return []string{"cargo", "fetch"} }

func (rustLanguage) CheckCommand() []string {
	return []string{"cargo", "check", "--offline", "--locked", "--message-format", "short"}
}

// rustDiagnosticRe matches a "--message-format short" error line, for
// example "src/main.rs:3:5: error[E0308]: mismatched types".
var rustDiagnosticRe = regexp.MustCompile(`^(\S+\.rs):(\d+):(\d+): (error.*)$`)

func (rustLanguage) Diagnostics(output string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, line := range strings.Split(output, "\n") {
		m := rustDiagnosticRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		lineNo, _ := strconv.Atoi(m[2])
		column, _ := strconv.Atoi(m[3])
		diagnostics = append(diagnostics, Diagnostic{Path: m[1], Line: lineNo, Column: column, Message: m[4]})
	}
	return diagnostics
}

// rustPathCommentRe matches the "// src/<path>.rs" first line naming a
// block's file.
var rustPathCommentRe = regexp.MustCompile(`^//\s*(src/[A-Za-z0-9_./-]+\.rs)\s*$`)

// rustMainRe matches a fn main declaration, optionally async or pub.
var rustMainRe = regexp.MustCompile(`(?m)^\s*(pub\s+)?(async\s+)?fn\s+main\s*\(`)

// rustBlockPath returns the "// src/<path>.rs" path named on the block's
// first non-empty line, or "".
func rustBlockPath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := rustPathCommentRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			return m[1]
		}
		return ""
	}
	return ""
}

// isCompleteRust reports whether a Rust block is a compilation unit: it names
// its file with a "// src/<path>.rs" first line, or declares fn main.
func isCompleteRust(content string) bool {
	return rustBlockPath(content) != "" || rustMainRe.MatchString(content)
}

func (l rustLanguage) Assemble(dir string, code []*Block, file []*Block) (map[*Block]string, error) {
	paths := map[*Block]string{}
	owner := map[string]*Block{}
	for _, block := range code {
		p := rustBlockPath(block.Content)
		if p == "" {
			p = "src/main.rs"
			if _, taken := owner[p]; taken {
				p = fmt.Sprintf("src/bin/example_%d.rs", block.Line)
			}
		}
		if clean := path.Clean(p); clean != p || strings.Contains(p, "..") {
			return nil, fmt.Errorf("block at line %d: invalid path %q", block.Line, p)
		}
		if other, taken := owner[p]; taken {
			return nil, fmt.Errorf("%s is claimed by the blocks at lines %d and %d; mark one <!-- eval:fragment -->", p, other.Line, block.Line)
		}
		owner[p] = block
		paths[block] = p
	}

	manifest, err := assembleCargoManifest(file)
	if err != nil {
		return nil, err
	}

	files := map[string]string{}
	for p, block := range owner {
		files[p] = block.Content + "\n"
	}
	root := "src/main.rs"
	if _, ok := files[root]; !ok {
		root = "src/lib.rs"
		if _, ok := files[root]; !ok {
			files[root] = "// Generated crate root for the example blocks.\n"
		}
	}
	var modules []string
	for p := range files {
		dirName, base := path.Split(p)
		if dirName != "src/" || p == "src/main.rs" || p == "src/lib.rs" {
			continue
		}
		modules = append(modules, strings.TrimSuffix(base, ".rs"))
	}
	sort.Strings(modules)
	for _, module := range modules {
		declared := regexp.MustCompile(`(?m)^\s*(pub(\([^)]*\))?\s+)?mod\s+` + regexp.QuoteMeta(module) + `\s*;`)
		if !declared.MatchString(files[root]) {
			files[root] += fmt.Sprintf("\n#[allow(dead_code, unused_imports)]\nmod %s;\n", module)
		}
	}

	files["Cargo.toml"] = manifest
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

// rustGeneratedPackage is the [package] table used when no toml block
// supplies one.
const rustGeneratedPackage = `[package]
name = "skill-example"
version = "0.0.0"
edition = "2024"
publish = false
`

// cargoDependencyTables are the tables whose entries merge across blocks;
// header-less lines belong to the first.
var cargoDependencyTables = []string{"dependencies", "dev-dependencies", "build-dependencies"}

// tomlHeaderRe matches a table or array-of-tables header.
var tomlHeaderRe = regexp.MustCompile(`^\s*(\[\[?)\s*([^\]]+?)\s*\]\]?\s*(#.*)?$`)

// tomlKeyRe matches the start of a key = value entry.
var tomlKeyRe = regexp.MustCompile(`^\s*("[^"]+"|[A-Za-z0-9_-]+)\s*=`)

// cargoEntry is one dependency entry with its leading comments and any
// continuation lines (multi-line arrays).
type cargoEntry struct {
	name  string
	lines []string
}

// tableEntry is a parsed entry and the dependency table it belongs to.
type tableEntry struct {
	table string
	entry *cargoEntry
}

func (e *cargoEntry) spec() string {
	var parts []string
	for _, line := range e.lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, " ")
}

// assembleCargoManifest merges the toml blocks of a rule file into one
// Cargo.toml. Blocks annotated eval:skip, eval:bad, or carrying a BAD marker
// are ignored.
func assembleCargoManifest(file []*Block) (string, error) {
	pkg := ""
	entries := map[string][]*cargoEntry{}
	seen := map[string]map[string]*cargoEntry{}
	for _, table := range cargoDependencyTables {
		seen[table] = map[string]*cargoEntry{}
	}
	var extra []string

	for _, block := range file {
		if block.Tag != "toml" || block.Annotation == AnnotationSkip || block.Annotation == AnnotationBad || block.HasBadMarker() {
			continue
		}
		table := "dependencies"
		var verbatim *[]string
		var current *cargoEntry
		var pending []string
		var section []string
		var parsed []tableEntry
		flushSection := func() {
			if len(section) == 0 {
				return
			}
			text := strings.TrimRight(strings.Join(section, "\n"), "\n") + "\n"
			if strings.HasPrefix(strings.TrimSpace(section[0]), "[package]") {
				pkg = text
			} else {
				extra = append(extra, text)
			}
			section = nil
		}
		for _, line := range strings.Split(block.Content, "\n") {
			if m := tomlHeaderRe.FindStringSubmatch(line); m != nil {
				flushSection()
				current, pending = nil, nil
				if m[1] == "[" && isDependencyTable(m[2]) {
					table, verbatim = m[2], nil
					continue
				}
				table = ""
				section = []string{line}
				verbatim = &section
				continue
			}
			if verbatim != nil {
				*verbatim = append(*verbatim, line)
				continue
			}
			trimmed := strings.TrimSpace(line)
			switch {
			case trimmed == "":
				current = nil
			case strings.HasPrefix(trimmed, "#"):
				pending = append(pending, line)
			case tomlKeyRe.MatchString(line):
				name := strings.Trim(tomlKeyRe.FindStringSubmatch(line)[1], `"`)
				current = &cargoEntry{name: name, lines: append(pending, line)}
				pending = nil
				parsed = append(parsed, tableEntry{table, current})
			case current != nil:
				current.lines = append(current.lines, line)
			default:
				return "", fmt.Errorf("toml block at line %d: cannot read %q as a [%s] entry", block.Line, trimmed, table)
			}
		}
		flushSection()
		// Merge once the block is read, so multi-line specifications compare
		// whole.
		for _, p := range parsed {
			if previous, dup := seen[p.table][p.entry.name]; dup {
				if previous.spec() != p.entry.spec() {
					return "", fmt.Errorf("toml block at line %d: [%s] %s is declared twice with different specifications: %q and %q", block.Line, p.table, p.entry.name, previous.spec(), p.entry.spec())
				}
				continue
			}
			seen[p.table][p.entry.name] = p.entry
			entries[p.table] = append(entries[p.table], p.entry)
		}
	}

	var b strings.Builder
	if pkg == "" {
		pkg = rustGeneratedPackage
	}
	b.WriteString(pkg)
	for _, table := range cargoDependencyTables {
		if len(entries[table]) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n[%s]\n", table)
		for _, entry := range entries[table] {
			b.WriteString(strings.Join(entry.lines, "\n"))
			b.WriteString("\n")
		}
	}
	for _, section := range extra {
		b.WriteString("\n")
		b.WriteString(section)
	}
	return b.String(), nil
}

func isDependencyTable(name string) bool {
	return slices.Contains(cargoDependencyTables, name)
}
