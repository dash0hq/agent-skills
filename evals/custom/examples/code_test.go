package examples

import (
	"strings"
	"testing"
)

func TestCodeValidatorNonContainerSkippedNoToolchain(t *testing.T) {
	// Complete blocks of a language without a container plug-in report
	// skipped-no-toolchain, never validated.
	code := NewCodeValidator("1.44.0", "0.20.0")
	code.Containers = NewContainerRunner(testFixturesDir, t.TempDir(), false)
	for _, lang := range []string{"python", "java", "csharp", "ruby"} {
		if code.Containers.Handles(lang) {
			continue
		}
		result := &Result{}
		code.Validate(result, &Block{Tag: lang, Content: "class C {}"})
		if result.Status != StatusSkippedNoToolchain {
			t.Errorf("%s: status = %q, want skipped-no-toolchain", lang, result.Status)
		}
		if !strings.Contains(result.Detail, lang) {
			t.Errorf("%s: detail does not name the language: %s", lang, result.Detail)
		}
	}
}

func TestCodeValidatorRoutesGoToContainers(t *testing.T) {
	code := NewCodeValidator("1.44.0", "0.20.0")
	block := &Block{File: "a.md", Line: 1, Tag: "go", Content: "package main\n\nfunc main() {}\n"}

	// Without a container runner, Go is not compiled on the host.
	result := &Result{}
	code.Validate(result, block)
	if result.Status != StatusSkippedNoToolchain {
		t.Errorf("without a container runner: status = %q, want skipped-no-toolchain", result.Status)
	}

	for _, require := range []bool{false, true} {
		code.Containers = NewContainerRunner(testFixturesDir, t.TempDir(), require)
		code.Containers.available = func() bool { return false }
		result = &Result{}
		code.Validate(result, block)
		if result.Status != "" {
			t.Errorf("require=%v: queued block has status %q before Finish", require, result.Status)
		}
		code.Finish([]*Block{block})
		want := StatusSkippedNoToolchain
		if require {
			want = StatusFailed
		}
		if result.Status != want || !strings.Contains(result.Detail, "docker unavailable") || !strings.Contains(result.Detail, "go") {
			t.Errorf("require=%v: after Finish without Docker: status = %q (%s), want %q", require, result.Status, result.Detail, want)
		}
	}
}

func TestCodeValidatorFinishPinsGo(t *testing.T) {
	// Finish hands the validator's pins to the Go plug-in.
	code := NewCodeValidator("1.44.0", "0.20.0")
	code.Containers = NewContainerRunner(testFixturesDir, t.TempDir(), false)
	code.Containers.available = func() bool { return false }
	code.Finish(nil)
	language, ok := code.Containers.languages["go"].(golangLanguage)
	if !ok || language.coreVersion != "1.44.0" || language.logVersion != "0.20.0" {
		t.Errorf("Go plug-in after Finish = %+v, want pins 1.44.0 and 0.20.0", code.Containers.languages["go"])
	}
}
