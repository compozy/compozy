package core

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/profiling"
	"github.com/gin-gonic/gin"
)

const daemonProfileInvalidCode = "daemon_profile_invalid"

// CaptureDaemonProfile is experimental and registered exclusively by the UDS transport.
func (h *BaseHandlers) CaptureDaemonProfile(c *gin.Context) {
	seconds := profiling.DefaultSeconds
	if raw, ok := c.GetQuery("seconds"); ok {
		value, err := strconv.Atoi(raw)
		if err != nil || !profiling.Windowed(c.Param("kind")) {
			c.JSON(
				http.StatusBadRequest,
				contract.ErrorPayload{
					Code:  daemonProfileInvalidCode,
					Error: "seconds requires an integer from 1 to 120 and a cpu, mutex or block profile",
				},
			)
			return
		}
		seconds = value
	}
	var data bytes.Buffer
	if err := profiling.Capture(c.Request.Context(), &data, c.Param("kind"), seconds); err != nil {
		status, code := http.StatusInternalServerError, "profile_failed"
		if errors.Is(err, profiling.ErrInvalid) {
			status, code = http.StatusBadRequest, daemonProfileInvalidCode
		}
		if errors.Is(err, profiling.ErrBusy) {
			status, code = http.StatusConflict, "profile_busy"
		}
		c.JSON(status, contract.ErrorPayload{Code: code, Error: err.Error()})
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+c.Param("kind")+`.pprof"`)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/octet-stream", data.Bytes())
}
