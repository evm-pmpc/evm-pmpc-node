package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func FetchBootstrapAddresses(apiURL string) ([]string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(fmt.Sprintf("%s/api/v1/addresses", apiURL))
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request to bootstrap API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bootstrap API returned non-200 status code: %d", resp.StatusCode)
	}

	var addrs []string
	if err := json.NewDecoder(resp.Body).Decode(&addrs); err != nil {
		return nil, fmt.Errorf("failed to decode bootstrap API json response: %w", err)
	}

	return addrs, nil
}
