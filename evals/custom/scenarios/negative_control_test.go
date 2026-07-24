package scenarios

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dash0hq/agent-skills/evals/custom/harness"

	"github.com/dash0hq/agent-skills/evals/custom/internal/testutil"
)

// Negative control: an unmodified fixture emits nothing, so the scenario
// fails with class agent-telemetry — proving the assertions cannot pass
// vacuously. No Docker is needed: the Go fixture has zero dependencies and
// builds and runs directly on the host with the Go toolchain the tests
// already require. The stub agent shows skill evidence but changes nothing.
func TestNegativeControlUninstrumentedFixtureEmitsNothing(t *testing.T) {
	root := testutil.RepoRoot(t)
	skillFile := filepath.Join(root, "skills", "otel-instrumentation", "rules", "sdks", "go.md")
	stub := testutil.WriteStub(t, testutil.GoodStubBody(string(harness.SkillInstrumentation), skillFile))

	fix := newHostGoFixture(t)
	runner := &harness.Runner{
		RepoRoot: root,
		Agent:    &harness.Agent{Binary: stub, PluginDir: root, Model: "claude-fable-5"},
		Hooks:    fix.hooks(),
	}

	sc := GoHTTP()
	sc.Timeout = 3 * time.Minute
	sc.TelemetryTimeout = 2 * time.Second

	v := runner.Run(t, sc)

	require.False(t, v.Passed, "an uninstrumented fixture must not pass: %+v", v)
	require.Equal(t, harness.ClassAgentTelemetry, v.Class, "detail: %s", v.Detail)
	require.Equal(t, harness.MaxAgentAttempts, v.AgentAttempts)
}

// hostGoFixture builds and runs the Go fixture directly on the host: a
// Docker-free stand-in for the Docker hooks, used by the negative control.
// It doubles as a fixture contract check: Build fails if the fixture does
// not compile, Run fails if it does not serve GET /checkout or does not call
// DOWNSTREAM_URL.
type hostGoFixture struct {
	t  *testing.T
	mu sync.Mutex
	// procs are the started fixture processes, reaped at test cleanup.
	procs []*exec.Cmd
	// servers are the stub downstream servers, closed at test cleanup.
	servers []*httptest.Server
}

func newHostGoFixture(t *testing.T) *hostGoFixture {
	f := &hostGoFixture{t: t}
	t.Cleanup(f.close)
	return f
}

func (f *hostGoFixture) close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, cmd := range f.procs {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}
	for _, srv := range f.servers {
		srv.Close()
	}
}

func (f *hostGoFixture) hooks() harness.FixtureHooks {
	return harness.FixtureHooks{Build: f.build, Run: f.run}
}

func (f *hostGoFixture) build(ctx context.Context, workdir string, _ map[string]string) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-o", "eval-app", ".")
	cmd.Dir = workdir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build: %w\n%s", err, out)
	}
	return nil
}

func (f *hostGoFixture) run(ctx context.Context, workdir string, env map[string]string) error {
	// Stub downstream server for the fixture's outbound call, tracking
	// whether the fixture actually called it.
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"sku":"TEST-SKU-0001","stock":7}`)
	}))
	f.mu.Lock()
	f.servers = append(f.servers, downstream)
	f.mu.Unlock()

	port, err := freePort()
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, filepath.Join(workdir, "eval-app"))
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), "PORT="+port, "DOWNSTREAM_URL="+downstream.URL)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start fixture: %w", err)
	}
	f.mu.Lock()
	f.procs = append(f.procs, cmd)
	f.mu.Unlock()

	// Readiness plus traffic: 3 successful GET /checkout requests.
	client := &http.Client{Timeout: 2 * time.Second}
	url := "http://127.0.0.1:" + port + "/checkout"
	deadline := time.Now().Add(15 * time.Second)
	succeeded := 0
	var lastErr error
	for succeeded < 3 {
		if time.Now().After(deadline) || ctx.Err() != nil {
			return fmt.Errorf("fixture did not serve 3 successful GET /checkout requests in time (last error: %v)", lastErr)
		}
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(100 * time.Millisecond)
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		succeeded++
	}
	return nil
}

func freePort() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer ln.Close()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	return port, err
}
