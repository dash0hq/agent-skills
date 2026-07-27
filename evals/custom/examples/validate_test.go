package examples

import (
	"strings"
	"sync"
	"testing"

	"github.com/dash0hq/agent-skills/evals/custom/examples/otelcolbin"
	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

var (
	binaryOnce sync.Once
	binaryPath string
	binaryErr  error
)

// testCollectorValidator returns a validator backed by the pinned
// otelcol-contrib binary, skipping the test when the binary cannot be
// fetched in this environment.
func testCollectorValidator(t *testing.T) *CollectorValidator {
	t.Helper()
	binaryOnce.Do(func() {
		versions, err := harness.LoadVersions("../versions.env")
		if err != nil {
			binaryErr = err
			return
		}
		binaryPath, binaryErr = otelcolbin.Fetch(versions.OtelcolContribVersion, versions.Raw)
	})
	if binaryErr != nil {
		t.Skipf("otelcol-contrib binary unavailable: %v", binaryErr)
	}
	return &CollectorValidator{BinaryPath: binaryPath}
}

func testValidator(t *testing.T) *Validator {
	t.Helper()
	code := testCodeValidator(t)
	validator, err := NewValidator(testCollectorValidator(t), code, false)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	return validator
}

// testCodeValidator returns a CodeValidator using the pinned OpenTelemetry Go
// versions from the committed versions.env, cleaned up at test end.
func testCodeValidator(t *testing.T) *CodeValidator {
	t.Helper()
	versions, err := harness.LoadVersions("../versions.env")
	if err != nil {
		t.Fatalf("LoadVersions: %v", err)
	}
	code := NewCodeValidator(versions.OtelGoCoreVersion, versions.OtelGoLogVersion)
	t.Cleanup(code.Cleanup)
	return code
}

func findResult(report *Report, category Category) *Result {
	for _, result := range report.Results {
		if result.Category == category {
			return result
		}
	}
	return nil
}

func TestValidateTreeBasic(t *testing.T) {
	validator := testValidator(t)
	report, err := validator.ValidateTree("testdata/tree-basic")
	if err != nil {
		t.Fatalf("ValidateTree: %v", err)
	}
	if failures := report.Failures(); len(failures) != 0 {
		for _, failure := range failures {
			t.Errorf("unexpected failure %s:%d: %s", failure.File, failure.Line, failure.Detail)
		}
	}

	// eval:skip honored and reported.
	skip := findResult(report, CategorySkip)
	if skip == nil || skip.Status != StatusExempt || skip.Annotation != AnnotationSkip {
		t.Errorf("skip block not reported as exempt: %+v", skip)
	}

	// BAD-marked block exempt and categorized distinctly.
	bad := findResult(report, CategoryBad)
	if bad == nil || bad.Status != StatusExempt {
		t.Errorf("BAD block not reported as exempt: %+v", bad)
	}

	// The self-contained Go block is complete and compiles.
	code := findResult(report, CategoryCodeComplete)
	if code == nil || code.Status != StatusValidated {
		t.Errorf("go block not reported as compiled code-complete: %+v", code)
	}

	// The embedded CR config validates as Collector config, with
	// placeholder substitution recorded.
	var embedded *Result
	for _, result := range report.Results {
		if strings.Contains(result.Detail, "OpenTelemetryCollector spec.config") {
			embedded = result
		}
	}
	if embedded == nil {
		t.Fatalf("no embedded Collector config result: %+v", report.Results)
	}
	if embedded.Status != StatusValidated {
		t.Errorf("embedded config status = %q (%s)", embedded.Status, embedded.Detail)
	}
	if len(embedded.Substitutions) == 0 {
		t.Errorf("embedded config placeholder substitutions not recorded")
	}
}

func TestValidateTreeFragmentErrorDetected(t *testing.T) {
	// AE3: an invalid Collector block fails, naming file and position, even
	// though the block is a service-less fragment that gets scaffolded.
	validator := testValidator(t)
	report, err := validator.ValidateTree("testdata/tree-invalid")
	if err != nil {
		t.Fatalf("ValidateTree: %v", err)
	}
	failures := report.Failures()
	if len(failures) != 1 {
		t.Fatalf("got %d failures, want 1: %+v", len(failures), failures)
	}
	failure := failures[0]
	if !strings.Contains(failure.File, "bad-config.md") || failure.Line != 8 {
		t.Errorf("failure position = %s:%d, want bad-config.md:8", failure.File, failure.Line)
	}
	if failure.Category != CategoryCollectorFragment {
		t.Errorf("failure category = %q, want collector-fragment", failure.Category)
	}
	if !strings.Contains(failure.Detail, "check_interval") {
		t.Errorf("failure detail does not name the component error: %s", failure.Detail)
	}
}

func TestValidateTreeUnclassifiedFails(t *testing.T) {
	// Untagged blocks that no heuristic matches are validation failures, so
	// new untagged blocks cannot slip in silently. This path needs no
	// binary: use dry-run mode, which reports the same failure.
	validator, err := NewValidator(nil, nil, true)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	report, err := validator.ValidateTree("testdata/tree-unclassified")
	if err != nil {
		t.Fatalf("ValidateTree: %v", err)
	}
	failures := report.Failures()
	if len(failures) != 1 {
		t.Fatalf("got %d failures, want 1: %+v", len(failures), failures)
	}
	failure := failures[0]
	if !strings.Contains(failure.File, "untagged.md") || failure.Line != 4 {
		t.Errorf("failure position = %s:%d, want untagged.md:4", failure.File, failure.Line)
	}
	if failure.Category != CategoryUnclassified {
		t.Errorf("failure category = %q, want unclassified", failure.Category)
	}
}

func TestConnectorFragmentValidates(t *testing.T) {
	// A connectors: fragment is scaffolded with paired source and target
	// pipelines and passes otelcol-contrib validate.
	validator := testValidator(t)
	result := &Result{}
	validator.validateCollectorDoc(result, `
connectors:
  signaltometrics:
    spans:
      - name: http.server.request.duration
        description: "Duration of HTTP server requests."
        unit: s
        conditions:
          - kind == SPAN_KIND_SERVER and attributes["http.request.method"] != nil
        exponential_histogram:
          max_size: 160
          value: Seconds(end_time - start_time)
          count: "1"
`)
	if result.Status != StatusValidated {
		t.Fatalf("connector fragment status = %q: %s", result.Status, result.Detail)
	}
	if result.Category != CategoryCollectorFragment {
		t.Errorf("category = %q, want collector-fragment", result.Category)
	}
}

func TestEmbeddedConfigMapErrorDetected(t *testing.T) {
	// An error inside a ConfigMap-embedded Collector config is detected.
	validator := testValidator(t)
	doc := &Document{
		Block: &Block{File: "inline.md", Line: 1, Tag: "yaml"},
		Line:  2,
		Content: `apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-config
data:
  config.yaml: |
    processors:
      memory_limiter:
        limit_mib: 512
`,
	}
	results := validator.validateK8sDoc(&Result{File: doc.Block.File, Line: doc.Line}, doc)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (manifest + embedded): %+v", len(results), results)
	}
	embedded := results[1]
	if embedded.Status != StatusFailed {
		t.Fatalf("embedded config status = %q, want failed", embedded.Status)
	}
	if !strings.Contains(embedded.Detail, "ConfigMap data[config.yaml]") {
		t.Errorf("embedded failure does not name its source: %s", embedded.Detail)
	}
	if !strings.Contains(embedded.Detail, "check_interval") {
		t.Errorf("embedded failure does not name the component error: %s", embedded.Detail)
	}
}

func TestEmbeddedConfigMapBrokenYAMLDetected(t *testing.T) {
	// A ConfigMap data value that is Collector configuration but is
	// syntactically broken YAML must be extracted and fail validation, not be
	// silently skipped. The tab-indented line makes the value unparseable
	// while its top-level receivers/service sections mark it as Collector
	// configuration; the .yaml key also forces extraction.
	validator := testValidator(t)
	doc := &Document{
		Block: &Block{File: "inline.md", Line: 1, Tag: "yaml"},
		Line:  2,
		Content: "apiVersion: v1\n" +
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
			"          receivers: [otlp]\n" +
			"          exporters: [debug]\n",
	}
	results := validator.validateK8sDoc(&Result{File: doc.Block.File, Line: doc.Line}, doc)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (manifest + embedded): %+v", len(results), results)
	}
	embedded := results[1]
	if embedded.Status != StatusFailed {
		t.Fatalf("broken embedded config status = %q, want failed", embedded.Status)
	}
	if !strings.Contains(embedded.Detail, "ConfigMap data[config.yaml]") {
		t.Errorf("embedded failure does not name its source: %s", embedded.Detail)
	}
}

func TestEmbeddedCRErrorDetected(t *testing.T) {
	// An error inside an OpenTelemetryCollector CR spec.config is detected.
	validator := testValidator(t)
	doc := &Document{
		Block: &Block{File: "inline.md", Line: 1, Tag: "yaml"},
		Line:  2,
		Content: `apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: otel
spec:
  config:
    processors:
      memory_limiter:
        limit_mib: 512
`,
	}
	results := validator.validateK8sDoc(&Result{File: doc.Block.File, Line: doc.Line}, doc)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (manifest + embedded): %+v", len(results), results)
	}
	embedded := results[1]
	if embedded.Status != StatusFailed {
		t.Fatalf("embedded config status = %q, want failed", embedded.Status)
	}
	if !strings.Contains(embedded.Detail, "OpenTelemetryCollector spec.config") {
		t.Errorf("embedded failure does not name its source: %s", embedded.Detail)
	}
	if !strings.Contains(embedded.Detail, "check_interval") {
		t.Errorf("embedded failure does not name the component error: %s", embedded.Detail)
	}
}

func TestRenderSummaryHeadlineCounts(t *testing.T) {
	// The headline counts every status and reports code-complete compiles
	// and the exempt breakdown, so a green run cannot overstate itself.
	report := &Report{Results: []*Result{
		{File: "a.md", Category: CategoryCollectorConfig, Status: StatusValidated},
		{File: "a.md", Category: CategoryCodeComplete, Tag: "go", Status: StatusValidated},
		{File: "a.md", Category: CategoryCodeFragment, Tag: "go", Status: StatusExempt},
		{File: "a.md", Category: CategoryCodeFragment, Tag: "python", Status: StatusExempt},
		{File: "a.md", Category: CategoryBash, Tag: "bash", Status: StatusExempt},
		{File: "a.md", Category: CategoryCodeComplete, Tag: "java", Status: StatusSkippedNoToolchain},
	}}
	line := report.summaryLine()
	// Two validated results, of which one is a compiled code-complete block.
	if !strings.Contains(line, "2 validated (1 code compiled)") {
		t.Errorf("headline validated/compiled counts wrong: %q", line)
	}
	// Three exempt results, with the code-fragment category leading (count 2).
	if !strings.Contains(line, "3 exempt") {
		t.Errorf("headline exempt count wrong: %q", line)
	}
	if !strings.Contains(line, "2 code-fragment") {
		t.Errorf("headline exempt breakdown missing code-fragment: %q", line)
	}
	if !strings.Contains(line, "1 skipped-no-toolchain") {
		t.Errorf("headline skipped-no-toolchain count wrong: %q", line)
	}
	if !strings.Contains(line, "0 failed") {
		t.Errorf("headline failed count wrong: %q", line)
	}
	// The headline is a single line.
	if strings.Contains(line, "\n") {
		t.Errorf("headline is not a single line: %q", line)
	}
}

func TestDryRunReportsSubstitutions(t *testing.T) {
	validator, err := NewValidator(nil, nil, true)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	report, err := validator.ValidateTree("testdata/tree-basic")
	if err != nil {
		t.Fatalf("ValidateTree: %v", err)
	}
	rendered := report.Render()
	if !strings.Contains(rendered, "<OTLP_ENDPOINT>") {
		t.Errorf("dry-run report does not list placeholder substitutions:\n%s", rendered)
	}
	if !strings.Contains(rendered, "eval:skip") {
		t.Errorf("dry-run report does not list the skip exemption:\n%s", rendered)
	}
}
