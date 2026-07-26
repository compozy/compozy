package core

import (
	"errors"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	diagnosticspkg "github.com/compozy/agh/internal/diagnostics"
	taskpkg "github.com/compozy/agh/internal/task"
)

func errorPayloadForMessage(message string, err error) contract.ErrorPayload {
	message = diagnosticspkg.Redact(taskpkg.RedactClaimTokens(message))
	payload := contract.ErrorPayload{Error: message}
	if item, ok := diagnosticspkg.ItemFromError(err); ok {
		payload.Diagnostic = &item
	} else if errors.Is(err, aghconfig.ErrAgentNameReserved) {
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
	var carrier diagnosticCodeCarrier
	if !errors.As(err, &carrier) {
		return ""
	}
	return strings.TrimSpace(carrier.DiagnosticCode())
}
