package tkrouter

import (
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
)

// exportTasks handles POST /api/v0/tasks/export.
//
//	@Summary      Export tasks
//	@Description  Write task YAML files for the active project.
//	@Tags         tasks
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Success      200  {object}  router.Response{data=map[string]any}  "Example: {\"data\":{\"written\":6},\"message\":\"export completed\"}"
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /tasks/export [post]
func exportTasks(c *gin.Context) {
	router.ExportResource(c, resources.ResourceTask)
}

// importTasks handles POST /api/v0/tasks/import.
//
//	@Summary      Import tasks
//	@Description  Read task YAML files from the project directory.
//	@Tags         tasks
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Param        strategy  query  string  false  "seed_only|overwrite_conflicts"  Enums(seed_only,overwrite_conflicts)
//	@Success      200  {object}  router.Response{data=map[string]any}  "Example: {\"data\":{\"imported\":4,\"skipped\":1,\"overwritten\":0,\"strategy\":\"seed_only\"},\"message\":\"import completed\"}"
//	@Failure      400  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /tasks/import [post]
func importTasks(c *gin.Context) {
	router.ImportResource(c, resources.ResourceTask)
}
