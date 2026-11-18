package examples

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyRust(t *testing.T) {
	cases := []struct {
		name    string
		tag     string
		content string
		want    Category
	}{
		{
			name:    "src path comment makes a complete block",
			tag:     "rust",
			content: "// src/telemetry.rs\nuse opentelemetry::global;\n",
			want:    CategoryCodeComplete,
		},
		{
			name:    "src path comment after blank lines",
			tag:     "rust",
			content: "\n// src/logs/json.rs\npub struct Json;\n",
			want:    CategoryCodeComplete,
		},
		{
			name:    "fn main makes a complete block",
			tag:     "rust",
			content: "mod telemetry;\n\nfn main() {}\n",
			want:    CategoryCodeComplete,
		},
		{
			name:    "async fn main under an attribute",
			tag:     "rust",
			content: "#[tokio::main]\nasync fn main() {}\n",
			want:    CategoryCodeComplete,
		},
		{
			name:    "use and function snippet is a fragment",
			tag:     "rust",
			content: "use tracing::instrument;\n\n#[instrument]\nfn checkout() {}\n",
			want:    CategoryCodeFragment,
		},
		{
			name:    "path comment that is not under src is a fragment",
			tag:     "rust",
			content: "// examples/demo.rs\nfn demo() {}\n",
			want:    CategoryCodeFragment,
		},
		{
			name:    "path comment on a later line is a fragment",
			tag:     "rust",
			content: "use std::fmt;\n// src/telemetry.rs\n",
			want:    CategoryCodeFragment,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyContent(t, tc.tag, tc.content, AnnotationNone); got != tc.want {
				t.Errorf("Classify = %q, want %q", got, tc.want)
			}
		})
	}
	if got := classifyContent(t, "rust", "fn main() {}\n", AnnotationFragment); got != CategoryCodeFragment {
		t.Errorf("eval:fragment on a complete Rust block: Classify = %q, want %q", got, CategoryCodeFragment)
	}
}

func TestNormalizeTagRs(t *testing.T) {
	if got := NormalizeTag("rs"); got != "rust" {
		t.Errorf(`NormalizeTag("rs") = %q, want "rust"`, got)
	}
}

// completeBlocks extracts path and returns its complete blocks of tag.
func completeBlocks(t *testing.T, path, tag string) (code, all []*Block) {
	t.Helper()
	all, err := ExtractFile(path)
	if err != nil {
		t.Fatalf("ExtractFile: %v", err)
	}
	for _, block := range all {
		if block.Tag == tag && Classify(block.Documents()[0]) == CategoryCodeComplete {
			code = append(code, block)
		}
	}
	return code, all
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestRustAssembleLayout(t *testing.T) {
	code, all := completeBlocks(t, "testdata/rust-ok/rule.md", "rust")
	if len(code) != 3 {
		t.Fatalf("complete Rust blocks = %d, want 3 (format.rs, endpoint.rs, main)", len(code))
	}
	dir := t.TempDir()
	paths, err := rustLanguage{}.Assemble(dir, code, all)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	want := []string{"src/format.rs", "src/endpoint.rs", "src/main.rs"}
	for i, block := range code {
		if paths[block] != want[i] {
			t.Errorf("block at line %d written to %q, want %q", block.Line, paths[block], want[i])
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "lib.rs")); err == nil {
		t.Errorf("src/lib.rs generated although src/main.rs is the crate root")
	}

	main := readFile(t, filepath.Join(dir, "src", "main.rs"))
	if strings.Count(main, "mod format;") != 1 {
		t.Errorf("src/main.rs declares format other than once:\n%s", main)
	}
	if !strings.Contains(main, "\nmod endpoint;\n") {
		t.Errorf("src/main.rs does not declare the undeclared endpoint module:\n%s", main)
	}

	manifest := readFile(t, filepath.Join(dir, "Cargo.toml"))
	wantManifest := `[package]
name = "skill-example"
version = "0.0.0"
edition = "2024"
publish = false

[dependencies]
# Integer formatting for the order IDs.
itoa = "=1.0.15"
ryu = "=1.0.20"
`
	if manifest != wantManifest {
		t.Errorf("Cargo.toml =\n%s\nwant\n%s", manifest, wantManifest)
	}
}

func TestRustAssembleLibRootAndBins(t *testing.T) {
	code := []*Block{
		{File: "a.md", Line: 3, Tag: "rust", Content: "// src/telemetry.rs\npub fn init() {}"},
		{File: "a.md", Line: 9, Tag: "rust", Content: "// src/logs/json.rs\npub struct Json;"},
	}
	dir := t.TempDir()
	if _, err := (rustLanguage{}).Assemble(dir, code, code); err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	lib := readFile(t, filepath.Join(dir, "src", "lib.rs"))
	if !strings.Contains(lib, "mod telemetry;") {
		t.Errorf("generated src/lib.rs does not declare telemetry:\n%s", lib)
	}
	if strings.Contains(lib, "mod json") || strings.Contains(lib, "mod logs") {
		t.Errorf("generated src/lib.rs declares a nested module, which its parent must declare:\n%s", lib)
	}

	mains := []*Block{
		{File: "b.md", Line: 3, Tag: "rust", Content: "fn main() {}"},
		{File: "b.md", Line: 20, Tag: "rust", Content: "fn main() {}"},
	}
	paths, err := rustLanguage{}.Assemble(t.TempDir(), mains, mains)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if paths[mains[0]] != "src/main.rs" || paths[mains[1]] != "src/bin/example_20.rs" {
		t.Errorf("fn main paths = %q, %q; want src/main.rs, src/bin/example_20.rs", paths[mains[0]], paths[mains[1]])
	}
}

func TestRustAssembleManifestTables(t *testing.T) {
	file := []*Block{
		{Line: 1, Tag: "toml", Content: `[[bin]]
name = "bootstrap"
path = "src/main.rs"

[dependencies]
tokio = { version = "=1.54.1", features = ["macros"] }
opentelemetry-otlp = { version = "=0.33.1", features = [
    "http-proto",
] }`},
		{Line: 20, Tag: "toml", Content: `opentelemetry-otlp = { version = "=0.33.1", features = [
    "http-proto",
] }

[dev-dependencies]
tokio-test = "0.4"`},
		{Line: 30, Tag: "toml", Annotation: AnnotationSkip, Content: "[workspace]\nmembers = []"},
		{Line: 40, Tag: "toml", Content: "# BAD: unpinned\ntokio = \"*\""},
	}
	manifest, err := assembleCargoManifest(file)
	if err != nil {
		t.Fatalf("assembleCargoManifest: %v", err)
	}
	want := `[package]
name = "skill-example"
version = "0.0.0"
edition = "2024"
publish = false

[dependencies]
tokio = { version = "=1.54.1", features = ["macros"] }
opentelemetry-otlp = { version = "=0.33.1", features = [
    "http-proto",
] }

[dev-dependencies]
tokio-test = "0.4"

[[bin]]
name = "bootstrap"
path = "src/main.rs"
`
	if manifest != want {
		t.Errorf("Cargo.toml =\n%s\nwant\n%s", manifest, want)
	}
}

func TestRustAssemblePackageTableReplacesGenerated(t *testing.T) {
	file := []*Block{{Line: 1, Tag: "toml", Content: "[package]\nname = \"checkout\"\nversion = \"0.1.0\"\nedition = \"2024\"\n\n[dependencies]\nitoa = \"1\""}}
	manifest, err := assembleCargoManifest(file)
	if err != nil {
		t.Fatalf("assembleCargoManifest: %v", err)
	}
	if strings.Contains(manifest, "skill-example") || !strings.HasPrefix(manifest, "[package]\nname = \"checkout\"") {
		t.Errorf("[package] from the toml block did not replace the generated one:\n%s", manifest)
	}
	if strings.Count(manifest, "[package]") != 1 {
		t.Errorf("Cargo.toml has more than 1 [package] table:\n%s", manifest)
	}
}

func TestRustAssembleErrors(t *testing.T) {
	conflicting := []*Block{
		{Line: 1, Tag: "toml", Content: `tokio = "=1.54.1"`},
		{Line: 5, Tag: "toml", Content: `tokio = { version = "=1.54.1", features = ["macros"] }`},
	}
	if _, err := assembleCargoManifest(conflicting); err == nil || !strings.Contains(err.Error(), "tokio is declared twice") {
		t.Errorf("conflicting dependency specs: err = %v, want a declared-twice error", err)
	}

	sameFile := []*Block{
		{Line: 3, Tag: "rust", Content: "// src/telemetry.rs\npub fn a() {}"},
		{Line: 9, Tag: "rust", Content: "// src/telemetry.rs\npub fn b() {}"},
	}
	if _, err := (rustLanguage{}).Assemble(t.TempDir(), sameFile, sameFile); err == nil || !strings.Contains(err.Error(), "lines 3 and 9") {
		t.Errorf("2 blocks claiming 1 path: err = %v, want a claimed-twice error", err)
	}

	escape := []*Block{{Line: 3, Tag: "rust", Content: "// src/../build.rs\nfn main() {}"}}
	if _, err := (rustLanguage{}).Assemble(t.TempDir(), escape, escape); err == nil {
		t.Errorf("path escaping src/: want an error")
	}
}

func TestRustDiagnostics(t *testing.T) {
	output := `    Checking skill-example v0.0.0 (/work)
src/main.rs:4:18: error[E0425]: cannot find function ` + "`app`" + ` in this scope
src/format.rs:2:9: warning: unused variable: ` + "`x`" + `
error: could not compile ` + "`skill-example`" + ` (bin "skill-example") due to 1 previous error
`
	got := rustLanguage{}.Diagnostics(output)
	if len(got) != 1 {
		t.Fatalf("diagnostics = %+v, want only the 1 error", got)
	}
	want := Diagnostic{Path: "src/main.rs", Line: 4, Column: 18, Message: "error[E0425]: cannot find function `app` in this scope"}
	if got[0] != want {
		t.Errorf("diagnostic = %+v, want %+v", got[0], want)
	}
}
