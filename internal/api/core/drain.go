package core

import (
	"errors"
	"net/http"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/gin-gonic/gin"
)

var errDrainControllerUnavailable = errors.New("api: daemon drain controller is unavailable")

// DrainDaemon closes new-work admission without interrupting admitted work.
func (h *BaseHandlers) DrainDaemon(c *gin.Context) {
	h.setDaemonDrainState(c, true)
}

// UndrainDaemon restores new-work admission.
func (h *BaseHandlers) UndrainDaemon(c *gin.Context) {
	h.setDaemonDrainState(c, false)
}

func (h *BaseHandlers) setDaemonDrainState(c *gin.Context, draining bool) {
	if h == nil || h.DrainController == nil {
		RespondError(c, http.StatusServiceUnavailable, errDrainControllerUnavailable, false)
		return
	}
	var err error
	if draining {
		err = h.DrainController.Drain(c.Request.Context())
	} else {
		err = h.DrainController.Undrain(c.Request.Context())
	}
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, contract.DrainStatusResponse{State: h.DrainController.DrainState()})
}
