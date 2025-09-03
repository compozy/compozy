package resolver

import (
	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
)

// ToolResolver resolves the final set of tools for an agent based on
// hierarchical inheritance rules
type ToolResolver interface {
	// ResolveTools returns the final tool configuration for an agent
	// considering project, workflow, and agent-level definitions
	//
	// Resolution Rules:
	// 1. If agent.Tools is non-empty, return agent.Tools (disable inheritance)
	// 2. Otherwise, merge tools with precedence: Project < Workflow < Agent
	// 3. Tool ID collisions are resolved by higher precedence
	// 4. All returned tools must have valid, non-empty IDs
	// 5. Returns error if any tool lacks required ID field
	ResolveTools(
		projectConfig *project.Config,
		workflowConfig *workflow.Config,
		agentConfig *agent.Config,
	) ([]tool.Config, error)
}
