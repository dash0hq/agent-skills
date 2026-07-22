// Package testutil holds test helpers shared by the harness and scenarios
// test packages: locating the repository root, writing stub agent scripts,
// and building the stub stream-json transcripts those scripts print. It is a
// normal (non-_test) package so both test packages can import it; it must
// never call the real Claude CLI or any API.
package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// RepoRoot returns the absolute path of the repository checkout, resolved
// relative to the caller's package directory (two levels up from a package
// under evals/). It fails the test when the resolved root lacks a skills/
// directory.
func RepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	if _, err := os.Stat(filepath.Join(root, "skills")); err != nil {
		t.Fatalf("repo root %s has no skills/ directory: %v", root, err)
	}
	return root
}

// WriteStub writes an executable stub agent script and returns its path.
// Tests never call the real CLI or any API; the stub prints a canned
// transcript instead.
func WriteStub(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "claude-stub.sh")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o755))
	return p
}

// PluginName is the plugin name the harness injects and detects; it must match
// harness.PluginName. Duplicated here to avoid a test-only import cycle.
const PluginName = "dash0-agent-skills"

// Transcript lines emitted by stub agents. InitLine omits slash_commands, so
// the harness treats the skill as not loaded; use InitLineWithSkill for a stub
// that should reach a passing or agent-attributable verdict.
const (
	InitLine   = `{"type":"system","subtype":"init","plugin_errors":[]}`
	ResultLine = `{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.05}`
)

// InitLineWithSkill builds a system/init line whose slash_commands list
// includes the given skill's command, matching the CLI behaviour under --bare
// where plugin skills surface only as slash commands.
func InitLineWithSkill(skill string) string {
	return fmt.Sprintf(`{"type":"system","subtype":"init","plugin_errors":[],"slash_commands":["%s:%s"]}`, PluginName, skill)
}

// ReadToolLine builds a stream-json assistant event recording a Read of the
// given file path.
func ReadToolLine(filePath string) string {
	return fmt.Sprintf(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"%s"}}]}}`, filePath)
}

// CatBody wraps transcript lines in a heredoc-printing shell fragment.
func CatBody(lines ...string) string {
	return "cat <<'TRANSCRIPT'\n" + strings.Join(lines, "\n") + "\nTRANSCRIPT\n"
}

// GoodStubBody is a stub agent that loads the plugin cleanly, exposes the given
// skill's slash command in its init event, and finishes with a result event.
// It is the shared passing stub; skillFile records a rule-file Read that
// documents which skill the stub stands in for.
func GoodStubBody(skill, skillFile string) string {
	return "#!/bin/sh\n" + CatBody(InitLineWithSkill(skill), ReadToolLine(skillFile), ResultLine)
}

// NoSkillStubBody is a stub agent whose init event omits the skill's slash
// command, so the harness cannot load the skill and classifies the run infra.
func NoSkillStubBody() string {
	return "#!/bin/sh\n" + CatBody(InitLine, ResultLine)
}
