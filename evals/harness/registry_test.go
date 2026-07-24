package harness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The registry completeness test against the real skills/ tree lives in the
// evals/scenarios package, where the scenarios are registered: an empty
// DefaultRegistry no longer validates once dedicated rule files are covered
// by real scenarios instead of pendingScenarios allowlist entries.

// writeSkillTree materializes a minimal skills/ tree in a temp repo root.
func writeSkillTree(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("# stub\n"), 0o644))
	}
	return root
}

func TestValidateFailsOnUnmappedRuleFile(t *testing.T) {
	root := writeSkillTree(t,
		"skills/otel-ottl/rules/redaction.md",
		"skills/otel-ottl/rules/unmapped.md",
	)
	reg := &Registry{
		classification: map[string]RuleClassification{
			"skills/otel-ottl/rules/redaction.md": {Class: ClassificationDedicated},
		},
		byID: map[string]int{},
	}
	require.NoError(t, reg.Register(Scenario{
		ID:        "ottl-redaction",
		Skill:     SkillOTTL,
		RuleFiles: []string{"skills/otel-ottl/rules/redaction.md"},
	}))

	err := reg.Validate(root, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "skills/otel-ottl/rules/unmapped.md", "the unmapped file must be named")
}

func TestValidateFailsOnScenarioDeclaringMissingRuleFile(t *testing.T) {
	root := writeSkillTree(t, "skills/otel-ottl/rules/redaction.md")
	reg := &Registry{
		classification: map[string]RuleClassification{
			"skills/otel-ottl/rules/redaction.md": {Class: ClassificationDedicated},
		},
		byID: map[string]int{},
	}
	require.NoError(t, reg.Register(Scenario{
		ID:        "ottl-redaction",
		Skill:     SkillOTTL,
		RuleFiles: []string{"skills/otel-ottl/rules/redaction.md", "skills/otel-ottl/rules/gone.md"},
	}))

	err := reg.Validate(root, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ottl-redaction")
	require.Contains(t, err.Error(), "skills/otel-ottl/rules/gone.md")
}

func TestValidateFailsOnUncoveredDedicatedFile(t *testing.T) {
	root := writeSkillTree(t, "skills/otel-ottl/rules/redaction.md")
	reg := &Registry{
		classification: map[string]RuleClassification{
			"skills/otel-ottl/rules/redaction.md": {Class: ClassificationDedicated},
		},
		byID: map[string]int{},
	}

	err := reg.Validate(root, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "skills/otel-ottl/rules/redaction.md")
	require.Contains(t, err.Error(), "pendingScenarios")

	// The same registry passes once the file is allowlisted with its unit.
	require.NoError(t, reg.Validate(root, map[string]string{
		"skills/otel-ottl/rules/redaction.md": "U5",
	}))
}

// The completeness check works in both directions: once a scenario covers a
// dedicated rule file, a leftover pendingScenarios entry for it is an error.
func TestValidateFailsOnStalePendingEntry(t *testing.T) {
	root := writeSkillTree(t, "skills/otel-ottl/rules/redaction.md")
	reg := &Registry{
		classification: map[string]RuleClassification{
			"skills/otel-ottl/rules/redaction.md": {Class: ClassificationDedicated},
		},
		byID: map[string]int{},
	}
	require.NoError(t, reg.Register(Scenario{
		ID:        "ottl-redaction",
		Skill:     SkillOTTL,
		RuleFiles: []string{"skills/otel-ottl/rules/redaction.md"},
	}))

	err := reg.Validate(root, map[string]string{
		"skills/otel-ottl/rules/redaction.md": "U5",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "stale")
	require.Contains(t, err.Error(), "skills/otel-ottl/rules/redaction.md")
}

func TestValidateFailsOnExemptWithoutReason(t *testing.T) {
	root := writeSkillTree(t, "skills/otel-ottl/rules/notes.md")
	reg := &Registry{
		classification: map[string]RuleClassification{
			"skills/otel-ottl/rules/notes.md": {Class: ClassificationExempt},
		},
		byID: map[string]int{},
	}

	err := reg.Validate(root, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no reason")
}

// selectionRegistry builds a synthetic scenario set spanning all 4 skills.
func selectionRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry()
	scenarios := []Scenario{
		{ID: "go-http", Skill: SkillInstrumentation, RuleFiles: []string{"skills/otel-instrumentation/rules/sdks/go.md"}, FixturePath: "evals/fixtures/go-service", Smoke: true},
		{ID: "nodejs-http", Skill: SkillInstrumentation, RuleFiles: []string{"skills/otel-instrumentation/rules/sdks/nodejs.md"}, FixturePath: "evals/fixtures/nodejs-service"},
		{ID: "collector-pipeline", Skill: SkillCollector, RuleFiles: []string{"skills/otel-collector/rules/pipelines.md"}, Smoke: true},
		{ID: "ottl-redaction", Skill: SkillOTTL, RuleFiles: []string{"skills/otel-ottl/rules/redaction.md"}},
		{ID: "semconv-attributes", Skill: SkillSemConv, RuleFiles: []string{"skills/otel-semantic-conventions/rules/attributes.md"}},
	}
	for _, sc := range scenarios {
		require.NoError(t, r.Register(sc))
	}
	return r
}

func TestSelect(t *testing.T) {
	r := selectionRegistry(t)
	allIDs := []string{"go-http", "nodejs-http", "collector-pipeline", "ottl-redaction", "semconv-attributes"}

	tests := []struct {
		name    string
		changed []string
		want    []string
	}{
		{
			// Covers AE2: a diff touching only the Go SDK rule file selects
			// exactly the scenarios declaring it.
			name:    "dedicated rule file selects declaring scenarios",
			changed: []string{"skills/otel-instrumentation/rules/sdks/go.md"},
			want:    []string{"go-http"},
		},
		{
			name:    "shared rule file selects all scenarios of the skill",
			changed: []string{"skills/otel-instrumentation/rules/resources.md"},
			want:    []string{"go-http", "nodejs-http"},
		},
		{
			name:    "SKILL.md selects all scenarios of the skill",
			changed: []string{"skills/otel-instrumentation/SKILL.md"},
			want:    []string{"go-http", "nodejs-http"},
		},
		{
			name:    "fixture path selects the scenarios using it",
			changed: []string{"evals/fixtures/nodejs-service/Dockerfile"},
			want:    []string{"nodejs-http"},
		},
		{
			name:    "harness change selects one smoke scenario per skill",
			changed: []string{"evals/harness/runner.go"},
			want:    []string{"go-http", "collector-pipeline", "ottl-redaction", "semconv-attributes"},
		},
		{
			name:    "cmd change selects one smoke scenario per skill",
			changed: []string{"evals/cmd/relay/main.go"},
			want:    []string{"go-http", "collector-pipeline", "ottl-redaction", "semconv-attributes"},
		},
		{
			name:    "plugin manifest selects the full matrix",
			changed: []string{".claude-plugin/plugin.json"},
			want:    allIDs,
		},
		{
			name:    "CLAUDE.md selects the full matrix",
			changed: []string{"CLAUDE.md"},
			want:    allIDs,
		},
		{
			name:    "pinned versions select the full matrix",
			changed: []string{"evals/versions.env"},
			want:    allIDs,
		},
		{
			name:    "unrelated path selects nothing",
			changed: []string{"README.md", "docs/plans/some-plan.md"},
			want:    nil,
		},
		{
			name:    "multiple paths union without duplicates",
			changed: []string{"skills/otel-instrumentation/rules/sdks/go.md", "evals/fixtures/go-service/main.go", "skills/otel-ottl/rules/redaction.md"},
			want:    []string{"go-http", "ottl-redaction"},
		},
		{
			name:    "unclassified rule file fails safe to the whole skill",
			changed: []string{"skills/otel-collector/rules/brand-new.md"},
			want:    []string{"collector-pipeline"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scenarioIDs(r.Select(tt.changed))
			if len(tt.want) == 0 {
				require.Empty(t, got)
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}

// A FullMatrixOnly scenario is excluded from every path-based selection route
// — dedicated, shared, SKILL.md, fixture, and smoke — and included only when a
// full-matrix trigger selects everything.
func TestSelectFullMatrixOnly(t *testing.T) {
	r := selectionRegistry(t)
	require.NoError(t, r.Register(Scenario{
		ID:             "full-only",
		Skill:          SkillInstrumentation,
		RuleFiles:      []string{"skills/otel-instrumentation/rules/sdks/go.md"},
		FixturePath:    "evals/fixtures/go-service",
		Smoke:          true,
		FullMatrixOnly: true,
	}))

	tests := []struct {
		name    string
		changed []string
	}{
		{"dedicated rule file", []string{"skills/otel-instrumentation/rules/sdks/go.md"}},
		{"shared rule file", []string{"skills/otel-instrumentation/rules/resources.md"}},
		{"SKILL.md", []string{"skills/otel-instrumentation/SKILL.md"}},
		{"fixture path", []string{"evals/fixtures/go-service/main.go"}},
		{"harness smoke", []string{"evals/harness/runner.go"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotContains(t, scenarioIDs(r.Select(tt.changed)), "full-only")
		})
	}

	t.Run("smoke selection still picks a per-skill scenario", func(t *testing.T) {
		got := scenarioIDs(r.Select([]string{"evals/harness/runner.go"}))
		require.Contains(t, got, "go-http", "a FullMatrixOnly scenario must not displace the skill's smoke scenario")
	})

	t.Run("full-matrix trigger includes the FullMatrixOnly scenario", func(t *testing.T) {
		require.Contains(t, scenarioIDs(r.Select([]string{"evals/versions.env"})), "full-only")
	})
}

func TestSelectExemptRuleFileSelectsNothing(t *testing.T) {
	r := &Registry{
		classification: map[string]RuleClassification{
			"skills/otel-ottl/rules/notes.md": {Class: ClassificationExempt, Reason: "prose only, no runnable guidance"},
		},
		byID: map[string]int{},
	}
	require.NoError(t, r.Register(Scenario{ID: "ottl-redaction", Skill: SkillOTTL, RuleFiles: []string{"skills/otel-ottl/rules/redaction.md"}}))

	require.Empty(t, r.Select([]string{"skills/otel-ottl/rules/notes.md"}))
}

func TestRegistryRegisterRejectsDuplicatesAndUnknownSkills(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.Register(Scenario{ID: "a", Skill: SkillOTTL}))
	require.Error(t, r.Register(Scenario{ID: "a", Skill: SkillOTTL}), "duplicate ID")
	require.Error(t, r.Register(Scenario{ID: "b", Skill: Skill("nope")}), "unknown skill")
	require.Error(t, r.Register(Scenario{Skill: SkillOTTL}), "empty ID")
}
