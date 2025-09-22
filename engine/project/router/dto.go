package projectrouter

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
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

// toProjectDTO maps a UC map payload (from project.Config.AsMap) to ProjectDTO.
// Keep mappers pure and independent from HTTP frameworks.
func toProjectDTO(src map[string]any) ProjectDTO {
	dto := ProjectDTO{
		Name:        router.AsString(src["name"]),
		Version:     router.AsString(src["version"]),
		Description: router.AsString(src["description"]),
		Author:      toAuthor(src["author"]),
		Workflows:   toSliceOfMaps(src["workflows"]),
		Models:      toSliceOfMaps(src["models"]),
		Schemas:     toSliceOfMaps(src["schemas"]),
		Config:      router.AsMap(src["config"]),
		Runtime:     router.AsMap(src["runtime"]),
		AutoLoad:    router.AsMap(src["autoload"]),
		Tools:       toSliceOfMaps(src["tools"]),
		Memories:    toSliceOfMaps(src["memories"]),
		Monitoring:  router.AsMap(src["monitoring"]),
	}
	return dto
}

func toAuthor(v any) *core.Author {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	a := &core.Author{}
	if name, ok2 := m["name"].(string); ok2 {
		a.Name = name
	}
	if email, ok2 := m["email"].(string); ok2 {
		a.Email = email
	}
	if org, ok2 := m["organization"].(string); ok2 {
		a.Organization = org
	}
	if url, ok2 := m["url"].(string); ok2 {
		a.URL = url
	}
	return a
}

func toSliceOfMaps(v any) []map[string]any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []map[string]any:
		return t
	case []any:
		res := make([]map[string]any, 0, len(t))
		for i := range t {
			if m := router.AsMap(t[i]); m != nil {
				res = append(res, m)
			}
		}
		return res
	default:
		return nil
	}
}
