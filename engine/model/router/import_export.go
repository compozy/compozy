package modelrouter

import (
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
)

// -----------------------------------------------------------------------------
// Handlers
// -----------------------------------------------------------------------------

// exportModels handles POST /models/export.
//
//	@Summary      Export models
//	@Description  Write model YAML files for the active project.
//	@Tags         models
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Success      200  {object}  router.Response{data=map[string]int}  "Example: {\"data\":{\"written\":4},\"message\":\"export completed\"}"
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /models/export [post]
func exportModels(c *gin.Context) {
	router.ExportResource(c, resources.ResourceModel)
}

// importModels handles POST /models/import.
//
//	@Summary      Import models
//	@Description  Read model YAML files from the project directory.
//	@Tags         models
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Param        strategy  query  string  false  "seed_only|overwrite_conflicts"  Enums(seed_only,overwrite_conflicts)
//	@Success      200  {object}  router.Response{data=map[string]any}  "Example: {\"data\":{\"imported\":2,\"skipped\":0,\"overwritten\":1,\"strategy\":\"seed_only\"},\"message\":\"import completed\"}"
//	@Failure      400  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /models/import [post]
func importModels(c *gin.Context) {
	router.ImportResource(c, resources.ResourceModel)
}
