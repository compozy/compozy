package core

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/version"
	"github.com/gin-gonic/gin"
)

// GetRuntimeIdentity returns the bounded daemon identity shared by HTTP and UDS.
func (h *BaseHandlers) GetRuntimeIdentity(c *gin.Context) {
	httpPort := h.HTTPPortValue()
	if httpPort <= 0 {
		httpPort = h.Config.HTTP.Port
	}
	c.JSON(http.StatusOK, contract.RuntimeIdentityPayload{
		SchemaVersion: contract.StatusSchemaVersion,
		Daemon: contract.DaemonIdentityPayload{
			PID:         h.PID(),
			StartedAt:   h.StartedAt,
			HTTPHost:    strings.TrimSpace(h.Config.HTTP.Host),
			HTTPPort:    httpPort,
			UserHomeDir: h.daemonUserHomeDir(),
			Version:     strings.TrimSpace(version.Current().Version),
		},
	})
}
