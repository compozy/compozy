package extensionpkg

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/resources"
)

var ErrExtensionAgentConflict = errors.New("extension: shipped agent conflicts with a visible agent")

// AgentConflictError names every colliding agent in stable order.
type AgentConflictError struct {
	Agents []string
}

func (e *AgentConflictError) Error() string {
	if e == nil {
		return ErrExtensionAgentConflict.Error()
	}
	agents := slices.Clone(e.Agents)
	slices.Sort(agents)
	return fmt.Sprintf("%s: %s", ErrExtensionAgentConflict, strings.Join(agents, ", "))
}

func (e *AgentConflictError) Unwrap() error { return ErrExtensionAgentConflict }

// KitItem is one shipped or live extension resource, keyed by kind and name.
type KitItem struct {
	Kind resources.ResourceKind `json:"kind"`
	ID   string                 `json:"id"`
	Name string                 `json:"name"`
	Live bool                   `json:"live"`
}

// ExtensionInventory is the shipped/live union for one extension.
type ExtensionInventory struct {
	Extension string    `json:"extension"`
	Enabled   bool      `json:"enabled"`
	Items     []KitItem `json:"items"`
}

// EnablePreview is the mutation-free view of one enable operation.
type EnablePreview struct {
	Extension                   string    `json:"extension"`
	WouldPublish                []KitItem `json:"would_publish"`
	AgentConflicts              []string  `json:"agent_conflicts"`
	MissingEnv                  []string  `json:"missing_env"`
	AutomationStarting          []string  `json:"automation_starting"`
	NetworkRequirementDigest    string    `json:"network_requirement_digest"`
	NetworkConfirmationRequired bool      `json:"network_confirmation_required"`
}
