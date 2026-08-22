package cli

import (
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
)

type loopFollowTimelineBuffer struct {
	pending *looppkg.TimelineEntry
	count   int
}

func (b *loopFollowTimelineBuffer) Push(entry looppkg.TimelineEntry) []looppkg.TimelineEntry {
	if !loopFollowHeartbeatKind(entry.Kind) {
		ready := make([]looppkg.TimelineEntry, 0, 2)
		if pending, ok := b.Flush(); ok {
			ready = append(ready, pending)
		}
		return append(ready, entry)
	}
	if b.pending == nil {
		copyEntry := entry
		b.pending = &copyEntry
		b.count = 1
		return nil
	}
	if b.pending.Kind != entry.Kind {
		previous, _ := b.Flush()
		copyEntry := entry
		b.pending = &copyEntry
		b.count = 1
		return []looppkg.TimelineEntry{previous}
	}
	b.pending.Seq = entry.Seq
	b.pending.At = entry.At
	b.count++
	return nil
}

func (b *loopFollowTimelineBuffer) Flush() (looppkg.TimelineEntry, bool) {
	if b.pending == nil {
		return looppkg.TimelineEntry{}, false
	}
	entry := *b.pending
	if b.count > 1 {
		entry.Title = fmt.Sprintf("%s ×%d", entry.Title, b.count)
	}
	b.pending = nil
	b.count = 0
	return entry, true
}

func loopFollowHeartbeatKind(kind looppkg.RunEventKind) bool {
	switch kind {
	case looppkg.RunEventTokenTick,
		looppkg.RunEventRuntimeApplied,
		looppkg.RunEventPredicateDiagnostic:
		return true
	default:
		return false
	}
}
