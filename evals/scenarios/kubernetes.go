// The Kubernetes eval scenarios (U6): kind-backed coverage of the
// otel-instrumentation Kubernetes platform rule file and the 4 Collector
// deployment rule files. Scenario declarations are data, like scenarios.go;
// the kind runtime (cluster, telemetry bridge, per-scenario fixture hooks)
// lives in kind.go and kindfixture.go, and the kind-gated test entrypoint in
// kubernetes_test.go.
//
// The scenarios register through harness.RegisterDefaultScenarios from init,
// so they join scenarios.Default() (registry completeness, CI selection)
// without editing the declaration list in scenarios.go. They are marked
// RequiresKind: ordinary skill-wide selection and the smoke set skip them,
// and CI (U7) runs them in a dedicated job that provisions a kind cluster.
package scenarios

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/harness"
)

// Kubernetes scenario IDs.
const (
	// K8sDownwardAPIID covers platforms/k8s.md: the agent wires downward-API
	// pod metadata into OTEL_RESOURCE_ATTRIBUTES on a pre-instrumented
	// workload, and the attributes are asserted at the sink.
	K8sDownwardAPIID = "k8s-platform-downward-api"
	// K8sOTelOperatorID covers deployment/opentelemetry-operator.md: the
	// agent authors OpenTelemetryCollector and Instrumentation CRs against
	// an operator the harness installed without cert-manager.
	K8sOTelOperatorID = "k8s-deploy-otel-operator"
	// K8sHelmChartID covers deployment/collector-helm-chart.md: the agent
	// authors Helm values and the harness installs the chart with them.
	K8sHelmChartID = "k8s-deploy-helm-chart"
	// K8sRawManifestsID covers deployment/raw-manifests.md: the agent
	// authors raw Collector manifests the harness applies.
	K8sRawManifestsID = "k8s-deploy-raw-manifests"
	// K8sDash0OperatorID covers deployment/dash0-operator.md: the harness
	// installs the Dash0 operator with a runtime-generated placeholder
	// token, the agent authors the Dash0Monitoring resource, and the
	// assertion targets the operator's export attempts at the relay.
	K8sDash0OperatorID = "k8s-deploy-dash0-operator"
)

// Identity values the Kubernetes prompts demand and the assertions check.
const (
	// K8sServiceName is the service.name required from workloads whose
	// exporter configuration the agent (or the harness workload) controls.
	K8sServiceName = "checkout-service-eval"
	// k8sContainerName is the container name of the seed checkout
	// Deployments, asserted as k8s.container.name in the downward-API
	// scenario (there is no downward-API field for it; the skill teaches
	// setting it as a literal).
	k8sContainerName = "checkout-service"
	// k8sServiceNamespace and k8sEnvironmentName pin the placeholder values
	// of the platforms/k8s.md pod-spec example so the assertion can check
	// them verbatim.
	k8sServiceNamespace = "checkout"
	k8sEnvironmentName  = "eval"
)

// Kubernetes per-scenario budgets: cluster image loads, Helm installs, and
// operator reconciliation take longer than the plain-Docker scenarios, and
// Collector batch processors delay the first export.
const (
	kindScenarioTimeout  = 25 * time.Minute
	kindTelemetryTimeout = 120 * time.Second
)

// KindScenarioSpec describes how the KindFixture exercises one kind scenario:
// the workload images it builds and loads into the cluster, and the
// scenario-specific deployment step run after the namespace and runtime
// Secrets exist. It mirrors CollectorScenarioSpec (collector_hooks.go): the
// per-scenario data lives here as a table alongside the declarations, so
// adding a scenario is one edit site rather than two switch arms.
type KindScenarioSpec struct {
	// Images maps each workload image tag to its build-context directory,
	// relative to the evals/ module root; the KindFixture builds and loads
	// each before the run. The shared helper image is added by the fixture.
	Images map[string]string
	// Run performs the scenario-specific deployment. relay and token are the
	// per-attempt telemetry bridge and its bearer token; only the Dash0
	// operator scenario needs them, but every hook receives them so the
	// signature is uniform. resourceAttrs is the per-run OTEL_RESOURCE_ATTRIBUTES
	// value (carrying test.id); the operator scenario substitutes it for the
	// seed placeholder before apply, and other scenarios ignore it.
	Run func(f *KindFixture, ctx context.Context, relay *kindRelay, namespace, workdir, token, resourceAttrs string) error
}

// kindScenarioSpecs maps each kind scenario ID to its execution spec.
// NewKindFixture looks the spec up; a missing entry is the former switch
// default and fails the fixture the same way.
var kindScenarioSpecs = map[string]KindScenarioSpec{
	K8sDownwardAPIID: {
		Images: map[string]string{instrumentedGoImage: "fixtures/k8s/instrumented-go-service"},
		Run: func(f *KindFixture, ctx context.Context, _ *kindRelay, namespace, workdir, _, _ string) error {
			return f.runDownwardAPI(ctx, namespace, workdir)
		},
	},
	K8sHelmChartID: {
		Images: map[string]string{instrumentedGoImage: "fixtures/k8s/instrumented-go-service"},
		Run: func(f *KindFixture, ctx context.Context, _ *kindRelay, namespace, workdir, _, _ string) error {
			return f.runHelmChart(ctx, namespace, workdir)
		},
	},
	K8sRawManifestsID: {
		Images: map[string]string{instrumentedGoImage: "fixtures/k8s/instrumented-go-service"},
		Run: func(f *KindFixture, ctx context.Context, _ *kindRelay, namespace, workdir, _, _ string) error {
			return f.runRawManifests(ctx, namespace, workdir)
		},
	},
	K8sOTelOperatorID: {
		Images: map[string]string{nodejsImage: "fixtures/nodejs-service"},
		Run: func(f *KindFixture, ctx context.Context, _ *kindRelay, namespace, workdir, _, resourceAttrs string) error {
			return f.runOTelOperator(ctx, namespace, workdir, resourceAttrs)
		},
	},
	K8sDash0OperatorID: {
		Images: map[string]string{nodejsImage: "fixtures/nodejs-service"},
		Run: func(f *KindFixture, ctx context.Context, relay *kindRelay, namespace, workdir, token, _ string) error {
			return f.runDash0Operator(ctx, relay, namespace, workdir, token)
		},
	},
}

func init() {
	harness.RegisterDefaultScenarios(KubernetesScenarios()...)
}

// KubernetesScenarios returns the kind-backed scenarios in registration
// order. They register into every DefaultRegistry via init; the kind test
// entrypoint (kubernetes_test.go) iterates this slice directly.
func KubernetesScenarios() []harness.Scenario {
	return []harness.Scenario{
		K8sDownwardAPI(),
		K8sOTelOperator(),
		K8sHelmChart(),
		K8sRawManifests(),
		K8sDash0Operator(),
	}
}

// k8sPromptCommonRequirements is shared prompt text enforcing the runtime
// credential contract inside the cluster: endpoint and token exist only in
// the harness-created eval-otlp and otel-auth Secrets, never as literals in
// manifests the agent writes.
const k8sPromptCommonRequirements = `
Requirements:
- The runtime environment provides two Secrets in the target namespace: "eval-otlp" (keys EVAL_OTLP_ENDPOINT, EVAL_OTLP_TOKEN, OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_EXPORTER_OTLP_PROTOCOL, OTEL_EXPORTER_OTLP_HEADERS, OTEL_RESOURCE_ATTRIBUTES, EVAL_EXTRA_RESOURCE_ATTRIBUTES) and "otel-auth" (key "token", carrying the value for OTEL_EXPORTER_OTLP_HEADERS).
- Never write endpoint URLs, tokens, or Authorization header values as literals into any file; reference the Secrets (secretKeyRef, envFrom, or ${env:...} substitution in Collector configuration) instead.
- Do not create Secret manifests and do not change the harness-owned env entries marked as such in the seed manifests.
- All resources you author are applied into a single namespace chosen by the harness; do not hardcode namespace names in metadata or cross-namespace DNS names.`

// K8sDownwardAPI is the Kubernetes platform scenario: the workload image is
// already instrumented (evals/fixtures/k8s/instrumented-go-service), so the
// pod spec is the entire task, exactly the scope of platforms/k8s.md.
func K8sDownwardAPI() harness.Scenario {
	return harness.Scenario{
		ID:           K8sDownwardAPIID,
		Skill:        harness.SkillInstrumentation,
		RuleFiles:    []string{"skills/otel-instrumentation/rules/platforms/k8s.md"},
		RequiresKind: true,
		FixturePath:  "evals/fixtures/k8s/downward-api-workspace",
		Prompt: `The current directory contains Kubernetes manifests for an OpenTelemetry-instrumented checkout service (deployment.yaml, service.yaml). The container image reads all OpenTelemetry configuration from the standard OTEL_* environment variables. Configure the Deployment's pod spec for Kubernetes per the otel-instrumentation skill (dash0-agent-skills:otel-instrumentation).

Configuration goals:
- Set OTEL_EXPORTER_OTLP_ENDPOINT and OTEL_EXPORTER_OTLP_PROTOCOL from the "eval-otlp" Secret (keys of the same names), and OTEL_EXPORTER_OTLP_HEADERS from the "otel-auth" Secret (key "token").
- Set the service name to "` + K8sServiceName + `".
- Expose pod metadata via the downward API and set OTEL_RESOURCE_ATTRIBUTES so every signal carries k8s.pod.uid, k8s.pod.name, k8s.node.name, and k8s.container.name (the container is named "` + k8sContainerName + `").
- Set service.instance.id from the pod UID, service.namespace to "` + k8sServiceNamespace + `", service.version to "1.0.0-test", and deployment.environment.name to "` + k8sEnvironmentName + `".
` + k8sPromptCommonRequirements,
		Timeout:          kindScenarioTimeout,
		TelemetryTimeout: kindTelemetryTimeout,
		Assert:           assertDownwardAPIResources(K8sServiceName, k8sContainerName),
	}
}

// K8sOTelOperator is the OpenTelemetry operator scenario. The harness
// installs the operator Helm chart with admissionWebhooks.certManager
// disabled and autoGenerateCert enabled (no cert-manager on the eval
// cluster); the agent authors the OpenTelemetryCollector and Instrumentation
// custom resources and annotates the seed Node.js workload for
// auto-instrumentation.
func K8sOTelOperator() harness.Scenario {
	return harness.Scenario{
		ID:           K8sOTelOperatorID,
		Skill:        harness.SkillCollector,
		RuleFiles:    []string{"skills/otel-collector/rules/deployment/opentelemetry-operator.md"},
		RequiresKind: true,
		FixturePath:  "evals/fixtures/k8s/operator-workspace",
		Prompt: `The current directory contains Kubernetes manifests for an uninstrumented Node.js checkout service (app-deployment.yaml, app-service.yaml, downstream.yaml). The OpenTelemetry Operator is already installed in the cluster. Author the operator custom resources per the otel-collector skill (dash0-agent-skills:otel-collector) so the service is auto-instrumented and its telemetry is exported.

Deployment goals:
- Author an OpenTelemetryCollector resource named "otel" in deployment mode whose config receives OTLP (gRPC and HTTP) and exports over OTLP/HTTP to ${env:EVAL_OTLP_ENDPOINT} with an "Authorization: Bearer ${env:EVAL_OTLP_TOKEN}" header; provide EVAL_OTLP_ENDPOINT and EVAL_OTLP_TOKEN to the Collector container from the "eval-otlp" Secret. Include a memory_limiter processor in every pipeline.
- Author an Instrumentation resource for Node.js with a version-pinned auto-instrumentation image (not "latest"), exporting to the "otel" Collector's OTLP/HTTP service endpoint (http://otel-collector:4318).
- Annotate the checkout Deployment's pod template (not the Deployment metadata) for Node.js auto-instrumentation injection.
` + k8sPromptCommonRequirements,
		Timeout:          kindScenarioTimeout,
		TelemetryTimeout: kindTelemetryTimeout,
		Assert:           assertServerCheckoutSpan(""),
	}
}

// K8sHelmChart is the Collector Helm chart scenario: the agent authors
// values.yaml, the harness installs open-telemetry/opentelemetry-collector
// with it and points a pre-instrumented workload at the chart's OTLP
// service. Telemetry at the sink proves the agent-authored pipeline works
// end to end.
func K8sHelmChart() harness.Scenario {
	return harness.Scenario{
		ID:           K8sHelmChartID,
		Skill:        harness.SkillCollector,
		RuleFiles:    []string{"skills/otel-collector/rules/deployment/collector-helm-chart.md"},
		RequiresKind: true,
		FixturePath:  "evals/fixtures/k8s/helm-workspace",
		Prompt: `Author OpenTelemetry Collector Helm chart values in values.yaml in the current directory, per the otel-collector skill (dash0-agent-skills:otel-collector). The harness installs the chart with:

  helm install eval-collector open-telemetry/opentelemetry-collector -f values.yaml

and deploys an instrumented application that exports OTLP/HTTP to the chart's Collector service.

Deployment goals:
- Gateway configuration: mode "deployment" with the Kubernetes Collector distribution image and matching command name.
- The Collector config receives OTLP (gRPC and HTTP on 0.0.0.0) and exports over OTLP/HTTP to ${env:EVAL_OTLP_ENDPOINT} with an "Authorization: Bearer ${env:EVAL_OTLP_TOKEN}" header.
- Provide EVAL_OTLP_ENDPOINT and EVAL_OTLP_TOKEN to the Collector container from the "eval-otlp" Secret via extraEnvsFrom.
- Include a memory_limiter processor in every pipeline, sized against the container memory limit.
` + k8sPromptCommonRequirements,
		Timeout:          kindScenarioTimeout,
		TelemetryTimeout: kindTelemetryTimeout,
		Assert:           assertServerCheckoutSpan(K8sServiceName),
	}
}

// K8sRawManifests is the raw-manifest deployment scenario: the agent authors
// the ConfigMap, Deployment, Service, and any RBAC by hand; the harness
// applies the workspace and points a pre-instrumented workload at the
// agent's Service.
func K8sRawManifests() harness.Scenario {
	return harness.Scenario{
		ID:           K8sRawManifestsID,
		Skill:        harness.SkillCollector,
		RuleFiles:    []string{"skills/otel-collector/rules/deployment/raw-manifests.md"},
		RequiresKind: true,
		FixturePath:  "evals/fixtures/k8s/raw-manifests-workspace",
		Prompt: `Author raw Kubernetes manifests for an OpenTelemetry Collector gateway in the current directory (start from collector.yaml; add files as needed), per the otel-collector skill (dash0-agent-skills:otel-collector). The harness applies every YAML file in this directory into one namespace and deploys an instrumented application that exports OTLP/HTTP to your Collector Service.

Deployment goals:
- A Deployment running a version-pinned Collector image, its configuration mounted from a ConfigMap.
- A Service named "otel-collector" exposing the OTLP gRPC (4317) and OTLP/HTTP (4318) ports.
- The Collector config receives OTLP (gRPC and HTTP on 0.0.0.0) and exports over OTLP/HTTP to ${env:EVAL_OTLP_ENDPOINT} with an "Authorization: Bearer ${env:EVAL_OTLP_TOKEN}" header; provide both variables to the Collector container from the "eval-otlp" Secret.
- Include a memory_limiter processor in every pipeline, and health-check based liveness and readiness probes.
` + k8sPromptCommonRequirements,
		Timeout:          kindScenarioTimeout,
		TelemetryTimeout: kindTelemetryTimeout,
		Assert:           assertServerCheckoutSpan(K8sServiceName),
	}
}

// K8sDash0Operator is the Dash0 operator scenario. The harness installs the
// dash0-operator Helm chart with a runtime-generated placeholder token
// (never a real Dash0 token, never committed) and the relay as the export
// endpoint; the agent authors the Dash0Monitoring resource. The assertion
// targets the operator's export attempts at the relay, not a live Dash0
// backend.
//
// The evals-spike workflow (job 2) must confirm this operator behavior
// before the scenario is trusted: that the operator's collectors start and
// attempt OTLP export with a placeholder token against a plaintext (http://)
// endpoint, and that auto-instrumented workload telemetry reaches the
// export path.
func K8sDash0Operator() harness.Scenario {
	return harness.Scenario{
		ID:           K8sDash0OperatorID,
		Skill:        harness.SkillCollector,
		RuleFiles:    []string{"skills/otel-collector/rules/deployment/dash0-operator.md"},
		RequiresKind: true,
		FixturePath:  "evals/fixtures/k8s/dash0-workspace",
		Prompt: `The current directory contains Kubernetes manifests for an uninstrumented Node.js checkout service (app-deployment.yaml, app-service.yaml, downstream.yaml). The Dash0 Kubernetes Operator is already installed in the cluster with export configured. Enable monitoring for the target namespace per the otel-collector skill (dash0-agent-skills:otel-collector).

Deployment goals:
- Author the Dash0Monitoring resource in monitoring.yaml so all workloads in the namespace are instrumented, including the already-deployed checkout service.
` + k8sPromptCommonRequirements,
		Timeout:          kindScenarioTimeout,
		TelemetryTimeout: kindTelemetryTimeout,
		Assert:           assertServerCheckoutSpan(""),
	}
}

// --- Kubernetes assertions ---

// assertDownwardAPIResources builds the k8s-platform-downward-api assertion:
// spans under serviceName must carry the downward-API resource attributes
// platforms/k8s.md teaches, with service.instance.id derived from the pod
// UID and k8s.container.name set as a literal.
func assertDownwardAPIResources(serviceName, containerName string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		all := sink.Traces(t)
		svc := all.WithResourceAttribute("service.name", serviceName)
		if svc.Len() == 0 {
			return fmt.Errorf("no spans with resource attribute service.name=%q at the sink (%d spans total)", serviceName, all.Len())
		}
		if !hasHTTPSpan(svc, tracepb.Span_SPAN_KIND_SERVER, "GET", "/checkout") {
			return fmt.Errorf("no SERVER span for GET /checkout with service.name=%q (span names: %v)", serviceName, svc.Names())
		}

		res := svc.Spans()[0].Resource.GetAttributes()
		podUID := attrValue(res, "k8s.pod.uid")
		if podUID == "" {
			return fmt.Errorf("resource attribute k8s.pod.uid is missing or empty")
		}
		for _, key := range []string{"k8s.pod.name", "k8s.node.name"} {
			if attrValue(res, key) == "" {
				return fmt.Errorf("resource attribute %s is missing or empty", key)
			}
		}
		if !strings.HasPrefix(attrValue(res, "k8s.pod.name"), containerName) {
			return fmt.Errorf("k8s.pod.name %q does not look like a pod of the %q Deployment", attrValue(res, "k8s.pod.name"), containerName)
		}
		if got := attrValue(res, "k8s.container.name"); got != containerName {
			return fmt.Errorf("k8s.container.name is %q, want the literal container name %q", got, containerName)
		}
		if got := attrValue(res, "service.instance.id"); got != podUID {
			return fmt.Errorf("service.instance.id %q is not derived from the pod UID %q", got, podUID)
		}
		if got := attrValue(res, "service.namespace"); got != k8sServiceNamespace {
			return fmt.Errorf("service.namespace is %q, want %q", got, k8sServiceNamespace)
		}
		if got := attrValue(res, "deployment.environment.name"); got != k8sEnvironmentName {
			return fmt.Errorf("deployment.environment.name is %q, want %q", got, k8sEnvironmentName)
		}
		return nil
	}
}

// assertServerCheckoutSpan builds the assertion of the Collector deployment
// scenarios: a SERVER span for GET /checkout must reach the sink through the
// agent-deployed export path. serviceName narrows the check to one
// service.name; the empty string accepts any service (auto-instrumentation
// derives the name from the workload).
func assertServerCheckoutSpan(serviceName string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		view := sink.Traces(t)
		if serviceName != "" {
			view = view.WithResourceAttribute("service.name", serviceName)
			if view.Len() == 0 {
				return fmt.Errorf("no spans with resource attribute service.name=%q at the sink (%d spans total)", serviceName, sink.Traces(t).Len())
			}
		}
		if view.Len() == 0 {
			return fmt.Errorf("no spans at the sink")
		}
		if !hasHTTPSpan(view, tracepb.Span_SPAN_KIND_SERVER, "GET", "/checkout") {
			return fmt.Errorf("no SERVER span for GET /checkout at the sink (span names: %v)", view.Names())
		}
		return nil
	}
}
