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
	session.clearPromptCancellation(turnID)
	session.clearCurrentTurnID()
	session.clearCurrentTurnSource()
	session.clearCurrentPromptMessage()
	session.clearCurrentPromptMeta()
	session.clearCurrentSkillInvocations()
	session.clearCurrentPromptCancel()
	session.finishCurrentPromptCompletion()
}
