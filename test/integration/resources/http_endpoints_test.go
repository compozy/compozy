package resourcesintegration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/resources"
	testshelpers "github.com/compozy/compozy/test/helpers/server"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type apiResponse struct {
	Status  int            `json:"status"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
	Error   *apiError      `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func performJSONRequest(
	t *testing.T,
	engine *gin.Engine,
	method, path string,
	body []byte,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)
	return res
}

func decodeResponse(t *testing.T, res *httptest.ResponseRecorder) apiResponse {
	t.Helper()
	var out apiResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &out))
	return out
}

func createModelPayload(t *testing.T, id, provider, model string) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"id": id, "type": "model", "provider": provider, "model": model})
	require.NoError(t, err)
	return payload
}

func TestResourcesIntegration_CRUDAndListing(t *testing.T) {
	harness := testshelpers.NewServerHarness(t)
	engine := harness.Engine
	t.Run("Should create fetch update list and delete resources with ETag control", func(t *testing.T) {
		createBody := createModelPayload(t, "model-one", "openai", "gpt-4")
		createRes := performJSONRequest(t, engine, http.MethodPost, "/api/v0/resources/model", createBody, nil)
		require.Equal(t, http.StatusCreated, createRes.Code)
		etag := strings.TrimSpace(createRes.Header().Get("ETag"))
		require.NotEmpty(t, etag)
		location := createRes.Header().Get("Location")
		require.Equal(t, "/api/v0/resources/model/model-one", location)
		getRes := performJSONRequest(t, engine, http.MethodGet, "/api/v0/resources/model/model-one", nil, nil)
		require.Equal(t, http.StatusOK, getRes.Code)
		getBody := decodeResponse(t, getRes)
		value := getBody.Data
		require.Equal(t, "openai", value["provider"])
		require.Equal(t, etag, getRes.Header().Get("ETag"))
		putBody := createModelPayload(t, "model-one", "openai", "gpt-4.1")
		putHeaders := map[string]string{"If-Match": etag}
		putRes := performJSONRequest(
			t,
			engine,
			http.MethodPut,
			"/api/v0/resources/model/model-one",
			putBody,
			putHeaders,
		)
		require.Equal(t, http.StatusOK, putRes.Code)
		updatedETag := strings.TrimSpace(putRes.Header().Get("ETag"))
		require.NotEmpty(t, updatedETag)
		require.NotEqual(t, etag, updatedETag)
		staleHeaders := map[string]string{"If-Match": etag}
		staleRes := performJSONRequest(
			t,
			engine,
			http.MethodPut,
			"/api/v0/resources/model/model-one",
			putBody,
			staleHeaders,
		)
		require.Equal(t, http.StatusConflict, staleRes.Code)
		listBody := createModelPayload(t, "model-two", "openai", "gpt-4o")
		require.Equal(
			t,
			http.StatusCreated,
			performJSONRequest(t, engine, http.MethodPost, "/api/v0/resources/model", listBody, nil).Code,
		)
		listRes := performJSONRequest(t, engine, http.MethodGet, "/api/v0/resources/model?q=model-", nil, nil)
		require.Equal(t, http.StatusOK, listRes.Code)
		listPayload := decodeResponse(t, listRes)
		keysRaw, ok := listPayload.Data["keys"].([]any)
		require.True(t, ok)
		require.ElementsMatch(t, []string{"model-one", "model-two"}, []string{keysRaw[0].(string), keysRaw[1].(string)})
		deleteRes := performJSONRequest(t, engine, http.MethodDelete, "/api/v0/resources/model/model-one", nil, nil)
		require.Equal(t, http.StatusOK, deleteRes.Code)
		deleteAgain := performJSONRequest(t, engine, http.MethodDelete, "/api/v0/resources/model/model-one", nil, nil)
		require.Equal(t, http.StatusOK, deleteAgain.Code)
		missing := performJSONRequest(t, engine, http.MethodGet, "/api/v0/resources/model/model-one", nil, nil)
		require.Equal(t, http.StatusNotFound, missing.Code)
	})
}

func TestResourcesIntegration_ValidationErrors(t *testing.T) {
	harness := testshelpers.NewServerHarness(t)
	engine := harness.Engine
	t.Run("Should reject invalid creation payloads", func(t *testing.T) {
		noID := []byte(`{"type":"model","provider":"openai","model":"gpt"}`)
		require.Equal(
			t,
			http.StatusBadRequest,
			performJSONRequest(t, engine, http.MethodPost, "/api/v0/resources/model", noID, nil).Code,
		)
		projectField := []byte(`{"id":"model-x","type":"model","project":"alt","provider":"openai","model":"gpt"}`)
		require.Equal(
			t,
			http.StatusBadRequest,
			performJSONRequest(t, engine, http.MethodPost, "/api/v0/resources/model", projectField, nil).Code,
		)
		badID := []byte(`{"id":"model x","type":"model","provider":"openai","model":"gpt"}`)
		require.Equal(
			t,
			http.StatusBadRequest,
			performJSONRequest(t, engine, http.MethodPost, "/api/v0/resources/model", badID, nil).Code,
		)
		typeMismatch := []byte(`{"id":"model-y","type":"agent","provider":"openai","model":"gpt"}`)
		require.Equal(
			t,
			http.StatusBadRequest,
			performJSONRequest(t, engine, http.MethodPost, "/api/v0/resources/model", typeMismatch, nil).Code,
		)
		require.Equal(
			t,
			http.StatusBadRequest,
			performJSONRequest(
				t,
				engine,
				http.MethodPost,
				"/api/v0/resources/unknown",
				createModelPayload(t, "model-z", "openai", "gpt"),
				nil,
			).Code,
		)
		req := httptest.NewRequest(http.MethodPost, "/api/v0/resources/model", strings.NewReader("{invalid"))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, req)
		require.Equal(t, http.StatusBadRequest, res.Code)
	})
	t.Run("Should surface get and put validation errors", func(t *testing.T) {
		require.Equal(
			t,
			http.StatusCreated,
			performJSONRequest(
				t,
				engine,
				http.MethodPost,
				"/api/v0/resources/model",
				createModelPayload(t, "model-base", "openai", "gpt-4"),
				nil,
			).Code,
		)
		blankID := performJSONRequest(t, engine, http.MethodGet, "/api/v0/resources/model/%20", nil, nil)
		require.Equal(t, http.StatusBadRequest, blankID.Code)
		missing := performJSONRequest(t, engine, http.MethodGet, "/api/v0/resources/model/missing", nil, nil)
		require.Equal(t, http.StatusNotFound, missing.Code)
		mismatchBody := []byte(`{"id":"other","type":"model","provider":"openai","model":"gpt"}`)
		require.Equal(
			t,
			http.StatusBadRequest,
			performJSONRequest(t, engine, http.MethodPut, "/api/v0/resources/model/model-base", mismatchBody, nil).Code,
		)
		wrongType := []byte(`{"id":"model-base","type":"agent","provider":"openai","model":"gpt"}`)
		require.Equal(
			t,
			http.StatusBadRequest,
			performJSONRequest(t, engine, http.MethodPut, "/api/v0/resources/model/model-base", wrongType, nil).Code,
		)
		badID := []byte(`{"id":"bad id","type":"model","provider":"openai","model":"gpt"}`)
		require.Equal(
			t,
			http.StatusBadRequest,
			performJSONRequest(t, engine, http.MethodPut, "/api/v0/resources/model/model-base", badID, nil).Code,
		)
		staleHeaders := map[string]string{"If-Match": "bogus"}
		require.Equal(
			t,
			http.StatusConflict,
			performJSONRequest(
				t,
				engine,
				http.MethodPut,
				"/api/v0/resources/model/missing",
				createModelPayload(t, "missing", "openai", "gpt"),
				staleHeaders,
			).Code,
		)
	})
	t.Run("Should reject invalid import strategy", func(t *testing.T) {
		res := performJSONRequest(t, engine, http.MethodPost, "/api/v0/admin/import-yaml?strategy=unknown", nil, nil)
		require.Equal(t, http.StatusBadRequest, res.Code)
	})
}

func TestResourcesIntegration_ImportExport(t *testing.T) {
	harness := testshelpers.NewServerHarness(t)
	engine := harness.Engine
	projectDir := harness.Project.GetCWD().PathStr()
	t.Run("Should export resources to deterministic YAML", func(t *testing.T) {
		require.Equal(
			t,
			http.StatusCreated,
			performJSONRequest(
				t,
				engine,
				http.MethodPost,
				"/api/v0/resources/model",
				createModelPayload(t, "to-export", "openai", "gpt-4"),
				nil,
			).Code,
		)
		exportRes := performJSONRequest(t, engine, http.MethodGet, "/api/v0/admin/export-yaml", nil, nil)
		require.Equal(t, http.StatusOK, exportRes.Code)
		out := decodeResponse(t, exportRes)
		count, ok := out.Data["model"].(float64)
		require.True(t, ok)
		require.Equal(t, 1.0, count)
		modelPath := filepath.Join(projectDir, "models", "to-export.yaml")
		content, err := os.ReadFile(modelPath)
		require.NoError(t, err)
		require.Contains(t, string(content), "id: to-export")
	})
	t.Run("Should import YAML resources with overwrite strategy", func(t *testing.T) {
		repoModelsDir := filepath.Join(projectDir, "models")
		require.NoError(t, os.MkdirAll(repoModelsDir, 0o755))
		seedPath := filepath.Join(repoModelsDir, "seed-model.yaml")
		seedContent := "id: seed-model\ntype: model\nprovider: openai\nmodel: gpt-4\n"
		require.NoError(t, os.WriteFile(seedPath, []byte(seedContent), 0o600))
		importRes := performJSONRequest(
			t,
			engine,
			http.MethodPost,
			"/api/v0/admin/import-yaml?strategy=seed_only",
			nil,
			nil,
		)
		require.Equal(t, http.StatusOK, importRes.Code)
		resp := decodeResponse(t, importRes)
		importedMap, ok := resp.Data["imported"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, 1.0, importedMap["model"])
		updatedContent := "id: seed-model\ntype: model\nprovider: vertex\nmodel: gemini\n"
		require.NoError(t, os.WriteFile(seedPath, []byte(updatedContent), 0o600))
		overwriteRes := performJSONRequest(
			t,
			engine,
			http.MethodPost,
			"/api/v0/admin/import-yaml?strategy=overwrite_conflicts",
			nil,
			nil,
		)
		require.Equal(t, http.StatusOK, overwriteRes.Code)
		overwritePayload := decodeResponse(t, overwriteRes)
		overwritten, ok := overwritePayload.Data["overwritten"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, 1.0, overwritten["model"])
		val, et, err := harness.ResourceStore.Get(
			harness.Ctx,
			resources.ResourceKey{Project: harness.Project.Name, Type: resources.ResourceModel, ID: "seed-model"},
		)
		require.NoError(t, err)
		require.NotEmpty(t, et)
		valueMap, ok := val.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "vertex", valueMap["provider"])
	})
}
