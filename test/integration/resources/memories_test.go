package resources

import (
	"net/http"
	"testing"

	agentcfg "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	storepkg "github.com/compozy/compozy/engine/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoriesEndpoints(t *testing.T) {
	t.Run("Should create and update memory configuration", func(t *testing.T) {
		client := newResourceClient(t)
		createRes := client.do(http.MethodPut, "/api/v0/memories/session", memoryPayload("session"), nil)
		require.Equal(t, http.StatusCreated, createRes.Code)
		etag := createRes.Header().Get("ETag")
		require.NotEmpty(t, etag)
		require.Equal(t, "/api/v0/memories/session", createRes.Header().Get("Location"))
		getRes := client.do(http.MethodGet, "/api/v0/memories/session", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		assert.Equal(t, "memory", data["resource"])
		updateBody := cloneMap(memoryPayload("session"))
		updateBody["max_tokens"] = 256
		updateRes := client.do(
			http.MethodPut,
			"/api/v0/memories/session",
			updateBody,
			map[string]string{"If-Match": etag},
		)
		require.Equal(t, http.StatusOK, updateRes.Code)
		newEtag := updateRes.Header().Get("ETag")
		require.NotEmpty(t, newEtag)
		afterRes := client.do(http.MethodGet, "/api/v0/memories/session", nil, nil)
		require.Equal(t, http.StatusOK, afterRes.Code)
		assert.Equal(t, newEtag, afterRes.Header().Get("ETag"))
		afterData := decodeData(t, afterRes)
		assert.EqualValues(t, 256, afterData["max_tokens"])
		staleRes := client.do(
			http.MethodPut,
			"/api/v0/memories/session",
			updateBody,
			map[string]string{"If-Match": "\"stale\""},
		)
		require.Equal(t, http.StatusPreconditionFailed, staleRes.Code)
		client.do(http.MethodPut, "/api/v0/memories/cache", memoryPayload("cache"), nil)
		ids := collectIDs(t, client, "/api/v0/memories?limit=1", "memories", "id")
		assert.ElementsMatch(t, []string{"cache", "session"}, ids)
		delRes := client.do(http.MethodDelete, "/api/v0/memories/session", nil, nil)
		require.Equal(t, http.StatusNoContent, delRes.Code)
		missingRes := client.do(http.MethodGet, "/api/v0/memories/session", nil, nil)
		require.Equal(t, http.StatusNotFound, missingRes.Code)
	})
	t.Run("Should prevent deleting referenced memory", func(t *testing.T) {
		client := newResourceClient(t)
		body := memoryPayload("context")
		res := client.do(http.MethodPut, "/api/v0/memories/context", body, nil)
		require.Equal(t, http.StatusCreated, res.Code)
		store := client.store()
		ag := &agentcfg.Config{ID: "helper"}
		ag.Memory = []core.MemoryReference{{ID: "context"}}
		_, err := store.Put(
			client.harness.Ctx,
			storepkg.ResourceKey{Project: client.harness.Project.Name, Type: storepkg.ResourceAgent, ID: ag.ID},
			ag,
		)
		require.NoError(t, err)
		delRes := client.do(http.MethodDelete, "/api/v0/memories/context", nil, nil)
		require.Equal(t, http.StatusConflict, delRes.Code)
		assert.Equal(t, "application/problem+json", delRes.Header().Get("Content-Type"))
		assert.Contains(t, delRes.Body.String(), "agents")
	})
	t.Run("Should reject malformed memory payload", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodPut, "/api/v0/memories/bad", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/problem+json", res.Header().Get("Content-Type"))
	})
}
