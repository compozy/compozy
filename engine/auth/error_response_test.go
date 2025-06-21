package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should send error response with details", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		SendErrorResponse(c, http.StatusBadRequest, "Bad Request", "Missing required field")
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Bad Request", response.Error)
		assert.Equal(t, "Missing required field", response.Details)
	})
	t.Run("Should send error response without details", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		SendErrorResponse(c, http.StatusInternalServerError, "Internal Server Error", "")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Internal Server Error", response.Error)
		assert.Empty(t, response.Details)
	})
}

func TestSendSuccessResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should send success response with data", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		testData := map[string]string{"key": "value"}
		SendSuccessResponse(c, http.StatusOK, testData, "Operation successful")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
		var response SuccessResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Operation successful", response.Message)
		// Check data
		dataMap, ok := response.Data.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "value", dataMap["key"])
	})
	t.Run("Should send success response with nil data", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		SendSuccessResponse(c, http.StatusCreated, nil, "Created successfully")
		assert.Equal(t, http.StatusCreated, w.Code)
		var response SuccessResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Created successfully", response.Message)
		assert.Nil(t, response.Data)
	})
}

func TestSendUnauthorizedError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should send 401 unauthorized error and abort", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		SendUnauthorizedError(c, "Invalid API key")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.True(t, c.IsAborted())
		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Unauthorized", response.Error)
		assert.Equal(t, "Invalid API key", response.Details)
	})
}

func TestSendForbiddenError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should send 403 forbidden error and abort", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		SendForbiddenError(c, "Insufficient permissions")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.True(t, c.IsAborted())
		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Forbidden", response.Error)
		assert.Equal(t, "Insufficient permissions", response.Details)
	})
}

func TestSendBadRequestError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should send 400 bad request error and abort", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		SendBadRequestError(c, "Invalid request format")
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.True(t, c.IsAborted())
		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Bad Request", response.Error)
		assert.Equal(t, "Invalid request format", response.Details)
	})
}

func TestSendNotFoundError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should send 404 not found error and abort", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		SendNotFoundError(c, "Resource not found")
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.True(t, c.IsAborted())
		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Not Found", response.Error)
		assert.Equal(t, "Resource not found", response.Details)
	})
}

func TestSendInternalServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should send 500 internal server error and abort", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		SendInternalServerError(c, "Database connection failed")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.True(t, c.IsAborted())
		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Internal Server Error", response.Error)
		assert.Equal(t, "Database connection failed", response.Details)
	})
}

func TestSendRateLimitError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should send 429 rate limit error and abort", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		SendRateLimitError(c, "Too many requests")
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.True(t, c.IsAborted())
		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Rate Limit Exceeded", response.Error)
		assert.Equal(t, "Too many requests", response.Details)
	})
}
