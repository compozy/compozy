package session

import (
	"time"

	"github.com/compozy/compozy/internal/acp"
)

const (
	promptChunkCoalesceInterval = 25 * time.Millisecond
	promptChunkCoalesceMaxBytes = 4096
	promptEventBatchMaxEvents   = 64
)

type promptChunkCoalescer struct {
	pending *promptChunkPending
}

type promptChunkPending struct {
	events     []acp.AgentEvent
	totalBytes int
}

func (c *promptChunkCoalescer) hasPending() bool {
	return c != nil && c.pending != nil
}

func (c *promptChunkCoalescer) append(event acp.AgentEvent, runtimeEvent bool) bool {
	if c == nil || runtimeEvent || isPromptTerminalEvent(event.Type) || isPromptAttentionEvent(event.Type) {
		return false
	}
	if c.pending == nil {
		c.pending = &promptChunkPending{
			events:     []acp.AgentEvent{event},
			totalBytes: len(event.Text),
		}
		return true
	}
	c.pending.events = append(c.pending.events, event)
	c.pending.totalBytes += len(event.Text)
	return true
}

func isPromptAttentionEvent(eventType string) bool {
	switch eventType {
	case acp.EventTypePermission, acp.EventTypeClarify:
		return true
	default:
		return false
	}
}

func (c *promptChunkCoalescer) take() ([]acp.AgentEvent, bool) {
	if c == nil || c.pending == nil {
		return nil, false
	}
	pending := c.pending
	c.pending = nil
	return pending.events, true
}

func (c *promptChunkCoalescer) shouldFlush() bool {
	if c == nil || c.pending == nil {
		return false
	}
	return len(c.pending.events) >= promptEventBatchMaxEvents ||
		c.pending.totalBytes >= promptChunkCoalesceMaxBytes
}
