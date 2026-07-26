package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

const (
	repairContentKey = "content"
	repairTypeKey    = "type"
	repairStatusKey  = "status"
)

const (
	RepairSeverityInfo    = "info"
	RepairSeverityWarning = "warning"
	RepairSeverityError   = "error"

	RepairIssueSequenceGap                = "event_sequence_gap"
	RepairIssueSequenceDuplicate          = "event_sequence_duplicate"
	RepairIssueSequenceRegression         = "event_sequence_regression"
	RepairIssueInvalidEventJSON           = "invalid_event_json"
	RepairIssueEventTypeMismatch          = "event_type_mismatch"
	RepairIssueNoRepairableTurn           = "no_repairable_turn"
	RepairIssueSessionNotStopped          = "session_not_stopped"
	RepairIssueStopReasonRequiresForce    = "stop_reason_requires_force"
	RepairIssueDanglingToolCallMissingID  = "dangling_tool_call_missing_id"
	RepairIssueTerminalEventAlreadyExists = "terminal_event_already_exists"

	RepairActionAppendInterruptedToolResult = "append_interrupted_tool_result"
	RepairActionAppendTerminalError         = "append_terminal_error"

	repairInterruptedToolMessage = "Tool call interrupted before a result was persisted."
	repairTerminalErrorMessage   = "Session interrupted before a terminal prompt event was persisted."
)

// RepairOpts controls one persisted session repair pass.
type RepairOpts struct {
	SessionID string
	DryRun    bool
	Force     bool
}

// RepairResult describes the detected inconsistencies and append-only
// repair events planned or persisted for a session.
type RepairResult struct {
	SessionID string
	Issues    []RepairIssue
	Actions   []RepairAction
	Persisted bool
}

// RepairIssue is one non-mutating diagnostic discovered during repair.
type RepairIssue struct {
	Code     string
	Severity string
	TurnID   string
	EventID  string
	Detail   string
}

// RepairAction is one append-only mutation planned or persisted by repair.
type RepairAction struct {
	Code       string
	TurnID     string
	EventID    string
	ToolCallID string
	ToolName   string
	Persisted  bool
}

// repairEvent keeps the stored row and decoded ACP payload together so repair
// analysis can reconcile metadata and transcript content in one pass.
type repairEvent struct {
	stored store.SessionEvent
	agent  acp.AgentEvent
}

// repairTurnState tracks the latest prompt turn shape so repair can append only
// the terminal events that are still missing for that turn.
type repairTurnState struct {
	turnID        string
	hasPromptData bool
	terminal      bool
	toolCalls     map[string]repairToolCall
	toolResults   map[string]struct{}
}

// repairToolCall captures the minimum persisted tool-call metadata needed to
// synthesize a matching interrupted tool_result when none was recorded.
type repairToolCall struct {
	toolName string
}

// repairAnalysis accumulates diagnostics plus the final turn state that drives
// append-only repair planning, including whether analysis must block mutation.
type repairAnalysis struct {
	issues []RepairIssue
	turn   repairTurnState
	block  bool
}

// RepairSession inspects one persisted session transcript and, when safe,
// appends terminal repair events for an interrupted final prompt turn.
func (m *Manager) RepairSession(
	ctx context.Context,
	opts RepairOpts,
) (result *RepairResult, err error) {
	if ctx == nil {
		return nil, errors.New("session: repair context is required")
	}

	target, err := normalizeStoredSessionID(opts.SessionID)
	if err != nil {
		return nil, err
	}

	meta, err := m.readMetaWithContext(ctx, target)
	if err != nil {
		return nil, err
	}

	recorder, cleanup, err := m.openMutationRecorder(ctx, target)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cleanupErr := cleanup(); cleanupErr != nil && err == nil {
			err = cleanupErr
		}
	}()

	events, err := recorder.Query(ctx, store.EventQuery{})
	if err != nil {
		return nil, fmt.Errorf("session: query events for repair %q: %w", target, err)
	}

	result, actions := planSessionRepair(target, meta, opts, events)
	if len(actions) == 0 {
		return result, nil
	}

	persisted, err := m.persistRepairActions(ctx, recorder, meta, actions)
	if err != nil {
		return result, err
	}
	result.Actions = persisted
	result.Persisted = len(persisted) > 0
	return result, nil
}

func planSessionRepair(
	target string,
	meta store.SessionMeta,
	opts RepairOpts,
	events []store.SessionEvent,
) (*RepairResult, []RepairAction) {
	result := &RepairResult{SessionID: target}
	analysis := analyzeRepairEvents(events)
	result.Issues = append(result.Issues, analysis.issues...)
	if analysis.block {
		return result, nil
	}
	if strings.TrimSpace(analysis.turn.turnID) == "" {
		result.Issues = append(result.Issues, RepairIssue{
			Code:     RepairIssueNoRepairableTurn,
			Severity: RepairSeverityInfo,
			Detail:   "no prompt turn exists in the session event store",
		})
		return result, nil
	}
	if analysis.turn.terminal {
		result.Issues = append(result.Issues, RepairIssue{
			Code:     RepairIssueTerminalEventAlreadyExists,
			Severity: RepairSeverityInfo,
			TurnID:   analysis.turn.turnID,
			Detail:   "the final prompt turn already has a terminal event",
		})
	}

	if strings.TrimSpace(meta.State) != string(StateStopped) {
		result.Issues = append(result.Issues, RepairIssue{
			Code:     RepairIssueSessionNotStopped,
			Severity: RepairSeverityError,
			TurnID:   analysis.turn.turnID,
			Detail:   "repair only mutates stopped sessions",
		})
		return result, nil
	}
	stopReason := sessionMetaStopReason(meta)
	if !opts.Force && !repairDefaultStopReason(stopReason) {
		result.Issues = append(result.Issues, RepairIssue{
			Code:     RepairIssueStopReasonRequiresForce,
			Severity: RepairSeverityError,
			TurnID:   analysis.turn.turnID,
			Detail:   fmt.Sprintf("stop reason %q requires force before repair can mutate", stopReason),
		})
		return result, nil
	}

	actions := planRepairActions(analysis.turn, !analysis.turn.terminal)
	result.Actions = append(result.Actions, actions...)
	if opts.DryRun || len(actions) == 0 {
		return result, nil
	}
	return result, actions
}

func analyzeRepairEvents(events []store.SessionEvent) repairAnalysis {
	analysis := repairAnalysis{}
	turns := make(map[string]*repairTurnState)
	var lastTurnID string
	var previousSequence int64

	for _, event := range sortedRepairEvents(events) {
		trackRepairSequence(&analysis, event, &previousSequence)
		agentEvent, eventType, ok := decodeRepairEvent(&analysis, event)
		if !ok {
			continue
		}
		turnID := strings.TrimSpace(firstNonEmpty(event.TurnID, agentEvent.TurnID))
		if turnID == "" || eventType == EventTypeSessionStopped {
			continue
		}
		turn := ensureRepairTurn(turns, turnID)
		lastTurnID = turnID
		applyRepairEvent(
			turn,
			&repairEvent{stored: event, agent: agentEvent},
			eventType,
			&analysis,
		)
	}

	if lastTurnID != "" {
		analysis.turn = *turns[lastTurnID]
	}
	return analysis
}

func sortedRepairEvents(events []store.SessionEvent) []store.SessionEvent {
	ordered := append([]store.SessionEvent(nil), events...)
	slices.SortStableFunc(ordered, func(a store.SessionEvent, b store.SessionEvent) int {
		switch {
		case a.Sequence < b.Sequence:
			return -1
		case a.Sequence > b.Sequence:
			return 1
		default:
			return strings.Compare(a.ID, b.ID)
		}
	})
	return ordered
}

func trackRepairSequence(
	analysis *repairAnalysis,
	event store.SessionEvent,
	previousSequence *int64,
) {
	expected := *previousSequence + 1
	switch {
	case *previousSequence > 0 && event.Sequence == *previousSequence:
		analysis.issues = append(analysis.issues, RepairIssue{
			Code:     RepairIssueSequenceDuplicate,
			Severity: RepairSeverityError,
			EventID:  event.ID,
			Detail:   fmt.Sprintf("duplicate sequence %d", event.Sequence),
		})
		analysis.block = true
	case *previousSequence > 0 && event.Sequence < *previousSequence:
		analysis.issues = append(analysis.issues, RepairIssue{
			Code:     RepairIssueSequenceRegression,
			Severity: RepairSeverityError,
			EventID:  event.ID,
			Detail:   fmt.Sprintf("sequence regressed from %d to %d", *previousSequence, event.Sequence),
		})
		analysis.block = true
	case event.Sequence != expected:
		analysis.issues = append(analysis.issues, RepairIssue{
			Code:     RepairIssueSequenceGap,
			Severity: RepairSeverityWarning,
			EventID:  event.ID,
			Detail:   fmt.Sprintf("expected sequence %d, found %d", expected, event.Sequence),
		})
	}
	if event.Sequence > *previousSequence {
		*previousSequence = event.Sequence
	}
}

func decodeRepairEvent(
	analysis *repairAnalysis,
	event store.SessionEvent,
) (acp.AgentEvent, string, bool) {
	agentEvent, err := transcript.UnmarshalAgentEvent(event.Content)
	if err != nil {
		analysis.issues = append(analysis.issues, RepairIssue{
			Code:     RepairIssueInvalidEventJSON,
			Severity: RepairSeverityError,
			TurnID:   strings.TrimSpace(event.TurnID),
			EventID:  event.ID,
			Detail:   err.Error(),
		})
		analysis.block = true
		return acp.AgentEvent{}, "", false
	}

	eventType := strings.TrimSpace(event.Type)
	decodedType := strings.TrimSpace(agentEvent.Type)
	if decodedType != "" && eventType != "" && decodedType != eventType {
		analysis.issues = append(analysis.issues, RepairIssue{
			Code:     RepairIssueEventTypeMismatch,
			Severity: RepairSeverityError,
			TurnID:   strings.TrimSpace(event.TurnID),
			EventID:  event.ID,
			Detail:   fmt.Sprintf("stored type %q does not match payload type %q", eventType, decodedType),
		})
		analysis.block = true
	}
	if eventType == "" {
		eventType = decodedType
	}
	return agentEvent, eventType, true
}
