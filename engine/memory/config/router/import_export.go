package memoryrouter

import (
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
)

// exportMemories handles POST /memories/export.
//
//	@Summary      Export memories
//	@Description  Write memory YAML files for the active project.
//	@Tags         memories
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Success      200  {object}  router.Response{data=map[string]int}  "Example: {\"data\":{\"written\":5},\"message\":\"export completed\"}"
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /memories/export [post]
func exportMemories(c *gin.Context) {
	router.ExportResource(c, resources.ResourceMemory)
}

// importMemories handles POST /memories/import.
//
//	@Summary      Import memories
//	@Description  Read memory YAML files from the project directory.
//	@Tags         memories
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Param        strategy  query  string  false  "seed_only|overwrite_conflicts"  Enums(seed_only,overwrite_conflicts)
//	@Success      200  {object}  router.Response{data=map[string]any}  "Example: {\"data\":{\"imported\":3,\"skipped\":1,\"overwritten\":1,\"strategy\":\"seed_only\"},\"message\":\"import completed\"}"
//	@Failure      400  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /memories/import [post]
func importMemories(c *gin.Context) {
	router.ImportResource(c, resources.ResourceMemory)
}
