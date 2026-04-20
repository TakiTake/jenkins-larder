package upstream

import (
	"fmt"
)

// TODO (T018): Implement URL construction matching Jenkins Update Center pattern

// PluginURL constructs the upstream URL for a plugin download
// Pattern: {baseURL}/download/plugins/{name}/{version}/{name}.{extension}
func (c *Client) PluginURL(name, version, extension string) string {
	filename := fmt.Sprintf("%s.%s", name, extension)
	return fmt.Sprintf("%s/download/plugins/%s/%s/%s",
		c.BaseURL, name, version, filename)
}

// UpdateCenterURL returns the URL to the update-center.json
func (c *Client) UpdateCenterURL() string {
	return fmt.Sprintf("%s/update-center.json", c.BaseURL)
}

// ParsePluginURL extracts plugin name, version, and extension from a download URL
func ParsePluginURL(path string) (name, version, extension string, err error) {
	// Expected pattern: /download/plugins/{name}/{version}/{name}.{extension}
	// Example: /download/plugins/git/4.11.0/git.hpi

	// Split path into segments
	parts := []string{}
	for _, part := range path {
		if part != '/' {
			if len(parts) == 0 {
				parts = append(parts, string(part))
			} else {
				parts[len(parts)-1] += string(part)
			}
		} else if len(parts) > 0 && parts[len(parts)-1] != "" {
			parts = append(parts, "")
		}
	}

	// Filter empty parts
	filtered := []string{}
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	parts = filtered

	// Need at least: ["download", "plugins", name, version, filename]
	if len(parts) < 5 {
		return "", "", "", fmt.Errorf("invalid plugin URL path: %s", path)
	}

	if parts[0] != "download" || parts[1] != "plugins" {
		return "", "", "", fmt.Errorf("invalid plugin URL pattern: %s", path)
	}

	name = parts[2]
	version = parts[3]
	filename := parts[4]

	// Extract extension from filename (e.g., "git.hpi" -> "hpi")
	dotIndex := -1
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			dotIndex = i
			break
		}
	}

	if dotIndex == -1 {
		return "", "", "", fmt.Errorf("filename missing extension: %s", filename)
	}

	extension = filename[dotIndex+1:]
	expectedFilename := name + "." + extension

	if filename != expectedFilename {
		return "", "", "", fmt.Errorf("filename mismatch: expected %s, got %s", expectedFilename, filename)
	}

	return name, version, extension, nil
}
