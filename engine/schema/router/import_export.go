package schemarouter

import (
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
)

// exportSchemas handles POST /schemas/export.
//
//	@Summary      Export schemas
//	@Description  Write schema YAML files for the active project.
//	@Tags         schemas
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Success      200  {object}  router.Response{data=map[string]int}  "Example: {\"data\":{\"written\":2},\"message\":\"export completed\"}"
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /schemas/export [post]
func exportSchemas(c *gin.Context) {
	router.ExportResource(c, resources.ResourceSchema)
}

// importSchemas handles POST /schemas/import.
//
//	@Summary      Import schemas
//	@Description  Read schema YAML files from the project directory.
//	@Tags         schemas
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Param        strategy  query  string  false  "seed_only|overwrite_conflicts"  Enums(seed_only,overwrite_conflicts)
//	@Success      200  {object}  router.Response{data=map[string]any}  "Example: {\"data\":{\"imported\":1,\"skipped\":1,\"overwritten\":0,\"strategy\":\"overwrite_conflicts\"},\"message\":\"import completed\"}"
//	@Failure      400  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /schemas/import [post]
func importSchemas(c *gin.Context) {
	router.ImportResource(c, resources.ResourceSchema)
}
