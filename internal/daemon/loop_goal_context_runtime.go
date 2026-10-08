package daemon

import (
	"context"
	"errors"
	"slices"

	"github.com/compozy/compozy/internal/acp"
	looppkg "github.com/compozy/compozy/internal/loop"
	goalpkg "github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type loopGoalContextRuntime struct {
	sessions loopSessionEventReader
}

var _ goalpkg.ContextHealth = (*loopGoalContextRuntime)(nil)

func (r *loopGoalContextRuntime) Usage(
	ctx context.Context,
	binding looppkg.ActionSessionBinding,
) (goalpkg.ContextUsage, error) {
	if r == nil || r.sessions == nil {
		return goalpkg.ContextUsage{}, errors.New("daemon: Goal context event reader is unavailable")
	}
	events, err := r.sessions.Events(ctx, binding.SessionID, store.EventQuery{})
	if err != nil {
		return goalpkg.ContextUsage{}, err
	}
	for _, event := range slices.Backward(events) {
		if event.Type == acp.EventTypeCompaction {
			snapshot, err := transcript.UnmarshalAgentEvent(event.Content)
			if err != nil {
				return goalpkg.ContextUsage{}, err
			}
			if snapshot.Compaction != nil && snapshot.Compaction.Terminal {
				break
			}
		}
		usage, found, decodeErr := goalContextUsageFromEvent(event)
		if decodeErr != nil {
			return goalpkg.ContextUsage{}, decodeErr
		}
		if found {
			return usage, nil
		}
	}
	return goalpkg.ContextUsage{}, nil
}

// UsageAtSequence accepts a pinned observation only while it remains newer than the compaction boundary.
func (r *loopGoalContextRuntime) UsageAtSequence(
	ctx context.Context,
	binding looppkg.ActionSessionBinding,
	sequence int64,
) (goalpkg.ContextUsage, error) {
	if r == nil || r.sessions == nil {
		return goalpkg.ContextUsage{}, errors.New("daemon: Goal context event reader is unavailable")
	}
	if sequence < 1 {
		return goalpkg.ContextUsage{}, errors.New("daemon: Goal context usage sequence must be positive")
	}
	events, err := r.sessions.Events(ctx, binding.SessionID, store.EventQuery{})
	if err != nil {
		return goalpkg.ContextUsage{}, err
	}
	for _, event := range events {
		if event.Type != acp.EventTypeCompaction || event.Sequence < sequence {
			continue
		}
		snapshot, err := transcript.UnmarshalAgentEvent(event.Content)
		if err != nil {
			return goalpkg.ContextUsage{}, err
		}
		if snapshot.Compaction != nil && snapshot.Compaction.Terminal {
			return goalpkg.ContextUsage{}, nil
		}
	}
	for _, event := range events {
		if event.Sequence != sequence {
			continue
		}
		usage, found, decodeErr := goalContextUsageFromEvent(event)
		if decodeErr != nil {
			return goalpkg.ContextUsage{}, decodeErr
		}
		if found {
			return usage, nil
		}
	}
	return goalpkg.ContextUsage{}, nil
}

func goalContextUsageFromEvent(event store.SessionEvent) (goalpkg.ContextUsage, bool, error) {
	agentEvent, err := transcript.UnmarshalAgentEvent(event.Content)
	if err != nil {
		return goalpkg.ContextUsage{}, false, err
	}
	if agentEvent.Usage == nil || agentEvent.Usage.ContextUsed == nil || agentEvent.Usage.ContextSize == nil {
		return goalpkg.ContextUsage{}, false, nil
	}
	if *agentEvent.Usage.ContextUsed < 0 || *agentEvent.Usage.ContextSize <= 0 {
		return goalpkg.ContextUsage{}, false, errors.New("daemon: Goal context usage is outside the valid range")
	}
	return goalpkg.ContextUsage{
		Used:       *agentEvent.Usage.ContextUsed,
		Size:       *agentEvent.Usage.ContextSize,
		Sequence:   event.Sequence,
		Known:      true,
		ReportedAt: event.Timestamp.UTC(),
	}, true, nil
}

func (r *loopGoalContextRuntime) CompactionCommand(
	ctx context.Context,
	binding looppkg.ActionSessionBinding,
) (string, bool, error) {
	if r == nil || r.sessions == nil {
		return "", false, errors.New("daemon: Goal command event reader is unavailable")
	}
	events, err := r.sessions.Events(ctx, binding.SessionID, store.EventQuery{})
	if err != nil {
		return "", false, err
	}
	for _, event := range slices.Backward(events) {
		agentEvent, err := transcript.UnmarshalAgentEvent(event.Content)
		if err != nil {
			return "", false, err
		}
		if agentEvent.Title != acp.SystemEventTitleAvailableCommandsUpdate {
			continue
		}
		command, ok := session.ResolveCompactionCommand(agentEvent.AvailableCommandSet().Values())
		return command, ok, nil
	}
	return "", false, nil
}
