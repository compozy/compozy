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
		updateRes := client.do(
			http.MethodPut,
			"/api/v0/tools/http",
			toolPayload("http", "POST", "https://example.com"),
			map[string]string{"If-Match": etag},
		)
		require.Equal(t, http.StatusOK, updateRes.Code)
		newEtag := updateRes.Header().Get("ETag")
		require.NotEmpty(t, newEtag)
		afterRes := client.do(http.MethodGet, "/api/v0/tools/http", nil, nil)
		require.Equal(t, http.StatusOK, afterRes.Code)
		assert.Equal(t, newEtag, afterRes.Header().Get("ETag"))
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
		assert.Equal(t, "application/problem+json", delRes.Header().Get("Content-Type"))
		bodyStr := delRes.Body.String()
		assert.Contains(t, bodyStr, "workflows")
		assert.Contains(t, bodyStr, "tasks")
	})
	t.Run("Should reject malformed tool payload", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodPut, "/api/v0/tools/bad", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/problem+json", res.Header().Get("Content-Type"))
	})
}
