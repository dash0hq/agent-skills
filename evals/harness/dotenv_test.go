package harness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// chdir switches to dir for the duration of the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// fakeRepo builds a throwaway repo root (a directory with skills/) plus an
// evals/ subdirectory, and returns the root. Tests chdir into evals/ so the
// working-directory walk finds the root the way a real `go test ./scenarios`
// invocation does, and confirms the loader reads the repo-root .env from there.
func fakeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "skills"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "evals"), 0o755))
	return root
}

func TestLoadDotEnvSetsUnsetVariables(t *testing.T) {
	root := fakeRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".env"),
		[]byte("# local config\nANTHROPIC_API_KEY=sk-from-file\nEVAL_SCENARIOS=instr-go-http # inline comment\n"), 0o644))
	chdir(t, filepath.Join(root, "evals"))

	require.NoError(t, os.Unsetenv("ANTHROPIC_API_KEY"))
	require.NoError(t, os.Unsetenv("EVAL_SCENARIOS"))

	loaded, err := LoadDotEnv()
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"ANTHROPIC_API_KEY", "EVAL_SCENARIOS"}, loaded)
	require.Equal(t, "sk-from-file", os.Getenv("ANTHROPIC_API_KEY"))
	// Inline comment stripped, matching shell `source` semantics.
	require.Equal(t, "instr-go-http", os.Getenv("EVAL_SCENARIOS"))
}

func TestLoadDotEnvNeverOverwritesEnvironment(t *testing.T) {
	root := fakeRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".env"),
		[]byte("ANTHROPIC_API_KEY=sk-from-file\n"), 0o644))
	chdir(t, filepath.Join(root, "evals"))

	// A value already in the environment (as on the command line) wins.
	t.Setenv("ANTHROPIC_API_KEY", "sk-from-env")

	loaded, err := LoadDotEnv()
	require.NoError(t, err)
	require.NotContains(t, loaded, "ANTHROPIC_API_KEY")
	require.Equal(t, "sk-from-env", os.Getenv("ANTHROPIC_API_KEY"))
}

func TestLoadDotEnvMissingFileIsNotAnError(t *testing.T) {
	root := fakeRepo(t)
	chdir(t, filepath.Join(root, "evals"))

	loaded, err := LoadDotEnv()
	require.NoError(t, err)
	require.Empty(t, loaded)
}
