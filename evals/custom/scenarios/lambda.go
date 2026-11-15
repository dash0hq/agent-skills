// AWS Lambda scenarios: the agent instruments a CDK-deployed Lambda function
// fixture, and the harness deploys it to a real AWS account, invokes it, and
// reads the telemetry back from a Dash0 dataset into the sink, where the
// ordinary assertions judge it (see evals/custom/README.md, "AWS Lambda
// scenarios", and evals/custom/aws/eval-access.yaml for the account setup).
package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

// LambdaNodejsDash0LayerID covers platforms/aws-lambda.md and
// platforms/aws-lambda-dash0-layers.md: the agent instruments a Node.js
// function with the Dash0 Lambda extension layer (path B).
const LambdaNodejsDash0LayerID = "lambda-nodejs-dash0-layer"

// Names the account setup in evals/custom/aws/eval-access.yaml scopes every
// permission to; changing one here requires changing it there.
const (
	lambdaStackPrefix         = "AgentSkillsEval-"
	lambdaCDKQualifier        = "agskeval"
	lambdaPermissionsBoundary = "AgentSkillsEvalFunctionBoundary"
)

const (
	// lambdaServiceName is the service identity the prompt asks for; the
	// function name must stay in faas.name, not service.name.
	lambdaServiceName = "checkout-service"
	// lambdaInvocations is how many times the harness invokes the function.
	lambdaInvocations = 3
	// lambdaScenarioTimeout covers the agent run, npm install, synthesis,
	// a CloudFormation deployment, and the invocations.
	lambdaScenarioTimeout = 30 * time.Minute
	// lambdaTelemetryTimeout covers Dash0 ingestion latency on top of the
	// extension's export at the end of each invocation.
	lambdaTelemetryTimeout = 5 * time.Minute
	// lambdaPollInterval is how often the harness re-reads spans from Dash0.
	lambdaPollInterval = 15 * time.Second
	// lambdaQueryLimit is the CLI's ceiling for JSON output.
	lambdaQueryLimit = 100
)

// Environment the AWS Lambda tests read. The aws-scenarios CI job sets them
// from repository variables and secrets; a local run sets them by hand.
const (
	EnvLambdaRegion              = "EVAL_AWS_REGION"
	EnvLambdaDash0OTLPEndpoint   = "EVAL_DASH0_OTLP_ENDPOINT"
	EnvLambdaDash0APIURL         = "EVAL_DASH0_API_URL"
	EnvLambdaDash0Dataset        = "EVAL_DASH0_DATASET"
	EnvLambdaDash0TokenSecretARN = "EVAL_DASH0_TOKEN_SECRET_ARN"
	// EnvLambdaDash0AuthToken is the token the harness reads spans back
	// with. The harness hides it from the agent and from agent-authored code.
	EnvLambdaDash0AuthToken = "DASH0_AUTH_TOKEN"
)

func init() { registerScenarios(LambdaScenarios()...) }

// LambdaScenarios returns the AWS Lambda scenario set.
func LambdaScenarios() []harness.Scenario {
	return []harness.Scenario{{
		ID:    LambdaNodejsDash0LayerID,
		Skill: harness.SkillInstrumentation,
		RuleFiles: []string{
			"skills/otel-instrumentation/rules/platforms/aws-lambda.md",
			"skills/otel-instrumentation/rules/platforms/aws-lambda-dash0-layers.md",
		},
		RequiresAWS: true,
		FixturePath: "evals/custom/fixtures/lambda/nodejs-function",
		Prompt: `The current directory contains an AWS CDK app (TypeScript) that deploys the Node.js 22 AWS Lambda function CheckoutFunction (lib/checkout-stack.ts; the handler is lambda/index.js) as a ZIP archive. It emits no telemetry. Instrument it so its traces reach Dash0, per the otel-instrumentation skill (dash0-agent-skills:otel-instrumentation).

Requirements:
- Do not change the handler code in lambda/index.js.
- Dash0 is the only telemetry backend.
- The stack props already carry the Dash0 OTLP/HTTP endpoint (dash0Endpoint), the dataset (dash0Dataset), and the ARN of the AWS Secrets Manager secret holding the Dash0 auth token as a plain string (dash0TokenSecretArn). Use them; never write the token, the endpoint, or the dataset as literals.
- Attribute the telemetry to the service checkout-service, not to the function name.
- Keep everything marked harness-owned unchanged: the function name, the handling of the EVAL_* and DASH0_* variables in bin/app.ts, and the OTEL_RESOURCE_ATTRIBUTES value (append further attributes after a comma).
- Do not add a function URL, an API, or any other trigger.
- Do not deploy or synthesize against AWS; the harness deploys and invokes the function.`,
		Timeout:          lambdaScenarioTimeout,
		TelemetryTimeout: lambdaTelemetryTimeout,
		Assert:           assertLambdaInvocation(lambdaServiceName),
	}}
}

// LambdaConfig is the account and Dash0 configuration of a Lambda run.
type LambdaConfig struct {
	Region         string
	OTLPEndpoint   string
	APIURL         string
	Dataset        string
	TokenSecretARN string
	AuthToken      string
}

// LambdaConfigFromEnv reads the configuration and returns the names of the
// variables that are unset.
func LambdaConfigFromEnv() (LambdaConfig, []string) {
	cfg := LambdaConfig{
		Region:         os.Getenv(EnvLambdaRegion),
		OTLPEndpoint:   os.Getenv(EnvLambdaDash0OTLPEndpoint),
		APIURL:         os.Getenv(EnvLambdaDash0APIURL),
		Dataset:        os.Getenv(EnvLambdaDash0Dataset),
		TokenSecretARN: os.Getenv(EnvLambdaDash0TokenSecretARN),
		AuthToken:      os.Getenv(EnvLambdaDash0AuthToken),
	}
	var missing []string
	for name, v := range map[string]string{
		EnvLambdaRegion:              cfg.Region,
		EnvLambdaDash0OTLPEndpoint:   cfg.OTLPEndpoint,
		EnvLambdaDash0APIURL:         cfg.APIURL,
		EnvLambdaDash0Dataset:        cfg.Dataset,
		EnvLambdaDash0TokenSecretARN: cfg.TokenSecretARN,
		EnvLambdaDash0AuthToken:      cfg.AuthToken,
	} {
		if v == "" {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return cfg, missing
}

// account returns the AWS account ID embedded in the token secret ARN.
func (c LambdaConfig) account() string {
	parts := strings.Split(c.TokenSecretARN, ":")
	if len(parts) < 5 {
		return ""
	}
	return parts[4]
}

// LambdaFixture implements the harness FixtureHooks contract for one Lambda
// scenario. Every attempt deploys its own stack; every stack is destroyed
// when the test finishes.
type LambdaFixture struct {
	t   testingT
	cfg LambdaConfig

	mu        sync.Mutex
	stackName string
	cleanups  []func()
}

// NewLambdaFixture returns a LambdaFixture whose stacks are destroyed by
// Close. Defer Close right after creating the fixture: cdk destroy runs from
// the attempt workspace, which the runner creates with t.TempDir after this
// t.Cleanup fallback is registered, so the cleanup alone would run after the
// workspace is gone.
func NewLambdaFixture(t testingT, cfg LambdaConfig) *LambdaFixture {
	f := &LambdaFixture{t: t, cfg: cfg}
	t.Cleanup(f.Close)
	return f
}

// Hooks returns the harness fixture hooks backed by this LambdaFixture.
func (f *LambdaFixture) Hooks() harness.FixtureHooks {
	return harness.FixtureHooks{Build: f.build, Run: f.run, Diagnose: f.diagnose}
}

// Close stops the pollers and destroys every stack, in reverse order.
func (f *LambdaFixture) Close() {
	f.mu.Lock()
	cleanups := f.cleanups
	f.cleanups = nil
	f.mu.Unlock()
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
}

func (f *LambdaFixture) addCleanup(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanups = append(f.cleanups, fn)
}

// cdkEnv is the environment of the CDK app: the harness-owned inputs the
// fixture's bin/app.ts reads. Synthesis executes agent-authored code, so it
// runs on the scrubbed environment, without AWS credentials or the Dash0
// token; deployment and destruction add the credentials back but run from the
// pre-synthesized assembly, so no agent code executes alongside them.
func (f *LambdaFixture) cdkEnv(base []string, resourceAttrs string) []string {
	return append(base,
		"EVAL_STACK_NAME="+f.stackName,
		"EVAL_PERMISSIONS_BOUNDARY="+lambdaPermissionsBoundary,
		"EVAL_RESOURCE_ATTRIBUTES="+resourceAttrs,
		"CDK_DEFAULT_REGION="+f.cfg.Region,
		"CDK_DEFAULT_ACCOUNT="+f.cfg.account(),
		"AWS_REGION="+f.cfg.Region,
		"DASH0_ENDPOINT="+f.cfg.OTLPEndpoint,
		"DASH0_DATASET="+f.cfg.Dataset,
		"DASH0_TOKEN_SECRET_ARN="+f.cfg.TokenSecretARN,
	)
}

// build installs the app's dependencies, synthesizes it into cdk.out against
// the eval bootstrap qualifier, and checks the synthesized template for the
// properties telemetry cannot show.
func (f *LambdaFixture) build(ctx context.Context, workdir string, env map[string]string) error {
	f.stackName = lambdaStackPrefix + "nodejs-" + randomSuffix()
	scrubbed := harness.ScrubbedEnviron()
	if out, err := runIn(ctx, workdir, scrubbed, "npm", "install", "--no-audit", "--no-fund"); err != nil {
		return fmt.Errorf("npm install: %w\n%s", err, out)
	}
	if out, err := runIn(ctx, workdir, f.cdkEnv(scrubbed, env["OTEL_RESOURCE_ATTRIBUTES"]),
		"npx", "cdk", "synth", "--quiet", "--output", "cdk.out",
		"--context", "@aws-cdk/core:bootstrapQualifier="+lambdaCDKQualifier); err != nil {
		return fmt.Errorf("cdk synth: %w\n%s", err, out)
	}
	return checkLambdaTemplates(filepath.Join(workdir, "cdk.out"))
}

// run deploys the synthesized assembly, invokes the function, and starts
// copying its spans from Dash0 into the attempt's sink.
func (f *LambdaFixture) run(ctx context.Context, workdir string, env map[string]string) error {
	creds := f.cdkEnv(os.Environ(), env["OTEL_RESOURCE_ATTRIBUTES"])
	stack := f.stackName
	f.addCleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if out, err := runIn(ctx, workdir, creds, "npx", "cdk", "destroy", "--app", "cdk.out", "--force"); err != nil {
			f.t.Errorf("cdk destroy %s failed; delete the stack by hand: %v\n%s", stack, err, out)
		}
	})

	deployedAt := time.Now()
	if out, err := runIn(ctx, workdir, creds, "npx", "cdk", "deploy", "--app", "cdk.out",
		"--require-approval", "never", "--tags", "project=agent-skills-evals"); err != nil {
		return fmt.Errorf("cdk deploy %s: %w\n%s", stack, err, out)
	}
	for i := 0; i < lambdaInvocations; i++ {
		if err := invokeLambda(ctx, creds, stack); err != nil {
			return err
		}
	}

	pollCtx, stop := context.WithCancel(context.Background())
	f.addCleanup(stop)
	go f.pollDash0(pollCtx, env[harness.EnvSinkDir], env["OTEL_RESOURCE_ATTRIBUTES"], deployedAt.Add(-time.Minute))
	return nil
}

// invokeLambda invokes the function synchronously and fails on a function
// error, so a function the instrumentation broke is reported as such rather
// than as missing telemetry.
func invokeLambda(ctx context.Context, env []string, function string) error {
	payload, err := os.CreateTemp("", "eval-lambda-response-*.json")
	if err != nil {
		return err
	}
	_ = payload.Close()
	defer os.Remove(payload.Name())
	out, err := runIn(ctx, "", env, "aws", "lambda", "invoke", "--function-name", function,
		"--payload", "{}", "--cli-binary-format", "raw-in-base64-out", "--output", "json", payload.Name())
	if err != nil {
		return fmt.Errorf("invoke %s (the function name must stay the stack name): %w\n%s", function, err, out)
	}
	var meta struct {
		StatusCode    int    `json:"StatusCode"`
		FunctionError string `json:"FunctionError"`
	}
	if err := json.Unmarshal([]byte(out), &meta); err != nil {
		return fmt.Errorf("invoke %s: unreadable response metadata: %w\n%s", function, err, out)
	}
	if meta.StatusCode != 200 || meta.FunctionError != "" {
		body, _ := os.ReadFile(payload.Name())
		return fmt.Errorf("invoke %s: status %d, function error %q: %s", function, meta.StatusCode, meta.FunctionError, body)
	}
	return nil
}

// pollDash0 re-reads the run's spans from Dash0 until ctx ends and replaces
// the sink's traces.jsonl with each result, so the runner's assertion polls
// see telemetry as it is ingested.
func (f *LambdaFixture) pollDash0(ctx context.Context, sinkDir, resourceAttrs string, from time.Time) {
	testID := testIDFrom(resourceAttrs)
	failures := 0
	for {
		req, err := f.queryDash0(ctx, testID, from)
		switch {
		case err != nil && ctx.Err() == nil:
			if failures++; failures <= 3 {
				f.t.Logf("dash0 span query failed (attempt %d): %v", failures, err)
			}
		case err == nil:
			if err := writeTraces(sinkDir, req); err != nil {
				f.t.Logf("write sink traces: %v", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(lambdaPollInterval):
		}
	}
}

// queryDash0 returns the spans carrying test.id=testID since from.
func (f *LambdaFixture) queryDash0(ctx context.Context, testID string, from time.Time) (*coltracepb.ExportTraceServiceRequest, error) {
	out, err := runIn(ctx, "", []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"DASH0_API_URL=" + f.cfg.APIURL,
		"DASH0_AUTH_TOKEN=" + f.cfg.AuthToken,
		"DASH0_DATASET=" + f.cfg.Dataset,
	}, "dash0", "spans", "query",
		"--from", from.UTC().Format(time.RFC3339),
		"--filter", otelsink.TestIDAttribute+" is "+testID,
		"--precision", "disabled",
		"--limit", fmt.Sprint(lambdaQueryLimit),
		"-o", "json")
	if err != nil {
		return nil, fmt.Errorf("%w\n%s", err, out)
	}
	return parseDash0Spans([]byte(out))
}

// parseDash0Spans decodes `dash0 spans query -o json` output: OTLP/JSON
// resourceSpans, plus fields such as dash0ExplorerUrl that are not OTLP.
func parseDash0Spans(data []byte) (*coltracepb.ExportTraceServiceRequest, error) {
	req := &coltracepb.ExportTraceServiceRequest{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("decode dash0 spans: %w (output starts %q)", err, truncate(string(data), 200))
	}
	return req, nil
}

// writeTraces atomically replaces the sink's traces.jsonl with req.
func writeTraces(sinkDir string, req *coltracepb.ExportTraceServiceRequest) error {
	line, err := protojson.Marshal(req)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(sinkDir, ".traces-*.jsonl")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(append(line, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(sinkDir, "traces.jsonl"))
}

// diagnose dumps the function's recent CloudWatch log lines, which carry
// the extension's own startup and export messages.
func (f *LambdaFixture) diagnose(ctx context.Context, class harness.FailureClass, detail string) {
	f.t.Logf("lambda diagnostic [%s]: %s", class, detail)
	out, err := runIn(ctx, "", os.Environ(), "aws", "logs", "filter-log-events",
		"--region", f.cfg.Region,
		"--log-group-name", "/aws/lambda/"+f.stackName,
		"--limit", "80", "--query", "events[].message", "--output", "text")
	if err != nil {
		f.t.Logf("  [function logs] error: %v\n%s", err, out)
		return
	}
	f.t.Logf("  [function logs /aws/lambda/%s]\n%s", f.stackName, strings.TrimSpace(out))
}

// checkLambdaTemplates fails on synthesized properties the rules forbid and
// telemetry cannot reveal: X-Ray tracing left Active, which competes with
// OpenTelemetry for the trace context, and an auth token written into the
// function environment instead of read from Secrets Manager.
func checkLambdaTemplates(assemblyDir string) error {
	templates, err := filepath.Glob(filepath.Join(assemblyDir, "*.template.json"))
	if err != nil || len(templates) == 0 {
		return fmt.Errorf("no synthesized templates in %s", assemblyDir)
	}
	var problems []string
	for _, path := range templates {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var tpl struct {
			Resources map[string]struct {
				Type       string `json:"Type"`
				Properties struct {
					TracingConfig struct {
						Mode string `json:"Mode"`
					} `json:"TracingConfig"`
					Environment struct {
						Variables map[string]any `json:"Variables"`
					} `json:"Environment"`
				} `json:"Properties"`
			} `json:"Resources"`
		}
		if err := json.Unmarshal(data, &tpl); err != nil {
			return fmt.Errorf("parse %s: %w", filepath.Base(path), err)
		}
		for id, res := range tpl.Resources {
			if res.Type != "AWS::Lambda::Function" {
				continue
			}
			if res.Properties.TracingConfig.Mode == "Active" {
				problems = append(problems, fmt.Sprintf("%s: X-Ray tracing is Active; set it to PassThrough", id))
			}
			for name := range res.Properties.Environment.Variables {
				if strings.Contains(strings.ToUpper(name), "TOKEN") && !strings.Contains(strings.ToUpper(name), "SECRET") {
					problems = append(problems, fmt.Sprintf("%s: environment variable %s carries a token; read it from Secrets Manager instead", id, name))
				}
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("synthesized template check failed:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

// assertLambdaInvocation builds the Lambda assertion: the invocation span
// (SERVER) and the handler's outbound fetch (CLIENT, same trace) must reach
// Dash0, attributed to serviceName with the function identity in faas.name.
func assertLambdaInvocation(serviceName string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		all := sink.Traces(t)
		if all.Len() == 0 {
			return fmt.Errorf("no spans at the sink")
		}
		view := all.WithResourceAttribute("service.name", serviceName)
		if view.Len() == 0 {
			return fmt.Errorf("no spans with resource attribute service.name=%q (service.name values: %v)", serviceName, resourceValues(all, "service.name"))
		}
		invocationTraces := map[string]bool{}
		for _, sv := range view.WithKind(tracepb.Span_SPAN_KIND_SERVER).Spans() {
			if strings.HasPrefix(attrValue(sv.Resource.GetAttributes(), "faas.name"), lambdaStackPrefix) {
				invocationTraces[string(sv.Span.GetTraceId())] = true
			}
		}
		if len(invocationTraces) == 0 {
			return fmt.Errorf("no SERVER invocation span with faas.name set to the function (faas.name values: %v, span names: %v)", resourceValues(view, "faas.name"), view.Names())
		}
		for _, sv := range view.WithKind(tracepb.Span_SPAN_KIND_CLIENT).Spans() {
			if invocationTraces[string(sv.Span.GetTraceId())] {
				return nil
			}
		}
		return fmt.Errorf("no CLIENT span for the handler's outbound fetch in an invocation trace (span names: %v)", view.Names())
	}
}

// resourceValues lists the distinct values of a resource attribute in view.
func resourceValues(view *otelsink.Traces, key string) []string {
	seen := map[string]bool{}
	var out []string
	for _, sv := range view.Spans() {
		v := attrValue(sv.Resource.GetAttributes(), key)
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// testIDFrom extracts test.id from an OTEL_RESOURCE_ATTRIBUTES value.
func testIDFrom(resourceAttrs string) string {
	for _, kv := range strings.Split(resourceAttrs, ",") {
		if key, value, ok := strings.Cut(kv, "="); ok && strings.TrimSpace(key) == otelsink.TestIDAttribute {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// runIn executes one command in dir (the current directory when empty) with
// exactly env, and returns its combined output.
func runIn(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
