package tool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/tool"
)

// TestToolInheritance_HierarchicalResolution verifies tools are correctly inherited
// from project to workflow to agent configurations
func TestToolInheritance_HierarchicalResolution(t *testing.T) {
	t.Run("Should inherit tools from all levels when agent has no tools", func(t *testing.T) {
		// Arrange
		projectConfig := CreateTestProjectConfig([]tool.Config{
			CreateTestTool("tool-A", "Project tool A"),
			CreateTestTool("tool-B", "Project tool B"),
		})
		workflowConfig := CreateTestWorkflowConfig([]tool.Config{
			CreateTestTool("tool-C", "Workflow tool C"),
		})
		agentConfig := CreateTestAgentConfig([]tool.Config{}) // Empty tools

		// Act
		resolvedTools, err := ResolveToolsWithHierarchy(t.Context(), projectConfig, workflowConfig, agentConfig)

		// Assert
		require.NoError(t, err)
		AssertToolsEqual(t, []string{"tool-A", "tool-B", "tool-C"}, resolvedTools)
	})

	t.Run("Should inherit only project tools when workflow and agent have no tools", func(t *testing.T) {
		// Arrange
		projectConfig := CreateTestProjectConfig([]tool.Config{
			CreateTestTool("tool-X", "Project tool X"),
			CreateTestTool("tool-Y", "Project tool Y"),
		})
		workflowConfig := CreateTestWorkflowConfig([]tool.Config{}) // Empty
		agentConfig := CreateTestAgentConfig([]tool.Config{})       // Empty

		// Act
		resolvedTools, err := ResolveToolsWithHierarchy(t.Context(), projectConfig, workflowConfig, agentConfig)

		// Assert
		require.NoError(t, err)
		AssertToolsEqual(t, []string{"tool-X", "tool-Y"}, resolvedTools)
	})

	t.Run("Should handle nil configurations gracefully", func(t *testing.T) {
		// Act - all configs are nil
		resolvedTools, err := ResolveToolsWithHierarchy(t.Context(), nil, nil, nil)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, resolvedTools, "Should return empty list for nil configs")
	})
}

// TestToolInheritance_PrecedenceOverride verifies that higher-level configurations
// override lower-level ones correctly
func TestToolInheritance_PrecedenceOverride(t *testing.T) {
	t.Run("Should override project tool with workflow tool of same ID", func(t *testing.T) {
		// Arrange
		projectConfig := CreateTestProjectConfig([]tool.Config{
			CreateTestTool("shared-tool", "Project version"),
			CreateTestTool("tool-1", "Project tool 1"),
		})
		workflowConfig := CreateTestWorkflowConfig([]tool.Config{
			CreateTestTool("shared-tool", "Workflow version"), // Override
			CreateTestTool("tool-2", "Workflow tool 2"),
		})
		agentConfig := CreateTestAgentConfig([]tool.Config{}) // Empty

		// Act
		resolvedTools, err := ResolveToolsWithHierarchy(t.Context(), projectConfig, workflowConfig, agentConfig)

		// Assert
		require.NoError(t, err)
		AssertToolsEqual(t, []string{"shared-tool", "tool-1", "tool-2"}, resolvedTools)
		AssertToolPrecedence(t, resolvedTools, "shared-tool", "Workflow version")
	})

	t.Run("Should use only agent tools when agent has tools defined", func(t *testing.T) {
		// Arrange
		projectConfig := CreateTestProjectConfig([]tool.Config{
			CreateTestTool("tool-1", "Project tool 1"),
			CreateTestTool("tool-2", "Project tool 2"),
		})
		workflowConfig := CreateTestWorkflowConfig([]tool.Config{
			CreateTestTool("tool-3", "Workflow tool 3"),
			CreateTestTool("tool-4", "Workflow tool 4"),
		})
		agentConfig := CreateTestAgentConfig([]tool.Config{
			CreateTestTool("tool-5", "Agent tool 5"),
		})

		// Act
		resolvedTools, err := ResolveToolsWithHierarchy(t.Context(), projectConfig, workflowConfig, agentConfig)

		// Assert
		require.NoError(t, err)
		AssertToolsEqual(t, []string{"tool-5"}, resolvedTools)
		assert.Len(t, resolvedTools, 1, "Should only have agent tools")
	})
}

// TestToolInheritance_DeterministicOrdering ensures tools are always returned
// in the same alphabetical order
func TestToolInheritance_DeterministicOrdering(t *testing.T) {
	t.Run("Should maintain alphabetical order across multiple resolutions", func(t *testing.T) {
		// Arrange
		projectConfig := CreateTestProjectConfig([]tool.Config{
			CreateTestTool("zebra", "Last alphabetically"),
			CreateTestTool("alpha", "First alphabetically"),
			CreateTestTool("gamma", "Third alphabetically"),
			CreateTestTool("beta", "Second alphabetically"),
		})

		// Act - resolve multiple times
		for range 10 {
			resolvedTools, err := ResolveToolsWithHierarchy(t.Context(), projectConfig, nil, nil)

			// Assert - same order every time
			require.NoError(t, err)
			AssertToolsEqual(t, []string{"alpha", "beta", "gamma", "zebra"}, resolvedTools)
		}
	})
}

// TestToolInheritance_LLMServiceIntegration verifies that resolved tools are
// correctly integrated with the LLM service
func TestToolInheritance_LLMServiceIntegration(t *testing.T) {
	t.Run("Should create LLM service with resolved tools", func(t *testing.T) {
		// Arrange
		ctx := t.Context()
		projectConfig := CreateTestProjectConfig([]tool.Config{
			CreateTestTool("project-tool", "Project level tool"),
		})
		workflowConfig := CreateTestWorkflowConfig([]tool.Config{
			CreateTestTool("workflow-tool", "Workflow level tool"),
		})
		agentConfig := CreateTestAgentConfig([]tool.Config{}) // Empty to test inheritance

		// Act
		resolvedTools, err := ResolveToolsWithHierarchy(t.Context(), projectConfig, workflowConfig, agentConfig)
		require.NoError(t, err)

		service := CreateLLMServiceWithResolvedTools(ctx, t, resolvedTools, agentConfig)

		// Assert
		assert.NotNil(t, service, "LLM service should be created")
		assert.Len(t, resolvedTools, 2, "Should have 2 resolved tools")
		AssertToolsEqual(t, []string{"project-tool", "workflow-tool"}, resolvedTools)
	})

	t.Run("Should maintain backward compatibility with direct agent tools", func(t *testing.T) {
		// Arrange
		ctx := t.Context()
		agentConfig := CreateTestAgentConfig([]tool.Config{
			CreateTestTool("direct-tool", "Directly configured tool"),
		})

		// Act - use agent tools directly
		resolvedTools, err := ResolveToolsWithHierarchy(t.Context(), nil, nil, agentConfig)
		require.NoError(t, err)

		service := CreateLLMServiceWithResolvedTools(ctx, t, resolvedTools, agentConfig)

		// Assert
		assert.NotNil(t, service, "LLM service should be created")
		AssertToolsEqual(t, []string{"direct-tool"}, resolvedTools)
	})
}

// TestToolInheritance_EdgeCases handles edge cases and error conditions
func TestToolInheritance_EdgeCases(t *testing.T) {
	t.Run("Should handle empty tool arrays gracefully", func(t *testing.T) {
		// Arrange
		projectConfig := CreateTestProjectConfig([]tool.Config{})
		workflowConfig := CreateTestWorkflowConfig([]tool.Config{})
		agentConfig := CreateTestAgentConfig([]tool.Config{})

		// Act
		resolvedTools, err := ResolveToolsWithHierarchy(t.Context(), projectConfig, workflowConfig, agentConfig)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, resolvedTools, "Should return empty list for empty configs")
	})

	t.Run("Should handle duplicate tool IDs with last-one-wins precedence", func(t *testing.T) {
		// Arrange
		projectConfig := CreateTestProjectConfig([]tool.Config{
			CreateTestTool("duplicate", "First version"),
			CreateTestTool("duplicate", "Second version"),
		})
		workflowConfig := CreateTestWorkflowConfig([]tool.Config{
			CreateTestTool("duplicate", "Third version"),
		})

		// Act
		resolvedTools, err := ResolveToolsWithHierarchy(t.Context(), projectConfig, workflowConfig, nil)

		// Assert
		require.NoError(t, err)
		assert.Len(t, resolvedTools, 1, "Should have only one tool after deduplication")
		AssertToolPrecedence(t, resolvedTools, "duplicate", "Third version")
	})

	t.Run("Should handle mixed nil and non-nil configurations", func(t *testing.T) {
		// Arrange
		workflowConfig := CreateTestWorkflowConfig([]tool.Config{
			CreateTestTool("workflow-tool", "Workflow tool"),
		})

		// Act - project and agent are nil
		resolvedTools, err := ResolveToolsWithHierarchy(t.Context(), nil, workflowConfig, nil)

		// Assert
		require.NoError(t, err)
		AssertToolsEqual(t, []string{"workflow-tool"}, resolvedTools)
	})
}

// TestToolInheritance_ComplexScenarios tests more complex real-world scenarios
func TestToolInheritance_ComplexScenarios(t *testing.T) {
	t.Run("Should handle multi-level override correctly", func(t *testing.T) {
		// Arrange - complex scenario with multiple overrides
		projectConfig := CreateTestProjectConfig([]tool.Config{
			CreateTestTool("tool-A", "Project A"),
			CreateTestTool("tool-B", "Project B"),
			CreateTestTool("shared", "Project shared"),
		})
		workflowConfig := CreateTestWorkflowConfig([]tool.Config{
			CreateTestTool("tool-B", "Workflow B override"),
			CreateTestTool("tool-C", "Workflow C"),
			CreateTestTool("shared", "Workflow shared override"),
		})
		agentConfig := CreateTestAgentConfig([]tool.Config{}) // Empty to inherit

		// Act
		resolvedTools, err := ResolveToolsWithHierarchy(t.Context(), projectConfig, workflowConfig, agentConfig)

		// Assert
		require.NoError(t, err)
		AssertToolsEqual(t, []string{"shared", "tool-A", "tool-B", "tool-C"}, resolvedTools)
		AssertToolPrecedence(t, resolvedTools, "tool-B", "Workflow B override")
		AssertToolPrecedence(t, resolvedTools, "shared", "Workflow shared override")
		AssertToolPrecedence(t, resolvedTools, "tool-A", "Project A")
	})
}
