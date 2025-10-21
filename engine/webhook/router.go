package webhook

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// SuccessResponse represents the unified success response envelope
type SuccessResponse struct {
	Data    any    `json:"data"`
	Message string `json:"message"`
}

// ErrorResponse represents the unified error response envelope
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

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
// @Param slug path string true "Webhook slug" example("my-webhook")
// @Param X-Idempotency-Key header string false "Optional idempotency key to prevent duplicate processing" example("idemp-123")
// @Param X-Correlation-ID header string false "Optional correlation ID for request tracing" example("corr-456")
// @Param X-Sig header string false "Optional HMAC signature header (configurable per webhook)" example("sha256=abc123...")
// @Param Stripe-Signature header string false "Stripe webhook signature (when using stripe verification)" example("t=123,v1=def...")
// @Param X-Hub-Signature-256 header string false "GitHub webhook signature (when using github verification)" example("sha256=ghi789...")
// @Param payload body object true "Arbitrary JSON payload" example({"event":"user.created","data":{"id":123,"name":"John"}})
// @Success 202 {object} SuccessResponse "Accepted and enqueued"
// @Success 200 {object} SuccessResponse "Processed successfully"
// @Failure 400 {object} ErrorResponse "Invalid or oversized payload"
// @Failure 401 {object} ErrorResponse "Signature verification failed"
// @Failure 404 {object} ErrorResponse "Webhook not found"
// @Failure 409 {object} ErrorResponse "Duplicate idempotency key"
// @Failure 422 {object} ErrorResponse "Unprocessable entity"
// @Failure 429 {object} ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @x-example-success {"data":{"result":"ok"},"message":"Success"}
// @x-example-error {"error":"bad_request","details":"invalid JSON payload"}
// @Router /hooks/{slug} [post]
func RegisterPublic(r *gin.RouterGroup, p Processor) {
	r.POST("/:slug", func(c *gin.Context) {
		slug := c.Param("slug")
		res, err := p.Process(c.Request.Context(), slug, c.Request)
		if err != nil {
			statusCode, errorResponse := mapWebhookError(c.Request.Context(), slug, err)
			c.JSON(statusCode, errorResponse)
			return
		}
		status := res.Status
		if status == http.StatusNoContent {
			status = http.StatusOK
		}
		envelope := SuccessResponse{
			Data:    res.Payload,
			Message: "Success",
		}
		c.JSON(status, envelope)
	})
}

// mapWebhookError maps webhook processing errors to appropriate HTTP status codes and error responses
func mapWebhookError(ctx context.Context, slug string, err error) (int, ErrorResponse) {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Details: "webhook not found",
		}
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Details: "signature verification failed",
		}
	case errors.Is(err, ErrDuplicate):
		return http.StatusConflict, ErrorResponse{
			Error:   "duplicate",
			Details: "idempotency key already processed",
		}
	case errors.Is(err, ErrBadRequest):
		return http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Details: "invalid or oversized payload",
		}
	case errors.Is(err, ErrUnprocessableEntity):
		return http.StatusUnprocessableEntity, ErrorResponse{
			Error:   "unprocessable_entity",
			Details: "payload processing failed",
		}
	default:
		logger.FromContext(ctx).Error("webhook processing failed", "error", err, "slug", slug)
		return http.StatusInternalServerError, ErrorResponse{
			Error:   "internal",
			Details: "internal server error",
		}
	}
}
