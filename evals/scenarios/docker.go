package scenarios

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dash0hq/agent-skills/evals/harness"
)

// Stable tags for the harness-owned infrastructure images; both are built
// from the evals/ module (see the Dockerfiles under evals/cmd/). They are
// deliberately not removed on cleanup so Docker's layer cache serves repeat
// runs.
const (
	relayImage  = "agent-skills-eval-relay:local"
	helperImage = "agent-skills-eval-helper:local"
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
//     fixture, a stub downstream server, and the containerized OTLP relay
//     (dual-homed: also on the default bridge with host-gateway access), so
//     the running fixture can reach nothing but the relay (R21). The relay
//     forwards to the host's in-process relay through a host-side TCP proxy,
//     re-presenting the per-run bearer token. Traffic is driven at
//     GET /checkout by a helper container on the internal network.
//
// All created resources are removed by Close, which NewDockerFixture wires to
// t.Cleanup. Resources of failed attempts stay up until then, so their logs
// remain inspectable while the scenario retries.
type DockerFixture struct {
	// evalsDir is the absolute path of the evals/ module root, the build
	// context for the relay and helper images.
	evalsDir string

	mu           sync.Mutex
	currentImage string
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
	d := &DockerFixture{evalsDir: evalsDir}
	t.Cleanup(d.Close)
	return d
}

// Hooks returns the harness fixture hooks backed by this DockerFixture.
func (d *DockerFixture) Hooks() harness.FixtureHooks {
	return harness.FixtureHooks{Build: d.build, Run: d.run}
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

// containerTopology names the resources buildContainerTopology started and
// that the traffic-driver steps need to reference.
type containerTopology struct {
	// network is the internal Docker network every fixture-facing container
	// joins; the traffic drivers attach to it too.
	network string
	// appName is the fixture container's name, used to attach its logs to a
	// traffic-driver failure.
	appName string
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

	// Host-side TCP proxy: exposes the loopback-bound in-process relay on
	// all host interfaces so the container relay can reach it via
	// host-gateway. Only the token-protected relay is exposed, never the
	// sink.
	proxyAddr, closeProxy, err := startHostProxy(env[harness.EnvOTLPEndpoint])
	if err != nil {
		return containerTopology{}, fmt.Errorf("start host proxy: %w", err)
	}
	d.addCleanup(closeProxy)
	_, proxyPort, err := net.SplitHostPort(proxyAddr)
	if err != nil {
		closeProxy()
		return containerTopology{}, fmt.Errorf("split proxy address: %w", err)
	}

	// Dual-homed container relay: default bridge (for the host-gateway
	// route to the proxy) plus the internal network (alias otel-relay).
	// The bearer token travels via an env file, never via command line.
	relayName := "eval-relay-" + suffix
	relayEnvFile, err := writeEnvFile(map[string]string{
		"RELAY_UPSTREAM":     "http://host.docker.internal:" + proxyPort,
		"RELAY_BEARER_TOKEN": env[harness.EnvOTLPToken],
	})
	if err != nil {
		return containerTopology{}, err
	}
	defer os.Remove(relayEnvFile)
	if out, err := docker(ctx, "run", "-d", "--name", relayName,
		"--add-host", "host.docker.internal:host-gateway",
		"--env-file", relayEnvFile,
		relayImage); err != nil {
		return containerTopology{}, fmt.Errorf("docker run relay: %w\n%s", err, out)
	}
	d.addCleanup(func() { _, _ = docker(context.Background(), "rm", "-f", relayName) })
	if out, err := docker(ctx, "network", "connect", "--alias", relayAlias, network, relayName); err != nil {
		return containerTopology{}, fmt.Errorf("connect relay to the internal network: %w\n%s", err, out)
	}

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

	return containerTopology{network: network, appName: appName}, nil
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

	// Traffic driver: a helper container on the internal network probes
	// GET /checkout, retrying while the fixture starts up.
	checkoutURL := fmt.Sprintf("http://%s:%s/checkout", appAlias, appPort)
	if out, err := docker(ctx, "run", "--rm", "--network", topo.network,
		helperImage, "probe", checkoutURL, trafficRequests); err != nil {
		logs, _ := docker(context.Background(), "logs", topo.appName)
		return fmt.Errorf("traffic driver failed against %s: %w\n%s\nfixture logs:\n%s", checkoutURL, err, out, logs)
	}
	return nil
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

// startHostProxy listens on all host interfaces and forwards TCP to the
// host-local upstream URL (the loopback-bound in-process relay). It returns
// the listen address and a closer.
func startHostProxy(upstreamURL string) (string, func(), error) {
	u, err := url.Parse(upstreamURL)
	if err != nil {
		return "", nil, fmt.Errorf("parse upstream %q: %w", upstreamURL, err)
	}
	target := u.Host
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return "", nil, fmt.Errorf("listen: %w", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // listener closed
			}
			go proxyConn(conn, target)
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close() }, nil
}

// proxyConn pipes one accepted connection to the target and back.
func proxyConn(client net.Conn, target string) {
	defer client.Close()
	upstream, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		return
	}
	defer upstream.Close()
	done := make(chan struct{}, 2)
	go pipe(upstream, client, done)
	go pipe(client, upstream, done)
	// Wait for both directions so the second goroutine cannot outlive the
	// connection and leak; the channel is buffered at 2 so neither blocks.
	<-done
	<-done
}

// pipe copies src to dst and half-closes the write side when possible.
func pipe(dst, src net.Conn, done chan<- struct{}) {
	_, _ = io.Copy(dst, src)
	if cw, ok := dst.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
	done <- struct{}{}
}

// randomSuffix returns a short random hex string for container, image, and
// network names.
func randomSuffix() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
