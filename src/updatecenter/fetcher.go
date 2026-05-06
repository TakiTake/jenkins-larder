package updatecenter

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Fetch downloads the raw JSONP update-center.json from the upstream URL.
func Fetch(ctx context.Context, client *http.Client, upstreamURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", upstreamURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Jenkins-Plugin-Cache-Mirror/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch update center: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update center fetch failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}
