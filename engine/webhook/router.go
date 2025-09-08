package webhook

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// Processor defines the minimal interface required by the HTTP router.
// It is implemented by Orchestrator.
type Processor interface {
	Process(ctx context.Context, slug string, r *http.Request) (Result, error)
}

// RegisterPublic registers public webhook endpoints under the provided router group.
// Path: POST /:slug
//
// @Summary Trigger webhook
// @Description Accepts webhook payloads and triggers matching workflow events.
// @Tags webhooks
// @Accept json
// @Produce json
// @Param slug path string true "Webhook slug"
// @Success 202 {object} map[string]any "Accepted and enqueued"
// @Success 200 {object} map[string]any "Processed with no matching event"
// @Failure 400 {object} map[string]any "Invalid or oversized payload"
// @Failure 401 {object} map[string]any "Signature verification failed"
// @Failure 404 {object} map[string]any "Webhook not found"
// @Failure 429 {object} map[string]any "Rate limit exceeded"
// @Failure 500 {object} map[string]any "Internal server error"
// @Router /hooks/{slug} [post]
func RegisterPublic(r *gin.RouterGroup, p Processor) {
	r.POST("/:slug", func(c *gin.Context) {
		slug := c.Param("slug")
		res, err := p.Process(c.Request.Context(), slug, c.Request)
		if err != nil {
			switch err {
			case ErrNotFound:
				c.JSON(res.Status, gin.H{"error": "not_found"})
			case ErrUnauthorized:
				c.JSON(res.Status, gin.H{"error": "unauthorized"})
			case ErrDuplicate:
				c.JSON(res.Status, gin.H{"error": "duplicate"})
			case ErrBadRequest:
				c.JSON(res.Status, gin.H{"error": "bad_request"})
			case ErrUnprocessableEntity:
				c.JSON(res.Status, gin.H{"error": "unprocessable_entity"})
			default:
				logger.FromContext(c.Request.Context()).Error("webhook processing failed", "error", err, "slug", slug)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			}
			return
		}
		if res.Payload == nil {
			c.Status(res.Status)
			return
		}
		if res.Status == http.StatusNoContent {
			// Return 200 with explicit payload to avoid proxies dropping bodies on 204
			c.JSON(http.StatusOK, res.Payload)
			return
		}
		c.JSON(res.Status, res.Payload)
	})
}
