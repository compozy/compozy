package session

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

type promptTurnDispatchState struct {
	reportMessageIDs []string
	reportBlock      string
	session          *Session
	turnID           string
	runID            string
	generation       int64
	turnSource       TurnSource
	inputClass       string
	userMessage      string
	messageSeq       int
	turnEnded        bool
	// stopReason is the provider stop reason of the turn's terminal done event.
	stopReason  acp.PromptStopReason
	openMessage *promptMessageDispatchState
	managed     *managedInputExecution
	recovery    *promptRecoveryState
}

type promptRecoveryState struct {
	executionCtx    context.Context
	request         acp.PromptRequest
	originalMessage string
	budgets         map[store.SessionRuntimeRecoveryReason]*promptRecoveryBudget
	// workObserved records that the provider already produced output or tool work this turn.
	workObserved bool
}

// promptRecoveryBudget tracks one failure class so each keeps its own bounded schedule.
type promptRecoveryBudget struct {
	attempts          int
	startedAt         time.Time
	expiresAt         time.Time
	exhaustedRecorded bool
}

func (r *promptRecoveryState) budget(reason store.SessionRuntimeRecoveryReason) *promptRecoveryBudget {
	if r.budgets == nil {
		r.budgets = make(map[store.SessionRuntimeRecoveryReason]*promptRecoveryBudget, 2)
	}
	budget := r.budgets[reason]
	if budget == nil {
		budget = &promptRecoveryBudget{}
		r.budgets[reason] = budget
	}
	return budget
}

func (r *promptRecoveryState) observeDelivered(eventType string) {
	if r == nil || r.workObserved {
		return
	}
	switch eventType {
	case acp.EventTypeAgentMessage, acp.EventTypeThought, acp.EventTypePlan, acp.EventTypeToolCall,
		acp.EventTypeToolResult, acp.EventTypePermission, acp.EventTypeClarify:
		r.workObserved = true
	}
}

type promptMessageDispatchState struct {
	id      string
	role    string
	text    strings.Builder
	lastRaw json.RawMessage
}

func clearPromptState(session *Session, turnID string) {
	if session == nil {
		return
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	// A late completion must not clear a newer prompt's ownership or draft metadata.
	if turnID != "" && session.currentTurnID != turnID {
		return
	}
	if session.currentPromptCancelTurn == turnID {
		session.currentPromptCancelTurn = ""
	}
	session.currentTurnID = ""
	session.openToolCalls = nil
	session.hasBoundarySteer = false
	session.currentTurnSource = ""
	session.currentPromptMessage = ""
	session.currentPromptDelivery = PromptDeliveryNormal
	session.currentPromptMeta = acp.PromptMeta{}
	session.currentTurnHopFloor = 0
	session.currentSkillInvocations = nil
	session.currentPromptCancel = nil
	session.promptCancelRequested = false
	if session.currentPromptDone != nil {
		close(session.currentPromptDone)
		session.currentPromptDone = nil
	}
}
