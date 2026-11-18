package examples

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// testFixturesDir holds the eval fixtures whose Dockerfiles name the compile
// images.
const testFixturesDir = "../fixtures"

func TestBuildImage(t *testing.T) {
	cases := []struct {
		name       string
		dockerfile string
		want       string
		wantErr    string
	}{
		{
			name:       "build stage",
			dockerfile: "# comment\nFROM rust:1.99-slim-trixie AS build\nRUN cargo build\n\nFROM debian:trixie-slim\n",
			want:       "rust:1.99-slim-trixie",
		},
		{
			name:       "lowercase keywords and platform flag",
			dockerfile: "from --platform=linux/amd64 golang:1.24 as build\n",
			want:       "golang:1.24",
		},
		{
			name:       "single-stage Dockerfile",
			dockerfile: "FROM node:22-alpine\nWORKDIR /app\n",
			want:       "node:22-alpine",
		},
		{
			name:       "several stages without a build stage",
			dockerfile: "FROM node:22-alpine AS deps\nFROM node:22-alpine\n",
			wantErr:    "neither",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Dockerfile")
			if err := os.WriteFile(path, []byte(tc.dockerfile), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := BuildImage(path)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("BuildImage = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if _, err := BuildImage(filepath.Join(t.TempDir(), "Dockerfile")); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing Dockerfile: err = %v, want not found", err)
	}
}

// queueFile enqueues the complete blocks of tags in path on runner and
// returns their results and every block of the file.
func queueFile(t *testing.T, runner *ContainerRunner, path string, tags ...string) ([]*Result, []*Block) {
	t.Helper()
	code, all := completeBlocks(t, path, tags...)
	var results []*Result
	for _, block := range code {
		result := &Result{File: block.File, Line: block.Line + 1, Tag: block.Tag, Category: CategoryCodeComplete}
		runner.Enqueue(result, block)
		results = append(results, result)
	}
	return results, all
}

func TestContainerRunnerDockerAbsent(t *testing.T) {
	for _, require := range []bool{false, true} {
		runner := NewContainerRunner(testFixturesDir, t.TempDir(), require)
		runner.available = func() bool { return false }
		results, all := queueFile(t, runner, "testdata/rust-ok/rule.md", "rust")
		runner.Run(all)
		want := StatusSkippedNoToolchain
		if require {
			want = StatusFailed
		}
		for _, result := range results {
			if result.Status != want {
				t.Errorf("require=%v line %d: status = %q, want %q", require, result.Line, result.Status, want)
			}
			if !strings.Contains(result.Detail, "docker unavailable") || !strings.Contains(result.Detail, "rust") {
				t.Errorf("require=%v line %d: detail does not explain the skip: %s", require, result.Line, result.Detail)
			}
		}
	}
}

func TestContainerRunnerFixtureMissing(t *testing.T) {
	for _, require := range []bool{false, true} {
		runner := NewContainerRunner(t.TempDir(), t.TempDir(), require)
		runner.available = func() bool { return true }
		results, all := queueFile(t, runner, "testdata/rust-ok/rule.md", "rust")
		runner.Run(all)
		want := StatusSkippedNoToolchain
		if require {
			want = StatusFailed
		}
		for _, result := range results {
			if result.Status != want || !strings.Contains(result.Detail, "Dockerfile") {
				t.Errorf("require=%v: status = %q (%s), want %q naming the Dockerfile", require, result.Status, result.Detail, want)
			}
		}
	}
}

func TestCodeValidatorRoutesRustToContainers(t *testing.T) {
	code := NewCodeValidator("1.44.0", "0.20.0")
	block := &Block{File: "a.md", Line: 1, Tag: "rust", Content: "fn main() {}"}

	result := &Result{}
	code.Validate(result, block)
	if result.Status != StatusSkippedNoToolchain {
		t.Errorf("without a container runner: status = %q, want skipped-no-toolchain", result.Status)
	}

	code.Containers = NewContainerRunner(testFixturesDir, t.TempDir(), false)
	code.Containers.available = func() bool { return false }
	result = &Result{}
	code.Validate(result, block)
	if result.Status != "" {
		t.Errorf("queued block has status %q before Finish", result.Status)
	}
	code.Finish([]*Block{block})
	if result.Status != StatusSkippedNoToolchain {
		t.Errorf("after Finish without Docker: status = %q, want skipped-no-toolchain", result.Status)
	}
}

func TestContainerCacheKey(t *testing.T) {
	runner := NewContainerRunner(testFixturesDir, t.TempDir(), false)
	_, all := completeBlocks(t, "testdata/rust-ok/rule.md", "rust")
	key, err := runner.CacheKey(all)
	if err != nil {
		t.Fatalf("CacheKey: %v", err)
	}
	if !strings.HasPrefix(key, "containers-rust-rust_1.99-slim-trixie-") {
		t.Errorf("key = %q, want the rust image in it", key)
	}
	again, err := runner.CacheKey(all)
	if err != nil || again != key {
		t.Errorf("CacheKey is not deterministic: %q then %q (%v)", key, again, err)
	}
	_, broken := completeBlocks(t, "testdata/rust-broken/rule.md", "rust")
	other, err := runner.CacheKey(broken)
	if err != nil || other == key {
		t.Errorf("different manifests share the key %q (%v)", other, err)
	}
	none, err := runner.CacheKey(nil)
	if err != nil || none != "containers-none" {
		t.Errorf("empty tree: key = %q, %v; want containers-none", none, err)
	}
}

// containerTestRunner returns a constructor for fail-closed runners backed by
// the real fixtures and cache. Container-backed tests need Docker and network
// access to pull images and packages, so they skip under -short and without
// Docker, unless EVAL_REQUIRE_CONTAINERS is set, in which case they fail
// closed.
func containerTestRunner(t *testing.T) func() *ContainerRunner {
	t.Helper()
	require := RequireContainersFromEnv()
	if testing.Short() && !require {
		t.Skip("container-backed test; skipped under -short")
	}
	if !DockerAvailable() {
		if require {
			t.Fatalf("Docker unavailable and %s is set", RequireContainersEnv)
		}
		t.Skip("Docker unavailable")
	}
	cacheDir, err := DefaultContainerCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	return func() *ContainerRunner {
		// Always fail closed here: a skip would hide whether the project
		// compiles at all.
		return NewContainerRunner(testFixturesDir, cacheDir, true)
	}
}

// lineOf returns the 1-based line of the first line in path containing
// needle.
func lineOf(t *testing.T, path, needle string) int {
	t.Helper()
	for i, line := range strings.Split(readFile(t, path), "\n") {
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	t.Fatalf("%q not found in %s", needle, path)
	return 0
}

// TestRustContainerCheck compiles the Rust testdata crates in the fixture
// build image.
func TestRustContainerCheck(t *testing.T) {
	newRunner := containerTestRunner(t)

	t.Run("ok", func(t *testing.T) {
		runner := newRunner()
		results, all := queueFile(t, runner, "testdata/rust-ok/rule.md", "rust")
		runner.Run(all)
		for _, result := range results {
			if result.Status != StatusValidated {
				t.Errorf("line %d: status = %q, want validated (detail: %s)", result.Line, result.Status, result.Detail)
			}
		}
	})

	t.Run("broken", func(t *testing.T) {
		runner := newRunner()
		path := "testdata/rust-broken/rule.md"
		results, all := queueFile(t, runner, path, "rust")
		runner.Run(all)
		appLine := lineOf(t, path, "app()")
		var mainResult, formatResult *Result
		for _, result := range results {
			if result.Status != StatusFailed {
				t.Errorf("line %d: status = %q, want failed", result.Line, result.Status)
			}
			if strings.Contains(result.Detail, "E0425") {
				mainResult = result
			} else {
				formatResult = result
			}
		}
		if mainResult == nil {
			t.Fatalf("no result carries the E0425 diagnostic: %+v", results)
		}
		if wantLoc := path + ":" + strconv.Itoa(appLine) + ":"; !strings.Contains(mainResult.Detail, wantLoc) {
			t.Errorf("diagnostic not mapped to %s: %s", wantLoc, mainResult.Detail)
		}
		if formatResult == nil || !strings.Contains(formatResult.Detail, "errors in src/main.rs") {
			t.Errorf("the module block does not point at the failing block: %+v", formatResult)
		}
	})
}

// TestJavaScriptContainerCheck type-checks the JavaScript and TypeScript
// testdata projects in the nodejs-service fixture image.
func TestJavaScriptContainerCheck(t *testing.T) {
	newRunner := containerTestRunner(t)

	t.Run("ok", func(t *testing.T) {
		runner := newRunner()
		results, all := queueFile(t, runner, "testdata/javascript-ok/rule.md", "javascript", "typescript")
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
		path := "testdata/javascript-broken/rule.md"
		results, all := queueFile(t, runner, path, "javascript", "typescript")
		runner.Run(all)
		wantLoc := path + ":" + strconv.Itoa(lineOf(t, path, "SpanStatusCode.FAILED")) + ":"
		var mapped, pointer int
		for _, result := range results {
			if result.Status != StatusFailed {
				t.Errorf("line %d: status = %q, want failed", result.Line, result.Status)
			}
			switch {
			case strings.Contains(result.Detail, wantLoc) && strings.Contains(result.Detail, "TS2339"):
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
