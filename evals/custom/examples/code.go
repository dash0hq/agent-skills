package examples

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// CodeValidator compiles complete SDK code blocks. Go blocks compile against a
// pinned OpenTelemetry Go SDK dependency set via the host go toolchain; blocks
// of a language with a container plug-in (see ContainerLanguage) are queued
// on Containers and compiled per rule file by Finish; blocks in every other
// language are reported skipped-no-toolchain. The zero value is not usable;
// build one with NewCodeValidator.
type CodeValidator struct {
	// goCoreVersion pins the stable OpenTelemetry Go modules
	// (OTEL_GO_CORE_VERSION).
	goCoreVersion string
	// goLogVersion pins the pre-release OpenTelemetry Go log modules
	// (OTEL_GO_LOG_VERSION).
	goLogVersion string

	// goBinary is the resolved path to the go toolchain, or "" when it is
	// unavailable on this host.
	goBinary string

	// moduleOnce guards one-time creation of the shared Go module directory
	// so repeated Go blocks reuse the same downloaded dependency cache.
	moduleOnce sync.Once
	// moduleDir is the shared temp module directory (seeded go.mod).
	moduleDir string
	// moduleErr is the error from seeding moduleDir, if any.
	moduleErr error
	// fileCounter serializes .go file names within moduleDir.
	fileMu      sync.Mutex
	fileCounter int

	// Containers compiles the languages with a container plug-in; when nil,
	// their complete blocks report skipped-no-toolchain.
	Containers *ContainerRunner
}

// NewCodeValidator builds a CodeValidator with the pinned OpenTelemetry Go
// versions. It resolves the go toolchain once; if go is absent, Go blocks
// report skipped-no-toolchain rather than failing.
func NewCodeValidator(goCoreVersion, goLogVersion string) *CodeValidator {
	goBinary, err := exec.LookPath("go")
	if err != nil {
		goBinary = ""
	}
	return &CodeValidator{
		goCoreVersion: goCoreVersion,
		goLogVersion:  goLogVersion,
		goBinary:      goBinary,
	}
}

// Validate compiles one code-complete block and records the outcome on result.
// It routes by the block's normalized tag: Go compiles on the host, a
// container language is queued until Finish, and every other language
// reports StatusSkippedNoToolchain.
func (c *CodeValidator) Validate(result *Result, block *Block) {
	switch {
	case block.Tag == "go":
		c.validateGo(result, block.Content)
	case c.Containers != nil && c.Containers.Handles(block.Tag):
		c.Containers.Enqueue(result, block)
	default:
		result.Status = StatusSkippedNoToolchain
		result.Detail = fmt.Sprintf("no %s compiler wired (no container plug-in for the language yet); complete block not compiled", block.Tag)
	}
}

// Finish compiles the queued container-language blocks, one container per
// rule file and language. all is every block of the validated tree.
func (c *CodeValidator) Finish(all []*Block) {
	if c.Containers != nil {
		c.Containers.Run(all)
	}
}

// validateGo writes content to a .go file in the shared pinned module and runs
// go build on it.
func (c *CodeValidator) validateGo(result *Result, content string) {
	if c.goBinary == "" {
		result.Status = StatusSkippedNoToolchain
		result.Detail = "go toolchain unavailable; complete Go block not compiled"
		return
	}
	c.moduleOnce.Do(c.seedModule)
	if c.moduleErr != nil {
		result.Status = StatusSkippedNoToolchain
		result.Detail = "go module setup failed: " + c.moduleErr.Error()
		return
	}

	c.fileMu.Lock()
	c.fileCounter++
	name := fmt.Sprintf("block_%d.go", c.fileCounter)
	c.fileMu.Unlock()
	path := filepath.Join(c.moduleDir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		result.Status = StatusSkippedNoToolchain
		result.Detail = "go block write failed: " + err.Error()
		return
	}
	defer os.Remove(path)

	// Build only this file. -mod=mod lets MVS resolve the indirect deps
	// deterministically from the pinned direct requires without a go.sum,
	// GOTOOLCHAIN=local pins the compiler to the host toolchain.
	cmd := exec.Command(c.goBinary, "build", "-mod=mod", "-o", os.DevNull, path)
	cmd.Dir = c.moduleDir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=")
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Status = StatusFailed
		result.Detail = "go build failed: " + strings.TrimSpace(string(output))
		return
	}
	result.Status = StatusValidated
}

// seedModule creates the shared temp module with a go.mod pinning the
// OpenTelemetry Go SDK modules the skill examples use. It runs once per
// validator.
func (c *CodeValidator) seedModule() {
	dir, err := os.MkdirTemp("", "eval-go-code-")
	if err != nil {
		c.moduleErr = err
		return
	}
	c.moduleDir = dir
	goMod := c.goModContent()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		c.moduleErr = err
		return
	}
}

// goModContent renders the seed go.mod pinning the OpenTelemetry Go modules at
// the versions from evals/custom/versions.env. The stable modules share
// goCoreVersion; the log and log-exporter modules track goLogVersion.
func (c *CodeValidator) goModContent() string {
	core := "v" + c.goCoreVersion
	log := "v" + c.goLogVersion
	var b strings.Builder
	b.WriteString("module evalcodeblock\n\ngo 1.24\n\nrequire (\n")
	requires := []struct{ path, version string }{
		{"go.opentelemetry.io/otel", core},
		{"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc", log},
		{"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp", log},
		{"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc", core},
		{"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp", core},
		{"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc", core},
		{"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp", core},
		{"go.opentelemetry.io/otel/exporters/stdout/stdoutlog", log},
		{"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric", core},
		{"go.opentelemetry.io/otel/exporters/stdout/stdouttrace", core},
		{"go.opentelemetry.io/otel/log", log},
		{"go.opentelemetry.io/otel/metric", core},
		{"go.opentelemetry.io/otel/sdk", core},
		{"go.opentelemetry.io/otel/sdk/log", log},
		{"go.opentelemetry.io/otel/sdk/metric", core},
		{"go.opentelemetry.io/otel/trace", core},
	}
	for _, r := range requires {
		fmt.Fprintf(&b, "\t%s %s\n", r.path, r.version)
	}
	b.WriteString(")\n")
	return b.String()
}

// Cleanup removes the shared module directory. It is safe to call when the
// module was never created.
func (c *CodeValidator) Cleanup() {
	if c.moduleDir != "" {
		os.RemoveAll(c.moduleDir)
	}
}
