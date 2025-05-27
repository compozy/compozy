package router

import (
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddleware(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	t.Run("App state middleware - successful injection", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows")
		require.NoError(t, err, "Failed to make GET request")

		// If we get a successful response, it means app state was properly injected
		htb.AssertSuccessResponse(resp, http.StatusOK)
	})

	t.Run("Error handler middleware - handles errors properly", func(t *testing.T) {
		// Test with a non-existent workflow ID to trigger an error
		resp, err := htb.GET(baseURL + "/workflows/non-existent-workflow")
		require.NoError(t, err, "Failed to make GET request")

		// Should return 404 with proper error structure
		htb.AssertErrorResponse(resp, http.StatusNotFound, router.ErrNotFoundCode)
	})

	t.Run("JSON response format", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows")
		require.NoError(t, err, "Failed to make GET request")

		// Verify the response is valid JSON with expected structure
		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, http.StatusOK, apiResp.Status)
		assert.Equal(t, "workflows retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data)
		assert.Nil(t, apiResp.Error)
	})

	t.Run("Unsupported method returns 404", func(t *testing.T) {
		req := utils.HTTPRequest{
			Method: http.MethodPatch,
			Path:   baseURL + "/workflows",
		}

		resp, err := htb.MakeRequest(req)
		require.NoError(t, err, "Failed to make PATCH request")

		// PATCH on /workflows is not a defined route, so it should return 404
		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"PATCH method should return 404 for undefined route")
	})

	t.Run("CORS headers", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows")
		require.NoError(t, err, "Failed to make GET request")

		// Note: CORS headers might not be present in test mode
		// This test mainly verifies the middleware doesn't break the request
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestErrorHandling(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	t.Run("Standard error response format", func(t *testing.T) {
		// Test with a route that should return an error
		resp, err := htb.GET(baseURL + "/workflows/nonexistent-workflow")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertErrorResponse(resp, http.StatusNotFound, router.ErrNotFoundCode)

		// Verify error response structure
		assert.NotNil(t, apiResp.Error, "Error response should have error field")
		assert.Equal(t, router.ErrNotFoundCode, apiResp.Error.Code, "Error code should match")
		assert.NotEmpty(t, apiResp.Error.Message, "Error message should not be empty")
		assert.Equal(t, http.StatusNotFound, apiResp.Status, "Status should match HTTP status")
	})

	t.Run("Bad request error handling", func(t *testing.T) {
		// Test with invalid JSON in POST request
		req := utils.HTTPRequest{
			Method: http.MethodPost,
			Path:   baseURL + "/workflows/test-workflow/executions",
			Body:   "invalid json string",
		}

		resp, err := htb.MakeRequest(req)
		require.NoError(t, err, "Failed to make POST request")

		apiResp := htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
		assert.Contains(t, apiResp.Error.Message, "invalid input", "Error message should indicate invalid input")
	})

	t.Run("Missing parameter error handling", func(t *testing.T) {
		// Test with empty workflow_id parameter
		resp, err := htb.GET(baseURL + "/workflows//executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
		assert.Contains(t, apiResp.Error.Message, "workflow_id is required",
			"Error message should indicate missing workflow_id")
	})
}

func TestAPIVersioning(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()

	t.Run("Correct API version in URL", func(t *testing.T) {
		resp, err := htb.GET("/api/" + version + "/workflows")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertSuccessResponse(resp, http.StatusOK)
	})

	t.Run("Invalid API version returns 404", func(t *testing.T) {
		resp, err := htb.GET("/api/invalid-version/workflows")
		require.NoError(t, err, "Failed to make GET request")

		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"Should return 404 for invalid API version")
	})

	t.Run("Missing API version returns 404", func(t *testing.T) {
		resp, err := htb.GET("/workflows")
		require.NoError(t, err, "Failed to make GET request")

		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"Should return 404 for missing API version")
	})
}

func TestHTTPMethods(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	t.Run("GET method support", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertSuccessResponse(resp, http.StatusOK)
	})

	t.Run("POST method support", func(t *testing.T) {
		input := map[string]interface{}{
			"test_param": "test_value",
		}

		resp, err := htb.POST(baseURL+"/workflows/test-workflow/executions", input)
		require.NoError(t, err, "Failed to make POST request")

		// Should either accept or return an error, but not method not allowed
		assert.NotEqual(t, http.StatusMethodNotAllowed, resp.StatusCode,
			"POST method should be supported")
	})

	t.Run("Unsupported method returns 404", func(t *testing.T) {
		req := utils.HTTPRequest{
			Method: http.MethodPatch,
			Path:   baseURL + "/workflows",
		}

		resp, err := htb.MakeRequest(req)
		require.NoError(t, err, "Failed to make PATCH request")

		// PATCH on /workflows is not a defined route, so it should return 404
		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"PATCH method should return 404 for undefined route")
	})
}

func TestResponseHeaders(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	t.Run("Content-Type header for JSON responses", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows")
		require.NoError(t, err, "Failed to make GET request")

		contentType := resp.Headers.Get("Content-Type")
		assert.Contains(t, contentType, "application/json",
			"Response should have JSON content type")
	})

	t.Run("Response headers are properly set", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows")
		require.NoError(t, err, "Failed to make GET request")

		// Verify that we get some standard headers
		assert.NotEmpty(t, resp.Headers.Get("Content-Type"), "Content-Type header should be set")
		assert.NotEmpty(t, resp.Headers.Get("Content-Length"), "Content-Length header should be set")
	})
}
