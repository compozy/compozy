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
		assert.Equal(t, "application/problem+json", staleRes.Header().Get("Content-Type"))
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
		assert.Equal(t, "application/problem+json", delRes.Header().Get("Content-Type"))
		bodyStr := delRes.Body.String()
		assert.Contains(t, bodyStr, "workflows")
		assert.Contains(t, bodyStr, "tasks")
	})
	t.Run("Should reject malformed agent payload", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodPut, "/api/v0/agents/bad", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/problem+json", res.Header().Get("Content-Type"))
	})
}
