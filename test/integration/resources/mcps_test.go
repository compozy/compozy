package resources

import (
	"net/http"
	"testing"

	agentcfg "github.com/compozy/compozy/engine/agent"
	mcppkg "github.com/compozy/compozy/engine/mcp"
	storepkg "github.com/compozy/compozy/engine/resources"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPSEndpoints(t *testing.T) {
	t.Setenv("MCP_PROXY_URL", "http://localhost:6001")
	t.Run("Should upsert and retrieve mcps", func(t *testing.T) {
		client := newResourceClient(t)
		payload := mcpPayload("filesystem")
		payload["env"] = map[string]any{"MCP_DEBUG": "true"}
		createRes := client.do(
			http.MethodPut,
			"/api/v0/mcps/filesystem",
			payload,
			nil,
		)
		require.Equal(t, http.StatusCreated, createRes.Code)
		etag := createRes.Header().Get("ETag")
		require.NotEmpty(t, etag)
		require.Equal(t, "/api/v0/mcps/filesystem", createRes.Header().Get("Location"))
		getRes := client.do(http.MethodGet, "/api/v0/mcps/filesystem", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		assert.Equal(t, "filesystem", data["id"])
		assert.Equal(t, mcpproxy.TransportSSE.String(), data["transport"])
		env, ok := data["env"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "true", env["MCP_DEBUG"])
		updateBody := cloneMap(mcpPayload("filesystem"))
		updateBody["headers"] = map[string]any{"Authorization": "Bearer token"}
		updateBody["env"] = map[string]any{"MCP_DEBUG": "false"}
		updateRes := client.do(
			http.MethodPut,
			"/api/v0/mcps/filesystem",
			updateBody,
			map[string]string{"If-Match": etag},
		)
		require.Equal(t, http.StatusOK, updateRes.Code)
		newEtag := updateRes.Header().Get("ETag")
		require.NotEmpty(t, newEtag)
		afterRes := client.do(http.MethodGet, "/api/v0/mcps/filesystem", nil, nil)
		require.Equal(t, http.StatusOK, afterRes.Code)
		assert.Equal(t, newEtag, afterRes.Header().Get("ETag"))
		afterData := decodeData(t, afterRes)
		headers, ok := afterData["headers"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Bearer token", headers["Authorization"])
		afterEnv, ok := afterData["env"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "false", afterEnv["MCP_DEBUG"])
		staleRes := client.do(
			http.MethodPut,
			"/api/v0/mcps/filesystem",
			updateBody,
			map[string]string{"If-Match": "\"invalid\""},
		)
		require.Equal(t, http.StatusPreconditionFailed, staleRes.Code)
		client.do(http.MethodPut, "/api/v0/mcps/remote", mcpPayload("remote"), nil)
		ids := collectIDs(t, client, "/api/v0/mcps?limit=1", "mcps", "id")
		assert.ElementsMatch(t, []string{"filesystem", "remote"}, ids)
		delRes := client.do(http.MethodDelete, "/api/v0/mcps/filesystem", nil, nil)
		require.Equal(t, http.StatusNoContent, delRes.Code)
		missingRes := client.do(http.MethodGet, "/api/v0/mcps/filesystem", nil, nil)
		require.Equal(t, http.StatusNotFound, missingRes.Code)
	})
	t.Run("Should prevent deleting mcp in use", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodPut, "/api/v0/mcps/shared", mcpPayload("shared"), nil)
		require.Equal(t, http.StatusCreated, res.Code)
		store := client.store()
		ag := &agentcfg.Config{ID: "planner"}
		ag.MCPs = []mcppkg.Config{{ID: "shared"}}
		_, err := store.Put(
			client.harness.Ctx,
			storepkg.ResourceKey{Project: client.harness.Project.Name, Type: storepkg.ResourceAgent, ID: ag.ID},
			ag,
		)
		require.NoError(t, err)
		delRes := client.do(http.MethodDelete, "/api/v0/mcps/shared", nil, nil)
		require.Equal(t, http.StatusConflict, delRes.Code)
		assert.Equal(t, "application/problem+json", delRes.Header().Get("Content-Type"))
		assert.Contains(t, delRes.Body.String(), "agents")
	})
	t.Run("Should reject malformed mcp payload", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodPut, "/api/v0/mcps/bad", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/problem+json", res.Header().Get("Content-Type"))
	})
}

func TestMCPQueries(t *testing.T) {
	t.Setenv("MCP_PROXY_URL", "http://localhost:6001")
	t.Run("Should list with pagination", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/mcps/a", mcpPayload("a"), nil)
		client.do(http.MethodPut, "/api/v0/mcps/b", mcpPayload("b"), nil)
		client.do(http.MethodPut, "/api/v0/mcps/c", mcpPayload("c"), nil)
		res := client.do(http.MethodGet, "/api/v0/mcps?limit=1", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "mcps")
		require.Len(t, items, 1)
		_, hasNext := page["next_cursor"]
		assert.True(t, hasNext)
		assert.Equal(t, float64(3), page["total"])
		assert.Contains(t, res.Header().Get("Link"), "rel=\"next\"")
		next := page["next_cursor"].(string)
		res2 := client.do(http.MethodGet, "/api/v0/mcps?limit=1&cursor="+next, nil, nil)
		require.Equal(t, http.StatusOK, res2.Code)
		assert.Contains(t, res2.Header().Get("Link"), "rel=\"prev\"")
	})
	t.Run("Should filter by prefix q", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/mcps/filesystem", mcpPayload("filesystem"), nil)
		client.do(http.MethodPut, "/api/v0/mcps/github", mcpPayload("github"), nil)
		res := client.do(http.MethodGet, "/api/v0/mcps?q=git", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "mcps")
		assert.Equal(t, float64(1), page["total"])
		assert.Equal(t, "github", items[0]["id"].(string))
	})
	t.Run("Should reject invalid cursor on list", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/mcps/x", mcpPayload("x"), nil)
		res := client.do(http.MethodGet, "/api/v0/mcps?cursor=abc", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/problem+json", res.Header().Get("Content-Type"))
	})
}
