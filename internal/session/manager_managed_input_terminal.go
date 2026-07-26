package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

const (
	managedInputOutcomeCompleted     = "completed"
	managedInputOutcomeInvalidResult = "invalid-result"
	managedInputOutcomeFailed        = "failed"
	managedInputReasonStopInvalid    = "goal_stop_reason_invalid"
	managedInputReasonPromptFailed   = "goal_prompt_request_failed"
)

func (m *Manager) finishManagedInputPrompt(
	ctx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
) {
	if m == nil || session == nil || turnState == nil || turnState.managed == nil {
		return
	}
	execution := turnState.managed
	defer m.releaseManagedInputLease(execution.submission.Owner)
	terminal, err := m.managedInputTerminal(ctx, session, execution.submission)
	if err != nil {
		m.recordManagedInputAmbiguous(
			ctx,
			execution.lifecycle,
			execution.submission,
			managedInputReasonRecoveryAmbiguous,
			err,
		)
		m.sessionLogger(session).Error("session: managed input terminal proof failed", "error", err)
		return
	}
	if err := execution.lifecycle.RecordTerminal(ctx, terminal); err != nil {
		m.recordManagedInputAmbiguous(
			ctx,
			execution.lifecycle,
			execution.submission,
			managedInputReasonRecoveryAmbiguous,
			err,
		)
		m.sessionLogger(session).Error(
			"session: persist managed input terminal failed",
			"error", err,
			"queue_entry_id", execution.submission.Owner.QueueEntryID,
		)
	}
}

func (m *Manager) managedInputTerminal(
	ctx context.Context,
	session *Session,
	submission ManagedInputSubmission,
) (ManagedInputTerminal, error) {
	recorder := session.recorderHandle()
	if recorder == nil {
		return ManagedInputTerminal{}, errors.New("session: managed input event recorder is unavailable")
	}
	events, err := recorder.Query(ctx, store.EventQuery{TurnID: submission.Owner.PromptID})
	if err != nil {
		return ManagedInputTerminal{}, fmt.Errorf("session: query managed input events: %w", err)
	}
	if len(events) == 0 {
		return ManagedInputTerminal{}, errors.New("session: managed input has no persisted events")
	}
	accumulator := newManagedInputTerminalAccumulator(submission, m.now().UTC())
	for _, event := range events {
		if err := accumulator.addEvent(event); err != nil {
			return ManagedInputTerminal{}, err
		}
	}
	return accumulator.finish()
}

type managedInputTerminalAccumulator struct {
	terminal      ManagedInputTerminal
	promptID      string
	previousSeq   int64
	text          strings.Builder
	terminalEvent *acp.AgentEvent
}

func newManagedInputTerminalAccumulator(
	submission ManagedInputSubmission,
	terminalAt time.Time,
) *managedInputTerminalAccumulator {
	return &managedInputTerminalAccumulator{
		terminal: ManagedInputTerminal{
			Owner:         submission.Owner,
			DispatchToken: submission.DispatchToken,
			DriverTurnID:  submission.Owner.PromptID,
			TerminalAt:    terminalAt,
		},
		promptID: submission.Owner.PromptID,
	}
}

func (a *managedInputTerminalAccumulator) addEvent(event store.SessionEvent) error {
	if event.TurnID != a.promptID || event.Sequence <= 0 ||
		(a.previousSeq > 0 && event.Sequence <= a.previousSeq) {
		return errors.New("session: managed input events are not strictly ordered")
	}
	a.previousSeq = event.Sequence
	agentEvent, err := transcript.UnmarshalAgentEvent(event.Content)
	if err != nil {
		return fmt.Errorf("session: decode managed input event %d: %w", event.Sequence, err)
	}
	if isManagedInputProofAuxiliaryEvent(agentEvent.Type) {
		return nil
	}
	if a.terminalEvent != nil {
		return errors.New("session: managed input requires one final terminal event")
	}
	if a.terminal.EventStartSeq == 0 {
		a.terminal.EventStartSeq = event.Sequence
	}
	a.appendText(agentEvent)
	if err := a.observeUsage(agentEvent.Usage); err != nil {
		return fmt.Errorf("session: invalid managed input usage at sequence %d: %w", event.Sequence, err)
	}
	if agentEvent.Type == acp.EventTypeDone || agentEvent.Type == acp.EventTypeError {
		copyEvent := agentEvent
		a.terminalEvent = &copyEvent
		a.terminal.EventEndSeq = event.Sequence
		if !agentEvent.Timestamp.IsZero() {
			a.terminal.TerminalAt = agentEvent.Timestamp.UTC()
		}
	}
	return nil
}

func isManagedInputProofAuxiliaryEvent(eventType string) bool {
	return eventType == eventspkg.TranscriptMarkerCreated || eventType == eventspkg.TranscriptMarkerRedacted
}

func (a *managedInputTerminalAccumulator) appendText(event acp.AgentEvent) {
	if event.Type != acp.EventTypeAgentMessage || strings.TrimSpace(event.Text) == "" {
		return
	}
	if a.text.Len() > 0 {
		a.text.WriteByte('\n')
	}
	a.text.WriteString(event.Text)
}

func (a *managedInputTerminalAccumulator) observeUsage(usage *acp.TokenUsage) error {
	tokens, reported, err := managedInputTokens(usage)
	if err != nil {
		return err
	}
	if reported {
		a.terminal.TokensUsed = tokens
		a.terminal.TokensReported = true
	}
	return nil
}

func (a *managedInputTerminalAccumulator) finish() (ManagedInputTerminal, error) {
	if a.terminalEvent == nil {
		return ManagedInputTerminal{}, errors.New("session: managed input terminal event is missing")
	}
	a.terminal.Text = a.text.String()
	candidate := bytes.TrimSpace([]byte(a.terminal.Text))
	if json.Valid(candidate) {
		a.terminal.Structured = append(a.terminal.Structured[:0], candidate...)
	}
	classifyManagedInputTerminal(&a.terminal, *a.terminalEvent)
	return a.terminal, nil
}

func classifyManagedInputTerminal(terminal *ManagedInputTerminal, event acp.AgentEvent) {
	if event.Type == acp.EventTypeError {
		terminal.Outcome = managedInputOutcomeFailed
		terminal.ReasonCode = managedInputReasonPromptFailed
		return
	}
	stopReason := acp.PromptStopReason(strings.TrimSpace(string(event.PromptStopReason)))
	if managedInputStopReasonValid(stopReason) {
		terminal.Outcome = managedInputOutcomeCompleted
		terminal.StopReason = stopReason
		return
	}
	terminal.Outcome = managedInputOutcomeInvalidResult
	terminal.ReasonCode = managedInputReasonStopInvalid
}

func managedInputStopReasonValid(reason acp.PromptStopReason) bool {
	return reason.Valid()
}

func managedInputTokens(usage *acp.TokenUsage) (int64, bool, error) {
	if usage == nil {
		return 0, false, nil
	}
	if usage.TotalTokens != nil {
		if *usage.TotalTokens < 0 {
			return 0, false, errors.New("total tokens must be nonnegative")
		}
		return *usage.TotalTokens, true, nil
	}
	if usage.InputTokens == nil && usage.OutputTokens == nil {
		return 0, false, nil
	}
	var total int64
	if usage.InputTokens != nil {
		if *usage.InputTokens < 0 {
			return 0, false, errors.New("input tokens must be nonnegative")
		}
		total = *usage.InputTokens
	}
	if usage.OutputTokens != nil {
		if *usage.OutputTokens < 0 {
			return 0, false, errors.New("output tokens must be nonnegative")
		}
		if total > math.MaxInt64-*usage.OutputTokens {
			return 0, false, errors.New("input and output token sum overflows int64")
		}
		total += *usage.OutputTokens
	}
	return total, true, nil
}
