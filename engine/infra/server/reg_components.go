package server

import (
	agentrouter "github.com/compozy/compozy/engine/agent/router"
	"github.com/compozy/compozy/engine/memory"
	memrouter "github.com/compozy/compozy/engine/memory/router"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	toolrouter "github.com/compozy/compozy/engine/tool/router"
	wfrouter "github.com/compozy/compozy/engine/workflow/router"
	schedulerouter "github.com/compozy/compozy/engine/workflow/schedule/router"
	"github.com/gin-gonic/gin"
)

func setupComponentRoutes(apiBase *gin.RouterGroup, healthService *memory.HealthService) {
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
