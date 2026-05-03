package updatecenter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Parse removes the JSONP wrapper and unmarshals the JSON content.
// The JSONP format is: updateCenter.post(\n{...}\n);
func Parse(raw []byte) (map[string]interface{}, error) {
	// Remove JSONP wrapper: updateCenter.post(\n...\n);
	content := string(raw)

	// Remove leading whitespace and find the start of the JSON
	start := strings.Index(content, "{")
	if start == -1 {
		return nil, fmt.Errorf("invalid update center format: no JSON object found")
	}

	// Find the closing brace (accounting for nested objects)
	end := strings.LastIndex(content, "}")
	if end == -1 {
		return nil, fmt.Errorf("invalid update center format: no closing brace found")
	}

	jsonContent := content[start : end+1]

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonContent), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result, nil
}

// Render marshals the JSON and wraps it in JSONP format.
func Render(uc map[string]interface{}) ([]byte, error) {
	jsonBytes, err := json.MarshalIndent(uc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Wrap in JSONP format
	var buf bytes.Buffer
	buf.WriteString("updateCenter.post(\n")
	buf.Write(jsonBytes)
	buf.WriteString("\n);")

	return buf.Bytes(), nil
}
