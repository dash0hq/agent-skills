package examples

import (
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

var (
	sharedOTTL     *OTTLValidator
	sharedOTTLOnce sync.Once
	sharedOTTLErr  error
)

func testOTTLValidator(t *testing.T) *OTTLValidator {
	t.Helper()
	sharedOTTLOnce.Do(func() {
		sharedOTTL, sharedOTTLErr = NewOTTLValidator()
	})
	if sharedOTTLErr != nil {
		t.Fatalf("NewOTTLValidator: %v", sharedOTTLErr)
	}
	return sharedOTTL
}

func transformConfig(t *testing.T, config string) map[string]any {
	t.Helper()
	var node map[string]any
	if err := yaml.Unmarshal([]byte(config), &node); err != nil {
		t.Fatalf("unmarshal test config: %v", err)
	}
	return node
}

func TestOTTLStatementContextMatters(t *testing.T) {
	validator := testOTTLValidator(t)

	// A log-context statement fails inside trace_statements (span context)
	// but parses inside log_statements (log context).
	statement := `set(log.attributes["team"], "checkout")`
	failing := transformConfig(t, `
processors:
  transform:
    trace_statements:
      - context: span
        statements:
          - `+statement+`
`)
	errs := validator.ValidateCollectorOTTL(failing)
	if len(errs) != 1 {
		t.Fatalf("trace context: got %d errors, want 1: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "trace_statements") {
		t.Errorf("error does not name the statement group: %v", errs[0])
	}

	passing := transformConfig(t, `
processors:
  transform:
    log_statements:
      - context: log
        statements:
          - `+statement+`
`)
	if errs := validator.ValidateCollectorOTTL(passing); len(errs) != 0 {
		t.Fatalf("log context: unexpected errors: %v", errs)
	}
}

func TestOTTLInvalidSyntaxFails(t *testing.T) {
	validator := testOTTLValidator(t)
	node := transformConfig(t, `
processors:
  transform:
    trace_statements:
      - context: span
        statements:
          - set(span.attributes["a"
`)
	if errs := validator.ValidateCollectorOTTL(node); len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
}

func TestOTTLFilterConditions(t *testing.T) {
	validator := testOTTLValidator(t)
	node := transformConfig(t, `
processors:
  filter:
    metrics:
      datapoint:
        - 'IsMatch(ConvertCase(String(metric.name), "lower"), "^k8s\\.replicaset\\.")'
    logs:
      log_record:
        - 'IsMatch(log.body["string"], "-----BEGIN PRIVATE KEY-----")'
`)
	if errs := validator.ValidateCollectorOTTL(node); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	invalid := transformConfig(t, `
processors:
  filter:
    traces:
      span:
        - 'NoSuchFunction(span.name)'
`)
	if errs := validator.ValidateCollectorOTTL(invalid); len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
}

func TestOTTLFlatStatementsForm(t *testing.T) {
	validator := testOTTLValidator(t)
	node := transformConfig(t, `
processors:
  transform:
    trace_statements:
      - set(span.attributes["env"], "production")
    statements:
      - set(log.attributes["env"], "production")
`)
	if errs := validator.ValidateCollectorOTTL(node); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestOTTLConfmapEscaping(t *testing.T) {
	validator := testOTTLValidator(t)
	// $$1 in Collector YAML reaches the OTTL parser as $1.
	node := transformConfig(t, `
processors:
  transform:
    log_statements:
      - context: log
        statements:
          - replace_pattern(log.body["string"], "\\b(\\d{4})\\d{5,11}(\\d{4})\\b", "$$1****$$2")
`)
	if errs := validator.ValidateCollectorOTTL(node); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateBareOTTL(t *testing.T) {
	validator := testOTTLValidator(t)
	valid := []string{
		`set(resource.attributes["k8s.cluster.name"], "prod")`,
		`IsMatch(metric.name, "^k8s\\.replicaset\\..*$")`,
		"time_unix_nano < UnixNano(Now()) - 21600000000000",
		"resource.attributes[\"service.namespace\"] != nil\nand\nIsMatch(String(resource.attributes[\"service.namespace\"]), \"^platform.*$\")",
		"span.name\nspan.attributes[\"http.method\"]",
		`Substring(log.body.string, 0, 1024)`,
	}
	for _, content := range valid {
		if err := validator.ValidateBareOTTL(content); err != nil {
			t.Errorf("ValidateBareOTTL(%q) = %v, want nil", content, err)
		}
	}
	invalid := []string{
		`set(span.attributes["a"`,
		`NoSuchFunction(span.name)`,
	}
	for _, content := range invalid {
		if err := validator.ValidateBareOTTL(content); err == nil {
			t.Errorf("ValidateBareOTTL(%q) = nil, want error", content)
		}
	}
}
