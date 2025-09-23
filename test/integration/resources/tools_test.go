package resources

import (
	"net/http"
	"testing"

	storepkg "github.com/compozy/compozy/engine/resources"
	taskcfg "github.com/compozy/compozy/engine/task"
	toolcfg "github.com/compozy/compozy/engine/tool"
	workflowcfg "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolsEndpoints(t *testing.T) {
	t.Run("Should manage tools and enforce concurrency", func(t *testing.T) {
		client := newResourceClient(t)
		createRes := client.do(
			http.MethodPut,
			"/api/v0/tools/http",
			toolPayload("http", "GET", "https://example.com"),
			nil,
		)
		require.Equal(t, http.StatusCreated, createRes.Code)
		etag := createRes.Header().Get("ETag")
		require.NotEmpty(t, etag)
		require.Equal(t, "/api/v0/tools/http", createRes.Header().Get("Location"))
		getRes := client.do(http.MethodGet, "/api/v0/tools/http", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		assert.Equal(t, "http", data["id"])
		updateBody := toolPayload("http", "POST", "https://example.com")
		updateBody["with"] = map[string]any{"mode": "plain"}
		updateBody["env"] = map[string]any{"LOG_LEVEL": "debug"}
		updateRes := client.do(
			http.MethodPut,
			"/api/v0/tools/http",
			updateBody,
			map[string]string{"If-Match": etag},
		)
		require.Equal(t, http.StatusOK, updateRes.Code)
		newEtag := updateRes.Header().Get("ETag")
		require.NotEmpty(t, newEtag)
		assert.NotEqual(t, etag, newEtag)
		afterRes := client.do(http.MethodGet, "/api/v0/tools/http", nil, nil)
		require.Equal(t, http.StatusOK, afterRes.Code)
		assert.Equal(t, newEtag, afterRes.Header().Get("ETag"))
		afterData := decodeData(t, afterRes)
		cfg, ok := afterData["config"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "POST", cfg["method"])
		with, ok := afterData["with"].(map[string]any)
		if ok {
			assert.Equal(t, "plain", with["mode"])
		}
		env, ok := afterData["env"].(map[string]any)
		if ok {
			assert.Equal(t, "debug", env["LOG_LEVEL"])
		}
		staleRes := client.do(
			http.MethodPut,
			"/api/v0/tools/http",
			toolPayload("http", "POST", "https://example.com"),
			map[string]string{"If-Match": "\"tool-etag\""},
		)
		require.Equal(t, http.StatusPreconditionFailed, staleRes.Code)
		client.do(http.MethodPut, "/api/v0/tools/logger", toolPayload("logger", "GET", "https://example.com/log"), nil)
		ids := collectIDs(t, client, "/api/v0/tools?limit=1", "tools", "id")
		assert.ElementsMatch(t, []string{"http", "logger"}, ids)
		delRes := client.do(http.MethodDelete, "/api/v0/tools/http", nil, nil)
		require.Equal(t, http.StatusNoContent, delRes.Code)
		missingRes := client.do(http.MethodGet, "/api/v0/tools/http", nil, nil)
		require.Equal(t, http.StatusNotFound, missingRes.Code)
	})

	t.Run("Should expose tool CWD from stored configuration", func(t *testing.T) {
		client := newResourceClient(t)
		store := client.store()
		cfg := &toolcfg.Config{Resource: "tool", ID: "with-cwd"}
		require.NoError(t, cfg.SetCWD("."))
		_, err := store.Put(
			client.harness.Ctx,
			storepkg.ResourceKey{Project: client.harness.Project.Name, Type: storepkg.ResourceTool, ID: cfg.ID},
			cfg,
		)
		require.NoError(t, err)
		res := client.do(http.MethodGet, "/api/v0/tools/with-cwd", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		data := decodeData(t, res)
		cwd, ok := data["cwd"].(string)
		require.True(t, ok)
		assert.Equal(t, cfg.GetCWD().PathStr(), cwd)
	})
	t.Run("Should surface conflict when tool is referenced", func(t *testing.T) {
		client := newResourceClient(t)
		body := toolPayload("notify", "POST", "https://notify")
		res := client.do(http.MethodPut, "/api/v0/tools/notify", body, nil)
		require.Equal(t, http.StatusCreated, res.Code)
		store := client.store()
		wf := &workflowcfg.Config{ID: "wf-tools", Tools: []toolcfg.Config{{ID: "notify"}}}
		_, err := store.Put(
			client.harness.Ctx,
			storepkg.ResourceKey{Project: client.harness.Project.Name, Type: storepkg.ResourceWorkflow, ID: wf.ID},
			wf,
		)
		require.NoError(t, err)
		tk := &taskcfg.Config{
			BaseConfig: taskcfg.BaseConfig{
				ID:   "call-tool",
				Type: taskcfg.TaskTypeBasic,
				Tool: &toolcfg.Config{ID: "notify"},
			},
		}
		_, err = store.Put(
			client.harness.Ctx,
			storepkg.ResourceKey{Project: client.harness.Project.Name, Type: storepkg.ResourceTask, ID: tk.ID},
			tk,
		)
		require.NoError(t, err)
		delRes := client.do(http.MethodDelete, "/api/v0/tools/notify", nil, nil)
		require.Equal(t, http.StatusConflict, delRes.Code)
		assert.Contains(t, delRes.Header().Get("Content-Type"), "application/problem+json")
		bodyStr := delRes.Body.String()
		assert.Contains(t, bodyStr, "workflows")
		assert.Contains(t, bodyStr, "tasks")
	})
	t.Run("Should reject malformed tool payload", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodPut, "/api/v0/tools/bad", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Contains(t, res.Header().Get("Content-Type"), "application/problem+json")
	})
}

func TestToolsQueries(t *testing.T) {
	t.Run("Should list with pagination", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/tools/t1", toolPayload("t1", "GET", "https://ex.com/1"), nil)
		client.do(http.MethodPut, "/api/v0/tools/t2", toolPayload("t2", "GET", "https://ex.com/2"), nil)
		client.do(http.MethodPut, "/api/v0/tools/t3", toolPayload("t3", "GET", "https://ex.com/3"), nil)
		res := client.do(http.MethodGet, "/api/v0/tools?limit=1", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "tools")
		require.Len(t, items, 1)
		_, hasNext := page["next_cursor"]
		assert.True(t, hasNext)
		assert.Equal(t, float64(3), page["total"])
		assert.Contains(t, res.Header().Get("Link"), "rel=\"next\"")
		next := page["next_cursor"].(string)
		res2 := client.do(http.MethodGet, "/api/v0/tools?limit=1&cursor="+next, nil, nil)
		require.Equal(t, http.StatusOK, res2.Code)
		assert.Contains(t, res2.Header().Get("Link"), "rel=\"prev\"")
	})
	t.Run("Should filter by prefix q", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/tools/logger-x", toolPayload("logger-x", "GET", "https://ex.com"), nil)
		client.do(http.MethodPut, "/api/v0/tools/http-y", toolPayload("http-y", "GET", "https://ex.com"), nil)
		res := client.do(http.MethodGet, "/api/v0/tools?q=logger-", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "tools")
		assert.Equal(t, float64(1), page["total"])
		assert.Equal(t, "logger-x", items[0]["id"].(string))
	})
	t.Run("Should filter by workflow_id relations", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/tools/router-a", toolPayload("router-a", "GET", "https://ex.com"), nil)
		client.do(http.MethodPut, "/api/v0/tools/reviewer-a", toolPayload("reviewer-a", "GET", "https://ex.com"), nil)
		client.do(http.MethodPut, "/api/v0/tools/misc-a", toolPayload("misc-a", "GET", "https://ex.com"), nil)
		wf := workflowPayload("wf-tools-rel", "rel")
		wf["tools"] = []map[string]any{{"id": "router-a"}, {"id": "reviewer-a"}, {"id": "ghost"}}
		client.do(http.MethodPut, "/api/v0/workflows/wf-tools-rel", wf, nil)
		res := client.do(http.MethodGet, "/api/v0/tools?workflow_id=wf-tools-rel", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "tools")
		assert.Equal(t, float64(2), page["total"])
		ids := make([]string, 0, len(items))
		for i := range items {
			ids = append(ids, items[i]["id"].(string))
		}
		assert.ElementsMatch(t, []string{"router-a", "reviewer-a"}, ids)
		assert.NotContains(t, ids, "ghost")
	})
	t.Run("Should reject invalid cursor on list", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/tools/c1", toolPayload("c1", "GET", "https://e.com"), nil)
		res := client.do(http.MethodGet, "/api/v0/tools?cursor=abc", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Contains(t, res.Header().Get("Content-Type"), "application/problem+json")
	})
}
