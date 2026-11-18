package examples

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyJava(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Category
	}{
		{"public class", "import x.Y;\n\npublic class A {}\n", CategoryCodeComplete},
		{"annotated record", "@Deprecated\nrecord Order(String id) {}\n", CategoryCodeComplete},
		{"sealed interface", "public sealed interface Shape permits Circle {}\n", CategoryCodeComplete},
		{"path comment", "// src/main/java/test/A.java\n// a comment-only file\n", CategoryCodeComplete},
		{"path comment outside src/main/java", "// A.java\nvoid run() {}\n", CategoryCodeFragment},
		{"top-level method", "import x.Y;\n\n@PostMapping(\"/a\")\npublic Object create() {\n    return null;\n}\n", CategoryCodeFragment},
		{"indented class", "    class Inner {}\n", CategoryCodeFragment},
		{"statements only", "span.setAttribute(\"order.id\", orderId);\n", CategoryCodeFragment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyContent(t, "java", tc.content, AnnotationNone); got != tc.want {
				t.Errorf("Classify = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJavaAssembleLayout(t *testing.T) {
	code, all := completeBlocks(t, "testdata/java-ok/rule.md", "java")
	if len(code) != 3 {
		t.Fatalf("complete Java blocks = %d, want 3", len(code))
	}
	dir := t.TempDir()
	paths, err := javaLanguage{}.Assemble(dir, code, all)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	want := []string{
		"src/main/java/test/checkout/telemetry/Tracing.java",
		"src/main/java/test/checkout/CheckoutService.java",
		"src/main/java/Region.java",
	}
	for i, block := range code {
		if paths[block] != want[i] {
			t.Errorf("block at line %d written to %q, want %q", block.Line, paths[block], want[i])
		}
		if got := readFile(t, filepath.Join(dir, want[i])); got != block.Content+"\n" {
			t.Errorf("%s content = %q, want the block content", want[i], got)
		}
	}
	if pom := readFile(t, filepath.Join(dir, "pom.xml")); !strings.Contains(pom, "<artifactId>opentelemetry-api</artifactId>") {
		t.Errorf("pinned pom.xml not copied: %s", pom)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("project root has %d entries, want pom.xml and src", len(entries))
	}
}

func TestJavaAssembleErrors(t *testing.T) {
	clash := []*Block{
		{Line: 3, Tag: "java", Content: "public class Main {}"},
		{Line: 9, Tag: "java", Content: "class Main { int x; }"},
	}
	if _, err := (javaLanguage{}).Assemble(t.TempDir(), clash, clash); err == nil || !strings.Contains(err.Error(), "lines 3 and 9") {
		t.Errorf("2 blocks claiming 1 path: err = %v, want a claimed-twice error", err)
	}
	escape := []*Block{{Line: 3, Tag: "java", Content: "// src/main/java/../../A.java\nclass A {}"}}
	if _, err := (javaLanguage{}).Assemble(t.TempDir(), escape, escape); err == nil {
		t.Errorf("path escaping src/main/java: want an error")
	}
	untyped := []*Block{{Line: 3, Tag: "java", Content: "import x.Y;"}}
	if _, err := (javaLanguage{}).Assemble(t.TempDir(), untyped, untyped); err == nil {
		t.Errorf("block without a type or a path: want an error")
	}
}

func TestJavaDerivedPath(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"public type wins over earlier types", "package a.b;\n\nrecord Order() {}\n\npublic final class Service {}\n", "src/main/java/a/b/Service.java"},
		{"first type without a public one", "interface Repo {}\nclass Impl implements Repo {}\n", "src/main/java/Repo.java"},
		{"annotation type", "package a;\npublic @interface Traced {}\n", "src/main/java/a/Traced.java"},
		{"no type", "import a.B;\n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := javaDerivedPath(tc.content); got != tc.want {
				t.Errorf("javaDerivedPath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJavaDiagnostics(t *testing.T) {
	output := `[INFO] Compiling 2 source files with javac [debug release 21] to target/classes
[INFO] -------------------------------------------------------------
[ERROR] COMPILATION ERROR :
[INFO] -------------------------------------------------------------
[ERROR] /work/src/main/java/Checkout.java:[6,44] cannot find symbol
  symbol:   variable FAILED
  location: class io.opentelemetry.api.trace.StatusCode
[WARNING] /work/src/main/java/Enduser.java:[3,1] some warning
[ERROR] /work/src/main/java/a/Service.java:[12,5] ';' expected
[INFO] 2 errors
[INFO] -------------------------------------------------------------
[INFO] BUILD FAILURE
[ERROR] Failed to execute goal org.apache.maven.plugins:maven-compiler-plugin:3.16.0:compile (default-compile) on project skill-examples-java: Compilation failure: Compilation failure:
[ERROR] /work/src/main/java/Checkout.java:[6,44] cannot find symbol
[ERROR]   symbol:   variable FAILED
[ERROR]   location: class io.opentelemetry.api.trace.StatusCode
[ERROR] /work/src/main/java/a/Service.java:[12,5] ';' expected
[ERROR] -> [Help 1]
`
	got := javaLanguage{}.Diagnostics(output)
	want := []Diagnostic{
		{Path: "src/main/java/Checkout.java", Line: 6, Column: 44, Message: "cannot find symbol (symbol: variable FAILED) (location: class io.opentelemetry.api.trace.StatusCode)"},
		{Path: "src/main/java/a/Service.java", Line: 12, Column: 5, Message: "';' expected"},
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
