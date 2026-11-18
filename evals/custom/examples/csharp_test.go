package examples

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestClassifyCSharp(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Category
	}{
		{"path comment with top-level statements", "// Program.cs\nConsole.WriteLine(\"hi\");\n", CategoryCodeComplete},
		{"public class", "using System.Diagnostics;\n\npublic class OrderService\n{\n}\n", CategoryCodeComplete},
		{"file-scoped namespace", "namespace SkillExamples;\n\nclass A {}\n", CategoryCodeComplete},
		{"record", "public record Order(string Id);\n", CategoryCodeComplete},
		{"static partial class", "public static partial class Log {}\n", CategoryCodeComplete},
		{"top-level statements without a path", "using OpenTelemetry;\n\nvar builder = Sdk.CreateTracerProviderBuilder();\n", CategoryCodeFragment},
		{"method without a type", "[HttpPost(\"/api/orders\")]\npublic IActionResult CreateOrder() => Ok();\n", CategoryCodeFragment},
		{"indented class", "    public class Nested {}\n", CategoryCodeFragment},
		{"path comment naming a non-C# file", "// appsettings.json\nvar x = 1;\n", CategoryCodeFragment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyContent(t, "csharp", tc.content, AnnotationNone); got != tc.want {
				t.Errorf("Classify = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCSharpAssembleLayout(t *testing.T) {
	code, all := completeBlocks(t, "testdata/csharp-ok/rule.md", "csharp")
	if len(code) != 3 {
		t.Fatalf("complete C# blocks = %d, want 3", len(code))
	}
	dir := t.TempDir()
	paths, err := csharpLanguage{}.Assemble(dir, code, all)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	want := []string{
		"Telemetry/Tracing.cs",
		"examples/Block" + strconv.Itoa(code[1].Line) + ".cs",
		"Program.cs",
	}
	for i, block := range code {
		if paths[block] != want[i] {
			t.Errorf("block at line %d written to %q, want %q", block.Line, paths[block], want[i])
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(want[i]))); err != nil {
			t.Errorf("%s not written: %v", want[i], err)
		}
	}
	csproj := readFile(t, filepath.Join(dir, "Examples.csproj"))
	for _, needle := range []string{"<TargetFramework>net8.0</TargetFramework>", `Condition="Exists('Program.cs')">Exe`, `Include="OpenTelemetry"`} {
		if !strings.Contains(csproj, needle) {
			t.Errorf("Examples.csproj lacks %s", needle)
		}
	}
}

func TestCSharpAssembleErrors(t *testing.T) {
	clash := []*Block{
		{Line: 3, Tag: "csharp", Content: "// Program.cs\nvar a = 1;"},
		{Line: 9, Tag: "csharp", Content: "// Program.cs\nvar b = 2;"},
	}
	if _, err := (csharpLanguage{}).Assemble(t.TempDir(), clash, clash); err == nil || !strings.Contains(err.Error(), "lines 3 and 9") {
		t.Errorf("2 blocks claiming 1 path: err = %v, want a claimed-twice error", err)
	}
	for _, p := range []string{"../A.cs", "obj/A.cs", "./A.cs"} {
		escape := []*Block{{Line: 3, Tag: "csharp", Content: "// " + p + "\nclass A {}"}}
		if _, err := (csharpLanguage{}).Assemble(t.TempDir(), escape, escape); err == nil {
			t.Errorf("path %q: want an invalid-path error", p)
		}
	}
}

func TestCSharpDiagnostics(t *testing.T) {
	output := `/work/examples/Block12.cs(9,44): error CS0117: 'ActivityStatusCode' does not contain a definition for 'Failed' [/work/Examples.csproj]
/work/examples/Block12.cs(4,1): warning CS8618: Non-nullable property 'Id' must contain a non-null value [/work/Examples.csproj]
/work/examples/Block12.cs(9,44): error CS0117: 'ActivityStatusCode' does not contain a definition for 'Failed' [/work/Examples.csproj]
/work/Program.cs(1,1): error CS8805: Program using top-level statements must be an executable. [/work/Examples.csproj]
CSC : error CS5001: Program does not contain a static 'Main' method suitable for an entry point [/work/Examples.csproj]
`
	got := csharpLanguage{}.Diagnostics(output)
	want := []Diagnostic{
		{Path: "examples/Block12.cs", Line: 9, Column: 44, Message: "error CS0117: 'ActivityStatusCode' does not contain a definition for 'Failed'"},
		{Path: "Program.cs", Line: 1, Column: 1, Message: "error CS8805: Program using top-level statements must be an executable."},
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

func TestCSharpRunArgsPasswd(t *testing.T) {
	dir := t.TempDir()
	args, err := csharpLanguage{}.RunArgs(dir)
	if err != nil {
		t.Fatalf("RunArgs: %v", err)
	}
	passwd := filepath.Join(dir, "passwd")
	if len(args) != 2 || args[0] != "-v" || args[1] != passwd+":/etc/passwd:ro" {
		t.Errorf("args = %v, want a read-only /etc/passwd mount", args)
	}
	entry := ":" + strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid()) + ":"
	if !strings.Contains(readFile(t, passwd), entry) {
		t.Errorf("passwd file lacks the host user entry %q", entry)
	}
}
