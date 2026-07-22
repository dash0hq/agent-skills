// Command relay runs the dual-homed OTLP receiver-forwarder as a standalone
// process, so it can run as a container attached to both an internal fixture
// network (receiving OTLP from egress-restricted fixtures and kind pods) and
// a bridge network with host-gateway access (forwarding to the loopback
// otelsink).
//
// Configuration is environment-driven:
//
//	RELAY_UPSTREAM      required; OTLP/HTTP base URL to forward to
//	RELAY_GRPC_LISTEN   optional; gRPC listen address (default ":4317")
//	RELAY_HTTP_LISTEN   optional; HTTP listen address (default ":4318")
//	RELAY_BEARER_TOKEN  optional; when set, required on every export request
//
// The bearer token value is never logged.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dash0hq/agent-skills/evals/harness"
)

func main() {
	upstream := os.Getenv("RELAY_UPSTREAM")
	if upstream == "" {
		log.Fatal("relay: RELAY_UPSTREAM is required")
	}

	relay, err := harness.StartRelay(harness.RelayConfig{
		Upstream:    upstream,
		BearerToken: os.Getenv("RELAY_BEARER_TOKEN"), // value must never be logged
		GRPCListen:  envOr("RELAY_GRPC_LISTEN", ":4317"),
		HTTPListen:  envOr("RELAY_HTTP_LISTEN", ":4318"),
	})
	if err != nil {
		log.Fatalf("relay: %v", err)
	}
	defer relay.Close()

	log.Printf("relay: forwarding to %s (gRPC %s, HTTP %s, auth %s)",
		upstream, relay.GRPCEndpoint(), relay.HTTPEndpoint(), authMode())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Print("relay: shutting down")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// authMode describes whether a token is required without touching its value.
func authMode() string {
	if os.Getenv("RELAY_BEARER_TOKEN") != "" {
		return "bearer-token required"
	}
	return "open"
}
