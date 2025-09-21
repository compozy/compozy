package server

import (
	"path/filepath"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources/exporter"
	"github.com/gin-gonic/gin"
)

// adminExportYAMLHandler handles exporting store content to YAML files under the project CWD.
//
//	@Summary      Export store contents to YAML
//	@Description  Writes deterministic YAML files to project directories.
//	@Description  Targets: agents/, tools/, workflows/, schemas/, mcps/, models/.
//	@Description  Admin only.
//	@Security     ApiKeyAuth
//	@Tags         admin
//	@Produce      json
//	@Success      200  {object}  router.Response{data=map[string]any}  "Example: {\"data\":{\"workflow\":5,\"agent\":2},\"message\":\"export completed\"}"
//	@Failure      401  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      403  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /admin/export-yaml [post]
func adminExportYAMLHandler(c *gin.Context) {
	st := router.GetAppState(c)
	if st == nil {
		return
	}
	store, ok := getResourceStoreFromState(st)
	if !ok {
		router.RespondWithServerError(c, router.ErrInternalCode, "resource store not available", nil)
		return
	}
	cwd, _, ok := extractProjectPaths(st)
	if !ok {
		router.RespondWithServerError(c, router.ErrInternalCode, "project configuration not available", nil)
		return
	}
	project := st.ProjectConfig.Name
	root := filepath.Clean(cwd)
	out, err := exporter.ExportToDir(c.Request.Context(), project, store, root)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "export failed", err)
		return
	}
	payload := map[string]any{}
	for k, v := range out.Written {
		payload[string(k)] = v
	}
	router.RespondOK(c, "export completed", payload)
}
