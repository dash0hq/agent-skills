package scenarios

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dash0hq/agent-skills/evals/custom/internal/testutil"
)

// fixtureSourceFiles lists, per fixture, the files that together carry the
// service's contract surface (endpoint, environment variables, synthetic
// data) for the contract lint below. Ports supplied through the container
// command line list the Dockerfile as a source.
var fixtureSourceFiles = map[string][]string{
	"evals/custom/fixtures/browser-service": {"server.js", "static/app.js"},
	"evals/custom/fixtures/dotnet-service":  {"Program.cs"},
	"evals/custom/fixtures/go-service":      {"main.go"},
	"evals/custom/fixtures/java-service":    {"src/main/java/checkout/CheckoutServer.java", "src/main/resources/application.properties"},
	"evals/custom/fixtures/nextjs-service":  {"app/checkout/route.js", "Dockerfile"},
	"evals/custom/fixtures/nodejs-service":  {"server.js"},
	"evals/custom/fixtures/php-service":     {"index.php", "Dockerfile"},
	"evals/custom/fixtures/python-service":  {"app.py"},
	"evals/custom/fixtures/ruby-service":    {"config.ru", "Dockerfile"},
	"evals/custom/fixtures/scala-service":   {"src/main/scala/checkout/CheckoutServer.scala"},
}

// nonServiceFixtures lists fixture directories that are not HTTP services and
// are therefore outside the service contract lint: each entry names why. A
// directory must appear in exactly one of fixtureSourceFiles or this map, so
// nothing lands unnoticed.
var nonServiceFixtures = map[string]string{
	"evals/custom/fixtures/collector-workspace": "Collector config workspace the agent edits (U5); no service contract",
	"evals/custom/fixtures/k8s":                 "Kubernetes manifests and workspaces the agent edits (U6); no service contract",
}

// browserContractExceptions documents how the browser fixture deviates from
// the "exactly 1 inbound endpoint" contract (see the "Browser fixture
// exception" section of evals/custom/fixtures/README.md). The full contract lint
// still applies to it; these are additions, not relaxations, and the
// dedicated subtest below fails if any documented exception disappears from
// the fixture, so the list cannot go stale.
//
//   - GET /checkout-data: the same-origin endpoint the page's JavaScript
//     fetches on load; it reuses the /checkout handler, so the DOWNSTREAM_URL
//     call still happens per the contract.
//   - GET /env.js: exposes the server's EVAL_-prefixed runtime configuration
//     to the page as window.__EVAL_ENV__, because browsers cannot read
//     process environment variables.
//   - GET / and the static assets: the page (static/index.html plus
//     static/app.js) whose in-browser activity is the scenario's telemetry
//     source; the server process itself stays out of the assertions.
var browserContractExceptions = map[string][]string{
	"server.js": {
		"/checkout-data",
		"/env.js",
		"window.__EVAL_ENV__",
		"EVAL_",
	},
	"static/app.js": {
		"fetch('/checkout-data')",
	},
	"static/index.html": {
		"/env.js",
		"/app.js",
	},
}

// The fixture contract lint (see evals/custom/fixtures/README.md): every fixture
// exposes GET /checkout, reads PORT and DOWNSTREAM_URL, ships a Dockerfile,
// contains no OpenTelemetry dependencies anywhere in its tree, and uses
// obviously synthetic data.
func TestFixturesFollowTheContract(t *testing.T) {
	root := testutil.RepoRoot(t)

	// Every fixture directory on disk must be linted: a new fixture cannot
	// land without a fixtureSourceFiles entry.
	entries, err := os.ReadDir(filepath.Join(root, "evals", "custom", "fixtures"))
	require.NoError(t, err)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := "evals/custom/fixtures/" + entry.Name()
		_, service := fixtureSourceFiles[path]
		_, nonService := nonServiceFixtures[path]
		require.True(t, service || nonService, "fixture directory %s has neither a fixtureSourceFiles nor a nonServiceFixtures entry; add one so the lint covers it", entry.Name())
		require.False(t, service && nonService, "fixture directory %s appears in both fixtureSourceFiles and nonServiceFixtures", entry.Name())
	}

	for fixture, sources := range fixtureSourceFiles {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			dir := filepath.Join(root, filepath.FromSlash(fixture))

			_, err := os.Stat(filepath.Join(dir, "Dockerfile"))
			require.NoError(t, err, "fixture must ship a Dockerfile")

			var combined strings.Builder
			for _, src := range sources {
				content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(src)))
				require.NoError(t, err)
				combined.Write(content)
			}
			code := combined.String()

			require.Contains(t, code, "/checkout", "inbound endpoint")
			require.Contains(t, code, "PORT", "listen port from the environment")
			require.Contains(t, code, "DOWNSTREAM_URL", "outbound call target from the environment")
			require.Contains(t, code, "user@example.test", "synthetic customer data")
			require.Contains(t, code, "TEST-0001", "synthetic order identifier")

			// The fixture is the code the agent instruments: no file in its
			// tree may carry an OpenTelemetry dependency of any kind.
			err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				content, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(dir, p)
				if err != nil {
					return err
				}
				lower := strings.ToLower(string(content))
				require.NotContains(t, lower, "opentelemetry",
					"%s must not reference OpenTelemetry", rel)
				require.NotContains(t, lower, "otel",
					"%s must not reference OTel", rel)
				return nil
			})
			require.NoError(t, err)
		})
	}
}

// The browser fixture's documented contract exceptions must all exist in the
// fixture: a stale browserContractExceptions entry (or a fixture edit that
// silently drops the exception surface the scenario prompt relies on) fails
// here.
func TestBrowserFixtureExceptionSurface(t *testing.T) {
	root := testutil.RepoRoot(t)
	dir := filepath.Join(root, "evals", "custom", "fixtures", "browser-service")
	for file, markers := range browserContractExceptions {
		content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file)))
		require.NoError(t, err)
		for _, marker := range markers {
			require.Contains(t, string(content), marker,
				"documented browser contract exception %q missing from %s", marker, file)
		}
	}
}
