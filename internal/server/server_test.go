package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/project"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestProjectConfig(t *testing.T, cwd string) *project.ProjectConfig {
	config := &project.ProjectConfig{}
	toolCWD, err := common.CWDFromPath(cwd)
	require.NoError(t, err)
	err = config.SetCWD(toolCWD.PathStr())
	require.NoError(t, err)
	return config
}

func Test_NewAppState(t *testing.T) {
	t.Run("Should use current directory when CWD is empty", func(t *testing.T) {
		config := createTestProjectConfig(t, "")
		state, err := NewAppState(config, nil, nil)
		assert.NoError(t, err)
		assert.NotNil(t, state)
		assert.NotEmpty(t, state.CWD)
		assert.True(t, filepath.IsAbs(state.CWD.PathStr()))
	})

	t.Run("Should convert relative path to absolute", func(t *testing.T) {
		config := createTestProjectConfig(t, "test")
		state, err := NewAppState(config, nil, nil)
		assert.NoError(t, err)
		assert.NotNil(t, state)
		assert.NotEmpty(t, state.CWD)
		assert.True(t, filepath.IsAbs(state.CWD.PathStr()))
	})

	t.Run("Should work with absolute path", func(t *testing.T) {
		config := createTestProjectConfig(t, "/tmp")
		state, err := NewAppState(config, nil, nil)
		assert.NoError(t, err)
		assert.NotNil(t, state)
		assert.NotEmpty(t, state.CWD)
		assert.True(t, filepath.IsAbs(state.CWD.PathStr()))
	})
}

func Test_AppStateContext(t *testing.T) {
	t.Run("Should handle app state in context correctly", func(t *testing.T) {
		config := createTestProjectConfig(t, "")
		state, err := NewAppState(config, nil, nil)
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
	})
}

func Test_ServerCreation(t *testing.T) {
	config := createTestProjectConfig(t, "")
	state, err := NewAppState(config, nil, nil)
	require.NoError(t, err)

	t.Run("Should create server with default config", func(t *testing.T) {
		server := NewServer(nil, state)
		assert.NotNil(t, server)
		assert.Equal(t, state, server.State)
		assert.Equal(t, "0.0.0.0", server.Config.Host)
		assert.Equal(t, 3000, server.Config.Port)
	})

	t.Run("Should create server with custom config", func(t *testing.T) {
		config := &ServerConfig{
			Host:        "localhost",
			Port:        8080,
			CORSEnabled: true,
		}
		server := NewServer(config, state)
		assert.NotNil(t, server)
		assert.Equal(t, state, server.State)
		assert.Equal(t, config.Host, server.Config.Host)
		assert.Equal(t, config.Port, server.Config.Port)
	})
}

func Test_HealthEndpoint(t *testing.T) {
	t.Run("Should return healthy status", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		config := createTestProjectConfig(t, "")
		state, err := NewAppState(config, nil, nil)
		require.NoError(t, err)

		server := NewServer(nil, state)
		err = server.buildRouter()
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "healthy")
	})
}

func Test_WebhookEndpoint(t *testing.T) {
	t.Run("Should return 404 for non-existent webhook", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		config := createTestProjectConfig(t, "")
		state, err := NewAppState(config, nil, nil)
		require.NoError(t, err)

		server := NewServer(nil, state)
		err = server.buildRouter()
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/non-existent", nil)
		req.Header.Set("Content-Type", "application/json")
		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
