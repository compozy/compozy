package resources

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectEndpoint(t *testing.T) {
	t.Run("Should upsert and fetch project config", func(t *testing.T) {
		client := newResourceClient(t)
		body := projectPayload("2.0.0", "test project")
		res := client.do(http.MethodPut, "/api/v0/project", body, nil)
		require.Equal(t, http.StatusCreated, res.Code)
		etag := res.Header().Get("ETag")
		require.NotEmpty(t, etag)
		require.Equal(t, "/api/v0/project", res.Header().Get("Location"))
		getRes := client.do(http.MethodGet, "/api/v0/project", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		assert.Equal(t, "test project", data["description"])
		updateBody := projectPayload("2.1.0", "updated")
		updateRes := client.do(http.MethodPut, "/api/v0/project", updateBody, map[string]string{"If-Match": etag})
		require.Equal(t, http.StatusOK, updateRes.Code)
		newEtag := updateRes.Header().Get("ETag")
		require.NotEmpty(t, newEtag)
		assert.NotEqual(t, etag, newEtag)
		afterRes := client.do(http.MethodGet, "/api/v0/project", nil, nil)
		require.Equal(t, http.StatusOK, afterRes.Code)
		assert.Equal(t, newEtag, afterRes.Header().Get("ETag"))
		afterData := decodeData(t, afterRes)
		assert.Equal(t, "2.1.0", afterData["version"])
		staleRes := client.do(http.MethodPut, "/api/v0/project", updateBody, map[string]string{"If-Match": "\"old\""})
		require.Equal(t, http.StatusPreconditionFailed, staleRes.Code)
		nameRes := client.do(http.MethodGet, "/api/v0/project", nil, nil)
		require.Equal(t, http.StatusOK, nameRes.Code)
		dataFiltered := decodeData(t, nameRes)
		assert.Equal(t, client.harness.Project.Name, dataFiltered["name"])
	})
	t.Run("Should refuse project deletion", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodDelete, "/api/v0/project", nil, nil)
		require.Equal(t, http.StatusMethodNotAllowed, res.Code)
		assert.Contains(t, res.Header().Get("Content-Type"), "application/problem+json")
	})
	t.Run("Should reject malformed project payload", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodPut, "/api/v0/project", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Contains(t, res.Header().Get("Content-Type"), "application/problem+json")
	})
}
