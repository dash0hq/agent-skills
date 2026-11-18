package examples

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestClassifyRuby(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Category
	}{
		{"path comment without requires", "# config/environments/production.rb\nconfig.lograge.enabled = true\n", CategoryCodeComplete},
		{"top-level require", "require 'opentelemetry/sdk'\nOpenTelemetry::SDK.configure\n", CategoryCodeComplete},
		{"top-level require with double quotes and parentheses", "require(\"semantic_logger\")\n", CategoryCodeComplete},
		{"require_relative only", "require_relative 'lib/telemetry'\nTelemetry.setup\n", CategoryCodeFragment},
		{"indented require", "def setup\n  require 'opentelemetry/sdk'\nend\n", CategoryCodeFragment},
		{"comment naming a non-Ruby file", "# config/settings.yml\nfoo = 1\n", CategoryCodeFragment},
		{"statements only", "span = OpenTelemetry::Trace.current_span\nspan.set_attribute('order.id', order.id)\n", CategoryCodeFragment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyContent(t, "ruby", tc.content, AnnotationNone); got != tc.want {
				t.Errorf("Classify = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRubyAssembleLayout(t *testing.T) {
	code, all := completeBlocks(t, "testdata/ruby-ok/rule.md", "ruby")
	if len(code) != 2 {
		t.Fatalf("complete blocks = %d, want 2", len(code))
	}
	dir := t.TempDir()
	paths, err := rubyLanguage{}.Assemble(dir, code, all)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	want := []string{
		"config/initializers/opentelemetry.rb",
		"examples/block_" + strconv.Itoa(code[1].Line) + ".rb",
	}
	for i, block := range code {
		if paths[block] != want[i] {
			t.Errorf("block at line %d written to %q, want %q", block.Line, paths[block], want[i])
		}
		if got := readFile(t, filepath.Join(dir, want[i])); got != block.Content+"\n" {
			t.Errorf("%s content = %q, want the block content", want[i], got)
		}
	}
	for _, name := range []string{"Gemfile", "Gemfile.lock", rubyCheckDir + "/check.rb"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("pinned %s not copied: %v", name, err)
		}
	}
	if got, wantList := readFile(t, filepath.Join(dir, rubyCheckDir, "files.txt")), want[0]+"\n"+want[1]+"\n"; got != wantList {
		t.Errorf("files.txt = %q, want %q (sorted)", got, wantList)
	}
}

func TestRubyAssembleErrors(t *testing.T) {
	cases := []struct {
		name    string
		code    []*Block
		wantErr string
	}{
		{
			name: "2 blocks claiming 1 path",
			code: []*Block{
				{Line: 3, Tag: "ruby", Content: "# lib/a.rb\nA = 1"},
				{Line: 9, Tag: "ruby", Content: "# lib/a.rb\nA = 2"},
			},
			wantErr: "lines 3 and 9",
		},
		{
			name:    "path escaping the project",
			code:    []*Block{{Line: 3, Tag: "ruby", Content: "# ../a.rb\nA = 1"}},
			wantErr: "invalid path",
		},
		{
			name:    "path inside the checker directory",
			code:    []*Block{{Line: 3, Tag: "ruby", Content: "# .eval/check.rb\nA = 1"}},
			wantErr: "invalid path",
		},
		{
			name:    "path inside the gem cache",
			code:    []*Block{{Line: 3, Tag: "ruby", Content: "# vendor/cache/a.rb\nA = 1"}},
			wantErr: "invalid path",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rubyLanguage{}.Assemble(t.TempDir(), tc.code, tc.code)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestRubyDiagnostics(t *testing.T) {
	output := `examples/block_5.rb:9: warning: assigned but unused variable - x
/usr/local/bin/ruby: examples/block_7.rb:2: syntax errors found (SyntaxError)
examples/block_7.rb:2:3: error: syntax error: unexpected integer
config/initializers/opentelemetry.rb:4:34: error: undefined method 'failed' for OpenTelemetry::Trace::Status
`
	got := rubyLanguage{}.Diagnostics(output)
	want := []Diagnostic{
		{Path: "examples/block_7.rb", Line: 2, Column: 3, Message: "syntax error: unexpected integer"},
		{Path: "config/initializers/opentelemetry.rb", Line: 4, Column: 34, Message: "undefined method 'failed' for OpenTelemetry::Trace::Status"},
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

func TestRubyPinnedGemfileIsExact(t *testing.T) {
	data, err := rubyPins.ReadFile("containers/ruby/Gemfile")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "gem ") {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) != 2 || strings.ContainsAny(fields[1], "~><=") {
			t.Errorf("Gemfile line %q does not pin an exact version", line)
		}
	}
}
