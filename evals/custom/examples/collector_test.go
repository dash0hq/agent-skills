package examples

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func parseYAML(t *testing.T, content string) map[string]any {
	t.Helper()
	var node map[string]any
	if err := yaml.Unmarshal([]byte(content), &node); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return node
}

func TestScaffoldProcessorFragment(t *testing.T) {
	node := parseYAML(t, "processors:\n  batch: {}\n")
	if !ScaffoldCollectorConfig(node) {
		t.Fatalf("expected scaffolding to modify the fragment")
	}
	pipelines := collectorPipelines(node)
	if len(pipelines) != 1 {
		t.Fatalf("got %d pipelines, want 1: %v", len(pipelines), pipelines)
	}
	pipeline, ok := pipelines["traces"].(map[string]any)
	if !ok {
		t.Fatalf("no traces pipeline: %v", pipelines)
	}
	if got := pipeline["processors"].([]any); len(got) != 1 || got[0] != "batch" {
		t.Errorf("pipeline processors = %v, want [batch]", got)
	}
	// The stub otlp receiver and debug exporter are defined.
	if _, ok := section(node, "receivers")["otlp"]; !ok {
		t.Errorf("stub otlp receiver not defined")
	}
	if _, ok := section(node, "exporters")["debug"]; !ok {
		t.Errorf("stub debug exporter not defined")
	}
	if missing := missingComponents(node); len(missing) != 0 {
		t.Errorf("scaffolded config still misses components: %v", missing)
	}
}

func TestScaffoldTailSamplingPicksTraces(t *testing.T) {
	node := parseYAML(t, `
processors:
  tail_sampling:
    decision_wait: 30s
    policies:
      - name: errors
        type: status_code
        status_code:
          status_codes: [ERROR]
`)
	ScaffoldCollectorConfig(node)
	pipelines := collectorPipelines(node)
	if _, ok := pipelines["traces"]; !ok {
		t.Fatalf("tail_sampling fragment did not produce a traces pipeline: %v", pipelines)
	}
	if _, ok := pipelines["metrics"]; ok {
		t.Errorf("tail_sampling fragment must not produce a metrics pipeline: %v", pipelines)
	}
}

func TestScaffoldConnectorPairedPipelines(t *testing.T) {
	node := parseYAML(t, `
connectors:
  signaltometrics:
    spans:
      - name: http.server.request.duration
        description: "Duration of HTTP server requests."
        unit: s
        exponential_histogram:
          max_size: 160
          value: Seconds(end_time - start_time)
          count: "1"
`)
	ScaffoldCollectorConfig(node)
	pipelines := collectorPipelines(node)

	source, ok := pipelines["traces"].(map[string]any)
	if !ok {
		t.Fatalf("no traces source pipeline: %v", pipelines)
	}
	if exporters := source["exporters"].([]any); !containsAny(exporters, "signaltometrics") {
		t.Errorf("source pipeline does not export to the connector: %v", exporters)
	}

	var target map[string]any
	for name, pipeline := range pipelines {
		if strings.HasPrefix(name, "metrics/") {
			target = pipeline.(map[string]any)
		}
	}
	if target == nil {
		t.Fatalf("no metrics target pipeline: %v", pipelines)
	}
	if receivers := target["receivers"].([]any); !containsAny(receivers, "signaltometrics") {
		t.Errorf("target pipeline does not receive from the connector: %v", receivers)
	}
}

func TestScaffoldStubsReferencedConnector(t *testing.T) {
	// A service-only fragment referencing a known connector type defines it
	// under connectors, not receivers or exporters.
	node := parseYAML(t, `
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [signaltometrics, otlp]
    metrics/red:
      receivers: [signaltometrics]
      exporters: [otlp]
`)
	ScaffoldCollectorConfig(node)
	if _, ok := section(node, "connectors")["signaltometrics"]; !ok {
		t.Errorf("signaltometrics not defined as a connector: %v", node)
	}
	if _, ok := section(node, "receivers")["signaltometrics"]; ok {
		t.Errorf("signaltometrics wrongly defined as a receiver")
	}
	if missing := missingComponents(node); len(missing) != 0 {
		t.Errorf("still missing: %v", missing)
	}
}

func TestScaffoldExtensionsJoinService(t *testing.T) {
	node := parseYAML(t, `
extensions:
  health_check:
    endpoint: 0.0.0.0:13133

service:
  extensions: [health_check]
`)
	ScaffoldCollectorConfig(node)
	if pipelines := collectorPipelines(node); len(pipelines) == 0 {
		t.Fatalf("extensions-only fragment needs a generated pipeline")
	}
}

func TestScaffoldNoChangeForCompleteConfig(t *testing.T) {
	node := parseYAML(t, `
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
exporters:
  debug: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`)
	if ScaffoldCollectorConfig(node) {
		t.Errorf("complete config must not be modified")
	}
}

func containsAny(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestSubstitutePlaceholders(t *testing.T) {
	content := `exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer <AUTH_TOKEN>"
processors:
  resource:
    attributes:
      - key: k8s.cluster.name
        value: "<CLUSTER_NAME>"
        action: upsert
`
	substituted, substitutions := SubstitutePlaceholders(content)
	if strings.Contains(substituted, "<OTLP_ENDPOINT>") {
		t.Errorf("endpoint placeholder not substituted")
	}
	if !strings.Contains(substituted, "endpoint: https://eval-dummy.invalid:4317") {
		t.Errorf("endpoint placeholder not endpoint-shaped:\n%s", substituted)
	}
	if !strings.Contains(substituted, "eval-dummy-cluster-name") {
		t.Errorf("opaque placeholder missing:\n%s", substituted)
	}
	if len(substitutions) != 3 {
		t.Errorf("got %d substitutions, want 3: %v", len(substitutions), substitutions)
	}
}

func TestDummyEnvDiscoversReferences(t *testing.T) {
	env := dummyEnv("endpoint: ${env:MY_ENDPOINT}\ntoken: ${env:DASH0_AUTH_TOKEN}\n")
	joined := strings.Join(env, "\n")
	for _, want := range []string{"DASH0_AUTH_TOKEN=", "DASH0_TOKEN=", "MY_ENDPOINT="} {
		if !strings.Contains(joined, want) {
			t.Errorf("dummy env missing %s: %v", want, env)
		}
	}
}

func TestRetargetFileStorageDirectory(t *testing.T) {
	node := parseYAML(t, `
extensions:
  file_storage:
    directory: /var/lib/otelcol/queue
`)
	substitutions := retargetHostPaths(node)
	if len(substitutions) != 1 {
		t.Fatalf("got %d substitutions, want 1: %v", len(substitutions), substitutions)
	}
	directory := section(node, "extensions")["file_storage"].(map[string]any)["directory"].(string)
	if directory == "/var/lib/otelcol/queue" {
		t.Errorf("directory not retargeted")
	}
}
