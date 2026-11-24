package examples

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestClassifyJavaScriptPathComment(t *testing.T) {
	cases := []struct {
		name    string
		tag     string
		content string
		want    Category
	}{
		{"path comment without imports", "typescript", "// src/lib/flags.ts\nexport const enabled = true;\n", CategoryCodeComplete},
		{"top-level import", "javascript", "import { trace } from '@opentelemetry/api';\n", CategoryCodeComplete},
		{"path comment naming a non-script file", "javascript", "// next.config.json\nmodule.exports = {};\n", CategoryCodeFragment},
		{"statements only", "typescript", "span.setAttribute('order.id', orderId);\n", CategoryCodeFragment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyContent(t, tc.tag, tc.content, AnnotationNone); got != tc.want {
				t.Errorf("Classify = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJavaScriptAssembleLayout(t *testing.T) {
	code, all := completeBlocks(t, "testdata/javascript-ok/rule.md", "javascript", "typescript")
	dir := t.TempDir()
	paths, err := javascriptLanguage{}.Assemble(dir, code, all)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	want := []string{
		"src/lib/telemetry.ts",
		"examples/block_" + strconv.Itoa(code[1].Line) + ".ts",
		"examples/block_" + strconv.Itoa(code[2].Line) + ".js",
	}
	for i, block := range code {
		if paths[block] != want[i] {
			t.Errorf("block at line %d written to %q, want %q", block.Line, paths[block], want[i])
		}
		if _, err := os.Stat(filepath.Join(dir, want[i])); err != nil {
			t.Errorf("%s not written: %v", want[i], err)
		}
	}
	for _, name := range (javascriptLanguage{}).ManifestFiles() {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("pinned %s not copied: %v", name, err)
		}
	}

	var tsconfig struct {
		CompilerOptions map[string]any `json:"compilerOptions"`
		Files           []string       `json:"files"`
	}
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(dir, "tsconfig.json"))), &tsconfig); err != nil {
		t.Fatalf("tsconfig.json: %v", err)
	}
	if strings.Join(tsconfig.Files, ",") != strings.Join([]string{want[1], want[2], want[0]}, ",") {
		t.Errorf("tsconfig files = %v, want the 3 written blocks, sorted", tsconfig.Files)
	}
	if tsconfig.CompilerOptions["checkJs"] != true || tsconfig.CompilerOptions["noEmit"] != true {
		t.Errorf("tsconfig compilerOptions = %v, want checkJs and noEmit", tsconfig.CompilerOptions)
	}
}

func TestJavaScriptAssembleExtensionsAndErrors(t *testing.T) {
	code := []*Block{
		{Line: 3, Tag: "typescript", Content: "import x from 'y';\nexport const A = () => <div />;"},
		{Line: 9, Tag: "javascript", Content: "import x from 'y';\nexport const B = () => <p>hi</p>;"},
	}
	paths, err := javascriptLanguage{}.Assemble(t.TempDir(), code, code)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if paths[code[0]] != "examples/block_3.tsx" || paths[code[1]] != "examples/block_9.jsx" {
		t.Errorf("JSX block paths = %q, %q; want .tsx and .jsx", paths[code[0]], paths[code[1]])
	}

	clash := []*Block{
		{Line: 3, Tag: "typescript", Content: "// src/a.ts\nexport const a = 1;"},
		{Line: 9, Tag: "typescript", Content: "// src/a.ts\nexport const a = 2;"},
	}
	if _, err := (javascriptLanguage{}).Assemble(t.TempDir(), clash, clash); err == nil || !strings.Contains(err.Error(), "lines 3 and 9") {
		t.Errorf("2 blocks claiming 1 path: err = %v, want a claimed-twice error", err)
	}
	escape := []*Block{{Line: 3, Tag: "typescript", Content: "// ../a.ts\nexport const a = 1;"}}
	if _, err := (javascriptLanguage{}).Assemble(t.TempDir(), escape, escape); err == nil {
		t.Errorf("path escaping the project: want an error")
	}
}

func TestJavaScriptDiagnostics(t *testing.T) {
	output := `examples/block_5.ts(4,8): error TS2304: Cannot find name 'orderId'.
examples/block_5.ts(5,38): error TS2339: Property 'FAILED' does not exist on type 'typeof SpanStatusCode'.
src/lib/telemetry.ts(2,1): error TS2552: Cannot find name 'tracr'. Did you mean 'trace'?
`
	got := javascriptLanguage{}.Diagnostics(output)
	want := Diagnostic{Path: "examples/block_5.ts", Line: 5, Column: 38, Message: "error TS2339: Property 'FAILED' does not exist on type 'typeof SpanStatusCode'."}
	if len(got) != 1 || got[0] != want {
		t.Errorf("diagnostics = %+v, want only %+v", got, want)
	}
}
