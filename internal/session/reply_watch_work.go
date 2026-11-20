package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const replyWaitUninspected = "remaining work could not be inspected"

func (s *replyWatchService) initialReplyWait(ctx context.Context, w store.ReplyWatch, after int64) string {
	waiting, err := s.replyContinuationWork(ctx, w)
	if waiting == "" && err == nil {
		waiting, err = s.manager.replyWakeAfter(ctx, w, after)
	}
	if err != nil {
		s.logWatchError(ctx, w, err)
		return replyWaitUninspected
	}
	return waiting
}

func (s *replyWatchService) replyContinuationWork(ctx context.Context, w store.ReplyWatch) (string, error) {
	var waiting []string
	add := func(label string) {
		if label != "" && !slices.Contains(waiting, label) {
			waiting = append(waiting, label)
		}
	}
	for _, kind := range []WorkSignalKind{WorkSignalActiveMonitor, WorkSignalActiveChild, WorkSignalScheduledWait} {
		signals, err := s.manager.workSignals.inspectKind(ctx, w.TargetSessionID, kind, s.manager.now())
		if err != nil {
			return "", err
		}
		for _, signal := range signals {
			if kind != WorkSignalActiveChild || s.replyChildWillWake(ctx, signal.Ref) {
				add(replyWaitLabel(kind))
				break
			}
		}
	}
	replies, err := s.store.ListPendingReplyWatchesBySender(ctx, w.TargetSessionID)
	if err != nil {
		return "", err
	}
	now := s.manager.now()
	for _, reply := range replies {
		// Waiting on the sender cannot be the reason to hold the sender's answer.
		if reply.TargetSessionID != w.SenderSessionID && !s.replyWatchExpired(reply, now) {
			add("replies from other sessions")
			break
		}
	}
	if err := s.manager.inspectAdmittedReplyWakes(ctx, w, add); err != nil {
		return "", err
	}
	return strings.Join(waiting, ", "), nil
}

func (s *replyWatchService) replyChildWillWake(ctx context.Context, id string) bool {
	child, live := s.manager.Get(id)
	if !live {
		// Starting children and the subagent store are owned by the signal source.
		return true
	}
	info := child.Info()
	if info.Lineage == nil || info.Lineage.SpawnRole == store.SubagentSpawnRole {
		return true
	}
	if !info.Lineage.NotifyCreator {
		return false
	}
	if info.State != StateActive || info.Prompting {
		return true
	}
	wakeKinds := []WorkSignalKind{
		WorkSignalActiveMonitor, WorkSignalScheduledWait, WorkSignalAwaitingReply, WorkSignalActiveChild,
	}
	work := s.manager.workSignals.Inspect(ctx, id, s.manager.now())
	if slices.ContainsFunc(work.WorkSignals, func(signal WorkSignal) bool {
		return slices.Contains(wakeKinds, signal.Kind)
	}) || slices.ContainsFunc(work.Sources, func(source SignalSourceState) bool {
		return source.State == signalSourceUnknown && slices.Contains(wakeKinds, source.Kind)
	}) {
		return true
	}
	queue, err := s.manager.InputQueueSummary(ctx, id)
	return err != nil || queue.PendingInputs > 0
}

func replyWaitLabel(kind WorkSignalKind) string {
	switch kind {
	case WorkSignalActiveMonitor:
		return "monitors"
	case WorkSignalActiveChild:
		return "subagents or child sessions"
	default:
		return "scheduled waits"
	}
}

// Inspect the queue before the running turn so a wake crossing the dispatch boundary
// is visible from at least one of its two durable sources.
func (m *Manager) inspectAdmittedReplyWakes(ctx context.Context, w store.ReplyWatch, add func(string)) error {
	if m.inputQueueStore != nil {
		inputs, err := m.inputQueueStore.ListPendingSessionInputs(ctx, w.TargetSessionID)
		if err != nil {
			return fmt.Errorf("session: read pending inputs for reply wakes: %w", err)
		}
		for i := range inputs {
			entry := &inputs[i]
			if entry.OwnerKind != store.SessionInputOwnerSynthetic || entry.SyntheticPrompt == nil {
				continue
			}
			meta, err := syntheticPromptMeta(entry.SyntheticPrompt)
			if err != nil {
				return err
			}
			add(replyWakeLabel(meta, w.SenderSessionID))
		}
	}
	run, err := m.ActivePromptRun(ctx, w.TargetSessionID)
	if errors.Is(err, ErrPromptNotActive) {
		return nil
	}
	if err != nil {
		return err
	}
	events, err := m.Events(ctx, w.TargetSessionID, store.EventQuery{
		TurnID: run.TurnID, Type: acp.EventTypeSyntheticReentry,
	})
	if err != nil {
		return err
	}
	for _, stored := range events {
		event, err := transcript.UnmarshalAgentEvent(stored.Content)
		if err != nil {
			return fmt.Errorf("session: decode active turn reply wake: %w", err)
		}
		add(replyWakeLabel(event.Synthetic, w.SenderSessionID))
	}
	return nil
}

// A dispatched wake is recorded before its queue entry is marked sent.
func (m *Manager) replyWakeAfter(ctx context.Context, w store.ReplyWatch, after int64) (string, error) {
	query := store.EventQuery{
		Type: acp.EventTypeSyntheticReentry, TopLevelOnly: true, Forward: true, AfterSequence: after, Limit: 200,
	}
	for {
		events, err := m.Events(ctx, w.TargetSessionID, query)
		if err != nil {
			return "", fmt.Errorf("session: read reply wakes after the consuming turn: %w", err)
		}
		for _, stored := range events {
			event, err := transcript.UnmarshalAgentEvent(stored.Content)
			if err != nil {
				return "", fmt.Errorf("session: decode reply wake after the consuming turn: %w", err)
			}
			if label := replyWakeLabel(event.Synthetic, w.SenderSessionID); label != "" {
				return label, nil
			}
		}
		if len(events) < query.Limit {
			return "", nil
		}
		query.AfterSequence = events[len(events)-1].Sequence
	}
}

func replyWakeLabel(meta *acp.PromptSyntheticMeta, sender string) string {
	if meta == nil {
		return ""
	}
	switch meta.Kind {
	case acp.PromptSyntheticKindMonitor:
		return "monitor wakes"
	case SpawnWakeKindChildSettled, subagentWakeKind:
		return "subagent or child completion wakes"
	case acp.PromptSyntheticKindSessionReply:
		if meta.ChildSessionID != "" && meta.ChildSessionID != sender {
			return "replies from other sessions"
		}
	case "":
		// Plain creator notifications and scheduler task wakes predate discriminator kinds.
		if meta.ChildSessionID != "" {
			return "subagent or child completion wakes"
		}
		if meta.TaskRunID != "" && meta.Goal == nil {
			return "scheduled wakes"
		}
	}
	return ""
}
