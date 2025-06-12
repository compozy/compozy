package mcp

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid http config",
			config: Config{
				ID:  "test-server",
				URL: "http://localhost:3000/mcp",
			},
			wantErr: false,
		},
		{
			name: "valid https config",
			config: Config{
				ID:  "test-server",
				URL: "https://api.example.com/mcp",
			},
			wantErr: false,
		},
		{
			name: "default proto version is set",
			config: Config{
				ID:  "test-server",
				URL: "http://localhost:3000/mcp",
			},
			wantErr: false,
		},
		{
			name: "missing id",
			config: Config{
				URL: "http://localhost:3000/mcp",
			},
			wantErr: true,
			errMsg:  "mcp id is required",
		},
		{
			name: "missing url",
			config: Config{
				ID: "test-server",
			},
			wantErr: true,
			errMsg:  "mcp url is required",
		},
		{
			name: "invalid url format",
			config: Config{
				ID:  "test-server",
				URL: "not-a-url",
			},
			wantErr: true,
			errMsg:  "mcp url must use http or https scheme",
		},
		{
			name: "invalid url scheme",
			config: Config{
				ID:  "test-server",
				URL: "ftp://localhost:3000/mcp",
			},
			wantErr: true,
			errMsg:  "mcp url must use http or https scheme",
		},
		{
			name: "missing host",
			config: Config{
				ID:  "test-server",
				URL: "http:///mcp",
			},
			wantErr: true,
			errMsg:  "mcp url must include a host",
		},
		{
			name: "invalid proto version",
			config: Config{
				ID:    "test-server",
				URL:   "http://localhost:3000/mcp",
				Proto: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid protocol version: invalid",
		},
		{
			name: "negative timeout",
			config: Config{
				ID:           "test-server",
				URL:          "http://localhost:3000/mcp",
				StartTimeout: -1 * time.Second,
			},
			wantErr: true,
			errMsg:  "start_timeout cannot be negative",
		},
		{
			name: "negative max sessions",
			config: Config{
				ID:          "test-server",
				URL:         "http://localhost:3000/mcp",
				MaxSessions: -1,
			},
			wantErr: true,
			errMsg:  "max_sessions cannot be negative",
		},
		{
			name: "valid proxy config",
			config: Config{
				ID:       "test-server",
				ProxyURL: "http://localhost:7077",
				UseProxy: true,
			},
			wantErr: false,
		},
		{
			name: "proxy enabled but no proxy url",
			config: Config{
				ID:       "test-server",
				UseProxy: true,
			},
			wantErr: true,
			errMsg:  "proxy_url is required when use_proxy is true",
		},
		{
			name: "invalid proxy url format",
			config: Config{
				ID:       "test-server",
				ProxyURL: "not-a-url",
				UseProxy: true,
			},
			wantErr: true,
			errMsg:  "proxy url must use http or https scheme",
		},
		{
			name: "proxy with https",
			config: Config{
				ID:       "test-server",
				ProxyURL: "https://proxy.example.com:7077",
				UseProxy: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalProto := tt.config.Proto

			// Call SetDefaults before Validate for cases that have valid ID and (URL or proxy config)
			if tt.config.ID != "" && (tt.config.URL != "" || tt.config.UseProxy || tt.config.ProxyURL != "") {
				tt.config.SetDefaults()
			}

			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				// Check defaults are set when not specified
				if originalProto == "" {
					assert.Equal(t, "2025-03-26", tt.config.Proto)
				}
			}
		})
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected Config
	}{
		{
			name: "sets default proto and transport",
			config: Config{
				ID:  "test-server",
				URL: "http://localhost:3000/mcp",
			},
			expected: Config{
				ID:        "test-server",
				URL:       "http://localhost:3000/mcp",
				Proto:     DefaultProtocolVersion,
				Transport: DefaultTransport,
			},
		},
		{
			name: "preserves existing proto and transport",
			config: Config{
				ID:        "test-server",
				URL:       "http://localhost:3000/mcp",
				Proto:     "2024-01-01",
				Transport: TransportStreamableHTTP,
			},
			expected: Config{
				ID:        "test-server",
				URL:       "http://localhost:3000/mcp",
				Proto:     "2024-01-01",
				Transport: TransportStreamableHTTP,
			},
		},
		{
			name: "does not auto-enable proxy from proxy url",
			config: Config{
				ID:       "test-server",
				ProxyURL: "http://localhost:7077",
			},
			expected: Config{
				ID:        "test-server",
				ProxyURL:  "http://localhost:7077",
				UseProxy:  false, // Should NOT auto-enable
				Proto:     DefaultProtocolVersion,
				Transport: DefaultTransport,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.SetDefaults()
			assert.Equal(t, tt.expected.Proto, tt.config.Proto)
			assert.Equal(t, tt.expected.Transport, tt.config.Transport)
			assert.Equal(t, tt.expected.UseProxy, tt.config.UseProxy)
			assert.Equal(t, tt.expected.ProxyURL, tt.config.ProxyURL)
		})
	}
}

func TestConfig_Clone(t *testing.T) {
	original := &Config{
		ID:  "test-server",
		URL: "http://localhost:3000/mcp",
		Env: map[string]string{
			"NODE_ENV": "production",
			"PORT":     "3000",
		},
		Proto:        "2025-03-26",
		StartTimeout: 30 * time.Second,
		MaxSessions:  10,
		ProxyURL:     "http://localhost:7077",
		UseProxy:     true,
	}

	clone := original.Clone()

	// Check that all fields are copied
	assert.Equal(t, original.ID, clone.ID)
	assert.Equal(t, original.URL, clone.URL)
	assert.Equal(t, original.Env, clone.Env)
	assert.Equal(t, original.Proto, clone.Proto)
	assert.Equal(t, original.StartTimeout, clone.StartTimeout)
	assert.Equal(t, original.MaxSessions, clone.MaxSessions)
	assert.Equal(t, original.ProxyURL, clone.ProxyURL)
	assert.Equal(t, original.UseProxy, clone.UseProxy)

	// Check that maps are deep copied
	clone.Env["NODE_ENV"] = "development"

	assert.NotEqual(t, original.Env["NODE_ENV"], clone.Env["NODE_ENV"])
}

func TestIsValidProtoVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"valid version", "2025-03-26", true},
		{"valid version with different date", "2024-12-01", true},
		{"invalid format - no dashes", "20250326", false},
		{"invalid format - too many parts", "2025-03-26-01", false},
		{"invalid format - too few parts", "2025-03", false},
		{"invalid format - wrong year length", "25-03-26", false},
		{"invalid format - wrong month length", "2025-3-26", false},
		{"invalid format - wrong day length", "2025-03-6", false},
		{"invalid format - non-numeric", "2025-MM-DD", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidProtoVersion(tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfig_EnvironmentVariables(t *testing.T) {
	// Save original environment
	originalProxyURL := os.Getenv("MCP_PROXY_URL")
	originalUseProxy := os.Getenv("MCP_USE_PROXY")

	// Clean up after test
	defer func() {
		os.Setenv("MCP_PROXY_URL", originalProxyURL)
		os.Setenv("MCP_USE_PROXY", originalUseProxy)
	}()

	tests := []struct {
		name        string
		envProxyURL string
		envUseProxy string
		config      Config
		expectedURL string
		expectedUse bool
	}{
		{
			name:        "proxy url from environment without explicit use proxy",
			envProxyURL: "http://localhost:7077",
			envUseProxy: "",
			config:      Config{ID: "test"},
			expectedURL: "http://localhost:7077",
			expectedUse: false, // Should NOT auto-enable proxy
		},
		{
			name:        "use proxy from environment",
			envProxyURL: "",
			envUseProxy: "true",
			config:      Config{ID: "test"},
			expectedURL: "",
			expectedUse: true,
		},
		{
			name:        "proxy url from env but use proxy disabled",
			envProxyURL: "http://localhost:7077",
			envUseProxy: "false",
			config:      Config{ID: "test"},
			expectedURL: "http://localhost:7077",
			expectedUse: false,
		},
		{
			name:        "config overrides environment",
			envProxyURL: "http://localhost:7077",
			envUseProxy: "false",
			config:      Config{ID: "test", ProxyURL: "http://custom:8080", UseProxy: true},
			expectedURL: "http://custom:8080",
			expectedUse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			os.Setenv("MCP_PROXY_URL", tt.envProxyURL)
			os.Setenv("MCP_USE_PROXY", tt.envUseProxy)

			tt.config.SetDefaults()

			assert.Equal(t, tt.expectedURL, tt.config.ProxyURL)
			assert.Equal(t, tt.expectedUse, tt.config.UseProxy)
		})
	}
}
