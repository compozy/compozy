package resources

import (
	"net/http"
	"testing"

	agentcfg "github.com/compozy/compozy/engine/agent"
	storepkg "github.com/compozy/compozy/engine/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelsEndpoints(t *testing.T) {
	t.Run("Should upsert and retrieve models", func(t *testing.T) {
		client := newResourceClient(t)
		createRes := client.do(
			http.MethodPut,
			"/api/v0/models/openai-gpt-4o-mini",
			modelPayload("openai", "gpt-4o-mini"),
			nil,
		)
		require.Equal(t, http.StatusCreated, createRes.Code)
		etag := createRes.Header().Get("ETag")
		require.NotEmpty(t, etag)
		require.Equal(t, "/api/v0/models/openai-gpt-4o-mini", createRes.Header().Get("Location"))
		getRes := client.do(http.MethodGet, "/api/v0/models/openai-gpt-4o-mini", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		data := decodeData(t, getRes)
		assert.Equal(t, "gpt-4o-mini", data["model"])
		updateBody := cloneMap(modelPayload("openai", "gpt-4o-mini"))
		updateBody["params"] = map[string]any{"temperature": 0.1}
		updateRes := client.do(
			http.MethodPut,
			"/api/v0/models/openai-gpt-4o-mini",
			updateBody,
			map[string]string{"If-Match": etag},
		)
		require.Equal(t, http.StatusOK, updateRes.Code)
		newEtag := updateRes.Header().Get("ETag")
		require.NotEmpty(t, newEtag)
		afterRes := client.do(http.MethodGet, "/api/v0/models/openai-gpt-4o-mini", nil, nil)
		require.Equal(t, http.StatusOK, afterRes.Code)
		assert.Equal(t, newEtag, afterRes.Header().Get("ETag"))
		afterData := decodeData(t, afterRes)
		params, ok := afterData["params"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 0.1, params["temperature"])
		staleRes := client.do(
			http.MethodPut,
			"/api/v0/models/openai-gpt-4o-mini",
			updateBody,
			map[string]string{"If-Match": "\"invalid\""},
		)
		require.Equal(t, http.StatusPreconditionFailed, staleRes.Code)
		client.do(http.MethodPut, "/api/v0/models/anthropic-sonnet", modelPayload("anthropic", "sonnet"), nil)
		ids := collectIDs(t, client, "/api/v0/models?limit=1", "models", "model")
		assert.ElementsMatch(t, []string{"gpt-4o-mini", "sonnet"}, ids)
		delRes := client.do(http.MethodDelete, "/api/v0/models/openai-gpt-4o-mini", nil, nil)
		require.Equal(t, http.StatusNoContent, delRes.Code)
		missingRes := client.do(http.MethodGet, "/api/v0/models/openai-gpt-4o-mini", nil, nil)
		require.Equal(t, http.StatusNotFound, missingRes.Code)
	})
	t.Run("Should prevent deleting model in use", func(t *testing.T) {
		client := newResourceClient(t)
		body := modelPayload("openai", "gpt-big")
		res := client.do(http.MethodPut, "/api/v0/models/openai-gpt-big", body, nil)
		require.Equal(t, http.StatusCreated, res.Code)
		store := client.store()
		ag := &agentcfg.Config{ID: "analyst"}
		ag.Model.Ref = "openai-gpt-big"
		_, err := store.Put(
			client.harness.Ctx,
			storepkg.ResourceKey{Project: client.harness.Project.Name, Type: storepkg.ResourceAgent, ID: ag.ID},
			ag,
		)
		require.NoError(t, err)
		delRes := client.do(http.MethodDelete, "/api/v0/models/openai-gpt-big", nil, nil)
		require.Equal(t, http.StatusConflict, delRes.Code)
		assert.Equal(t, "application/problem+json", delRes.Header().Get("Content-Type"))
		assert.Contains(t, delRes.Body.String(), "agents")
	})
	t.Run("Should reject malformed model payload", func(t *testing.T) {
		client := newResourceClient(t)
		res := client.do(http.MethodPut, "/api/v0/models/openai:broken", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/problem+json", res.Header().Get("Content-Type"))
	})
}
