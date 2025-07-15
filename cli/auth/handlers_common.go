package auth

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

const (
	keyCtrlC    = "ctrl+c"
	keyEnter    = "enter"
	keyDown     = "down"
	roleUser    = "user"
	roleAdmin   = "admin"
	sortName    = "name"
	sortEmail   = "email"
	sortRole    = "role"
	sortCreated = "created"
)

// outputJSONError outputs an error in JSON format
func outputJSONError(message string) error {
	response := map[string]any{
		"error": message,
	}
	encoder := json.NewEncoder(os.Stderr)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		// If we can't encode the error response, just return the original error
		return errors.New(message)
	}
	// Return error to indicate failure (JSON was written to stderr)
	return errors.New(message)
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// sortKeys sorts a slice of KeyInfo based on the specified field

// Common message types for TUI models
type errMsg struct{ err error }
type clipboardCopiedMsg struct{}
