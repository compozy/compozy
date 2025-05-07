package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/parser/workflow"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAppState(t *testing.T) {
	tests := []struct {
		name      string
		cwd       string
		workflows []*workflow.WorkflowConfig
		wantErr   bool
	}{
		{
			name:      "empty cwd should use current directory",
			cwd:       "",
			workflows: nil,
			wantErr:   false,
		},
		{
			name:      "relative path should be converted to absolute",
			cwd:       "test",
			workflows: nil,
			wantErr:   false,
		},
		{
			name:      "absolute path should work",
			cwd:       "/tmp",
			workflows: nil,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := NewAppState(tt.cwd, tt.workflows)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, state)
			assert.NotEmpty(t, state.CWD)
			assert.True(t, filepath.IsAbs(state.CWD))
		})
	}
}

func TestAppStateContext(t *testing.T) {
	state, err := NewAppState("", nil)
	require.NoError(t, err)

	ctx := context.Background()
	ctxWithState := WithAppState(ctx, state)

	// Test GetAppState with valid context
	retrievedState, err := GetAppState(ctxWithState)
	assert.NoError(t, err)
	assert.Equal(t, state, retrievedState)

	// Test GetAppState with invalid context
	_, err = GetAppState(ctx)
	assert.Error(t, err)
}

func TestServerCreation(t *testing.T) {
	state, err := NewAppState("", nil)
	require.NoError(t, err)

	tests := []struct {
		name   string
		config *ServerConfig
		state  *AppState
	}{
		{
			name:   "with default config",
			config: nil,
			state:  state,
		},
		{
			name: "with custom config",
			config: &ServerConfig{
				Host:        "localhost",
				Port:        8080,
				CORSEnabled: true,
			},
			state: state,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(tt.config, tt.state)
			assert.NotNil(t, server)
			assert.Equal(t, tt.state, server.State)
			if tt.config == nil {
				assert.Equal(t, "0.0.0.0", server.Config.Host)
				assert.Equal(t, 3000, server.Config.Port)
			} else {
				assert.Equal(t, tt.config.Host, server.Config.Host)
				assert.Equal(t, tt.config.Port, server.Config.Port)
			}
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	state, err := NewAppState("", nil)
	require.NoError(t, err)

	server := NewServer(nil, state)
	err = server.buildRouter()
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestWebhookEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	state, err := NewAppState("", nil)
	require.NoError(t, err)

	server := NewServer(nil, state)
	err = server.buildRouter()
	require.NoError(t, err)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{
			name:       "non-existent webhook",
			method:     "POST",
			path:       "/non-existent",
			body:       `{"key": "value"}`,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, tt.path, nil)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
				req.Body = http.NoBody // TODO: Add proper request body
			}
			server.router.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
