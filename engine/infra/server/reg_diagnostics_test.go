package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/routes"
	ginmode "github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetupDiagnosticEndpoints(t *testing.T) {
	ginmode.EnsureGinTestMode()
	version := core.GetVersion()
	base := routes.Base()
	engine := gin.New()
	setupDiagnosticEndpoints(engine, version, base, nil)

	t.Run("Should return identical metadata for root and versioned base", func(t *testing.T) {
		recorderRoot := httptest.NewRecorder()
		recorderAPI := httptest.NewRecorder()
		requestRoot := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		requestRoot.Host = "example.com"
		requestAPI := httptest.NewRequest(http.MethodGet, base, http.NoBody)
		requestAPI.Host = "example.com"
		engine.ServeHTTP(recorderRoot, requestRoot)
		require.Equal(t, http.StatusOK, recorderRoot.Code)
		require.Equal(t, "application/json; charset=utf-8", recorderRoot.Header().Get("Content-Type"))
		engine.ServeHTTP(recorderAPI, requestAPI)
		require.Equal(t, http.StatusOK, recorderAPI.Code)
		require.Equal(t, "application/json; charset=utf-8", recorderAPI.Header().Get("Content-Type"))
		var rootBody map[string]any
		var apiBody map[string]any
		require.NoError(t, json.Unmarshal(recorderRoot.Body.Bytes(), &rootBody))
		require.NoError(t, json.Unmarshal(recorderAPI.Body.Bytes(), &apiBody))
		require.Equal(t, rootBody, apiBody)
	})
}
