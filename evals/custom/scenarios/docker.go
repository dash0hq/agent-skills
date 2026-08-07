package scenarios

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dash0hq/agent-skills/evals/custom/examples"
	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

// Stable tags for the harness-owned infrastructure images; both are built
// from the evals/custom/ module (see the Dockerfiles under evals/custom/cmd/). They are
// deliberately not removed on cleanup so Docker's layer cache serves repeat
// runs.
const (
	relayImage  = "agent-skills-eval-relay:local"
	helperImage = "agent-skills-eval-helper:local"
	// curlImage is used only by the delivery-stall diagnostic to probe the
	// relay directly from the internal network.
	curlImage = "curlimages/curl:8.17.0"
)

// Container topology constants: aliases on the internal fixture network and
// the ports behind them.
const (
	relayAlias      = "otel-relay"
	relayPort       = "4318"
	downstreamAlias = "downstream"
	downstreamPort  = "9090"
	appAlias        = "app"
	appPort         = "8080"
	trafficRequests = "3"
)

// DockerAvailable reports whether the docker CLI and a reachable daemon are
// available. Docker-dependent tests skip when it returns false.
func DockerAvailable() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "info").Run() == nil
}

// DockerFixture implements the harness FixtureHooks contract with Docker:
//
//   - Build produces an image from the (possibly agent-modified) fixture
//     workspace; build steps may reach package registries (R21).
//   - Run creates an internal Docker network whose only members are the
//     fixture, a stub downstream server, and the containerized OTLP relay, so
//     the running fixture can reach nothing but the relay (R21). The relay
//     runs in sink mode, requiring the per-run bearer token and writing every
//     received export to a mounted host directory (the otelsink dir) — so
//     delivery is container-to-container and needs no route back to the host
//     (host.docker.internal does not work on Linux CI runners). Traffic is
//     driven at GET /checkout by a helper container on the internal network.
//
// All created resources are removed by Close, which NewDockerFixture wires to
// t.Cleanup. Resources of failed attempts stay up until then, so their logs
// remain inspectable while the scenario retries.
type DockerFixture struct {
	// evalsDir is the absolute path of the evals/custom/ module root, the build
	// context for the relay and helper images.
	evalsDir string
	// t is used only for diagnostic logging (container logs on a delivery
	// stall); never for assertions.
	t testingT

	mu           sync.Mutex
	currentImage string
	currentApp   string
	cleanups     []func()
	infraBuilt   bool
}

// testingT is the subset of *testing.T the fixture needs, kept narrow so
// non-test callers could supply their own.
type testingT interface {
	Cleanup(func())
	Logf(format string, args ...any)
	Fatalf(format string, args ...any)
	Helper()
}

// NewDockerFixture returns a DockerFixture whose resources are cleaned up
// when the test finishes.
func NewDockerFixture(t testingT, evalsDir string) *DockerFixture {
	t.Helper()
	d := &DockerFixture{evalsDir: evalsDir, t: t}
	t.Cleanup(d.Close)
	return d
}

// Hooks returns the harness fixture hooks backed by this DockerFixture.
func (d *DockerFixture) Hooks() harness.FixtureHooks {
	return harness.FixtureHooks{Build: d.build, Run: d.run, AppOutput: d.appOutput}
}

// appOutput returns the current fixture container's captured stdout/stderr.
// The runner calls it on every assertion poll of an AssertApp scenario, so
// it re-reads the container logs each time.
func (d *DockerFixture) appOutput(ctx context.Context) (string, error) {
	d.mu.Lock()
	app := d.currentApp
	d.mu.Unlock()
	if app == "" {
		return "", fmt.Errorf("docker logs: no fixture container running for this attempt")
	}
	out, err := docker(ctx, "logs", app)
	if err != nil {
		return "", fmt.Errorf("docker logs %s: %w\n%s", app, err, out)
	}
	return out, nil
}

// Close removes every Docker resource (and host proxy) created so far, in
// reverse creation order.
func (d *DockerFixture) Close() {
	d.mu.Lock()
	cleanups := d.cleanups
	d.cleanups = nil
	d.mu.Unlock()
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
}

func (d *DockerFixture) addCleanup(f func()) {
	d.mu.Lock()
	d.cleanups = append(d.cleanups, f)
	d.mu.Unlock()
}

// build compiles the fixture workspace into an image tagged per attempt.
func (d *DockerFixture) build(ctx context.Context, workdir string, _ map[string]string) error {
	if err := checkClassicBuilderCompatibility(workdir); err != nil {
		return err
	}
	tag := "eval-fixture-" + randomSuffix()
	if out, err := docker(ctx, "build", "-t", tag, workdir); err != nil {
		return fmt.Errorf("docker build of the fixture workspace failed: %w\n%s", err, out)
	}
	d.addCleanup(func() { _, _ = docker(context.Background(), "rmi", "-f", tag) })
	d.mu.Lock()
	d.currentImage = tag
	d.mu.Unlock()
	return nil
}

// checkClassicBuilderCompatibility scans the workspace for Dockerfiles and
// rejects instructions that the classic (non-BuildKit) Docker builder fails
// to parse, most notably the legacy space-separated ENV form with a value
// containing spaces (issue #120). The harness's own `docker build` runs
// under BuildKit, which accepts that form, so without this gate an
// agent-written Dockerfile can pass the eval here yet break `docker build`
// on stock GitHub Actions runners and other classic-builder CI environments.
func checkClassicBuilderCompatibility(workdir string) error {
	var problems []string
	err := filepath.WalkDir(workdir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if name != "Dockerfile" && !strings.HasPrefix(name, "Dockerfile.") &&
			!strings.HasSuffix(strings.ToLower(name), ".dockerfile") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relPath, relErr := filepath.Rel(workdir, path)
		if relErr != nil {
			relPath = path
		}
		for _, issue := range examples.LintDockerfile(string(content)) {
			if issue.BreaksClassicBuilder {
				problems = append(problems, fmt.Sprintf("%s:%d: %s", relPath, issue.Line, issue.Message))
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("classic-builder compatibility scan of the fixture workspace failed: %w", err)
	}
	if len(problems) > 0 {
		return fmt.Errorf("the fixture workspace contains Dockerfile instructions the classic (non-BuildKit) docker builder rejects:\n%s",
			strings.Join(problems, "\n"))
	}
	return nil
}

// containerTopology names the resources buildContainerTopology started and
// that the traffic-driver steps need to reference.
type containerTopology struct {
	// network is the internal Docker network every fixture-facing container
	// joins; the traffic drivers attach to it too.
	network string
	// appName is the fixture container's name, used to attach its logs to a
	// traffic-driver failure.
	appName string
	// relayName is the sink relay container's name, used for delivery
	// diagnostics.
	relayName string
	// sinkDir is the host directory the sink relay writes into (mounted at
	// /out in the relay), read for the delivery-stall diagnostic.
	sinkDir string
}

// buildContainerTopology starts the container topology shared by every
// Docker-backed scenario: the internal network, the host-side proxy, the
// dual-homed relay container, the stub downstream, and the fixture container.
// It returns the resources the caller's traffic-driver step references.
// extraAppEnv contributes additional entries to the fixture's env file (the
// browser scenario forwards EVAL_RESOURCE_ATTRIBUTES this way); it is empty
// for the HTTP probe topology.
func (d *DockerFixture) buildContainerTopology(ctx context.Context, image string, env, extraAppEnv map[string]string) (containerTopology, error) {
	suffix := randomSuffix()

	// Internal network: the running fixture can reach only its members.
	network := "eval-int-" + suffix
	if out, err := docker(ctx, "network", "create", "--internal", network); err != nil {
		return containerTopology{}, fmt.Errorf("docker network create: %w\n%s", err, out)
	}
	d.addCleanup(func() { _, _ = docker(context.Background(), "network", "rm", network) })

	// In-network OTLP sink relay: on the internal network only (alias
	// otel-relay), writing every received export to the mounted host sink
	// directory. Delivery is container-to-container, so it needs no route back
	// to the host — host.docker.internal / host-gateway does not work on Linux
	// CI runners, which is why the earlier host-proxy design failed there. The
	// bearer token travels via an env file, never via the command line.
	sinkDir := env[harness.EnvSinkDir]
	if sinkDir == "" {
		return containerTopology{}, fmt.Errorf("fixture environment missing %s", harness.EnvSinkDir)
	}
	relayName := "eval-relay-" + suffix
	relayEnvFile, err := writeEnvFile(map[string]string{
		"RELAY_SINK_DIR":     "/out",
		"RELAY_BEARER_TOKEN": env[harness.EnvOTLPToken],
	})
	if err != nil {
		return containerTopology{}, err
	}
	defer os.Remove(relayEnvFile)
	if out, err := docker(ctx, "run", "-d", "--name", relayName,
		"--network", network, "--network-alias", relayAlias,
		// Run as the host user so files written to the mounted sink dir are
		// owned by (and cleanable by) the test process rather than root.
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-v", sinkDir+":/out",
		"--env-file", relayEnvFile,
		relayImage); err != nil {
		return containerTopology{}, fmt.Errorf("docker run sink relay: %w\n%s", err, out)
	}
	d.addCleanup(func() { _, _ = docker(context.Background(), "rm", "-f", relayName) })

	// Stub downstream server the fixture's outbound call targets.
	downstreamName := "eval-downstream-" + suffix
	if out, err := docker(ctx, "run", "-d", "--name", downstreamName,
		"--network", network, "--network-alias", downstreamAlias,
		helperImage, "serve"); err != nil {
		return containerTopology{}, fmt.Errorf("docker run downstream stub: %w\n%s", err, out)
	}
	d.addCleanup(func() { _, _ = docker(context.Background(), "rm", "-f", downstreamName) })

	// The fixture itself, on the internal network only, with the composed
	// environment retargeted at the container relay.
	appName := "eval-app-" + suffix
	relayEndpoint := fmt.Sprintf("http://%s:%s", relayAlias, relayPort)
	appEnv := map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": relayEndpoint,
		"OTEL_EXPORTER_OTLP_PROTOCOL": env["OTEL_EXPORTER_OTLP_PROTOCOL"],
		"OTEL_EXPORTER_OTLP_HEADERS":  env["OTEL_EXPORTER_OTLP_HEADERS"],
		"OTEL_RESOURCE_ATTRIBUTES":    env["OTEL_RESOURCE_ATTRIBUTES"],
		harness.EnvOTLPEndpoint:       relayEndpoint,
		harness.EnvOTLPToken:          env[harness.EnvOTLPToken],
		"PORT":                        appPort,
		"DOWNSTREAM_URL":              fmt.Sprintf("http://%s:%s/inventory", downstreamAlias, downstreamPort),
	}
	for k, v := range extraAppEnv {
		appEnv[k] = v
	}
	appEnvFile, err := writeEnvFile(appEnv)
	if err != nil {
		return containerTopology{}, err
	}
	defer os.Remove(appEnvFile)
	if out, err := docker(ctx, "run", "-d", "--name", appName,
		"--network", network, "--network-alias", appAlias,
		"--env-file", appEnvFile,
		image); err != nil {
		return containerTopology{}, fmt.Errorf("docker run fixture: %w\n%s", err, out)
	}
	d.addCleanup(func() { _, _ = docker(context.Background(), "rm", "-f", appName) })

	return containerTopology{network: network, appName: appName, relayName: relayName, sinkDir: sinkDir}, nil
}

// run starts the container topology for one attempt and drives traffic at the
// fixture. The containers keep running when it returns, so telemetry flushed
// by batch processors still reaches the sink while the runner waits.
func (d *DockerFixture) run(ctx context.Context, _ string, env map[string]string) error {
	d.mu.Lock()
	image := d.currentImage
	d.mu.Unlock()
	if image == "" {
		return fmt.Errorf("docker run: no fixture image built for this attempt")
	}
	if err := d.ensureInfraImages(ctx); err != nil {
		return err
	}

	topo, err := d.buildContainerTopology(ctx, image, env, nil)
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.currentApp = topo.appName
	d.mu.Unlock()

	// Traffic driver: a helper container on the internal network probes
	// GET /checkout, retrying while the fixture starts up.
	checkoutURL := fmt.Sprintf("http://%s:%s/checkout", appAlias, appPort)
	if out, err := docker(ctx, "run", "--rm", "--network", topo.network,
		helperImage, "probe", checkoutURL, trafficRequests); err != nil {
		logs, _ := docker(context.Background(), "logs", topo.appName)
		return fmt.Errorf("traffic driver failed against %s: %w\n%s\nfixture logs:\n%s", checkoutURL, err, out, logs)
	}
	d.diagnoseDeliveryStall(ctx, topo, env[harness.EnvOTLPToken])
	return nil
}

// diagnoseDeliveryStall gives batched exporters a short window to deliver and,
// if nothing lands in the sink dir, dumps the fixture and sink-relay container
// logs plus the relay's state and the sink dir listing. This surfaces why
// container-to-container OTLP delivery stalled (fixture export error, relay
// auth rejection, relay write/permission failure) directly in the CI job log,
// where the containers are otherwise torn down before they can be inspected.
func (d *DockerFixture) diagnoseDeliveryStall(ctx context.Context, topo containerTopology, token string) {
	if d.t == nil || topo.sinkDir == "" {
		return
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if entries, _ := filepath.Glob(filepath.Join(topo.sinkDir, "*.jsonl")); len(entries) > 0 {
			return // telemetry is arriving; no stall to diagnose
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
	var listing strings.Builder
	if entries, err := os.ReadDir(topo.sinkDir); err != nil {
		fmt.Fprintf(&listing, "  (read error: %v)", err)
	} else if len(entries) == 0 {
		listing.WriteString("  (empty)")
	} else {
		for _, e := range entries {
			if info, ierr := e.Info(); ierr == nil {
				fmt.Fprintf(&listing, "  %s (%d bytes, mode %s)\n", e.Name(), info.Size(), info.Mode())
			} else {
				fmt.Fprintf(&listing, "  %s\n", e.Name())
			}
		}
	}
	relayLogs, _ := docker(context.Background(), "logs", topo.relayName)
	appLogs, _ := docker(context.Background(), "logs", topo.appName)
	state, _ := docker(context.Background(), "inspect", "-f", "{{.State.Status}} exit={{.State.ExitCode}}", topo.relayName)

	// The fixture's OTLP env: confirms it is pointed at the relay alias.
	envOut, _ := docker(context.Background(), "inspect", "-f", "{{range .Config.Env}}{{println .}}{{end}}", topo.appName)
	var otelEnv strings.Builder
	for _, line := range strings.Split(envOut, "\n") {
		if strings.HasPrefix(line, "OTEL_") {
			otelEnv.WriteString("  " + line + "\n")
		}
	}

	// Direct relay probe from inside the internal network: isolates relay
	// reachability + alias resolution + auth + write from the fixture's own
	// export. If this lands in the sink dir, the relay path is sound and the
	// fixture is not exporting; if it does not, the relay/network path itself
	// is the fault.
	probeBody := `{"resourceSpans":[{"resource":{"attributes":[{"key":"test.id","value":{"stringValue":"diag-probe"}}]},"scopeSpans":[{"spans":[{"name":"diag-probe"}]}]}]}`
	probeURL := fmt.Sprintf("http://%s:%s/v1/traces", relayAlias, relayPort)
	probeOut, probeErr := docker(context.Background(), "run", "--rm", "--network", topo.network, curlImage,
		"-sS", "-m", "10", "-o", "/dev/null", "-w", "HTTP %{http_code}",
		"-X", "POST", probeURL,
		"-H", "Content-Type: application/json",
		"-H", "Authorization: Bearer "+token,
		"-d", probeBody)
	if probeErr != nil {
		probeOut = fmt.Sprintf("(probe failed: %v) %s", probeErr, probeOut)
	}
	probeLanded := false
	if entries, _ := filepath.Glob(filepath.Join(topo.sinkDir, "*.jsonl")); len(entries) > 0 {
		probeLanded = true
	}

	d.t.Logf("delivery-stall diagnostic: no telemetry in the sink dir after traffic.\n"+
		"relay state: %s\n"+
		"fixture OTLP env:\n%s\n"+
		"direct relay probe (%s): %s -> landed in sink: %t\n"+
		"host-side sink dir listing (%s):\n%s\n"+
		"sink relay logs:\n%s\n"+
		"fixture logs:\n%s",
		strings.TrimSpace(state), otelEnv.String(), probeURL, strings.TrimSpace(probeOut), probeLanded,
		topo.sinkDir, listing.String(), relayLogs, appLogs)
}

// ensureInfraImages builds the relay and helper images once per fixture.
func (d *DockerFixture) ensureInfraImages(ctx context.Context) error {
	d.mu.Lock()
	built := d.infraBuilt
	d.mu.Unlock()
	if built {
		return nil
	}
	for image, dockerfile := range map[string]string{
		relayImage:  filepath.Join("cmd", "relay", "Dockerfile"),
		helperImage: filepath.Join("cmd", "evalhelper", "Dockerfile"),
	} {
		if out, err := docker(ctx, "build", "-f", filepath.Join(d.evalsDir, dockerfile), "-t", image, d.evalsDir); err != nil {
			return fmt.Errorf("docker build %s: %w\n%s", image, err, out)
		}
	}
	d.mu.Lock()
	d.infraBuilt = true
	d.mu.Unlock()
	return nil
}

// docker runs one docker CLI command and returns its combined output.
func docker(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	return string(out), err
}

// writeEnvFile writes KEY=VALUE lines to a private temp file for
// --env-file, keeping secret values out of command lines and process lists.
func writeEnvFile(env map[string]string) (string, error) {
	f, err := os.CreateTemp("", "eval-env-*")
	if err != nil {
		return "", fmt.Errorf("create env file: %w", err)
	}
	var b strings.Builder
	for k, v := range env {
		fmt.Fprintf(&b, "%s=%s\n", k, v)
	}
	if _, err := f.WriteString(b.String()); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("write env file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("close env file: %w", err)
	}
	return f.Name(), nil
}

// randomSuffix returns a short random hex string for container, image, and
// network names.
func randomSuffix() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
