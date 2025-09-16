package wfrouter

import (
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/gin-gonic/gin"
)

func ensureWorkerReady(c *gin.Context) (*appstate.State, bool) {
	state := router.GetAppStateWithWorker(c)
	if state == nil {
		return nil, false
	}
	return state, true
}
