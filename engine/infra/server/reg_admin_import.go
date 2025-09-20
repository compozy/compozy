package server

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources/importer"
	"github.com/gin-gonic/gin"
)

// adminImportYAMLHandler handles importing YAML files from the project CWD into the ResourceStore.
//
//	@Summary      Import YAML into store
//	@Description  Reads YAML from project directories and upserts to ResourceStore.
//	@Description  Strategies: seed_only | overwrite_conflicts. Admin only.
//	@Tags         admin
//	@Produce      json
//	@Param        strategy  query  string  false  "seed_only|overwrite_conflicts"  Enums(seed_only,overwrite_conflicts)
//	@Success      200  {object}  router.Response{data=map[string]any}
//	@Failure      401  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      403  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      409  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /api/v0/admin/import-yaml [post]
func adminImportYAMLHandler(c *gin.Context) {
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
	stratParam := strings.TrimSpace(strings.ToLower(c.Query("strategy")))
	var start importer.Strategy
	switch stratParam {
	case "", "seed_only":
		start = importer.SeedOnly
	case "overwrite_conflicts":
		start = importer.OverwriteConflicts
	default:
		router.RespondWithError(
			c,
			http.StatusBadRequest,
			router.NewRequestError(http.StatusBadRequest, "invalid strategy", nil),
		)
		return
	}
	user := "admin"
	if u, ok := userctx.UserFromContext(c.Request.Context()); ok && u != nil {
		user = u.Email
		if user == "" {
			user = u.ID.String()
		}
	}
	root := filepath.Clean(cwd)
	out, err := importer.ImportFromDir(c.Request.Context(), st.ProjectConfig.Name, store, root, start, user)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "import failed", err)
		return
	}
	payload := map[string]any{
		"imported":    out.Imported,
		"skipped":     out.Skipped,
		"overwritten": out.Overwritten,
		"strategy":    string(start),
	}
	router.RespondOK(c, "import completed", payload)
}
