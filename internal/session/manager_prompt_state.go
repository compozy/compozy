package session

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/internal/acp"
)

type promptTurnDispatchState struct {
	session     *Session
	turnID      string
	runID       string
	generation  int64
	turnSource  TurnSource
	inputClass  string
	userMessage string
	messageSeq  int
	turnEnded   bool
	openMessage *promptMessageDispatchState
	managed     *managedInputExecution
	recovery    *promptRecoveryState
}

type promptRecoveryState struct {
	executionCtx      context.Context
	request           acp.PromptRequest
	attempts          int
	exhaustedRecorded bool
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
	session.currentTurnSource = ""
	session.currentPromptMessage = ""
	session.currentPromptMeta = acp.PromptMeta{}
	session.currentSkillInvocations = nil
	session.currentPromptCancel = nil
	session.promptCancelRequested = false
	if session.currentPromptDone != nil {
		close(session.currentPromptDone)
		session.currentPromptDone = nil
	}
}
