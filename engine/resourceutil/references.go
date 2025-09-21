package resourceutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
)

func WorkflowsReferencingAgent(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	agentID string,
) ([]string, error) {
	return collectWorkflowReferences(ctx, store, project, func(cfg *workflow.Config) bool {
		for i := range cfg.Agents {
			if strings.TrimSpace(cfg.Agents[i].ID) == agentID {
				return true
			}
		}
		return false
	})
}

func WorkflowsReferencingTool(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	toolID string,
) ([]string, error) {
	return collectWorkflowReferences(ctx, store, project, func(cfg *workflow.Config) bool {
		for i := range cfg.Tools {
			if strings.TrimSpace(cfg.Tools[i].ID) == toolID {
				return true
			}
		}
		return false
	})
}

func WorkflowsReferencingTask(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	taskID string,
) ([]string, error) {
	return collectWorkflowReferences(ctx, store, project, func(cfg *workflow.Config) bool {
		for i := range cfg.Tasks {
			if strings.TrimSpace(cfg.Tasks[i].ID) == taskID {
				return true
			}
		}
		return false
	})
}

func WorkflowsReferencingSchema(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	schemaID string,
) ([]string, error) {
	id := strings.TrimSpace(schemaID)
	if id == "" {
		return nil, nil
	}
	return collectWorkflowReferences(ctx, store, project, func(cfg *workflow.Config) bool {
		return workflowUsesSchema(cfg, id)
	})
}

func workflowUsesSchema(cfg *workflow.Config, schemaID string) bool {
	if schemaRefMatches(cfg.Opts.InputSchema, schemaID) {
		return true
	}
	if triggersUseSchema(cfg, schemaID) {
		return true
	}
	if collectionsUseSchema(cfg.Tasks, schemaID) {
		return true
	}
	if toolsUseSchema(cfg.Tools, schemaID) {
		return true
	}
	for i := range cfg.Agents {
		if agentHasSchemaRefs(&cfg.Agents[i], schemaID) {
			return true
		}
	}
	return false
}

func triggersUseSchema(cfg *workflow.Config, schemaID string) bool {
	for i := range cfg.Triggers {
		trigger := &cfg.Triggers[i]
		if schemaRefMatches(trigger.Schema, schemaID) {
			return true
		}
		if trigger.Webhook == nil {
			continue
		}
		for j := range trigger.Webhook.Events {
			if schemaRefMatches(trigger.Webhook.Events[j].Schema, schemaID) {
				return true
			}
		}
	}
	return false
}

func collectionsUseSchema(tasks []task.Config, schemaID string) bool {
	for i := range tasks {
		if schemaRefMatches(tasks[i].InputSchema, schemaID) {
			return true
		}
		if schemaRefMatches(tasks[i].OutputSchema, schemaID) {
			return true
		}
	}
	return false
}

func toolsUseSchema(tools []tool.Config, schemaID string) bool {
	for i := range tools {
		if schemaRefMatches(tools[i].InputSchema, schemaID) {
			return true
		}
		if schemaRefMatches(tools[i].OutputSchema, schemaID) {
			return true
		}
	}
	return false
}

func AgentsReferencingSchema(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	schemaID string,
) ([]string, error) {
	id := strings.TrimSpace(schemaID)
	if id == "" {
		return nil, nil
	}
	items, err := store.ListWithValues(ctx, project, resources.ResourceAgent)
	if err != nil {
		return nil, err
	}
	refs := make([]string, 0)
	for i := range items {
		ag, err := decodeAgent(items[i].Value, items[i].Key.ID)
		if err != nil {
			return nil, err
		}
		if agentHasSchemaRefs(ag, schemaID) {
			refs = append(refs, ag.ID)
		}
	}
	return refs, nil
}

func agentHasSchemaRefs(cfg *agent.Config, schemaID string) bool {
	if cfg == nil {
		return false
	}
	for i := range cfg.Actions {
		a := cfg.Actions[i]
		if a == nil {
			continue
		}
		if schemaRefMatches(a.InputSchema, schemaID) || schemaRefMatches(a.OutputSchema, schemaID) {
			return true
		}
	}
	return false
}

func decodeTask(value any, id string) (*task.Config, error) {
	switch v := value.(type) {
	case *task.Config:
		return v, nil
	case task.Config:
		clone := v
		return &clone, nil
	case map[string]any:
		cfg := &task.Config{}
		if err := cfg.FromMap(v); err != nil {
			return nil, fmt.Errorf("decode task: %w", err)
		}
		if strings.TrimSpace(cfg.ID) == "" {
			cfg.ID = id
		}
		return cfg, nil
	default:
		return nil, fmt.Errorf("decode task: unsupported type %T", value)
	}
}

func TasksReferencingSchema(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	schemaID string,
) ([]string, error) {
	id := strings.TrimSpace(schemaID)
	if id == "" {
		return nil, nil
	}
	items, err := store.ListWithValues(ctx, project, resources.ResourceTask)
	if err != nil {
		return nil, err
	}
	refs := make([]string, 0)
	for i := range items {
		tk, err := decodeTask(items[i].Value, items[i].Key.ID)
		if err != nil {
			return nil, err
		}
		if schemaRefMatches(tk.InputSchema, id) || schemaRefMatches(tk.OutputSchema, id) {
			refs = append(refs, tk.ID)
		}
	}
	return refs, nil
}

func schemaRefMatches(sc *schema.Schema, schemaID string) bool {
	if sc == nil {
		return false
	}
	if ref, ok := (*sc)["__schema_ref__"]; ok {
		if s, ok2 := ref.(string); ok2 {
			return strings.TrimSpace(s) == schemaID
		}
	}
	if idVal, ok := (*sc)["id"]; ok {
		if s, ok2 := idVal.(string); ok2 {
			return strings.TrimSpace(s) == schemaID
		}
	}
	return false
}

func collectWorkflowReferences(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	match func(*workflow.Config) bool,
) ([]string, error) {
	items, err := store.ListWithValues(ctx, project, resources.ResourceWorkflow)
	if err != nil {
		return nil, err
	}
	refs := make([]string, 0)
	for i := range items {
		cfg, err := DecodeStoredWorkflow(items[i].Value, items[i].Key.ID)
		if err != nil {
			return nil, err
		}
		if match(cfg) {
			refs = append(refs, cfg.ID)
		}
	}
	return refs, nil
}

// DecodeStoredWorkflow decodes a workflow config from various stored representations
// and normalizes the ID by setting it to the provided id when empty.
func DecodeStoredWorkflow(value any, id string) (*workflow.Config, error) {
	switch v := value.(type) {
	case *workflow.Config:
		if strings.TrimSpace(v.ID) == "" {
			v.ID = id
		}
		return v, nil
	case workflow.Config:
		clone := v
		if strings.TrimSpace(clone.ID) == "" {
			clone.ID = id
		}
		return &clone, nil
	case map[string]any:
		cfg := &workflow.Config{}
		err := cfg.FromMap(v)
		if err != nil {
			return nil, fmt.Errorf("decode workflow: %w", err)
		}
		if strings.TrimSpace(cfg.ID) == "" {
			cfg.ID = id
		}
		return cfg, nil
	default:
		return nil, fmt.Errorf("decode workflow: unsupported type %T", value)
	}
}

func AgentsReferencingModel(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	modelID string,
) ([]string, error) {
	items, err := store.ListWithValues(ctx, project, resources.ResourceAgent)
	if err != nil {
		return nil, err
	}
	refs := make([]string, 0)
	for i := range items {
		ag, err := decodeAgent(items[i].Value, items[i].Key.ID)
		if err != nil {
			return nil, err
		}
		if ag.Model.HasRef() && strings.TrimSpace(ag.Model.Ref) == modelID {
			refs = append(refs, ag.ID)
		}
	}
	return refs, nil
}

func decodeAgent(value any, id string) (*agent.Config, error) {
	switch v := value.(type) {
	case *agent.Config:
		return v, nil
	case agent.Config:
		clone := v
		return &clone, nil
	case map[string]any:
		cfg := &agent.Config{}
		if err := cfg.FromMap(v); err != nil {
			return nil, fmt.Errorf("decode agent: %w", err)
		}
		if strings.TrimSpace(cfg.ID) == "" {
			cfg.ID = id
		}
		return cfg, nil
	default:
		return nil, fmt.Errorf("decode agent: unsupported type %T", value)
	}
}

func decodeTool(value any, id string) (*tool.Config, error) {
	switch v := value.(type) {
	case *tool.Config:
		return v, nil
	case tool.Config:
		clone := v
		return &clone, nil
	case map[string]any:
		cfg := &tool.Config{}
		if err := cfg.FromMap(v); err != nil {
			return nil, fmt.Errorf("decode tool: %w", err)
		}
		if strings.TrimSpace(cfg.ID) == "" {
			cfg.ID = id
		}
		return cfg, nil
	default:
		return nil, fmt.Errorf("decode tool: unsupported type %T", value)
	}
}

func ToolsReferencingSchema(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	schemaID string,
) ([]string, error) {
	id := strings.TrimSpace(schemaID)
	if id == "" {
		return nil, nil
	}
	items, err := store.ListWithValues(ctx, project, resources.ResourceTool)
	if err != nil {
		return nil, err
	}
	refs := make([]string, 0)
	for i := range items {
		tl, err := decodeTool(items[i].Value, items[i].Key.ID)
		if err != nil {
			return nil, err
		}
		if schemaRefMatches(tl.InputSchema, id) || schemaRefMatches(tl.OutputSchema, id) {
			refs = append(refs, tl.ID)
		}
	}
	return refs, nil
}

func taskReferencesTool(cfg *task.Config, toolID string) bool {
	if cfg == nil {
		return false
	}
	if cfg.Tool != nil {
		if strings.TrimSpace(cfg.Tool.ID) == toolID {
			return true
		}
	}
	return false
}

func TasksReferencingToolResources(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	toolID string,
) ([]string, error) {
	items, err := store.ListWithValues(ctx, project, resources.ResourceTask)
	if err != nil {
		return nil, err
	}
	refs := make([]string, 0)
	for i := range items {
		tk, err := decodeTask(items[i].Value, items[i].Key.ID)
		if err != nil {
			return nil, err
		}
		if taskReferencesTool(tk, toolID) {
			refs = append(refs, tk.ID)
		}
	}
	return refs, nil
}

func taskReferencesAgent(cfg *task.Config, agentID string) bool {
	if cfg == nil {
		return false
	}
	if cfg.Agent != nil {
		if strings.TrimSpace(cfg.Agent.ID) == agentID {
			return true
		}
	}
	return false
}

func AgentsReferencingMemory(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	memoryID string,
) ([]string, error) {
	items, err := store.ListWithValues(ctx, project, resources.ResourceAgent)
	if err != nil {
		return nil, err
	}
	refs := make([]string, 0)
	for i := range items {
		ag, err := decodeAgent(items[i].Value, items[i].Key.ID)
		if err != nil {
			return nil, err
		}
		for j := range ag.Memory {
			if strings.TrimSpace(ag.Memory[j].ID) == memoryID {
				refs = append(refs, ag.ID)
				break
			}
		}
	}
	return refs, nil
}

func TasksReferencingAgentResources(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	agentID string,
) ([]string, error) {
	items, err := store.ListWithValues(ctx, project, resources.ResourceTask)
	if err != nil {
		return nil, err
	}
	refs := make([]string, 0)
	for i := range items {
		tk, err := decodeTask(items[i].Value, items[i].Key.ID)
		if err != nil {
			return nil, err
		}
		if taskReferencesAgent(tk, agentID) {
			refs = append(refs, tk.ID)
		}
	}
	return refs, nil
}

func taskReferencesTask(cfg *task.Config, targetID string) bool {
	if cfg == nil {
		return false
	}
	id := strings.TrimSpace(targetID)
	if id == "" {
		return false
	}
	if cfg.OnSuccess != nil && cfg.OnSuccess.Next != nil {
		if strings.TrimSpace(*cfg.OnSuccess.Next) == id {
			return true
		}
	}
	if cfg.OnError != nil && cfg.OnError.Next != nil {
		if strings.TrimSpace(*cfg.OnError.Next) == id {
			return true
		}
	}
	if cfg.Routes != nil {
		for _, v := range cfg.Routes {
			if s, ok := v.(string); ok {
				if strings.TrimSpace(s) == id {
					return true
				}
			}
		}
	}
	return false
}

// TasksReferencingTaskResources returns task IDs that reference the given task ID
// via OnSuccess.Next, OnError.Next, or router Routes string targets.
func TasksReferencingTaskResources(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	taskID string,
) ([]string, error) {
	items, err := store.ListWithValues(ctx, project, resources.ResourceTask)
	if err != nil {
		return nil, err
	}
	refs := make([]string, 0)
	for i := range items {
		tk, err := decodeTask(items[i].Value, items[i].Key.ID)
		if err != nil {
			return nil, err
		}
		if taskReferencesTask(tk, taskID) {
			refs = append(refs, tk.ID)
		}
	}
	return refs, nil
}
