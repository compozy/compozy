package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

func renderJSONPreview(value any) (string, error) {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("cli: render JSON preview: %w", err)
	}
	return string(content), nil
}

func firstCLIValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
