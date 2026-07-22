package harness

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	// Registers the server-side gzip decompressor so gRPC exporters that
	// compress (grpc-encoding: gzip) are decoded; the HTTP path handles
	// Content-Encoding: gzip explicitly in httpHandler.
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

// NewToken returns a hex-encoded 256-bit bearer token from crypto/rand.
// Token values must never be written to logs, errors, transcripts, or stdout;
// the relay and the harness only ever compare them.
func NewToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("relay: generate token: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// RelayConfig configures a Relay.
type RelayConfig struct {
	// Upstream is the OTLP/HTTP base URL every export request is forwarded
	// to (typically the loopback otelsink's HTTPEndpoint).
	Upstream string
	// BearerToken, when non-empty, is required on every export request:
	// "Authorization: Bearer <token>" over HTTP, an "authorization"
	// metadata entry over gRPC. CORS preflights are answered without
	// authentication (browsers do not attach headers to preflights).
	// The value is never logged.
	BearerToken string
	// GRPCListen is the gRPC listen address; empty means "127.0.0.1:0".
	GRPCListen string
	// HTTPListen is the HTTP listen address; empty means "127.0.0.1:0".
	HTTPListen string
}

// Relay is a dual-homed OTLP receiver-forwarder: it accepts OTLP over gRPC
// and HTTP/protobuf and forwards the raw export requests to a configurable
// upstream (the loopback otelsink). Run in-process for host-local scenarios,
// or as a container (see evals/cmd/relay) attached to both an internal
// fixture network and a bridge network so restricted fixtures and kind pods
// can reach the host sink.
type Relay struct {
	cfg        RelayConfig
	grpcAddr   string
	httpAddr   string
	grpcServer *grpc.Server
	httpServer *http.Server
	client     *http.Client
}

// StartRelay launches the relay listeners and returns once both accept
// connections. Call Close to shut it down.
func StartRelay(cfg RelayConfig) (*Relay, error) {
	if cfg.Upstream == "" {
		return nil, fmt.Errorf("relay: Upstream is required")
	}
	if cfg.GRPCListen == "" {
		cfg.GRPCListen = "127.0.0.1:0"
	}
	if cfg.HTTPListen == "" {
		cfg.HTTPListen = "127.0.0.1:0"
	}

	r := &Relay{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}

	grpcLn, err := net.Listen("tcp", cfg.GRPCListen)
	if err != nil {
		return nil, fmt.Errorf("relay: listen gRPC: %w", err)
	}
	httpLn, err := net.Listen("tcp", cfg.HTTPListen)
	if err != nil {
		_ = grpcLn.Close()
		return nil, fmt.Errorf("relay: listen HTTP: %w", err)
	}
	r.grpcAddr = grpcLn.Addr().String()
	r.httpAddr = httpLn.Addr().String()

	r.grpcServer = grpc.NewServer(grpc.UnaryInterceptor(r.authUnaryInterceptor))
	coltracepb.RegisterTraceServiceServer(r.grpcServer, &relayTraceService{relay: r})
	colmetricspb.RegisterMetricsServiceServer(r.grpcServer, &relayMetricsService{relay: r})
	collogspb.RegisterLogsServiceServer(r.grpcServer, &relayLogsService{relay: r})
	go func() { _ = r.grpcServer.Serve(grpcLn) }()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", r.httpHandler("/v1/traces"))
	mux.HandleFunc("/v1/metrics", r.httpHandler("/v1/metrics"))
	mux.HandleFunc("/v1/logs", r.httpHandler("/v1/logs"))
	r.httpServer = &http.Server{Handler: mux, ReadHeaderTimeout: 30 * time.Second}
	go func() { _ = r.httpServer.Serve(httpLn) }()

	return r, nil
}

// Close shuts down both listeners.
func (r *Relay) Close() {
	r.grpcServer.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = r.httpServer.Shutdown(ctx)
}

// GRPCEndpoint returns the host:port of the OTLP/gRPC listener.
func (r *Relay) GRPCEndpoint() string { return r.grpcAddr }

// HTTPEndpoint returns the base URL of the OTLP/HTTP listener.
func (r *Relay) HTTPEndpoint() string { return "http://" + r.httpAddr }

// forward posts a serialized export request to the upstream signal path.
// When a bearer token is configured, it is re-presented upstream, so relays
// chain: a container relay on the fixture network can forward to the host's
// in-process relay, which enforces the same per-run token. The sink itself
// ignores the header.
func (r *Relay) forward(ctx context.Context, path, contentType string, body []byte) (int, []byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.cfg.Upstream+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, "", err
	}
	req.Header.Set("Content-Type", contentType)
	if r.cfg.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.BearerToken)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, "", err
	}
	return resp.StatusCode, respBody, resp.Header.Get("Content-Type"), nil
}

// gunzip decompresses a gzip-encoded request body.
func gunzip(b []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

// tokenValid compares the presented token in constant time. It never logs or
// returns either value.
func (r *Relay) tokenValid(presented string) bool {
	return subtle.ConstantTimeCompare([]byte(presented), []byte(r.cfg.BearerToken)) == 1
}

// --- HTTP ---

// httpHandler forwards OTLP/HTTP export requests for one signal path and
// answers CORS preflights permissively (the browser fixture exports OTLP/HTTP
// from an arbitrary origin).
func (r *Relay) httpHandler(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		setCORSHeaders(w, req)
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if req.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.cfg.BearerToken != "" {
			presented, ok := strings.CutPrefix(req.Header.Get("Authorization"), "Bearer ")
			if !ok || !r.tokenValid(presented) {
				// Deliberately generic: no token material in the response.
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// The upstream sink decodes protobuf/JSON but not Content-Encoding,
		// and forward() does not re-present the encoding, so decompress here.
		// OTLP exporters that default to gzip (for example the OpenTelemetry
		// Ruby OTLP exporter) would otherwise deliver bytes the sink cannot
		// parse, and the export would fail silently.
		if strings.EqualFold(req.Header.Get("Content-Encoding"), "gzip") {
			body, err = gunzip(body)
			if err != nil {
				http.Error(w, "invalid gzip body", http.StatusBadRequest)
				return
			}
		}
		code, respBody, contentType, err := r.forward(req.Context(), path, req.Header.Get("Content-Type"), body)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(code)
		_, _ = w.Write(respBody)
	}
}

// setCORSHeaders answers CORS permissively: any origin may export OTLP/HTTP
// through the relay (authentication still applies to the actual export).
func setCORSHeaders(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if reqHeaders := req.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
		w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
	} else {
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	}
	w.Header().Set("Access-Control-Max-Age", "3600")
}

// --- gRPC ---

// authUnaryInterceptor enforces the bearer token on gRPC exports when one is
// configured. Errors carry no token material.
func (r *Relay) authUnaryInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if r.cfg.BearerToken == "" {
		return handler(ctx, req)
	}
	md, _ := metadata.FromIncomingContext(ctx)
	for _, v := range md.Get("authorization") {
		if presented, ok := strings.CutPrefix(v, "Bearer "); ok && r.tokenValid(presented) {
			return handler(ctx, req)
		}
	}
	return nil, status.Error(codes.Unauthenticated, "invalid or missing bearer token")
}

// forwardProto re-serializes a decoded gRPC export request and forwards it to
// the upstream OTLP/HTTP endpoint as protobuf.
func (r *Relay) forwardProto(ctx context.Context, path string, msg proto.Message) error {
	body, err := proto.Marshal(msg)
	if err != nil {
		return status.Errorf(codes.Internal, "relay: marshal: %v", err)
	}
	code, _, _, err := r.forward(ctx, path, "application/x-protobuf", body)
	if err != nil {
		return status.Error(codes.Unavailable, "relay: upstream unavailable")
	}
	if code != http.StatusOK {
		return status.Errorf(codes.Unavailable, "relay: upstream returned status %d", code)
	}
	return nil
}

type relayTraceService struct {
	coltracepb.UnimplementedTraceServiceServer
	relay *Relay
}

func (s *relayTraceService) Export(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	if err := s.relay.forwardProto(ctx, "/v1/traces", req); err != nil {
		return nil, err
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

type relayMetricsService struct {
	colmetricspb.UnimplementedMetricsServiceServer
	relay *Relay
}

func (s *relayMetricsService) Export(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) (*colmetricspb.ExportMetricsServiceResponse, error) {
	if err := s.relay.forwardProto(ctx, "/v1/metrics", req); err != nil {
		return nil, err
	}
	return &colmetricspb.ExportMetricsServiceResponse{}, nil
}

type relayLogsService struct {
	collogspb.UnimplementedLogsServiceServer
	relay *Relay
}

func (s *relayLogsService) Export(ctx context.Context, req *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	if err := s.relay.forwardProto(ctx, "/v1/logs", req); err != nil {
		return nil, err
	}
	return &collogspb.ExportLogsServiceResponse{}, nil
}
