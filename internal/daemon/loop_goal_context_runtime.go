package daemon

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/acp"
	looppkg "github.com/compozy/agh/internal/loop"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
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
	events, err := r.sessions.Events(ctx, binding.SessionID, store.EventQuery{
		AfterSequence:  sequence - 1,
		BeforeSequence: sequence + 1,
	})
	if err != nil {
		return goalpkg.ContextUsage{}, err
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

func (r *loopGoalContextRuntime) HasAdvertisedCommand(
	ctx context.Context,
	binding looppkg.ActionSessionBinding,
	command string,
) (bool, error) {
	if r == nil || r.sessions == nil {
		return false, errors.New("daemon: Goal command event reader is unavailable")
	}
	target := strings.TrimSpace(command)
	if target == "" {
		return false, errors.New("daemon: advertised command is required")
	}
	events, err := r.sessions.Events(ctx, binding.SessionID, store.EventQuery{})
	if err != nil {
		return false, err
	}
	for _, event := range slices.Backward(events) {
		agentEvent, decodeErr := transcript.UnmarshalAgentEvent(event.Content)
		if decodeErr != nil {
			return false, decodeErr
		}
		if agentEvent.Title != acp.SystemEventTitleAvailableCommandsUpdate {
			continue
		}
		for _, advertised := range agentEvent.AvailableCommands.Values() {
			if strings.TrimSpace(advertised.Name) == target {
				return true, nil
			}
		}
		return false, nil
	}
	return false, nil
}
