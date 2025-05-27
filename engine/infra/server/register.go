package server

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	wfrouter "github.com/compozy/compozy/engine/workflow/router"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, state *appstate.State) error {
	version := core.GetVersion()
	prefixURL := fmt.Sprintf("/api/%s", version)
	apiBase := router.Group(prefixURL)
	wfrouter.Register(apiBase)

	logger.Info("Completed route registration",
		"total_workflows", len(state.Workflows),
	)
	return nil
}
