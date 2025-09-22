package resources

import (
	"net/http"
	"testing"

	storepkg "github.com/compozy/compozy/engine/resources"
	schemacfg "github.com/compozy/compozy/engine/schema"
	workflowcfg "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemasEndpoints(t *testing.T) {
	t.Run("Should create and manage schemas", func(t *testing.T) {
		client := newResourceClient(t)
		createBody := schemaPayload(map[string]any{"name": map[string]any{"type": "string"}})
		createRes := client.do(http.MethodPut, "/api/v0/schemas/user", createBody, nil)
		require.Equal(t, http.StatusCreated, createRes.Code)
		etag := createRes.Header().Get("ETag")
		require.NotEmpty(t, etag)
		require.Equal(t, "/api/v0/schemas/user", createRes.Header().Get("Location"))
		getRes := client.do(http.MethodGet, "/api/v0/schemas/user", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		body, ok := data["body"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "object", body["type"])
		updateBody := schemaPayload(map[string]any{
			"name": map[string]any{"type": "string"},
			"age":  map[string]any{"type": "number"},
		})
		updateRes := client.do(http.MethodPut, "/api/v0/schemas/user", updateBody, map[string]string{"If-Match": etag})
		require.Equal(t, http.StatusOK, updateRes.Code)
		newEtag := updateRes.Header().Get("ETag")
		require.NotEmpty(t, newEtag)
		assert.NotEqual(t, etag, newEtag)
		afterRes := client.do(http.MethodGet, "/api/v0/schemas/user", nil, nil)
		require.Equal(t, http.StatusOK, afterRes.Code)
		assert.Equal(t, newEtag, afterRes.Header().Get("ETag"))
		afterData := decodeData(t, afterRes)
		afterBody, ok := afterData["body"].(map[string]any)
		require.True(t, ok)
		props, ok := afterBody["properties"].(map[string]any)
		require.True(t, ok)
		_, hasAge := props["age"]
		assert.True(t, hasAge)
		staleRes := client.do(
			http.MethodPut,
			"/api/v0/schemas/user",
			updateBody,
			map[string]string{"If-Match": "\"schema-bad\""},
		)
		require.Equal(t, http.StatusPreconditionFailed, staleRes.Code)
		assert.Contains(t, staleRes.Header().Get("Content-Type"), "application/problem+json")
		client.do(http.MethodPut, "/api/v0/schemas/audit", schemaPayload(map[string]any{}), nil)
		// Walk paginated list and collect schema IDs from typed items (body.id)
		path := "/api/v0/schemas?limit=1"
		visited := map[string]bool{}
		collected := make([]string, 0)
		for path != "" {
			if visited[path] {
				break
			}
			visited[path] = true
			res := client.do(http.MethodGet, path, nil, nil)
			require.Equal(t, http.StatusOK, res.Code)
			items, _ := decodeList(t, res, "schemas")
			for i := range items {
				body, ok := items[i]["body"].(map[string]any)
				require.True(t, ok)
				if id, _ := body["id"].(string); id != "" {
					collected = append(collected, id)
				}
			}
			nextLink := extractLink(res.Header().Get("Link"), "next")
			if nextLink == "" {
				break
			}
			path = normalizeLink(nextLink)
		}
		assert.ElementsMatch(t, []string{"audit", "user"}, collected)
		delRes := client.do(http.MethodDelete, "/api/v0/schemas/user", nil, nil)
		require.Equal(t, http.StatusNoContent, delRes.Code)
		missingRes := client.do(http.MethodGet, "/api/v0/schemas/user", nil, nil)
		require.Equal(t, http.StatusNotFound, missingRes.Code)
	})
	t.Run("Should report conflict when workflow uses schema", func(t *testing.T) {
		client := newResourceClient(t)
		body := schemaPayload(map[string]any{})
		res := client.do(http.MethodPut, "/api/v0/schemas/contact", body, nil)
		require.Equal(t, http.StatusCreated, res.Code)
		store := client.store()
		schemaRef := schemacfg.Schema(map[string]any{"__schema_ref__": "contact"})
		wf := &workflowcfg.Config{ID: "wf-schema", Opts: workflowcfg.Opts{InputSchema: &schemaRef}}
		_, err := store.Put(
			client.harness.Ctx,
			storepkg.ResourceKey{Project: client.harness.Project.Name, Type: storepkg.ResourceWorkflow, ID: wf.ID},
			wf,
		)
		require.NoError(t, err)
		delRes := client.do(http.MethodDelete, "/api/v0/schemas/contact", nil, nil)
		require.Equal(t, http.StatusConflict, delRes.Code)
		assert.Equal(t, "application/problem+json", delRes.Header().Get("Content-Type"))
		assert.Contains(t, delRes.Body.String(), "workflows")
	})
	t.Run("Should reject malformed schema payload", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodPut, "/api/v0/schemas/bad", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/problem+json", res.Header().Get("Content-Type"))
	})
}

func TestSchemasQueries(t *testing.T) {
	t.Run("Should filter by prefix q", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(
			http.MethodPut,
			"/api/v0/schemas/a-user",
			schemaPayload(map[string]any{"n": map[string]any{"type": "string"}}),
			nil,
		)
		client.do(http.MethodPut, "/api/v0/schemas/b-audit", schemaPayload(map[string]any{}), nil)
		res := client.do(http.MethodGet, "/api/v0/schemas?q=a-", nil, nil)
		require.Equal(t, http.StatusOK, res.Code)
		items, page := decodeList(t, res, "schemas")
		assert.Equal(t, float64(1), page["total"])
		// list item wraps schema under body.id
		body, ok := items[0]["body"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "a-user", body["id"].(string))
	})
	t.Run("Should reject invalid cursor on list", func(t *testing.T) {
		client := newResourceClient(t)
		client.do(http.MethodPut, "/api/v0/schemas/c1", schemaPayload(map[string]any{}), nil)
		res := client.do(http.MethodGet, "/api/v0/schemas?cursor=abc", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/problem+json", res.Header().Get("Content-Type"))
	})
}
