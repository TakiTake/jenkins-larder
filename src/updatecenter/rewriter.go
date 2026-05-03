package updatecenter

import (
	"strings"
)

// RewritePluginURLs rewrites all plugin download URLs in the update center JSON
// to point to the agent's base URL instead of updates.jenkins.io.
func RewritePluginURLs(uc map[string]interface{}, agentBaseURL string) error {
	// Rewrite plugin URLs
	if plugins, ok := uc["plugins"].(map[string]interface{}); ok {
		for _, plugin := range plugins {
			if pluginMap, ok := plugin.(map[string]interface{}); ok {
				if url, ok := pluginMap["url"].(string); ok {
					// Replace the upstream base URL with the agent base URL
					newURL := rewriteURL(url, agentBaseURL)
					pluginMap["url"] = newURL
				}
			}
		}
	}

	// Rewrite core URL if present
	if core, ok := uc["core"].(map[string]interface{}); ok {
		if url, ok := core["url"].(string); ok {
			newURL := rewriteURL(url, agentBaseURL)
			core["url"] = newURL
		}
	}

	return nil
}

// rewriteURL takes a URL and replaces the base URL with the agent's base URL.
// For example: https://updates.jenkins.io/download/plugins/git/1.0.0/git.hpi
// becomes: http://agent-base-url/download/plugins/git/1.0.0/git.hpi
func rewriteURL(originalURL, agentBaseURL string) string {
	// Find the path part of the URL (starting with /download or /war, etc.)
	// Common patterns: /download/plugins/..., /download/war/...
	const downloadPrefix = "/download/"

	if idx := strings.Index(originalURL, downloadPrefix); idx != -1 {
		path := originalURL[idx:]

		// Ensure agent base URL doesn't end with /
		agentBaseURL = strings.TrimSuffix(agentBaseURL, "/")

		return agentBaseURL + path
	}

	// If we can't find /download/, just return the original URL
	return originalURL
}
