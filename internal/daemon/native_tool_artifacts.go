package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
)

type nativeToolArtifactReadInput struct {
	ArtifactURI string `json:"artifact_uri"`
	Offset      int64  `json:"offset,omitempty"`
	Limit       int64  `json:"limit,omitempty"`
}

func (n *daemonNativeTools) toolArtifactBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDToolArtifactRead: {
			call:         n.toolArtifactRead,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) toolArtifactRead(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeToolArtifactReadInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if strings.TrimSpace(scope.WorkspaceID) == "" {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			req.ToolID,
			"tool artifact read requires a workspace scope",
			toolspkg.ErrToolDenied,
			toolspkg.ReasonScopeMismatch,
		)
	}
	if n == nil || n.deps == nil || n.deps.ToolArtifacts == nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			req.ToolID,
			"tool artifact store is unavailable",
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonDependencyMissing,
		)
	}
	if _, err := toolspkg.ParseToolArtifactURI(input.ArtifactURI); err != nil ||
		input.Offset < 0 || input.Limit < 0 || input.Limit > toolspkg.MaxToolArtifactPageBytes {
		if err == nil {
			err = errors.New("artifact offset or limit is outside supported bounds")
		}
		return toolspkg.ToolResult{}, nativeToolArtifactValidationError(req.ToolID, err)
	}
	page, err := n.deps.ToolArtifacts.ReadPage(
		ctx,
		scope.WorkspaceID,
		input.ArtifactURI,
		input.Offset,
		input.Limit,
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeToolArtifactReadError(req.ToolID, err)
	}
	return structuredResult(
		page,
		fmt.Sprintf("read %d of %d artifact bytes", page.Bytes, page.TotalBytes),
	)
}

func nativeToolArtifactReadError(id toolspkg.ToolID, err error) error {
	switch {
	case errors.Is(err, toolspkg.ErrToolArtifactInvalidRange):
		return nativeToolArtifactValidationError(id, err)
	case errors.Is(err, toolspkg.ErrToolArtifactNotFound):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			id,
			"tool artifact not found",
			errors.Join(toolspkg.ErrToolNotFound, err),
			toolspkg.ReasonToolArtifactNotFound,
		)
	case errors.Is(err, toolspkg.ErrToolArtifactCorrupt):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			id,
			"tool artifact is corrupt",
			errors.Join(toolspkg.ErrToolBackendFailed, err),
			toolspkg.ReasonToolArtifactCorrupt,
		)
	default:
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			id,
			"tool artifact read failed",
			errors.Join(toolspkg.ErrToolBackendFailed, err),
			toolspkg.ReasonBackendUnhealthy,
		)
	}
}

func nativeToolArtifactValidationError(id toolspkg.ToolID, err error) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		"invalid tool artifact read",
		errors.Join(toolspkg.ErrToolInvalidInput, err),
		toolspkg.ReasonSchemaInvalid,
	)
}
