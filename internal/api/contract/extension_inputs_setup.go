package contract

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/extensioninput"
)

// Extension input setup states. Clients key prompting and UI off these, never off the set boolean.
const (
	ExtensionInputStateSet       = "set"
	ExtensionInputStateMissing   = "missing"
	ExtensionInputStateInvalid   = "invalid"
	ExtensionInputStateDaemonEnv = "daemon_env"
)

// ExtensionInputStatusPayload describes one declared input and its state, never its value or reference.
type ExtensionInputStatusPayload struct {
	ID       string                         `json:"id"`
	Prompt   string                         `json:"prompt"`
	Type     string                         `json:"type"`
	Required bool                           `json:"required"`
	Binding  MarketplaceInputBindingPayload `json:"binding"`
	Default  json.RawMessage                `json:"default,omitempty"`
	Targets  []string                       `json:"targets"`
	State    string                         `json:"state"`
}

// ExtensionInputsPayload is the setup view of one extension instance. Changed and Reloaded are set by writes.
type ExtensionInputsPayload struct {
	Extension      string                        `json:"extension"`
	WorkspaceID    string                        `json:"workspace_id"`
	Profile        string                        `json:"profile"`
	Enabled        bool                          `json:"enabled"`
	Inputs         []ExtensionInputStatusPayload `json:"inputs"`
	Missing        []string                      `json:"missing"`
	Invalid        []string                      `json:"invalid"`
	DaemonEnv      []string                      `json:"daemon_env"`
	MissingEnv     []string                      `json:"missing_env"`
	MCPAuthServers []string                      `json:"mcp_auth_servers"`
	Changed        []string                      `json:"changed,omitempty"`
	Reloaded       bool                          `json:"reloaded,omitzero"`
}

// SetExtensionInputsRequest merges typed values or Vault references into one instance; omitted inputs are unchanged.
type SetExtensionInputsRequest struct {
	Inputs map[string]extensioninput.Value `json:"inputs"`
}
