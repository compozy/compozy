package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) StreamTerminalCatalog(c *gin.Context) {
	if h == nil || h.terminalCatalog == nil {
		if h != nil {
			h.respondTerminalUnavailable(c)
		}
		return
	}
	service, profileID, ok := h.terminalService(c, false)
	if !ok {
		return
	}
	after, err := terminalCatalogCursor(c.GetHeader("Last-Event-ID"))
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	replay, reset, fence, changed := h.terminalCatalog.read(workspaceID, profileID, after)
	stop, done, accepting := h.terminalStreams.begin()
	if !accepting {
		h.respondTerminalError(
			c,
			&terminalpkg.Error{
				Code:    "terminal_shutting_down",
				Message: "terminal streams are shutting down",
				Err:     terminalpkg.ErrShuttingDown,
			},
		)
		return
	}
	defer done()
	writer, err := PrepareSSE(c)
	if err != nil {
		return
	}
	if reset {
		if err := h.writeTerminalCatalogSnapshot(c, writer, service, workspaceID, profileID, fence); err != nil {
			return
		}
	} else {
		for _, event := range replay {
			if err := writeTerminalCatalogEvent(writer, event); err != nil {
				return
			}
			after = event.Sequence
		}
	}
	if reset {
		after = fence
	}
	h.streamTerminalCatalog(c, writer, service, workspaceID, profileID, after, changed, stop)
}

func (h *BaseHandlers) streamTerminalCatalog(
	c *gin.Context,
	writer FlushWriter,
	service terminalpkg.Manager,
	workspaceID, profileID string,
	after uint64,
	changed <-chan struct{},
	stop <-chan struct{},
) {
	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-changed:
			replay, reset, fence, nextChanged := h.terminalCatalog.read(workspaceID, profileID, after)
			changed = nextChanged
			if reset {
				if err := h.writeTerminalCatalogSnapshot(
					c,
					writer,
					service,
					workspaceID,
					profileID,
					fence,
				); err != nil {
					return
				}
				after = fence
				continue
			}
			for _, event := range replay {
				if err := writeTerminalCatalogEvent(writer, event); err != nil {
					return
				}
				after = event.Sequence
			}
		case <-keepAlive.C:
			if err := WriteSSEComment(writer, "keep-alive"); err != nil {
				return
			}
		case <-stop:
			return
		case <-c.Request.Context().Done():
			return
		}
	}
}

func (h *BaseHandlers) writeTerminalCatalogSnapshot(
	c *gin.Context,
	writer FlushWriter,
	service terminalpkg.Manager,
	workspaceID, profileID string,
	fence uint64,
) error {
	items, err := h.terminalListForProfile(c, service, workspaceID, profileID)
	if err != nil {
		h.logSSEWriteFailure("terminal.snapshot", err)
		return err
	}
	return WriteSSE(writer, SSEMessage{
		ID: strconv.FormatUint(fence, 10), Name: "terminal.snapshot", Data: gin.H{"terminals": items},
	})
}

func terminalCatalogCursor(raw string) (uint64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, &terminalpkg.Error{
			Code:    "terminal_cursor_invalid",
			Message: "terminal catalog cursor is invalid",
			Err:     terminalpkg.ErrUnsupported,
		}
	}
	return value, nil
}

func writeTerminalCatalogEvent(writer FlushWriter, event terminalCatalogEvent) error {
	name, payload, err := terminalCatalogPayload(event.Event)
	if err != nil {
		return err
	}
	return WriteSSE(writer, SSEMessage{ID: strconv.FormatUint(event.Sequence, 10), Name: name, Data: payload})
}

func terminalCatalogPayload(event terminalpkg.Event) (string, any, error) {
	switch event.Kind {
	case terminalpkg.EventKindOpened:
		if event.Info == nil {
			return "", nil, errors.New("terminal catalog: opened event has no terminal info")
		}
		return "terminal.created", gin.H{
			terminalPayloadKey: terminalInfoFromDomain(*event.Info, event.ProfileName),
		}, nil
	case terminalpkg.EventKindClosed:
		exit := event.Exit
		if exit == nil && event.Info != nil {
			exit = event.Info.Exit
		}
		return "terminal.closed", gin.H{
			terminalIDPayloadKey: event.TerminalID,
			"exit":               terminalExitFromDomain(exit),
		}, nil
	case terminalpkg.EventKindTitleChanged:
		return "terminal.title_changed", gin.H{terminalIDPayloadKey: event.TerminalID, "title": event.Detail.Title}, nil
	case terminalpkg.EventKindLeaseChanged:
		var controllerKind terminalpkg.ActorKind
		var controllerID string
		if event.Info != nil && event.Info.Controller != nil {
			controllerKind = event.Info.Controller.Kind
			controllerID = event.Info.Controller.ID
		}
		return "terminal.lease_changed", gin.H{
			terminalIDPayloadKey: event.TerminalID, "lease": event.Detail.LeaseTo,
			"controller_kind": controllerKind, "controller_id": controllerID, "reason": event.Reason,
		}, nil
	case terminalpkg.EventKindModeChanged:
		return "terminal.mode_changed", gin.H{terminalIDPayloadKey: event.TerminalID, "mode": event.Detail.Mode}, nil
	default:
		return "", nil, fmt.Errorf("terminal catalog: unsupported event %q", event.Kind)
	}
}
