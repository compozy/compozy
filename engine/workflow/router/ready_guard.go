package wfrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/gin-gonic/gin"
)

func ensureWorkerReady(c *gin.Context) (*appstate.State, bool) {
	state := router.GetAppState(c)
	if state == nil {
		if !c.Writer.Written() {
			reqErr := router.NewRequestError(
				http.StatusServiceUnavailable,
				"application state not initialized",
				nil,
			)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
		}
		return nil, false
	}
	if state.Worker == nil {
		reqErr := router.NewRequestError(
			http.StatusServiceUnavailable,
			"worker is not running; configure Redis or start the worker",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	return state, true
}
