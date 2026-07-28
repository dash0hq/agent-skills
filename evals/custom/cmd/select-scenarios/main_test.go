package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dash0hq/agent-skills/evals/custom/scenarios"
)

// ids extracts the scenario IDs from an output, optionally filtered by the
// requiresKind flag (nil means all).
func ids(out *output, requiresKind *bool) []string {
	var result []string
	for _, sc := range out.Scenarios {
		if requiresKind != nil && sc.RequiresKind != *requiresKind {
			continue
		}
		result = append(result, sc.ID)
	}
	return result
}

func boolPtr(b bool) *bool { return &b }

func TestBuildSelectionPRGoRuleFile(t *testing.T) {
	reg := scenarios.Default()
	out, err := buildSelection(reg, gatePR, []string{"skills/otel-instrumentation/rules/sdks/go.md"}, nil)
	require.NoError(t, err)

	// Exactly the scenarios declaring the Go SDK rule file (AE2).
	require.Equal(t, []string{scenarios.GoHTTPID, scenarios.GoLogsID}, ids(out, nil))
	require.Equal(t, 2, out.Count)
	require.Equal(t, 0, out.KindCount)
	require.Empty(t, out.Quarantined)
}

func TestBuildSelectionQuarantineExcludedInPRIncludedInNightly(t *testing.T) {
	reg := scenarios.Default()
	quarantined := []string{scenarios.GoHTTPID}

	pr, err := buildSelection(reg, gatePR, []string{"skills/otel-instrumentation/rules/sdks/go.md"}, quarantined)
	require.NoError(t, err)
	require.Equal(t, []string{scenarios.GoLogsID}, ids(pr, nil),
		"quarantined scenarios must be excluded from the PR gate")
	require.Equal(t, 1, pr.Count)
	require.Equal(t, []string{scenarios.GoHTTPID}, pr.Quarantined,
		"the excluded ID must be reported")

	nightly, err := buildSelection(reg, gateNightly, nil, quarantined)
	require.NoError(t, err)
	require.Contains(t, ids(nightly, nil), scenarios.GoHTTPID,
		"nightly must include quarantined scenarios")
	require.Equal(t, []string{scenarios.GoHTTPID}, nightly.Quarantined)
}

func TestBuildSelectionNightlyIsTheFullMatrix(t *testing.T) {
	reg := scenarios.Default()
	out, err := buildSelection(reg, gateNightly, nil, nil)
	require.NoError(t, err)

	all := reg.Scenarios()
	require.Len(t, out.Scenarios, len(all))
	require.Equal(t, len(all), out.Count+out.KindCount)
	require.Contains(t, ids(out, boolPtr(true)), scenarios.K8sDownwardAPIID,
		"nightly must include every RequiresKind scenario")
	require.Equal(t, 5, out.KindCount, "the 5 Kubernetes scenarios form the kind matrix")
}

func TestBuildSelectionKindSplitting(t *testing.T) {
	reg := scenarios.Default()
	out, err := buildSelection(reg, gatePR, []string{"skills/otel-instrumentation/rules/platforms/k8s.md"}, nil)
	require.NoError(t, err)

	require.Equal(t, 0, out.Count)
	require.Equal(t, 1, out.KindCount)
	require.Equal(t, []string{scenarios.K8sDownwardAPIID}, ids(out, boolPtr(true)))
	require.True(t, out.Scenarios[0].RequiresKind,
		"the requiresKind flag must let workflows split the kind job matrix")
}

func TestBuildSelectionFullMatrixTriggerViaVersionsEnv(t *testing.T) {
	reg := scenarios.Default()
	out, err := buildSelection(reg, gatePR, []string{"evals/custom/versions.env"}, nil)
	require.NoError(t, err)

	all := reg.Scenarios()
	require.Len(t, out.Scenarios, len(all), "a pin bump must select the full matrix (R19)")
	require.NotZero(t, out.KindCount)
}

func TestBuildSelectionZeroScenarioDiff(t *testing.T) {
	reg := scenarios.Default()
	out, err := buildSelection(reg, gatePR, []string{"README.md", "LICENSE"}, nil)
	require.NoError(t, err)

	require.Equal(t, 0, out.Count)
	require.Equal(t, 0, out.KindCount)
	require.Empty(t, out.Scenarios)

	// The JSON must carry empty arrays, not null, so jq in the workflows
	// never trips over a missing value.
	body, err := json.Marshal(out)
	require.NoError(t, err)
	require.Contains(t, string(body), `"scenarios":[]`)
	require.Contains(t, string(body), `"quarantined":[]`)
}

func TestRunWithChangedFileExitsCleanly(t *testing.T) {
	root := testRepoRoot(t)

	changed := filepath.Join(t.TempDir(), "changed.txt")
	require.NoError(t, os.WriteFile(changed, []byte("skills/otel-instrumentation/rules/sdks/go.md\n\n"), 0o644))

	var stdout, stderr bytes.Buffer
	require.NoError(t, run(&stdout, &stderr, gatePR, changed, "", root, ""))

	var out output
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &out))
	// The Go SDK diff selects both Go scenarios; neither is quarantined in
	// the real evals/custom/quarantine.yaml, so both run and nothing is excluded.
	require.ElementsMatch(t, []string{scenarios.GoHTTPID, scenarios.GoLogsID}, ids(&out, nil))
	require.Empty(t, out.Quarantined)
	require.Empty(t, stderr.String(), "no quarantine warnings expected")
}

func TestRunWithChangedFromGitSelectsFromDiff(t *testing.T) {
	root := gitRepoWithGoRuleChange(t)

	var stdout, stderr bytes.Buffer
	require.NoError(t, run(&stdout, &stderr, gatePR, "", "HEAD~1", root, ""))

	var out output
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &out))
	// The diff touches the Go SDK rule file, selecting both Go scenarios;
	// instr-go-logs is quarantined, so the PR gate reports it and keeps
	// instr-go-http.
	require.Equal(t, []string{scenarios.GoHTTPID}, ids(&out, nil))
	require.Contains(t, out.Quarantined, scenarios.GoLogsID)
}

func TestRunWithChangedFromGitBadRefErrors(t *testing.T) {
	root := gitRepoWithGoRuleChange(t)

	var stdout, stderr bytes.Buffer
	err := run(&stdout, &stderr, gatePR, "", "no-such-ref", root, "")
	require.Error(t, err, "an unresolvable base ref must fail")
	require.Contains(t, err.Error(), "git diff")
}

// gitRepoWithGoRuleChange initializes a throwaway git repository whose HEAD
// commit changes the Go SDK rule file (relative to its parent), plus the
// evals/custom/quarantine.yaml the run reads, and returns its root.
func gitRepoWithGoRuleChange(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=eval", "GIT_AUTHOR_EMAIL=eval@example.test",
			"GIT_COMMITTER_NAME=eval", "GIT_COMMITTER_EMAIL=eval@example.test",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}

	git("init", "-q")
	// The run reads evals/custom/quarantine.yaml; mirror the repo's known-red entry.
	write("evals/custom/quarantine.yaml", "quarantine:\n  - id: "+scenarios.GoLogsID+"\n    since: 2026-07-17\n")
	write("skills/otel-instrumentation/rules/sdks/go.md", "initial\n")
	git("add", "-A")
	git("commit", "-q", "-m", "base")

	write("skills/otel-instrumentation/rules/sdks/go.md", "initial\nchanged\n")
	git("add", "-A")
	git("commit", "-q", "-m", "touch go rule")

	return root
}

func TestRunZeroScenarioDiffExitsCleanly(t *testing.T) {
	root := testRepoRoot(t)

	changed := filepath.Join(t.TempDir(), "changed.txt")
	require.NoError(t, os.WriteFile(changed, []byte("LICENSE\n"), 0o644))

	var stdout, stderr bytes.Buffer
	require.NoError(t, run(&stdout, &stderr, gatePR, changed, "", root, ""))

	var out output
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &out))
	require.Equal(t, 0, out.Count)
	require.Equal(t, 0, out.KindCount)
}

func TestRunFlagValidation(t *testing.T) {
	root := testRepoRoot(t)
	var stdout, stderr bytes.Buffer

	require.Error(t, run(&stdout, &stderr, gatePR, "", "", root, ""),
		"pr gate requires a changed-paths input")
	require.Error(t, run(&stdout, &stderr, gatePR, "a", "b", root, ""),
		"--changed and --changed-from-git are mutually exclusive")
	require.Error(t, run(&stdout, &stderr, gateNightly, "a", "", root, ""),
		"changed inputs are invalid for the nightly gate")
	require.Error(t, run(&stdout, &stderr, "release", "", "", root, ""),
		"unknown gates are rejected")
}

func TestRunNightlyGate(t *testing.T) {
	root := testRepoRoot(t)

	var stdout, stderr bytes.Buffer
	require.NoError(t, run(&stdout, &stderr, gateNightly, "", "", root, ""))

	var out output
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &out))
	require.Equal(t, len(scenarios.Default().Scenarios()), out.Count+out.KindCount)
}

func TestLoadQuarantine(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "quarantine.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
quarantine:
  - id: instr-browser-http
    since: 2026-07-17
    issue: https://github.com/dash0hq/agent-skills/issues/123
  - id: instr-scala-http
`), 0o644))
	ids, err := loadQuarantine(path)
	require.NoError(t, err)
	require.Equal(t, []string{"instr-browser-http", "instr-scala-http"}, ids)

	empty := filepath.Join(dir, "empty.yaml")
	require.NoError(t, os.WriteFile(empty, []byte("quarantine: []\n"), 0o644))
	ids, err = loadQuarantine(empty)
	require.NoError(t, err)
	require.Empty(t, ids)

	missingID := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(missingID, []byte("quarantine:\n  - since: 2026-07-17\n"), 0o644))
	_, err = loadQuarantine(missingID)
	require.Error(t, err, "entries without an id must be rejected")

	_, err = loadQuarantine(filepath.Join(dir, "does-not-exist.yaml"))
	require.Error(t, err)
}

func TestCheckQuarantineIDs(t *testing.T) {
	reg := scenarios.Default()

	// pr gate: an unmatched ID is a hard error so a typo cannot silently
	// leave a scenario unquarantined and still running the gate.
	var prErr bytes.Buffer
	err := checkQuarantineIDs(&prErr, reg, []string{scenarios.GoHTTPID, "no-such-scenario"}, gatePR)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no-such-scenario")
	require.NotContains(t, err.Error(), scenarios.GoHTTPID)

	// nightly gate: a stale entry is a warning only, never a hard failure.
	var nightlyErr bytes.Buffer
	require.NoError(t, checkQuarantineIDs(&nightlyErr, reg, []string{"no-such-scenario"}, gateNightly))
	require.Contains(t, nightlyErr.String(), "no-such-scenario")

	// All-known IDs: no error, no warning, in either gate.
	var quiet bytes.Buffer
	require.NoError(t, checkQuarantineIDs(&quiet, reg, []string{scenarios.GoHTTPID}, gatePR))
	require.Empty(t, quiet.String())
}

// testRepoRoot walks up from the package directory to the repository root
// (the directory containing skills/), so the tests never depend on git.
func testRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if info, statErr := os.Stat(filepath.Join(dir, "skills")); statErr == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "no skills/ directory found walking up from %s", dir)
		dir = parent
	}
}
