package scenarios

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/custom/examples"
	"github.com/dash0hq/agent-skills/evals/custom/harness"

	"github.com/dash0hq/agent-skills/evals/custom/internal/testutil"
)

// --- hermetic tests (no Docker, no kind, no agent) ---

// kubernetesScenarioRuleFiles maps every kind scenario to its dedicated rule
// file, in registration order.
var kubernetesScenarioRuleFiles = map[string]string{
	K8sDownwardAPIID:   "skills/otel-instrumentation/rules/platforms/k8s.md",
	K8sOTelOperatorID:  "skills/otel-collector/rules/deployment/opentelemetry-operator.md",
	K8sHelmChartID:     "skills/otel-collector/rules/deployment/collector-helm-chart.md",
	K8sRawManifestsID:  "skills/otel-collector/rules/deployment/raw-manifests.md",
	K8sDash0OperatorID: "skills/otel-collector/rules/deployment/dash0-operator.md",
}

func TestKubernetesScenarioDeclarations(t *testing.T) {
	root := testutil.RepoRoot(t)
	byID := map[string]harness.Scenario{}
	for _, sc := range Default().Scenarios() {
		byID[sc.ID] = sc
	}

	for id, ruleFile := range kubernetesScenarioRuleFiles {
		sc, ok := byID[id]
		require.True(t, ok, "scenario %s must be registered in the default registry", id)
		require.True(t, sc.RequiresKind, "%s must be marked RequiresKind", id)
		require.False(t, sc.Smoke, "%s must not join the smoke set", id)
		require.False(t, sc.FullMatrixOnly, "%s must stay selectable through its rule file", id)
		require.Equal(t, []string{ruleFile}, sc.RuleFiles, "rule files of %s", id)
		require.NotNil(t, sc.Assert, "%s must declare an assertion", id)
		require.NotEmpty(t, sc.FixturePath, "%s must declare a fixture workspace", id)
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(sc.FixturePath)))
		require.NoError(t, err, "fixture path of %s", id)

		spec, ok := kindScenarioSpecs[id]
		require.True(t, ok, "%s must have a KindScenarioSpec", id)
		require.NotNil(t, spec.Run, "%s spec must declare a Run hook", id)
		require.NotEmpty(t, spec.Images, "%s spec must declare at least one workload image", id)
	}
}

func TestKubernetesSelection(t *testing.T) {
	reg := Default()

	tests := []struct {
		name    string
		changed []string
		want    []string
	}{
		{
			name:    "k8s platform rule file selects only the downward-API scenario",
			changed: []string{"skills/otel-instrumentation/rules/platforms/k8s.md"},
			want:    []string{K8sDownwardAPIID},
		},
		{
			name:    "helm chart rule file selects only the Helm scenario",
			changed: []string{"skills/otel-collector/rules/deployment/collector-helm-chart.md"},
			want:    []string{K8sHelmChartID},
		},
		{
			name:    "dash0 operator rule file selects only the Dash0 scenario",
			changed: []string{"skills/otel-collector/rules/deployment/dash0-operator.md"},
			want:    []string{K8sDash0OperatorID},
		},
		{
			name:    "k8s fixture change selects the owning scenario",
			changed: []string{"evals/custom/fixtures/k8s/raw-manifests-workspace/collector.yaml"},
			want:    []string{K8sRawManifestsID},
		},
		{
			// Skill-wide selection excludes RequiresKind scenarios so
			// ordinary PR runs stay light; the non-kind collector scenario
			// from U5 is still selected.
			name:    "shared collector rule file selects no kind scenarios",
			changed: []string{"skills/otel-collector/rules/processors.md"},
			want:    []string{CollectorPipelineHardeningID},
		},
		{
			name:    "collector SKILL.md selects no kind scenarios",
			changed: []string{"skills/otel-collector/SKILL.md"},
			want:    []string{CollectorPipelineHardeningID},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, scenarioIDs(reg.Select(tt.changed)))
		})
	}
}

// TestRunSecretEnvRetargetsAtTheKindRelay covers the runtime token-injection
// composition: the Secret content points workloads at the relay's
// kind-network address and carries the per-run token, while the test.id
// resource attributes pass through unchanged.
func TestRunSecretEnvRetargetsAtTheKindRelay(t *testing.T) {
	relay := &kindRelay{
		IP:           "172.19.0.9",
		HTTPEndpoint: "http://172.19.0.9:4318",
		GRPCEndpoint: "172.19.0.9:4317",
	}
	env := map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://127.0.0.1:52341",
		"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
		"OTEL_EXPORTER_OTLP_HEADERS":  "Authorization=Bearer%20tok-TEST-0001",
		"OTEL_RESOURCE_ATTRIBUTES":    otelsink.TestIDAttribute + "=TEST-run-1",
		harness.EnvOTLPEndpoint:       "http://127.0.0.1:52341",
		harness.EnvOTLPToken:          "tok-TEST-0001",
	}

	secret := runSecretEnv(relay, env)

	require.Equal(t, "http://172.19.0.9:4318", secret["EVAL_OTLP_ENDPOINT"])
	require.Equal(t, "http://172.19.0.9:4318", secret["OTEL_EXPORTER_OTLP_ENDPOINT"])
	require.Equal(t, "172.19.0.9:4317", secret["EVAL_OTLP_GRPC_ENDPOINT"])
	require.Equal(t, "tok-TEST-0001", secret["EVAL_OTLP_TOKEN"])
	require.Equal(t, "Authorization=Bearer%20tok-TEST-0001", secret["OTEL_EXPORTER_OTLP_HEADERS"])
	require.Equal(t, "test.id=TEST-run-1", secret["OTEL_RESOURCE_ATTRIBUTES"])
	require.Equal(t, "test.id=TEST-run-1", secret["EVAL_EXTRA_RESOURCE_ATTRIBUTES"])
	for key, value := range secret {
		require.NotContains(t, value, "127.0.0.1", "secret key %s must not leak the host-local endpoint", key)
	}
}

// TestK8sFixtureManifestsAreValidAndClassifyAsK8s parses every YAML file
// under evals/custom/fixtures/k8s and requires each non-empty document to classify
// as a Kubernetes manifest under the evals/custom/examples rules. Agent-authored
// seed stubs (comment-only files) are allowed.
func TestK8sFixtureManifestsAreValidAndClassifyAsK8s(t *testing.T) {
	root := testutil.RepoRoot(t)
	k8sDir := filepath.Join(root, "evals", "custom", "fixtures", "k8s")
	checked := 0
	err := filepath.WalkDir(k8sDir, func(p string, d os.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() || !isYAMLFile(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		require.NoError(t, err)
		content, err := os.ReadFile(p)
		require.NoError(t, err)
		if !hasYAMLContent(string(content)) {
			return nil // comment-only seed stub the agent fills in
		}
		block := &examples.Block{File: rel, Tag: "yaml", Content: string(content)}
		for _, doc := range block.Documents() {
			require.Equal(t, examples.CategoryK8sManifest, examples.Classify(doc),
				"%s document %d must classify as a Kubernetes manifest", rel, doc.Index)
			checked++
		}
		return nil
	})
	require.NoError(t, err)
	require.NotZero(t, checked, "no Kubernetes fixture manifests found under %s", k8sDir)
}

// tokenLikePatterns match credential material that must never appear in
// committed fixtures: bearer values, token assignments, and authorization
// headers with literal values. Secret references by name and key (for
// example secretKeyRef with key "token") carry no value and do not match.
var tokenLikePatterns = []*regexp.Regexp{
	// 16+ characters after "bearer": matches token material without tripping
	// on prose like "the per-run bearer token".
	regexp.MustCompile(`(?i)bearer[ =:]+[a-z0-9._~+/-]{16,}`),
	regexp.MustCompile(`(?i)auth[-_]?token\s*[:=]\s*["']?[a-z0-9]`),
	regexp.MustCompile(`(?i)(otlp|otel|dash0)[-_]token\s*[:=]\s*["']?[a-z0-9]`),
	regexp.MustCompile(`(?i)authorization\s*[:=]\s*["']?\S`),
	regexp.MustCompile(`EVAL_OTLP_TOKEN\s*[:=]\s*\S`),
}

// TestNoTokenMaterialInK8sFixtures is the grep-style guard from the U6
// contract: bearer tokens are injected at run time (kubectl Secrets composed
// from env files), so nothing token-like may live under evals/custom/fixtures/k8s.
func TestNoTokenMaterialInK8sFixtures(t *testing.T) {
	root := testutil.RepoRoot(t)
	k8sDir := filepath.Join(root, "evals", "custom", "fixtures", "k8s")
	err := filepath.WalkDir(k8sDir, func(p string, d os.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		require.NoError(t, relErr)
		content, readErr := os.ReadFile(p)
		require.NoError(t, readErr)
		for i, line := range strings.Split(string(content), "\n") {
			for _, re := range tokenLikePatterns {
				require.False(t, re.MatchString(line),
					"%s:%d looks like committed token material (pattern %s): %s", rel, i+1, re, strings.TrimSpace(line))
			}
		}
		return nil
	})
	require.NoError(t, err)
}

// TestSubstituteTokens covers the operator scenario's apply-time
// substitution: the placeholder is replaced with the per-run value, multiple
// occurrences across content are all replaced, and content without the token
// (or a nil substitution map) is returned unchanged.
func TestSubstituteTokens(t *testing.T) {
	value := otelsink.TestIDAttribute + "=TEST-run-1"

	t.Run("replaces the placeholder", func(t *testing.T) {
		content := "            - name: OTEL_RESOURCE_ATTRIBUTES\n              value: \"" + operatorResourceAttrsPlaceholder + "\"\n"
		got := substituteTokens(content, map[string]string{operatorResourceAttrsPlaceholder: value})
		require.NotContains(t, got, operatorResourceAttrsPlaceholder)
		require.Contains(t, got, value)
	})

	t.Run("replaces every occurrence", func(t *testing.T) {
		content := operatorResourceAttrsPlaceholder + " and " + operatorResourceAttrsPlaceholder
		got := substituteTokens(content, map[string]string{operatorResourceAttrsPlaceholder: value})
		require.Equal(t, value+" and "+value, got)
	})

	t.Run("no token leaves content unchanged", func(t *testing.T) {
		content := "apiVersion: apps/v1\nkind: Deployment\n"
		require.Equal(t, content, substituteTokens(content, map[string]string{operatorResourceAttrsPlaceholder: value}))
	})

	t.Run("nil substitutions leave content unchanged", func(t *testing.T) {
		content := operatorResourceAttrsPlaceholder
		require.Equal(t, content, substituteTokens(content, nil))
	})
}

// TestOperatorSeedCarriesResourceAttrsPlaceholder pins the contract between
// the operator seed manifest and the harness: the seed must carry the plain
// OTEL_RESOURCE_ATTRIBUTES placeholder (not a valueFrom) so the harness can
// substitute the per-run value and the operator can merge its k8s.* attrs.
func TestOperatorSeedCarriesResourceAttrsPlaceholder(t *testing.T) {
	root := testutil.RepoRoot(t)
	path := filepath.Join(root, "evals", "custom", "fixtures", "k8s", "operator-workspace", "app-deployment.yaml")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(content), operatorResourceAttrsPlaceholder,
		"operator seed must carry the %s placeholder", operatorResourceAttrsPlaceholder)
	require.NotRegexp(t, `(?s)OTEL_RESOURCE_ATTRIBUTES\s+valueFrom`, string(content),
		"operator seed OTEL_RESOURCE_ATTRIBUTES must be a plain value, not a valueFrom")
}

func TestHasYAMLContent(t *testing.T) {
	require.False(t, hasYAMLContent(""))
	require.False(t, hasYAMLContent("# only comments\n\n# more\n---\n"))
	require.True(t, hasYAMLContent("# comment\napiVersion: v1\n"))
	require.True(t, isYAMLFile("deployment.yaml"))
	require.True(t, isYAMLFile("values.YML"))
	require.False(t, isYAMLFile("Dockerfile"))
}

// --- assertion tests, fed through the sink's real OTLP endpoints ---

func downwardAPIResource(overrides map[string]string) map[string]string {
	attrs := map[string]string{
		"service.name":                K8sServiceName,
		"service.namespace":           k8sServiceNamespace,
		"service.instance.id":         "TEST-POD-UID-0001",
		"deployment.environment.name": k8sEnvironmentName,
		"k8s.pod.uid":                 "TEST-POD-UID-0001",
		"k8s.pod.name":                "checkout-service-6d4f9c-x2s8k",
		"k8s.node.name":               "agent-skills-evals-control-plane",
		"k8s.container.name":          k8sContainerName,
	}
	for k, v := range overrides {
		if v == "" {
			delete(attrs, k)
			continue
		}
		attrs[k] = v
	}
	return attrs
}

func checkoutServerSpan() spanSpec {
	return spanSpec{
		name: "GET /checkout",
		kind: tracepb.Span_SPAN_KIND_SERVER,
		attrs: map[string]string{
			"http.request.method": "GET",
			"url.path":            "/checkout",
		},
	}
}

func TestAssertDownwardAPIResourcesPasses(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, downwardAPIResource(nil), checkoutServerSpan())
	require.NoError(t, assertDownwardAPIResources(K8sServiceName, k8sContainerName)(t, sink))
}

func TestAssertDownwardAPIResourcesFailures(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		wantErr   string
	}{
		{
			name:      "missing pod uid",
			overrides: map[string]string{"k8s.pod.uid": ""},
			wantErr:   "k8s.pod.uid",
		},
		{
			name:      "instance id not derived from the pod uid",
			overrides: map[string]string{"service.instance.id": "TEST-OTHER-ID"},
			wantErr:   "service.instance.id",
		},
		{
			name:      "container name mismatch",
			overrides: map[string]string{"k8s.container.name": "sidecar"},
			wantErr:   "k8s.container.name",
		},
		{
			name:      "missing node name",
			overrides: map[string]string{"k8s.node.name": ""},
			wantErr:   "k8s.node.name",
		},
		{
			name:      "wrong service namespace",
			overrides: map[string]string{"service.namespace": "prod"},
			wantErr:   "service.namespace",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := otelsink.Start(t)
			feedSpans(t, sink, downwardAPIResource(tt.overrides), checkoutServerSpan())
			err := assertDownwardAPIResources(K8sServiceName, k8sContainerName)(t, sink)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestAssertServerCheckoutSpan(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": K8sServiceName}, checkoutServerSpan())
	require.NoError(t, assertServerCheckoutSpan(K8sServiceName)(t, sink))
	require.NoError(t, assertServerCheckoutSpan("")(t, sink))
	require.Error(t, assertServerCheckoutSpan("other-service")(t, sink))

	empty := otelsink.Start(t)
	require.Error(t, assertServerCheckoutSpan("")(t, empty))
}

// --- kind-gated tests (skip cleanly without kind, Docker, or the API key) ---

// requireKind skips the test unless the kind CLI, kubectl, and a Docker
// daemon are available.
func requireKind(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping kind cluster tests in -short mode")
	}
	if !KindAvailable() {
		t.Skip("skipping: kind, kubectl, or the Docker daemon not available")
	}
}

// ensureCluster provisions (or reuses) the eval kind cluster.
func ensureCluster(t *testing.T, root string) *KindCluster {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cluster, err := EnsureKindCluster(ctx, root, filepath.Join(t.TempDir(), "kubeconfig"))
	require.NoError(t, err)
	return cluster
}

// TestKindBridge is the U6 bridge validation without any agent: a pod in the
// kind cluster exports OTLP through the relay on the kind Docker network,
// and the host sink receives it scoped to the run's test.id. The
// evals-spike workflow runs the same route on a GitHub runner.
func TestKindBridge(t *testing.T) {
	requireKind(t)
	root := testutil.RepoRoot(t)
	evalsDir := filepath.Join(root, "evals", "custom")
	cluster := ensureCluster(t, root)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Host side: sink plus the token-guarded in-process relay.
	sink := otelsink.Start(t)
	token, err := harness.NewToken()
	require.NoError(t, err)
	hostRelay, err := harness.StartRelay(harness.RelayConfig{Upstream: sink.HTTPEndpoint(), BearerToken: token})
	require.NoError(t, err)
	t.Cleanup(hostRelay.Close)
	env := harness.FixtureEnv(sink, hostRelay, token)

	// Bridge: sink relay container on the kind Docker network, writing every
	// received export to the host sink dir (container-to-container delivery,
	// no host.docker.internal hop).
	relay, err := startKindRelay(ctx, evalsDir, env[harness.EnvSinkDir], token)
	require.NoError(t, err)
	t.Cleanup(relay.Close)

	// Workload images.
	for image, dir := range map[string]string{
		instrumentedGoImage: filepath.Join(evalsDir, "fixtures", "k8s", "instrumented-go-service"),
	} {
		out, buildErr := docker(ctx, "build", "-t", image, dir)
		require.NoError(t, buildErr, "docker build %s:\n%s", image, out)
		require.NoError(t, cluster.LoadImage(ctx, image))
	}
	out, err := docker(ctx, "build", "-f", filepath.Join(evalsDir, "cmd", "evalhelper", "Dockerfile"), "-t", helperImage, evalsDir)
	require.NoError(t, err, "docker build %s:\n%s", helperImage, out)
	require.NoError(t, cluster.LoadImage(ctx, helperImage))

	// In-cluster side: namespace, runtime Secrets, workload exporting
	// straight at the relay (no Collector in between for the bridge test).
	namespace := "eval-bridge-" + randomSuffix()
	require.NoError(t, createRunSecrets(ctx, cluster, namespace, runSecretEnv(relay, env)))
	t.Cleanup(func() {
		_, _ = cluster.Kubectl(context.Background(), "delete", "namespace", namespace, "--ignore-not-found", "--wait=false")
	})

	fix := &KindFixture{t: t, cluster: cluster, repoRoot: root, evalsDir: evalsDir, scenarioID: K8sDownwardAPIID}
	t.Cleanup(fix.Close)
	require.NoError(t, fix.deployAndProbeWorkload(ctx, namespace, relay.HTTPEndpoint))

	deadline := time.Now().Add(2 * time.Minute)
	for sink.Traces(t).Len() == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("no spans with %s=%s reached the sink through the kind bridge", otelsink.TestIDAttribute, sink.TestID())
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NotZero(t, sink.Traces(t).WithResourceAttribute("service.name", K8sServiceName).Len(),
		"bridge telemetry must carry the workload's service.name")
}

// TestKubernetesScenarios is the kind eval entrypoint: it runs every
// RequiresKind scenario end to end (real agent, real cluster). It skips
// cleanly without kind, Docker, Helm, or the API key, so plain
// `go test ./...` stays hermetic.
func TestKubernetesScenarios(t *testing.T) {
	requireKind(t)
	if !HelmAvailable() {
		t.Skip("skipping: helm not available")
	}
	if loaded, err := harness.LoadDotEnv(); err != nil {
		t.Fatalf("loading .env: %v", err)
	} else if len(loaded) > 0 {
		t.Logf("loaded %d variable(s) from .env: %s", len(loaded), strings.Join(loaded, ", "))
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("skipping: ANTHROPIC_API_KEY unset; refusing to invoke the real agent (set it in .env or the environment)")
	}

	root := testutil.RepoRoot(t)
	versions, err := harness.LoadVersions(filepath.Join(root, "evals", "custom", "versions.env"))
	require.NoError(t, err)
	binary := os.Getenv("EVAL_AGENT_BINARY")
	selected := scenarioFilter(os.Getenv("EVAL_SCENARIOS"))
	cluster := ensureCluster(t, root)

	for _, sc := range KubernetesScenarios() {
		t.Run(sc.ID, func(t *testing.T) {
			if !selected(sc.ID) {
				t.Skip("skipped by the EVAL_SCENARIOS filter")
			}
			fix := NewKindFixture(t, cluster, root, sc.ID)
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
