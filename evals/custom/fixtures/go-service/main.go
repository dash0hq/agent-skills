// Command go-service is a deliberately uninstrumented HTTP service used as an
// eval fixture (see evals/custom/fixtures/README.md for the contract). It serves
// GET /checkout, performs one outbound HTTP call to DOWNSTREAM_URL while
// handling it, listens on PORT (default 8080), and uses only obviously
// synthetic data (user@example.test, TEST-0001).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type checkoutResponse struct {
	OrderID       string `json:"order_id"`
	CustomerEmail string `json:"customer_email"`
	Status        string `json:"status"`
	Inventory     string `json:"inventory"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	downstreamURL := os.Getenv("DOWNSTREAM_URL")
	if downstreamURL == "" {
		downstreamURL = "http://localhost:9090/inventory"
	}

	client := &http.Client{Timeout: 5 * time.Second}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /checkout", func(w http.ResponseWriter, r *http.Request) {
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
		defer resp.Body.Close()
		inventory, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "inventory unavailable", http.StatusBadGateway)
			return
		}

		logger.Info("checkout completed",
			"order.id", "TEST-0001",
			"customer.email", "user@example.test",
		)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(checkoutResponse{
			OrderID:       "TEST-0001",
			CustomerEmail: "user@example.test",
			Status:        "confirmed",
			Inventory:     string(inventory),
		})
	})

	addr := fmt.Sprintf(":%s", port)
	logger.Info("go-service listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}
