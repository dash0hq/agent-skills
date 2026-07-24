package harness

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dash0hq/agent-skills/evals/custom/internal/testutil"
)

func newParser(skill Skill) (*streamParser, *AgentResult) {
	res := &AgentResult{}
	return &streamParser{
		inv: &Invocation{Skill: skill},
		res: res,
	}, res
}

// The init event's slash_commands list is the only signal, under --bare, that
// the plugin exposed the target skill; the parser sets SkillLoaded from it.
func TestStreamParserSkillLoadedFromInit(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"namespaced command", testutil.InitLineWithSkill("otel-instrumentation"), true},
		{"leading-slash command", `{"type":"system","subtype":"init","plugin_errors":[],"slash_commands":["/dash0-agent-skills:otel-instrumentation"]}`, true},
		{"command for a different skill", testutil.InitLineWithSkill("otel-collector"), false},
		{"no slash_commands", testutil.InitLine, false},
		{"bare skill name is not a plugin command", `{"type":"system","subtype":"init","plugin_errors":[],"slash_commands":["otel-instrumentation"]}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, res := newParser(SkillInstrumentation)
			p.feed([]byte(tt.line))
			require.True(t, res.SawInit)
			require.Equal(t, tt.want, res.SkillLoaded)
		})
	}
}

func TestStreamParserInitAndResult(t *testing.T) {
	p, res := newParser(SkillInstrumentation)
	p.feed([]byte(`{"type":"system","subtype":"init","plugin_errors":["skill A failed","skill B failed"]}`))
	p.feed([]byte(`{"type":"result","subtype":"success","is_error":true,"total_cost_usd":1.25}`))
	p.feed([]byte(`not json at all`)) // tolerated

	require.True(t, res.SawInit)
	require.Equal(t, []string{"skill A failed", "skill B failed"}, res.PluginErrors)
	require.True(t, res.SawResult)
	require.True(t, res.IsError)
	require.InDelta(t, 1.25, res.CostUSD, 1e-9)
}

func TestStreamParserOverloadMarkerInStream(t *testing.T) {
	p, res := newParser(SkillInstrumentation)
	p.feed([]byte(`{"type":"result","subtype":"error","is_error":true,"result":"API Error: overloaded_error"}`))
	require.True(t, res.Overloaded)
}

func TestStreamParserOverloadMarkerScopedToResultAndSystem(t *testing.T) {
	t.Run("benign 429 in assistant prose does not classify as overloaded", func(t *testing.T) {
		p, res := newParser(SkillInstrumentation)
		p.feed([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"The endpoint returned status 429 in the logs the user pasted."}]}}`))
		require.False(t, res.Overloaded)
	})
	t.Run("overload marker in a result event classifies as overloaded", func(t *testing.T) {
		p, res := newParser(SkillInstrumentation)
		p.feed([]byte(`{"type":"result","subtype":"error","is_error":true,"result":"API error: 429 rate_limit_error"}`))
		require.True(t, res.Overloaded)
	})
	t.Run("overload marker in a system event classifies as overloaded", func(t *testing.T) {
		p, res := newParser(SkillInstrumentation)
		p.feed([]byte(`{"type":"system","subtype":"warning","message":"overloaded_error from upstream"}`))
		require.True(t, res.Overloaded)
	})
}

func TestOverload429529StderrAnchored(t *testing.T) {
	t.Run("benign prose mentioning 429 does not match", func(t *testing.T) {
		require.False(t, overload429529.MatchString("Instrument the checkout service; it handled 429 requests per second."))
	})
	t.Run("API-error-shaped stderr line matches", func(t *testing.T) {
		require.True(t, overload429529.MatchString("API Error: 429 Too Many Requests"))
		require.True(t, overload429529.MatchString("http status 529 overloaded"))
	})
}

func TestClassifyAgentRun(t *testing.T) {
	good := func() *AgentResult {
		return &AgentResult{SawInit: true, SawResult: true, SkillLoaded: true}
	}

	tests := []struct {
		name   string
		mutate func(*AgentResult)
		want   FailureClass
	}{
		{"clean run", func(r *AgentResult) {}, ClassNone},
		{"plugin errors are infra", func(r *AgentResult) { r.PluginErrors = []string{"boom"} }, ClassInfra},
		{"overload markers are infra", func(r *AgentResult) { r.Overloaded = true }, ClassInfra},
		{"timeout is agent-telemetry", func(r *AgentResult) { r.TimedOut = true; r.SawResult = false; r.ExitErr = errors.New("killed") }, ClassAgentTelemetry},
		{"non-zero exit without result is infra", func(r *AgentResult) { r.SawResult = false; r.ExitErr = errors.New("exit 1") }, ClassInfra},
		{"stream without result event is infra", func(r *AgentResult) { r.SawResult = false }, ClassInfra},
		{"skill not loaded is infra", func(r *AgentResult) { r.SkillLoaded = false }, ClassInfra},
		{"is_error api-shaped result is infra", func(r *AgentResult) { r.IsError = true; r.ResultSubtype = "error_api_overloaded" }, ClassInfra},
		{"is_error auth-shaped result is infra", func(r *AgentResult) { r.IsError = true; r.ResultSubtype = "error_authentication" }, ClassInfra},
		{"is_error max-turns result is agent-telemetry", func(r *AgentResult) { r.IsError = true; r.ResultSubtype = "error_max_turns" }, ClassAgentTelemetry},
		{"is_error execution result is agent-telemetry", func(r *AgentResult) { r.IsError = true; r.ResultSubtype = "error_during_execution" }, ClassAgentTelemetry},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := good()
			tt.mutate(res)
			class, _ := classifyAgentRun(res)
			require.Equal(t, tt.want, class)
		})
	}

	t.Run("nil result is infra", func(t *testing.T) {
		class, _ := classifyAgentRun(nil)
		require.Equal(t, ClassInfra, class)
	})
}

func TestFailureClassAgentAttributable(t *testing.T) {
	require.True(t, ClassAgentBuild.AgentAttributable())
	require.True(t, ClassAgentTelemetry.AgentAttributable())
	require.True(t, ClassAgentAssert.AgentAttributable())
	require.False(t, ClassInfra.AgentAttributable())
	require.False(t, ClassInfraFail.AgentAttributable())
	require.False(t, ClassNone.AgentAttributable())
}
