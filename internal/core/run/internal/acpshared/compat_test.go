package acpshared

import (
	"github.com/compozy/compozy/internal/core/run/transcript"
)

func newSessionUpdateHandler(cfg SessionUpdateHandlerConfig) *SessionUpdateHandler {
	return NewSessionUpdateHandler(cfg)
}

const transcriptEntryAssistantMessage = transcript.EntryKindAssistantMessage
