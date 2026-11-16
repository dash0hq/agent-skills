// Checkout handler: looks up inventory with one outbound HTTP call and returns
// a synthetic order. There is deliberately no OpenTelemetry code here.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
)

const inventoryURL = "https://checkip.amazonaws.com/"

var httpClient = &http.Client{Timeout: 5 * time.Second}

type response struct {
	StatusCode int    `json:"statusCode"`
	Body       string `json:"body"`
}

func handleRequest(ctx context.Context, _ json.RawMessage) (response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, inventoryURL, nil)
	if err != nil {
		return response{}, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return response{}, fmt.Errorf("inventory lookup: %w", err)
	}
	defer resp.Body.Close()
	inventory, err := io.ReadAll(resp.Body)
	if err != nil {
		return response{}, fmt.Errorf("inventory lookup: %w", err)
	}
	log.Print(`{"message":"checkout completed","order.id":"TEST-0001"}`)
	body, err := json.Marshal(map[string]string{
		"order_id":       "TEST-0001",
		"customer_email": "user@example.test",
		"status":         "confirmed",
		"inventory":      strings.TrimSpace(string(inventory)),
	})
	if err != nil {
		return response{}, err
	}
	return response{StatusCode: http.StatusOK, Body: string(body)}, nil
}

func main() {
	lambda.Start(handleRequest)
}
