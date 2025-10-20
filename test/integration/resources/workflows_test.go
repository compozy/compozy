package resources

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowsEndpoints(t *testing.T) {
	t.Run("Should perform workflow lifecycle with concurrency", func(t *testing.T) {
		client := newResourceClient(t)
		createRes := client.do(
			http.MethodPut,
			"/api/v0/workflows/wf-int",
			workflowPayload("wf-int", "integration"),
			nil,
		)
		require.Equal(t, http.StatusCreated, createRes.Code)
		etag := createRes.Header().Get("ETag")
		require.NotEmpty(t, etag)
		require.Equal(t, "/api/v0/workflows/wf-int", createRes.Header().Get("Location"))
		getRes := client.do(http.MethodGet, "/api/v0/workflows/wf-int", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		assert.Equal(t, "integration", data["description"])
		updateRes := client.do(
			http.MethodPut,
			"/api/v0/workflows/wf-int",
			workflowPayload("wf-int", "integration updated"),
			map[string]string{"If-Match": etag},
		)
		require.Equal(t, http.StatusOK, updateRes.Code)
		newEtag := updateRes.Header().Get("ETag")
		require.NotEmpty(t, newEtag)
		afterRes := client.do(http.MethodGet, "/api/v0/workflows/wf-int", nil, nil)
		require.Equal(t, http.StatusOK, afterRes.Code)
		assert.Equal(t, newEtag, afterRes.Header().Get("ETag"))
		afterData := decodeData(t, afterRes)
		assert.Equal(t, "integration updated", afterData["description"])
		staleRes := client.do(
			http.MethodPut,
			"/api/v0/workflows/wf-int",
			workflowPayload("wf-int", "integration updated"),
			map[string]string{"If-Match": "\"workflow-stale\""},
		)
		require.Equal(t, http.StatusPreconditionFailed, staleRes.Code)
		client.do(http.MethodPut, "/api/v0/workflows/wf-alt", workflowPayload("wf-alt", "secondary"), nil)
		ids := collectIDs(t, client, "/api/v0/workflows?limit=1", "workflows", "id")
		assert.ElementsMatch(t, []string{"wf-alt", "wf-int"}, ids)
		delRes := client.do(http.MethodDelete, "/api/v0/workflows/wf-int", nil, nil)
		require.Equal(t, http.StatusNoContent, delRes.Code)
		missingRes := client.do(http.MethodGet, "/api/v0/workflows/wf-int", nil, nil)
		require.Equal(t, http.StatusNotFound, missingRes.Code)
	})

	t.Run("Should expose workflow metadata fields", func(t *testing.T) {
		client := newResourceClient(t)
		payload := workflowPayload("wf-meta", "metadata")
		payload["resource"] = "compozy:workflow:wf-meta"
		payload["schemas"] = []map[string]any{{"id": "wf-schema", "type": "object"}}
		payload["outputs"] = map[string]any{"result": "{{ .tasks.step.output.value }}"}
		payload["mcps"] = []map[string]any{{"id": "linker", "transport": "sse", "url": "http://localhost:6001/linker"}}
		wfTasks := []map[string]any{{"id": "step", "type": "basic", "on_success": map[string]any{"next": "done"}}}
		payload["tasks"] = wfTasks
		res := client.do(http.MethodPut, "/api/v0/workflows/wf-meta", payload, nil)
		require.Equal(t, http.StatusCreated, res.Code)
		getRes := client.do(http.MethodGet, "/api/v0/workflows/wf-meta?expand=tasks", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		assert.Equal(t, "compozy:workflow:wf-meta", data["resource"])
		schemas, ok := data["schemas"].([]any)
		require.True(t, ok)
		require.Len(t, schemas, 1)
		outputs, ok := data["outputs"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "{{ .tasks.step.output.value }}", outputs["result"])
		mcps, ok := data["mcps"].([]any)
		require.True(t, ok)
		require.Len(t, mcps, 1)
		tasks, ok := data["tasks"].([]any)
		require.True(t, ok)
		require.Len(t, tasks, 1)
		taskEntry, ok := tasks[0].(map[string]any)
		require.True(t, ok)
		success, ok := taskEntry["on_success"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "done", success["next"])
	})
	t.Run("Should reject malformed workflow payload", func(t *testing.T) {
		client := newResourceClient(t)
		invalid := workflowPayload("other", "mismatch")
		res := client.do(http.MethodPut, "/api/v0/workflows/bad", invalid, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/json", res.Header().Get("Content-Type"))
	})
}

func TestWorkflowsQueries(t *testing.T) {
	t.Run("Should support list pagination and filter", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/workflows/wf-a", workflowPayload("wf-a", "alpha"), nil)
		client.do(http.MethodPut, "/api/v0/workflows/wf-b", workflowPayload("wf-b", "beta"), nil)
		client.do(http.MethodPut, "/api/v0/workflows/other", workflowPayload("other", "other"), nil)
		res := client.do(http.MethodGet, "/api/v0/workflows?q=wf-", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "workflows")
		require.Equal(t, float64(2), page["total"]) // only wf-a,wf-b
		ids := make([]string, 0, len(items))
		for i := range items {
			ids = append(ids, items[i]["id"].(string))
		}
		assert.ElementsMatch(t, []string{"wf-a", "wf-b"}, ids)
		for i := range items {
			_, hasEtag := items[i]["etag"]
			_, hasID := items[i]["id"]
			_, hasTaskCount := items[i]["task_count"]
			assert.True(t, hasEtag)
			assert.True(t, hasID)
			assert.True(t, hasTaskCount)
			assert.Equal(t, 0, int(items[i]["task_count"].(float64)))
		}
		pageLink := res.Header().Get("Link")
		assert.NotContains(t, pageLink, "rel=\"prev\"")
		assert.NotContains(t, pageLink, "rel=\"next\"")
		paged := client.do(http.MethodGet, "/api/v0/workflows?limit=1", nil, nil)
		require.Equal(t, http.StatusOK, paged.Code)
		_, page2 := decodeList(t, paged, "workflows")
		_, hasNext := page2["next_cursor"]
		assert.True(t, hasNext)
		assert.Equal(t, float64(3), page2["total"]) // wf-a, wf-b, other
		link := paged.Header().Get("Link")
		assert.Contains(t, link, "rel=\"next\"")
		// round-trip: use next_cursor to navigate and observe prev link
		next := page2["next_cursor"].(string)
		page2Res := client.do(http.MethodGet, "/api/v0/workflows?limit=1&cursor="+next, nil, nil)
		require.Equal(t, http.StatusOK, page2Res.Code)
		_, page3 := decodeList(t, page2Res, "workflows")
		assert.Contains(t, page2Res.Header().Get("Link"), "rel=\"prev\"")
		assert.Equal(t, float64(3), page3["total"])
	})
	t.Run("Should expand tasks on get", func(t *testing.T) {
		client := newResourceClient(t)
		payload := workflowPayload("wf-exp", "expand test")
		payload["tasks"] = []map[string]any{{"id": "t1", "type": "basic"}, {"id": "t2", "type": "basic"}}
		putRes := client.do(http.MethodPut, "/api/v0/workflows/wf-exp", payload, nil)
		require.Equal(t, http.StatusCreated, putRes.Code)
		getCompact := client.do(http.MethodGet, "/api/v0/workflows/wf-exp", nil, nil)
		require.Equal(t, http.StatusOK, getCompact.Code)
		data := decodeData(t, getCompact)
		assert.Equal(t, "wf-exp", data["id"])
		assert.ElementsMatch(t, []any{"t1", "t2"}, data["tasks"].([]any))
		assert.ElementsMatch(t, []any{"t1", "t2"}, data["task_ids"].([]any))
		assert.Equal(t, float64(2), data["task_count"])
		getExpanded := client.do(http.MethodGet, "/api/v0/workflows/wf-exp?expand=tasks", nil, nil)
		require.Equal(t, http.StatusOK, getExpanded.Code)
		expanded := decodeData(t, getExpanded)
		assert.Equal(t, "wf-exp", expanded["id"])
		rows, ok := expanded["tasks"].([]any)
		require.True(t, ok)
		require.Len(t, rows, 2)
		for i := range rows {
			row, ok := rows[i].(map[string]any)
			require.True(t, ok)
			assert.Contains(t, []any{"t1", "t2"}, row["id"])
			assert.Equal(t, "basic", row["type"])
		}
		assert.Equal(t, float64(2), expanded["task_count"])
	})

	t.Run("Should expand nested subtasks when requested", func(t *testing.T) {
		client := newResourceClient(t)
		nested := workflowPayload("wf-nested", "nested expand")
		nested["tasks"] = []map[string]any{
			{
				"id":    "parent",
				"type":  "composite",
				"tasks": []map[string]any{{"id": "child-a", "type": "basic"}, {"id": "child-b", "type": "basic"}},
			},
		}
		client.do(http.MethodPut, "/api/v0/workflows/wf-nested", nested, nil)
		res := client.do(http.MethodGet, "/api/v0/workflows/wf-nested?expand=tasks", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		data := decodeData(t, res)
		tasks, ok := data["tasks"].([]any)
		require.True(t, ok)
		require.Len(t, tasks, 1)
		parentTask, ok := tasks[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "parent", parentTask["id"])
		nestedTasks, ok := parentTask["tasks"].([]any)
		require.True(t, ok)
		require.Len(t, nestedTasks, 2)
		ids := make([]any, 0, len(nestedTasks))
		for i := range nestedTasks {
			child, ok := nestedTasks[i].(map[string]any)
			require.True(t, ok)
			ids = append(ids, child["id"])
		}
		assert.ElementsMatch(t, []any{"child-a", "child-b"}, ids)
	})

	t.Run("Should expand tools with typed DTO", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/tools/logger", toolPayload("logger", "GET", "https://example.com/log"), nil)
		wf := workflowPayload("wf-tools-expand", "expand tools")
		wf["tools"] = []map[string]any{{"id": "logger"}}
		client.do(http.MethodPut, "/api/v0/workflows/wf-tools-expand", wf, nil)
		res := client.do(http.MethodGet, "/api/v0/workflows/wf-tools-expand?expand=tools", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		data := decodeData(t, res)
		toolsVal, ok := data["tools"]
		require.True(t, ok)
		tools, ok := toolsVal.([]any)
		require.True(t, ok)
		require.Len(t, tools, 1)
		entry, ok := tools[0].(map[string]any)
		require.True(t, ok)
		id, ok := entry["id"].(string)
		require.True(t, ok)
		assert.Equal(t, "logger", id)
	})

	t.Run("Should ignore unknown workflow expand keys", func(t *testing.T) {
		client := newResourceClient(t)
		payload := workflowPayload("wf-expand-mixed", "mixed expand")
		payload["tasks"] = []map[string]any{{"id": "wf-child", "type": "basic"}}
		res := client.do(http.MethodPut, "/api/v0/workflows/wf-expand-mixed", payload, nil)
		require.Equal(t, http.StatusCreated, res.Code)
		getRes := client.do(http.MethodGet, "/api/v0/workflows/wf-expand-mixed?expand=tasks,unknown", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		items, ok := data["tasks"].([]any)
		require.True(t, ok)
		require.Len(t, items, 1)
		child, ok := items[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "wf-child", child["id"])
	})
	t.Run("Should reject invalid cursor on list", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/workflows/wf-c1", workflowPayload("wf-c1", "c1"), nil)
		res := client.do(http.MethodGet, "/api/v0/workflows?cursor=abc", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/json", res.Header().Get("Content-Type"))
	})

	t.Run("Should expand tasks in list when requested", func(t *testing.T) {
		client := newResourceClient(t)
		wf := workflowPayload("wf-list-exp", "list expand")
		wf["tasks"] = []map[string]any{{"id": "ex1", "type": "basic"}}
		client.do(http.MethodPut, "/api/v0/workflows/wf-list-exp", wf, nil)
		res := client.do(http.MethodGet, "/api/v0/workflows?q=wf-list-exp&expand=tasks", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, _ := decodeList(t, res, "workflows")
		require.Len(t, items, 1)
		assert.Equal(t, "wf-list-exp", items[0]["id"])
		ts, ok := items[0]["tasks"].([]any)
		require.True(t, ok)
		require.Len(t, ts, 1)
		taskObj, ok := ts[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "ex1", taskObj["id"])
		assert.Equal(t, "basic", taskObj["type"])
	})

	t.Run("Should ignore unknown expand", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/workflows/wf-unk", workflowPayload("wf-unk", "unk"), nil)
		res := client.do(http.MethodGet, "/api/v0/workflows/wf-unk?expand=unknown", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		data := decodeData(t, res)
		assert.Equal(t, "wf-unk", data["id"])
		assert.NotContains(t, data, "unknown")
	})
}
