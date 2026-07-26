package session

import (
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
)

const (
	promptChunkCoalesceInterval = 25 * time.Millisecond
	promptChunkCoalesceMaxBytes = 4096
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
	if c == nil || !isCoalesciblePromptChunk(event, runtimeEvent) {
		return false
	}
	if c.pending == nil {
		c.pending = &promptChunkPending{
			events:     []acp.AgentEvent{event},
			totalBytes: len(event.Text),
		}
		return true
	}
	if !samePromptChunkRun(c.pending.events[len(c.pending.events)-1], event) {
		return false
	}
	c.pending.events = append(c.pending.events, event)
	c.pending.totalBytes += len(event.Text)
	return true
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
	return c.pending.totalBytes >= promptChunkCoalesceMaxBytes
}

func isCoalesciblePromptChunk(event acp.AgentEvent, runtimeEvent bool) bool {
	if runtimeEvent || strings.TrimSpace(event.Text) == "" {
		return false
	}
	switch strings.TrimSpace(event.Type) {
	case acp.EventTypeAgentMessage, acp.EventTypeThought:
	default:
		return false
	}
	return event.Usage == nil &&
		event.Failure == nil &&
		event.Runtime == nil &&
		strings.TrimSpace(event.ToolCallID) == "" &&
		strings.TrimSpace(event.Action) == "" &&
		strings.TrimSpace(event.Resource) == "" &&
		strings.TrimSpace(event.Decision) == "" &&
		event.Synthetic == nil
}

func samePromptChunkRun(left acp.AgentEvent, right acp.AgentEvent) bool {
	return strings.TrimSpace(left.Type) == strings.TrimSpace(right.Type) &&
		strings.TrimSpace(left.SessionID) == strings.TrimSpace(right.SessionID) &&
		strings.TrimSpace(left.TurnID) == strings.TrimSpace(right.TurnID) &&
		strings.TrimSpace(left.RequestID) == strings.TrimSpace(right.RequestID)
}
