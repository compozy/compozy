package resourcesrouter

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup) {
	group := apiBase.Group("/resources")
	group.POST("/:type", createResource)
	group.GET("/:type", listResources)
	group.GET("/:type/:id", getResourceByID)
	group.PUT("/:type/:id", putResourceByID)
	group.DELETE("/:type/:id", deleteResourceByID)
}

func resourceTypeFromPath(c *gin.Context) (resources.ResourceType, bool) {
	t := strings.TrimSpace(c.Param("type"))
	switch t {
	case "agent":
		return resources.ResourceAgent, true
	case "tool":
		return resources.ResourceTool, true
	case "mcp":
		return resources.ResourceMCP, true
	case "schema":
		return resources.ResourceSchema, true
	case "model":
		return resources.ResourceModel, true
	case "workflow":
		return resources.ResourceWorkflow, true
	case "memory":
		return resources.ResourceMemory, true
	case "project":
		return resources.ResourceProject, true
	default:
		reqErr := router.NewRequestError(http.StatusBadRequest, "unknown resource type", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return "", false
	}
}

func projectFromQueryOrDefault(c *gin.Context) string {
	p := strings.TrimSpace(c.Query("project"))
	if p != "" {
		return p
	}
	st := router.GetAppState(c)
	if st == nil {
		return ""
	}
	return st.ProjectConfig.Name
}

func setETag(c *gin.Context, etag string) { c.Header("ETag", etag) }

func setLocation(c *gin.Context, typ, id string) {
	c.Header("Location", routes.Base()+"/resources/"+typ+"/"+id)
}
