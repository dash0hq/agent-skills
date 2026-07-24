// Command evalhelper backs the Docker-based fixture hooks with 2 small,
// dependency-free utilities that run as containers on the internal fixture
// network (see evals/fixtures/README.md):
//
//	evalhelper serve
//	    Stub downstream HTTP server: answers every GET with 200 and a small
//	    synthetic JSON body. Listens on PORT (default 9090).
//
//	evalhelper probe <url> [count]
//	    Traffic driver: performs count (default 3) successful GET requests
//	    against url, retrying until the target is ready or PROBE_TIMEOUT
//	    (default 60s) elapses. Exits non-zero when the requests cannot be
//	    completed, failing the scenario's Run hook.
//
// All data served is obviously synthetic (TEST-SKU-0001 style).
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("evalhelper: usage: evalhelper serve | evalhelper probe <url> [count]")
	}
	switch os.Args[1] {
	case "serve":
		serve()
	case "probe":
		if len(os.Args) < 3 {
			log.Fatal("evalhelper: usage: evalhelper probe <url> [count]")
		}
		count := 3
		if len(os.Args) >= 4 {
			n, err := strconv.Atoi(os.Args[3])
			if err != nil || n < 1 {
				log.Fatalf("evalhelper: invalid probe count %q", os.Args[3])
			}
			count = n
		}
		if err := probe(os.Args[2], count); err != nil {
			log.Fatalf("evalhelper: %v", err)
		}
	default:
		log.Fatalf("evalhelper: unknown mode %q", os.Args[1])
	}
}

// serve runs the stub downstream server the fixtures call via DOWNSTREAM_URL.
func serve() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"sku":"TEST-SKU-0001","stock":7}`)
	})
	log.Printf("evalhelper: stub downstream listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("evalhelper: serve: %v", err)
	}
}

// probe drives traffic at a fixture endpoint, retrying while it starts up.
func probe(url string, count int) error {
	timeout := 60 * time.Second
	if v := os.Getenv("PROBE_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid PROBE_TIMEOUT %q: %w", v, err)
		}
		timeout = d
	}
	client := &http.Client{Timeout: 10 * time.Second}
	deadline := time.Now().Add(timeout)

	succeeded := 0
	var lastErr error
	for succeeded < count {
		if time.Now().After(deadline) {
			return fmt.Errorf("probe %s: %d/%d requests succeeded before the %s deadline (last error: %v)", url, succeeded, count, timeout, lastErr)
		}
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		succeeded++
	}
	log.Printf("evalhelper: probe %s: %d requests succeeded", url, succeeded)
	return nil
}
