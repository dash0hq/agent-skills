package examples

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// golangLanguage compiles the complete Go blocks of one rule file as one
// module, built and vetted inside the go-service fixture's build image.
//
// Assembly rules:
//
//  1. A block whose first non-empty line is a "// <path>.go" comment naming a
//     relative path is written to that path; any other complete block (one
//     declaring a top-level package) to examples/block_<line>/main.go, so
//     every such block is its own package and several package main blocks
//     never share a directory.
//  2. Blocks that name files in the same directory form one package and must
//     declare the same package name, as in any Go module.
//  3. The generated go.mod declares the module evalcodeblock and requires the
//     OpenTelemetry Go modules at the OTEL_GO_CORE_VERSION and
//     OTEL_GO_LOG_VERSION pins of versions.env; go mod tidy resolves every
//     other import during the network phase.
//  4. go build and go vet run over ./... with the network off and the module
//     proxy disabled, so every compiler and vet error fails its block.
type golangLanguage struct {
	// coreVersion pins the stable OpenTelemetry Go modules
	// (OTEL_GO_CORE_VERSION), without the leading "v".
	coreVersion string
	// logVersion pins the pre-release OpenTelemetry Go log modules
	// (OTEL_GO_LOG_VERSION), without the leading "v".
	logVersion string
}

// NewGoLanguage returns the Go container plug-in pinned to the OpenTelemetry
// Go module versions of versions.env. The plug-in ContainerLanguages
// registers has no pins and fails assembly; register this one on the runner
// (see ContainerRunner.Register) before compiling or computing a cache key.
func NewGoLanguage(coreVersion, logVersion string) ContainerLanguage {
	return golangLanguage{coreVersion: coreVersion, logVersion: logVersion}
}

func (golangLanguage) Name() string    { return "go" }
func (golangLanguage) Tags() []string  { return []string{"go"} }
func (golangLanguage) Fixture() string { return "go-service" }

// ManifestFiles keys the CI package cache on go.mod, which carries the pins.
func (golangLanguage) ManifestFiles() []string { return []string{"go.mod"} }

// Caches mounts GOMODCACHE, which CI caches, and GOCACHE, which keeps warm
// rebuilds fast locally.
func (golangLanguage) Caches() []ContainerCache {
	return []ContainerCache{
		{Name: "packages", Path: "/cache/gomod"},
		{Name: "build", Path: "/cache/gobuild"},
	}
}

// Env keeps the image's toolchain (GOTOOLCHAIN=local) so a go directive newer
// than the fixture's Go fails instead of downloading another toolchain.
func (golangLanguage) Env() []string {
	return []string{
		"GOMODCACHE=/cache/gomod",
		"GOCACHE=/cache/gobuild",
		"GOTOOLCHAIN=local",
		"GOFLAGS=",
		"CGO_ENABLED=0",
	}
}

// FetchCommand resolves the module graph and writes go.sum.
func (golangLanguage) FetchCommand() []string { return []string{"go", "mod", "tidy"} }

func (golangLanguage) CheckLabel() string { return "go build and go vet" }

// CheckCommand builds and vets every package with the module proxy and the
// checksum database off, so nothing is fetched in the check phase.
func (golangLanguage) CheckCommand() []string {
	return []string{"env", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=-mod=mod",
		"sh", "-c", "go build ./... && go vet ./..."}
}

// goDiagnosticRe matches a go build or go vet error line, for example
// "examples/block_12/main.go:5:2: undefined: app" or, from go vet,
// "vet: ./main.go:5:2: ...".
var goDiagnosticRe = regexp.MustCompile(`^(?:vet: )?(?:\./)?(\S+\.go):(\d+):(\d+): (.+)$`)

func (golangLanguage) Diagnostics(output string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, line := range strings.Split(output, "\n") {
		m := goDiagnosticRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		lineNo, _ := strconv.Atoi(m[2])
		column, _ := strconv.Atoi(m[3])
		diagnostics = append(diagnostics, Diagnostic{Path: m[1], Line: lineNo, Column: column, Message: m[4]})
	}
	return diagnostics
}

// goPathCommentRe matches the "// <path>.go" first line naming a block's
// file.
var goPathCommentRe = regexp.MustCompile(`^//\s*([A-Za-z0-9_./-]+\.go)\s*$`)

// goBlockPath returns the path named on the block's first non-empty line, or
// "".
func goBlockPath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := goPathCommentRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			return m[1]
		}
		return ""
	}
	return ""
}

func (l golangLanguage) Assemble(dir string, code []*Block, _ []*Block) (map[*Block]string, error) {
	if l.coreVersion == "" || l.logVersion == "" {
		return nil, errors.New("OpenTelemetry Go versions not set; register NewGoLanguage(OTEL_GO_CORE_VERSION, OTEL_GO_LOG_VERSION) on the container runner")
	}
	paths := map[*Block]string{}
	owner := map[string]*Block{}
	for _, block := range code {
		p := goBlockPath(block.Content)
		if p == "" {
			p = fmt.Sprintf("examples/block_%d/main.go", block.Line)
		}
		if path.IsAbs(p) || path.Clean(p) != p || strings.HasPrefix(p, "..") || strings.HasPrefix(p, "vendor/") {
			return nil, fmt.Errorf("block at line %d: invalid path %q", block.Line, p)
		}
		if other, taken := owner[p]; taken {
			return nil, fmt.Errorf("%s is claimed by the blocks at lines %d and %d; mark one <!-- eval:fragment -->", p, other.Line, block.Line)
		}
		owner[p] = block
		paths[block] = p
	}

	files := map[string]string{"go.mod": l.goMod()}
	for p, block := range owner {
		files[p] = block.Content + "\n"
	}
	for p, content := range files {
		target := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// goModRequires lists the OpenTelemetry Go modules the skill examples use;
// log reports whether a module tracks OTEL_GO_LOG_VERSION rather than
// OTEL_GO_CORE_VERSION.
var goModRequires = []struct {
	path string
	log  bool
}{
	{"go.opentelemetry.io/otel", false},
	{"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc", true},
	{"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp", true},
	{"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc", false},
	{"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp", false},
	{"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc", false},
	{"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp", false},
	{"go.opentelemetry.io/otel/exporters/stdout/stdoutlog", true},
	{"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric", false},
	{"go.opentelemetry.io/otel/exporters/stdout/stdouttrace", false},
	{"go.opentelemetry.io/otel/log", true},
	{"go.opentelemetry.io/otel/metric", false},
	{"go.opentelemetry.io/otel/sdk", false},
	{"go.opentelemetry.io/otel/sdk/log", true},
	{"go.opentelemetry.io/otel/sdk/metric", false},
	{"go.opentelemetry.io/otel/trace", false},
}

// goMod renders the go.mod pinning the OpenTelemetry Go modules. The go
// directive is the oldest Go the examples target; go mod tidy raises it when
// a pinned module requires a newer Go that the image provides.
func (l golangLanguage) goMod() string {
	var b strings.Builder
	b.WriteString("module evalcodeblock\n\ngo 1.24\n\nrequire (\n")
	for _, r := range goModRequires {
		version := l.coreVersion
		if r.log {
			version = l.logVersion
		}
		fmt.Fprintf(&b, "\t%s v%s\n", r.path, version)
	}
	b.WriteString(")\n")
	return b.String()
}
