package mcpproxy

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransportType_IsValid(t *testing.T) {
	tests := []struct {
		transport TransportType
		expected  bool
	}{
		{TransportStdio, true},
		{TransportSSE, true},
		{TransportStreamableHTTP, true},
		{TransportType("invalid"), false},
		{TransportType(""), false},
	}

	for _, test := range tests {
		t.Run(string(test.transport), func(t *testing.T) {
			assert.Equal(t, test.expected, test.transport.IsValid())
		})
	}
}

func TestTransportType_String(t *testing.T) {
	assert.Equal(t, "stdio", TransportStdio.String())
	assert.Equal(t, "sse", TransportSSE.String())
	assert.Equal(t, "streamable-http", TransportStreamableHTTP.String())
}

func TestMCPDefinition_Validate(t *testing.T) {
	t.Run("Valid stdio definition", func(t *testing.T) {
		def := &MCPDefinition{
			Name:      "test-server",
			Transport: TransportStdio,
			Command:   "/usr/bin/test",
		}

		assert.NoError(t, def.Validate())
	})

	t.Run("Valid SSE definition", func(t *testing.T) {
		def := &MCPDefinition{
			Name:      "test-server",
			Transport: TransportSSE,
			URL:       "https://example.com/sse",
		}

		assert.NoError(t, def.Validate())
	})

	t.Run("Valid streamable-http definition", func(t *testing.T) {
		def := &MCPDefinition{
			Name:      "test-server",
			Transport: TransportStreamableHTTP,
			URL:       "https://example.com/stream",
		}

		assert.NoError(t, def.Validate())
	})

	t.Run("Missing name", func(t *testing.T) {
		def := &MCPDefinition{
			Transport: TransportStdio,
			Command:   "/usr/bin/test",
		}

		err := def.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("Invalid transport", func(t *testing.T) {
		def := &MCPDefinition{
			Name:      "test-server",
			Transport: TransportType("invalid"),
		}

		err := def.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid transport type")
	})

	t.Run("Stdio without command", func(t *testing.T) {
		def := &MCPDefinition{
			Name:      "test-server",
			Transport: TransportStdio,
		}

		err := def.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "command is required")
	})

	t.Run("SSE without URL", func(t *testing.T) {
		def := &MCPDefinition{
			Name:      "test-server",
			Transport: TransportSSE,
		}

		err := def.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "url is required")
	})

	t.Run("Negative timeout", func(t *testing.T) {
		def := &MCPDefinition{
			Name:      "test-server",
			Transport: TransportSSE,
			URL:       "https://example.com",
			Timeout:   -5 * time.Second,
		}

		err := def.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout cannot be negative")
	})

	t.Run("Negative max reconnects", func(t *testing.T) {
		def := &MCPDefinition{
			Name:          "test-server",
			Transport:     TransportStdio,
			Command:       "/usr/bin/test",
			MaxReconnects: -1,
		}

		err := def.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "maxReconnects cannot be negative")
	})

	t.Run("Invalid tool filter", func(t *testing.T) {
		def := &MCPDefinition{
			Name:      "test-server",
			Transport: TransportStdio,
			Command:   "/usr/bin/test",
			ToolFilter: &ToolFilter{
				Mode: ToolFilterMode("invalid"),
				List: []string{"tool1"},
			},
		}

		err := def.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool filter validation failed")
	})
}

func TestToolFilter_Validate(t *testing.T) {
	t.Run("Valid allow filter", func(t *testing.T) {
		filter := &ToolFilter{
			Mode: ToolFilterAllow,
			List: []string{"tool1", "tool2"},
		}

		assert.NoError(t, filter.Validate())
	})

	t.Run("Valid block filter", func(t *testing.T) {
		filter := &ToolFilter{
			Mode: ToolFilterBlock,
			List: []string{"tool1"},
		}

		assert.NoError(t, filter.Validate())
	})

	t.Run("Invalid mode", func(t *testing.T) {
		filter := &ToolFilter{
			Mode: ToolFilterMode("invalid"),
			List: []string{"tool1"},
		}

		err := filter.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tool filter mode")
	})

	t.Run("Empty list", func(t *testing.T) {
		filter := &ToolFilter{
			Mode: ToolFilterAllow,
			List: []string{},
		}

		err := filter.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool filter list cannot be empty")
	})
}

func TestMCPDefinition_SetDefaults(t *testing.T) {
	def := &MCPDefinition{
		Name:               "test-server",
		Transport:          TransportSSE,
		URL:                "https://example.com",
		AutoReconnect:      true,
		HealthCheckEnabled: true,
	}

	def.SetDefaults()

	assert.False(t, def.CreatedAt.IsZero())
	assert.False(t, def.UpdatedAt.IsZero())
	assert.Equal(t, 30*time.Second, def.Timeout)
	assert.Equal(t, 5, def.MaxReconnects)
	assert.Equal(t, 5*time.Second, def.ReconnectDelay)
	assert.Equal(t, 30*time.Second, def.HealthCheckInterval)
	assert.NotNil(t, def.Headers)
	assert.NotNil(t, def.Tags)
}

func TestMCPDefinition_SetDefaults_Stdio(t *testing.T) {
	def := &MCPDefinition{
		Name:      "test-server",
		Transport: TransportStdio,
		Command:   "/usr/bin/test",
	}

	def.SetDefaults()

	assert.NotNil(t, def.Env)
	assert.Equal(t, time.Duration(0), def.Timeout) // No default timeout for stdio
}

func TestMCPDefinition_Clone(t *testing.T) {
	original := &MCPDefinition{
		Name:      "test-server",
		Transport: TransportStdio,
		Command:   "/usr/bin/test",
		Args:      []string{"arg1", "arg2"},
		Env:       map[string]string{"KEY": "value"},
		Tags:      map[string]string{"tag": "value"},
	}

	clone := original.Clone()

	assert.Equal(t, original.Name, clone.Name)
	assert.Equal(t, original.Transport, clone.Transport)
	assert.Equal(t, original.Command, clone.Command)
	assert.Equal(t, original.Args, clone.Args)
	assert.Equal(t, original.Env, clone.Env)
	assert.Equal(t, original.Tags, clone.Tags)

	// Verify it's a deep copy
	clone.Name = "modified"
	clone.Args[0] = "modified"
	clone.Env["KEY"] = "modified"
	clone.Tags["tag"] = "modified"

	assert.Equal(t, "test-server", original.Name)
	assert.Equal(t, "arg1", original.Args[0])
	assert.Equal(t, "value", original.Env["KEY"])
	assert.Equal(t, "value", original.Tags["tag"])
}

func TestMCPDefinition_GetNamespace(t *testing.T) {
	def := &MCPDefinition{Name: "test-server"}
	assert.Equal(t, "mcp_proxy:test-server", def.GetNamespace())
}

func TestMCPDefinition_JSON(t *testing.T) {
	def := &MCPDefinition{
		Name:        "test-server",
		Description: "A test server",
		Transport:   TransportStdio,
		Command:     "/usr/bin/test",
		Args:        []string{"--verbose"},
		Env:         map[string]string{"DEBUG": "true"},
	}

	def.SetDefaults()

	// Test ToJSON
	jsonData, err := def.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, string(jsonData), "test-server")
	assert.Contains(t, string(jsonData), "stdio")

	// Test FromJSON
	restored, err := FromJSON(jsonData)
	require.NoError(t, err)

	assert.Equal(t, def.Name, restored.Name)
	assert.Equal(t, def.Description, restored.Description)
	assert.Equal(t, def.Transport, restored.Transport)
	assert.Equal(t, def.Command, restored.Command)
	assert.Equal(t, def.Args, restored.Args)
	assert.Equal(t, def.Env, restored.Env)
}

func TestFromJSON_InvalidData(t *testing.T) {
	t.Run("Invalid JSON", func(t *testing.T) {
		_, err := FromJSON([]byte("invalid json"))
		assert.Error(t, err)
	})

	t.Run("Invalid definition", func(t *testing.T) {
		invalidDef := map[string]any{
			"name":      "",
			"transport": "invalid",
		}
		jsonData, _ := json.Marshal(invalidDef)

		_, err := FromJSON(jsonData)
		assert.Error(t, err)
	})
}

func TestNewMCPStatus(t *testing.T) {
	status := NewMCPStatus("test-server")

	assert.Equal(t, "test-server", status.Name)
	assert.Equal(t, StatusDisconnected, status.Status)
	assert.Equal(t, 0, status.ReconnectAttempts)
	assert.Equal(t, int64(0), status.TotalRequests)
	assert.Equal(t, int64(0), status.TotalErrors)
}

func TestMCPStatus_UpdateStatus(t *testing.T) {
	status := NewMCPStatus("test-server")

	t.Run("Connected status", func(t *testing.T) {
		status.UpdateStatus(StatusConnected, "")

		assert.Equal(t, StatusConnected, status.Status)
		assert.NotNil(t, status.LastConnected)
		assert.Empty(t, status.LastError)
		assert.Nil(t, status.LastErrorTime)
		assert.Equal(t, 0, status.ReconnectAttempts)
	})

	t.Run("Error status", func(t *testing.T) {
		status.UpdateStatus(StatusError, "connection failed")

		assert.Equal(t, StatusError, status.Status)
		assert.Equal(t, "connection failed", status.LastError)
		assert.NotNil(t, status.LastErrorTime)
		assert.Equal(t, int64(1), status.TotalErrors)
	})

	t.Run("Connecting status", func(t *testing.T) {
		status.UpdateStatus(StatusConnecting, "")

		assert.Equal(t, StatusConnecting, status.Status)
		assert.Equal(t, 1, status.ReconnectAttempts)
	})
}

func TestMCPStatus_RecordRequest(t *testing.T) {
	status := NewMCPStatus("test-server")

	// First request
	status.RecordRequest(100 * time.Millisecond)
	assert.Equal(t, int64(1), status.TotalRequests)
	assert.Equal(t, 100*time.Millisecond, status.AvgResponseTime)

	// Second request (should update average)
	status.RecordRequest(200 * time.Millisecond)
	assert.Equal(t, int64(2), status.TotalRequests)
	// Average should be between 100ms and 200ms (exponential moving average)
	assert.Greater(t, status.AvgResponseTime, 100*time.Millisecond)
	assert.Less(t, status.AvgResponseTime, 200*time.Millisecond)
}

func TestMCPStatus_CalculateUpTime(t *testing.T) {
	status := NewMCPStatus("test-server")

	t.Run("Not connected", func(t *testing.T) {
		upTime := status.CalculateUpTime()
		assert.Equal(t, time.Duration(0), upTime)
	})

	t.Run("Connected", func(t *testing.T) {
		now := time.Now()
		status.LastConnected = &now
		status.Status = StatusConnected

		time.Sleep(10 * time.Millisecond)
		upTime := status.CalculateUpTime()
		assert.Greater(t, upTime, 5*time.Millisecond)
	})
}
