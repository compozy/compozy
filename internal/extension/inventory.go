package extensionpkg

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/resources"
)

// ErrExtensionAgentConflict reports that a shipped agent would shadow a visible agent.
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
	Kind     resources.ResourceKind `json:"kind"`
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Live     bool                   `json:"live"`
	SpecJSON []byte                 `json:"-"`
}
