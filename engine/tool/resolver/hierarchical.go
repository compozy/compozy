package resolver

import (
	"fmt"
	"sort"

	"github.com/compozy/compozy/engine/agent"
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
	projectConfig *project.Config,
	workflowConfig *workflow.Config,
	agentConfig *agent.Config,
) ([]tool.Config, error) {
	// Explicit agent tools disable all inheritance
	// CRITICAL FIX: Check the correct nested path for agent tools
	if agentConfig != nil && agentConfig.Tools != nil && len(agentConfig.Tools) > 0 {
		return agentConfig.Tools, nil
	}
	// Build tool map with precedence: Project -> Workflow -> Agent
	toolMap := make(map[string]tool.Config)
	// Start with project-level tools (lowest precedence)
	if projectConfig != nil {
		for i := range projectConfig.Tools {
			t := &projectConfig.Tools[i]
			if t.ID == "" {
				return nil, fmt.Errorf("tool missing ID in project config at index %d", i)
			}
			// Validate tool before adding
			if err := t.Validate(); err != nil {
				return nil, fmt.Errorf("invalid tool '%s' in project config: %w", t.ID, err)
			}
			toolMap[t.ID] = *t
		}
	}
	// Override with workflow-level tools
	if workflowConfig != nil {
		for i := range workflowConfig.Tools {
			t := &workflowConfig.Tools[i]
			if t.ID == "" {
				return nil, fmt.Errorf("tool missing ID in workflow config at index %d", i)
			}
			// Validate tool before overriding/adding
			if err := t.Validate(); err != nil {
				return nil, fmt.Errorf("invalid tool '%s' in workflow config: %w", t.ID, err)
			}
			toolMap[t.ID] = *t
		}
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
