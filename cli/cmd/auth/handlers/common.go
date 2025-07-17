package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/cli/api"
)

const (
	keyCtrlC = api.KeyCtrlC
	keyEnter = api.KeyEnter
	keyDown  = api.KeyDown

	roleUser  = api.RoleUser
	roleAdmin = api.RoleAdmin
)

// errMsg is a common message type for TUI error handling
type errMsg struct {
	err error
}

// contains checks if string s contains substr (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// outputJSONError outputs an error in JSON format
func outputJSONError(message string) error {
	errorResponse := map[string]any{
		"error": message,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(errorResponse); err != nil {
		return fmt.Errorf("failed to encode JSON error response: %w", err)
	}
	return fmt.Errorf("%s", message)
}
