package server

import (
	agentrouter "github.com/compozy/compozy/engine/agent/router"
	knowledgerouter "github.com/compozy/compozy/engine/infra/server/router/knowledge"
	mcprouter "github.com/compozy/compozy/engine/mcp/router"
	memory "github.com/compozy/compozy/engine/memory"
	memrouter "github.com/compozy/compozy/engine/memory/router"
	memoryrouter "github.com/compozy/compozy/engine/memoryconfig/router"
	modelrouter "github.com/compozy/compozy/engine/model/router"
	projectrouter "github.com/compozy/compozy/engine/project/router"
	schemarouter "github.com/compozy/compozy/engine/schema/router"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	toolrouter "github.com/compozy/compozy/engine/tool/router"
	wfrouter "github.com/compozy/compozy/engine/workflow/router"
	schedulerouter "github.com/compozy/compozy/engine/workflow/schedule/router"
	"github.com/gin-gonic/gin"
)

func setupComponentRoutes(apiBase *gin.RouterGroup, healthService *memory.HealthService) {
	projectrouter.Register(apiBase)
	schemarouter.Register(apiBase)
	modelrouter.Register(apiBase)
	mcprouter.Register(apiBase)
	memoryrouter.Register(apiBase)
	knowledgerouter.Register(apiBase)
	wfrouter.Register(apiBase)
	tkrouter.Register(apiBase)
	agentrouter.Register(apiBase)
	toolrouter.Register(apiBase)
	schedulerouter.Register(apiBase)
	memrouter.Register(apiBase)
	if healthService != nil {
		memory.RegisterMemoryHealthRoutes(apiBase, healthService)
	}
}
