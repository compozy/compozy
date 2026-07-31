package contract

import (
	"strings"

	"github.com/compozy/compozy/internal/session"
)

// PromptRuntimeSelectionFromPayload converts an optional prompt runtime snapshot.
func PromptRuntimeSelectionFromPayload(
	payload *PromptRuntimeSelectionPayload,
) *session.RuntimeSelection {
	if payload == nil {
		return nil
	}
	return &session.RuntimeSelection{
		Provider:        strings.TrimSpace(payload.Provider),
		Model:           strings.TrimSpace(payload.Model),
		ReasoningEffort: strings.TrimSpace(string(payload.ReasoningEffort)),
		Speed:           payload.Speed,
	}
}
