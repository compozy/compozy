package core

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func agentPluginManifestErrorPayload(
	status int,
	err error,
	maskInternal bool,
) contract.ExtensionValidationErrorPayload {
	message := redactAgentPluginIssueText(ErrorPayloadForStatus(status, err, maskInternal).Error)
	diagnostic := diagnosticspkg.NewItem(diagnosticspkg.ItemSpec{
		ID:            "extension.agent_plugin.manifest_invalid",
		Code:          diagnosticcontract.CodeExtensionAgentPluginManifestInvalid,
		Category:      diagnosticcontract.CategoryExtension,
		Title:         message,
		Message:       message,
		Severity:      diagnosticcontract.SeverityError,
		DataFreshness: diagnosticcontract.FreshnessLive,
	})
	issues := []contract.ValidationIssue{{
		Path: "plugin.json", Message: message, Severity: contract.IssueSeverityError,
	}}
	if validationErr, ok := errors.AsType[*extensionpkg.AgentPluginManifestValidationError](err); ok &&
		validationErr != nil && len(validationErr.Issues) > 0 {
		issues = make([]contract.ValidationIssue, 0, len(validationErr.Issues))
		for _, item := range validationErr.Issues {
			issues = append(issues, contract.ValidationIssue{
				Path:     "plugin.json",
				Field:    redactAgentPluginIssueText(item.Path),
				Message:  redactAgentPluginIssueText(item.Message),
				Severity: contract.IssueSeverityError,
			})
		}
	}
	return contract.ExtensionValidationErrorPayload{
		Error: message, Diagnostic: &diagnostic, Issues: issues,
	}
}

func redactAgentPluginIssueText(value string) string {
	return diagnosticspkg.Redact(taskpkg.RedactClaimTokens(strings.TrimSpace(value)))
}
