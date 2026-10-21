package core

import (
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/listcursor"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) StreamTerminalAggregate(c *gin.Context) {
	breadth, ok := h.aggregateBreadth(c)
	if !ok {
		return
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if h.terminalCatalog == nil {
		h.respondTerminalUnavailable(c)
		return
	}
	allowed, err := h.aggregateWorkspaceSet(c, breadth)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	fingerprint, err := listcursor.Fingerprint(struct {
		Breadth WorkspaceBreadth
		Scope   store.ReadScope
	}{breadth, scope})
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	after, err := terminalAggregateCursor(c.GetHeader("Last-Event-ID"), fingerprint)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	matches := func(event terminalpkg.Event) bool {
		return allowed[event.WorkspaceID] && scope.Matches(event.ProfileID)
	}
	replay, reset, fence, changed := h.terminalCatalog.readSelection(after, matches)
	stop, done, accepting := h.terminalStreams.begin()
	if !accepting {
		h.respondTerminalError(c, terminalpkg.ErrShuttingDown)
		return
	}
	defer done()
	writer, err := PrepareSSE(c)
	if err != nil {
		return
	}
	id := func(sequence uint64) string { return strconv.FormatUint(sequence, 10) + "." + fingerprint }
	write := func() error {
		if reset {
			items, readErr := h.aggregateTerminalItems(c, breadth, scope)
			if readErr != nil {
				return readErr
			}
			if writeErr := WriteSSE(writer, SSEMessage{ID: id(fence), Name: "terminal.snapshot", Data: gin.H{"terminals": items, "reset": true}}); writeErr != nil {
				return writeErr
			}
			after = fence
			return nil
		}
		for _, event := range replay {
			name, payload, payloadErr := terminalCatalogPayload(event.Event)
			if payloadErr != nil {
				return payloadErr
			}
			if writeErr := WriteSSE(writer, SSEMessage{ID: id(event.Sequence), Name: name, Data: payload}); writeErr != nil {
				return writeErr
			}
			after = event.Sequence
		}
		return nil
	}
	if err := write(); err != nil {
		h.logSSEWriteFailure("terminal.aggregate", err)
		return
	}
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-changed:
			replay, reset, fence, changed = h.terminalCatalog.readSelection(after, matches)
			if err := write(); err != nil {
				h.logSSEWriteFailure("terminal.aggregate", err)
				return
			}
		case <-ticker.C:
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

func terminalAggregateCursor(value, fingerprint string) (uint64, error) {
	raw, previous, found := strings.Cut(value, ".")
	if !found || previous != fingerprint {
		return 0, nil
	}
	return terminalCatalogCursor(raw)
}
