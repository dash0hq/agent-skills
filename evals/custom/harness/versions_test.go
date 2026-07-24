package harness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadVersionsFromCommittedFile(t *testing.T) {
	v, err := LoadVersions(filepath.Join("..", "versions.env"))
	require.NoError(t, err)
	require.Equal(t, "2.1.179", v.ClaudeCodeVersion)
	require.Equal(t, "claude-opus-4-8", v.EvalModel)
	require.Equal(t, "0.156.0", v.OtelcolContribVersion)
	require.Equal(t, "1.44.0", v.OtelGoCoreVersion)
	require.Equal(t, "0.20.0", v.OtelGoLogVersion)
}

func TestLoadVersionsRejectsMissingOtelGoPins(t *testing.T) {
	// The required tool pins are present, but the OpenTelemetry Go pins are
	// absent, so the loader names the missing OTEL_GO_* keys.
	p := filepath.Join(t.TempDir(), "versions.env")
	require.NoError(t, os.WriteFile(p, []byte("CLAUDE_CODE_VERSION=1.0.0\nEVAL_MODEL=m\nOTELCOL_CONTRIB_VERSION=0.156.0\n"), 0o644))
	_, err := LoadVersions(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "OTEL_GO_CORE_VERSION")
}

func TestLoadVersionsRejectsMissingPins(t *testing.T) {
	p := filepath.Join(t.TempDir(), "versions.env")
	require.NoError(t, os.WriteFile(p, []byte("CLAUDE_CODE_VERSION=1.0.0\n"), 0o644))
	_, err := LoadVersions(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "EVAL_MODEL")
}

func TestLoadVersionsRejectsEmptyValueAfterEquals(t *testing.T) {
	// A required pin present as KEY= (empty value after the equals) still
	// counts as unpinned, and the error names the missing fields.
	p := filepath.Join(t.TempDir(), "versions.env")
	require.NoError(t, os.WriteFile(p, []byte("CLAUDE_CODE_VERSION=1.0.0\nEVAL_MODEL=\nOTELCOL_CONTRIB_VERSION=0.156.0\n"), 0o644))
	_, err := LoadVersions(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "EVAL_MODEL")
}

func TestLoadVersionsRejectsMalformedLine(t *testing.T) {
	p := filepath.Join(t.TempDir(), "versions.env")
	require.NoError(t, os.WriteFile(p, []byte("# comment\n\nNOT-A-PAIR\n"), 0o644))
	_, err := LoadVersions(p)
	require.Error(t, err)
	require.Contains(t, err.Error(), "NOT-A-PAIR")
}
