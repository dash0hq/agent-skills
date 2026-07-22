package harness

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"
)

func startRelayForTest(t *testing.T, token string) (*otelsink.Sink, *Relay) {
	t.Helper()
	sink := otelsink.Start(t)
	relay, err := StartRelay(RelayConfig{Upstream: sink.HTTPEndpoint(), BearerToken: token})
	require.NoError(t, err)
	t.Cleanup(relay.Close)
	return sink, relay
}

func postTraces(t *testing.T, relay *Relay, authorization string, testID string) *http.Response {
	t.Helper()
	body, err := protojson.Marshal(makeTraceRequest(testID, "relayed-span", nil))
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, relay.HTTPEndpoint()+"/v1/traces", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestRelayForwardsOTLPHTTP(t *testing.T) {
	token, err := NewToken()
	require.NoError(t, err)
	sink, relay := startRelayForTest(t, token)

	resp := postTraces(t, relay, "Bearer "+token, sink.TestID())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, sink.Traces(t).WithName("relayed-span").Len(), "export forwarded to the upstream sink")
}

// OTLP exporters that default to gzip (for example the OpenTelemetry Ruby
// OTLP exporter) send Content-Encoding: gzip. The upstream sink parses
// protobuf but not the content encoding, so the relay must decompress before
// forwarding, or the export fails silently and no telemetry reaches the sink.
func TestRelayDecompressesGzipHTTP(t *testing.T) {
	token, err := NewToken()
	require.NoError(t, err)
	sink, relay := startRelayForTest(t, token)

	raw, err := proto.Marshal(makeTraceRequest(sink.TestID(), "gzipped-span", nil))
	require.NoError(t, err)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err = zw.Write(raw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	req, err := http.NewRequest(http.MethodPost, relay.HTTPEndpoint()+"/v1/traces", bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, sink.Traces(t).WithName("gzipped-span").Len(), "gzip-compressed export decompressed and forwarded to the sink")
}

func TestRelayRejectsWrongOrMissingToken(t *testing.T) {
	token, err := NewToken()
	require.NoError(t, err)
	sink, relay := startRelayForTest(t, token)

	wrong, err := NewToken()
	require.NoError(t, err)

	tests := []struct {
		name          string
		authorization string
	}{
		{"missing header", ""},
		{"wrong token", "Bearer " + wrong},
		{"not a bearer scheme", "Basic dXNlcjpwYXNz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := postTraces(t, relay, tt.authorization, sink.TestID())
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
	require.Equal(t, 0, sink.Traces(t).Len(), "nothing reached the sink without a valid token")
}

func TestRelayWithoutTokenConfiguredIsOpen(t *testing.T) {
	sink, relay := startRelayForTest(t, "")

	resp := postTraces(t, relay, "", sink.TestID())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, sink.Traces(t).WithName("relayed-span").Len())
}

func TestRelayForwardsOTLPGRPC(t *testing.T) {
	token, err := NewToken()
	require.NoError(t, err)
	sink, relay := startRelayForTest(t, token)

	conn, err := grpc.NewClient(relay.GRPCEndpoint(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := coltracepb.NewTraceServiceClient(conn)

	t.Run("with token", func(t *testing.T) {
		ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
		_, err := client.Export(ctx, makeTraceRequest(sink.TestID(), "grpc-span", nil))
		require.NoError(t, err)
		require.Equal(t, 1, sink.Traces(t).WithName("grpc-span").Len())
	})

	t.Run("without token", func(t *testing.T) {
		_, err := client.Export(context.Background(), makeTraceRequest(sink.TestID(), "grpc-span-unauth", nil))
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Equal(t, 0, sink.Traces(t).WithName("grpc-span-unauth").Len())
	})
}

// CORS preflights must be answered permissively and without authentication:
// browsers do not attach the Authorization header to preflight requests.
func TestRelayCORSPreflight(t *testing.T) {
	token, err := NewToken()
	require.NoError(t, err)
	_, relay := startRelayForTest(t, token)

	req, err := http.NewRequest(http.MethodOptions, relay.HTTPEndpoint()+"/v1/traces", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://fixture.example.test:8080")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type,authorization")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	require.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "POST")
	require.Equal(t, "content-type,authorization", resp.Header.Get("Access-Control-Allow-Headers"))
}

// Relays chain: a downstream relay (the container relay on the fixture
// network) forwards to an upstream relay (the host in-process relay) by
// re-presenting its configured bearer token, so the upstream's token check
// still passes.
func TestRelayChainsThroughUpstreamRelay(t *testing.T) {
	token, err := NewToken()
	require.NoError(t, err)
	sink, upstream := startRelayForTest(t, token)

	downstream, err := StartRelay(RelayConfig{Upstream: upstream.HTTPEndpoint(), BearerToken: token})
	require.NoError(t, err)
	t.Cleanup(downstream.Close)

	resp := postTraces(t, downstream, "Bearer "+token, sink.TestID())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, sink.Traces(t).WithName("relayed-span").Len(), "export traversed both relays to the sink")
}

func TestTokensAreRandomAndNeverEmpty(t *testing.T) {
	a, err := NewToken()
	require.NoError(t, err)
	b, err := NewToken()
	require.NoError(t, err)
	require.Len(t, a, 64, "hex-encoded 32 bytes")
	require.NotEqual(t, a, b)
}

// TestRelayDockerImageBuilds verifies the standalone relay container image
// builds; it requires a working Docker daemon and skips otherwise.
func TestRelayDockerImageBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker image build in -short mode")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("skipping: docker CLI not available on PATH")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("skipping: docker daemon not reachable")
	}

	moduleRoot, err := filepath.Abs("..")
	require.NoError(t, err)

	tag := "agent-skills-eval-relay:test"
	cmd := exec.Command("docker", "build", "-f", filepath.Join("cmd", "relay", "Dockerfile"), "-t", tag, ".")
	cmd.Dir = moduleRoot
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "docker build failed:\n%s", out)
	t.Cleanup(func() { _ = exec.Command("docker", "rmi", "-f", tag).Run() })
}
