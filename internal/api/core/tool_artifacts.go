package core

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/gin-gonic/gin"
)

// ReadToolArtifact returns one retained result page within the resolved workspace.
func (h *BaseHandlers) ReadToolArtifact(c *gin.Context) {
	if h.ToolArtifacts == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("tool artifact store is not configured"))
		return
	}
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("artifact_id"))
	uri := toolspkg.ToolArtifactURIPrefix + id
	if _, err := toolspkg.ParseToolArtifactURI(uri); err != nil {
		h.respondToolError(c, toolArtifactValidationError(err))
		return
	}
	offset, err := parseToolArtifactQueryInt(c, "offset", 0)
	if err != nil {
		h.respondToolError(c, toolArtifactValidationError(err))
		return
	}
	limit, err := parseToolArtifactQueryInt(c, "limit", 0)
	if err != nil {
		h.respondToolError(c, toolArtifactValidationError(err))
		return
	}
	if offset < 0 || limit < 0 || limit > toolspkg.MaxToolArtifactPageBytes {
		h.respondToolError(c, toolArtifactValidationError(errors.New("offset or limit is outside supported bounds")))
		return
	}
	page, err := h.ToolArtifacts.ReadPage(c.Request.Context(), scope.ID, uri, offset, limit)
	if err != nil {
		h.respondToolError(c, toolArtifactReadError(err))
		return
	}
	c.JSON(http.StatusOK, toolArtifactPageResponse(page))
}

func parseToolArtifactQueryInt(c *gin.Context, name string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	return value, nil
}

func toolArtifactValidationError(err error) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		toolspkg.ToolIDToolArtifactRead,
		"invalid tool artifact read",
		errors.Join(toolspkg.ErrToolInvalidInput, err),
		toolspkg.ReasonSchemaInvalid,
	)
}

func toolArtifactReadError(err error) error {
	switch {
	case errors.Is(err, toolspkg.ErrToolArtifactInvalidRange):
		return toolArtifactValidationError(err)
	case errors.Is(err, toolspkg.ErrToolArtifactNotFound):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			toolspkg.ToolIDToolArtifactRead,
			"tool artifact not found",
			errors.Join(toolspkg.ErrToolNotFound, err),
			toolspkg.ReasonToolArtifactNotFound,
		)
	case errors.Is(err, toolspkg.ErrToolArtifactCorrupt):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			toolspkg.ToolIDToolArtifactRead,
			"tool artifact is corrupt",
			errors.Join(toolspkg.ErrToolBackendFailed, err),
			toolspkg.ReasonToolArtifactCorrupt,
		)
	default:
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			toolspkg.ToolIDToolArtifactRead,
			"tool artifact read failed",
			errors.Join(toolspkg.ErrToolBackendFailed, err),
			toolspkg.ReasonBackendUnhealthy,
		)
	}
}

func toolArtifactPageResponse(page toolspkg.ToolArtifactPage) contract.ToolArtifactPageResponse {
	return contract.ToolArtifactPageResponse{
		Artifact:   page.Artifact,
		Offset:     page.Offset,
		Bytes:      page.Bytes,
		TotalBytes: page.TotalBytes,
		DataBase64: page.DataBase64,
		NextOffset: page.NextOffset,
		EOF:        page.EOF,
	}
}
