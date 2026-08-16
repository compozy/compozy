package session

import (
	"encoding/json"
	"strings"
)

type promptTurnDispatchState struct {
	session     *Session
	turnID      string
	turnSource  TurnSource
	inputClass  string
	userMessage string
	messageSeq  int
	turnEnded   bool
	openMessage *promptMessageDispatchState
	managed     *managedInputExecution
}

type promptMessageDispatchState struct {
	id      string
	role    string
	text    strings.Builder
	lastRaw json.RawMessage
}

func claimPromptState(session *Session, req promptRequest) func() {
	session.setCurrentTurnID(req.turnID)
	session.setCurrentTurnSource(req.turnSource)
	session.setCurrentPromptMessage(req.authoredMessage)
	session.setCurrentPromptMeta(req.meta)
	session.setCurrentSkillInvocations(req.skillInvocations)
	return func() {
		session.clearPromptCancellation(req.turnID)
		session.clearCurrentTurnID()
		session.clearCurrentTurnSource()
		session.clearCurrentPromptMessage()
		session.clearCurrentPromptMeta()
		session.clearCurrentSkillInvocations()
		session.finishCurrentPromptCompletion()
	}
}
