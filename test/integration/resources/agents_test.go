package resources

import (
	"net/http"
	"testing"

	agentcfg "github.com/compozy/compozy/engine/agent"
	storepkg "github.com/compozy/compozy/engine/resources"
	taskcfg "github.com/compozy/compozy/engine/task"
	workflowcfg "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentsEndpoints(t *testing.T) {
	t.Run("Should manage agents with ETag concurrency", func(t *testing.T) {
		client := newResourceClient(t)
		createRes := client.do(http.MethodPut, "/api/v0/agents/support", agentPayload("support", "assist"), nil)
		require.Equal(t, http.StatusCreated, createRes.Code)
		etag := createRes.Header().Get("ETag")
		require.NotEmpty(t, etag)
		require.Equal(t, "/api/v0/agents/support", createRes.Header().Get("Location"))
		getRes := client.do(http.MethodGet, "/api/v0/agents/support", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		assert.Equal(t, "support", data["id"])
		updateRes := client.do(
			http.MethodPut,
			"/api/v0/agents/support",
			agentPayload("support", "assist users"),
			map[string]string{"If-Match": etag},
		)
		require.Equal(t, http.StatusOK, updateRes.Code)
		newEtag := updateRes.Header().Get("ETag")
		require.NotEmpty(t, newEtag)
		assert.NotEqual(t, etag, newEtag)
		afterRes := client.do(http.MethodGet, "/api/v0/agents/support", nil, nil)
		require.Equal(t, http.StatusOK, afterRes.Code)
		assert.Equal(t, newEtag, afterRes.Header().Get("ETag"))
		staleRes := client.do(
			http.MethodPut,
			"/api/v0/agents/support",
			agentPayload("support", "assist users"),
			map[string]string{"If-Match": "\"bogus\""},
		)
		require.Equal(t, http.StatusPreconditionFailed, staleRes.Code)
		assert.Contains(t, staleRes.Header().Get("Content-Type"), "application/json")
		client.do(http.MethodPut, "/api/v0/agents/reserve", agentPayload("reserve", "assist"), nil)
		ids := collectIDs(t, client, "/api/v0/agents?limit=1", "agents", "id")
		assert.ElementsMatch(t, []string{"reserve", "support"}, ids)
		filterRes := client.do(http.MethodGet, "/api/v0/agents", nil, nil)
		require.Equal(t, http.StatusOK, filterRes.Code)
		filteredItems, page := decodeList(t, filterRes, "agents")
		assert.EqualValues(t, len(filteredItems), page["total"])
		for i := range filteredItems {
			assert.Contains(t, filteredItems[i], "id")
			assert.Contains(t, filteredItems[i], "etag")
		}
		delRes := client.do(http.MethodDelete, "/api/v0/agents/support", nil, nil)
		require.Equal(t, http.StatusNoContent, delRes.Code)
		missingRes := client.do(http.MethodGet, "/api/v0/agents/support", nil, nil)
		require.Equal(t, http.StatusNotFound, missingRes.Code)
	})
	t.Run("Should block deleting referenced agent with conflict details", func(t *testing.T) {
		client := newResourceClient(t)
		body := agentPayload("planner", "plan")
		res := client.do(http.MethodPut, "/api/v0/agents/planner", body, nil)
		require.Equal(t, http.StatusCreated, res.Code)
		store := client.store()
		wf := &workflowcfg.Config{
			ID: "wf1",
			Agents: []agentcfg.Config{
				{ID: "planner", Instructions: "plan", Model: agentcfg.Model{Ref: "openai:gpt-4o-mini"}},
			},
		}
		_, err := store.Put(
			client.harness.Ctx,
			storepkg.ResourceKey{Project: client.harness.Project.Name, Type: storepkg.ResourceWorkflow, ID: wf.ID},
			wf,
		)
		require.NoError(t, err)
		tk := &taskcfg.Config{
			BaseConfig: taskcfg.BaseConfig{
				ID:    "task1",
				Type:  taskcfg.TaskTypeBasic,
				Agent: &agentcfg.Config{ID: "planner"},
			},
		}
		_, err = store.Put(
			client.harness.Ctx,
			storepkg.ResourceKey{Project: client.harness.Project.Name, Type: storepkg.ResourceTask, ID: tk.ID},
			tk,
		)
		require.NoError(t, err)
		delRes := client.do(http.MethodDelete, "/api/v0/agents/planner", nil, nil)
		require.Equal(t, http.StatusConflict, delRes.Code)
		assert.Contains(t, delRes.Header().Get("Content-Type"), "application/json")
		bodyStr := delRes.Body.String()
		assert.Contains(t, bodyStr, "workflows")
		assert.Contains(t, bodyStr, "tasks")
	})
	t.Run("Should reject malformed agent payload", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodPut, "/api/v0/agents/bad", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Contains(t, res.Header().Get("Content-Type"), "application/json")
	})
}

func TestAgentsQueries(t *testing.T) {
	t.Run("Should list with pagination", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/agents/a1", agentPayload("a1", "a1"), nil)
		client.do(http.MethodPut, "/api/v0/agents/a2", agentPayload("a2", "a2"), nil)
		client.do(http.MethodPut, "/api/v0/agents/a3", agentPayload("a3", "a3"), nil)
		res := client.do(http.MethodGet, "/api/v0/agents?limit=1", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "agents")
		require.Len(t, items, 1)
		_, hasNext := page["next_cursor"]
		assert.True(t, hasNext)
		assert.Equal(t, float64(3), page["total"])
		assert.Contains(t, res.Header().Get("Link"), "rel=\"next\"")
		next := page["next_cursor"].(string)
		res2 := client.do(http.MethodGet, "/api/v0/agents?limit=1&cursor="+next, nil, nil)
		require.Equal(t, http.StatusOK, res2.Code)
		assert.Contains(t, res2.Header().Get("Link"), "rel=\"prev\"")
	})
	t.Run("Should filter by prefix q", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/agents/planner-x", agentPayload("planner-x", "p"), nil)
		client.do(http.MethodPut, "/api/v0/agents/helper-y", agentPayload("helper-y", "h"), nil)
		res := client.do(http.MethodGet, "/api/v0/agents?q=planner-", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "agents")
		require.Len(t, items, 1)
		assert.Equal(t, float64(1), page["total"])
		assert.Equal(t, "planner-x", items[0]["id"].(string))
	})
	t.Run("Should filter by workflow_id relations", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/agents/router-a", agentPayload("router-a", "r"), nil)
		client.do(http.MethodPut, "/api/v0/agents/reviewer-a", agentPayload("reviewer-a", "r"), nil)
		client.do(http.MethodPut, "/api/v0/agents/misc-a", agentPayload("misc-a", "m"), nil)
		wf := workflowPayload("wf-agents-rel", "rel")
		wf["agents"] = []map[string]any{{"id": "router-a"}, {"id": "reviewer-a"}, {"id": "ghost"}}
		client.do(http.MethodPut, "/api/v0/workflows/wf-agents-rel", wf, nil)
		res := client.do(http.MethodGet, "/api/v0/agents?workflow_id=wf-agents-rel", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "agents")
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
		client.do(http.MethodPut, "/api/v0/agents/c1", agentPayload("c1", "c"), nil)
		res := client.do(http.MethodGet, "/api/v0/agents?cursor=abc", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Contains(t, res.Header().Get("Content-Type"), "application/json")
	})
}
