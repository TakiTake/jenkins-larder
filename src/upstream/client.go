package upstream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TODO (T017): Implement upstream HTTP client

// Client handles communication with Jenkins Update Center
type Client struct {
	BaseURL        string
	HTTPClient     *http.Client
	TimeoutSeconds int
}

// NewClient creates a new upstream client
func NewClient(baseURL string, timeoutSeconds int) *Client {
	return &Client{
		BaseURL:        baseURL,
		TimeoutSeconds: timeoutSeconds,
		HTTPClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}
}

// DownloadPlugin downloads a plugin from the upstream Jenkins Update Center
func (c *Client) DownloadPlugin(ctx context.Context, name, version, extension string) (io.ReadCloser, error) {
	url := c.PluginURL(name, version, extension)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Jenkins-Plugin-Cache-Mirror/1.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download plugin: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// CheckPluginExists verifies if a plugin version exists upstream
func (c *Client) CheckPluginExists(ctx context.Context, name, version, extension string) (bool, error) {
	url := c.PluginURL(name, version, extension)

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	} else if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

// GetPluginMetadata retrieves metadata for a plugin from update center
// TODO (T039): Implement metadata retrieval from update-center.json
func (c *Client) GetPluginMetadata(ctx context.Context, name string) (map[string]interface{}, error) {
	// TODO: Download update-center.json
	// TODO: Parse JSON
	// TODO: Extract plugin metadata
	return nil, fmt.Errorf("not implemented")
}
