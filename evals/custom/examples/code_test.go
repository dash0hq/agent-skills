package examples

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

// newTestCodeValidator builds a CodeValidator from the committed pins,
// skipping when the go toolchain is unavailable (the Go-compile tests require
// the host go binary).
func newTestCodeValidator(t *testing.T) *CodeValidator {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable; skipping Go-compile tests")
	}
	versions, err := harness.LoadVersions("../versions.env")
	if err != nil {
		t.Fatalf("LoadVersions: %v", err)
	}
	code := NewCodeValidator(versions.OtelGoCoreVersion, versions.OtelGoLogVersion)
	t.Cleanup(code.Cleanup)
	return code
}

func TestCodeValidatorGoCompiles(t *testing.T) {
	code := newTestCodeValidator(t)
	result := &Result{}
	code.Validate(result, "go", `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`)
	if result.Status != StatusValidated {
		t.Fatalf("status = %q, want validated (detail: %s)", result.Status, result.Detail)
	}
}

func TestCodeValidatorGoOtelBlockCompiles(t *testing.T) {
	code := newTestCodeValidator(t)
	result := &Result{}
	// A minimal but self-contained program exercising the pinned OTel Go SDK
	// modules, matching the shape of the complete block in skills/.
	code.Validate(result, "go", `package main

import (
	"context"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	_ = context.Background()
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
}
`)
	if result.Status != StatusValidated {
		t.Fatalf("status = %q, want validated (detail: %s)", result.Status, result.Detail)
	}
}

func TestCodeValidatorGoTypeErrorFails(t *testing.T) {
	code := newTestCodeValidator(t)
	result := &Result{}
	// A deliberate type error: assigning a string to an int.
	code.Validate(result, "go", `package main

func main() {
	var n int = "not an int"
	_ = n
}
`)
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(result.Detail, "go build failed") {
		t.Errorf("detail does not carry compiler output: %s", result.Detail)
	}
}

func TestCodeValidatorNonGoSkippedNoToolchain(t *testing.T) {
	// A CodeValidator that never touches go: non-Go complete blocks report
	// skipped-no-toolchain, never validated. This test does not require the
	// go toolchain.
	code := NewCodeValidator("1.44.0", "0.20.0")
	for _, lang := range []string{"python", "java", "csharp", "typescript"} {
		result := &Result{}
		code.Validate(result, lang, "class C {}")
		if result.Status != StatusSkippedNoToolchain {
			t.Errorf("%s: status = %q, want skipped-no-toolchain", lang, result.Status)
		}
		if !strings.Contains(result.Detail, lang) {
			t.Errorf("%s: detail does not name the language: %s", lang, result.Detail)
		}
	}
}

func TestCodeValidatorGoAbsentSkipsNotValidated(t *testing.T) {
	// When the go binary is unavailable, a Go block is skipped-no-toolchain,
	// never validated. Simulate absence by clearing the resolved binary path.
	code := NewCodeValidator("1.44.0", "0.20.0")
	code.goBinary = ""
	result := &Result{}
	code.Validate(result, "go", "package main\n\nfunc main() {}\n")
	if result.Status != StatusSkippedNoToolchain {
		t.Fatalf("status = %q, want skipped-no-toolchain", result.Status)
	}
	if !strings.Contains(result.Detail, "go toolchain unavailable") {
		t.Errorf("detail does not explain the skip: %s", result.Detail)
	}
}
