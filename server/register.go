package server

import (
	wfroute "github.com/compozy/compozy/engine/router/workflow"
	"github.com/compozy/compozy/pkg/app"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, state *app.State) error {
	apiBase := router.Group("/api")
	wfroute.Register(apiBase)
	logger.Info("Completed route registration",
		"total_workflows", len(state.Workflows),
	)
	return nil
}
