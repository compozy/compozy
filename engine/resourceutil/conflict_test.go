package resourceutil

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/test/helpers"
)

func TestConflictErrorDefaultMessage(t *testing.T) {
	t.Run("Should return default message when details empty", func(t *testing.T) {
		err := ConflictError{Details: []ReferenceDetail{}}
		assert.Equal(t, "resource has conflicting references", err.Error())
	})
}

func TestConflictErrorReferenceCounting(t *testing.T) {
	cases := []struct {
		name    string
		details []ReferenceDetail
		expect  string
	}{
		{
			name:    "Should use singular when only one reference",
			details: []ReferenceDetail{{Resource: "agents", IDs: []string{"ag1"}}},
			expect:  "resource referenced by 1 collection: agents",
		},
		{
			name: "Should count total references across resources",
			details: []ReferenceDetail{
				{Resource: "workflows", IDs: []string{"wf1", "wf2"}},
				{Resource: "agents", IDs: []string{"ag1"}},
			},
			expect: "resource referenced by 3 collections",
		},
		{
			name:    "Should handle blank resources using total references",
			details: []ReferenceDetail{{Resource: " ", IDs: []string{"id1", "id2"}}},
			expect:  "resource referenced by 2 collections",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := ConflictError{Details: tc.details}.Error()
			assert.Contains(t, msg, tc.expect)
		})
	}
}

func TestConflictErrorFormatsResourceTypes(t *testing.T) {
	t.Run("Should deduplicate and sort resource types", func(t *testing.T) {
		err := ConflictError{
			Details: []ReferenceDetail{
				{Resource: "workflows", IDs: []string{"wf1"}},
				{Resource: "agents", IDs: []string{"ag1"}},
				{Resource: "workflows", IDs: []string{"wf2"}},
			},
		}
		msg := err.Error()
		assert.Contains(t, msg, "resource referenced by 3 collections")
		assert.Contains(t, msg, "agents, workflows")
	})
}

func TestRespondConflictIncludesReferences(t *testing.T) {
	t.Run("Should serialize detail and references", func(t *testing.T) {
		c, w := setupTestContext(t)
		details := []ReferenceDetail{
			{Resource: "workflows", IDs: []string{"wf1", "wf2"}},
			{Resource: "agents", IDs: []string{"ag1"}},
		}
		conflictErr := ConflictError{Details: details}
		RespondConflict(c, conflictErr, details)
		require.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

		response := decodeProblemResponse(t, w.Body.Bytes())
		detail, ok := response["detail"].(string)
		require.True(t, ok)
		assert.Equal(t, conflictErr.Error(), detail)

		references, ok := response["references"].([]any)
		require.True(t, ok)
		require.Len(t, references, 2)
	})
}

func TestRespondConflictDefaultDetail(t *testing.T) {
	t.Run("Should fall back to default message when error empty", func(t *testing.T) {
		c, w := setupTestContext(t)
		RespondConflict(c, errors.New("   "), nil)
		response := decodeProblemResponse(t, w.Body.Bytes())
		assert.Equal(t, "resource has active references", response["detail"])
	})
}

func TestRespondConflictPreservesErrorMessage(t *testing.T) {
	t.Run("Should propagate provided error detail", func(t *testing.T) {
		c, w := setupTestContext(t)
		err := errors.New("resource in use by workflows")
		RespondConflict(c, err, []ReferenceDetail{})
		response := decodeProblemResponse(t, w.Body.Bytes())
		assert.Equal(t, "resource in use by workflows", response["detail"])
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

func decodeProblemResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	return payload
}
