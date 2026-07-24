package scenarios

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dash0hq/agent-skills/evals/custom/harness"

	"github.com/dash0hq/agent-skills/evals/custom/internal/testutil"
)

// TestScenarios is the eval entrypoint: it runs every registered scenario end
// to end — real Claude Code CLI (pinned via evals/custom/versions.env), real fixture
// infrastructure — and fails on any red verdict. It skips cleanly when the
// API key is unavailable, and each scenario skips when the infrastructure it
// needs (Docker, or the pinned otelcol-contrib binary for the Collector
// scenarios; see fixtureHooksFor) is unavailable, so plain `go test ./...`
// stays hermetic.
//
// Run a single scenario with the -run pattern:
//
//	ANTHROPIC_API_KEY=... go test ./scenarios -run 'TestScenarios/instr-go-http' -v -timeout 30m
//
// or with the EVAL_SCENARIOS env filter (comma-separated scenario IDs):
//
//	EVAL_SCENARIOS=instr-go-http,instr-nodejs-http ANTHROPIC_API_KEY=... go test ./scenarios -run TestScenarios -v -timeout 60m
//
// EVAL_AGENT_BINARY overrides the CLI binary (default: `claude` on PATH; CI
// installs the pinned CLAUDE_CODE_VERSION under that name).
func TestScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping agent scenarios in -short mode")
	}
	if loaded, err := harness.LoadDotEnv(); err != nil {
		t.Fatalf("loading .env: %v", err)
	} else if len(loaded) > 0 {
		t.Logf("loaded %d variable(s) from .env: %s", len(loaded), strings.Join(loaded, ", "))
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("skipping: ANTHROPIC_API_KEY unset; refusing to invoke the real agent (set it in .env or the environment)")
	}

	root := testutil.RepoRoot(t)
	versions, err := harness.LoadVersions(filepath.Join(root, "evals", "custom", "versions.env"))
	require.NoError(t, err)
	binary := os.Getenv("EVAL_AGENT_BINARY")
	selected := scenarioFilter(os.Getenv("EVAL_SCENARIOS"))

	for _, sc := range All() {
		t.Run(sc.ID, func(t *testing.T) {
			if !selected(sc.ID) {
				t.Skip("skipped by the EVAL_SCENARIOS filter")
			}
			v := runOne(t, root, root, binary, versions.EvalModel, sc)
			if !v.Passed {
				t.Errorf("scenario %s failed with class %s: %s", sc.ID, v.Class, v.Detail)
			}
		})
	}
}

// runOne executes one scenario with the fixture hooks its ID selects (see
// fixtureHooksFor): host-process otelcol-contrib for the Collector-backed
// scenarios, the Docker topology for everything else (the browser scenario
// gets its Chromium-driven variant via fixtureHooks).
func runOne(t *testing.T, repoRootDir, pluginDir, binary, model string, sc harness.Scenario) harness.Verdict {
	t.Helper()
	runner := &harness.Runner{
		RepoRoot: repoRootDir,
		Agent:    &harness.Agent{Binary: binary, PluginDir: pluginDir, Model: model},
		Hooks:    fixtureHooksFor(t, sc),
	}
	v := runner.Run(t, sc)
	logVerdict(t, v)
	return v
}

// logVerdict records the verdict JSON (which never carries token material)
// for maintainers reading the test output. When EVAL_VERDICT_DIR names a
// directory, the verdict is also written to
// <dir>/verdict-<scenario-id>.json, so CI can upload verdicts as evidence
// artifacts and sum cost across a run (R18).
func logVerdict(t *testing.T, v harness.Verdict) {
	t.Helper()
	body, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	t.Logf("verdict for %s:\n%s", v.ScenarioID, body)
	if dir := os.Getenv("EVAL_VERDICT_DIR"); dir != "" {
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "verdict-"+v.ScenarioID+".json"), body, 0o644))
	}
}

// scenarioFilter parses the EVAL_SCENARIOS comma-separated ID list; an empty
// value selects every scenario.
func scenarioFilter(value string) func(string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return func(string) bool { return true }
	}
	wanted := map[string]bool{}
	for _, id := range strings.Split(value, ",") {
		wanted[strings.TrimSpace(id)] = true
	}
	return func(id string) bool { return wanted[id] }
}

func TestScenarioFilter(t *testing.T) {
	all := scenarioFilter("")
	require.True(t, all(GoHTTPID))
	require.True(t, all(NodeHTTPID))

	some := scenarioFilter(" instr-go-http , instr-nodejs-http ")
	require.True(t, some(GoHTTPID))
	require.True(t, some(NodeHTTPID))
	require.False(t, some(GoLogsID))
}
