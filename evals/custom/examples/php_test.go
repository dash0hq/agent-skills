package examples

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestClassifyPHP(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Category
	}{
		{"open tag", "<?php\n\necho 'ok';\n", CategoryCodeComplete},
		{"open tag with code on the same line", "<?php declare(strict_types=1);\n", CategoryCodeComplete},
		{"path comment", "// src/Telemetry.php\nfunction tracer() {}\n", CategoryCodeComplete},
		{"top-level use", "use OpenTelemetry\\API\\Globals;\n\n$tracer = Globals::tracerProvider();\n", CategoryCodeComplete},
		{"top-level use function", "use function OpenTelemetry\\Instrumentation\\hook;\nhook();\n", CategoryCodeComplete},
		{"statements only", "$span->setAttribute('order.id', $orderId);\n", CategoryCodeFragment},
		{"fully qualified name without use", "register_shutdown_function(function () {\n    \\OpenTelemetry\\API\\Globals::tracerProvider();\n});\n", CategoryCodeFragment},
		{"closure use clause", "$f = function () use ($span) {\n    $span->end();\n};\n", CategoryCodeFragment},
		{"indented trait use", "class A\n{\n    use Foo\\Bar;\n}\n", CategoryCodeFragment},
		{"path comment naming a non-PHP file", "// composer.json\n$x = 1;\n", CategoryCodeFragment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyContent(t, "php", tc.content, AnnotationNone); got != tc.want {
				t.Errorf("Classify = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPHPBlockPath(t *testing.T) {
	cases := map[string]string{
		"// src/Telemetry.php\nclass A {}":        "src/Telemetry.php",
		"<?php\n// src/Telemetry.php\nclass A {}": "src/Telemetry.php",
		"<?php\n\n// src/Telemetry.php\n":         "src/Telemetry.php",
		"<?php\nclass A {}\n// src/Telemetry.php": "",
		"use A\\B;\n// src/Telemetry.php":         "",
	}
	for content, want := range cases {
		if got := phpBlockPath(content); got != want {
			t.Errorf("phpBlockPath(%q) = %q, want %q", content, got, want)
		}
	}
}

func TestPHPAssembleLayout(t *testing.T) {
	code, all := completeBlocks(t, "testdata/php-ok/rule.md", "php")
	if len(code) != 3 {
		t.Fatalf("complete blocks = %d, want 3", len(code))
	}
	dir := t.TempDir()
	paths, err := phpLanguage{}.Assemble(dir, code, all)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	want := []string{
		"src/Telemetry.php",
		"examples/block_" + strconv.Itoa(code[1].Line) + ".php",
		"examples/block_" + strconv.Itoa(code[2].Line) + ".php",
	}
	for i, block := range code {
		if paths[block] != want[i] {
			t.Errorf("block at line %d written to %q, want %q", block.Line, paths[block], want[i])
		}
	}
	for _, name := range (phpLanguage{}).ManifestFiles() {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("pinned %s not copied: %v", name, err)
		}
	}

	// A block without an open tag gets one on its first line, so line
	// numbers stay the block's.
	written := readFile(t, filepath.Join(dir, want[1]))
	if !strings.HasPrefix(written, "<?php use OpenTelemetry\\API\\Trace\\Span;\n") {
		t.Errorf("%s does not start with an inline open tag:\n%s", want[1], written)
	}
	if strings.Count(written, "\n") != strings.Count(code[1].Content, "\n")+1 {
		t.Errorf("%s changed the block's line count", want[1])
	}
	if written := readFile(t, filepath.Join(dir, want[0])); strings.Count(written, "<?php") != 1 {
		t.Errorf("%s has a doubled open tag:\n%s", want[0], written)
	}

	neon := readFile(t, filepath.Join(dir, "phpstan.neon"))
	for _, needle := range []string{
		"level: 2",
		"    - " + want[1] + "\n    - " + want[2] + "\n    - " + want[0] + "\n",
		"identifier: variable.undefined",
		"identifier: function.notFound",
	} {
		if !strings.Contains(neon, needle) {
			t.Errorf("phpstan.neon lacks %q:\n%s", needle, neon)
		}
	}
}

func TestPHPAssembleErrors(t *testing.T) {
	clash := []*Block{
		{Line: 3, Tag: "php", Content: "// src/A.php\nclass A {}"},
		{Line: 9, Tag: "php", Content: "<?php\n// src/A.php\nclass B {}"},
	}
	if _, err := (phpLanguage{}).Assemble(t.TempDir(), clash, clash); err == nil || !strings.Contains(err.Error(), "lines 3 and 9") {
		t.Errorf("2 blocks claiming 1 path: err = %v, want a claimed-twice error", err)
	}
	for _, p := range []string{"../A.php", "vendor/autoload.php"} {
		escape := []*Block{{Line: 3, Tag: "php", Content: "// " + p + "\nclass A {}"}}
		if _, err := (phpLanguage{}).Assemble(t.TempDir(), escape, escape); err == nil {
			t.Errorf("path %q: want an error", p)
		}
	}
}

func TestPHPFetchImage(t *testing.T) {
	image, err := phpLanguage{}.FetchImage(filepath.Join(testFixturesDir, "php-service", "Dockerfile"))
	if err != nil || image != "composer:2" {
		t.Errorf("FetchImage = %q, %v; want the composer image the fixture copies from", image, err)
	}
	path := filepath.Join(t.TempDir(), "Dockerfile")
	if err := os.WriteFile(path, []byte("FROM php:8.3-cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (phpLanguage{}).FetchImage(path); err == nil || !strings.Contains(err.Error(), "composer") {
		t.Errorf("Dockerfile without composer: err = %v, want a missing-composer error", err)
	}
}

func TestPHPDiagnostics(t *testing.T) {
	output := `PHP Parse error:  syntax error, unexpected token "}" in examples/block_7.php on line 4
Parse error: syntax error, unexpected token "}" in examples/block_7.php on line 4
Errors parsing examples/block_7.php
/work/examples/block_12.php:5:Call to an undefined method OpenTelemetry\API\Trace\SpanInterface::setStatusCode().
src/Telemetry.php:9:Class App\Missing not found.
Note: Using configuration file /work/phpstan.neon.
`
	got := phpLanguage{}.Diagnostics(output)
	want := []Diagnostic{
		{Path: "examples/block_7.php", Line: 4, Column: 1, Message: `php -l: syntax error, unexpected token "}"`},
		{Path: "examples/block_12.php", Line: 5, Column: 1, Message: `PHPStan: Call to an undefined method OpenTelemetry\API\Trace\SpanInterface::setStatusCode().`},
		{Path: "src/Telemetry.php", Line: 9, Column: 1, Message: `PHPStan: Class App\Missing not found.`},
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

// TestPHPContainerCheck analyses the PHP testdata projects in the
// php-service fixture image.
func TestPHPContainerCheck(t *testing.T) {
	newRunner := containerTestRunner(t)

	t.Run("ok", func(t *testing.T) {
		runner := newRunner()
		results, all := queueFile(t, runner, "testdata/php-ok/rule.md", "php")
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
		runner := newRunner()
		path := "testdata/php-broken/rule.md"
		results, all := queueFile(t, runner, path, "php")
		runner.Run(all)
		wantLoc := path + ":" + strconv.Itoa(lineOf(t, path, "setStatusCode")) + ":"
		var mapped, pointer int
		for _, result := range results {
			if result.Status != StatusFailed {
				t.Errorf("line %d: status = %q, want failed", result.Line, result.Status)
			}
			switch {
			case strings.Contains(result.Detail, wantLoc) && strings.Contains(result.Detail, "setStatusCode"):
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
