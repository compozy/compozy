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
	stream := &terminalAggregateStream{
		h: h, c: c, writer: writer, breadth: breadth, scope: scope, fingerprint: fingerprint, after: after,
	}
	if err := stream.write(replay, reset, fence); err != nil {
		h.logSSEWriteFailure("terminal.aggregate", err)
		return
	}
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-changed:
			replay, reset, fence, changed = h.terminalCatalog.readSelection(stream.after, matches)
			if err := stream.write(replay, reset, fence); err != nil {
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

// terminalAggregateStream writes one aggregate terminal SSE stream and tracks its replay position.
type terminalAggregateStream struct {
	h           *BaseHandlers
	c           *gin.Context
	writer      FlushWriter
	breadth     WorkspaceBreadth
	scope       store.ReadScope
	fingerprint string
	after       uint64
}

func (s *terminalAggregateStream) id(sequence uint64) string {
	return strconv.FormatUint(sequence, 10) + "." + s.fingerprint
}

// write sends a fresh snapshot after a reset, otherwise the replayed catalog events in order.
func (s *terminalAggregateStream) write(replay []terminalCatalogEvent, reset bool, fence uint64) error {
	if reset {
		items, err := s.h.aggregateTerminalItems(s.c, s.breadth, s.scope)
		if err != nil {
			return err
		}
		if err := WriteSSE(s.writer, SSEMessage{
			ID: s.id(fence), Name: "terminal.snapshot", Data: gin.H{terminalsKey: items, "reset": true},
		}); err != nil {
			return err
		}
		s.after = fence
		return nil
	}
	for _, event := range replay {
		name, payload, err := terminalCatalogPayload(event.Event)
		if err != nil {
			return err
		}
		if err := WriteSSE(s.writer, SSEMessage{ID: s.id(event.Sequence), Name: name, Data: payload}); err != nil {
			return err
		}
		s.after = event.Sequence
	}
	return nil
}

func terminalAggregateCursor(value, fingerprint string) (uint64, error) {
	raw, previous, found := strings.Cut(value, ".")
	if !found || previous != fingerprint {
		return 0, nil
	}
	return terminalCatalogCursor(raw)
}
