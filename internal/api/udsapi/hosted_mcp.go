package udsapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	core "github.com/compozy/agh/internal/api/core"
	mcppkg "github.com/compozy/agh/internal/mcp"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/gin-gonic/gin"
)

var errHostedMCPBackend = errors.New("hosted-mcp backend error")

func registerHostedMCPRoutes(api gin.IRouter, handlers *Handlers) {
	hosted := api.Group("/internal/hosted-mcp")
	{
		hosted.POST("/bind", handlers.bindHostedMCP)
		hosted.GET("/projection", handlers.hostedMCPProjection)
		hosted.GET("/projection/stream", handlers.streamHostedMCPProjection)
		hosted.POST("/tools/call", handlers.callHostedMCP)
		hosted.POST("/release", handlers.releaseHostedMCP)
	}
}

func (h *Handlers) bindHostedMCP(c *gin.Context) {
	if h == nil || h.HostedMCP == nil {
		core.RespondError(c, http.StatusServiceUnavailable, mcppkg.ErrHostedDisabled, false)
		return
	}
	var req mcppkg.HostedBindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, http.StatusBadRequest, err, false)
		return
	}
	peer, err := mcppkg.PeerInfoFromContext(c.Request.Context())
	if err != nil {
		core.RespondError(c, http.StatusForbidden, err, false)
		return
	}
	response, err := h.HostedMCP.Bind(c.Request.Context(), req, peer)
	if err != nil {
		respondHostedMCPError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handlers) hostedMCPProjection(c *gin.Context) {
	if h == nil || h.HostedMCP == nil {
		core.RespondError(c, http.StatusServiceUnavailable, mcppkg.ErrHostedDisabled, false)
		return
	}
	peer, err := mcppkg.PeerInfoFromContext(c.Request.Context())
	if err != nil {
		core.RespondError(c, http.StatusForbidden, err, false)
		return
	}
	response, err := h.HostedMCP.Projection(c.Request.Context(), c.Query("bind_id"), peer)
	if err != nil {
		respondHostedMCPError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handlers) streamHostedMCPProjection(c *gin.Context) {
	if h == nil || h.HostedMCP == nil {
		core.RespondError(c, http.StatusServiceUnavailable, mcppkg.ErrHostedDisabled, false)
		return
	}
	peer, err := mcppkg.PeerInfoFromContext(c.Request.Context())
	if err != nil {
		core.RespondError(c, http.StatusForbidden, err, false)
		return
	}
	bindID := strings.TrimSpace(c.Query("bind_id"))
	lastDigest := strings.TrimSpace(c.Query("last_digest"))
	response, err := h.HostedMCP.Projection(c.Request.Context(), bindID, peer)
	if err != nil {
		respondHostedMCPError(c, err)
		return
	}
	writer, err := core.PrepareSSE(c)
	if err != nil {
		core.RespondError(c, http.StatusInternalServerError, err, false)
		return
	}
	interval := h.PollInterval
	if interval <= 0 {
		interval = defaultPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if response.Digest != "" && response.Digest != lastDigest {
			if writeErr := core.WriteSSE(writer, core.SSEMessage{
				ID:   response.Digest,
				Name: "projection",
				Data: response,
			}); writeErr != nil {
				if h.Logger != nil {
					h.Logger.Warn("udsapi: failed to emit hosted MCP projection", "error", writeErr)
				}
				return
			}
			lastDigest = response.Digest
		}
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case <-ticker.C:
		}
		response, err = h.HostedMCP.Projection(c.Request.Context(), bindID, peer)
		if err != nil {
			status := hostedMCPStatus(err)
			if h.Logger != nil {
				h.Logger.Warn("udsapi: hosted MCP projection failed", "status", status)
			}
			if writeErr := core.WriteSSE(writer, core.SSEMessage{
				Name: "error",
				Data: hostedMCPStreamErrorData(err),
			}); writeErr != nil && h.Logger != nil {
				h.Logger.Warn("udsapi: failed to emit hosted MCP error", "error", writeErr)
			}
			return
		}
	}
}

type hostedMCPStreamErrorPayload struct {
	Error   string `json:"error"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// hostedMCPStreamErrorData keeps stream failures stable without exposing backend error text.
func hostedMCPStreamErrorData(err error) hostedMCPStreamErrorPayload {
	status := hostedMCPStatus(err)
	return hostedMCPStreamErrorPayload{
		Error:   "hosted_mcp_projection_failed",
		Status:  status,
		Message: http.StatusText(status),
	}
}

func (h *Handlers) callHostedMCP(c *gin.Context) {
	if h == nil || h.HostedMCP == nil {
		core.RespondError(c, http.StatusServiceUnavailable, mcppkg.ErrHostedDisabled, false)
		return
	}
	var req mcppkg.HostedCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, http.StatusBadRequest, err, false)
		return
	}
	peer, err := mcppkg.PeerInfoFromContext(c.Request.Context())
	if err != nil {
		core.RespondError(c, http.StatusForbidden, err, false)
		return
	}
	response, err := h.HostedMCP.Call(c.Request.Context(), req, peer)
	if err != nil {
		respondHostedMCPError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handlers) releaseHostedMCP(c *gin.Context) {
	if h == nil || h.HostedMCP == nil {
		core.RespondError(c, http.StatusServiceUnavailable, mcppkg.ErrHostedDisabled, false)
		return
	}
	var req mcppkg.HostedReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, http.StatusBadRequest, err, false)
		return
	}
	peer, err := mcppkg.PeerInfoFromContext(c.Request.Context())
	if err != nil {
		core.RespondError(c, http.StatusForbidden, err, false)
		return
	}
	if err := h.HostedMCP.ReleaseBindForPeer(c.Request.Context(), req.BindID, peer); err != nil {
		respondHostedMCPError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func respondHostedMCPError(c *gin.Context, err error) {
	if isHostedMCPToolError(err) {
		core.RespondToolError(c, err, false)
		return
	}
	core.RespondError(c, hostedMCPStatus(err), hostedMCPJSONError(err), false)
}

func hostedMCPSafeError() error {
	return errHostedMCPBackend
}

func hostedMCPJSONError(err error) error {
	if isHostedMCPToolError(err) {
		return errors.New("hosted_mcp_tool_error: hosted tool request failed")
	}
	if hostedMCPStatus(err) >= http.StatusInternalServerError {
		return hostedMCPSafeError()
	}
	switch {
	case errors.Is(err, mcppkg.ErrHostedSessionRequired):
		return errors.New("hosted_mcp_session_required: session_id is required")
	case errors.Is(err, mcppkg.ErrHostedNonceRequired):
		return errors.New("hosted_mcp_nonce_required: bind_nonce is required")
	case errors.Is(err, mcppkg.ErrHostedBindRequired):
		return errors.New("hosted_mcp_bind_required: bind_id is required")
	case errors.Is(err, mcppkg.ErrHostedNonceInvalid):
		return errors.New("hosted_mcp_nonce_invalid: bind_nonce is invalid")
	case errors.Is(err, mcppkg.ErrHostedNonceExpired):
		return errors.New("hosted_mcp_nonce_expired: bind_nonce expired")
	case errors.Is(err, mcppkg.ErrHostedBindNotFound):
		return errors.New("hosted_mcp_bind_not_found: bind_id was not found or expired")
	case errors.Is(err, mcppkg.ErrHostedPeerInvalid):
		return errors.New("hosted_mcp_peer_invalid: peer validation failed")
	case errors.Is(err, mcppkg.ErrHostedBinaryInvalid):
		return errors.New("hosted_mcp_binary_invalid: peer executable validation failed")
	default:
		return errors.New("hosted_mcp_request_invalid: hosted MCP request is invalid")
	}
}

func hostedMCPStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case isHostedMCPToolError(err):
		return core.StatusForToolError(err)
	case errors.Is(err, mcppkg.ErrHostedDisabled), errors.Is(err, mcppkg.ErrHostedRegistryRequired):
		return http.StatusServiceUnavailable
	case errors.Is(err, mcppkg.ErrHostedSessionRequired),
		errors.Is(err, mcppkg.ErrHostedNonceRequired),
		errors.Is(err, mcppkg.ErrHostedBindRequired):
		return http.StatusBadRequest
	case errors.Is(err, mcppkg.ErrHostedNonceInvalid),
		errors.Is(err, mcppkg.ErrHostedNonceExpired),
		errors.Is(err, mcppkg.ErrHostedPeerInvalid),
		errors.Is(err, mcppkg.ErrHostedBinaryInvalid),
		errors.Is(err, mcppkg.ErrHostedBindNotFound):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func isHostedMCPToolError(err error) bool {
	var toolErr *toolspkg.ToolError
	var validationErr *toolspkg.ValidationError
	return errors.As(err, &toolErr) ||
		errors.As(err, &validationErr) ||
		errors.Is(err, toolspkg.ErrToolInvalidInput) ||
		errors.Is(err, toolspkg.ErrToolNotFound) ||
		errors.Is(err, toolspkg.ErrToolDenied) ||
		errors.Is(err, toolspkg.ErrToolApprovalRequired) ||
		errors.Is(err, toolspkg.ErrToolConflict) ||
		errors.Is(err, toolspkg.ErrToolUnavailable) ||
		errors.Is(err, toolspkg.ErrToolResultTooLarge) ||
		errors.Is(err, toolspkg.ErrToolBackendFailed) ||
		errors.Is(err, toolspkg.ErrToolCanceled) ||
		errors.Is(err, toolspkg.ErrToolTimedOut)
}
