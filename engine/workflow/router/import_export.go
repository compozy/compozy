package wfrouter

import (
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
)

// exportWorkflows handles POST /workflows/export.
//
//	@Summary      Export workflows
//	@Description  Write workflow YAML files for the active project.
//	@Tags         workflows
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Success      200  {object}  router.Response{data=map[string]int}  "Example: {\"data\":{\"written\":2},\"message\":\"export completed\"}"
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /workflows/export [post]
func exportWorkflows(c *gin.Context) {
	router.ExportResource(c, resources.ResourceWorkflow)
}

// importWorkflows handles POST /workflows/import.
//
//	@Summary      Import workflows
//	@Description  Read workflow YAML files from the project directory.
//	@Tags         workflows
//	@Produce      json
//	@Security     ApiKeyAuth
//	@Param        strategy  query  string  false  "seed_only|overwrite_conflicts"  Enums(seed_only,overwrite_conflicts)
//	@Success      200  {object}  router.Response{data=map[string]any}  "Example: {\"data\":{\"imported\":2,\"skipped\":0,\"overwritten\":0,\"strategy\":\"seed_only\"},\"message\":\"import completed\"}"
//	@Failure      400  {object}  router.Response{error=router.ErrorInfo}
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo}
//	@Router       /workflows/import [post]
func importWorkflows(c *gin.Context) {
	router.ImportResource(c, resources.ResourceWorkflow)
}
