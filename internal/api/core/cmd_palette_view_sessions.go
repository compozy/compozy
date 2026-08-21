package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/gin-gonic/gin"
)

const cmdPaletteViewFrameEvent = "cmd_palette.view.frame"

func (h *BaseHandlers) OpenCmdPaletteViewSession(c *gin.Context) {
	var body contract.CmdPaletteViewSessionOpenRequest
	if err := decodeStrictJSONBody(c, &body); err != nil {
		c.JSON(http.StatusBadRequest, contract.CmdPaletteError{
			Error: cmdPaletteInvalidRequestError, Message: err.Error(),
		})
		return
	}
	workspaceID, ok := h.resolveCmdPaletteWorkspace(c, body.Workspace)
	if !ok {
		return
	}
	service, ok := h.cmdPaletteViewSessionService(c, workspaceID)
	if !ok {
		return
	}
	result, err := service.OpenSession(c.Request.Context(), cmdpalette.ViewSessionOpenRequest{
		Workspace:       workspaceID,
		View:            strings.TrimSpace(c.Param("id")),
		Args:            body.Args,
		AttachmentToken: strings.TrimSpace(c.GetHeader(cmdPaletteClientAttachmentHeader)),
	})
	if err != nil {
		h.respondCmdPaletteViewSessionError(c, workspaceID, err)
		return
	}
	c.JSON(http.StatusOK, contract.CmdPaletteViewSessionOpenResponse{
		ViewSession: result.Token.ViewSession,
		StreamToken: result.Token.StreamToken,
		FirstFrame:  result.FirstFrame,
	})
}

func (h *BaseHandlers) StreamCmdPaletteViewSession(c *gin.Context) {
	service, ok := h.cmdPaletteViewSessionService(c, "")
	if !ok {
		return
	}
	replay, frames, cancel, err := service.SubscribeSessionFrames(
		c.Request.Context(),
		cmdpalette.SessionToken{
			ViewSession: strings.TrimSpace(c.Param("session")),
			StreamToken: strings.TrimSpace(c.Query("token")),
		},
	)
	if err != nil {
		h.respondCmdPaletteViewSessionError(c, "", err)
		return
	}
	defer cancel()
	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondCmdPaletteViewSessionError(c, "", err)
		return
	}
	if err := WriteSSEComment(writer, "command palette view session ready"); err != nil {
		h.logSSEWriteFailure(cmdPaletteViewFrameEvent, err)
		return
	}
	if err := WriteSSE(writer, SSEMessage{Name: cmdPaletteViewFrameEvent, Data: replay}); err != nil {
		h.logSSEWriteFailure(cmdPaletteViewFrameEvent, err)
		return
	}
	h.runCmdPaletteViewSessionStream(c.Request.Context(), writer, frames)
}

func (h *BaseHandlers) runCmdPaletteViewSessionStream(
	ctx context.Context,
	writer FlushWriter,
	frames <-chan cmdpalette.ViewFrame,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.StreamDoneChannel():
			return
		case frame, open := <-frames:
			if !open {
				return
			}
			if err := WriteSSE(writer, SSEMessage{Name: cmdPaletteViewFrameEvent, Data: frame}); err != nil {
				h.logSSEWriteFailure(cmdPaletteViewFrameEvent, err)
				return
			}
		}
	}
}

func (h *BaseHandlers) AdmitCmdPaletteViewSessionEvent(c *gin.Context) {
	var body contract.CmdPaletteViewSessionEventRequest
	if err := decodeStrictJSONBody(c, &body); err != nil {
		c.JSON(http.StatusBadRequest, contract.CmdPaletteError{
			Error: cmdPaletteInvalidRequestError, Message: err.Error(),
		})
		return
	}
	service, ok := h.cmdPaletteViewSessionService(c, "")
	if !ok {
		return
	}
	sessionID := strings.TrimSpace(c.Param("session"))
	err := service.AdmitEvent(c.Request.Context(), cmdpalette.SessionToken{
		ViewSession:     sessionID,
		AttachmentToken: strings.TrimSpace(c.GetHeader(cmdPaletteClientAttachmentHeader)),
	}, cmdpalette.ViewEvent{
		ViewSession: sessionID,
		Handler:     strings.TrimSpace(body.Handler), Args: body.Args,
		Revision: strings.TrimSpace(body.Revision), Seq: body.Seq,
		AckEffects: body.AckEffects, EffectResult: body.EffectResult,
	})
	if err != nil {
		h.respondCmdPaletteViewSessionError(c, "", err)
		return
	}
	c.JSON(http.StatusAccepted, contract.CmdPaletteViewSessionAccepted{Accepted: true})
}

func (h *BaseHandlers) CloseCmdPaletteViewSession(c *gin.Context) {
	service, ok := h.cmdPaletteViewSessionService(c, "")
	if !ok {
		return
	}
	err := service.CloseSession(c.Request.Context(), cmdpalette.SessionToken{
		ViewSession:     strings.TrimSpace(c.Param("session")),
		AttachmentToken: strings.TrimSpace(c.GetHeader(cmdPaletteClientAttachmentHeader)),
	}, "palette_dismissed")
	if err != nil {
		h.respondCmdPaletteViewSessionError(c, "", err)
		return
	}
	c.JSON(http.StatusOK, contract.CmdPaletteViewSessionClosed{Closed: true})
}

func (h *BaseHandlers) respondCmdPaletteViewSessionError(
	c *gin.Context,
	workspaceID cmdpalette.WorkspaceID,
	err error,
) {
	status, code := cmdPaletteViewSessionStatus(err)
	if status >= http.StatusInternalServerError && h.Logger != nil {
		h.Logger.Error("command palette view session request failed", "workspace_id", workspaceID, "error", err)
	}
	c.JSON(status, contract.CmdPaletteError{Error: code, Message: err.Error()})
}

func (h *BaseHandlers) cmdPaletteViewSessionService(
	c *gin.Context,
	workspaceID cmdpalette.WorkspaceID,
) (cmdpalette.ViewSessionService, bool) {
	service, ok := h.CmdPalette.(cmdpalette.ViewSessionService)
	if !ok {
		h.respondCmdPaletteViewSessionError(
			c,
			workspaceID,
			errors.New("cmd palette view session service is unavailable"),
		)
		return nil, false
	}
	return service, true
}
