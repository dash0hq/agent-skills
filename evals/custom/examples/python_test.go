package examples

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestClassifyPythonPathComment(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Category
	}{
		{"path comment without imports", "# app/settings.py\nSERVICE_NAME = \"checkout\"\n", CategoryCodeComplete},
		{"top-level import", "from opentelemetry import trace\n", CategoryCodeComplete},
		{"path comment naming a non-module path", "# my-app/settings.py\nSERVICE_NAME = \"checkout\"\n", CategoryCodeFragment},
		{"path comment naming a non-Python file", "# config.yaml\nSERVICE_NAME = \"checkout\"\n", CategoryCodeFragment},
		{"statements only", "span.set_attribute(\"order.id\", order_id)\n", CategoryCodeFragment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyContent(t, "python", tc.content, AnnotationNone); got != tc.want {
				t.Errorf("Classify = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPythonAssembleLayout(t *testing.T) {
	code, all := completeBlocks(t, "testdata/python-ok/rule.md", "python")
	if len(code) != 3 {
		t.Fatalf("complete Python blocks = %d, want 3", len(code))
	}
	dir := t.TempDir()
	paths, err := pythonLanguage{}.Assemble(dir, code, all)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	want := []string{
		"app/telemetry.py",
		"examples/block_" + strconv.Itoa(code[1].Line) + ".py",
		"examples/block_" + strconv.Itoa(code[2].Line) + ".py",
	}
	for i, block := range code {
		if paths[block] != want[i] {
			t.Errorf("block at line %d written to %q, want %q", block.Line, paths[block], want[i])
		}
		if _, err := os.Stat(filepath.Join(dir, want[i])); err != nil {
			t.Errorf("%s not written: %v", want[i], err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err != nil {
		t.Errorf("pinned requirements.txt not copied: %v", err)
	}

	config := readFile(t, filepath.Join(dir, "mypy.ini"))
	wantFiles := "files = " + strings.Join([]string{want[0], want[1], want[2]}, ", ") + "\n"
	for _, line := range []string{wantFiles, "check_untyped_defs = True\n", "explicit_package_bases = True\n", "disable_error_code = name-defined\n"} {
		if !strings.Contains(config, line) {
			t.Errorf("mypy.ini lacks %q:\n%s", line, config)
		}
	}
}

func TestPythonAssembleErrors(t *testing.T) {
	clash := []*Block{
		{Line: 3, Tag: "python", Content: "# app/a.py\nA = 1"},
		{Line: 9, Tag: "python", Content: "# app/a.py\nA = 2"},
	}
	if _, err := (pythonLanguage{}).Assemble(t.TempDir(), clash, clash); err == nil || !strings.Contains(err.Error(), "lines 3 and 9") {
		t.Errorf("2 blocks claiming 1 path: err = %v, want a claimed-twice error", err)
	}
	// A path comment that is not a module path is not a path comment: the
	// block falls back to the generated name instead of escaping the project.
	escape := []*Block{{Line: 3, Tag: "python", Content: "# ../a.py\nimport os"}}
	paths, err := pythonLanguage{}.Assemble(t.TempDir(), escape, escape)
	if err != nil || paths[escape[0]] != "examples/block_3.py" {
		t.Errorf("path escaping the project: path = %q, err = %v; want examples/block_3.py", paths[escape[0]], err)
	}
}

func TestPythonDiagnostics(t *testing.T) {
	output := `examples/block_5.py:7:21: error: "type[StatusCode]" has no attribute "FAILED"  [attr-defined]
examples/block_5.py:3:1: note: By default the bodies of untyped functions are not checked
app/telemetry.py:2: error: invalid syntax  [syntax]
Found 2 errors in 2 files (checked 3 source files)
`
	got := pythonLanguage{}.Diagnostics(output)
	want := []Diagnostic{
		{Path: "examples/block_5.py", Line: 7, Column: 21, Message: `error: "type[StatusCode]" has no attribute "FAILED"  [attr-defined]`},
		{Path: "app/telemetry.py", Line: 2, Column: 1, Message: "error: invalid syntax  [syntax]"},
	}
	if len(got) != len(want) {
		t.Fatalf("diagnostics = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("diagnostic %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
