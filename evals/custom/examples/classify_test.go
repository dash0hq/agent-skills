package examples

import "testing"

func classifyContent(t *testing.T, tag, content string, annotation Annotation) Category {
	t.Helper()
	block := &Block{Tag: tag, Annotation: annotation, Content: content}
	docs := block.Documents()
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	return Classify(docs[0])
}

func TestClassifyHeuristics(t *testing.T) {
	cases := []struct {
		name    string
		tag     string
		content string
		want    Category
	}{
		{
			name: "apiVersion wins over collector keys",
			tag:  "yaml",
			content: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c\ndata:\n" +
				"  config.yaml: |\n    receivers:\n      otlp: {}\n",
			want: CategoryK8sManifest,
		},
		{
			name:    "collector fragment",
			tag:     "yaml",
			content: "processors:\n  batch: {}\n",
			want:    CategoryCollectorFragment,
		},
		{
			name: "complete collector config",
			tag:  "yaml",
			content: `receivers:
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
`,
			want: CategoryCollectorConfig,
		},
		{
			name: "config with service but undefined components is a fragment",
			tag:  "yaml",
			content: `exporters:
  debug: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`,
			want: CategoryCollectorFragment,
		},
		{
			name:    "docker compose",
			tag:     "yaml",
			content: "services:\n  collector:\n    image: otel/opentelemetry-collector-contrib:0.156.0\n",
			want:    CategoryDockerCompose,
		},
		{
			name:    "bare OTTL statement",
			tag:     "",
			content: `set(resource.attributes["k8s.cluster.name"], "prod")`,
			want:    CategoryOTTLStatements,
		},
		{
			name:    "bare OTTL condition",
			tag:     "",
			content: `IsMatch(metric.name, "^k8s\\.replicaset\\..*$")`,
			want:    CategoryOTTLStatements,
		},
		{
			name:    "multi-line OTTL condition with connective",
			tag:     "",
			content: "resource.attributes[\"service.namespace\"] != nil\nand\nIsMatch(String(resource.attributes[\"service.namespace\"]), \"^platform.*$\")",
			want:    CategoryOTTLStatements,
		},
		{
			name:    "OTTL path lines",
			tag:     "",
			content: "span.name\nspan.attributes[\"http.method\"]\n",
			want:    CategoryOTTLStatements,
		},
		{
			name:    "prose is unclassified",
			tag:     "",
			content: "Applications send data to the Collector.",
			want:    CategoryUnclassified,
		},
		{
			name:    "yaml with unknown top-level keys is unclassified",
			tag:     "yaml",
			content: "mode: daemonset\nimage:\n  repository: example\n",
			want:    CategoryUnclassified,
		},
		{
			name:    "bash tag",
			tag:     "bash",
			content: "echo hi",
			want:    CategoryBash,
		},
		{
			name:    "complete go block with package declaration",
			tag:     "go",
			content: "package main\n\nfunc main() {}\n",
			want:    CategoryCodeComplete,
		},
		{
			name:    "go import snippet is a fragment",
			tag:     "go",
			content: "import (\n\t\"go.opentelemetry.io/otel\"\n)\n",
			want:    CategoryCodeFragment,
		},
		{
			name:    "go method body is a fragment",
			tag:     "go",
			content: "tp := sdktrace.NewTracerProvider()\notel.SetTracerProvider(tp)\n",
			want:    CategoryCodeFragment,
		},
		{
			name:    "python module with top-level import is complete",
			tag:     "python",
			content: "import os\n\nprint(os.getenv(\"OTEL_SERVICE_NAME\"))\n",
			want:    CategoryCodeComplete,
		},
		{
			name:    "python method body is a fragment",
			tag:     "python",
			content: "with tracer.start_as_current_span(\"work\"):\n    do_work()\n",
			want:    CategoryCodeFragment,
		},
		{
			name:    "ruby always biases to fragment",
			tag:     "ruby",
			content: "require \"opentelemetry/sdk\"\nOpenTelemetry::SDK.configure\n",
			want:    CategoryCodeFragment,
		},
		{
			name:    "json has no validator",
			tag:     "json",
			content: `{"a": 1}`,
			want:    CategoryNotValidated,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := classifyContent(t, testCase.tag, testCase.content, AnnotationNone)
			if got != testCase.want {
				t.Errorf("got %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestClassifyAnnotationWins(t *testing.T) {
	cases := []struct {
		annotation Annotation
		want       Category
	}{
		{AnnotationSkip, CategorySkip},
		{AnnotationBad, CategoryBad},
		{AnnotationCollectorConfig, CategoryCollectorConfig},
		{AnnotationFragment, CategoryCollectorFragment},
		{AnnotationK8s, CategoryK8sManifest},
	}
	for _, testCase := range cases {
		got := classifyContent(t, "", "anything at all", testCase.annotation)
		if got != testCase.want {
			t.Errorf("annotation %q: got %q, want %q", testCase.annotation, got, testCase.want)
		}
	}
}

func TestClassifyFragmentAnnotationIsContextAware(t *testing.T) {
	// eval:fragment on a code-tagged block yields a code fragment even when
	// the block looks complete, so a genuinely-complete-but-uncompilable Go
	// block can be opted out of compilation.
	got := classifyContent(t, "go", "package main\n\nfunc main() {}\n", AnnotationFragment)
	if got != CategoryCodeFragment {
		t.Errorf("code + eval:fragment: got %q, want %q", got, CategoryCodeFragment)
	}
	// eval:fragment on a yaml or untagged block still means a Collector
	// fragment.
	got = classifyContent(t, "yaml", "processors:\n  batch: {}\n", AnnotationFragment)
	if got != CategoryCollectorFragment {
		t.Errorf("yaml + eval:fragment: got %q, want %q", got, CategoryCollectorFragment)
	}
}

func TestClassifyBadMarker(t *testing.T) {
	content := "# BAD — do not do this\nset(resource.attributes[\"service.name\"], \"REDACTED\")"
	if got := classifyContent(t, "", content, AnnotationNone); got != CategoryBad {
		t.Errorf("got %q, want %q", got, CategoryBad)
	}
}
