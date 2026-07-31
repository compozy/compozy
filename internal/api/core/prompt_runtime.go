package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
)

// PromptRuntimeSelectionFromPayload converts an optional prompt runtime snapshot.
func PromptRuntimeSelectionFromPayload(
	payload *contract.PromptRuntimeSelectionPayload,
) *session.RuntimeSelection {
	return contract.PromptRuntimeSelectionFromPayload(payload)
}
