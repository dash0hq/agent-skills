package examples

import (
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// csharpPins is the pinned project the C# examples are compiled against.
//
//go:embed containers/csharp/Examples.csproj
var csharpPins embed.FS

// csharpLanguage compiles the complete C# blocks of one rule file as one
// project, with dotnet build inside the dotnet-service fixture's build image.
//
// Assembly rules:
//
//  1. A block whose first non-empty line is a "// <path>.cs" comment is
//     written to that path; any other complete block (one declaring a
//     namespace or a top-level type) to examples/Block<line>.cs.
//  2. The project uses the pinned Examples.csproj under containers/csharp/:
//     the Web SDK, net8.0, nullable and implicit usings enabled, as in the
//     dotnet-service fixture; examples reference no package outside it.
//  3. Only a block written to Program.cs may hold top-level statements; that
//     file makes the project an executable, otherwise it is a library.
//  4. All blocks of the rule file share one assembly, so every name a block
//     uses must be declared in it, in another block of the file, or in the
//     pinned packages. Every compiler error fails the block it occurs in;
//     warnings do not.
type csharpLanguage struct{}

func (csharpLanguage) Name() string    { return "csharp" }
func (csharpLanguage) Tags() []string  { return []string{"csharp"} }
func (csharpLanguage) Fixture() string { return "dotnet-service" }

func (csharpLanguage) ManifestFiles() []string { return []string{"Examples.csproj"} }

// Caches mounts the NuGet global packages folder, which CI caches. The
// restore output (obj/) lives in the project dir, which both phases share.
func (csharpLanguage) Caches() []ContainerCache {
	return []ContainerCache{{Name: "packages", Path: "/cache/nuget"}}
}

func (csharpLanguage) Env() []string {
	return []string{
		"NUGET_PACKAGES=/cache/nuget",
		"DOTNET_CLI_HOME=/tmp",
		"DOTNET_CLI_TELEMETRY_OPTOUT=1",
		"DOTNET_NOLOGO=1",
		"DOTNET_GENERATE_ASPNET_CERTIFICATE=false",
	}
}

// RunArgs mounts a passwd file naming the host user: the container runs as
// the host uid, and the .NET runtime fails to create named mutexes (which
// NuGet takes on every dotnet invocation) for a uid without a passwd entry.
func (csharpLanguage) RunArgs(langDir string) ([]string, error) {
	passwd := filepath.Join(langDir, "passwd")
	content := fmt.Sprintf("root:x:0:0:root:/root:/bin/sh\nevals:x:%d:%d:evals:/tmp:/bin/sh\n", os.Getuid(), os.Getgid())
	if err := os.WriteFile(passwd, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("container passwd file: %w", err)
	}
	return []string{"-v", passwd + ":/etc/passwd:ro"}, nil
}

func (csharpLanguage) FetchCommand() []string {
	return []string{"dotnet", "restore", "-tl:off", "-v", "q"}
}

func (csharpLanguage) CheckLabel() string { return "dotnet build" }

// CheckCommand builds without restoring and prints errors only, once each.
func (csharpLanguage) CheckCommand() []string {
	return []string{"dotnet", "build", "--no-restore", "-tl:off", "-nodeReuse:false",
		"-clp:ErrorsOnly;NoSummary", "-p:UseSharedCompilation=false"}
}

// csharpDiagnosticRe matches an MSBuild error line, for example
// "/work/examples/Block12.cs(3,5): error CS0103: The name 'x' does not
// exist in the current context [/work/Examples.csproj]".
var csharpDiagnosticRe = regexp.MustCompile(`^(\S+\.cs)\((\d+),(\d+)\): (error [A-Z]+\d+: .*?)(?: \[[^\]]*\])?$`)

func (csharpLanguage) Diagnostics(output string) []Diagnostic {
	var diagnostics []Diagnostic
	seen := map[Diagnostic]bool{}
	for _, line := range strings.Split(output, "\n") {
		m := csharpDiagnosticRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		lineNo, _ := strconv.Atoi(m[2])
		column, _ := strconv.Atoi(m[3])
		d := Diagnostic{Path: strings.TrimPrefix(m[1], "/work/"), Line: lineNo, Column: column, Message: m[4]}
		if !seen[d] {
			seen[d] = true
			diagnostics = append(diagnostics, d)
		}
	}
	return diagnostics
}

// csharpPathCommentRe matches the "// <path>.cs" first line naming a block's
// file.
var csharpPathCommentRe = regexp.MustCompile(`^//\s*([A-Za-z0-9_./-]+\.cs)\s*$`)

// csharpBlockPath returns the "// <path>.cs" path named on the block's first
// non-empty line, or "".
func csharpBlockPath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := csharpPathCommentRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			return m[1]
		}
		return ""
	}
	return ""
}

// isCompleteCSharp reports whether a C# block is a compilation unit: it names
// its file with a "// <path>.cs" first line, or declares a namespace or a
// top-level type at column 0.
func isCompleteCSharp(content string) bool {
	return csharpBlockPath(content) != "" || csharpTypeRe.MatchString(content)
}

func (csharpLanguage) Assemble(dir string, code []*Block, _ []*Block) (map[*Block]string, error) {
	paths := map[*Block]string{}
	owner := map[string]*Block{}
	for _, block := range code {
		p := csharpBlockPath(block.Content)
		if p == "" {
			p = fmt.Sprintf("examples/Block%d.cs", block.Line)
		}
		if path.IsAbs(p) || path.Clean(p) != p || strings.HasPrefix(p, "..") ||
			strings.HasPrefix(p, "obj/") || strings.HasPrefix(p, "bin/") {
			return nil, fmt.Errorf("block at line %d: invalid path %q", block.Line, p)
		}
		if other, taken := owner[p]; taken {
			return nil, fmt.Errorf("%s is claimed by the blocks at lines %d and %d; mark one <!-- eval:fragment -->", p, other.Line, block.Line)
		}
		owner[p] = block
		paths[block] = p
	}

	files := map[string]string{}
	for p, block := range owner {
		files[p] = block.Content + "\n"
	}
	for _, name := range (csharpLanguage{}).ManifestFiles() {
		data, err := csharpPins.ReadFile("containers/csharp/" + name)
		if err != nil {
			return nil, err
		}
		files[name] = string(data)
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
