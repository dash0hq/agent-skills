package scenarios

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
	"github.com/dash0hq/agent-skills/evals/custom/internal/testutil"
)

// dash0SpansSample mirrors `dash0 spans query -o json` output: OTLP/JSON with
// numeric span kinds, base64 ids, and a field that is not OTLP.
const dash0SpansSample = `{
  "resourceSpans": [{
    "resource": {"attributes": [
      {"key": "service.name", "value": {"stringValue": "checkout-service"}},
      {"key": "faas.name", "value": {"stringValue": "AgentSkillsEval-nodejs-TEST0001"}},
      {"key": "test.id", "value": {"stringValue": "TEST-0001"}}
    ]},
    "scopeSpans": [{
      "scope": {"name": "dash0.lambda-extension"},
      "spans": [
        {"traceId": "AAECAwQFBgcICQoLDA0ODw==", "spanId": "AAECAwQFBgc=", "name": "AgentSkillsEval-nodejs-TEST0001", "kind": 2},
        {"traceId": "AAECAwQFBgcICQoLDA0ODw==", "spanId": "AQECAwQFBgc=", "name": "GET", "kind": 3}
      ]
    }]
  }],
  "dash0ExplorerUrl": "https://app.example.test/explorer"
}`

func TestParseDash0Spans(t *testing.T) {
	req, err := parseDash0Spans([]byte(dash0SpansSample))
	require.NoError(t, err)
	spans := req.GetResourceSpans()[0].GetScopeSpans()[0].GetSpans()
	require.Len(t, spans, 2)
	require.Equal(t, tracepb.Span_SPAN_KIND_SERVER, spans[0].GetKind())
	require.Equal(t, tracepb.Span_SPAN_KIND_CLIENT, spans[1].GetKind())
	require.Equal(t, spans[0].GetTraceId(), spans[1].GetTraceId())

	_, err = parseDash0Spans([]byte(`{"error": "unauthorized"`))
	require.Error(t, err)
}

func TestWriteTracesFeedsTheSink(t *testing.T) {
	sink := otelsink.Start(t)
	req, err := parseDash0Spans([]byte(strings.ReplaceAll(dash0SpansSample, "TEST-0001", sink.TestID())))
	require.NoError(t, err)

	require.NoError(t, writeTraces(sink.Dir(), req))
	require.NoError(t, writeTraces(sink.Dir(), req), "a second poll replaces the file instead of duplicating spans")

	require.Equal(t, 2, sink.Traces(t).Len())
	require.NoError(t, assertLambdaInvocation(lambdaServiceName)(t, sink))
}

func TestTestIDFrom(t *testing.T) {
	require.Equal(t, "TEST-0001", testIDFrom("service.namespace=shop, test.id=TEST-0001"))
	require.Equal(t, "", testIDFrom("service.namespace=shop"))
}

func TestAssertLambdaInvocation(t *testing.T) {
	traceID := []byte("TEST-trace-00001")
	invocation := spanSpec{name: "AgentSkillsEval-nodejs-TEST0001", kind: tracepb.Span_SPAN_KIND_SERVER, traceID: traceID}
	fetch := spanSpec{name: "GET", kind: tracepb.Span_SPAN_KIND_CLIENT, traceID: traceID}
	identity := map[string]string{"service.name": lambdaServiceName, "faas.name": "AgentSkillsEval-nodejs-TEST0001"}

	t.Run("passes on an invocation trace with the outbound call", func(t *testing.T) {
		sink := otelsink.Start(t)
		feedSpans(t, sink, identity, invocation, fetch)
		require.NoError(t, assertLambdaInvocation(lambdaServiceName)(t, sink))
	})

	t.Run("fails when service.name falls back to the function name", func(t *testing.T) {
		sink := otelsink.Start(t)
		feedSpans(t, sink, map[string]string{"service.name": "AgentSkillsEval-nodejs-TEST0001", "faas.name": "AgentSkillsEval-nodejs-TEST0001"}, invocation, fetch)
		err := assertLambdaInvocation(lambdaServiceName)(t, sink)
		require.ErrorContains(t, err, "service.name values: [AgentSkillsEval-nodejs-TEST0001]")
	})

	t.Run("fails without the outbound call", func(t *testing.T) {
		sink := otelsink.Start(t)
		feedSpans(t, sink, identity, invocation)
		require.ErrorContains(t, assertLambdaInvocation(lambdaServiceName)(t, sink), "no CLIENT span")
	})

	t.Run("fails without an invocation span", func(t *testing.T) {
		sink := otelsink.Start(t)
		feedSpans(t, sink, identity, fetch)
		require.ErrorContains(t, assertLambdaInvocation(lambdaServiceName)(t, sink), "no SERVER invocation span")
	})
}

func TestCheckLambdaTemplates(t *testing.T) {
	write := func(t *testing.T, mode string, env string) string {
		dir := t.TempDir()
		tpl := `{"Resources": {"CheckoutFunction": {"Type": "AWS::Lambda::Function", "Properties": {
			"TracingConfig": {"Mode": "` + mode + `"},
			"Environment": {"Variables": {` + env + `}}}}}}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, "CheckoutStack.template.json"), []byte(tpl), 0o644))
		return dir
	}

	require.NoError(t, checkLambdaTemplates(write(t, "PassThrough", `"DASH0_TOKEN_SECRET_ARN": "arn:aws:secretsmanager:eu-west-1:000000000000:secret:TEST-0001"`)))
	require.ErrorContains(t, checkLambdaTemplates(write(t, "Active", `"DASH0_TOKEN_SECRET_ARN": "x"`)), "X-Ray tracing is Active")
	require.ErrorContains(t, checkLambdaTemplates(write(t, "PassThrough", `"DASH0_AUTH_TOKEN": "TEST-0001"`)), "DASH0_AUTH_TOKEN carries a token")
	require.Error(t, checkLambdaTemplates(t.TempDir()), "an empty assembly is a build failure")

	// The upstream Collector layer reads the token from the environment, so a
	// deploy-time Secrets Manager reference passes, plain or inside Fn::Join.
	require.NoError(t, checkLambdaTemplates(write(t, "PassThrough", `"OTLP_AUTH_TOKEN": "{{resolve:secretsmanager:arn:aws:secretsmanager:eu-west-1:000000000000:secret:TEST-0001:SecretString:::}}"`)))
	require.NoError(t, checkLambdaTemplates(write(t, "PassThrough", `"OTLP_AUTH_TOKEN": {"Fn::Join": ["", ["{{resolve:secretsmanager:", {"Ref": "TEST"}, ":SecretString:::}}"]]}`)))
}

func TestCheckLambdaTemplatesLayerNames(t *testing.T) {
	write := func(t *testing.T, layerName string) string {
		dir := t.TempDir()
		tpl := `{"Resources": {"ConfigLayer": {"Type": "AWS::Lambda::LayerVersion", "Properties": {` + layerName + `}}}}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, "CheckoutStack.template.json"), []byte(tpl), 0o644))
		return dir
	}

	require.NoError(t, checkLambdaTemplates(write(t, `"LayerName": "AgentSkillsEval-TEST-0001-collector-config"`)))
	require.ErrorContains(t, checkLambdaTemplates(write(t, `"LayerName": "otel-collector-config"`)), "does not start with the stack name")
	require.ErrorContains(t, checkLambdaTemplates(write(t, ``)), "does not start with the stack name", "an unnamed layer gets a name the deployment role cannot create")
}

func TestCheckCollectorConfig(t *testing.T) {
	const good = `receivers:
  telemetryapi:
  otlp:
    protocols:
      grpc:
        endpoint: "localhost:4317"
      http:
        endpoint: "localhost:4318"
processors:
  decouple:
exporters:
  otlp_grpc:
    endpoint: ${env:OTLP_ENDPOINT}
service:
  pipelines:
    traces:
      receivers: [telemetryapi, otlp]
      processors: [decouple]
      exporters: [otlp_grpc]
`
	stage := func(t *testing.T, rel, content string) string {
		dir := t.TempDir()
		path := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		return dir
	}

	require.NoError(t, checkCollectorConfig(stage(t, "asset.TEST0001/otel-collector-config.yaml", good)))
	require.ErrorContains(t, checkCollectorConfig(stage(t, "asset.TEST0001/config/otel-collector-config.yaml", good)),
		"no Collector configuration", "a nested file unpacks where the Collector never looks")
	require.ErrorContains(t, checkCollectorConfig(stage(t, "asset.TEST0001/otel-collector-config.yaml",
		strings.ReplaceAll(good, "localhost:4317", "0.0.0.0:4317"))), "does not bind to localhost")
	require.ErrorContains(t, checkCollectorConfig(stage(t, "asset.TEST0001/otel-collector-config.yaml",
		strings.ReplaceAll(good, "processors: [decouple]", "processors: [memory_limiter, batch]"))), "does not run the decouple processor")
	require.ErrorContains(t, checkCollectorConfig(stage(t, "asset.TEST0001/otel-collector-config.yaml",
		strings.ReplaceAll(good, "processors: [decouple]", "processors: [memory_limiter, decouple]"))), "runs memory_limiter")
}

func TestLambdaConfigGRPCEndpoint(t *testing.T) {
	for _, endpoint := range []string{"https://ingress.example.test", "https://ingress.example.test/", "https://ingress.example.test:443"} {
		require.Equal(t, "ingress.example.test:4317", LambdaConfig{OTLPEndpoint: endpoint}.grpcEndpoint())
	}
}

func TestLambdaConfigAccount(t *testing.T) {
	cfg := LambdaConfig{TokenSecretARN: "arn:aws:secretsmanager:eu-west-1:000000000000:secret:TEST-0001"}
	require.Equal(t, "000000000000", cfg.account())
}

// --- AWS-gated test (skips cleanly without the account configuration) ---

// TestLambdaScenarios runs the AWS Lambda scenarios against a real account.
// It needs AWS credentials for the role in evals/custom/aws/eval-access.yaml,
// the npx, aws, and dash0 CLIs, and the EVAL_* and DASH0_AUTH_TOKEN variables
// LambdaConfigFromEnv reads.
func TestLambdaScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping AWS Lambda scenarios in -short mode")
	}
	if loaded, err := harness.LoadDotEnv(); err != nil {
		t.Fatalf("loading .env: %v", err)
	} else if len(loaded) > 0 {
		t.Logf("loaded %d variable(s) from .env: %s", len(loaded), strings.Join(loaded, ", "))
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("skipping: ANTHROPIC_API_KEY unset; refusing to invoke the real agent (set it in .env or the environment)")
	}
	cfg, missing := LambdaConfigFromEnv()
	if len(missing) > 0 {
		t.Skipf("skipping: AWS Lambda configuration unset: %s", strings.Join(missing, ", "))
	}
	for _, tool := range []string{"npx", "aws", "dash0"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("skipping: %s not on PATH", tool)
		}
	}

	root := testutil.RepoRoot(t)
	versions, err := harness.LoadVersions(filepath.Join(root, "evals", "custom", "versions.env"))
	require.NoError(t, err)
	binary := os.Getenv("EVAL_AGENT_BINARY")
	selected := scenarioFilter(os.Getenv("EVAL_SCENARIOS"))

	for _, sc := range LambdaScenarios() {
		t.Run(sc.ID, func(t *testing.T) {
			if !selected(sc.ID) {
				t.Skip("skipped by the EVAL_SCENARIOS filter")
			}
			fix := NewLambdaFixture(t, cfg, sc.ID)
			defer fix.Close()
			runner := &harness.Runner{
				RepoRoot: root,
				Agent:    &harness.Agent{Binary: binary, PluginDir: root, Model: versions.EvalModel},
				Hooks:    fix.Hooks(),
			}
			v := runner.Run(t, sc)
			logVerdict(t, v)
			if !v.Passed {
				t.Errorf("scenario %s failed with class %s: %s", sc.ID, v.Class, v.Detail)
			}
		})
	}
}
