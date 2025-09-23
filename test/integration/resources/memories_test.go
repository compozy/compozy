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
		assert.Equal(t, "/api/v0/memories/session", createRes.Header().Get("Location"))
		getRes := client.do(http.MethodGet, "/api/v0/memories/session", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		assert.Equal(t, "memory", data["resource"])
		assert.Equal(t, "buffer", data["type"])
		persist, ok := data["persistence"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "in_memory", persist["type"])
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
		assert.NotEqual(t, etag, newEtag)
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
		assert.Contains(t, staleRes.Header().Get("Content-Type"), "application/problem+json")
		client.do(http.MethodPut, "/api/v0/memories/cache", memoryPayload("cache"), nil)
		ids := collectIDs(t, client, "/api/v0/memories?limit=1", "memories", "id")
		assert.ElementsMatch(t, []string{"cache", "session"}, ids)
		delRes := client.do(http.MethodDelete, "/api/v0/memories/session", nil, nil)
		require.Equal(t, http.StatusNoContent, delRes.Code)
		missingRes := client.do(http.MethodGet, "/api/v0/memories/session", nil, nil)
		require.Equal(t, http.StatusNotFound, missingRes.Code)
		assert.Contains(t, missingRes.Header().Get("Content-Type"), "application/problem+json")
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
		assert.Contains(t, delRes.Header().Get("Content-Type"), "application/problem+json")
		assert.Contains(t, delRes.Body.String(), "agents")
	})
	t.Run("Should reject malformed memory payload", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodPut, "/api/v0/memories/bad", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Contains(t, res.Header().Get("Content-Type"), "application/problem+json")
	})
}

func TestMemoriesQueries(t *testing.T) {
	t.Run("Should list with pagination", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/memories/m1", memoryPayload("m1"), nil)
		client.do(http.MethodPut, "/api/v0/memories/m2", memoryPayload("m2"), nil)
		client.do(http.MethodPut, "/api/v0/memories/m3", memoryPayload("m3"), nil)
		res := client.do(http.MethodGet, "/api/v0/memories?limit=1", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "memories")
		require.Len(t, items, 1)
		_, hasNext := page["next_cursor"]
		assert.True(t, hasNext)
		assert.Equal(t, float64(3), page["total"])
		assert.Contains(t, res.Header().Get("Link"), "rel=\"next\"")
		next := page["next_cursor"].(string)
		res2 := client.do(http.MethodGet, "/api/v0/memories?limit=1&cursor="+next, nil, nil)
		require.Equal(t, http.StatusOK, res2.Code)
		assert.Contains(t, res2.Header().Get("Link"), "rel=\"prev\"")
	})
	t.Run("Should filter by prefix q", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/memories/context-x", memoryPayload("context-x"), nil)
		client.do(http.MethodPut, "/api/v0/memories/cache-y", memoryPayload("cache-y"), nil)
		res := client.do(http.MethodGet, "/api/v0/memories?q=context-", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "memories")
		assert.Equal(t, float64(1), page["total"])
		assert.Equal(t, "context-x", items[0]["id"].(string))
	})
	t.Run("Should reject invalid cursor on list", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/memories/c1", memoryPayload("c1"), nil)
		res := client.do(http.MethodGet, "/api/v0/memories?cursor=abc", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Contains(t, res.Header().Get("Content-Type"), "application/problem+json")
	})
}
