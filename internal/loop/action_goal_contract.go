package loop

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
)

// ActionStopReason is the closed ACP terminal reason vocabulary persisted for managed prompts.
type ActionStopReason string

const (
	// ActionStopEndTurn reports a normal provider turn boundary.
	ActionStopEndTurn ActionStopReason = "end_turn"
	// ActionStopMaxTokens reports provider context exhaustion.
	ActionStopMaxTokens ActionStopReason = "max_tokens"
	// ActionStopMaxTurnRequests reports exhaustion of one provider request allowance.
	ActionStopMaxTurnRequests ActionStopReason = "max_turn_requests"
	// ActionStopRefusal reports an explicit provider refusal.
	ActionStopRefusal ActionStopReason = "refusal"
	// ActionStopCancelled reports a correlated cancellation.
	ActionStopCancelled ActionStopReason = "cancelled" //nolint:misspell // ACP persists the British spelling.
)

// Valid reports whether the stop reason is part of the closed persisted vocabulary.
func (r ActionStopReason) Valid() bool {
	switch r {
	case ActionStopEndTurn,
		ActionStopMaxTokens,
		ActionStopMaxTurnRequests,
		ActionStopRefusal,
		ActionStopCancelled:
		return true
	default:
		return false
	}
}

// ActionPromptOutcome is the durable managed-input terminal classification.
type ActionPromptOutcome string

const (
	// ActionPromptOutcomeCompleted carries exactly one valid ACP stop reason.
	ActionPromptOutcomeCompleted ActionPromptOutcome = "completed"
	// ActionPromptOutcomeInvalidResult carries malformed terminal provider evidence.
	ActionPromptOutcomeInvalidResult ActionPromptOutcome = "invalid-result"
	// ActionPromptOutcomeFailed carries a normalized post-dispatch failure.
	ActionPromptOutcomeFailed ActionPromptOutcome = "failed"
	// ActionPromptOutcomeAmbiguous records an effect whose terminal truth cannot be proven.
	ActionPromptOutcomeAmbiguous ActionPromptOutcome = "ambiguous"
	// ActionPromptOutcomeBudgetFenced records a pre-submit budget fence.
	ActionPromptOutcomeBudgetFenced ActionPromptOutcome = "budget-fenced"
	// ActionPromptOutcomeControlFenced records a pre-submit control fence.
	ActionPromptOutcomeControlFenced ActionPromptOutcome = "control-fenced"
	// ActionPromptOutcomeRejectedBeforeSubmit records a proven effect-known-false rejection.
	ActionPromptOutcomeRejectedBeforeSubmit ActionPromptOutcome = "rejected-before-submit"
)

// ActionDisposition is the typed action-to-coordinator control vocabulary.
type ActionDisposition string

const (
	// ActionDispositionSucceeded follows normal action harvest.
	ActionDispositionSucceeded ActionDisposition = "succeeded"
	// ActionDispositionBlocked terminates the run as blocked.
	ActionDispositionBlocked ActionDisposition = "blocked"
	// ActionDispositionExhausted terminates the run as exhausted.
	ActionDispositionExhausted ActionDisposition = "exhausted"
	// ActionDispositionPaused parks the run until Resume.
	ActionDispositionPaused ActionDisposition = "paused"
	// ActionDispositionNeedsApproval parks the run behind a typed approval.
	ActionDispositionNeedsApproval ActionDisposition = "needs-approval"
)

const (
	actionGoalStatusComplete      = "complete"
	actionGoalStatusBudgetLimited = "budget-limited"
	actionGoalStatusUsageLimited  = "usage-limited"
)

// Valid reports whether the disposition is part of the closed coordinator-control vocabulary.
func (d ActionDisposition) Valid() bool {
	switch d {
	case ActionDispositionSucceeded,
		ActionDispositionBlocked,
		ActionDispositionExhausted,
		ActionDispositionPaused,
		ActionDispositionNeedsApproval:
		return true
	default:
		return false
	}
}

// ActionControl carries a neutral loop-owned control result through task completion.
type ActionControl struct {
	Disposition  ActionDisposition `json:"disposition"`
	GoalStatus   string            `json:"goal_status"`
	Cause        ReasonCode        `json:"cause,omitempty"`
	GateID       dsl.NodeID        `json:"gate_id,omitempty"`
	CheckpointID string            `json:"checkpoint_id,omitempty"`
}

// Validate enforces the Goal status/disposition contract before coordinator settlement.
func (c ActionControl) Validate() error {
	goalStatus := strings.TrimSpace(c.GoalStatus)
	checkpointID := strings.TrimSpace(c.CheckpointID)
	if !c.Disposition.Valid() {
		return invalidActionControlError("disposition", string(c.Disposition))
	}
	if checkpointID == "" {
		return invalidActionControlError("checkpoint_id", "")
	}
	if !actionControlGoalStatusMatches(c.Disposition, goalStatus) {
		return invalidActionControlError("goal_status", goalStatus)
	}
	cause := strings.TrimSpace(string(c.Cause))
	if c.Disposition != ActionDispositionSucceeded && cause == "" {
		return invalidActionControlError("cause", "")
	}
	if c.Disposition == ActionDispositionSucceeded && cause != "" {
		return invalidActionControlError("cause", cause)
	}
	gateID := strings.TrimSpace(string(c.GateID))
	if c.Disposition == ActionDispositionNeedsApproval && gateID == "" {
		return invalidActionControlError("gate_id", "")
	}
	if c.Disposition != ActionDispositionNeedsApproval && gateID != "" {
		return invalidActionControlError("gate_id", gateID)
	}
	return nil
}

func actionControlGoalStatusMatches(disposition ActionDisposition, status string) bool {
	switch disposition {
	case ActionDispositionSucceeded:
		return status == actionGoalStatusComplete
	case ActionDispositionBlocked:
		return status == string(ActionDispositionBlocked)
	case ActionDispositionExhausted:
		return status == actionGoalStatusBudgetLimited
	case ActionDispositionPaused:
		return status == string(ActionDispositionPaused)
	case ActionDispositionNeedsApproval:
		return status == string(ActionDispositionPaused) || status == actionGoalStatusBudgetLimited ||
			status == actionGoalStatusUsageLimited
	default:
		return false
	}
}

func invalidActionControlError(field string, value string) error {
	return reasonError(
		ReasonCodeGoalActionControlInvalid,
		fmt.Errorf("%w: action control %s is invalid", ErrValidation, field),
		map[string]string{"field": field, transformValueKey: value},
	)
}

// ActionPromptTerminal is the durable evidence reconstructed for a managed prompt.
type ActionPromptTerminal struct {
	PromptID         string
	Outcome          ActionPromptOutcome
	Text             string
	Structured       json.RawMessage
	EventStartSeq    int64
	EventEndSeq      int64
	TokensUsed       int64
	TokensReported   bool
	StopReason       ActionStopReason
	FenceDisposition ActionDisposition
	Disposition      ActionDisposition
	ReasonCode       ReasonCode
}
