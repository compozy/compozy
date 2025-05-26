package server

import (
	"github.com/compozy/compozy/engine/server/appstate"
	wfrouter "github.com/compozy/compozy/engine/workflow/router"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, state *appstate.State) error {
	apiBase := router.Group("/api")
	wfrouter.Register(apiBase)

	logger.Info("Completed route registration",
		"total_workflows", len(state.Workflows),
	)
	return nil
}
