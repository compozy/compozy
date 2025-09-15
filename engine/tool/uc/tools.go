package uc

import (
	"context"
	"errors"

	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
)

// -----------------------------------------------------------------------------
// GetTool
// -----------------------------------------------------------------------------

// ErrToolNotFound is returned when a tool cannot be found in any workflow.
var ErrToolNotFound = errors.New("tool not found")

type GetTool struct {
	workflows []*workflow.Config
	toolID    string
}

func NewGetTool(workflows []*workflow.Config, toolID string) *GetTool {
	return &GetTool{
		workflows: workflows,
		toolID:    toolID,
	}
}

func (uc *GetTool) Execute(_ context.Context) (*tool.Config, error) {
	for _, wf := range uc.workflows {
		for i := range wf.Tools {
			if wf.Tools[i].ID == uc.toolID {
				return &wf.Tools[i], nil
			}
		}
	}
	return nil, ErrToolNotFound
}

// -----------------------------------------------------------------------------
// ListTools
// -----------------------------------------------------------------------------

type ListTools struct {
	workflows []*workflow.Config
}

func NewListTools(workflows []*workflow.Config) *ListTools {
	return &ListTools{
		workflows: workflows,
	}
}

func (uc *ListTools) Execute(_ context.Context) ([]tool.Config, error) {
	tools := make([]tool.Config, 0)
	seen := make(map[string]bool)

	for _, wf := range uc.workflows {
		for i := range wf.Tools {
			if !seen[wf.Tools[i].ID] {
				tools = append(tools, wf.Tools[i])
				seen[wf.Tools[i].ID] = true
			}
		}
	}

	return tools, nil
}
