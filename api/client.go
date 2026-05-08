package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultClientTimeout = 10 * time.Second
	maxResponseBytes     = 1 << 20 // 1 MB cap on the bootstrap-list response
)

// FetchBootstrapAddresses GETs <apiURL>/api/v1/addresses and decodes the
// JSON array of multiaddr strings. The body is capped at maxResponseBytes
// to defend against an upstream that streams indefinitely.
//
// The request honors ctx for both connect and read timeouts; pass
// context.Background() if you only need the implicit 10s client timeout.
func FetchBootstrapAddresses(apiURL string) ([]string, error) {
	return FetchBootstrapAddressesCtx(context.Background(), apiURL)
}

// FetchBootstrapAddressesCtx is FetchBootstrapAddresses with an explicit
// context, useful when the caller wants startup cancellation to abort the
// fetch.
func FetchBootstrapAddressesCtx(ctx context.Context, apiURL string) ([]string, error) {
	client := &http.Client{Timeout: defaultClientTimeout}

	url := fmt.Sprintf("%s/api/v1/addresses", apiURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build bootstrap API request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request to bootstrap API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bootstrap API returned non-200 status code: %d", resp.StatusCode)
	}

	var addrs []string
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&addrs); err != nil {
		return nil, fmt.Errorf("failed to decode bootstrap API json response: %w", err)
	}

	return addrs, nil
}
