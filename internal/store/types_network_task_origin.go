package store

import (
	"strings"
	"time"
)

type NetworkTaskThreadOrigin struct {
	TaskID           string
	WorkspaceID      string
	Channel          string
	ThreadID         string
	OriginMessageID  string
	Digest           string
	SourceMessageIDs []string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// NetworkTaskThreadOriginQuery filters promoted task origin links.
type NetworkTaskThreadOriginQuery struct {
	TaskID      string
	WorkspaceID string
	Channel     string
	ThreadID    string
	Limit       int
}

// Validate ensures origin reads are either task-specific or thread-specific.
func (q NetworkTaskThreadOriginQuery) Validate() error {
	if strings.TrimSpace(q.TaskID) != "" {
		return requirePositiveLimit(q.Limit, "network task thread origin limit")
	}
	if err := (NetworkConversationRef{
		WorkspaceID: q.WorkspaceID,
		Channel:     q.Channel,
		Surface:     NetworkSurfaceThread,
		ThreadID:    q.ThreadID,
	}).Validate(); err != nil {
		return err
	}
	return requirePositiveLimit(q.Limit, "network task thread origin limit")
}

// Validate ensures the origin is thread-scoped and compact.
func (o NetworkTaskThreadOrigin) Validate() error {
	if err := requireField(o.TaskID, "network task thread origin task_id"); err != nil {
		return err
	}
	if err := (NetworkConversationRef{
		WorkspaceID: o.WorkspaceID,
		Channel:     o.Channel,
		Surface:     NetworkSurfaceThread,
		ThreadID:    o.ThreadID,
	}).Validate(); err != nil {
		return err
	}
	if err := requireField(o.OriginMessageID, "network task thread origin origin_message_id"); err != nil {
		return err
	}
	if err := requireField(o.Digest, "network task thread origin digest"); err != nil {
		return err
	}
	for _, messageID := range o.SourceMessageIDs {
		if err := requireField(messageID, "network task thread origin source_message_id"); err != nil {
			return err
		}
	}
	return nil
}
