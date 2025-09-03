package resolver

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setCWDAndValidateProject(t *testing.T, p *project.Config) {
	if p == nil {
		return
	}
	require.NoError(t, p.SetCWD("."))
	for i := range p.Tools {
		require.NoError(t, p.Tools[i].SetCWD(p.CWD.PathStr()))
	}
}

func setCWDAndValidateWorkflow(t *testing.T, w *workflow.Config) {
	if w == nil {
		return
	}
	require.NoError(t, w.SetCWD("."))
	for i := range w.Tools {
		require.NoError(t, w.Tools[i].SetCWD(w.CWD.PathStr()))
	}
}

func TestHierarchicalResolver_ResolveTools(t *testing.T) {
	t.Run("Should inherit project tools when agent has no tools", func(t *testing.T) {
		// Arrange
		projectConfig := &project.Config{
			Tools: []tool.Config{
				{ID: "project-tool-1", Description: "Project tool 1"},
				{ID: "project-tool-2", Description: "Project tool 2"},
			},
		}
		workflowConfig := &workflow.Config{}
		agentConfig := &agent.Config{}
		setCWDAndValidateProject(t, projectConfig)
		setCWDAndValidateWorkflow(t, workflowConfig)
		resolver := NewHierarchicalResolver()
		// Act
		result, err := resolver.ResolveTools(projectConfig, workflowConfig, agentConfig)
		// Assert
		require.NoError(t, err)
		assert.Len(t, result, 2)
		// Check that project tools are present
		toolIDs := make(map[string]bool)
		for _, t := range result {
			toolIDs[t.ID] = true
		}
		assert.True(t, toolIDs["project-tool-1"])
		assert.True(t, toolIDs["project-tool-2"])
	})
	t.Run("Should inherit workflow tools and override project tools", func(t *testing.T) {
		// Arrange
		projectConfig := &project.Config{
			Tools: []tool.Config{
				{ID: "shared-tool", Description: "Project version"},
				{ID: "project-only", Description: "Project only tool"},
			},
		}
		workflowConfig := &workflow.Config{
			Tools: []tool.Config{
				{ID: "shared-tool", Description: "Workflow version"},
				{ID: "workflow-only", Description: "Workflow only tool"},
			},
		}
		agentConfig := &agent.Config{}
		setCWDAndValidateProject(t, projectConfig)
		setCWDAndValidateWorkflow(t, workflowConfig)
		resolver := NewHierarchicalResolver()
		// Act
		result, err := resolver.ResolveTools(projectConfig, workflowConfig, agentConfig)
		// Assert
		require.NoError(t, err)
		assert.Len(t, result, 3) // shared-tool (workflow version), project-only, workflow-only
		// Check tool precedence
		toolMap := make(map[string]string)
		for _, t := range result {
			toolMap[t.ID] = t.Description
		}
		assert.Equal(t, "Workflow version", toolMap["shared-tool"], "Workflow should override project")
		assert.Equal(t, "Project only tool", toolMap["project-only"])
		assert.Equal(t, "Workflow only tool", toolMap["workflow-only"])
	})
	t.Run("Should disable inheritance when agent has explicit tools", func(t *testing.T) {
		// Arrange
		projectConfig := &project.Config{
			Tools: []tool.Config{
				{ID: "project-tool", Description: "Project tool"},
			},
		}
		workflowConfig := &workflow.Config{
			Tools: []tool.Config{
				{ID: "workflow-tool", Description: "Workflow tool"},
			},
		}
		agentConfig := &agent.Config{
			LLMProperties: agent.LLMProperties{
				Tools: []tool.Config{
					{ID: "agent-tool", Description: "Agent tool"},
				},
			},
		}
		setCWDAndValidateProject(t, projectConfig)
		setCWDAndValidateWorkflow(t, workflowConfig)
		resolver := NewHierarchicalResolver()
		// Act
		result, err := resolver.ResolveTools(projectConfig, workflowConfig, agentConfig)
		// Assert
		require.NoError(t, err)
		assert.Len(t, result, 1, "Should only have agent tools")
		assert.Equal(t, "agent-tool", result[0].ID)
		assert.Equal(t, "Agent tool", result[0].Description)
	})
	t.Run("Should handle nil configurations gracefully", func(t *testing.T) {
		// Arrange
		resolver := NewHierarchicalResolver()
		// Act & Assert - all nil
		result, err := resolver.ResolveTools(nil, nil, nil)
		require.NoError(t, err)
		assert.Empty(t, result)
		// Act & Assert - only project
		projectConfig := &project.Config{
			Tools: []tool.Config{
				{ID: "tool1", Description: "Tool 1"},
			},
		}
		setCWDAndValidateProject(t, projectConfig)
		result, err = resolver.ResolveTools(projectConfig, nil, nil)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		// Act & Assert - only workflow
		workflowConfig := &workflow.Config{
			Tools: []tool.Config{
				{ID: "tool2", Description: "Tool 2"},
			},
		}
		setCWDAndValidateWorkflow(t, workflowConfig)
		result, err = resolver.ResolveTools(nil, workflowConfig, nil)
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})
	t.Run("Should return error for missing tool IDs in project config", func(t *testing.T) {
		// Arrange
		projectConfig := &project.Config{
			Tools: []tool.Config{
				{ID: "valid-tool", Description: "Valid tool"},
				{Description: "Missing ID"}, // No ID
			},
		}
		setCWDAndValidateProject(t, projectConfig)
		resolver := NewHierarchicalResolver()
		// Act
		result, err := resolver.ResolveTools(projectConfig, nil, nil)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool missing ID in project config")
		assert.Nil(t, result)
	})
	t.Run("Should return error for missing tool IDs in workflow config", func(t *testing.T) {
		// Arrange
		workflowConfig := &workflow.Config{
			Tools: []tool.Config{
				{ID: "valid-tool", Description: "Valid tool"},
				{Description: "Missing ID"}, // No ID
			},
		}
		setCWDAndValidateWorkflow(t, workflowConfig)
		resolver := NewHierarchicalResolver()
		// Act
		result, err := resolver.ResolveTools(nil, workflowConfig, nil)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool missing ID in workflow config")
		assert.Nil(t, result)
	})
	t.Run("Should handle empty tool arrays correctly", func(t *testing.T) {
		// Arrange
		projectConfig := &project.Config{
			Tools: []tool.Config{},
		}
		workflowConfig := &workflow.Config{
			Tools: []tool.Config{},
		}
		agentConfig := &agent.Config{
			LLMProperties: agent.LLMProperties{
				Tools: []tool.Config{},
			},
		}
		setCWDAndValidateProject(t, projectConfig)
		setCWDAndValidateWorkflow(t, workflowConfig)
		resolver := NewHierarchicalResolver()
		// Act
		result, err := resolver.ResolveTools(projectConfig, workflowConfig, agentConfig)
		// Assert
		require.NoError(t, err)
		assert.Empty(t, result)
	})
	t.Run("Should resolve tool ID collisions by precedence", func(t *testing.T) {
		// Arrange
		projectConfig := &project.Config{
			Tools: []tool.Config{
				{ID: "tool1", Description: "Project version"},
				{ID: "tool2", Description: "Project version"},
				{ID: "tool3", Description: "Project version"},
			},
		}
		workflowConfig := &workflow.Config{
			Tools: []tool.Config{
				{ID: "tool2", Description: "Workflow version"},
				{ID: "tool3", Description: "Workflow version"},
			},
		}
		agentConfig := &agent.Config{}
		// Ensure CWD is set so validation in resolver passes
		setCWDAndValidateProject(t, projectConfig)
		setCWDAndValidateWorkflow(t, workflowConfig)
		resolver := NewHierarchicalResolver()
		// Act
		result, err := resolver.ResolveTools(projectConfig, workflowConfig, agentConfig)
		// Assert
		require.NoError(t, err)
		assert.Len(t, result, 3)
		// Verify precedence
		toolMap := make(map[string]string)
		for _, t := range result {
			toolMap[t.ID] = t.Description
		}
		assert.Equal(t, "Project version", toolMap["tool1"], "tool1 should be from project")
		assert.Equal(t, "Workflow version", toolMap["tool2"], "tool2 should be overridden by workflow")
		assert.Equal(t, "Workflow version", toolMap["tool3"], "tool3 should be overridden by workflow")
	})
}
