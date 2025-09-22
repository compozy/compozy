package agentrouter

import (
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
)

// exportAgents handles POST /agents/export.
//
//	@Summary      Export agents
//	@Description  Write agent YAML files for the active project.
//	@Tags         agents
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Success      200  {object}  router.Response{data=map[string]int}  "Example: {\"data\":{\"written\":2},\"message\":\"export completed\"}"
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /agents/export [post]
func exportAgents(c *gin.Context) {
	router.ExportResource(c, resources.ResourceAgent)
}

// importAgents handles POST /agents/import.
//
//	@Summary      Import agents
//	@Description  Read agent YAML files from the project directory.
//	@Tags         agents
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Param        strategy  query  string  false  "seed_only|overwrite_conflicts"  Enums(seed_only,overwrite_conflicts)
//	@Success      200  {object}  router.Response{data=map[string]any}  "Example: {\"data\":{\"imported\":1,\"skipped\":1,\"overwritten\":0,\"strategy\":\"seed_only\"},\"message\":\"import completed\"}"
//	@Failure      400  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /agents/import [post]
func importAgents(c *gin.Context) {
	router.ImportResource(c, resources.ResourceAgent)
}
