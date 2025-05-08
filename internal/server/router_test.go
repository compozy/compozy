package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/logger"
	"github.com/compozy/compozy/internal/parser/trigger"
	"github.com/compozy/compozy/internal/parser/workflow"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Initialize logger for tests
	logger.Init(logger.DefaultConfig())
}

func Test_NormalizePath(t *testing.T) {
	t.Run("Should keep path with leading slash unchanged", func(t *testing.T) {
		result := normalizePath("/test")
		assert.Equal(t, "/test", result)
	})

	t.Run("Should add leading slash to path without it", func(t *testing.T) {
		result := normalizePath("test")
		assert.Equal(t, "/test", result)
	})

	t.Run("Should normalize path with multiple leading slashes", func(t *testing.T) {
		result := normalizePath("///test")
		assert.Equal(t, "/test", result)
	})

	t.Run("Should preserve trailing slash", func(t *testing.T) {
		result := normalizePath("test/")
		assert.Equal(t, "/test/", result)
	})

	t.Run("Should return root path for empty input", func(t *testing.T) {
		result := normalizePath("")
		assert.Equal(t, "/", result)
	})

	t.Run("Should trim spaces from path", func(t *testing.T) {
		result := normalizePath(" test ")
		assert.Equal(t, "/test", result)
	})
}

func Test_RegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Should handle empty workflows", func(t *testing.T) {
		router := gin.New()
		state, err := NewAppState("", nil)
		require.NoError(t, err)

		err = RegisterRoutes(router, state)
		assert.NoError(t, err)
	})

	t.Run("Should handle workflows without webhook triggers", func(t *testing.T) {
		router := gin.New()
		state, err := NewAppState("", []*workflow.WorkflowConfig{
			{
				ID: "test-workflow",
				Trigger: trigger.TriggerConfig{
					Type: "invalid",
				},
			},
		})
		require.NoError(t, err)

		err = RegisterRoutes(router, state)
		assert.NoError(t, err)
	})

	t.Run("Should register webhook trigger with leading slash", func(t *testing.T) {
		router := gin.New()
		state, err := NewAppState("", []*workflow.WorkflowConfig{
			{
				Trigger: trigger.TriggerConfig{
					Type: trigger.TriggerTypeWebhook,
					Config: &trigger.WebhookConfig{
						URL: "/test-webhook",
					},
				},
			},
		})
		require.NoError(t, err)

		err = RegisterRoutes(router, state)
		assert.NoError(t, err)

		// Test registered route
		rec := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/test-webhook", nil)
		router.ServeHTTP(rec, req)
		assert.NotEqual(t, http.StatusNotFound, rec.Code)
	})

	t.Run("Should normalize webhook trigger without leading slash", func(t *testing.T) {
		router := gin.New()
		state, err := NewAppState("", []*workflow.WorkflowConfig{
			{
				Trigger: trigger.TriggerConfig{
					Type: trigger.TriggerTypeWebhook,
					Config: &trigger.WebhookConfig{
						URL: "test-webhook",
					},
				},
			},
		})
		require.NoError(t, err)

		err = RegisterRoutes(router, state)
		assert.NoError(t, err)

		// Test registered route
		rec := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/test-webhook", nil)
		router.ServeHTTP(rec, req)
		assert.NotEqual(t, http.StatusNotFound, rec.Code)
	})

	t.Run("Should return error for duplicate webhook URLs", func(t *testing.T) {
		router := gin.New()
		state, err := NewAppState("", []*workflow.WorkflowConfig{
			{
				Trigger: trigger.TriggerConfig{
					Type: trigger.TriggerTypeWebhook,
					Config: &trigger.WebhookConfig{
						URL: "/test-webhook",
					},
				},
			},
			{
				Trigger: trigger.TriggerConfig{
					Type: trigger.TriggerTypeWebhook,
					Config: &trigger.WebhookConfig{
						URL: "test-webhook",
					},
				},
			},
		})
		require.NoError(t, err)

		err = RegisterRoutes(router, state)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "route conflict")
	})
}

func Test_HandleRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testWorkflow := &workflow.WorkflowConfig{
		ID: "test-workflow",
		Trigger: trigger.TriggerConfig{
			Type: trigger.TriggerTypeWebhook,
			Config: &trigger.WebhookConfig{
				URL: "/test-webhook",
			},
		},
	}

	t.Run("Should handle valid JSON request", func(t *testing.T) {
		router := gin.New()
		state, err := NewAppState("", []*workflow.WorkflowConfig{testWorkflow})
		require.NoError(t, err)

		err = RegisterRoutes(router, state)
		require.NoError(t, err)

		body, err := json.Marshal(map[string]any{"key": "value"})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/test-webhook", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "success", response["status"])
		assert.Equal(t, "Workflow triggered successfully", response["message"])
		assert.Equal(t, map[string]any{}, response["data"])
	})

	t.Run("Should handle invalid JSON request", func(t *testing.T) {
		router := gin.New()
		state, err := NewAppState("", []*workflow.WorkflowConfig{testWorkflow})
		require.NoError(t, err)

		err = RegisterRoutes(router, state)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/test-webhook", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		errorObj := response["error"].(map[string]any)
		assert.Equal(t, "INTERNAL_ERROR", errorObj["code"])
		assert.Contains(t, errorObj["message"], "Invalid JSON input")
	})

	t.Run("Should return 404 for non-existent webhook", func(t *testing.T) {
		router := gin.New()
		state, err := NewAppState("", []*workflow.WorkflowConfig{testWorkflow})
		require.NoError(t, err)

		err = RegisterRoutes(router, state)
		require.NoError(t, err)

		body, err := json.Marshal(map[string]any{"key": "value"})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/non-existent", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
