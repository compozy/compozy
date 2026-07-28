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
