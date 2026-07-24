package harness

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Classification is the three-way rule-file classification driving PR
// scenario selection.
type Classification string

// The rule-file classifications.
const (
	// ClassificationDedicated files select exactly the scenarios that
	// declare them in Scenario.RuleFiles.
	ClassificationDedicated Classification = "dedicated"
	// ClassificationShared files select all scenarios of their skill.
	ClassificationShared Classification = "shared"
	// ClassificationExempt files select no scenarios; every exempt entry
	// must record a reason.
	ClassificationExempt Classification = "exempt"
)

// RuleClassification classifies one rule file.
type RuleClassification struct {
	Class Classification
	// Reason is required when Class is ClassificationExempt.
	Reason string
}

// defaultRuleClassification classifies every rule file under skills/*/rules/**
// (repository-relative paths). SKILL.md and other non-rule files under a
// skill directory are handled separately: they select all scenarios of their
// skill. The registry test enforces that this map stays complete against the
// real skills/ tree.
var defaultRuleClassification = map[string]RuleClassification{
	// otel-instrumentation — dedicated: the 10 SDK files and the Kubernetes
	// platform file.
	"skills/otel-instrumentation/rules/sdks/browser.md":  {Class: ClassificationDedicated},
	"skills/otel-instrumentation/rules/sdks/dotnet.md":   {Class: ClassificationDedicated},
	"skills/otel-instrumentation/rules/sdks/go.md":       {Class: ClassificationDedicated},
	"skills/otel-instrumentation/rules/sdks/java.md":     {Class: ClassificationDedicated},
	"skills/otel-instrumentation/rules/sdks/nextjs.md":   {Class: ClassificationDedicated},
	"skills/otel-instrumentation/rules/sdks/nodejs.md":   {Class: ClassificationDedicated},
	"skills/otel-instrumentation/rules/sdks/php.md":      {Class: ClassificationDedicated},
	"skills/otel-instrumentation/rules/sdks/python.md":   {Class: ClassificationDedicated},
	"skills/otel-instrumentation/rules/sdks/ruby.md":     {Class: ClassificationDedicated},
	"skills/otel-instrumentation/rules/sdks/scala.md":    {Class: ClassificationDedicated},
	"skills/otel-instrumentation/rules/platforms/k8s.md": {Class: ClassificationDedicated},
	// otel-instrumentation — shared: cross-cutting files affect every SDK
	// scenario.
	"skills/otel-instrumentation/rules/capture-database-query-parameters.md": {Class: ClassificationShared},
	"skills/otel-instrumentation/rules/logs.md":                              {Class: ClassificationShared},
	"skills/otel-instrumentation/rules/metrics.md":                           {Class: ClassificationShared},
	"skills/otel-instrumentation/rules/resolve-values.md":                    {Class: ClassificationShared},
	"skills/otel-instrumentation/rules/resources.md":                         {Class: ClassificationShared},
	"skills/otel-instrumentation/rules/sensitive-data.md":                    {Class: ClassificationShared},
	"skills/otel-instrumentation/rules/spans.md":                             {Class: ClassificationShared},
	"skills/otel-instrumentation/rules/telemetry.md":                         {Class: ClassificationShared},
	"skills/otel-instrumentation/rules/validation.md":                        {Class: ClassificationShared},

	// otel-collector — dedicated: the pipeline file and the 4 deployment
	// paths.
	"skills/otel-collector/rules/pipelines.md":                         {Class: ClassificationDedicated},
	"skills/otel-collector/rules/deployment/collector-helm-chart.md":   {Class: ClassificationDedicated},
	"skills/otel-collector/rules/deployment/dash0-operator.md":         {Class: ClassificationDedicated},
	"skills/otel-collector/rules/deployment/opentelemetry-operator.md": {Class: ClassificationDedicated},
	"skills/otel-collector/rules/deployment/raw-manifests.md":          {Class: ClassificationDedicated},
	// otel-collector — shared.
	"skills/otel-collector/rules/custom-distributions.md": {Class: ClassificationShared},
	"skills/otel-collector/rules/deployment.md":           {Class: ClassificationShared},
	"skills/otel-collector/rules/exporters.md":            {Class: ClassificationShared},
	"skills/otel-collector/rules/processors.md":           {Class: ClassificationShared},
	"skills/otel-collector/rules/receivers.md":            {Class: ClassificationShared},
	"skills/otel-collector/rules/red-metrics.md":          {Class: ClassificationShared},
	"skills/otel-collector/rules/sampling.md":             {Class: ClassificationShared},

	// otel-ottl — dedicated: the redaction and enrichment task files.
	"skills/otel-ottl/rules/enrichment.md": {Class: ClassificationDedicated},
	"skills/otel-ottl/rules/redaction.md":  {Class: ClassificationDedicated},
	// otel-ottl — shared.
	"skills/otel-ottl/rules/cardinality.md":        {Class: ClassificationShared},
	"skills/otel-ottl/rules/components.md":         {Class: ClassificationShared},
	"skills/otel-ottl/rules/function-reference.md": {Class: ClassificationShared},
	"skills/otel-ottl/rules/patterns.md":           {Class: ClassificationShared},

	// otel-semantic-conventions — dedicated.
	"skills/otel-semantic-conventions/rules/attributes.md": {Class: ClassificationDedicated},
	// otel-semantic-conventions — shared.
	"skills/otel-semantic-conventions/rules/dash0.md":      {Class: ClassificationShared},
	"skills/otel-semantic-conventions/rules/versioning.md": {Class: ClassificationShared},
}

// pendingScenarios is the allowlist for dedicated rule files whose scenarios
// land in later implementation units. Each entry names the unit that delivers
// the scenario; the registry test fails on any dedicated file that is neither
// covered by a registered scenario nor listed here, so entries must be
// removed as the scenarios land.
var pendingScenarios = map[string]string{
	// All dedicated rule files are covered by registered scenarios (U3-U6
	// delivered the full matrix); the allowlist is empty and stays so unless
	// a new dedicated rule file lands ahead of its scenario.
}

// Paths that trigger a full-matrix run when changed: the plugin manifest
// (skill loading), the repo agent rules (prompt-bearing), and the pinned
// CLI/model versions.
var fullMatrixTriggers = struct {
	prefixes []string
	exact    []string
}{
	prefixes: []string{".claude-plugin/"},
	exact:    []string{"CLAUDE.md", "evals/versions.env"},
}

// Paths that select one smoke scenario per skill when changed. Changes to the
// harness, its commands, or the scenario definitions themselves alter the
// measurement instrument, so a smoke run must prove it still works — otherwise
// a scenario-file edit could weaken an assertion on a green gate.
var smokeTriggers = []string{"evals/harness/", "evals/cmd/", "evals/scenarios/"}

// Registry holds the declared scenarios and the rule-file classification, and
// maps changed paths to the scenarios that must run.
type Registry struct {
	classification map[string]RuleClassification
	scenarios      []Scenario
	byID           map[string]int
}

// NewRegistry returns an empty registry carrying the default rule-file
// classification.
func NewRegistry() *Registry {
	return &Registry{
		classification: defaultRuleClassification,
		byID:           map[string]int{},
	}
}

// defaultScenarios holds scenarios contributed through
// RegisterDefaultScenarios; DefaultRegistry registers them before any caller
// adds its own.
var defaultScenarios []Scenario

// RegisterDefaultScenarios contributes scenarios to every registry returned
// by DefaultRegistry. Scenario packages call it from init functions of their
// own files, so implementation units add scenarios without editing a shared
// declaration list. It must only be called from package initialization
// (before any DefaultRegistry call); it is not safe for concurrent use.
func RegisterDefaultScenarios(scs ...Scenario) {
	defaultScenarios = append(defaultScenarios, scs...)
}

// DefaultRegistry returns a registry with the default rule-file
// classification and the scenarios contributed via
// RegisterDefaultScenarios. The fully populated registry lives in the
// evals/scenarios package (scenarios.Default()), which registers its
// remaining declared scenarios on top of this one; dedicated rule files
// whose scenarios land in later units stay covered by the pendingScenarios
// allowlist.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	for _, sc := range defaultScenarios {
		r.MustRegister(sc)
	}
	return r
}

// Register adds a scenario, rejecting invalid skills and duplicate IDs.
func (r *Registry) Register(sc Scenario) error {
	if sc.ID == "" {
		return fmt.Errorf("registry: scenario with empty ID")
	}
	if !sc.Skill.Valid() {
		return fmt.Errorf("registry: scenario %q declares unknown skill %q", sc.ID, sc.Skill)
	}
	if _, dup := r.byID[sc.ID]; dup {
		return fmt.Errorf("registry: duplicate scenario ID %q", sc.ID)
	}
	r.byID[sc.ID] = len(r.scenarios)
	r.scenarios = append(r.scenarios, sc)
	return nil
}

// MustRegister is Register that panics on error, for use in package-level
// scenario declarations.
func (r *Registry) MustRegister(sc Scenario) {
	if err := r.Register(sc); err != nil {
		panic(err)
	}
}

// Scenarios returns all registered scenarios in registration order.
func (r *Registry) Scenarios() []Scenario {
	return append([]Scenario(nil), r.scenarios...)
}

// Select maps a set of changed repository-relative paths to the scenarios
// that must run:
//
//  1. A change to .claude-plugin/**, CLAUDE.md, or evals/versions.env selects
//     the full matrix.
//  2. A dedicated rule file selects the scenarios declaring it; a shared rule
//     file, SKILL.md, or any other file under a skill's directory selects all
//     scenarios of that skill; an exempt rule file selects nothing.
//  3. A change under a scenario's fixture path selects that scenario.
//  4. A change under evals/harness/** or evals/cmd/** selects one smoke
//     scenario per skill.
//
// The result preserves registration order and contains no duplicates.
func (r *Registry) Select(changedPaths []string) []Scenario {
	selected := map[string]bool{}
	for _, raw := range changedPaths {
		p := normalizePath(raw)
		if isFullMatrixTrigger(p) {
			return r.Scenarios()
		}
		if skill, rest, ok := splitSkillPath(p); ok {
			r.selectForSkillFile(selected, skill, rest, p)
			continue
		}
		if hasAnyPrefix(p, smokeTriggers) {
			for _, sc := range r.smokeSet() {
				selected[sc.ID] = true
			}
			continue
		}
		for _, sc := range r.scenarios {
			if sc.FixturePath != "" && underPath(p, normalizePath(sc.FixturePath)) {
				selected[sc.ID] = true
			}
		}
	}

	var out []Scenario
	for _, sc := range r.scenarios {
		// FullMatrixOnly scenarios run only via the full-matrix early
		// return above, never through path-based selection.
		if sc.FullMatrixOnly {
			continue
		}
		if selected[sc.ID] {
			out = append(out, sc)
		}
	}
	return out
}

// selectForSkillFile applies the classification rules to one changed file
// under skills/<skill>/.
func (r *Registry) selectForSkillFile(selected map[string]bool, skill Skill, rest, full string) {
	if strings.HasPrefix(rest, "rules/") {
		if rc, ok := r.classification[full]; ok {
			switch rc.Class {
			case ClassificationDedicated:
				for _, sc := range r.scenarios {
					for _, rf := range sc.RuleFiles {
						if normalizePath(rf) == full {
							selected[sc.ID] = true
						}
					}
				}
			case ClassificationShared:
				r.selectSkill(selected, skill)
			case ClassificationExempt:
				// Selects nothing by definition.
			}
			return
		}
		// Unclassified rule file: fail safe by selecting the whole skill.
		// The registry Validate check fails CI on unclassified files, so
		// this branch only softens the window before that failure lands.
		r.selectSkill(selected, skill)
		return
	}
	// SKILL.md, README.md, and any other prompt-bearing file of the skill.
	r.selectSkill(selected, skill)
}

// selectSkill selects all scenarios of the skill, except RequiresKind
// scenarios: those run only via their dedicated rule files, their fixture
// paths, and full-matrix triggers (see Scenario.RequiresKind).
func (r *Registry) selectSkill(selected map[string]bool, skill Skill) {
	for _, sc := range r.scenarios {
		if sc.Skill == skill && !sc.RequiresKind {
			selected[sc.ID] = true
		}
	}
}

// smokeSet returns one smoke scenario per skill: the first Smoke-flagged
// scenario of the skill, falling back to the skill's first scenario.
// RequiresKind scenarios never join the smoke set: smoke runs must stay
// cheap, and harness-code changes must not force cluster provisioning.
func (r *Registry) smokeSet() []Scenario {
	var out []Scenario
	for _, skill := range Skills {
		var fallback *Scenario
		var picked *Scenario
		for i := range r.scenarios {
			sc := &r.scenarios[i]
			if sc.Skill != skill || sc.FullMatrixOnly || sc.RequiresKind {
				continue
			}
			if fallback == nil {
				fallback = sc
			}
			if sc.Smoke {
				picked = sc
				break
			}
		}
		if picked == nil {
			picked = fallback
		}
		if picked != nil {
			out = append(out, *picked)
		}
	}
	return out
}

// Validate enforces the registry invariants (R17) against the skills tree
// under repoRoot:
//
//   - every file under skills/*/rules/** appears in the classification;
//   - every classified file exists on disk;
//   - every exempt classification records a reason;
//   - every scenario's declared rule files exist on disk;
//   - every dedicated rule file has at least 1 registered scenario or an
//     entry in the pending allowlist;
//   - no pending allowlist entry is stale (its file already covered by a
//     registered scenario), so entries must be removed as scenarios land.
//
// All violations are reported at once.
func (r *Registry) Validate(repoRoot string, pending map[string]string) error {
	var problems []string

	onDisk := map[string]bool{}
	skillsDir := filepath.Join(repoRoot, "skills")
	err := filepath.WalkDir(skillsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		// Only files under skills/<skill>/rules/** are rule files.
		if len(parts) >= 4 && parts[2] == "rules" {
			onDisk[rel] = true
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("registry: walk %s: %w", skillsDir, err)
	}

	for f := range onDisk {
		if _, ok := r.classification[f]; !ok {
			problems = append(problems, fmt.Sprintf("rule file %s is not classified (add it to the rule-file classification as dedicated, shared, or exempt)", f))
		}
	}
	for f, rc := range r.classification {
		if !onDisk[f] {
			problems = append(problems, fmt.Sprintf("classified rule file %s does not exist on disk", f))
		}
		if rc.Class == ClassificationExempt && rc.Reason == "" {
			problems = append(problems, fmt.Sprintf("exempt rule file %s records no reason", f))
		}
	}

	covered := map[string]bool{}
	for _, sc := range r.scenarios {
		for _, rf := range sc.RuleFiles {
			rel := normalizePath(rf)
			covered[rel] = true
			if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(rel))); err != nil {
				problems = append(problems, fmt.Sprintf("scenario %s declares rule file %s which does not exist on disk", sc.ID, rel))
			}
		}
	}

	for f, rc := range r.classification {
		if rc.Class != ClassificationDedicated {
			continue
		}
		if covered[f] {
			continue
		}
		if unit, ok := pending[f]; ok {
			_ = unit // allowlisted: the named unit delivers the scenario
			continue
		}
		problems = append(problems, fmt.Sprintf("dedicated rule file %s has no registered scenario and no pendingScenarios allowlist entry", f))
	}

	for f, unit := range pending {
		if covered[f] {
			problems = append(problems, fmt.Sprintf("pendingScenarios entry for %s (unit %s) is stale: a registered scenario already covers the file, remove the entry", f, unit))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("registry validation failed:\n  - %s", strings.Join(problems, "\n  - "))
}

// PendingScenarios returns a copy of the allowlist of dedicated rule files
// whose scenarios land in later units, keyed by rule file with the delivering
// unit as value.
func PendingScenarios() map[string]string {
	out := make(map[string]string, len(pendingScenarios))
	for k, v := range pendingScenarios {
		out[k] = v
	}
	return out
}

// --- path helpers ---

func normalizePath(p string) string {
	p = filepath.ToSlash(p)
	p = strings.TrimPrefix(p, "./")
	return path.Clean(p)
}

func isFullMatrixTrigger(p string) bool {
	for _, e := range fullMatrixTriggers.exact {
		if p == e {
			return true
		}
	}
	return hasAnyPrefix(p, fullMatrixTriggers.prefixes)
}

func hasAnyPrefix(p string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// splitSkillPath splits "skills/<skill>/<rest>" and reports whether p names a
// file of a known skill.
func splitSkillPath(p string) (Skill, string, bool) {
	rest, ok := strings.CutPrefix(p, "skills/")
	if !ok {
		return "", "", false
	}
	name, rest, ok := strings.Cut(rest, "/")
	if !ok {
		return "", "", false
	}
	skill := Skill(name)
	if !skill.Valid() {
		return "", "", false
	}
	return skill, rest, true
}

// underPath reports whether p equals dir or lies underneath it.
func underPath(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+"/")
}
