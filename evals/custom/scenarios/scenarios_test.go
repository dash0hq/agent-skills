package scenarios

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dash0hq/agent-skills/evals/custom/harness"

	"github.com/dash0hq/agent-skills/evals/custom/internal/testutil"
)

// allScenarioIDs is every scenario ID (order-insensitive): the U3 core, the
// U4 SDK registrations, the U5 Collector, OTTL, and semconv registrations, and
// the U6 Kubernetes registrations.
var allScenarioIDs = []string{
	GoHTTPID, GoLogsID, NodeHTTPID,
	PythonHTTPID, JavaHTTPID, RubyHTTPID, PHPHTTPID, ScalaHTTPID,
	DotnetHTTPID, DotnetNuGetID, DotnetEnrichmentID,
	NextJSHTTPID, BrowserHTTPID,
	CollectorPipelineHardeningID, OTTLRedactionID, OTTLEnrichmentID, SemconvAttributesID,
	K8sDownwardAPIID, K8sOTelOperatorID, K8sHelmChartID, K8sRawManifestsID, K8sDash0OperatorID,
	PythonRetiredInstrumentationUpgradeID,
}

// instrumentationSelectable is every otel-instrumentation scenario reachable
// through path-based selection.
var instrumentationSelectable = []string{
	GoHTTPID, GoLogsID, NodeHTTPID,
	PythonHTTPID, JavaHTTPID, RubyHTTPID, PHPHTTPID, ScalaHTTPID,
	DotnetHTTPID, DotnetNuGetID, DotnetEnrichmentID,
	NextJSHTTPID, BrowserHTTPID,
	PythonRetiredInstrumentationUpgradeID,
}

// The registry completeness test (R17), in both directions: every dedicated
// rule file is covered by a registered scenario or a pendingScenarios entry,
// and no pendingScenarios entry is stale now that the U3 and U4 scenarios
// cover all 10 SDK rule files.
func TestDefaultRegistryValidatesRealTree(t *testing.T) {
	require.NoError(t, Default().Validate(testutil.RepoRoot(t), harness.PendingScenarios()))
}

func TestDefaultRegistration(t *testing.T) {
	reg := Default()
	ids := scenarioIDs(reg.Scenarios())
	require.ElementsMatch(t, allScenarioIDs, ids)

	byID := map[string]harness.Scenario{}
	for _, sc := range reg.Scenarios() {
		byID[sc.ID] = sc
	}

	require.True(t, byID[GoHTTPID].Smoke, "the Go HTTP scenario is the instrumentation smoke scenario")

	// Every selectable scenario asserts telemetry and runs against an
	// existing fixture directory.
	root := testutil.RepoRoot(t)
	for _, id := range instrumentationSelectable {
		sc := byID[id]
		require.NotNil(t, sc.Assert, "assertion of %s", id)
		require.NotEmpty(t, sc.FixturePath, "fixture path of %s", id)
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(sc.FixturePath)))
		require.NoError(t, err, "fixture path of %s", id)
	}
}

func TestSelection(t *testing.T) {
	reg := Default()

	tests := []struct {
		name    string
		changed []string
		want    []string
	}{
		{
			// Covers AE2 for the real registry: the Go SDK rule file
			// selects exactly the scenarios declaring it.
			name:    "go rule file selects the Go scenarios",
			changed: []string{"skills/otel-instrumentation/rules/sdks/go.md"},
			want:    []string{GoHTTPID, GoLogsID},
		},
		{
			name:    "nodejs rule file selects the Node.js scenario",
			changed: []string{"skills/otel-instrumentation/rules/sdks/nodejs.md"},
			want:    []string{NodeHTTPID},
		},
		{
			name:    "dotnet rule file selects the 3 .NET scenarios",
			changed: []string{"skills/otel-instrumentation/rules/sdks/dotnet.md"},
			want:    []string{DotnetHTTPID, DotnetNuGetID, DotnetEnrichmentID},
		},
		{
			name:    "browser rule file selects the browser scenario",
			changed: []string{"skills/otel-instrumentation/rules/sdks/browser.md"},
			want:    []string{BrowserHTTPID},
		},
		{
			name:    "shared logs rule file selects all instrumentation scenarios",
			changed: []string{"skills/otel-instrumentation/rules/logs.md"},
			want:    instrumentationSelectable,
		},
		{
			// spans.md is shared: even though the .NET enrichment scenario
			// declares it, the shared classification selects the whole skill.
			name:    "shared spans rule file selects all instrumentation scenarios",
			changed: []string{"skills/otel-instrumentation/rules/spans.md"},
			want:    instrumentationSelectable,
		},
		{
			name:    "go fixture selects the scenarios using it",
			changed: []string{"evals/custom/fixtures/go-service/main.go"},
			want:    []string{GoHTTPID, GoLogsID, SemconvAttributesID},
		},
		{
			name:    "dotnet fixture selects the 3 .NET scenarios",
			changed: []string{"evals/custom/fixtures/dotnet-service/Program.cs"},
			want:    []string{DotnetHTTPID, DotnetNuGetID, DotnetEnrichmentID},
		},
		{
			// Full-matrix triggers select everything, including the
			// RequiresKind Kubernetes scenarios.
			name:    "full-matrix trigger selects every scenario",
			changed: []string{"evals/custom/versions.env"},
			want:    allScenarioIDs,
		},
		{
			name:    "harness change selects one smoke scenario per skill",
			changed: []string{"evals/custom/harness/runner.go"},
			want:    []string{GoHTTPID, CollectorPipelineHardeningID, OTTLRedactionID, SemconvAttributesID},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ElementsMatch(t, tt.want, scenarioIDs(reg.Select(tt.changed)))
		})
	}
}
