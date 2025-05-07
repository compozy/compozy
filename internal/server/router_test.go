package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/parser/trigger"
	"github.com/compozy/compozy/internal/parser/workflow"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path with leading slash",
			input:    "/test",
			expected: "/test",
		},
		{
			name:     "path without leading slash",
			input:    "test",
			expected: "/test",
		},
		{
			name:     "path with multiple leading slashes",
			input:    "///test",
			expected: "/test",
		},
		{
			name:     "path with trailing slash",
			input:    "test/",
			expected: "/test/",
		},
		{
			name:     "empty path",
			input:    "",
			expected: "/",
		},
		{
			name:     "path with spaces",
			input:    " test ",
			expected: "/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		workflows   []*workflow.WorkflowConfig
		wantErr     bool
		errContains string
		testPaths   []string // Additional paths to test
	}{
		{
			name:      "empty workflows",
			workflows: nil,
			wantErr:   false,
		},
		{
			name: "no webhook triggers",
			workflows: []*workflow.WorkflowConfig{
				{
					ID: "test-workflow",
					Trigger: trigger.TriggerConfig{
						Type: "invalid",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid webhook trigger with leading slash",
			workflows: []*workflow.WorkflowConfig{
				{
					Trigger: trigger.TriggerConfig{
						Type: trigger.TriggerTypeWebhook,
						Config: &trigger.WebhookConfig{
							URL: "/test-webhook",
						},
					},
				},
			},
			wantErr:   false,
			testPaths: []string{"/test-webhook"},
		},
		{
			name: "valid webhook trigger without leading slash",
			workflows: []*workflow.WorkflowConfig{
				{
					Trigger: trigger.TriggerConfig{
						Type: trigger.TriggerTypeWebhook,
						Config: &trigger.WebhookConfig{
							URL: "test-webhook",
						},
					},
				},
			},
			wantErr:   false,
			testPaths: []string{"/test-webhook"},
		},
		{
			name: "duplicate webhook URLs",
			workflows: []*workflow.WorkflowConfig{
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
			},
			wantErr:     true,
			errContains: "route conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new router for each test case
			router := gin.New()
			state, err := NewAppState("", tt.workflows)
			require.NoError(t, err)

			err = RegisterRoutes(router, state)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)

			// Test registered routes
			if tt.workflows != nil {
				for _, workflow := range tt.workflows {
					if workflow.Trigger.Type == trigger.TriggerTypeWebhook && workflow.Trigger.Config != nil {
						// Test both with and without leading slash
						paths := []string{string(workflow.Trigger.Config.URL)}
						if tt.testPaths != nil {
							paths = tt.testPaths
						}

						for _, path := range paths {
							rec := httptest.NewRecorder()
							req, _ := http.NewRequest("POST", path, nil)
							router.ServeHTTP(rec, req)
							assert.NotEqual(t, http.StatusNotFound, rec.Code, "Route should be registered for path: %s", path)
						}
					}
				}
			}
		})
	}
}

func TestHandleRequest(t *testing.T) {
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

	tests := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
		wantBody   map[string]any
	}{
		{
			name:       "valid JSON request",
			method:     "POST",
			path:       "/test-webhook",
			body:       map[string]any{"key": "value"},
			wantStatus: http.StatusOK,
			wantBody: map[string]any{
				"status":  "success",
				"message": "Workflow triggered successfully",
				"data":    map[string]any{},
			},
		},
		{
			name:       "valid JSON request without leading slash",
			method:     "POST",
			path:       "/test-webhook",
			body:       map[string]any{"key": "value"},
			wantStatus: http.StatusOK,
			wantBody: map[string]any{
				"status":  "success",
				"message": "Workflow triggered successfully",
				"data":    map[string]any{},
			},
		},
		{
			name:       "invalid JSON request",
			method:     "POST",
			path:       "/test-webhook",
			body:       "invalid json",
			wantStatus: http.StatusBadRequest,
			wantBody: map[string]any{
				"status":  float64(http.StatusBadRequest),
				"message": "Invalid JSON input: invalid character 'i' looking for beginning of value",
			},
		},
		{
			name:       "non-existent webhook",
			method:     "POST",
			path:       "/non-existent",
			body:       map[string]any{"key": "value"},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new router for each test case
			router := gin.New()
			state, err := NewAppState("", []*workflow.WorkflowConfig{testWorkflow})
			require.NoError(t, err)

			err = RegisterRoutes(router, state)
			require.NoError(t, err)

			// Create request
			var reqBody io.Reader
			if tt.body != nil {
				switch v := tt.body.(type) {
				case string:
					reqBody = strings.NewReader(v)
				default:
					body, err := json.Marshal(v)
					require.NoError(t, err)
					reqBody = bytes.NewBuffer(body)
				}
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, tt.path, reqBody)
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.wantBody != nil {
				var response map[string]any
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				// Check response fields
				for key, value := range tt.wantBody {
					assert.Equal(t, value, response[key])
				}
			}
		})
	}
}
