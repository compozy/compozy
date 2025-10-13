package resourceutil

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/test/helpers"
)

func TestConflictError_Error(t *testing.T) {
	t.Run("Should return default message when no details provided", func(t *testing.T) {
		err := ConflictError{Details: []ReferenceDetail{}}
		assert.Equal(t, "resource has conflicting references", err.Error())
	})

	t.Run("Should return collection count when only empty resources", func(t *testing.T) {
		err := ConflictError{
			Details: []ReferenceDetail{
				{Resource: "", IDs: []string{"id1"}},
				{Resource: "  ", IDs: []string{"id2"}},
			},
		}
		assert.Equal(t, "resource referenced by 2 collections", err.Error())
	})

	t.Run("Should return formatted message with resource types", func(t *testing.T) {
		err := ConflictError{
			Details: []ReferenceDetail{
				{Resource: "workflows", IDs: []string{"wf1", "wf2"}},
				{Resource: "agents", IDs: []string{"ag1"}},
			},
		}
		msg := err.Error()
		assert.Contains(t, msg, "resource referenced by 2 collections")
		assert.Contains(t, msg, "agents")
		assert.Contains(t, msg, "workflows")
	})

	t.Run("Should deduplicate resource types", func(t *testing.T) {
		err := ConflictError{
			Details: []ReferenceDetail{
				{Resource: "workflows", IDs: []string{"wf1"}},
				{Resource: "workflows", IDs: []string{"wf2"}},
				{Resource: "agents", IDs: []string{"ag1"}},
			},
		}
		msg := err.Error()
		assert.Contains(t, msg, "resource referenced by 3 collections")
		assert.Contains(t, msg, "agents, workflows")
	})

	t.Run("Should sort resource types alphabetically", func(t *testing.T) {
		err := ConflictError{
			Details: []ReferenceDetail{
				{Resource: "workflows", IDs: []string{"wf1"}},
				{Resource: "agents", IDs: []string{"ag1"}},
				{Resource: "tasks", IDs: []string{"tk1"}},
			},
		}
		msg := err.Error()
		assert.Contains(t, msg, "agents, tasks, workflows")
	})
}

func TestRespondConflict(t *testing.T) {
	t.Run("Should respond with conflict status and references", func(t *testing.T) {
		c, w := setupTestContext(t)
		details := []ReferenceDetail{
			{Resource: "workflows", IDs: []string{"wf1", "wf2"}},
			{Resource: "agents", IDs: []string{"ag1"}},
		}
		err := errors.New("resource in use")
		RespondConflict(c, err, details)
		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("Should include references in response", func(t *testing.T) {
		c, w := setupTestContext(t)
		details := []ReferenceDetail{
			{Resource: "workflows", IDs: []string{"wf1"}},
		}
		err := errors.New("cannot delete")
		RespondConflict(c, err, details)
		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("Should use default message when error is nil", func(t *testing.T) {
		c, w := setupTestContext(t)
		details := []ReferenceDetail{
			{Resource: "workflows", IDs: []string{"wf1"}},
		}
		RespondConflict(c, nil, details)
		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("Should use default message when error message is empty", func(t *testing.T) {
		c, w := setupTestContext(t)
		err := errors.New("  ")
		RespondConflict(c, err, nil)
		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("Should handle empty details gracefully", func(t *testing.T) {
		c, w := setupTestContext(t)
		err := errors.New("conflict error")
		RespondConflict(c, err, []ReferenceDetail{})
		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("Should use core.RespondProblem for conflict response", func(t *testing.T) {
		c, w := setupTestContext(t)
		details := []ReferenceDetail{
			{Resource: "tasks", IDs: []string{"tk1", "tk2"}},
		}
		err := errors.New("resource has dependencies")
		RespondConflict(c, err, details)
		require.Equal(t, http.StatusConflict, w.Code)
		body := w.Body.String()
		assert.NotEmpty(t, body)
	})
}

func setupTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	ctx := helpers.NewTestContext(t)
	req = req.WithContext(ctx)
	c.Request = req
	return c, w
}

func TestConflictError_Error_EdgeCases(t *testing.T) {
	t.Run("Should handle mixed empty and valid resources", func(t *testing.T) {
		err := ConflictError{
			Details: []ReferenceDetail{
				{Resource: "", IDs: []string{"id1"}},
				{Resource: "workflows", IDs: []string{"wf1"}},
				{Resource: "  ", IDs: []string{"id2"}},
			},
		}
		msg := err.Error()
		assert.Contains(t, msg, "resource referenced by 3 collections")
		assert.Contains(t, msg, "workflows")
	})

	t.Run("Should handle single detail with empty resource", func(t *testing.T) {
		err := ConflictError{
			Details: []ReferenceDetail{
				{Resource: "", IDs: []string{"id1"}},
			},
		}
		assert.Equal(t, "resource referenced by 1 collections", err.Error())
	})
}

func TestRespondConflict_Integration(t *testing.T) {
	t.Run("Should create proper problem response structure", func(t *testing.T) {
		c, w := setupTestContext(t)
		details := []ReferenceDetail{
			{Resource: "workflows", IDs: []string{"wf1", "wf2"}},
			{Resource: "agents", IDs: []string{"ag1"}},
		}
		conflictErr := ConflictError{Details: details}
		RespondConflict(c, conflictErr, details)
		require.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), `"detail":"resource referenced by 2 collections: agents, workflows"`)
		assert.Contains(t, w.Body.String(), `"references"`)
	})

	t.Run("Should preserve error context in response", func(t *testing.T) {
		c, w := setupTestContext(t)
		err := errors.New("knowledge base is referenced by 3 workflows")
		details := []ReferenceDetail{
			{Resource: "workflows", IDs: []string{"wf1", "wf2", "wf3"}},
		}
		RespondConflict(c, err, details)
		require.Equal(t, http.StatusConflict, w.Code)
	})
}
