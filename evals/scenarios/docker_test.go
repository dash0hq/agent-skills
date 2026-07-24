package scenarios

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dash0hq/agent-skills/evals/internal/testutil"
)

// TestFixtureImagesBuild verifies both fixture Dockerfiles build; it requires
// a working Docker daemon and skips otherwise.
func TestFixtureImagesBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker image builds in -short mode")
	}
	if !DockerAvailable() {
		t.Skip("skipping: docker CLI or daemon not available")
	}
	root := testutil.RepoRoot(t)

	for _, fixture := range []string{"go-service", "nodejs-service"} {
		t.Run(fixture, func(t *testing.T) {
			tag := "eval-fixture-" + fixture + ":test"
			out, err := docker(context.Background(), "build", "-t", tag,
				filepath.Join(root, "evals", "fixtures", fixture))
			require.NoError(t, err, "docker build failed:\n%s", out)
			t.Cleanup(func() { _, _ = docker(context.Background(), "rmi", "-f", tag) })
		})
	}
}

// TestInfraImagesBuild verifies the relay and helper images build from the
// evals/ module; it requires a working Docker daemon and skips otherwise.
func TestInfraImagesBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker image builds in -short mode")
	}
	if !DockerAvailable() {
		t.Skip("skipping: docker CLI or daemon not available")
	}
	fix := NewDockerFixture(t, filepath.Join(testutil.RepoRoot(t), "evals"))
	require.NoError(t, fix.ensureInfraImages(context.Background()))
}
