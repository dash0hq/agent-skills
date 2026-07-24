// Package harness runs skill eval scenarios end to end: it prepares a fixture
// copy, starts an in-process OTLP sink and a dual-homed relay in front of it,
// invokes Claude Code headless against the fixture, builds and runs the
// (possibly agent-modified) fixture, waits for telemetry, evaluates
// deterministic assertions over the sink, and emits a Verdict with a failure
// class, retry accounting, cost, and evidence.
//
// No LLM judge participates in pass or fail: verdicts come exclusively from
// otelsink queries and the failure taxonomy defined in verdict.go.
package harness

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"
)

// Skill identifies one of the agent skills shipped in this repository.
type Skill string

// The four skills covered by the eval harness.
const (
	SkillInstrumentation Skill = "otel-instrumentation"
	SkillCollector       Skill = "otel-collector"
	SkillOTTL            Skill = "otel-ottl"
	SkillSemConv         Skill = "otel-semantic-conventions"
)

// Skills lists every skill covered by the eval harness, in a stable order.
var Skills = []Skill{SkillInstrumentation, SkillCollector, SkillOTTL, SkillSemConv}

// Valid reports whether s is one of the known skills.
func (s Skill) Valid() bool {
	for _, known := range Skills {
		if s == known {
			return true
		}
	}
	return false
}

// Placeholder contract for agent-authored artifacts.
//
// Task prompts may direct the agent to reference the environment variables
// below (for example as `${env:EVAL_OTLP_ENDPOINT}` in a Collector
// configuration). The harness supplies the actual values at run time —
// EnvOTLPEndpoint carries the relay's OTLP/HTTP base URL and EnvOTLPToken the
// per-run bearer token — to fixture containers, kind workloads, and build
// steps. Agent-authored artifacts execute verbatim: the harness never rewrites
// them to inject endpoints or credentials.
const (
	// EnvOTLPEndpoint is the environment variable carrying the OTLP endpoint
	// (the relay's OTLP/HTTP base URL) that agent-authored artifacts may
	// reference as ${env:EVAL_OTLP_ENDPOINT}.
	EnvOTLPEndpoint = "EVAL_OTLP_ENDPOINT"
	// EnvOTLPToken is the environment variable carrying the per-run bearer
	// token that agent-authored artifacts may reference as
	// ${env:EVAL_OTLP_TOKEN}. The value is generated per run and must never
	// be logged.
	EnvOTLPToken = "EVAL_OTLP_TOKEN"
	// EnvSinkDir carries the host directory the otelsink reads, for fixture
	// hooks that run an in-network sink relay writing telemetry to it (the
	// Docker topology mounts it into the sink relay container). It is internal
	// plumbing between the runner and the fixture hooks, never exposed to the
	// fixture workload itself.
	EnvSinkDir = "EVAL_SINK_DIR"
)

// Assertion decides whether the telemetry received by the sink satisfies a
// scenario. Implementations must use the sink's non-fatal query views
// (Traces, Metrics, Logs) and return an error describing the first unmet
// expectation; they must not use the sink's WaitFor* helpers, which fail the
// surrounding test instead of returning, bypassing failure classification.
type Assertion func(t *testing.T, sink *otelsink.Sink) error

// Scenario is a data-only description of one eval: what skill it exercises,
// which rule files it covers, which fixture it runs against, what the agent
// is asked to do, and how the resulting telemetry is judged.
type Scenario struct {
	// ID uniquely identifies the scenario (for example "go-http-basic").
	// It is the key used by quarantine.yaml and CI selection output.
	ID string

	// Skill is the skill under test. The runner derives the target skill
	// directory (skills/<skill>) from it for skill-consumption evidence.
	Skill Skill

	// RuleFiles lists the repository-relative rule files this scenario
	// covers (for example "skills/otel-instrumentation/rules/sdks/go.md").
	// Editing a dedicated rule file selects the scenarios declaring it;
	// the registry test enforces that every declared file exists.
	RuleFiles []string

	// FixturePath is the repository-relative directory of the fixture the
	// scenario runs against (for example "evals/custom/fixtures/go-service").
	// The runner copies it into a temporary workspace before the agent
	// touches it. Empty means the scenario starts from an empty workspace.
	FixturePath string

	// Prompt is the task given to the agent. It may reference the
	// placeholder environment variables EnvOTLPEndpoint and EnvOTLPToken
	// (see the placeholder contract above).
	Prompt string

	// Timeout is the wall-clock budget for one attempt, covering the agent
	// invocation and the fixture build and run. A stalled agent is killed
	// when it elapses and the attempt is classified ClassAgentTelemetry.
	// Zero means DefaultTimeout.
	Timeout time.Duration

	// TelemetryTimeout bounds how long the runner waits for the first
	// telemetry to arrive at the sink after the fixture runs. Zero means
	// DefaultTelemetryTimeout.
	TelemetryTimeout time.Duration

	// MaxTurns caps the agent's conversation turns (the CLI --max-turns
	// flag). Zero means DefaultMaxTurns.
	MaxTurns int

	// Smoke marks the scenario as the smoke scenario for its skill:
	// harness-code changes (evals/custom/harness/**, evals/custom/cmd/**) select one
	// smoke scenario per skill.
	Smoke bool

	// FullMatrixOnly excludes the scenario from path-based PR selection:
	// it is selected only when a full-matrix trigger (plugin manifest,
	// CLAUDE.md, pinned versions) selects every scenario, which is also
	// what nightly runs execute. No scenario currently sets it; it remains
	// for scenarios that should run only in full-matrix runs.
	FullMatrixOnly bool

	// RequiresKind marks the scenario as needing a kind Kubernetes cluster
	// (and Docker) at run time. Kind scenarios stay selectable through
	// their dedicated rule files, their fixture paths, and full-matrix
	// triggers, but skill-wide (shared rule file, SKILL.md) selection and
	// the per-skill smoke set skip them so ordinary PR runs stay light.
	// CI (U7) places RequiresKind scenarios in a dedicated heavier job
	// that provisions the cluster.
	RequiresKind bool

	// Assert judges the telemetry received by the sink. A returned error
	// classifies the attempt ClassAgentAssert.
	Assert Assertion
}

// Defaults applied by the runner when the corresponding Scenario field is zero.
const (
	// DefaultTimeout is the per-attempt wall-clock budget.
	DefaultTimeout = 10 * time.Minute
	// DefaultTelemetryTimeout is how long the runner waits for the first
	// telemetry after the fixture runs.
	DefaultTelemetryTimeout = 30 * time.Second
	// DefaultMaxTurns is the agent turn cap. Set generously as a runaway-loop
	// guard, not a tuning knob: the per-attempt scenario timeouts are the real
	// wall-clock ceiling (see scenarioTimeout/heavyScenarioTimeout). A weaker
	// agent model once exhausted a tighter 50-turn cap on instrumentation and
	// Collector tasks (error_max_turns) before converging, so keep ample
	// headroom here.
	DefaultMaxTurns = 100
)
