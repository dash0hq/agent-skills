package scenarios

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dash0hq/agent-skills/evals/custom/internal/testutil"
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
				filepath.Join(root, "evals", "custom", "fixtures", fixture))
			require.NoError(t, err, "docker build failed:\n%s", out)
			t.Cleanup(func() { _, _ = docker(context.Background(), "rmi", "-f", tag) })
		})
	}
}

// TestClassicBuilderCompatibilityGate verifies the pre-build gate that
// reproduces the issue #120 regression deterministically: an agent-written
// Dockerfile using the legacy space-separated ENV form with a multi-token
// value must fail the attempt even though the harness's BuildKit-backed
// `docker build` would accept it. The test is hermetic (no Docker needed).
func TestClassicBuilderCompatibilityGate(t *testing.T) {
	writeWorkspace := func(t *testing.T, dockerfile string) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0o644))
		return dir
	}

	t.Run("quoted name=value form passes", func(t *testing.T) {
		dir := writeWorkspace(t, "FROM node:22-alpine\n"+
			"ENV NODE_OPTIONS=\"--require @opentelemetry/auto-instrumentations-node/register\"\n")
		require.NoError(t, checkClassicBuilderCompatibility(dir))
	})

	t.Run("legacy multi-token ENV form fails with file and line", func(t *testing.T) {
		dir := writeWorkspace(t, "FROM node:22-alpine\n"+
			"ENV NODE_OPTIONS --require @opentelemetry/auto-instrumentations-node/register\n")
		err := checkClassicBuilderCompatibility(dir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Dockerfile:2")
		require.Contains(t, err.Error(), "classic")
	})

	t.Run("deprecated two-token legacy form still builds and passes the gate", func(t *testing.T) {
		dir := writeWorkspace(t, "FROM node:22-alpine\nENV NODE_ENV production\n")
		require.NoError(t, checkClassicBuilderCompatibility(dir))
	})

	t.Run("Dockerfiles under node_modules are ignored", func(t *testing.T) {
		dir := writeWorkspace(t, "FROM node:22-alpine\nENV NODE_ENV=production\n")
		nested := filepath.Join(dir, "node_modules", "some-pkg")
		require.NoError(t, os.MkdirAll(nested, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(nested, "Dockerfile"),
			[]byte("FROM scratch\nENV A --broken value\n"), 0o644))
		require.NoError(t, checkClassicBuilderCompatibility(dir))
	})
}

// TestInfraImagesBuild verifies the relay and helper images build from the
// evals/custom/ module; it requires a working Docker daemon and skips otherwise.
func TestInfraImagesBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker image builds in -short mode")
	}
	if !DockerAvailable() {
		t.Skip("skipping: docker CLI or daemon not available")
	}
	fix := NewDockerFixture(t, filepath.Join(testutil.RepoRoot(t), "evals", "custom"))
	require.NoError(t, fix.ensureInfraImages(context.Background()))
}
