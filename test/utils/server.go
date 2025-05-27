package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/orchestrator"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// HTTPTestBed provides a complete HTTP testing environment
type HTTPTestBed struct {
	*IntegrationTestBed
	Server     *httptest.Server
	Router     *gin.Engine
	AppState   *appstate.State
	BaseURL    string
	HTTPClient *http.Client
}

// SetupHTTPTestBed creates a complete HTTP testing environment
func SetupHTTPTestBed(t *testing.T, testTimeout time.Duration) *HTTPTestBed {
	t.Helper()

	// Use shared NATS server for better performance
	natsServer, natsClient := GetSharedNatsServer(t)

	// Setup base integration test bed with shared NATS
	integrationTB := SetupIntegrationTestBedWithNats(t, testTimeout, []core.ComponentType{
		core.ComponentWorkflow,
		core.ComponentTask,
		core.ComponentAgent,
		core.ComponentTool,
	}, natsServer, natsClient)

	// Create orchestrator
	orchConfig := orchestrator.Config{
		WorkflowRepoFactory: func() workflow.Repository {
			return integrationTB.WorkflowRepo
		},
		TaskRepoFactory: func() task.Repository {
			return integrationTB.TaskRepo
		},
		AgentRepoFactory: func() agent.Repository {
			return integrationTB.AgentRepo
		},
		ToolRepoFactory: func() tool.Repository {
			return integrationTB.ToolRepo
		},
	}

	orch := orchestrator.NewOrchestrator(
		integrationTB.NatsServer,
		integrationTB.NatsClient,
		integrationTB.Store,
		orchConfig,
		&project.Config{},
		[]*workflow.Config{},
	)

	err := orch.Setup(integrationTB.Ctx)
	require.NoError(t, err, "Failed to setup orchestrator")

	// Create app state
	deps := appstate.NewBaseDeps(
		integrationTB.NatsServer,
		integrationTB.NatsClient,
		integrationTB.Store,
		&project.Config{},
		[]*workflow.Config{},
	)

	// Set CWD on project config
	projectConfig := &project.Config{}
	err = projectConfig.SetCWD(integrationTB.StateDir)
	require.NoError(t, err, "Failed to set CWD on project config")
	deps.ProjectConfig = projectConfig

	appState, err := appstate.NewState(deps, orch)
	require.NoError(t, err, "Failed to create app state")

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	ginRouter := gin.New()
	ginRouter.Use(gin.Recovery())
	ginRouter.Use(appstate.StateMiddleware(appState))
	ginRouter.Use(router.ErrorHandler())

	// Register routes
	err = server.RegisterRoutes(ginRouter, appState)
	require.NoError(t, err, "Failed to register routes")

	// Create test server
	testServer := httptest.NewServer(ginRouter)

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: testTimeout,
	}

	return &HTTPTestBed{
		IntegrationTestBed: integrationTB,
		Server:             testServer,
		Router:             ginRouter,
		AppState:           appState,
		BaseURL:            testServer.URL,
		HTTPClient:         httpClient,
	}
}

// Cleanup cleans up the HTTP test bed
func (htb *HTTPTestBed) Cleanup() {
	htb.T.Helper()
	if htb.Server != nil {
		htb.Server.Close()
	}
	htb.IntegrationTestBed.Cleanup()
}

// HTTPResponse represents a standardized HTTP response for testing
type HTTPResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ParseJSON parses the response body as JSON into the provided struct
func (r *HTTPResponse) ParseJSON(v interface{}) error {
	return json.Unmarshal(r.Body, v)
}

// GetAPIResponse parses the response as the standard API response format
func (r *HTTPResponse) GetAPIResponse() (*router.Response, error) {
	var resp router.Response

	// Handle empty body
	if len(r.Body) == 0 {
		return &resp, nil
	}

	// Parse JSON response
	err := json.Unmarshal(r.Body, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w. Body: %s", err, string(r.Body))
	}

	return &resp, nil
}

// HTTPRequest represents an HTTP request for testing
type HTTPRequest struct {
	Method  string
	Path    string
	Body    interface{}
	Headers map[string]string
}

// MakeRequest makes an HTTP request and returns the response
func (htb *HTTPTestBed) MakeRequest(req HTTPRequest) (*HTTPResponse, error) {
	htb.T.Helper()

	var bodyReader *bytes.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	url := htb.BaseURL + req.Path
	var httpReq *http.Request
	var err error

	if bodyReader != nil {
		httpReq, err = http.NewRequestWithContext(htb.Ctx, req.Method, url, bodyReader)
	} else {
		httpReq, err = http.NewRequestWithContext(htb.Ctx, req.Method, url, http.NoBody)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set default headers
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Set custom headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := htb.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes := make([]byte, 0)
	if resp.Body != nil {
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		bodyBytes = buf.Bytes()
	}

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Body:       bodyBytes,
		Headers:    resp.Header,
	}, nil
}

// GET makes a GET request
func (htb *HTTPTestBed) GET(path string, headers ...map[string]string) (*HTTPResponse, error) {
	req := HTTPRequest{
		Method: http.MethodGet,
		Path:   path,
	}
	if len(headers) > 0 {
		req.Headers = headers[0]
	}
	return htb.MakeRequest(req)
}

// POST makes a POST request
func (htb *HTTPTestBed) POST(path string, body interface{}, headers ...map[string]string) (*HTTPResponse, error) {
	req := HTTPRequest{
		Method: http.MethodPost,
		Path:   path,
		Body:   body,
	}
	if len(headers) > 0 {
		req.Headers = headers[0]
	}
	return htb.MakeRequest(req)
}

// PUT makes a PUT request
func (htb *HTTPTestBed) PUT(path string, body interface{}, headers ...map[string]string) (*HTTPResponse, error) {
	req := HTTPRequest{
		Method: http.MethodPut,
		Path:   path,
		Body:   body,
	}
	if len(headers) > 0 {
		req.Headers = headers[0]
	}
	return htb.MakeRequest(req)
}

// DELETE makes a DELETE request
func (htb *HTTPTestBed) DELETE(path string, headers ...map[string]string) (*HTTPResponse, error) {
	req := HTTPRequest{
		Method: http.MethodDelete,
		Path:   path,
	}
	if len(headers) > 0 {
		req.Headers = headers[0]
	}
	return htb.MakeRequest(req)
}

// AssertSuccessResponse asserts that the response is successful and returns the parsed response
func (htb *HTTPTestBed) AssertSuccessResponse(resp *HTTPResponse, expectedStatus int) *router.Response {
	htb.T.Helper()
	require.Equal(htb.T, expectedStatus, resp.StatusCode,
		"Expected status code %d, got %d. Body: %s",
		expectedStatus, resp.StatusCode, string(resp.Body))

	apiResp, err := resp.GetAPIResponse()
	require.NoError(htb.T, err, "Failed to parse API response")
	require.Equal(htb.T, expectedStatus, apiResp.Status, "API response status should match HTTP status")
	require.Nil(htb.T, apiResp.Error, "API response should not have error")

	return apiResp
}

// AssertErrorResponse asserts that the response is an error and returns the parsed response
func (htb *HTTPTestBed) AssertErrorResponse(
	resp *HTTPResponse, expectedStatus int, expectedErrorCode string,
) *router.Response {
	htb.T.Helper()
	require.Equal(htb.T, expectedStatus, resp.StatusCode,
		"Expected status code %d, got %d. Body: %s",
		expectedStatus, resp.StatusCode, string(resp.Body))

	apiResp, err := resp.GetAPIResponse()
	require.NoError(htb.T, err, "Failed to parse API response")
	require.Equal(htb.T, expectedStatus, apiResp.Status, "API response status should match HTTP status")
	require.NotNil(htb.T, apiResp.Error, "API response should have error")
	require.Equal(htb.T, expectedErrorCode, apiResp.Error.Code, "Error code should match expected")

	return apiResp
}
