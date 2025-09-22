package toolrouter

import (
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
)

// exportTools handles POST /tools/export.
//
//	@Summary      Export tools
//	@Description  Write tool YAML files for the active project.
//	@Tags         tools
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Success      200  {object}  router.Response{data=map[string]int}  "Example: {\"data\":{\"written\":3},\"message\":\"export completed\"}"
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /tools/export [post]
func exportTools(c *gin.Context) {
	router.ExportResource(c, resources.ResourceTool)
}

// importTools handles POST /tools/import.
//
//	@Summary      Import tools
//	@Description  Read tool YAML files from the project directory.
//	@Tags         tools
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Param        strategy  query  string  false  "seed_only|overwrite_conflicts"  Enums(seed_only,overwrite_conflicts)
//	@Success      200  {object}  router.Response{data=map[string]any}  "Example: {\"data\":{\"imported\":2,\"skipped\":1,\"overwritten\":0,\"strategy\":\"seed_only\"},\"message\":\"import completed\"}"
//	@Failure      400  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /tools/import [post]
func importTools(c *gin.Context) {
	router.ImportResource(c, resources.ResourceTool)
}
