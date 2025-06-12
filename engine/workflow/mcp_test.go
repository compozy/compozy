package workflow

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMCPWorkflow(t *testing.T) {
	t.Run("Should load MCP workflow configuration successfully", func(t *testing.T) {
		cwd, err := core.CWDFromPath("./fixtures")
		require.NoError(t, err)

		config, err := Load(cwd, "mcp_workflow.yaml")
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify basic workflow properties
		assert.Equal(t, "mcp-test-workflow", config.ID)
		assert.Equal(t, "1.0.0", config.Version)
		assert.Equal(t, "Test workflow with MCP server integration", config.Description)
	})

	t.Run("Should parse MCP server configurations correctly", func(t *testing.T) {
		cwd, err := core.CWDFromPath("./fixtures")
		require.NoError(t, err)

		config, err := Load(cwd, "mcp_workflow.yaml")
		require.NoError(t, err)

		// Verify MCP configurations
		assert.Len(t, config.MCPs, 2)

		// Check primary MCP server
		primaryMCP := config.MCPs[0]
		assert.Equal(t, "primary-mcp-server", primaryMCP.ID)
		assert.Equal(t, "http://localhost:4000/mcp", primaryMCP.URL)
		assert.Equal(t, "{{ .env.MCP_API_KEY }}", primaryMCP.Env["API_KEY"])

		// Check secondary MCP server
		secondaryMCP := config.MCPs[1]
		assert.Equal(t, "secondary-mcp-server", secondaryMCP.ID)
		assert.Equal(t, "https://api.example.com/mcp", secondaryMCP.URL)
		assert.Equal(t, "{{ .env.EXTERNAL_MCP_TOKEN }}", secondaryMCP.Env["AUTH_TOKEN"])
	})

	t.Run("Should pass validation for valid MCP configuration", func(t *testing.T) {
		cwd, err := core.CWDFromPath("./fixtures")
		require.NoError(t, err)

		config, err := Load(cwd, "mcp_workflow.yaml")
		require.NoError(t, err)

		err = config.Validate()
		assert.NoError(t, err)
	})
}

func TestMCPWorkflowValidation(t *testing.T) {
	t.Run("Should validate individual MCP configurations", func(t *testing.T) {
		cwd, err := core.CWDFromPath("./fixtures")
		require.NoError(t, err)

		config, err := Load(cwd, "mcp_workflow.yaml")
		require.NoError(t, err)

		// Test that MCP configs are validated
		for i := range config.MCPs {
			config.MCPs[i].SetDefaults()
			err := config.MCPs[i].Validate()
			assert.NoError(t, err)
		}
	})
}
