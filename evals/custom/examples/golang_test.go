package examples

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

func TestClassifyGo(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Category
	}{
		{"package main", "package main\n\nfunc main() {}\n", CategoryCodeComplete},
		{"library package", "package telemetry\n\nfunc Init() {}\n", CategoryCodeComplete},
		{"path comment then package", "// telemetry/telemetry.go\npackage telemetry\n", CategoryCodeComplete},
		{"import and statements are a fragment", "import \"go.opentelemetry.io/otel\"\n\ntracer := otel.Tracer(\"x\")\n", CategoryCodeFragment},
		{"indented package is a fragment", "  package main\n", CategoryCodeFragment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyContent(t, "go", tc.content, AnnotationNone); got != tc.want {
				t.Errorf("Classify = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGoAssembleLayout(t *testing.T) {
	code, all := completeBlocks(t, "testdata/go-ok/rule.md", "go")
	if len(code) != 3 {
		t.Fatalf("complete Go blocks = %d, want 3 (telemetry.go and 2 package main blocks)", len(code))
	}
	dir := t.TempDir()
	paths, err := NewGoLanguage("1.44.0", "0.20.0").Assemble(dir, code, all)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	want := []string{
		"telemetry/telemetry.go",
		"examples/block_" + strconv.Itoa(code[1].Line) + "/main.go",
		"examples/block_" + strconv.Itoa(code[2].Line) + "/main.go",
	}
	for i, block := range code {
		if paths[block] != want[i] {
			t.Errorf("block at line %d written to %q, want %q", block.Line, paths[block], want[i])
		}
		written := readFile(t, filepath.Join(dir, filepath.FromSlash(want[i])))
		if written != block.Content+"\n" {
			t.Errorf("%s does not hold the block content:\n%s", want[i], written)
		}
	}
	if filepath.Dir(paths[code[1]]) == filepath.Dir(paths[code[2]]) {
		t.Errorf("2 package main blocks share the directory %s", filepath.Dir(paths[code[1]]))
	}
}

func TestGoAssembleGoMod(t *testing.T) {
	versions, err := harness.LoadVersions("../versions.env")
	if err != nil {
		t.Fatalf("LoadVersions: %v", err)
	}
	dir := t.TempDir()
	block := &Block{File: "a.md", Line: 3, Tag: "go", Content: "package main\n\nfunc main() {}"}
	if _, err := NewGoLanguage(versions.OtelGoCoreVersion, versions.OtelGoLogVersion).Assemble(dir, []*Block{block}, []*Block{block}); err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	goMod := readFile(t, filepath.Join(dir, "go.mod"))
	if !strings.HasPrefix(goMod, "module evalcodeblock\n\ngo 1.24\n\nrequire (\n") {
		t.Errorf("go.mod header =\n%s", goMod)
	}
	core := "v" + versions.OtelGoCoreVersion
	log := "v" + versions.OtelGoLogVersion
	for _, want := range []string{
		"\tgo.opentelemetry.io/otel " + core + "\n",
		"\tgo.opentelemetry.io/otel/sdk " + core + "\n",
		"\tgo.opentelemetry.io/otel/sdk/metric " + core + "\n",
		"\tgo.opentelemetry.io/otel/trace " + core + "\n",
		"\tgo.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc " + core + "\n",
		"\tgo.opentelemetry.io/otel/log " + log + "\n",
		"\tgo.opentelemetry.io/otel/sdk/log " + log + "\n",
		"\tgo.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp " + log + "\n",
		"\tgo.opentelemetry.io/otel/exporters/stdout/stdoutlog " + log + "\n",
	} {
		if !strings.Contains(goMod, want) {
			t.Errorf("go.mod lacks %q:\n%s", strings.TrimSpace(want), goMod)
		}
	}
	if got := strings.Count(goMod, "go.opentelemetry.io/"); got != len(goModRequires) {
		t.Errorf("go.mod requires %d modules, want %d", got, len(goModRequires))
	}
}

func TestGoAssembleErrors(t *testing.T) {
	block := &Block{Line: 3, Tag: "go", Content: "package main\n\nfunc main() {}"}
	if _, err := (golangLanguage{}).Assemble(t.TempDir(), []*Block{block}, nil); err == nil || !strings.Contains(err.Error(), "versions not set") {
		t.Errorf("unpinned plug-in: err = %v, want a versions-not-set error", err)
	}

	language := NewGoLanguage("1.44.0", "0.20.0")
	sameFile := []*Block{
		{Line: 3, Tag: "go", Content: "// telemetry/telemetry.go\npackage telemetry"},
		{Line: 9, Tag: "go", Content: "// telemetry/telemetry.go\npackage telemetry"},
	}
	if _, err := language.Assemble(t.TempDir(), sameFile, sameFile); err == nil || !strings.Contains(err.Error(), "lines 3 and 9") {
		t.Errorf("2 blocks claiming 1 path: err = %v, want a claimed-twice error", err)
	}

	for _, p := range []string{"../escape.go", "/abs/main.go", "a/../b.go"} {
		escape := []*Block{{Line: 3, Tag: "go", Content: "// " + p + "\npackage main"}}
		if _, err := language.Assemble(t.TempDir(), escape, escape); err == nil {
			t.Errorf("path %q: want an invalid-path error", p)
		}
	}
}

func TestGoDiagnostics(t *testing.T) {
	output := `go: downloading go.opentelemetry.io/otel v1.44.0
# evalcodeblock/examples/block_3
examples/block_3/main.go:12:17: undefined: codes
./main.go:4:2: "fmt" imported and not used
# evalcodeblock/telemetry
vet: telemetry/telemetry.go:7:2: fmt.Printf format %s reads arg #1, but call has 0 args
`
	got := golangLanguage{}.Diagnostics(output)
	want := []Diagnostic{
		{Path: "examples/block_3/main.go", Line: 12, Column: 17, Message: "undefined: codes"},
		{Path: "main.go", Line: 4, Column: 2, Message: `"fmt" imported and not used`},
		{Path: "telemetry/telemetry.go", Line: 7, Column: 2, Message: "fmt.Printf format %s reads arg #1, but call has 0 args"},
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

func TestGoCacheKeyCarriesPins(t *testing.T) {
	_, all := completeBlocks(t, "testdata/go-ok/rule.md", "go")
	key := func(core string) string {
		runner := NewContainerRunner(testFixturesDir, t.TempDir(), false)
		runner.Register(NewGoLanguage(core, "0.20.0"))
		key, err := runner.CacheKey(all)
		if err != nil {
			t.Fatalf("CacheKey: %v", err)
		}
		return key
	}
	first := key("1.44.0")
	image, err := BuildImage(filepath.Join(testFixturesDir, "go-service", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first, "containers-go-"+sanitizeKey(image)+"-") {
		t.Errorf("key = %q, want the go-service image %q in it", first, image)
	}
	if first == key("1.43.0") {
		t.Errorf("a different OTEL_GO_CORE_VERSION keeps the key %q", first)
	}
}

// TestGoContainerCheck builds and vets the Go testdata modules in the
// go-service fixture's build image.
func TestGoContainerCheck(t *testing.T) {
	newRunner := containerTestRunner(t)
	versions, err := harness.LoadVersions("../versions.env")
	if err != nil {
		t.Fatalf("LoadVersions: %v", err)
	}
	pinned := func() *ContainerRunner {
		runner := newRunner()
		runner.Register(NewGoLanguage(versions.OtelGoCoreVersion, versions.OtelGoLogVersion))
		return runner
	}

	t.Run("ok", func(t *testing.T) {
		runner := pinned()
		results, all := queueFile(t, runner, "testdata/go-ok/rule.md", "go")
		if len(results) != 3 {
			t.Fatalf("complete blocks = %d, want 3", len(results))
		}
		runner.Run(all)
		for _, result := range results {
			if result.Status != StatusValidated {
				t.Errorf("line %d: status = %q, want validated (detail: %s)", result.Line, result.Status, result.Detail)
			}
		}
	})

	t.Run("broken", func(t *testing.T) {
		runner := pinned()
		path := "testdata/go-broken/rule.md"
		results, all := queueFile(t, runner, path, "go")
		runner.Run(all)
		wantLoc := path + ":" + strconv.Itoa(lineOf(t, path, "codes.Failed")) + ":"
		var mapped, pointer int
		for _, result := range results {
			if result.Status != StatusFailed {
				t.Errorf("line %d: status = %q, want failed", result.Line, result.Status)
			}
			switch {
			case strings.Contains(result.Detail, wantLoc) && strings.Contains(result.Detail, "undefined: codes"):
				mapped++
			case strings.Contains(result.Detail, "errors in examples/block_"):
				pointer++
			default:
				t.Errorf("line %d: unexpected detail: %s", result.Line, result.Detail)
			}
		}
		if mapped != 1 || pointer != 1 {
			t.Errorf("mapped = %d, pointer = %d; want 1 each", mapped, pointer)
		}
	})
}
