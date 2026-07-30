package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
)

// PromptRuntimeSelectionFromPayload converts an optional prompt runtime snapshot.
func PromptRuntimeSelectionFromPayload(
	payload *contract.PromptRuntimeSelectionPayload,
) *session.RuntimeSelection {
	if payload == nil {
		return nil
	}
	return &session.RuntimeSelection{
		Provider:        payload.Provider,
		Model:           payload.Model,
		ReasoningEffort: string(payload.ReasoningEffort),
		Speed:           payload.Speed,
	}
}
