// Command instrumented-go-service is the Kubernetes eval workload: the same
// checkout service as evals/fixtures/go-service, but already instrumented
// with the OpenTelemetry SDK and configured entirely through the standard
// OTEL_* environment variables (endpoint, protocol, headers, service name,
// resource attributes).
//
// It exists for scenarios where the agent's task is not SDK instrumentation
// but the surrounding Kubernetes configuration: pod specs with downward-API
// resource attributes (skills/otel-instrumentation/rules/platforms/k8s.md)
// and Collector deployments the workload's telemetry must flow through.
// The agent edits manifests and Collector configuration; this image emits
// telemetry exactly as the environment tells it to.
//
// One eval-specific extension: resource attributes listed in
// EVAL_EXTRA_RESOURCE_ATTRIBUTES (same k=v,k=v syntax as
// OTEL_RESOURCE_ATTRIBUTES) are merged into the resource. The harness uses
// it to attach the per-run test.id without competing with the
// OTEL_RESOURCE_ATTRIBUTES value the agent authors in the pod spec.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	shutdown, err := setupTracing(ctx)
	if err != nil {
		logger.Error("tracing setup failed", "error", err)
		os.Exit(1)
	}
	defer shutdown(ctx)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	downstreamURL := os.Getenv("DOWNSTREAM_URL")

	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /checkout", func(w http.ResponseWriter, r *http.Request) {
		inventory := "TEST-SKU-0001"
		if downstreamURL != "" {
			req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, downstreamURL, nil)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				logger.Error("inventory lookup failed", "error", err)
				http.Error(w, "inventory unavailable", http.StatusBadGateway)
				return
			}
			body, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				http.Error(w, "inventory unavailable", http.StatusBadGateway)
				return
			}
			inventory = string(body)
		}

		logger.Info("checkout completed",
			"order.id", "TEST-0001",
			"customer.email", "user@example.test",
		)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"order_id":       "TEST-0001",
			"customer_email": "user@example.test",
			"status":         "confirmed",
			"inventory":      inventory,
		})
	})

	handler := otelhttp.NewHandler(mux, "http.server")
	addr := fmt.Sprintf(":%s", port)
	logger.Info("instrumented-go-service listening", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}

// setupTracing wires an OTLP/HTTP trace exporter configured from the OTEL_*
// environment variables and a resource combining the environment-provided
// attributes with the EVAL_EXTRA_RESOURCE_ATTRIBUTES merge described above.
func setupTracing(ctx context.Context) (func(context.Context) error, error) {
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithAttributes(extraResourceAttributes()...),
	)
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(time.Second)),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}

// extraResourceAttributes parses EVAL_EXTRA_RESOURCE_ATTRIBUTES (k=v,k=v)
// into attributes; malformed entries are skipped.
func extraResourceAttributes() []attribute.KeyValue {
	var out []attribute.KeyValue
	for _, pair := range strings.Split(os.Getenv("EVAL_EXTRA_RESOURCE_ATTRIBUTES"), ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok || key == "" {
			continue
		}
		out = append(out, attribute.String(key, value))
	}
	return out
}
