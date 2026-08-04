package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/task"
)

// NodeRequeueMutation requests one quarantined node to re-enter normal succession.
type NodeRequeueMutation struct {
	WorkspaceID      WorkspaceID
	RunID            RunID
	NodeID           NodeID
	Reason           string
	ExpectedRevision *int64
	Actor            task.ActorContext
	RequestedAt      time.Time
}

// Normalize trims requeue identities before validation and persistence lookup.
func (m NodeRequeueMutation) Normalize() NodeRequeueMutation {
	m.WorkspaceID = WorkspaceID(strings.TrimSpace(string(m.WorkspaceID)))
	m.RunID = RunID(strings.TrimSpace(string(m.RunID)))
	m.NodeID = NodeID(strings.TrimSpace(string(m.NodeID)))
	return m
}

// NodeRequeueResult returns the cleared control and committed coordinator wake.
type NodeRequeueResult struct {
	Control     NodeControl
	Coordinator task.Run
}

// NodeRequeueStore owns the atomic quarantine-clear and coordinator reservation.
type NodeRequeueStore interface {
	RequeueNode(context.Context, NodeRequeueMutation) (NodeRequeueResult, error)
}

// AppendQuarantineRequeue retains episode history and appends sanitized requeue provenance.
func AppendQuarantineRequeue(
	raw json.RawMessage,
	actor task.ActorContext,
	reason string,
	at time.Time,
	generation int,
) (json.RawMessage, error) {
	entry, err := decodeQuarantineEntry(raw)
	if err != nil {
		return nil, err
	}
	if entry.NodeID == "" || len(entry.Episodes) == 0 || generation < 1 || at.IsZero() {
		return nil, fmt.Errorf("%w: quarantine entry cannot be requeued", ErrValidation)
	}
	entry.Requeues = append(entry.Requeues, requeueProvenance(actor, reason, at, generation))
	return encodeQuarantineEntry(entry)
}

// ValidateNodeRequeueMutation rejects incomplete or stale node requeue requests.
func ValidateNodeRequeueMutation(m NodeRequeueMutation) error {
	if strings.TrimSpace(string(m.WorkspaceID)) == "" || strings.TrimSpace(string(m.RunID)) == "" ||
		strings.TrimSpace(string(m.NodeID)) == "" || m.RequestedAt.IsZero() {
		return fmt.Errorf("%w: node requeue identity is incomplete", ErrValidation)
	}
	if err := m.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: requeue actor: %w", ErrValidation, err)
	}
	if m.ExpectedRevision != nil && *m.ExpectedRevision < 0 {
		return fmt.Errorf("%w: expected requeue revision must be non-negative", ErrValidation)
	}
	return nil
}

func pendingRequeueNodes(controls []NodeControl, generation int) ([]NodeID, error) {
	nodeIDs := make([]NodeID, 0)
	for _, control := range controls {
		if control.Quarantined || len(control.QuarantineEntry) == 0 {
			continue
		}
		entry, err := decodeQuarantineEntry(control.QuarantineEntry)
		if err != nil {
			return nil, err
		}
		if len(entry.Requeues) == 0 {
			continue
		}
		latest := entry.Requeues[len(entry.Requeues)-1]
		if latest.Generation == generation {
			nodeIDs = append(nodeIDs, control.NodeID)
		}
	}
	return nodeIDs, nil
}
