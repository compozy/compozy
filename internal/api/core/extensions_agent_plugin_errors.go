package core

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extension/agentplugin"
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
	var manifestIssues []agentplugin.Issue
	if validationErr, ok := errors.AsType[*extensionpkg.AgentPluginManifestValidationError](
		err,
	); ok &&
		validationErr != nil {
		manifestIssues = validationErr.Issues
	} else if validationErr, ok := errors.AsType[*agentplugin.ManifestError](
		err,
	); ok &&
		validationErr != nil {
		manifestIssues = validationErr.Issues
	}
	if len(manifestIssues) > 0 {
		issues = make([]contract.ValidationIssue, 0, len(manifestIssues))
		for _, item := range manifestIssues {
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
