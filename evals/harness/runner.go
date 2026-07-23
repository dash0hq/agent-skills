package harness

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"
)

// telemetryPollInterval is how often the runner re-reads the sink while
// waiting for the first telemetry.
const telemetryPollInterval = 100 * time.Millisecond

// FixtureHooks are the pluggable per-fixture build and run steps. Actual
// fixtures (and their hooks) arrive with U3; the harness only defines the
// contract. Both hooks receive the fixture workspace (the temp copy the agent
// worked on) and the composed fixture environment (see FixtureEnv). A nil
// hook is skipped. An error from either hook classifies the attempt
// ClassAgentBuild.
type FixtureHooks struct {
	// Build compiles the possibly agent-modified fixture (for example a
	// container image build). Build steps may reach package registries.
	Build func(ctx context.Context, workdir string, env map[string]string) error
	// Run starts the fixture with the given environment and drives traffic
	// at it. At runtime the fixture must only be able to reach the relay
	// (R21); the composed environment points OTLP at the relay.
	Run func(ctx context.Context, workdir string, env map[string]string) error
}

// Runner executes scenarios end to end and applies the retry policy.
type Runner struct {
	// RepoRoot is the absolute path of the repository checkout; fixture
	// paths and skill directories resolve against it.
	RepoRoot string
	// Agent invokes the CLI (its Binary is injectable for tests).
	Agent *Agent
	// Hooks build and run the fixture; see FixtureHooks.
	Hooks FixtureHooks
}

// FixtureEnv composes the environment the harness supplies to fixtures and to
// agent-authored artifacts: OTLP export routed through the relay, the per-run
// test.id resource attribute for sink-side isolation, and the placeholder
// contract variables (EnvOTLPEndpoint, EnvOTLPToken). The sink's own Env()
// helpers serve host-local processes only; containers and cluster workloads
// need the relay address composed here.
func FixtureEnv(sink *otelsink.Sink, relay *Relay, token string) map[string]string {
	return map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": relay.HTTPEndpoint(),
		"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
		// W3C baggage-style header list; the space in "Bearer <token>" is
		// percent-encoded per the OTLP environment variable specification.
		"OTEL_EXPORTER_OTLP_HEADERS": "Authorization=Bearer%20" + token,
		"OTEL_RESOURCE_ATTRIBUTES":   otelsink.TestIDAttribute + "=" + sink.TestID(),
		EnvOTLPEndpoint:              relay.HTTPEndpoint(),
		EnvOTLPToken:                 token,
		// Host directory the otelsink reads; the Docker topology mounts it
		// into the in-network sink relay so telemetry is delivered container-
		// to-container (no host.docker.internal hop).
		EnvSinkDir: sink.Dir(),
	}
}

// attemptOutcome is the classified result of one attempt.
type attemptOutcome struct {
	class          FailureClass
	detail         string
	cost           float64
	transcriptPath string
	telemetryPaths []string
}

// Run executes the scenario under the retry policy and returns the terminal
// verdict:
//
//   - an infra-classified failure is retried without consuming the agent
//     retry, up to MaxInfraAttempts total infra failures, after which the
//     verdict is a terminal ClassInfraFail flagged non-skill-attributable;
//   - an agent-attributable failure is retried once (MaxAgentAttempts total
//     agent attempts); a pass on the retry is recorded as a flake.
func (r *Runner) Run(t *testing.T, sc Scenario) Verdict {
	t.Helper()

	v := Verdict{ScenarioID: sc.ID}
	runDir := t.TempDir()
	infraFailures := 0
	agentAttempts := 0
	var firstAgentClass FailureClass

	for attempt := 1; ; attempt++ {
		out := r.attempt(t, sc, filepath.Join(runDir, fmt.Sprintf("attempt-%d", attempt)))
		v.CostUSD += out.cost
		if out.transcriptPath != "" {
			v.Evidence.TranscriptPaths = append(v.Evidence.TranscriptPaths, out.transcriptPath)
		}
		if len(out.telemetryPaths) > 0 {
			v.Evidence.TelemetryPaths = out.telemetryPaths
		}

		if out.class == ClassInfra {
			infraFailures++
			v.InfraRetries = infraFailures
			if infraFailures >= MaxInfraAttempts {
				v.Class = ClassInfraFail
				v.Detail = fmt.Sprintf("terminal after %d infra failures (not attributable to the skill); last: %s", infraFailures, out.detail)
				v.NonSkillAttributable = true
				return v
			}
			continue
		}

		agentAttempts++
		v.AgentAttempts = agentAttempts

		if out.class == ClassNone {
			v.Passed = true
			if firstAgentClass != ClassNone {
				v.Flake = true
				v.FlakeNote = fmt.Sprintf("passed on retry after %s on the first agent attempt", firstAgentClass)
			}
			return v
		}

		if firstAgentClass == ClassNone {
			firstAgentClass = out.class
		}
		if agentAttempts >= MaxAgentAttempts {
			v.Class = out.class
			v.Detail = out.detail
			return v
		}
	}
}

// attempt runs the scenario once: fixture copy, sink, relay, agent, fixture
// hooks, telemetry wait, assertions.
func (r *Runner) attempt(t *testing.T, sc Scenario, attemptDir string) attemptOutcome {
	t.Helper()

	if err := os.MkdirAll(attemptDir, 0o755); err != nil {
		return attemptOutcome{class: ClassInfra, detail: fmt.Sprintf("create attempt dir: %v", err)}
	}

	timeout := sc.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Prepare the fixture workspace copy the agent may modify.
	workdir := filepath.Join(attemptDir, "workspace")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return attemptOutcome{class: ClassInfra, detail: fmt.Sprintf("create workspace: %v", err)}
	}
	if sc.FixturePath != "" {
		src := filepath.Join(r.RepoRoot, filepath.FromSlash(normalizePath(sc.FixturePath)))
		if err := copyDir(src, workdir); err != nil {
			return attemptOutcome{class: ClassInfra, detail: fmt.Sprintf("copy fixture %s: %v", sc.FixturePath, err)}
		}
	}

	// Start the sink and the relay in front of it; the relay requires the
	// per-run bearer token, which is never logged.
	sink := otelsink.Start(t)
	token, err := NewToken()
	if err != nil {
		return attemptOutcome{class: ClassInfra, detail: err.Error()}
	}
	relay, err := StartRelay(RelayConfig{Upstream: sink.HTTPEndpoint(), BearerToken: token})
	if err != nil {
		return attemptOutcome{class: ClassInfra, detail: fmt.Sprintf("start relay: %v", err)}
	}
	defer relay.Close()

	env := FixtureEnv(sink, relay, token)
	finish := func(class FailureClass, detail string, cost float64, transcript string) attemptOutcome {
		return attemptOutcome{
			class:          class,
			detail:         detail,
			cost:           cost,
			transcriptPath: transcript,
			telemetryPaths: telemetryFiles(sink.Dir()),
		}
	}

	// Invoke the agent against the workspace.
	transcript := filepath.Join(attemptDir, "transcript.jsonl")
	res, err := r.Agent.Invoke(ctx, Invocation{
		Prompt:         sc.Prompt,
		MaxTurns:       sc.MaxTurns,
		WorkDir:        workdir,
		Env:            env,
		Skill:          sc.Skill,
		TranscriptPath: transcript,
	})
	if err != nil {
		return finish(ClassInfra, fmt.Sprintf("agent invocation: %v", err), 0, "")
	}
	if class, detail := classifyAgentRun(res); class != ClassNone {
		return finish(class, detail, res.CostUSD, transcript)
	}

	// Build and run the (possibly agent-modified) fixture.
	if r.Hooks.Build != nil {
		if err := r.Hooks.Build(ctx, workdir, env); err != nil {
			return finish(ClassAgentBuild, fmt.Sprintf("fixture build failed: %v", err), res.CostUSD, transcript)
		}
	}
	if r.Hooks.Run != nil {
		if err := r.Hooks.Run(ctx, workdir, env); err != nil {
			return finish(ClassAgentBuild, fmt.Sprintf("fixture run failed: %v", err), res.CostUSD, transcript)
		}
	}

	// Wait for telemetry to satisfy the assertion within the telemetry budget.
	telemetryTimeout := sc.TelemetryTimeout
	if telemetryTimeout <= 0 {
		telemetryTimeout = DefaultTelemetryTimeout
	}
	// Debugging override: EVAL_TELEMETRY_TIMEOUT (a Go duration) widens the
	// window so a keep-alive run holds the sink, relay, and cluster up long
	// enough to inspect while telemetry may still arrive.
	if override := strings.TrimSpace(os.Getenv("EVAL_TELEMETRY_TIMEOUT")); override != "" {
		if d, err := time.ParseDuration(override); err == nil && d > 0 {
			telemetryTimeout = d
		}
	}

	class, detail := awaitTelemetry(t, ctx, sink, sc.Assert, telemetryTimeout)
	return finish(class, detail, res.CostUSD, transcript)
}

// awaitTelemetry polls the sink until the scenario's assertion passes, the
// timeout elapses, or ctx is done, and classifies the outcome. OTLP exporters
// batch and flush each signal on its own schedule — the OpenTelemetry Java
// agent, for example, emits logs about a second, spans about five seconds, and
// metrics about a minute after the triggering request — so the asserted signal
// routinely lags the first signal to reach the sink. Checking the assertion
// once when any telemetry first appears therefore races the slower signal;
// polling it across the whole telemetry budget removes that race.
//
// It distinguishes two failure modes: telemetry that never arrives at all is
// ClassAgentTelemetry (the agent produced nothing), while telemetry that
// arrives but never satisfies the assertion within the budget is
// ClassAgentAssert (the agent produced the wrong telemetry). A nil assertion
// passes as soon as any telemetry scoped to the sink's test.id is present.
func awaitTelemetry(t *testing.T, ctx context.Context, sink *otelsink.Sink, assert Assertion, timeout time.Duration) (FailureClass, string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		anyTelemetry := sink.Traces(t).Len() > 0 || sink.Metrics(t).Len() > 0 || sink.Logs(t).Len() > 0
		if anyTelemetry {
			if assert == nil {
				return ClassNone, ""
			}
			if lastErr = assert(t, sink); lastErr == nil {
				return ClassNone, ""
			}
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			if !anyTelemetry {
				return ClassAgentTelemetry, fmt.Sprintf("no telemetry with %s=%s reached the sink within %s", otelsink.TestIDAttribute, sink.TestID(), timeout)
			}
			if assert == nil {
				return ClassNone, ""
			}
			return ClassAgentAssert, fmt.Sprintf("assertion failed: %v", lastErr)
		}
		time.Sleep(telemetryPollInterval)
	}
}

// telemetryFiles lists the signal JSONL files that exist under the sink
// directory, as verdict evidence.
func telemetryFiles(dir string) []string {
	var out []string
	for _, name := range []string{"traces.jsonl", "metrics.jsonl", "logs.jsonl"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// copyDir copies the regular files under src into dst, preserving relative
// layout and file modes.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil // skip sockets, symlinks, and other irregular files
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}
