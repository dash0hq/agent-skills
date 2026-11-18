package examples

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestClassifyScala(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Category
	}{
		{"src path comment", "// src/main/scala/example/Flags.scala\nval enabled = true\n", CategoryCodeComplete},
		{"src path comment after blank lines", "\n// src/main/scala/A.scala\nobject A\n", CategoryCodeComplete},
		{"import and object", "import io.opentelemetry.api.trace.Span\n\nobject OrderService {\n  def tag(): Unit = ()\n}\n", CategoryCodeComplete},
		{"package and modified class", "package example\n\nfinal case class Order(id: String)\n", CategoryCodeComplete},
		{"main method", "@main def run(): Unit = println(\"TEST-0001\")\n", CategoryCodeComplete},
		{"import and top-level def", "import io.opentelemetry.api.trace.Span\n\ndef tag(): Unit = Span.current()\n", CategoryCodeFragment},
		{"object without imports", "object OrderService {\n  def tag(): Unit = ()\n}\n", CategoryCodeFragment},
		{"indented object under an import", "import scala.util.Using\n  object Nested\n", CategoryCodeFragment},
		{"sbt build definition", "// build.sbt\nlibraryDependencies += \"org.slf4j\" % \"slf4j-api\" % \"<version>\"\n", CategoryCodeFragment},
		{"statements only", "span.setStatus(StatusCode.ERROR, e.getMessage)\n", CategoryCodeFragment},
		{"path comment outside src/main/scala", "// src/test/scala/A.scala\nobject A\n", CategoryCodeFragment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyContent(t, "scala", tc.content, AnnotationNone); got != tc.want {
				t.Errorf("Classify = %q, want %q", got, tc.want)
			}
		})
	}
	if got := classifyContent(t, "scala", "import a.B\nobject C\n", AnnotationFragment); got != CategoryCodeFragment {
		t.Errorf("eval:fragment on a complete Scala block: Classify = %q, want %q", got, CategoryCodeFragment)
	}
}

func TestScalaAssembleLayout(t *testing.T) {
	code, all := completeBlocks(t, "testdata/scala-ok/rule.md", "scala")
	if len(code) != 4 {
		t.Fatalf("complete Scala blocks = %d, want 4", len(code))
	}
	dir := t.TempDir()
	paths, err := scalaLanguage{}.Assemble(dir, code, all)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	third, fourth := "block_"+strconv.Itoa(code[2].Line), "block_"+strconv.Itoa(code[3].Line)
	want := []string{
		"src/main/scala/example/Telemetry.scala",
		"src/main/scala/example/Checkout.scala",
		"examples/" + third + "/src/main/scala/Example.scala",
		"examples/" + fourth + "/src/main/scala/Example.scala",
	}
	for i, block := range code {
		if paths[block] != want[i] {
			t.Errorf("block at line %d written to %q, want %q", block.Line, paths[block], want[i])
		}
		if got := readFile(t, filepath.Join(dir, want[i])); got != block.Content+"\n" {
			t.Errorf("%s does not hold the block content: %q", want[i], got)
		}
	}
	for _, name := range (scalaLanguage{}).ManifestFiles() {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("pinned %s not copied: %v", name, err)
		}
	}
	if pin := readFile(t, filepath.Join(dir, "project/build.properties")); !strings.Contains(pin, "sbt.version=2.0.12") {
		t.Errorf("project/build.properties = %q, want the fixture's sbt version", pin)
	}
	build := readFile(t, filepath.Join(dir, "examples.sbt"))
	for _, needle := range []string{
		".aggregate(" + third + ", " + fourth + ")",
		"lazy val " + third + ` = project in file("examples/` + third + `")`,
		"lazy val " + fourth + ` = project in file("examples/` + fourth + `")`,
		"Tags.limitAll(1)",
	} {
		if !strings.Contains(build, needle) {
			t.Errorf("examples.sbt lacks %q:\n%s", needle, build)
		}
	}
}

func TestScalaAssembleErrors(t *testing.T) {
	clash := []*Block{
		{Line: 3, Tag: "scala", Content: "// src/main/scala/A.scala\nobject A"},
		{Line: 9, Tag: "scala", Content: "// src/main/scala/A.scala\nobject B"},
	}
	if _, err := (scalaLanguage{}).Assemble(t.TempDir(), clash, clash); err == nil || !strings.Contains(err.Error(), "lines 3 and 9") {
		t.Errorf("2 blocks claiming 1 path: err = %v, want a claimed-twice error", err)
	}
	escape := []*Block{{Line: 3, Tag: "scala", Content: "// src/main/scala/../../../A.scala\nobject A"}}
	if _, err := (scalaLanguage{}).Assemble(t.TempDir(), escape, escape); err == nil {
		t.Errorf("path escaping the project: want an error")
	}
	paths, err := scalaLanguage{}.Assemble(t.TempDir(), nil, nil)
	if err != nil || len(paths) != 0 {
		t.Errorf("no blocks: paths = %v, err = %v", paths, err)
	}
}

func TestScalaDiagnostics(t *testing.T) {
	output := `[info] compiling 1 Scala source to /work/target/out/jvm/scala-3.10.0/block_9/classes ...
[error] -- [E006] Not Found Error: /work/examples/block_9/src/main/scala/Example.scala:4:11
[error] 4 |  def f(o: Order): Order = saveOrder(o)
[error]   |           ^^^^^
[error]   |           Not found: type Order
[error]   |
[error]   | longer explanation available when compiling with ` + "`-explain`" + `
[error] -- [E008] Not Found Error: /work/examples/block_9/src/main/scala/Example.scala:5:52
[error] 5 |  def fail(): Unit = Span.current().setStatus(StatusCode.FAILED)
[error]   |                                                    ^^^^^^^^^^^^^^^^^
[error]   |value FAILED is not a member of object io.opentelemetry.api.trace.StatusCode
[error] -- [E006] Not Found Error: /work/src/main/scala/A.scala:1:7 --------------------
[error] 1 |import zio.ZIO
[error]   |       ^^^
[error]   |       Not found: zio - did you mean io?
[error]   |
[error] -- [E007] Type Mismatch Error: /work/src/main/scala/B.scala:2:24 ---------------
[error] 2 |object B { val x: Int = "s" }
[error]   |                        ^^^
[error]   |                        Found:    ("s" : String)
[error]   |                        Required: Int
[error] 4 errors found
[error] (Compile / compileIncremental) Compilation failed
`
	got := scalaLanguage{}.Diagnostics(output)
	want := []Diagnostic{
		{Path: "examples/block_9/src/main/scala/Example.scala", Line: 5, Column: 52, Message: "[E008] Not Found Error: value FAILED is not a member of object io.opentelemetry.api.trace.StatusCode"},
		{Path: "src/main/scala/A.scala", Line: 1, Column: 7, Message: "[E006] Not Found Error: Not found: zio - did you mean io?"},
		{Path: "src/main/scala/B.scala", Line: 2, Column: 24, Message: `[E007] Type Mismatch Error: Found:    ("s" : String) Required: Int`},
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
