package agentcatalog

import (
	"context"
	"errors"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/native"
)

const listToolID = "cp__list_agents"

func init() { //nolint:gochecknoinits // register builtin on startup
	native.RegisterProvider(ListDefinition)
}

// ListDefinition registers the cp__list_agents builtin tool.
func ListDefinition(env toolenv.Environment) builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:          listToolID,
		Description: "List available agents and their actions in the current project",
		Handler:     listHandler(env),
	}
}

func listHandler(env toolenv.Environment) builtin.Handler {
	return func(ctx context.Context, _ map[string]any) (core.Output, error) {
		if env == nil || env.ResourceStore() == nil {
			return nil, builtin.Internal(errors.New("resource store unavailable"), nil)
		}
		project, err := core.GetProjectName(ctx)
		if err != nil || project == "" {
			return nil, builtin.InvalidArgument(
				errors.New("project name required"),
				map[string]any{"reason": "project_required"},
			)
		}
		catalog := NewCatalog(env.ResourceStore())
		infos, err := catalog.ListAgents(ctx, project)
		if err != nil {
			return nil, builtin.Internal(err, nil)
		}
		agents := make([]map[string]any, len(infos))
		for i := range infos {
			entry := map[string]any{"agent_id": infos[i].ID}
			if len(infos[i].Actions) > 0 {
				actions := make([]string, len(infos[i].Actions))
				copy(actions, infos[i].Actions)
				entry["actions"] = actions
			}
			agents[i] = entry
		}
		return core.Output{"agents": agents}, nil
	}
}
