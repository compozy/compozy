package automation

import (
	"errors"
	"strings"
)

// ExtensionTriggerRequest describes one extension-originated trigger fire.
type ExtensionTriggerRequest struct {
	Event       string         `json:"event"`
	Scope       Scope          `json:"scope"`
	WorkspaceID string         `json:"workspace_id,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
}

// Validate ensures the extension trigger request matches the ext.* ingress contract.
func (r ExtensionTriggerRequest) Validate(path string) error {
	event := strings.TrimSpace(r.Event)
	if event == "" {
		return errors.New(nestedPath(path, "event") + " is required")
	}
	if event != r.Event {
		return errors.New(nestedPath(path, "event") + " must not contain surrounding whitespace")
	}
	if !strings.HasPrefix(event, "ext.") {
		return errors.New(nestedPath(path, "event") + " must start with \"ext.\"")
	}
	if err := ValidateScopeBinding(r.Scope, r.WorkspaceID, path, "workspace_id"); err != nil {
		return err
	}
	return nil
}
