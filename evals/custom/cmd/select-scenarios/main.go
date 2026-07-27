// Command select-scenarios maps changed repository paths to the eval
// scenarios that must run, and is the only source of scenario IDs for the CI
// workflows (scenario IDs are never hardcoded in workflow YAML). It wraps the
// registry's Select with the quarantine policy (R16):
//
//   - --gate pr: scenarios come from the diff (via --changed or
//     --changed-from-git) and quarantined IDs are excluded, so they cannot
//     block merges;
//   - --gate nightly: the full matrix, including quarantined scenarios,
//     FullMatrixOnly scenarios, and every
//     RequiresKind scenario.
//
// Output is JSON on stdout, shaped so workflows can build job matrices with
// jq and split RequiresKind scenarios into the dedicated kind job:
//
//	{
//	  "count": 2,        // scenarios that run without a kind cluster
//	  "kindCount": 1,    // scenarios needing the dedicated kind job
//	  "scenarios": [{"id": "...", "skill": "...", "requiresKind": false}],
//	  "quarantined": []  // pr: excluded IDs; nightly: quarantined IDs in the matrix
//	}
//
// An empty selection is not an error: the command prints count 0 and exits 0,
// and the workflows skip the scenario jobs.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
	"github.com/dash0hq/agent-skills/evals/custom/scenarios"
)

// The selection gates.
const (
	gatePR      = "pr"
	gateNightly = "nightly"
)

// scenarioOutput is one selected scenario in the JSON output.
type scenarioOutput struct {
	ID           string `json:"id"`
	Skill        string `json:"skill"`
	RequiresKind bool   `json:"requiresKind"`
}

// output is the JSON document written to stdout.
type output struct {
	// Count is the number of selected scenarios that run without a kind
	// cluster: the size of the agent-scenarios job matrix.
	Count int `json:"count"`
	// Scenarios lists every selected scenario (kind and non-kind) in
	// registration order.
	Scenarios []scenarioOutput `json:"scenarios"`
	// KindCount is the number of selected RequiresKind scenarios: the size
	// of the kind-scenarios job matrix.
	KindCount int `json:"kindCount"`
	// Quarantined lists quarantined scenario IDs relevant to the selection:
	// in pr mode the IDs excluded from it, in nightly mode the quarantined
	// IDs included in the full matrix.
	Quarantined []string `json:"quarantined"`
}

func main() {
	gate := flag.String("gate", gatePR, `selection gate: "pr" (diff-selected, quarantine excluded) or "nightly" (full matrix)`)
	changedFile := flag.String("changed", "", "path to a newline-delimited file of changed repository-relative paths (pr gate only)")
	baseRef := flag.String("changed-from-git", "", "base ref; changed paths come from git diff --name-only <base>...HEAD (pr gate only)")
	repoRootFlag := flag.String("repo-root", "", "repository root (default: git rev-parse --show-toplevel)")
	flag.Parse()

	if err := run(os.Stdout, os.Stderr, *gate, *changedFile, *baseRef, *repoRootFlag); err != nil {
		fmt.Fprintf(os.Stderr, "select-scenarios: %v\n", err)
		os.Exit(1)
	}
}

// run resolves the inputs, builds the selection, and writes the JSON output.
func run(stdout, stderr io.Writer, gate, changedFile, baseRef, root string) error {
	if root == "" {
		detected, err := repoRoot()
		if err != nil {
			return fmt.Errorf("cannot autodetect repo root (%v); pass --repo-root", err)
		}
		root = detected
	}

	var changed []string
	switch gate {
	case gatePR:
		switch {
		case changedFile != "" && baseRef != "":
			return fmt.Errorf("--changed and --changed-from-git are mutually exclusive")
		case changedFile != "":
			paths, err := readChangedFile(changedFile)
			if err != nil {
				return err
			}
			changed = paths
		case baseRef != "":
			paths, err := gitChangedPaths(root, baseRef)
			if err != nil {
				return err
			}
			changed = paths
		default:
			return fmt.Errorf("--gate pr requires --changed or --changed-from-git")
		}
	case gateNightly:
		if changedFile != "" || baseRef != "" {
			return fmt.Errorf("--changed and --changed-from-git are only valid with --gate pr")
		}
	default:
		return fmt.Errorf("unknown --gate %q (want %q or %q)", gate, gatePR, gateNightly)
	}

	quarantined, err := loadQuarantine(filepath.Join(root, "evals", "custom", "quarantine.yaml"))
	if err != nil {
		return err
	}

	reg := scenarios.Default()
	if err := checkQuarantineIDs(stderr, reg, quarantined, gate); err != nil {
		return err
	}

	out, err := buildSelection(reg, gate, changed, quarantined)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// buildSelection applies the gate policy over the registry: nightly returns
// the full matrix (quarantined scenarios included and reported), pr maps the
// changed paths through Registry.Select and excludes quarantined IDs.
func buildSelection(reg *harness.Registry, gate string, changed, quarantined []string) (*output, error) {
	inQuarantine := map[string]bool{}
	for _, id := range quarantined {
		inQuarantine[id] = true
	}

	out := &output{Scenarios: []scenarioOutput{}, Quarantined: []string{}}
	var selected []harness.Scenario
	switch gate {
	case gateNightly:
		selected = reg.Scenarios()
		for _, sc := range selected {
			if inQuarantine[sc.ID] {
				out.Quarantined = append(out.Quarantined, sc.ID)
			}
		}
	case gatePR:
		for _, sc := range reg.Select(changed) {
			if inQuarantine[sc.ID] {
				out.Quarantined = append(out.Quarantined, sc.ID)
				continue
			}
			selected = append(selected, sc)
		}
	default:
		return nil, fmt.Errorf("unknown gate %q", gate)
	}

	for _, sc := range selected {
		out.Scenarios = append(out.Scenarios, scenarioOutput{
			ID:           sc.ID,
			Skill:        string(sc.Skill),
			RequiresKind: sc.RequiresKind,
		})
		if sc.RequiresKind {
			out.KindCount++
		} else {
			out.Count++
		}
	}
	return out, nil
}

// quarantineFile mirrors the structure of evals/custom/quarantine.yaml.
type quarantineFile struct {
	Quarantine []quarantineEntry `yaml:"quarantine"`
}

// quarantineEntry is one quarantined scenario record.
type quarantineEntry struct {
	ID    string `yaml:"id"`
	Since string `yaml:"since"`
	Issue string `yaml:"issue"`
}

// loadQuarantine reads the quarantined scenario IDs from the YAML file at
// path. Entries without an ID are rejected so silent typos cannot disable
// the quarantine.
func loadQuarantine(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("quarantine: read %s: %w", path, err)
	}
	var qf quarantineFile
	if err := yaml.Unmarshal(data, &qf); err != nil {
		return nil, fmt.Errorf("quarantine: parse %s: %w", path, err)
	}
	ids := make([]string, 0, len(qf.Quarantine))
	for i, entry := range qf.Quarantine {
		if strings.TrimSpace(entry.ID) == "" {
			return nil, fmt.Errorf("quarantine: %s: entry %d has no id", path, i+1)
		}
		ids = append(ids, strings.TrimSpace(entry.ID))
	}
	return ids, nil
}

// checkQuarantineIDs reports quarantine entries that match no registered
// scenario. In the pr gate an unmatched ID is a hard error: a typo means the
// intended scenario is not actually excluded and silently keeps running the
// gate, defeating the quarantine. In the nightly gate it is a warning only, so
// a stale entry can never block the full-matrix run.
func checkQuarantineIDs(stderr io.Writer, reg *harness.Registry, quarantined []string, gate string) error {
	known := map[string]bool{}
	for _, sc := range reg.Scenarios() {
		known[sc.ID] = true
	}
	var unknown []string
	for _, id := range quarantined {
		if !known[id] {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	if gate == gatePR {
		return fmt.Errorf("quarantine entries match no registered scenario: %s", strings.Join(unknown, ", "))
	}
	for _, id := range unknown {
		fmt.Fprintf(stderr, "select-scenarios: warning: quarantine entry %q matches no registered scenario\n", id)
	}
	return nil
}

// readChangedFile reads a newline-delimited list of changed paths, skipping
// blank lines.
func readChangedFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read changed paths %s: %w", path, err)
	}
	return splitLines(string(data)), nil
}

// gitChangedPaths returns the paths changed between the merge base of baseRef
// and HEAD (git diff --name-only <base>...HEAD), matching how a pull request
// diff is computed.
func gitChangedPaths(root, baseRef string) ([]string, error) {
	cmd := exec.Command("git", "-C", root, "diff", "--name-only", baseRef+"...HEAD")
	out, err := cmd.Output()
	if err != nil {
		detail := ""
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			detail = ": " + strings.TrimSpace(string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("git diff --name-only %s...HEAD failed: %w%s", baseRef, err, detail)
	}
	return splitLines(string(out)), nil
}

// splitLines splits s on newlines, trimming whitespace and dropping empties.
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// repoRoot finds the repository root via git, falling back to walking up
// from the working directory until a skills/ directory appears.
func repoRoot() (string, error) {
	output, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, "skills")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no skills/ directory found walking up from the working directory")
		}
		dir = parent
	}
}
