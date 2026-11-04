package contract

import (
	"time"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
)

type HeartbeatSourceOutcomePayload = heartbeat.SourceOutcome
type HeartbeatSourceScriptPayload struct {
	LineCount    int        `json:"line_count,omitzero"`
	LastModified *time.Time `json:"last_modified,omitempty"`
	Name         string     `json:"name"`
	// Path is the script's absolute file path, for opening it in the operator's editor.
	Path           string  `json:"path,omitempty"`
	Executable     bool    `json:"executable"`
	SHA256         string  `json:"sha256"`
	Approved       bool    `json:"approved"`
	ApprovedSHA256 *string `json:"approved_sha256"`
}
type HeartbeatSourceLimitsPayload struct {
	MaxPerAgent int `json:"max_per_agent"`
	MaxItems    int `json:"max_items"`
}
type HeartbeatSourceCatalogPayload struct {
	Agent          string                         `json:"agent"`
	ScriptsEnabled bool                           `json:"scripts_enabled"`
	Limits         HeartbeatSourceLimitsPayload   `json:"limits"`
	Kinds          []source.KindDescriptor        `json:"kinds"`
	Scripts        []HeartbeatSourceScriptPayload `json:"scripts"`
}
type HeartbeatScriptApprovalRequest struct {
	SHA256 string `json:"sha256"`
}
type HeartbeatScriptApprovalPayload struct {
	Script     string    `json:"script"`
	SHA256     string    `json:"sha256"`
	ApprovedAt time.Time `json:"approved_at"`
}
