package handlers

import (
	"encoding/json"
	"fmt"
	"os"

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

// outputJSONError outputs an error in JSON format
func outputJSONError(message string) error {
	errorResponse := map[string]any{
		"error": message,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(errorResponse); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", message)
		return fmt.Errorf("failed to encode JSON error response: %w", err)
	}
	return nil
}

// outputJSONResponse outputs the response as JSON
func outputJSONResponse(response map[string]any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return outputJSONError(fmt.Sprintf("failed to encode JSON response: %v", err))
	}
	return nil
}
