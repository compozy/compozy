package core

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

const terminalCatalogWorkspaceIDKey = "workspace_id"

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
	workspaceID := strings.TrimSpace(c.Param(terminalCatalogWorkspaceIDKey))
	replay, reset, fence, changed := h.terminalCatalog.read(workspaceID, profileID, after)
	stop, done, accepting := h.terminalStreams.begin()
	if !accepting {
		h.respondTerminalError(c, fmt.Errorf("terminal streams are shutting down: %w", terminalpkg.ErrShuttingDown))
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
		after = fence
	} else {
		for _, event := range replay {
			if err := writeTerminalCatalogEvent(writer, event); err != nil {
				return
			}
			after = event.Sequence
		}
	}
	after, changed, err = h.writeTerminalCatalogRecordingRecovery(
		c, writer, service, workspaceID, profileID, after, changed, replay,
	)
	if err != nil {
		h.logSSEWriteFailure("terminal.recording_started", err)
		return
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
				recoveredAfter, recoveredChanged, recoveryErr := h.writeTerminalCatalogRecordingRecovery(
					c, writer, service, workspaceID, profileID, after, changed, nil,
				)
				if recoveryErr != nil {
					h.logSSEWriteFailure("terminal.recording_started", recoveryErr)
					return
				}
				after, changed = recoveredAfter, recoveredChanged
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
		return 0, fmt.Errorf("terminal catalog cursor is invalid: %w", terminalpkg.ErrUnsupported)
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

func (h *BaseHandlers) writeTerminalCatalogRecordingRecovery(
	c *gin.Context,
	writer FlushWriter,
	service terminalpkg.Manager,
	workspaceID, profileID string,
	after uint64,
	changed <-chan struct{},
	alreadyWritten []terminalCatalogEvent,
) (uint64, <-chan struct{}, error) {
	writtenRecordings := terminalCatalogRecordingEventIDs(alreadyWritten)
	for {
		recordings, err := service.ActiveRecordings(
			c.Request.Context(), workspaceID, store.ReadScope{ProfileID: profileID},
		)
		if err != nil {
			return after, changed, fmt.Errorf("terminal catalog: list active recordings: %w", err)
		}
		catchUp, reset, fence, nextChanged := h.terminalCatalog.read(workspaceID, profileID, after)
		changed = nextChanged
		if reset && after == 0 && fence == 0 {
			reset = false
		}
		if reset {
			if err := h.writeTerminalCatalogSnapshot(c, writer, service, workspaceID, profileID, fence); err != nil {
				return after, changed, err
			}
			after = fence
			writtenRecordings = make(map[string]struct{})
			continue
		}
		for _, event := range catchUp {
			if err := writeTerminalCatalogEvent(writer, event); err != nil {
				return after, changed, err
			}
			after = event.Sequence
			addTerminalCatalogRecordingEventID(writtenRecordings, event)
		}
		for _, recording := range recordings {
			if _, written := writtenRecordings[recording.ID]; written {
				continue
			}
			if err := WriteSSE(writer, SSEMessage{
				ID: strconv.FormatUint(after, 10), Name: "terminal.recording_started",
				Data: terminalCatalogRecordingPayload(
					workspaceID, recording.ProfileID, recording.TerminalID, recording.ID, recording.StartedAt,
				),
			}); err != nil {
				return after, changed, fmt.Errorf("terminal catalog: write active recording %q: %w", recording.ID, err)
			}
		}
		return after, changed, nil
	}
}

func terminalCatalogRecordingEventIDs(events []terminalCatalogEvent) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, event := range events {
		addTerminalCatalogRecordingEventID(ids, event)
	}
	return ids
}

func addTerminalCatalogRecordingEventID(ids map[string]struct{}, event terminalCatalogEvent) {
	if event.Event.Kind != terminalpkg.EventKindRecordingStarted &&
		event.Event.Kind != terminalpkg.EventKindRecordingStopped {
		return
	}
	if id := event.Event.DetailValue().RecordingID; id != "" {
		ids[id] = struct{}{}
	}
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
	case terminalpkg.EventKindModeChanged:
		return "terminal.mode_changed", gin.H{terminalIDPayloadKey: event.TerminalID, "mode": event.Detail.Mode}, nil
	case terminalpkg.EventKindRecordingStarted, terminalpkg.EventKindRecordingStopped:
		detail := event.DetailValue()
		if detail.RecordingID == "" {
			return "", nil, fmt.Errorf("terminal catalog: %s event has no recording id", event.Kind)
		}
		return "terminal." + string(event.Kind), terminalCatalogRecordingPayload(
			event.WorkspaceID, event.ProfileID, event.TerminalID, detail.RecordingID, event.At,
		), nil
	default:
		return "", nil, fmt.Errorf("terminal catalog: unsupported event %q", event.Kind)
	}
}

func terminalCatalogRecordingPayload(
	workspaceID string,
	profileID string,
	terminalID terminalpkg.ID,
	recordingID string,
	at time.Time,
) gin.H {
	return gin.H{
		terminalCatalogWorkspaceIDKey: workspaceID,
		"profile_id":                  profileID,
		terminalIDPayloadKey:          terminalID,
		"recording_id":                recordingID,
		"at":                          at,
	}
}
