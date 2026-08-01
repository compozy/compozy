package core

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func errorPayloadForMessage(message string, err error) contract.ErrorPayload {
	message = diagnosticspkg.Redact(taskpkg.RedactClaimTokens(message))
	payload := contract.ErrorPayload{Error: message}
	if item, ok := diagnosticspkg.ItemFromError(err); ok {
		payload.Diagnostic = &item
	} else if errors.Is(err, compozyconfig.ErrAgentNameReserved) {
		item := diagnosticspkg.NewItem(
			"agent.name.reserved",
			contract.CodeAgentNameReserved,
			contract.CategoryConfig,
			"Agent name is reserved",
			message,
			contract.SeverityError,
			contract.FreshnessLive,
		)
		payload.Diagnostic = &item
	} else if code := diagnosticCodeFromError(err); code != "" {
		if category, ok := contract.DiagnosticCodeCategory(code); ok {
			item := diagnosticspkg.NewItem(
				strings.ReplaceAll(code, "_", "."),
				code,
				category,
				"Role operation failed",
				message,
				contract.SeverityError,
				contract.FreshnessLive,
			)
			payload.Diagnostic = &item
		}
	}
	return payload
}

type diagnosticCodeCarrier interface {
	DiagnosticCode() string
}

func diagnosticCodeFromError(err error) string {
	switch {
	case errors.Is(err, store.ErrSessionPromptDispatchIndeterminate):
		return contract.CodePromptDispatchIndeterminate
	case errors.Is(err, store.ErrSessionPromptIdempotencyConflict):
		return contract.CodePromptIdempotencyConflict
	case errors.Is(err, store.ErrSessionPromptMessageConflict):
		return contract.CodePromptMessageIdentityConflict
	}
	var carrier diagnosticCodeCarrier
	if !errors.As(err, &carrier) {
		return ""
	}
	return strings.TrimSpace(carrier.DiagnosticCode())
}
