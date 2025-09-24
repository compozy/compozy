package mcprouter

import (
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
)

// exportMCPs handles POST /mcps/export.
//
//	@Summary      Export MCPs
//	@Description  Write MCP YAML files for the active project.
//	@Tags         mcps
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Success      200  {object}  router.Response{data=map[string]int}  "Example: {\"data\":{\"written\":3},\"message\":\"export completed\"}"
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /mcps/export [post]
func exportMCPs(c *gin.Context) {
	router.ExportResource(c, resources.ResourceMCP)
}

// importMCPs handles POST /mcps/import.
//
//	@Summary      Import MCPs
//	@Description  Read MCP YAML files from the project directory.
//	@Tags         mcps
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Param        strategy  query  string  false  "seed_only|overwrite_conflicts"  Enums(seed_only,overwrite_conflicts)
//	@Success      200  {object}  router.Response{data=map[string]any}  "Example: {\"data\":{\"imported\":1,\"skipped\":0,\"overwritten\":0,\"strategy\":\"seed_only\"},\"message\":\"import completed\"}"
//	@Failure      400  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /mcps/import [post]
func importMCPs(c *gin.Context) {
	router.ImportResource(c, resources.ResourceMCP)
}
