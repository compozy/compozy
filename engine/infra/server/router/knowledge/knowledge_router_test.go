package knowledgerouter_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	knowledgerouter "github.com/compozy/compozy/engine/infra/server/router/knowledge"
	"github.com/compozy/compozy/engine/infra/server/router/routertest"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type apiResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	Error   any    `json:"error"`
}

func setupRouter(t *testing.T) (*gin.Engine, *appstate.State, resources.ResourceStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(router.ErrorHandler())
	state := routertest.NewTestAppState(t)
	store := routertest.NewResourceStore(state)
	r.Use(func(c *gin.Context) {
		ctx := appstate.WithState(c.Request.Context(), state)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	knowledgerouter.Register(r.Group(routes.Base()))
	return r, state, store
}

func seedKnowledge(t *testing.T, store resources.ResourceStore, project string, ids ...string) {
	t.Helper()
	embedder := knowledge.EmbedderConfig{
		ID:       "default-embedder",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Config: knowledge.EmbedderRuntimeConfig{
			Dimension:     1536,
			BatchSize:     16,
			StripNewLines: ptrBool(true),
		},
	}
	vector := knowledge.VectorDBConfig{
		ID:   "default-vector",
		Type: knowledge.VectorDBTypePGVector,
		Config: knowledge.VectorDBConnConfig{
			DSN:         "{{ env \"PGVECTOR_DSN\" }}",
			Table:       "knowledge_chunks",
			Index:       "knowledge_chunks_idx",
			EnsureIndex: true,
			Metric:      "cosine",
			Dimension:   1536,
		},
	}
	ctx := context.Background()
	key := resources.ResourceKey{Project: project, Type: resources.ResourceEmbedder, ID: embedder.ID}
	_, err := store.Put(ctx, key, &embedder)
	require.NoError(t, err)
	require.NoError(t, resources.WriteMeta(ctx, store, project, key.Type, key.ID, "test", "tests"))
	vecKey := resources.ResourceKey{Project: project, Type: resources.ResourceVectorDB, ID: vector.ID}
	_, err = store.Put(ctx, vecKey, &vector)
	require.NoError(t, err)
	require.NoError(t, resources.WriteMeta(ctx, store, project, vecKey.Type, vecKey.ID, "test", "tests"))
	for _, id := range ids {
		kb := knowledge.BaseConfig{
			ID:       id,
			Embedder: embedder.ID,
			VectorDB: vector.ID,
			Sources: []knowledge.SourceConfig{
				{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"},
			},
		}
		kbKey := resources.ResourceKey{Project: project, Type: resources.ResourceKnowledgeBase, ID: id}
		_, err = store.Put(ctx, kbKey, &kb)
		require.NoError(t, err)
		require.NoError(t, resources.WriteMeta(ctx, store, project, kbKey.Type, kbKey.ID, "test", "tests"))
	}
}

func ptrBool(v bool) *bool {
	return &v
}

func TestKnowledgeGetConditional(t *testing.T) {
	t.Run("ShouldReturn304WhenETagMatches", func(t *testing.T) {
		r, state, store := setupRouter(t)
		project := state.ProjectConfig.Name
		seedKnowledge(t, store, project, "support")
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			routes.Base()+"/knowledge-bases/support",
			http.NoBody,
		)
		require.NoError(t, err)
		req = routertest.WithConfig(t, req)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		etag := rec.Header().Get("ETag")
		require.NotEmpty(t, etag)
		req2, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			routes.Base()+"/knowledge-bases/support",
			http.NoBody,
		)
		require.NoError(t, err)
		req2.Header.Set("If-None-Match", etag)
		req2 = routertest.WithConfig(t, req2)
		rec2 := httptest.NewRecorder()
		r.ServeHTTP(rec2, req2)
		require.Equal(t, http.StatusNotModified, rec2.Code)
		require.Empty(t, rec2.Body.String())
	})
}

func TestKnowledgeUpsertETagPrecondition(t *testing.T) {
	t.Run("ShouldEnforceStrongETagPreconditions", func(t *testing.T) {
		r, state, store := setupRouter(t)
		project := state.ProjectConfig.Name
		seedKnowledge(t, store, project, "support")
		initial := map[string]any{
			"id":          "support",
			"embedder":    "default-embedder",
			"vector_db":   "default-vector",
			"sources":     []map[string]any{{"type": "markdown_glob", "path": "docs/**/*.md"}},
			"description": "initial",
		}
		payload, err := json.Marshal(initial)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPut,
			routes.Base()+"/knowledge-bases/support",
			bytes.NewReader(payload),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		etag := strings.Trim(rec.Header().Get("ETag"), "\"")
		require.NotEmpty(t, etag)
		update := map[string]any{
			"id":          "support",
			"embedder":    "default-embedder",
			"vector_db":   "default-vector",
			"sources":     []map[string]any{{"type": "markdown_glob", "path": "docs/**/*.md"}},
			"description": "updated",
		}
		body, err := json.Marshal(update)
		require.NoError(t, err)
		req2, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPut,
			routes.Base()+"/knowledge-bases/support",
			bytes.NewReader(body),
		)
		require.NoError(t, err)
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("If-Match", "\""+etag+"\"")
		req2 = routertest.WithConfig(t, req2)
		rec2 := httptest.NewRecorder()
		r.ServeHTTP(rec2, req2)
		require.Equal(t, http.StatusOK, rec2.Code)
		newETag := strings.Trim(rec2.Header().Get("ETag"), "\"")
		require.NotEmpty(t, newETag)
		require.NotEqual(t, etag, newETag)
		req3, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPut,
			routes.Base()+"/knowledge-bases/support",
			bytes.NewReader(body),
		)
		require.NoError(t, err)
		req3.Header.Set("Content-Type", "application/json")
		req3.Header.Set("If-Match", "\""+etag+"\"")
		req3 = routertest.WithConfig(t, req3)
		rec3 := httptest.NewRecorder()
		r.ServeHTTP(rec3, req3)
		require.Equal(t, http.StatusPreconditionFailed, rec3.Code)
		require.Contains(t, rec3.Header().Get("Content-Type"), "application/problem+json")
	})
}

func TestKnowledgeListPagination(t *testing.T) {
	t.Run("ShouldPaginateKnowledgeBaseListings", func(t *testing.T) {
		r, state, store := setupRouter(t)
		project := state.ProjectConfig.Name
		seedKnowledge(t, store, project, "alpha", "beta", "gamma")
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			routes.Base()+"/knowledge-bases?limit=1",
			http.NoBody,
		)
		require.NoError(t, err)
		req = routertest.WithConfig(t, req)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp apiResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		data, ok := resp.Data.(map[string]any)
		require.True(t, ok)
		bases, ok := data["knowledge_bases"].([]any)
		require.True(t, ok)
		require.Len(t, bases, 1)
		page, ok := data["page"].(map[string]any)
		require.True(t, ok)
		nextCursor, _ := page["next_cursor"].(string)
		require.NotEmpty(t, nextCursor)
		prevCursor, _ := page["prev_cursor"].(string)
		require.Empty(t, prevCursor)
		path := routes.Base() + "/knowledge-bases?limit=1&cursor=" + nextCursor
		req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
		require.NoError(t, err)
		req2 = routertest.WithConfig(t, req2)
		rec2 := httptest.NewRecorder()
		r.ServeHTTP(rec2, req2)
		require.Equal(t, http.StatusOK, rec2.Code)
		var resp2 apiResponse
		require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp2))
		data2, ok := resp2.Data.(map[string]any)
		require.True(t, ok)
		page2, ok := data2["page"].(map[string]any)
		require.True(t, ok)
		prev2, _ := page2["prev_cursor"].(string)
		require.NotEmpty(t, prev2)
	})
}

func TestKnowledgeQueryValidation(t *testing.T) {
	t.Run("ShouldRejectEmptyQueryPayload", func(t *testing.T) {
		r, state, store := setupRouter(t)
		project := state.ProjectConfig.Name
		seedKnowledge(t, store, project, "support")
		reqBody := map[string]any{}
		payload, err := json.Marshal(reqBody)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			routes.Base()+"/knowledge-bases/support/query",
			bytes.NewReader(payload),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Header().Get("Content-Type"), "application/problem+json")
	})
}

func TestSwaggerKnowledgePaths(t *testing.T) {
	t.Run("ShouldMatchGoldenSnapshot", func(t *testing.T) {
		root := filepath.Join("..", "..", "..", "..", "..")
		data, err := os.ReadFile(filepath.Join(root, "docs", "swagger.json"))
		require.NoError(t, err)
		var spec map[string]any
		require.NoError(t, json.Unmarshal(data, &spec))

		paths := map[string]any{}
		if rawPaths, ok := spec["paths"].(map[string]any); ok {
			for _, key := range []string{
				"/knowledge-bases",
				"/knowledge-bases/{kb_id}",
				"/knowledge-bases/{kb_id}/ingest",
				"/knowledge-bases/{kb_id}/query",
			} {
				if entry, exists := rawPaths[key]; exists {
					paths[key] = entry
				}
			}
		}

		tags := make([]any, 0, 1)
		if rawTags, ok := spec["tags"].([]any); ok {
			for _, tag := range rawTags {
				tagMap, ok := tag.(map[string]any)
				if !ok {
					continue
				}
				name, ok := tagMap["name"].(string)
				if !ok {
					continue
				}
				if strings.EqualFold(name, "knowledge") {
					tags = append(tags, tagMap)
				}
			}
		}

		subset := map[string]any{"paths": paths, "tags": tags}
		actual, err := json.MarshalIndent(subset, "", "  ")
		require.NoError(t, err)

		goldenPath := filepath.Join(
			root,
			"engine",
			"infra",
			"server",
			"router",
			"testdata",
			"swagger",
			"knowledge.json",
		)
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			require.NoError(t, os.WriteFile(goldenPath, actual, 0o600))
		}
		expected, err := os.ReadFile(goldenPath)
		require.NoError(t, err)
		require.JSONEq(t, string(expected), string(actual))
	})
}
