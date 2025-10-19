package toolrouter

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/core/httpdto"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
)

// ToolDTO is the canonical typed representation for tools.
type ToolDTO struct {
	Resource     string         `json:"resource,omitempty"`
	ID           string         `json:"id"`
	Description  string         `json:"description,omitempty"`
	Timeout      string         `json:"timeout,omitempty"`
	InputSchema  *schema.Schema `json:"input,omitempty"`
	OutputSchema *schema.Schema `json:"output,omitempty"`
	With         *core.Input    `json:"with,omitempty"`
	Config       *core.Input    `json:"config,omitempty"`
	Env          *core.EnvMap   `json:"env,omitempty"`
	Cwd          string         `json:"cwd,omitempty"`
}

// ToolListItem is the list representation and includes an optional ETag.
type ToolListItem struct {
	ToolDTO
	ETag string `json:"etag,omitempty" example:"abc123"`
}

// ToolsListResponse is the typed list payload returned from GET /tools.
type ToolsListResponse struct {
	Tools []ToolListItem      `json:"tools"`
	Page  httpdto.PageInfoDTO `json:"page"`
}

// ToToolDTOForWorkflow converts UC map payloads for workflow expansion.
func ToToolDTOForWorkflow(src map[string]any) (ToolDTO, error) {
	return toToolDTO(src)
}

// ConvertToolConfigToDTO converts a tool.Config to ToolDTO with deep-copy semantics.
func ConvertToolConfigToDTO(cfg *tool.Config) (ToolDTO, error) {
	if cfg == nil {
		return ToolDTO{}, fmt.Errorf("tool config is nil")
	}
	clone, err := core.DeepCopy(cfg)
	if err != nil {
		return ToolDTO{}, fmt.Errorf("deep copy tool config: %w", err)
	}
	return ToolDTO{
		Resource:     clone.Resource,
		ID:           clone.ID,
		Description:  clone.Description,
		Timeout:      clone.Timeout,
		InputSchema:  clone.InputSchema,
		OutputSchema: clone.OutputSchema,
		With:         clone.With,
		Config:       clone.Config,
		Env:          clone.Env,
		Cwd:          cwdPath(clone.GetCWD()),
	}, nil
}

func cwdPath(cwd *core.PathCWD) string {
	if cwd == nil {
		return ""
	}
	return cwd.PathStr()
}

func toToolDTO(src map[string]any) (ToolDTO, error) {
	cfg := &tool.Config{}
	if err := cfg.FromMap(src); err != nil {
		return ToolDTO{}, fmt.Errorf("map to tool config: %w", err)
	}
	return ConvertToolConfigToDTO(cfg)
}

func toToolListItem(src map[string]any) (ToolListItem, error) {
	dto, err := toToolDTO(src)
	if err != nil {
		return ToolListItem{}, err
	}
	return ToolListItem{ToolDTO: dto, ETag: httpdto.AsString(src["_etag"])}, nil
}
