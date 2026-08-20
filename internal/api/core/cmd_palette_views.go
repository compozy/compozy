package core

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/gin-gonic/gin"
)

const (
	cmdPaletteViewPatchEvent = "cmd_palette.view.patch"
	cmdPaletteViewResetEvent = "cmd_palette.view.reset"
)

type cmdPaletteViewStream struct {
	h           *BaseHandlers
	service     cmdpalette.ViewSourceService
	writer      FlushWriter
	workspaceID cmdpalette.WorkspaceID
	viewID      string
	cursor      int64
	revision    string
	epoch       string
}

func (h *BaseHandlers) GetCmdPaletteView(c *gin.Context) {
	workspaceID, ok := h.resolveCmdPaletteWorkspace(c, c.Query("workspace"))
	if !ok {
		return
	}
	service, ok := h.cmdPaletteViewService(c, workspaceID)
	if !ok {
		return
	}
	snapshot, err := service.OpenSource(c.Request.Context(), workspaceID, c.Param("id"))
	if err != nil {
		h.respondCmdPaletteViewError(c, workspaceID, err)
		return
	}
	c.JSON(http.StatusOK, contract.CmdPaletteViewFromDomain(snapshot))
}

func (h *BaseHandlers) StreamCmdPaletteView(c *gin.Context) {
	workspaceID, ok := h.resolveCmdPaletteWorkspace(c, c.Query("workspace"))
	if !ok {
		return
	}
	service, ok := h.cmdPaletteViewService(c, workspaceID)
	if !ok {
		return
	}
	after, err := parseCmdPaletteViewSequence(c.Query("after"))
	if err != nil {
		h.respondCmdPaletteViewError(c, workspaceID, err)
		return
	}
	requestedEpoch := strings.TrimSpace(c.Query("stream_epoch"))
	viewID := c.Param("id")
	snapshot, events, cancel, err := service.SubscribeViewPatches(
		c.Request.Context(),
		cmdpalette.ViewPatchSubscribeRequest{
			Workspace: workspaceID, ViewID: viewID, After: after, StreamEpoch: requestedEpoch,
		},
	)
	if err != nil {
		h.respondCmdPaletteViewError(c, workspaceID, err)
		return
	}
	defer cancel()
	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondCmdPaletteViewError(c, workspaceID, err)
		return
	}
	if err := WriteSSEComment(writer, "command palette view stream ready"); err != nil {
		h.logSSEWriteFailure(cmdPaletteViewPatchEvent, err)
		return
	}
	stream := &cmdPaletteViewStream{
		h: h, service: service, writer: writer, workspaceID: workspaceID, viewID: viewID,
		cursor: after, revision: snapshot.Revision, epoch: snapshot.StreamEpoch,
	}
	if requestedEpoch != "" && requestedEpoch != stream.epoch {
		if err := writeCmdPaletteViewReset(writer, stream.cursor, snapshot); err != nil {
			h.logSSEWriteFailure(cmdPaletteViewResetEvent, err)
			return
		}
		stream.cursor = 0
	}
	stream.run(c.Request.Context(), events)
}

func (s *cmdPaletteViewStream) run(ctx context.Context, events <-chan cmdpalette.ViewPatchEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.h.StreamDoneChannel():
			return
		case event, open := <-events:
			if !open {
				return
			}
			if !s.writeEvent(ctx, event) {
				return
			}
		}
	}
}

func (s *cmdPaletteViewStream) writeEvent(ctx context.Context, event cmdpalette.ViewPatchEvent) bool {
	if event.Sequence <= s.cursor {
		return true
	}
	if event.StreamEpoch != s.epoch || event.Patch.From != s.revision {
		return s.resync(ctx, event.Sequence)
	}
	if err := cmdpalette.ValidateViewPatch(event.Patch); err != nil {
		s.h.writeSSEBestEffort(s.writer, SSEMessage{Name: handlersErrorKey, Data: ErrorPayloadForError(err)})
		return false
	}
	payload := contract.CmdPaletteViewPatch{
		Sequence: event.Sequence, StreamEpoch: s.epoch,
		Patch: &event.Patch, Revision: event.Patch.To,
	}
	if err := WriteSSE(s.writer, SSEMessage{Name: cmdPaletteViewPatchEvent, Data: payload}); err != nil {
		s.h.logSSEWriteFailure(cmdPaletteViewPatchEvent, err)
		return false
	}
	s.cursor = event.Sequence
	s.revision = event.Patch.To
	return true
}

func (s *cmdPaletteViewStream) resync(ctx context.Context, sequence int64) bool {
	snapshot, err := s.service.OpenSource(ctx, s.workspaceID, s.viewID)
	if err != nil {
		s.h.writeSSEBestEffort(s.writer, SSEMessage{Name: handlersErrorKey, Data: ErrorPayloadForError(err)})
		return false
	}
	if err := writeCmdPaletteViewReset(s.writer, sequence, snapshot); err != nil {
		s.h.logSSEWriteFailure(cmdPaletteViewResetEvent, err)
		return false
	}
	s.cursor = sequence
	s.revision = snapshot.Revision
	s.epoch = snapshot.StreamEpoch
	return true
}

func (h *BaseHandlers) cmdPaletteViewService(
	c *gin.Context,
	workspaceID cmdpalette.WorkspaceID,
) (cmdpalette.ViewSourceService, bool) {
	service, ok := h.CmdPalette.(cmdpalette.ViewSourceService)
	if !ok {
		h.respondCmdPaletteViewError(c, workspaceID, errors.New("cmd palette view service is unavailable"))
		return nil, false
	}
	return service, true
}

func (h *BaseHandlers) respondCmdPaletteViewError(
	c *gin.Context,
	_ cmdpalette.WorkspaceID,
	err error,
) {
	c.JSON(cmdPaletteViewStatus(err), contract.CmdPaletteError{Error: "view_error", Message: err.Error()})
}

func parseCmdPaletteViewSequence(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	sequence, err := strconv.ParseInt(value, 10, 64)
	if err != nil || sequence < 0 {
		return 0, cmdpalette.ErrViewInvalidSequence
	}
	return sequence, nil
}

func writeCmdPaletteViewReset(
	writer FlushWriter,
	sequence int64,
	snapshot cmdpalette.ViewSnapshot,
) error {
	payload := snapshot.Payload
	return WriteSSE(writer, SSEMessage{
		Name: cmdPaletteViewResetEvent,
		Data: contract.CmdPaletteViewPatch{
			Sequence: sequence, StreamEpoch: snapshot.StreamEpoch,
			Payload: &payload, Revision: snapshot.Revision, Reset: true,
		},
	})
}
