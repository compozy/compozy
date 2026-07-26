package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
)

const (
	compactionReasonPressure  = "context_pressure"
	compactionStrategySummary = "summary_archive"
)

type sessionCompactionState struct {
	inFlight      bool
	cancel        context.CancelFunc
	done          chan struct{}
	attemptTurnID string
	attempts      int
	cooldownUntil time.Time
}

type compactionFiredPayload struct {
	WorkspaceID  string  `json:"workspace_id"`
	SessionID    string  `json:"session_id"`
	TurnID       string  `json:"turn_id"`
	FromSequence int64   `json:"from_sequence"`
	ToSequence   int64   `json:"to_sequence"`
	ContextUsed  int64   `json:"context_used"`
	ContextSize  int64   `json:"context_size"`
	Pressure     float64 `json:"pressure"`
	Strategy     string  `json:"strategy"`
}

// SetCompactionHandler late-binds the daemon memory runtime after manager construction.
func (m *Manager) SetCompactionHandler(handler CompactionHandler) {
	if m == nil {
		return
	}
	m.compactionMu.Lock()
	m.compactionHandler = handler
	m.compactionMu.Unlock()
}

func (m *Manager) scheduleCompactionFromUsage(session *Session, event acp.AgentEvent) {
	if event.Usage == nil {
		return
	}
	if err := m.maybeCompact(session, *event.Usage); err != nil {
		m.sessionLogger(session).Warn(
			"session: schedule context compaction failed",
			"turn_id",
			event.TurnID,
			"error",
			err,
		)
	}
}

func (m *Manager) maybeCompact(session *Session, usage acp.TokenUsage) error {
	if m == nil || session == nil || !m.compactionPressureReached(usage) {
		return nil
	}
	turnID := strings.TrimSpace(usage.TurnID)
	if turnID == "" {
		return errors.New("session: compaction usage turn id is required")
	}

	m.compactionMu.Lock()
	defer m.compactionMu.Unlock()
	if m.compactionClosing || m.compactionHandler == nil {
		return nil
	}
	state := m.compactions[session.ID]
	if state == nil {
		state = &sessionCompactionState{}
		m.compactions[session.ID] = state
	}
	if state.inFlight || m.now().Before(state.cooldownUntil) {
		return nil
	}
	if state.attemptTurnID != turnID {
		state.attemptTurnID = turnID
		state.attempts = 0
	}
	if state.attempts >= m.compaction.MaxAttemptsPerTurn {
		return nil
	}

	workCtx, cancel := context.WithCancel(m.lifecycleCtx)
	state.inFlight = true
	state.cancel = cancel
	state.done = make(chan struct{})
	state.attempts++
	used, size := *usage.ContextUsed, *usage.ContextSize
	request := compactionWorkRequest{
		session: session,
		usage: acp.TokenUsage{
			TurnID:      turnID,
			ContextUsed: &used,
			ContextSize: &size,
		},
		handler: m.compactionHandler,
	}
	m.compactionWG.Add(1)
	go m.runPressureCompaction(workCtx, request)
	return nil
}

func (m *Manager) compactionPressureReached(usage acp.TokenUsage) bool {
	if !m.compaction.Enabled || m.compaction.PressureThreshold <= 0 ||
		usage.ContextUsed == nil || usage.ContextSize == nil || *usage.ContextSize <= 0 || *usage.ContextUsed < 0 {
		return false
	}
	return float64(*usage.ContextUsed)/float64(*usage.ContextSize) >= m.compaction.PressureThreshold
}

type compactionWorkRequest struct {
	session *Session
	usage   acp.TokenUsage
	handler CompactionHandler
}

func (m *Manager) runPressureCompaction(ctx context.Context, work compactionWorkRequest) {
	defer m.finishPressureCompaction(work.session.ID)
	if err := m.compactPersistedReplaySpan(ctx, work); err != nil {
		m.armCompactionCooldown(work.session.ID)
		m.sessionLogger(work.session).Warn(
			"session: pressure context compaction failed",
			"turn_id",
			work.usage.TurnID,
			"error",
			err,
		)
	}
}

func (m *Manager) compactPersistedReplaySpan(ctx context.Context, work compactionWorkRequest) error {
	span, err := queryCompleteCompactionSpan(ctx, work.session, work.usage.TurnID)
	if err != nil || len(span) == 0 {
		return err
	}
	info := work.session.Info()
	if info == nil {
		return errors.New("session: compaction session info is unavailable")
	}
	fromSequence, toSequence := span[0].Sequence, span[len(span)-1].Sequence
	pressure := float64(*work.usage.ContextUsed) / float64(*work.usage.ContextSize)
	if err := m.recordCompactionFired(ctx, work.session, compactionFiredPayload{
		WorkspaceID:  info.WorkspaceID,
		SessionID:    info.ID,
		TurnID:       strings.TrimSpace(work.usage.TurnID),
		FromSequence: fromSequence,
		ToSequence:   toSequence,
		ContextUsed:  *work.usage.ContextUsed,
		ContextSize:  *work.usage.ContextSize,
		Pressure:     pressure,
		Strategy:     compactionStrategySummary,
	}); err != nil {
		return err
	}

	_, err = m.runContextCompaction(
		ctx,
		work.session,
		work.usage.TurnID,
		compactionReasonPressure,
		compactionStrategySummary,
		"",
		compactionContextBlocks(fromSequence, toSequence, pressure),
		func(compactCtx context.Context, pre hookspkg.ContextPreCompactPayload) (hookspkg.ContextPostCompactPayload, error) {
			result, compactErr := work.handler.CompactSessionContext(compactCtx, CompactionRequest{
				WorkspaceID:  info.WorkspaceID,
				SessionID:    info.ID,
				AgentName:    info.AgentName,
				FromSequence: fromSequence,
				ToSequence:   toSequence,
				Events:       append([]store.SessionEvent(nil), span...),
			})
			if compactErr != nil {
				return hookspkg.ContextPostCompactPayload{}, fmt.Errorf(
					"session: update compaction coverage: %w",
					compactErr,
				)
			}
			if archiveErr := archiveCompactionSpan(
				compactCtx,
				work.session,
				fromSequence,
				toSequence,
				len(span),
			); archiveErr != nil {
				return hookspkg.ContextPostCompactPayload{}, archiveErr
			}
			return hookspkg.ContextPostCompactPayload{
				Summary:       result.Summary,
				ContextBlocks: pre.ContextBlocks,
			}, nil
		},
	)
	return err
}

func queryCompleteCompactionSpan(
	ctx context.Context,
	session *Session,
	triggerTurnID string,
) ([]store.SessionEvent, error) {
	recorder := session.recorderHandle()
	if recorder == nil {
		return nil, errors.New("session: compaction event recorder is unavailable")
	}
	events, err := recorder.Query(ctx, store.EventQuery{Archive: store.EventArchiveUnarchived})
	if err != nil {
		return nil, fmt.Errorf("session: query compaction replay span: %w", err)
	}
	return completePriorTurnPrefix(events, strings.TrimSpace(triggerTurnID)), nil
}

func completePriorTurnPrefix(events []store.SessionEvent, triggerTurnID string) []store.SessionEvent {
	end := 0
	for start := 0; start < len(events); {
		turnID := strings.TrimSpace(events[start].TurnID)
		if turnID == "" || turnID == triggerTurnID {
			break
		}
		next := start
		terminal := false
		for next < len(events) && strings.TrimSpace(events[next].TurnID) == turnID {
			terminal = terminal || events[next].Type == acp.EventTypeDone || events[next].Type == acp.EventTypeError
			next++
		}
		if !terminal && compactionGroupRequiresTerminal(events[start:next]) {
			break
		}
		end = next
		start = next
	}
	return append([]store.SessionEvent(nil), events[:end]...)
}

func compactionGroupRequiresTerminal(group []store.SessionEvent) bool {
	for _, event := range group {
		switch event.Type {
		case events.TranscriptMarkerCreated, acp.EventTypeClarify:
			continue
		default:
			return true
		}
	}
	return false
}

func archiveCompactionSpan(
	ctx context.Context,
	session *Session,
	fromSequence int64,
	toSequence int64,
	want int,
) error {
	archiver, ok := session.recorderHandle().(store.EventArchiver)
	if !ok {
		return errors.New("session: event recorder does not support compaction archives")
	}
	result, err := archiver.ArchiveEvents(ctx, store.EventArchiveRequest{
		FromSequence: fromSequence,
		ToSequence:   toSequence,
	})
	if err != nil {
		return fmt.Errorf("session: archive compacted replay span: %w", err)
	}
	if result.ArchivedCount != int64(want) {
		return fmt.Errorf(
			"session: archived %d compacted events, want %d for sequence range %d..%d",
			result.ArchivedCount,
			want,
			fromSequence,
			toSequence,
		)
	}
	return nil
}

func (m *Manager) recordCompactionFired(
	ctx context.Context,
	session *Session,
	payload compactionFiredPayload,
) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("session: marshal compaction fired payload: %w", err)
	}
	event := m.normalizeEvent(session, payload.TurnID, acp.AgentEvent{
		Type:      events.SessionCompactionFired,
		Raw:       raw,
		Timestamp: m.now(),
	})
	if err := m.recordEvent(ctx, session, event); err != nil {
		return fmt.Errorf("session: record compaction fired event: %w", err)
	}
	m.notifyAgentEvent(ctx, session, event)
	return nil
}

func compactionContextBlocks(fromSequence int64, toSequence int64, pressure float64) []hookspkg.ContextBlock {
	return []hookspkg.ContextBlock{{
		Kind: "persisted_transcript_span",
		Metadata: map[string]string{
			"from_sequence": strconv.FormatInt(fromSequence, 10),
			"to_sequence":   strconv.FormatInt(toSequence, 10),
			"pressure":      strconv.FormatFloat(pressure, 'f', 6, 64),
		},
	}}
}
