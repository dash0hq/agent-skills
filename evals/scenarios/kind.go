// Kind cluster support for the Kubernetes eval scenarios (U6).
//
// # Telemetry bridge
//
// kind nodes are Docker containers attached to the Docker bridge network
// named "kind". The harness runs the dual-homed OTLP relay
// (evals/cmd/relay) as a container on that same network, so in-cluster
// workloads and Collectors can reach it: pod egress to addresses on the
// node's Docker network is routed through the node like any other external
// address. Cluster DNS cannot resolve Docker network aliases, so the relay
// is addressed by its kind-network IP, discovered via docker inspect and
// injected into the cluster at run time through the eval-otlp Secret.
//
// The relay runs in sink mode: it writes every received export to a mounted
// host directory (the otelsink dir) instead of forwarding to the host, so
// delivery is container-to-container and needs no route back to the host. It
// still requires the per-run bearer token on this cluster-facing path (R21:
// in-cluster workloads can deliver telemetry only to the token-guarded relay,
// never to real backends). Writing to a mounted volume rather than reaching
// the host through host.docker.internal / host-gateway is what makes the
// bridge work on Linux CI runners, where that host hop does not.
//
// # Runtime credential injection
//
// The per-run bearer token and the relay endpoint exist in the cluster only
// as Secrets created by the harness at run time (kubectl create secret
// --from-env-file, keeping values out of command lines). Committed fixture
// manifests reference the Secrets by name and never contain token material;
// a test greps evals/fixtures/k8s to enforce that.
package scenarios

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Names shared between the kind harness code, the committed seed manifests,
// and the scenario prompts.
const (
	// kindClusterName must match the name in evals/fixtures/k8s/kind-config.yaml.
	kindClusterName = "agent-skills-evals"
	// kindNetworkName is the Docker network kind attaches its nodes to.
	kindNetworkName = "kind"
	// kindConfigPath is the cluster config, relative to the repository root.
	kindConfigPath = "evals/fixtures/k8s/kind-config.yaml"

	// runSecretName is the runtime-created Secret carrying the relay
	// endpoint, the per-run bearer token, and the OTEL_* values workloads
	// consume. Seed manifests and prompts reference it by name.
	runSecretName = "eval-otlp"
	// headerSecretName is the runtime-created Secret matching the
	// `otel-auth`/`token` reference the platforms/k8s.md pod-spec example
	// teaches for OTEL_EXPORTER_OTLP_HEADERS.
	headerSecretName = "otel-auth"

	// instrumentedGoImage is the pre-instrumented Kubernetes workload image
	// (evals/fixtures/k8s/instrumented-go-service).
	instrumentedGoImage = "instrumented-go-service-eval:local"
	// nodejsImage is the uninstrumented Node.js fixture image
	// (evals/fixtures/nodejs-service), the auto-instrumentation target.
	nodejsImage = "nodejs-service-eval:local"
)

// KindAvailable reports whether the kind CLI, the kubectl CLI, and a
// reachable Docker daemon are all present. Cluster-dependent tests skip when
// it returns false.
func KindAvailable() bool {
	for _, bin := range []string{"kind", "kubectl"} {
		if _, err := exec.LookPath(bin); err != nil {
			return false
		}
	}
	return DockerAvailable()
}

// HelmAvailable reports whether the helm CLI is present; the operator and
// Helm-chart scenarios additionally skip without it.
func HelmAvailable() bool {
	_, err := exec.LookPath("helm")
	return err == nil
}

// KindCluster is a handle to a created-or-reused kind cluster.
type KindCluster struct {
	// Name is the kind cluster name.
	Name string
	// Kubeconfig is the path of the kubeconfig scoped to this cluster.
	Kubeconfig string
}

// EnsureKindCluster creates the eval kind cluster from the committed config,
// or reuses it when a cluster of that name already exists, and writes a
// kubeconfig for it to kubeconfigPath.
func EnsureKindCluster(ctx context.Context, repoRoot, kubeconfigPath string) (*KindCluster, error) {
	c := &KindCluster{Name: kindClusterName, Kubeconfig: kubeconfigPath}
	out, err := runCmd(ctx, nil, "kind", "get", "clusters")
	if err != nil {
		return nil, fmt.Errorf("kind get clusters: %w\n%s", err, out)
	}
	for _, name := range strings.Fields(out) {
		if name == kindClusterName {
			if out, err := runCmd(ctx, nil, "kind", "export", "kubeconfig",
				"--name", kindClusterName, "--kubeconfig", kubeconfigPath); err != nil {
				return nil, fmt.Errorf("kind export kubeconfig: %w\n%s", err, out)
			}
			return c, nil
		}
	}
	if out, err := runCmd(ctx, nil, "kind", "create", "cluster",
		"--name", kindClusterName,
		"--config", filepath.Join(repoRoot, filepath.FromSlash(kindConfigPath)),
		"--kubeconfig", kubeconfigPath,
		"--wait", "180s"); err != nil {
		return nil, fmt.Errorf("kind create cluster: %w\n%s", err, out)
	}
	return c, nil
}

// LoadImage makes a local Docker image available to the cluster's nodes
// (kind load docker-image); workloads then reference it with
// imagePullPolicy: IfNotPresent.
func (c *KindCluster) LoadImage(ctx context.Context, image string) error {
	if out, err := runCmd(ctx, nil, "kind", "load", "docker-image", "--name", c.Name, image); err != nil {
		return fmt.Errorf("kind load docker-image %s: %w\n%s", image, err, out)
	}
	return nil
}

// Kubectl runs kubectl against this cluster and returns its combined output.
func (c *KindCluster) Kubectl(ctx context.Context, args ...string) (string, error) {
	return runCmd(ctx, []string{"KUBECONFIG=" + c.Kubeconfig}, "kubectl", args...)
}

// Helm runs helm against this cluster and returns its combined output.
func (c *KindCluster) Helm(ctx context.Context, args ...string) (string, error) {
	return runCmd(ctx, []string{"KUBECONFIG=" + c.Kubeconfig}, "helm", args...)
}

// kindRelay is the running bridge for one attempt: the OTLP sink relay
// container on the kind Docker network, writing telemetry to a mounted host
// directory. In-cluster pods reach it at its network IP; delivery is
// container-to-container, needing no route back to the host.
type kindRelay struct {
	// IP is the relay's address on the kind Docker network; in-cluster
	// exporters use http://IP:4318 (OTLP/HTTP) or IP:4317 (OTLP/gRPC).
	IP string
	// HTTPEndpoint is the OTLP/HTTP base URL as seen from inside the cluster.
	HTTPEndpoint string
	// GRPCEndpoint is the OTLP/gRPC host:port as seen from inside the cluster.
	GRPCEndpoint string

	containerName string
}

// startKindRelay builds the relay image (once per process, via the shared
// Docker layer cache) and runs it in sink mode on the kind Docker network,
// writing every received export to the mounted host sinkDir. In-cluster pods
// deliver OTLP to the relay's network IP; delivery is container-to-container,
// so no route back to the host is needed. The per-run bearer token is required
// on every export and travels via an env file, never via command line.
func startKindRelay(ctx context.Context, evalsDir, sinkDir, token string) (*kindRelay, error) {
	if out, err := docker(ctx, "build", "-f", filepath.Join(evalsDir, "cmd", "relay", "Dockerfile"),
		"-t", relayImage, evalsDir); err != nil {
		return nil, fmt.Errorf("docker build %s: %w\n%s", relayImage, err, out)
	}
	if sinkDir == "" {
		return nil, fmt.Errorf("startKindRelay: sinkDir is required")
	}

	envFile, err := writeEnvFile(map[string]string{
		"RELAY_SINK_DIR":     "/out",
		"RELAY_BEARER_TOKEN": token,
	})
	if err != nil {
		return nil, err
	}
	defer os.Remove(envFile)

	name := "eval-kind-relay-" + randomSuffix()
	if out, err := docker(ctx, "run", "-d", "--name", name,
		"--network", kindNetworkName,
		// Run as the host user so files written to the mounted sink dir are
		// owned by (and cleanable by) the test process rather than root.
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-v", sinkDir+":/out",
		"--env-file", envFile,
		relayImage); err != nil {
		return nil, fmt.Errorf("docker run kind relay: %w\n%s", err, out)
	}

	ip, err := containerNetworkIP(ctx, name, kindNetworkName)
	if err != nil {
		_, _ = docker(context.Background(), "rm", "-f", name)
		return nil, err
	}
	return &kindRelay{
		IP:            ip,
		HTTPEndpoint:  fmt.Sprintf("http://%s:%s", ip, relayPort),
		GRPCEndpoint:  fmt.Sprintf("%s:4317", ip),
		containerName: name,
	}, nil
}

// Close removes the relay container.
func (r *kindRelay) Close() {
	_, _ = docker(context.Background(), "rm", "-f", r.containerName)
}

// containerNetworkIP returns the container's IP address on the named Docker
// network.
func containerNetworkIP(ctx context.Context, container, network string) (string, error) {
	format := fmt.Sprintf(`{{with index .NetworkSettings.Networks %q}}{{.IPAddress}}{{end}}`, network)
	out, err := docker(ctx, "inspect", "-f", format, container)
	if err != nil {
		return "", fmt.Errorf("docker inspect %s: %w\n%s", container, err, out)
	}
	ip := strings.TrimSpace(out)
	if ip == "" {
		return "", fmt.Errorf("container %s has no address on the %s network", container, network)
	}
	return ip, nil
}

// runSecretEnv composes the key-value content of the eval-otlp Secret for
// one attempt: the placeholder-contract variables agent-authored artifacts
// reference, and the OTEL_* values workload containers consume. env is the
// harness-composed fixture environment (harness.FixtureEnv), whose test.id
// resource attribute and token are re-targeted at the in-cluster relay
// address.
func runSecretEnv(relay *kindRelay, env map[string]string) map[string]string {
	resourceAttrs := env["OTEL_RESOURCE_ATTRIBUTES"]
	return map[string]string{
		"EVAL_OTLP_ENDPOINT":             relay.HTTPEndpoint,
		"EVAL_OTLP_GRPC_ENDPOINT":        relay.GRPCEndpoint,
		"EVAL_OTLP_TOKEN":                env["EVAL_OTLP_TOKEN"],
		"OTEL_EXPORTER_OTLP_ENDPOINT":    relay.HTTPEndpoint,
		"OTEL_EXPORTER_OTLP_PROTOCOL":    "http/protobuf",
		"OTEL_EXPORTER_OTLP_HEADERS":     env["OTEL_EXPORTER_OTLP_HEADERS"],
		"OTEL_RESOURCE_ATTRIBUTES":       resourceAttrs,
		"EVAL_EXTRA_RESOURCE_ATTRIBUTES": resourceAttrs,
	}
}

// createRunSecrets creates the namespace and the two runtime Secrets for one
// attempt: eval-otlp (placeholder contract plus OTEL_* values) and otel-auth
// (the `token` key carrying the OTEL_EXPORTER_OTLP_HEADERS value, matching
// the secret reference the platforms/k8s.md example teaches). Values travel
// via --from-env-file so no token material reaches command lines.
func createRunSecrets(ctx context.Context, c *KindCluster, namespace string, secretEnv map[string]string) error {
	if out, err := c.Kubectl(ctx, "create", "namespace", namespace); err != nil {
		return fmt.Errorf("create namespace %s: %w\n%s", namespace, err, out)
	}
	runFile, err := writeEnvFile(secretEnv)
	if err != nil {
		return err
	}
	defer os.Remove(runFile)
	if out, err := c.Kubectl(ctx, "-n", namespace, "create", "secret", "generic", runSecretName,
		"--from-env-file", runFile); err != nil {
		return fmt.Errorf("create secret %s: %w\n%s", runSecretName, err, out)
	}
	headerFile, err := writeEnvFile(map[string]string{
		"token": secretEnv["OTEL_EXPORTER_OTLP_HEADERS"],
	})
	if err != nil {
		return err
	}
	defer os.Remove(headerFile)
	if out, err := c.Kubectl(ctx, "-n", namespace, "create", "secret", "generic", headerSecretName,
		"--from-env-file", headerFile); err != nil {
		return fmt.Errorf("create secret %s: %w\n%s", headerSecretName, err, out)
	}
	return nil
}

// applyWorkspace applies every YAML file of the (agent-edited) workspace
// into the namespace. It is applyWorkspaceWithSubst with no substitutions.
func applyWorkspace(ctx context.Context, c *KindCluster, namespace, workdir string) error {
	return applyWorkspaceWithSubst(ctx, c, namespace, workdir, nil)
}

// applyWorkspaceWithSubst applies every YAML file of the (agent-edited)
// workspace into the namespace, substituting each token in subst with its
// value in the file contents before the apply. Files that contain only
// comments are skipped so seed stubs the agent left untouched do not fail the
// apply. Substitution operates on the in-memory file contents (written to a
// temporary file for the apply), so the committed and agent-visible workspace
// files are never mutated on disk.
func applyWorkspaceWithSubst(ctx context.Context, c *KindCluster, namespace, workdir string, subst map[string]string) error {
	entries, err := os.ReadDir(workdir)
	if err != nil {
		return fmt.Errorf("read workspace %s: %w", workdir, err)
	}
	applied := 0
	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		path := filepath.Join(workdir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !hasYAMLContent(string(content)) {
			continue
		}
		text := substituteTokens(string(content), subst)
		applyPath := path
		if text != string(content) {
			tmp, err := os.CreateTemp("", "eval-manifest-*.yaml")
			if err != nil {
				return fmt.Errorf("write substituted manifest for %s: %w", entry.Name(), err)
			}
			applyPath = tmp.Name()
			if _, err := tmp.WriteString(text); err != nil {
				_ = tmp.Close()
				_ = os.Remove(applyPath)
				return fmt.Errorf("write substituted manifest for %s: %w", entry.Name(), err)
			}
			if err := tmp.Close(); err != nil {
				_ = os.Remove(applyPath)
				return fmt.Errorf("write substituted manifest for %s: %w", entry.Name(), err)
			}
		}
		out, err := c.Kubectl(ctx, "-n", namespace, "apply", "-f", applyPath)
		if applyPath != path {
			_ = os.Remove(applyPath)
		}
		if err != nil {
			return fmt.Errorf("kubectl apply %s: %w\n%s", entry.Name(), err, out)
		}
		applied++
	}
	if applied == 0 {
		return fmt.Errorf("workspace %s contains no applicable YAML manifests", workdir)
	}
	return nil
}

// substituteTokens replaces every token key in subst with its value in
// content. Content with no matching token is returned unchanged.
func substituteTokens(content string, subst map[string]string) string {
	for token, value := range subst {
		content = strings.ReplaceAll(content, token, value)
	}
	return content
}

// waitForDeployments waits until every Deployment in the namespace is
// available.
func waitForDeployments(ctx context.Context, c *KindCluster, namespace string, timeout time.Duration) error {
	out, err := c.Kubectl(ctx, "-n", namespace, "wait", "--for=condition=Available",
		"deployment", "--all", "--timeout", timeout.String())
	if err != nil {
		diag, _ := c.Kubectl(context.Background(), "-n", namespace, "get", "pods", "-o", "wide")
		return fmt.Errorf("deployments in %s not available: %w\n%s\npods:\n%s", namespace, err, out, diag)
	}
	return nil
}

// runProbe drives traffic at an in-cluster URL from a helper pod
// (evals/cmd/evalhelper probe) and waits for it to succeed.
func runProbe(ctx context.Context, c *KindCluster, namespace, url string, count int) error {
	name := "eval-probe-" + randomSuffix()
	if out, err := c.Kubectl(ctx, "-n", namespace, "run", name,
		"--image", helperImage, "--image-pull-policy", "IfNotPresent",
		"--restart", "Never", "--",
		"probe", url, fmt.Sprintf("%d", count)); err != nil {
		return fmt.Errorf("start probe pod: %w\n%s", err, out)
	}
	defer func() {
		_, _ = c.Kubectl(context.Background(), "-n", namespace, "delete", "pod", name, "--ignore-not-found")
	}()
	out, err := c.Kubectl(ctx, "-n", namespace, "wait",
		"--for=jsonpath={.status.phase}=Succeeded", "pod/"+name, "--timeout", "180s")
	if err != nil {
		logs, _ := c.Kubectl(context.Background(), "-n", namespace, "logs", name)
		return fmt.Errorf("probe %s did not succeed: %w\n%s\nprobe logs:\n%s", url, err, out, logs)
	}
	return nil
}

// isYAMLFile reports whether the file name has a YAML extension.
func isYAMLFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml"
}

// hasYAMLContent reports whether the text has any non-comment, non-blank
// line.
func hasYAMLContent(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && trimmed != "---" {
			return true
		}
	}
	return false
}

// runCmd executes one command with optional extra environment entries and
// returns its combined output.
func runCmd(ctx context.Context, extraEnv []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
