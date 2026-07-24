package harness

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultAgentBinary is the CLI binary name used when Agent.Binary is empty.
// The pinned version comes from evals/versions.env (CLAUDE_CODE_VERSION); CI
// installs exactly that version under this name.
const DefaultAgentBinary = "claude"

// PluginName is the plugin name under which this repository's skills are
// exposed to the CLI. Under --bare the CLI publishes plugin skills only as
// slash commands named "<PluginName>:<skill>", so the harness both injects
// the command to load the skill and looks for it in the init event's
// slash_commands list.
const PluginName = "dash0-agent-skills"

// skillCommand returns the slash command that loads the given skill, for
// example "/dash0-agent-skills:otel-collector".
func skillCommand(s Skill) string {
	return "/" + PluginName + ":" + string(s)
}

// Agent invokes the Claude Code CLI headless. The binary path is injectable so
// tests substitute stub scripts; tests never call the real CLI or any network
// API.
type Agent struct {
	// Binary is the path to the CLI binary. Empty means DefaultAgentBinary
	// resolved via PATH.
	Binary string
	// PluginDir is passed to --plugin-dir and must point at the repository
	// root, whose .claude-plugin/plugin.json exposes the skills.
	PluginDir string
	// Model is passed to --model (pinned via versions.env, EVAL_MODEL).
	Model string
}

// Invocation describes one headless agent run.
type Invocation struct {
	// Prompt is the task passed to -p.
	Prompt string
	// MaxTurns caps conversation turns; zero means DefaultMaxTurns.
	MaxTurns int
	// WorkDir is the working directory (the fixture workspace copy).
	WorkDir string
	// Env is merged over the parent process environment.
	Env map[string]string
	// Skill is the skill whose slash command the harness injects and whose
	// presence in the init event's slash_commands list it verifies.
	Skill Skill
	// TranscriptPath is where the raw stream-json transcript is written.
	TranscriptPath string
}

// AgentResult is the parsed outcome of one headless agent run.
type AgentResult struct {
	// SawInit is true once the system/init event was observed.
	SawInit bool
	// PluginErrors carries the plugin_errors entries from the system/init
	// event; non-empty means the plugin (and so the skills) failed to load
	// and the attempt is classified ClassInfra before any scenario work.
	PluginErrors []string
	// SkillLoaded is true when the system/init event's slash_commands list
	// includes the target skill's command ("<PluginName>:<skill>"), which
	// means the CLI resolved the plugin and exposed the skill; a false value
	// is a harness/plugin failure classified ClassInfra.
	SkillLoaded bool
	// SawResult is true once the final result event was observed.
	SawResult bool
	// IsError carries the is_error flag of the result event.
	IsError bool
	// ResultSubtype carries the subtype of the result event (for example
	// "success", "error_max_turns", or "error_during_execution").
	ResultSubtype string
	// CostUSD carries total_cost_usd from the result event.
	CostUSD float64
	// Overloaded is true when API overload markers (429/529, rate limit,
	// overloaded errors) appeared in the stream or on stderr.
	Overloaded bool
	// TimedOut is true when the run was killed at the wall-clock budget.
	TimedOut bool
	// ExitErr is the process exit error, if any.
	ExitErr error
	// TranscriptPath is where the transcript was written.
	TranscriptPath string
}

// Invoke runs the CLI once and parses its stream-json output. The returned
// error reports harness-level problems only (process could not start,
// transcript not writable); everything the agent did or failed to do is on
// the AgentResult and classified by classifyAgentRun.
func (a *Agent) Invoke(ctx context.Context, inv Invocation) (*AgentResult, error) {
	binary := a.Binary
	if binary == "" {
		binary = DefaultAgentBinary
	}
	maxTurns := inv.MaxTurns
	if maxTurns <= 0 {
		maxTurns = DefaultMaxTurns
	}

	// Under --bare the CLI exposes plugin skills only as slash commands, not
	// as a Skill tool a headless -p agent could self-invoke; prepend the
	// skill's slash command to the prompt so the CLI loads the skill.
	prompt := inv.Prompt
	if inv.Skill != "" {
		prompt = skillCommand(inv.Skill) + " " + inv.Prompt
	}

	args := []string{
		"--bare",
		"--plugin-dir", a.PluginDir,
		"-p", prompt,
		"--output-format", "stream-json",
		// --verbose is required by current CLI releases when combining
		// -p (print mode) with --output-format stream-json; without it
		// the CLI rejects the flag combination.
		"--verbose",
		// The agent must edit files and run build tooling autonomously. In
		// headless -p mode there is no interactive prompt, so without this the
		// CLI denies Edit/Write/Bash on a clean install and the agent makes no
		// changes at all — every scenario then fails with no telemetry. (It
		// only "worked" on a developer machine whose ~/.claude/settings.json
		// already allowed those tools.) The agent runs against a throwaway
		// workspace copy with egress restricted to the relay (R21), so
		// bypassing the prompt here is safe and required for reproducibility.
		"--dangerously-skip-permissions",
		"--max-turns", strconv.Itoa(maxTurns),
		"--model", a.Model,
	}

	transcript, err := os.Create(inv.TranscriptPath)
	if err != nil {
		return nil, fmt.Errorf("agent: create transcript: %w", err)
	}
	defer transcript.Close()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = inv.WorkDir
	cmd.Env = mergedEnv(inv.Env)
	// Give abandoned grandchildren holding the stdout pipe a short grace
	// period before Wait force-returns, so a killed stub cannot hang the
	// harness.
	cmd.WaitDelay = 2 * time.Second

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("agent: stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	res := &AgentResult{TranscriptPath: inv.TranscriptPath}
	parser := &streamParser{inv: &inv, res: res}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("agent: start %s: %w", binary, err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if _, err := transcript.Write(append(append([]byte(nil), line...), '\n')); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("agent: write transcript: %w", err)
		}
		parser.feed(line)
	}
	// Scanner errors (including os/exec.ErrWaitDelay pipe closure) surface
	// through the process exit below; the transcript keeps whatever arrived.

	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		res.TimedOut = true
	}
	res.ExitErr = waitErr

	if stderr.Len() > 0 {
		// Stderr is evidence too; it sits next to the transcript.
		_ = os.WriteFile(inv.TranscriptPath+".stderr", stderr.Bytes(), 0o644)
		if containsOverloadMarker(stderr.String()) || overload429529.MatchString(stderr.String()) {
			res.Overloaded = true
		}
	}

	return res, nil
}

// classifyAgentRun maps the parsed agent run to a failure class, or ClassNone
// when the run itself gives no reason to fail (build, telemetry, and
// assertion classification happen later in the runner).
func classifyAgentRun(res *AgentResult) (FailureClass, string) {
	switch {
	case res == nil:
		return ClassInfra, "agent produced no result"
	case len(res.PluginErrors) > 0:
		return ClassInfra, fmt.Sprintf("plugin_errors in system/init event: %s", strings.Join(res.PluginErrors, "; "))
	case res.Overloaded:
		return ClassInfra, "API overload marker (429/529/rate limit) in agent output"
	case res.TimedOut:
		return ClassAgentTelemetry, "agent killed at wall-clock timeout before telemetry could be produced"
	case res.ExitErr != nil && !res.SawResult:
		return ClassInfra, fmt.Sprintf("CLI exited without a result event: %v", res.ExitErr)
	case !res.SawResult:
		return ClassInfra, "CLI stream ended without a result event"
	case res.IsError:
		// The CLI reported the run itself as an error. API, auth, and overload
		// shapes are infrastructure; max-turns and execution errors are the
		// agent's own failure to finish, attributed like the timeout path
		// (agent-telemetry, since no telemetry could be produced).
		if isInfraResultSubtype(res.ResultSubtype) {
			return ClassInfra, fmt.Sprintf("CLI reported an infrastructure error result (subtype %q)", res.ResultSubtype)
		}
		return ClassAgentTelemetry, fmt.Sprintf("CLI reported an error result before telemetry could be produced (subtype %q)", res.ResultSubtype)
	case !res.SkillLoaded:
		return ClassInfra, "the plugin did not expose the target skill command in system/init slash_commands; the harness could not load the skill (check --plugin-dir and .claude-plugin/plugin.json)"
	default:
		return ClassNone, ""
	}
}

// streamParser incrementally consumes stream-json lines and fills an
// AgentResult.
type streamParser struct {
	inv *Invocation
	res *AgentResult
}

func (p *streamParser) feed(line []byte) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return
	}
	var ev map[string]any
	if err := json.Unmarshal(trimmed, &ev); err != nil {
		return // tolerate non-JSON noise on stdout
	}
	switch ev["type"] {
	case "system":
		// System events carry control-plane text; scan them for overload
		// markers, but not assistant prose or tool results, which may quote
		// "429" benignly.
		if containsOverloadMarker(string(trimmed)) {
			p.res.Overloaded = true
		}
		if ev["subtype"] == "init" {
			p.res.SawInit = true
			if errsAny, ok := ev["plugin_errors"].([]any); ok {
				for _, e := range errsAny {
					p.res.PluginErrors = append(p.res.PluginErrors, fmt.Sprint(e))
				}
			}
			// Under --bare the skill appears only as a slash command; treat
			// its presence in slash_commands as proof the skill loaded.
			if cmdsAny, ok := ev["slash_commands"].([]any); ok {
				want := PluginName + ":" + string(p.inv.Skill)
				for _, c := range cmdsAny {
					cmd, ok := c.(string)
					if !ok {
						continue
					}
					if strings.TrimPrefix(cmd, "/") == want {
						p.res.SkillLoaded = true
					}
				}
			}
		}
	case "result":
		// The result event reports the run's own outcome; an overload marker
		// here is a genuine API failure, unlike a mention in assistant prose.
		if containsOverloadMarker(string(trimmed)) {
			p.res.Overloaded = true
		}
		p.res.SawResult = true
		if cost, ok := ev["total_cost_usd"].(float64); ok {
			p.res.CostUSD = cost
		}
		if isErr, ok := ev["is_error"].(bool); ok {
			p.res.IsError = isErr
		}
		if subtype, ok := ev["subtype"].(string); ok {
			p.res.ResultSubtype = subtype
		}
	}
}

// overloadMarkers are substrings whose presence in agent output classifies the
// attempt as an API-side infra failure.
var overloadMarkers = []string{
	"overloaded_error",
	"rate_limit_error",
	"api error: 429",
	"api error: 529",
	"status 429",
	"status 529",
	"http 429",
	"http 529",
}

// overload429529 catches 429/529 status codes on API-error-shaped stderr
// lines, where the CLI prints human-readable API errors. It requires an error
// or status indicator alongside the code so a bare "429" mentioned in prose
// does not classify a run as an infra failure.
var overload429529 = regexp.MustCompile(`(?i)\b(api error|error|status(?: code)?|http)\b[^\n]*\b(429|529)\b`)

// infraResultSubtypeMarkers are substrings of a result subtype that mark the
// error as infrastructure-side (API, authentication, overload, or billing)
// rather than the agent's own failure to finish.
var infraResultSubtypeMarkers = []string{
	"api",
	"auth",
	"overload",
	"rate_limit",
	"ratelimit",
	"credit",
	"quota",
	"billing",
}

// isInfraResultSubtype reports whether an is_error result subtype names an
// infrastructure-side failure. Unknown subtypes are treated as
// agent-attributable, matching the timeout path.
func isInfraResultSubtype(subtype string) bool {
	lower := strings.ToLower(subtype)
	for _, m := range infraResultSubtypeMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func containsOverloadMarker(s string) bool {
	lower := strings.ToLower(s)
	for _, m := range overloadMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// mergedEnv layers extra over the parent environment.
func mergedEnv(extra map[string]string) []string {
	if len(extra) == 0 {
		return os.Environ()
	}
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}
