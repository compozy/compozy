package projectrouter

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
)

// ProjectDTO represents the typed API shape for the singleton project configuration.
// It exposes stable, transport-facing fields at the top level while allowing
// nested configuration blocks to remain flexible via maps for forward-compatibility.
type ProjectDTO struct {
	Name        string           `json:"name"`
	Version     string           `json:"version,omitempty"`
	Description string           `json:"description,omitempty"`
	Author      *core.Author     `json:"author,omitempty"`
	Workflows   []map[string]any `json:"workflows,omitempty"`
	Models      []map[string]any `json:"models,omitempty"`
	Schemas     []map[string]any `json:"schemas,omitempty"`
	Config      map[string]any   `json:"config,omitempty"`
	Runtime     map[string]any   `json:"runtime,omitempty"`
	AutoLoad    map[string]any   `json:"autoload,omitempty"`
	Tools       []map[string]any `json:"tools,omitempty"`
	Memories    []map[string]any `json:"memories,omitempty"`
	Monitoring  map[string]any   `json:"monitoring,omitempty"`
}

// toProjectDTO transforms a project.Config into a ProjectDTO using JSON marshaling.
// It avoids reflection-heavy mapstructure conversions used by AsMap() while keeping
// the existing transport shape intact.
func toProjectDTO(cfg *project.Config) (ProjectDTO, error) {
	if cfg == nil {
		return ProjectDTO{}, nil
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return ProjectDTO{}, fmt.Errorf("failed to marshal project config: %w", err)
	}
	var dto ProjectDTO
	if err := json.Unmarshal(encoded, &dto); err != nil {
		return ProjectDTO{}, fmt.Errorf("failed to map project config to DTO: %w", err)
	}
	return dto, nil
}
