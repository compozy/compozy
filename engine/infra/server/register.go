package server

import (
	"fmt"

	agentrouter "github.com/compozy/compozy/engine/agent/router"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	toolrouter "github.com/compozy/compozy/engine/tool/router"
	wfrouter "github.com/compozy/compozy/engine/workflow/router"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, state *appstate.State) error {
	version := core.GetVersion()
	prefixURL := fmt.Sprintf("/api/%s", version)
	apiBase := router.Group(prefixURL)

	// Register all component routers
	wfrouter.Register(apiBase)
	tkrouter.Register(apiBase)
	agentrouter.Register(apiBase)
	toolrouter.Register(apiBase)

	logger.Info("Completed route registration",
		"total_workflows", len(state.Workflows),
	)
	return nil
}
