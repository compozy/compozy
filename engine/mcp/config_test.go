package mcp

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
)

func TestConfig_SetDefaults(t *testing.T) {
	t.Run("Should set default protocol and transport", func(t *testing.T) {
		config := &Config{}
		config.SetDefaults()

		assert.Equal(t, DefaultProtocolVersion, config.Proto)
		assert.Equal(t, DefaultTransport, config.Transport)
	})

	t.Run("Should not override existing values", func(t *testing.T) {
		config := &Config{
			Proto:     "2024-01-01",
			Transport: mcpproxy.TransportStdio,
		}
		config.SetDefaults()

		assert.Equal(t, "2024-01-01", config.Proto)
		assert.Equal(t, mcpproxy.TransportStdio, config.Transport)
	})
}

func TestConfig_Validate(t *testing.T) {
	t.Run("Should validate successfully with all required fields", func(t *testing.T) {
		os.Setenv("MCP_PROXY_URL", "http://localhost:6001")
		defer os.Unsetenv("MCP_PROXY_URL")

		config := &Config{
			ID:        "test-mcp",
			URL:       "http://localhost:3000",
			Proto:     "2025-01-01",
			Transport: mcpproxy.TransportSSE,
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should fail validation when ID is missing", func(t *testing.T) {
		config := &Config{
			URL: "http://localhost:3000",
		}

		err := config.Validate()
		assert.EqualError(t, err, "mcp id is required")
	})

	t.Run("Should fail validation when URL is missing", func(t *testing.T) {
		config := &Config{
			ID: "test-mcp",
		}

		err := config.Validate()
		assert.EqualError(t, err, "mcp url is required for HTTP transports (sse, streamable-http)")
	})
}

func TestConfig_validateURL(t *testing.T) {
	t.Run("Should validate valid HTTP URL", func(t *testing.T) {
		config := &Config{URL: "http://localhost:3000", Transport: mcpproxy.TransportSSE}
		err := config.validateURL()
		assert.NoError(t, err)
	})

	t.Run("Should validate valid HTTPS URL", func(t *testing.T) {
		config := &Config{URL: "https://api.example.com/mcp", Transport: mcpproxy.TransportStreamableHTTP}
		err := config.validateURL()
		assert.NoError(t, err)
	})

	t.Run("Should fail with invalid scheme", func(t *testing.T) {
		config := &Config{URL: "ftp://localhost:3000", Transport: mcpproxy.TransportSSE}
		err := config.validateURL()
		assert.EqualError(t, err, "mcp url must use http or https scheme, got: ftp")
	})

	t.Run("Should fail with missing host", func(t *testing.T) {
		config := &Config{URL: "http://", Transport: mcpproxy.TransportSSE}
		err := config.validateURL()
		assert.EqualError(t, err, "mcp url must include a host")
	})

	t.Run("Should fail with malformed URL", func(t *testing.T) {
		config := &Config{URL: "not-a-url", Transport: mcpproxy.TransportSSE}
		err := config.validateURL()
		// The URL "not-a-url" is parsed as a relative URL with no scheme,
		// so it fails the scheme validation instead of format validation
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mcp url must use http or https scheme")
	})
}

func TestConfig_validateProxy(t *testing.T) {
	t.Run("Should validate valid proxy URL", func(t *testing.T) {
		os.Setenv("MCP_PROXY_URL", "http://localhost:6001")
		defer os.Unsetenv("MCP_PROXY_URL")

		config := &Config{}
		err := config.validateProxy()
		assert.NoError(t, err)
	})

	t.Run("Should fail when MCP_PROXY_URL is not set", func(t *testing.T) {
		os.Unsetenv("MCP_PROXY_URL")

		config := &Config{}
		err := config.validateProxy()
		assert.EqualError(t, err, "MCP_PROXY_URL environment variable is required for MCP server configuration")
	})

	t.Run("Should fail with invalid proxy URL scheme", func(t *testing.T) {
		os.Setenv("MCP_PROXY_URL", "ftp://localhost:6001")
		defer os.Unsetenv("MCP_PROXY_URL")

		config := &Config{}
		err := config.validateProxy()
		assert.EqualError(t, err, "proxy url must use http or https scheme, got: ftp")
	})

	t.Run("Should fail with missing proxy URL host", func(t *testing.T) {
		os.Setenv("MCP_PROXY_URL", "http://")
		defer os.Unsetenv("MCP_PROXY_URL")

		config := &Config{}
		err := config.validateProxy()
		assert.EqualError(t, err, "proxy url must include a host")
	})
}

func TestIsValidProtoVersion(t *testing.T) {
	t.Run("Should accept valid date formats", func(t *testing.T) {
		validDates := []string{
			"2025-01-01",
			"2024-12-31",
			"2023-02-28",
			"2024-02-29", // leap year
		}

		for _, date := range validDates {
			assert.True(t, isValidProtoVersion(date), "Expected %s to be valid", date)
		}
	})

	t.Run("Should reject invalid date formats", func(t *testing.T) {
		invalidDates := []string{
			"2025-13-01", // invalid month
			"2025-02-30", // invalid day for February
			"2023-02-29", // not a leap year
			"2025-00-01", // invalid month
			"2025-01-00", // invalid day
			"2025-1-1",   // wrong format
			"25-01-01",   // wrong year format
			"2025/01/01", // wrong separator
			"not-a-date", // not a date
			"",           // empty
		}

		for _, date := range invalidDates {
			assert.False(t, isValidProtoVersion(date), "Expected %s to be invalid", date)
		}
	})
}

func TestIsValidTransport(t *testing.T) {
	t.Run("Should accept valid transport types", func(t *testing.T) {
		validTransports := []mcpproxy.TransportType{
			mcpproxy.TransportSSE,
			mcpproxy.TransportStreamableHTTP,
			mcpproxy.TransportStdio,
		}

		for _, transport := range validTransports {
			assert.True(t, isValidTransport(transport), "Expected %s to be valid", transport)
		}
	})

	t.Run("Should reject invalid transport types", func(t *testing.T) {
		invalidTransports := []mcpproxy.TransportType{
			"invalid",
			"",
			"websocket",
		}

		for _, transport := range invalidTransports {
			assert.False(t, isValidTransport(transport), "Expected %s to be invalid", transport)
		}
	})
}

func TestConfig_Clone(t *testing.T) {
	t.Run("Should create deep copy of config", func(t *testing.T) {
		original := &Config{
			ID:           "test-mcp",
			URL:          "http://localhost:3000",
			Env:          map[string]string{"KEY": "value"},
			Proto:        "2025-01-01",
			Transport:    mcpproxy.TransportSSE,
			StartTimeout: 30 * time.Second,
			MaxSessions:  10,
		}

		clone, err := original.Clone()
		assert.NoError(t, err)

		// Verify all fields are copied
		assert.Equal(t, original.ID, clone.ID)
		assert.Equal(t, original.URL, clone.URL)
		assert.Equal(t, original.Proto, clone.Proto)
		assert.Equal(t, original.Transport, clone.Transport)
		assert.Equal(t, original.StartTimeout, clone.StartTimeout)
		assert.Equal(t, original.MaxSessions, clone.MaxSessions)

		// Verify env is deep copied
		assert.Equal(t, original.Env, clone.Env)
		clone.Env["NEW_KEY"] = "new_value"
		assert.NotEqual(t, original.Env, clone.Env)
		assert.NotContains(t, original.Env, "NEW_KEY")
	})
}

func TestValidateURLFormat(t *testing.T) {
	t.Run("Should validate HTTP and HTTPS URLs", func(t *testing.T) {
		validURLs := []string{
			"http://localhost:3000",
			"https://api.example.com",
			"http://127.0.0.1:6001/path",
			"https://subdomain.example.com:443/api/v1",
		}

		for _, url := range validURLs {
			err := validateURLFormat(url, "test url")
			assert.NoError(t, err, "Expected %s to be valid", url)
		}
	})

	t.Run("Should reject invalid schemes", func(t *testing.T) {
		invalidURLs := []string{
			"ftp://localhost:3000",
			"ws://localhost:3000",
			"file:///path/to/file",
		}

		for _, url := range invalidURLs {
			err := validateURLFormat(url, "test url")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "test url must use http or https scheme")
		}
	})

	t.Run("Should reject URLs without host", func(t *testing.T) {
		invalidURLs := []string{
			"http://",
			"https://",
		}

		for _, url := range invalidURLs {
			err := validateURLFormat(url, "test url")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "test url must include a host")
		}
	})

	t.Run("Should include context in error messages", func(t *testing.T) {
		err := validateURLFormat("invalid-url", "custom context")
		require.Error(t, err)
		// The URL "invalid-url" is parsed as a relative URL with no scheme,
		// so it fails the scheme validation with context
		assert.Contains(t, err.Error(), "custom context must use http or https scheme")
	})
}
