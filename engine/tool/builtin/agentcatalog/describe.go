package agentcatalog

import (
	"context"
	"errors"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/native"
)

const describeToolID = "cp__describe_agent"

func init() { //nolint:gochecknoinits // register builtin provider at startup
	native.RegisterProvider(DescribeDefinition)
}

// DescribeDefinition registers the cp__describe_agent builtin tool.
func DescribeDefinition(env toolenv.Environment) builtin.BuiltinDefinition {
	// Simple input schema for {agent_id:string}
	input := schema.Schema(map[string]any{
		"type":     "object",
		"required": []any{"agent_id"},
		"properties": map[string]any{
			"agent_id": map[string]any{"type": "string"},
		},
	})
	return builtin.BuiltinDefinition{
		ID:          describeToolID,
		Description: "Describe an agent's available actions and input hints",
		Handler:     describeHandler(env),
		InputSchema: &input,
	}
}

func describeHandler(env toolenv.Environment) builtin.Handler {
	return func(ctx context.Context, payload map[string]any) (core.Output, error) {
		if env == nil || env.ResourceStore() == nil {
			return nil, builtin.Internal(errors.New("resource store unavailable"), nil)
		}
		agentID, ok := payload["agent_id"].(string)
		if !ok || agentID == "" {
			return nil, builtin.InvalidArgument(
				errors.New("agent_id is required"),
				map[string]any{"field": "agent_id"},
			)
		}
		project, err := core.GetProjectName(ctx)
		if err != nil || project == "" {
			return nil, builtin.InvalidArgument(
				errors.New("project name required"),
				map[string]any{"reason": "project_required"},
			)
		}
		catalog := NewCatalog(env.ResourceStore())
		desc, err := catalog.DescribeAgent(ctx, project, agentID)
		if err != nil {
			if errors.Is(err, resources.ErrNotFound) {
				return nil, builtin.InvalidArgument(
					errors.New("agent not found"),
					map[string]any{"agent_id": agentID},
				)
			}
			return nil, builtin.Internal(err, nil)
		}
		actions := make([]map[string]any, len(desc.Actions))
		for i := range desc.Actions {
			entry := map[string]any{"id": desc.Actions[i].ID}
			if desc.Actions[i].Prompt != "" {
				entry["prompt"] = desc.Actions[i].Prompt
			}
			actions[i] = entry
		}
		output := core.Output{"agent_id": desc.ID}
		if len(actions) > 0 {
			output["actions"] = actions
		}
		return output, nil
	}
}
