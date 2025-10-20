package resolver

import (
	"context"
	"fmt"
	"sort"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
)

// hierarchicalResolver implements ToolResolver with hierarchical inheritance rules
type hierarchicalResolver struct{}

// NewHierarchicalResolver creates a new hierarchical tool resolver
func NewHierarchicalResolver() ToolResolver {
	return &hierarchicalResolver{}
}

// ResolveTools implements ToolResolver interface with hierarchical tool resolution
func (r *hierarchicalResolver) ResolveTools(
	ctx context.Context,
	projectConfig *project.Config,
	workflowConfig *workflow.Config,
	agentConfig *agent.Config,
) ([]tool.Config, error) {
	// Agent-level tools disable inheritance when present
	if agentConfig != nil && agentConfig.Tools != nil && len(agentConfig.Tools) > 0 {
		return agentConfig.Tools, nil
	}
	// Build tool map with precedence: Project -> Workflow -> Agent
	toolMap := make(map[string]tool.Config)
	// Start with project-level tools (lowest precedence)
	if err := mergeProjectTools(ctx, toolMap, projectConfig); err != nil {
		return nil, err
	}
	// Override with workflow-level tools
	if err := mergeWorkflowTools(ctx, toolMap, workflowConfig); err != nil {
		return nil, err
	}
	// FIX: Convert map back to slice with deterministic ordering
	// Sort by ID to ensure consistent order across runs
	toolIDs := make([]string, 0, len(toolMap))
	for id := range toolMap {
		toolIDs = append(toolIDs, id)
	}
	sort.Strings(toolIDs)

	result := make([]tool.Config, 0, len(toolMap))
	for _, id := range toolIDs {
		result = append(result, toolMap[id])
	}
	return result, nil
}

// mergeProjectTools adds validated project-level tools to the map with deep copies.
func mergeProjectTools(ctx context.Context, toolMap map[string]tool.Config, projectConfig *project.Config) error {
	if projectConfig == nil {
		return nil
	}
	for i := range projectConfig.Tools {
		t := &projectConfig.Tools[i]
		if t.ID == "" {
			return fmt.Errorf("tool missing ID in project config at index %d", i)
		}
		// Validate tool before adding
		if err := t.Validate(ctx); err != nil {
			return fmt.Errorf("invalid tool '%s' in project config: %w", t.ID, err)
		}
		clone, err := core.DeepCopy(t)
		if err != nil {
			return fmt.Errorf("failed to clone project tool '%s': %w", t.ID, err)
		}
		toolMap[t.ID] = *clone
	}
	return nil
}

// mergeWorkflowTools overrides map entries with validated workflow-level tools (deep copies).
func mergeWorkflowTools(ctx context.Context, toolMap map[string]tool.Config, workflowConfig *workflow.Config) error {
	if workflowConfig == nil {
		return nil
	}
	for i := range workflowConfig.Tools {
		t := &workflowConfig.Tools[i]
		if t.ID == "" {
			return fmt.Errorf("tool missing ID in workflow config at index %d", i)
		}
		// Validate tool before overriding/adding
		if err := t.Validate(ctx); err != nil {
			return fmt.Errorf("invalid tool '%s' in workflow config: %w", t.ID, err)
		}
		clone, err := core.DeepCopy(t)
		if err != nil {
			return fmt.Errorf("failed to clone workflow tool '%s': %w", t.ID, err)
		}
		toolMap[t.ID] = *clone
	}
	return nil
}
