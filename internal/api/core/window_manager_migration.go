package core

import (
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

// GetWindowManagerMigration reads the profile-owned migration record.
func (h *BaseHandlers) GetWindowManagerMigration(c *gin.Context) { h.windowManagerMigration(c, false) }

// UpdateWindowManagerMigration acknowledges or retries a preserved layout.
func (h *BaseHandlers) UpdateWindowManagerMigration(c *gin.Context) {
	h.windowManagerMigration(c, true)
}

func (h *BaseHandlers) windowManagerMigration(c *gin.Context, mutation bool) {
	workspace := windowManagerWorkspace(c)
	service, ok := h.windowManagerService(c, workspace)
	if !ok {
		return
	}
	var request contract.WindowManagerMigrationRequest
	if mutation {
		if err := decodeWindowManagerJSON(c, &request); err != nil {
			h.respondWindowManagerError(
				c,
				workspace,
				fmt.Errorf("migration request: %w: %v", windowmanager.ErrInvalidCommand, err),
			)
			return
		}
		if request.Action != "ack" && request.Action != "retry" {
			h.respondWindowManagerError(c, workspace, windowmanager.ErrInvalidCommand)
			return
		}
	}
	status, err := service.Migration(c.Request.Context(), workspace, request.Action, request.Replace)
	if err != nil {
		h.respondWindowManagerError(c, workspace, err)
		return
	}
	c.JSON(http.StatusOK, status)
}
