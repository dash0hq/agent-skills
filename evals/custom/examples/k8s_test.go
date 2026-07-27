package examples

import (
	"strings"
	"testing"
)

func TestParseK8sDocumentCRStructuredConfig(t *testing.T) {
	embedded, err := ParseK8sDocument(`
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: otel
spec:
  mode: deployment
  config:
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
    service:
      pipelines:
        traces:
          receivers: [otlp]
          exporters: [debug]
`)
	if err != nil {
		t.Fatalf("ParseK8sDocument: %v", err)
	}
	if len(embedded) != 1 {
		t.Fatalf("got %d embedded configs, want 1", len(embedded))
	}
	if embedded[0].Source != "OpenTelemetryCollector spec.config" {
		t.Errorf("source = %q", embedded[0].Source)
	}
	if !strings.Contains(embedded[0].Content, "otlp") {
		t.Errorf("embedded content = %q", embedded[0].Content)
	}
}

func TestParseK8sDocumentCRStringConfig(t *testing.T) {
	embedded, err := ParseK8sDocument(`
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: otel
spec:
  config: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
`)
	if err != nil {
		t.Fatalf("ParseK8sDocument: %v", err)
	}
	if len(embedded) != 1 {
		t.Fatalf("got %d embedded configs, want 1", len(embedded))
	}
	if !strings.Contains(embedded[0].Content, "protocols") {
		t.Errorf("embedded content = %q", embedded[0].Content)
	}
}

func TestParseK8sDocumentConfigMap(t *testing.T) {
	embedded, err := ParseK8sDocument(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-config
data:
  other.txt: |
    not a collector config
  config.yaml: |
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
	if err != nil {
		t.Fatalf("ParseK8sDocument: %v", err)
	}
	if len(embedded) != 1 {
		t.Fatalf("got %d embedded configs, want 1", len(embedded))
	}
	if embedded[0].Source != "ConfigMap data[config.yaml]" {
		t.Errorf("source = %q", embedded[0].Source)
	}
}

func TestParseK8sDocumentConfigMapBrokenYAMLExtracted(t *testing.T) {
	// A .yaml data value that is Collector configuration but syntactically
	// broken must still be extracted so downstream validation can fail it,
	// rather than being silently skipped when it fails to parse.
	embedded, err := ParseK8sDocument("apiVersion: v1\n" +
		"kind: ConfigMap\n" +
		"metadata:\n" +
		"  name: otel-config\n" +
		"data:\n" +
		"  config.yaml: |\n" +
		"    receivers:\n" +
		"      otlp:\n" +
		"    \tbad: indent\n" +
		"    service:\n" +
		"      pipelines:\n" +
		"        traces:\n" +
		"          receivers: [otlp]\n")
	if err != nil {
		t.Fatalf("ParseK8sDocument: %v", err)
	}
	if len(embedded) != 1 {
		t.Fatalf("got %d embedded configs, want 1", len(embedded))
	}
	if embedded[0].Source != "ConfigMap data[config.yaml]" {
		t.Errorf("source = %q", embedded[0].Source)
	}
}

func TestParseK8sDocumentPlainManifest(t *testing.T) {
	embedded, err := ParseK8sDocument("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\n")
	if err != nil {
		t.Fatalf("ParseK8sDocument: %v", err)
	}
	if len(embedded) != 0 {
		t.Errorf("plain manifest produced embedded configs: %v", embedded)
	}
}

func TestParseK8sDocumentInvalidYAML(t *testing.T) {
	if _, err := ParseK8sDocument("kind: ConfigMap\n\tbad indent"); err == nil {
		t.Errorf("expected YAML error")
	}
}
