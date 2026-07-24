// Per-scenario fixture hooks for the Kubernetes eval scenarios: how each
// kind scenario builds its images and turns the (agent-edited) workspace
// into running cluster resources. Cluster and bridge plumbing lives in
// kind.go; scenario declarations in kubernetes.go.
package scenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

// Pinned Helm chart versions for the harness-installed components. Bumping a
// pin only affects the kind scenarios; the Collector and operator images the
// charts deploy are pinned by the charts themselves.
const (
	// otelCollectorChartVersion pins open-telemetry/opentelemetry-collector
	// for the k8s-deploy-helm-chart scenario.
	otelCollectorChartVersion = "0.165.0"
	// otelOperatorChartVersion pins open-telemetry/opentelemetry-operator
	// for the k8s-deploy-otel-operator scenario.
	otelOperatorChartVersion = "0.120.0"
	// dash0OperatorChartVersion pins dash0-operator/dash0-operator for the
	// k8s-deploy-dash0-operator scenario.
	dash0OperatorChartVersion = "0.149.0"
)

// Cluster-facing names the hooks rely on.
const (
	// helmCollectorService is the OTLP Service created by the Collector
	// Helm chart for release "eval-collector" in deployment mode.
	helmCollectorService = "eval-collector-opentelemetry-collector"
	// rawCollectorService is the Service name the raw-manifests prompt
	// requires from the agent.
	rawCollectorService = "otel-collector"
	// operatorCollectorService is the Service the OpenTelemetry operator
	// creates for the OpenTelemetryCollector resource named "otel" the
	// operator prompt requires.
	operatorCollectorService = "otel-collector"
	// checkoutService is the Service name of the seed checkout workloads
	// and of the harness-deployed workload.
	checkoutService = "checkout-service"
	// dash0SystemNamespace is where the Dash0 operator Helm chart installs.
	dash0SystemNamespace = "dash0-system"
	// otelOperatorNamespace is where the OpenTelemetry operator Helm chart
	// installs (once per cluster, reused across attempts).
	otelOperatorNamespace = "opentelemetry-operator-system"
)

// deploymentTimeout bounds waiting for Deployments to become available.
const deploymentTimeout = 4 * time.Minute

// operatorResourceAttrsPlaceholder is the token the operator seed workload
// (evals/custom/fixtures/k8s/operator-workspace/app-deployment.yaml) carries as the
// plain OTEL_RESOURCE_ATTRIBUTES value. The operator scenario substitutes the
// per-run value (carrying test.id) for it at apply time; it must match the
// literal in that seed manifest.
const operatorResourceAttrsPlaceholder = "__EVAL_RESOURCE_ATTRIBUTES__"

// KindFixture implements the harness FixtureHooks contract for one
// Kubernetes scenario against a shared kind cluster. Every attempt gets its
// own namespace, runtime Secrets, and relay bridge (see kind.go); cluster
// resources of failed attempts are cleaned up when the test finishes.
type KindFixture struct {
	t          testingT
	cluster    *KindCluster
	repoRoot   string
	evalsDir   string
	scenarioID string
	spec       KindScenarioSpec

	mu       sync.Mutex
	cleanups []func()
}

// NewKindFixture returns a KindFixture for the scenario, cleaned up when the
// test finishes. It fails the test when the scenario ID has no registered
// KindScenarioSpec.
func NewKindFixture(t testingT, cluster *KindCluster, repoRoot, scenarioID string) *KindFixture {
	t.Helper()
	spec, ok := kindScenarioSpecs[scenarioID]
	if !ok {
		t.Fatalf("kind fixture: unknown scenario %q", scenarioID)
	}
	f := &KindFixture{
		t:          t,
		cluster:    cluster,
		repoRoot:   repoRoot,
		evalsDir:   filepath.Join(repoRoot, "evals", "custom"),
		scenarioID: scenarioID,
		spec:       spec,
	}
	t.Cleanup(f.Close)
	return f
}

// Hooks returns the harness fixture hooks backed by this KindFixture.
func (f *KindFixture) Hooks() harness.FixtureHooks {
	return harness.FixtureHooks{Build: f.build, Run: f.run}
}

// Close removes every cluster and Docker resource created so far, in
// reverse creation order.
func (f *KindFixture) Close() {
	f.mu.Lock()
	cleanups := f.cleanups
	f.cleanups = nil
	f.mu.Unlock()
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
}

func (f *KindFixture) addCleanup(fn func()) {
	// Debugging escape hatch: when EVAL_KEEP_CLUSTER is set, register no
	// teardown, so the relay, namespace, and any installed operator stay up
	// for inspection after the scenario (the kind cluster itself is never
	// deleted by EnsureKindCluster regardless). run() logs the handles.
	if keepClusterAlive() {
		return
	}
	f.mu.Lock()
	f.cleanups = append(f.cleanups, fn)
	f.mu.Unlock()
}

// keepClusterAlive reports whether EVAL_KEEP_CLUSTER asks the kind fixture to
// leave its resources running after the scenario for debugging.
func keepClusterAlive() bool {
	return strings.TrimSpace(os.Getenv("EVAL_KEEP_CLUSTER")) != ""
}

// build creates the images the scenario's workloads reference and loads them
// into the kind cluster. The workspace content plays no role here: the
// agent edits manifests and values, not the workload images.
func (f *KindFixture) build(ctx context.Context, _ string, _ map[string]string) error {
	images := map[string]string{
		helperImage: "", // built from the evals module, see below
	}
	for image, relDir := range f.spec.Images {
		images[image] = filepath.Join(f.evalsDir, filepath.FromSlash(relDir))
	}
	for image, dir := range images {
		var out string
		var err error
		if image == helperImage {
			out, err = docker(ctx, "build", "-f", filepath.Join(f.evalsDir, "cmd", "evalhelper", "Dockerfile"), "-t", image, f.evalsDir)
		} else {
			out, err = docker(ctx, "build", "-t", image, dir)
		}
		if err != nil {
			return fmt.Errorf("docker build %s: %w\n%s", image, err, out)
		}
		if err := f.cluster.LoadImage(ctx, image); err != nil {
			return err
		}
	}
	return nil
}

// run starts the telemetry bridge for the attempt, creates the namespace and
// runtime Secrets, and dispatches to the scenario-specific deployment.
func (f *KindFixture) run(ctx context.Context, workdir string, env map[string]string) error {
	relay, err := startKindRelay(ctx, f.evalsDir, env[harness.EnvSinkDir], env[harness.EnvOTLPToken])
	if err != nil {
		return err
	}
	f.addCleanup(relay.Close)

	namespace := "eval-" + randomSuffix()
	if err := createRunSecrets(ctx, f.cluster, namespace, runSecretEnv(relay, env)); err != nil {
		return err
	}
	f.addCleanup(func() {
		_, _ = f.cluster.Kubectl(context.Background(), "delete", "namespace", namespace, "--ignore-not-found", "--wait=false")
	})

	if keepClusterAlive() {
		f.t.Logf("EVAL_KEEP_CLUSTER set — leaving resources up for debugging:\n"+
			"  cluster=%s kubeconfig=%s\n"+
			"  namespace=%s\n"+
			"  relay container=%s ip=%s http=%s grpc=%s\n"+
			"  inspect: KUBECONFIG=%s kubectl -n %s get all,opentelemetrycollectors,instrumentations",
			f.cluster.Name, f.cluster.Kubeconfig, namespace,
			relay.containerName, relay.IP, relay.HTTPEndpoint, relay.GRPCEndpoint,
			f.cluster.Kubeconfig, namespace)
	}

	return f.spec.Run(f, ctx, relay, namespace, workdir, env[harness.EnvOTLPToken], env["OTEL_RESOURCE_ATTRIBUTES"])
}

// runDownwardAPI applies the agent-edited workspace (the pre-instrumented
// workload whose pod spec the agent configured) and drives traffic at it.
func (f *KindFixture) runDownwardAPI(ctx context.Context, namespace, workdir string) error {
	if err := applyWorkspace(ctx, f.cluster, namespace, workdir); err != nil {
		return err
	}
	if err := waitForDeployments(ctx, f.cluster, namespace, deploymentTimeout); err != nil {
		return err
	}
	return runProbe(ctx, f.cluster, namespace, checkoutURL(checkoutService), 3)
}

// runHelmChart installs the Collector chart with the agent-authored values,
// then deploys the harness workload pointed at the chart's OTLP Service.
func (f *KindFixture) runHelmChart(ctx context.Context, namespace, workdir string) error {
	if err := f.ensureHelmRepo(ctx, "open-telemetry", "https://open-telemetry.github.io/opentelemetry-helm-charts"); err != nil {
		return err
	}
	if out, err := f.cluster.Helm(ctx, "install", "eval-collector", "open-telemetry/opentelemetry-collector",
		"--version", otelCollectorChartVersion,
		"--namespace", namespace,
		"-f", filepath.Join(workdir, "values.yaml"),
		"--wait", "--timeout", "5m"); err != nil {
		return fmt.Errorf("helm install of the agent-authored values failed: %w\n%s", err, out)
	}
	return f.deployAndProbeWorkload(ctx, namespace, "http://"+helmCollectorService+":4318")
}

// runRawManifests applies the agent-authored Collector manifests, then
// deploys the harness workload pointed at the agent's Service.
func (f *KindFixture) runRawManifests(ctx context.Context, namespace, workdir string) error {
	if err := applyWorkspace(ctx, f.cluster, namespace, workdir); err != nil {
		return err
	}
	if err := waitForDeployments(ctx, f.cluster, namespace, deploymentTimeout); err != nil {
		return err
	}
	return f.deployAndProbeWorkload(ctx, namespace, "http://"+rawCollectorService+":4318")
}

// runOTelOperator ensures the operator is installed (without cert-manager:
// the chart's self-generated webhook certificate replaces it on the eval
// cluster), applies the agent-authored CRs plus the seed workload, and
// restarts the workload so the injection webhook sees pods created after
// the Instrumentation resource exists.
func (f *KindFixture) runOTelOperator(ctx context.Context, namespace, workdir, resourceAttrs string) error {
	if err := f.ensureHelmRepo(ctx, "open-telemetry", "https://open-telemetry.github.io/opentelemetry-helm-charts"); err != nil {
		return err
	}
	if _, err := f.cluster.Helm(ctx, "status", "opentelemetry-operator", "--namespace", otelOperatorNamespace); err != nil {
		if out, err := f.cluster.Helm(ctx, "install", "opentelemetry-operator", "open-telemetry/opentelemetry-operator",
			"--version", otelOperatorChartVersion,
			"--namespace", otelOperatorNamespace, "--create-namespace",
			"--set", "admissionWebhooks.certManager.enabled=false",
			"--set", "admissionWebhooks.autoGenerateCert.enabled=true",
			"--wait", "--timeout", "5m"); err != nil {
			return fmt.Errorf("helm install opentelemetry-operator: %w\n%s", err, out)
		}
	}
	// Substitute the per-run OTEL_RESOURCE_ATTRIBUTES value (carrying test.id)
	// for the seed placeholder: the seed keeps it as a plain value, not a
	// valueFrom, so the operator's auto-instrumentation can merge its k8s.*
	// attributes into it without producing an invalid value+valueFrom env.
	if err := applyWorkspaceWithSubst(ctx, f.cluster, namespace, workdir,
		map[string]string{operatorResourceAttrsPlaceholder: resourceAttrs}); err != nil {
		return err
	}
	// The operator reconciles the OpenTelemetryCollector resource into a
	// Deployment asynchronously; wait for it to appear before waiting on
	// availability.
	if err := f.waitForDeploymentExists(ctx, namespace, operatorCollectorService, deploymentTimeout); err != nil {
		return err
	}
	if err := waitForDeployments(ctx, f.cluster, namespace, deploymentTimeout); err != nil {
		return err
	}
	// The injection webhook skips pods created before the operator's informer
	// cache has synced the just-applied Instrumentation resource ("no
	// OpenTelemetry Instrumentation instances available"). Wait until the
	// resource is present in the namespace before recreating the workload.
	if err := f.waitForInstrumentation(ctx, namespace, deploymentTimeout); err != nil {
		return err
	}
	// Recreate the workload pods now that the Instrumentation resource
	// exists: the mutating webhook only injects at pod creation, and the
	// apply order inside the workspace is not guaranteed.
	if out, err := f.cluster.Kubectl(ctx, "-n", namespace, "rollout", "restart", "deployment", checkoutService); err != nil {
		return fmt.Errorf("rollout restart %s: %w\n%s", checkoutService, err, out)
	}
	if err := waitForDeployments(ctx, f.cluster, namespace, deploymentTimeout); err != nil {
		return f.wrapWorkloadFailure(ctx, namespace, err)
	}
	// Fail precisely if the operator did not inject instrumentation, rather
	// than letting the run drift into a silent telemetry timeout.
	if err := f.verifyInstrumentationInjected(ctx, namespace); err != nil {
		return err
	}
	return runProbe(ctx, f.cluster, namespace, checkoutURL(checkoutService), 3)
}

// waitForInstrumentation polls until at least one Instrumentation resource
// exists in the namespace, so the operator's injection webhook has it in its
// informer cache before the workload pods are recreated.
func (f *KindFixture) waitForInstrumentation(ctx context.Context, namespace string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		out, err := f.cluster.Kubectl(ctx, "-n", namespace, "get", "instrumentation",
			"-o", "jsonpath={.items[*].metadata.name}")
		if err == nil && strings.TrimSpace(out) != "" {
			return nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return fmt.Errorf("no Instrumentation resource appeared in %s within %s; the agent must author an Instrumentation resource", namespace, timeout)
		}
		time.Sleep(2 * time.Second)
	}
}

// verifyInstrumentationInjected confirms the operator mutated the workload
// pod: auto-instrumentation adds an init container (opentelemetry-auto-
// instrumentation-*) that copies the SDK into the app container. Its absence
// means the pod is running uninstrumented and no telemetry will flow, so this
// returns a precise error instead of deferring to the telemetry timeout.
func (f *KindFixture) verifyInstrumentationInjected(ctx context.Context, namespace string) error {
	out, err := f.cluster.Kubectl(ctx, "-n", namespace, "get", "pods",
		"-l", "app.kubernetes.io/name="+checkoutService,
		"-o", "jsonpath={range .items[*]}{.metadata.name}{\"|\"}{range .spec.initContainers[*]}{.name}{\" \"}{end}{\"\\n\"}{end}")
	if err != nil {
		return fmt.Errorf("inspect workload pods for injected init container: %w\n%s", err, out)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.Contains(line, "opentelemetry-auto-instrumentation") {
			return nil
		}
	}
	return fmt.Errorf("operator did not inject instrumentation into the %s workload (opentelemetry-auto-instrumentation init container absent); the pod is uninstrumented so no telemetry will flow.\npod init containers:\n%s", checkoutService, out)
}

// wrapWorkloadFailure augments a deployment-availability error with a
// FailedCreate signal on the workload's ReplicaSets, the symptom of the pod
// being rejected by the injection webhook (invalid value+valueFrom env).
func (f *KindFixture) wrapWorkloadFailure(ctx context.Context, namespace string, cause error) error {
	events, evErr := f.cluster.Kubectl(context.Background(), "-n", namespace, "get", "events",
		"--field-selector", "reason=FailedCreate", "-o", "jsonpath={range .items[*]}{.message}{\"\\n\"}{end}")
	if evErr == nil && strings.TrimSpace(events) != "" {
		return fmt.Errorf("%w\nworkload FailedCreate events (pod likely rejected by the injection webhook):\n%s", cause, events)
	}
	_ = ctx
	return cause
}

// runDash0Operator installs the Dash0 operator with a runtime placeholder
// token (the per-run relay bearer token, so the operator's export attempts
// authenticate at the relay and become observable at the sink) and the relay
// as export endpoint, then applies the agent-authored Dash0Monitoring plus
// the seed workload.
//
// The endpoint uses an explicit http:// scheme because the relay speaks
// plaintext gRPC. The evals-spike workflow (job 2) must confirm this
// operator behavior — plaintext export target plus placeholder token still
// produces export attempts at the relay — before this scenario is trusted.
func (f *KindFixture) runDash0Operator(ctx context.Context, relay *kindRelay, namespace, workdir, token string) error {
	if err := f.ensureHelmRepo(ctx, "dash0-operator", "https://dash0hq.github.io/dash0-operator"); err != nil {
		return err
	}
	if out, err := f.cluster.Kubectl(ctx, "create", "namespace", dash0SystemNamespace); err != nil && !strings.Contains(out, "already exists") {
		return fmt.Errorf("create namespace %s: %w\n%s", dash0SystemNamespace, err, out)
	}
	// The placeholder token travels via an env file, never via command line
	// or committed manifests.
	tokenFile, err := writeEnvFile(map[string]string{"auth-token": token})
	if err != nil {
		return err
	}
	defer os.Remove(tokenFile)
	_, _ = f.cluster.Kubectl(ctx, "-n", dash0SystemNamespace, "delete", "secret", "dash0-credentials", "--ignore-not-found")
	if out, err := f.cluster.Kubectl(ctx, "-n", dash0SystemNamespace, "create", "secret", "generic", "dash0-credentials",
		"--from-env-file", tokenFile); err != nil {
		return fmt.Errorf("create secret dash0-credentials: %w\n%s", err, out)
	}

	// helm upgrade --install: the export endpoint (the relay's kind-network
	// address) changes on every attempt.
	if out, err := f.cluster.Helm(ctx, "upgrade", "--install", "dash0-operator", "dash0-operator/dash0-operator",
		"--version", dash0OperatorChartVersion,
		"--namespace", dash0SystemNamespace,
		"--set", "operator.dash0Export.enabled=true",
		"--set", "operator.dash0Export.endpoint=http://"+relay.GRPCEndpoint,
		"--set", "operator.dash0Export.secretRef.name=dash0-credentials",
		"--set", "operator.dash0Export.secretRef.key=auth-token",
		"--set", "operator.clusterName=agent-skills-evals",
		"--wait", "--timeout", "5m"); err != nil {
		return fmt.Errorf("helm install dash0-operator: %w\n%s", err, out)
	}
	f.addCleanup(func() {
		_, _ = f.cluster.Helm(context.Background(), "uninstall", "dash0-operator", "--namespace", dash0SystemNamespace)
	})

	if err := applyWorkspace(ctx, f.cluster, namespace, workdir); err != nil {
		return err
	}
	if err := waitForDeployments(ctx, f.cluster, namespace, deploymentTimeout); err != nil {
		return err
	}
	return runProbe(ctx, f.cluster, namespace, checkoutURL(checkoutService), 3)
}

// deployAndProbeWorkload deploys the pre-instrumented harness workload
// pointed at the given in-cluster OTLP/HTTP endpoint (the agent-deployed
// Collector) and drives traffic at it. Reaching the sink then proves the
// full workload-to-Collector-to-relay path.
func (f *KindFixture) deployAndProbeWorkload(ctx context.Context, namespace, otlpEndpoint string) error {
	manifest := fmt.Sprintf(workloadManifestTemplate, checkoutService, checkoutService, checkoutService, checkoutService,
		instrumentedGoImage, otlpEndpoint, K8sServiceName, runSecretName, checkoutService, checkoutService, checkoutService)
	file, err := os.CreateTemp("", "eval-workload-*.yaml")
	if err != nil {
		return fmt.Errorf("write workload manifest: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(manifest); err != nil {
		_ = file.Close()
		return fmt.Errorf("write workload manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("write workload manifest: %w", err)
	}
	if out, err := f.cluster.Kubectl(ctx, "-n", namespace, "apply", "-f", file.Name()); err != nil {
		return fmt.Errorf("apply harness workload: %w\n%s", err, out)
	}
	if err := waitForDeployments(ctx, f.cluster, namespace, deploymentTimeout); err != nil {
		return err
	}
	return runProbe(ctx, f.cluster, namespace, checkoutURL(checkoutService), 3)
}

// waitForDeploymentExists polls until the named Deployment exists in the
// namespace (for resources created asynchronously by operators).
func (f *KindFixture) waitForDeploymentExists(ctx context.Context, namespace, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := f.cluster.Kubectl(ctx, "-n", namespace, "get", "deployment", name); err == nil {
			return nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			diag, _ := f.cluster.Kubectl(context.Background(), "-n", namespace, "get", "all")
			return fmt.Errorf("deployment %s/%s did not appear within %s\nnamespace content:\n%s", namespace, name, timeout, diag)
		}
		time.Sleep(2 * time.Second)
	}
}

// ensureHelmRepo adds (or refreshes) a Helm repository.
func (f *KindFixture) ensureHelmRepo(ctx context.Context, name, url string) error {
	if out, err := f.cluster.Helm(ctx, "repo", "add", name, url, "--force-update"); err != nil {
		return fmt.Errorf("helm repo add %s: %w\n%s", name, err, out)
	}
	if out, err := f.cluster.Helm(ctx, "repo", "update", name); err != nil {
		return fmt.Errorf("helm repo update %s: %w\n%s", name, err, out)
	}
	return nil
}

// checkoutURL is the in-cluster URL the traffic probe targets.
func checkoutURL(service string) string {
	return fmt.Sprintf("http://%s:%s/checkout", service, appPort)
}

// workloadManifestTemplate is the harness-owned pre-instrumented workload
// deployed behind agent-authored Collectors. Exporter endpoint and identity
// are parameters; the per-run test.id rides in via the eval-otlp Secret
// (EVAL_EXTRA_RESOURCE_ATTRIBUTES), so no token or endpoint literal is ever
// rendered into the manifest beyond the in-cluster Collector Service URL.
const workloadManifestTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  labels:
    app.kubernetes.io/name: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: %s
  template:
    metadata:
      labels:
        app.kubernetes.io/name: %s
    spec:
      containers:
        - name: checkout-service
          image: %s
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 8080
          env:
            - name: OTEL_EXPORTER_OTLP_ENDPOINT
              value: %s
            - name: OTEL_EXPORTER_OTLP_PROTOCOL
              value: http/protobuf
            - name: OTEL_SERVICE_NAME
              value: %s
            - name: EVAL_EXTRA_RESOURCE_ATTRIBUTES
              valueFrom:
                secretKeyRef:
                  name: %s
                  key: EVAL_EXTRA_RESOURCE_ATTRIBUTES
---
apiVersion: v1
kind: Service
metadata:
  name: %s
  labels:
    app.kubernetes.io/name: %s
spec:
  selector:
    app.kubernetes.io/name: %s
  ports:
    - name: http
      port: 8080
      targetPort: http
`
