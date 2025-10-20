package resources

import (
	"net/http"
	"testing"

	storepkg "github.com/compozy/compozy/engine/resources"
	taskcfg "github.com/compozy/compozy/engine/task"
	workflowcfg "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTasksEndpoints(t *testing.T) {
	t.Run("Should manage tasks with ETag and pagination", func(t *testing.T) {
		client := newResourceClient(t)
		createBody := taskPayload("approve", "approve request")
		createBody["sleep"] = "1s"
		createBody["on_success"] = map[string]any{"next": "review"}
		createBody["on_error"] = map[string]any{"next": "fallback"}
		createRes := client.do(http.MethodPut, "/api/v0/tasks/approve", createBody, nil)
		require.Equal(t, http.StatusCreated, createRes.Code)
		etag := createRes.Header().Get("ETag")
		require.NotEmpty(t, etag)
		require.Equal(t, "/api/v0/tasks/approve", createRes.Header().Get("Location"))
		getRes := client.do(http.MethodGet, "/api/v0/tasks/approve", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		assert.Equal(t, "approve", data["id"])
		success, ok := data["on_success"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "review", success["next"])
		errorTransition, ok := data["on_error"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "fallback", errorTransition["next"])
		updateBody := taskPayload("approve", "approve immediately")
		updateBody["sleep"] = "2s"
		updateRes := client.do(
			http.MethodPut,
			"/api/v0/tasks/approve",
			updateBody,
			map[string]string{"If-Match": etag},
		)
		require.Equal(t, http.StatusOK, updateRes.Code)
		newEtag := updateRes.Header().Get("ETag")
		require.NotEmpty(t, newEtag)
		assert.NotEqual(t, etag, newEtag)
		afterRes := client.do(http.MethodGet, "/api/v0/tasks/approve", nil, nil)
		require.Equal(t, http.StatusOK, afterRes.Code)
		assert.Equal(t, newEtag, afterRes.Header().Get("ETag"))
		staleRes := client.do(
			http.MethodPut,
			"/api/v0/tasks/approve",
			taskPayload("approve", "approve immediately"),
			map[string]string{"If-Match": "\"outdated\""},
		)
		require.Equal(t, http.StatusPreconditionFailed, staleRes.Code)
		client.do(http.MethodPut, "/api/v0/tasks/review", taskPayload("review", "review"), nil)
		ids := collectIDs(t, client, "/api/v0/tasks?limit=1", "tasks", "id")
		assert.ElementsMatch(t, []string{"approve", "review"}, ids)
		delRes := client.do(http.MethodDelete, "/api/v0/tasks/approve", nil, nil)
		require.Equal(t, http.StatusNoContent, delRes.Code)
		missingRes := client.do(http.MethodGet, "/api/v0/tasks/approve", nil, nil)
		require.Equal(t, http.StatusNotFound, missingRes.Code)
	})
	t.Run("Should expand subtasks on get", func(t *testing.T) {
		client := newResourceClient(t)
		composite := map[string]any{
			"id":    "comp-parent",
			"type":  "composite",
			"tasks": []map[string]any{{"id": "comp-child-a", "type": "basic"}, {"id": "comp-child-b", "type": "basic"}},
		}
		res := client.do(http.MethodPut, "/api/v0/tasks/comp-parent", composite, nil)
		require.Equal(t, http.StatusCreated, res.Code, res.Body.String())
		getRes := client.do(http.MethodGet, "/api/v0/tasks/comp-parent?expand=subtasks", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		assert.Equal(t, "comp-parent", data["id"])
		hasSubtasks, ok := data["has_subtasks"].(bool)
		require.True(t, ok)
		assert.True(t, hasSubtasks)
		ids, ok := data["subtask_ids"].([]any)
		require.True(t, ok)
		assert.ElementsMatch(t, []any{"comp-child-a", "comp-child-b"}, ids)
		subtasks, ok := data["tasks"].([]any)
		require.True(t, ok)
		require.Len(t, subtasks, 2)
		child0, ok := subtasks[0].(map[string]any)
		require.True(t, ok)
		assert.Contains(t, []any{"comp-child-a", "comp-child-b"}, child0["id"])
		collection := map[string]any{
			"id":    "collect-parent",
			"type":  "collection",
			"items": "{{ .input.items }}",
			"task":  map[string]any{"id": "collect-child", "type": "basic"},
		}
		collRes := client.do(http.MethodPut, "/api/v0/tasks/collect-parent", collection, nil)
		require.Equal(t, http.StatusCreated, collRes.Code, collRes.Body.String())
		collGet := client.do(http.MethodGet, "/api/v0/tasks/collect-parent?expand=subtasks", nil, nil)
		require.Equal(t, http.StatusOK, collGet.Code)
		collData := decodeData(t, collGet)
		collHas, ok := collData["has_subtasks"].(bool)
		require.True(t, ok)
		assert.True(t, collHas)
		template, ok := collData["task"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "collect-child", template["id"])
	})
	t.Run("Should return conflict when workflow references task", func(t *testing.T) {
		client := newResourceClient(t)
		body := taskPayload("route", "route")
		res := client.do(http.MethodPut, "/api/v0/tasks/route", body, nil)
		require.Equal(t, http.StatusCreated, res.Code)
		store := client.store()
		wf := &workflowcfg.Config{
			ID:    "wf-tasks",
			Tasks: []taskcfg.Config{{BaseConfig: taskcfg.BaseConfig{ID: "route"}}},
		}
		_, err := store.Put(
			client.harness.Ctx,
			storepkg.ResourceKey{Project: client.harness.Project.Name, Type: storepkg.ResourceWorkflow, ID: wf.ID},
			wf,
		)
		require.NoError(t, err)
		delRes := client.do(http.MethodDelete, "/api/v0/tasks/route", nil, nil)
		require.Equal(t, http.StatusConflict, delRes.Code)
		assert.Equal(t, "application/json", delRes.Header().Get("Content-Type"))
		assert.Contains(t, delRes.Body.String(), "workflows")
	})
	t.Run("Should reject malformed task payload", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodPut, "/api/v0/tasks/oops", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/json", res.Header().Get("Content-Type"))
	})
}

func TestTasksQueries(t *testing.T) {
	t.Run("Should list with pagination", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/tasks/a1", taskPayload("a1", "a1"), nil)
		client.do(http.MethodPut, "/api/v0/tasks/a2", taskPayload("a2", "a2"), nil)
		client.do(http.MethodPut, "/api/v0/tasks/a3", taskPayload("a3", "a3"), nil)
		res := client.do(http.MethodGet, "/api/v0/tasks?limit=1", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "tasks")
		require.Len(t, items, 1)
		_, hasNext := page["next_cursor"]
		assert.True(t, hasNext)
		assert.Equal(t, float64(3), page["total"])
		_, hasID := items[0]["id"]
		_, hasType := items[0]["type"]
		_, hasETag := items[0]["etag"]
		assert.True(t, hasID)
		assert.True(t, hasType)
		assert.True(t, hasETag)
		assert.Contains(t, res.Header().Get("Link"), "rel=\"next\"")
		next := page["next_cursor"].(string)
		res2 := client.do(http.MethodGet, "/api/v0/tasks?limit=1&cursor="+next, nil, nil)
		require.Equal(t, http.StatusOK, res2.Code)
		_, page2 := decodeList(t, res2, "tasks")
		assert.Contains(t, res2.Header().Get("Link"), "rel=\"prev\"")
		assert.Equal(t, float64(3), page2["total"])
	})
	t.Run("Should filter by prefix q", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/tasks/review-x", taskPayload("review-x", "rev"), nil)
		client.do(http.MethodPut, "/api/v0/tasks/approve-y", taskPayload("approve-y", "appr"), nil)
		res := client.do(http.MethodGet, "/api/v0/tasks?q=review-", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "tasks")
		assert.Equal(t, float64(1), page["total"])
		assert.Equal(t, "review-x", items[0]["id"].(string))
	})
	t.Run("Should filter by workflow_id relations", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/tasks/route-a", taskPayload("route-a", "r"), nil)
		client.do(http.MethodPut, "/api/v0/tasks/review-a", taskPayload("review-a", "r"), nil)
		client.do(http.MethodPut, "/api/v0/tasks/misc-a", taskPayload("misc-a", "m"), nil)
		wf := workflowPayload("wf-rel", "rel")
		wf["tasks"] = []map[string]any{
			{"id": "route-a", "type": "basic"},
			{"id": "review-a", "type": "basic"},
			{"id": "ghost", "type": "basic"},
		}
		client.do(http.MethodPut, "/api/v0/workflows/wf-rel", wf, nil)
		res := client.do(http.MethodGet, "/api/v0/tasks?workflow_id=wf-rel", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "tasks")
		assert.Equal(t, float64(2), page["total"]) // ghost not present in store
		ids := make([]string, 0, len(items))
		for i := range items {
			ids = append(ids, items[i]["id"].(string))
		}
		assert.ElementsMatch(t, []string{"route-a", "review-a"}, ids)
		assert.NotContains(t, ids, "ghost")
	})
	t.Run("Should reject invalid cursor on list", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/tasks/c1", taskPayload("c1", "c"), nil)
		res := client.do(http.MethodGet, "/api/v0/tasks?cursor=abc", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/json", res.Header().Get("Content-Type"))
	})

	t.Run("Should expand subtasks in list when requested", func(t *testing.T) {
		client := newResourceClient(t)
		parent := map[string]any{
			"id":    "list-parent",
			"type":  "composite",
			"tasks": []map[string]any{{"id": "list-child-a", "type": "basic"}, {"id": "list-child-b", "type": "basic"}},
		}
		client.do(http.MethodPut, "/api/v0/tasks/list-parent", parent, nil)
		res := client.do(http.MethodGet, "/api/v0/tasks?q=list-parent&expand=subtasks", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, _ := decodeList(t, res, "tasks")
		require.Len(t, items, 1)
		entry := items[0]
		subtasks, ok := entry["tasks"].([]any)
		require.True(t, ok)
		require.Len(t, subtasks, 2)
		ids, ok := entry["subtask_ids"].([]any)
		require.True(t, ok)
		assert.ElementsMatch(t, []any{"list-child-a", "list-child-b"}, ids)
	})

	t.Run("Should expose transitions in list payload", func(t *testing.T) {
		client := newResourceClient(t)
		body := taskPayload("transition-task", "transition")
		body["on_success"] = map[string]any{"next": "final-task"}
		client.do(http.MethodPut, "/api/v0/tasks/transition-task", body, nil)
		res := client.do(http.MethodGet, "/api/v0/tasks?q=transition-task", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, _ := decodeList(t, res, "tasks")
		require.Len(t, items, 1)
		success, ok := items[0]["on_success"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "final-task", success["next"])
	})

	t.Run("Should ignore unknown expand keys", func(t *testing.T) {
		client := newResourceClient(t)
		payload := map[string]any{
			"id":    "expand-mixed",
			"type":  "composite",
			"tasks": []map[string]any{{"id": "mixed-child", "type": "basic"}},
		}
		res := client.do(http.MethodPut, "/api/v0/tasks/expand-mixed", payload, nil)
		require.Equal(t, http.StatusCreated, res.Code, res.Body.String())
		getRes := client.do(http.MethodGet, "/api/v0/tasks/expand-mixed?expand=subtasks,unknown", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		subtasks, ok := data["tasks"].([]any)
		require.True(t, ok)
		require.Len(t, subtasks, 1)
		child, ok := subtasks[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "mixed-child", child["id"])
	})

	// fields= feature removed; no filtering behavior to test
}
