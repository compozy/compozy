package core

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

func RegisterMigrationNoticeRoutes(api gin.IRouter, handlers *BaseHandlers, guards ...gin.HandlerFunc) {
	routes := api.Group("/migration-notices", guards...)
	routes.GET("", handlers.ListMigrationNotices)
	routes.POST("/:key/dismiss", handlers.DismissMigrationNotice)
}

func (h *BaseHandlers) ListMigrationNotices(c *gin.Context) {
	response := contract.MigrationNoticesResponse{Notices: []contract.MigrationNoticePayload{}}
	if h.MigrationNotices != nil {
		notices, err := h.MigrationNotices.ListMigrationNotices(c.Request.Context())
		if err != nil {
			h.respondError(c, http.StatusInternalServerError, err)
			return
		}
		response.Notices = make([]contract.MigrationNoticePayload, 0, len(notices))
		for _, notice := range notices {
			response.Notices = append(response.Notices, contract.MigrationNoticePayload{
				Key: notice.Key, CreatedAt: notice.CreatedAt,
				Payload: contract.GatewayRemovalNoticePayload(notice.Payload),
			})
		}
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) DismissMigrationNotice(c *gin.Context) {
	if h.MigrationNotices != nil {
		if err := h.MigrationNotices.DismissMigrationNotice(c.Request.Context(), c.Param("key"), h.Now()); err != nil {
			h.respondError(c, http.StatusInternalServerError, err)
			return
		}
	}
	c.Status(http.StatusNoContent)
}
