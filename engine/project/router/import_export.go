package projectrouter

import (
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
)

// exportProject handles POST /project/export.
//
//	@Summary      Export project
//	@Description  Write the project YAML file for the active project.
//	@Tags         project
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Success      200  {object}  router.Response{data=map[string]int}  "Example: {\"data\":{\"written\":1},\"message\":\"export completed\"}"
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /project/export [post]
func exportProject(c *gin.Context) {
	router.ExportResource(c, resources.ResourceProject)
}

// importProject handles POST /project/import.
//
//	@Summary      Import project
//	@Description  Read the project YAML file from the project directory.
//	@Tags         project
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Param        strategy  query  string  false  "seed_only|overwrite_conflicts"  Enums(seed_only,overwrite_conflicts)
//	@Success      200  {object}  router.Response{data=map[string]any}  "Example: {\"data\":{\"imported\":1,\"skipped\":0,\"overwritten\":0,\"strategy\":\"seed_only\"},\"message\":\"import completed\"}"
//	@Failure      400  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /project/import [post]
func importProject(c *gin.Context) {
	router.ImportResource(c, resources.ResourceProject)
}
