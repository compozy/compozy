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
	path := "plugin.json"
	var manifestIssues []agentplugin.Issue
	if validationErr, _ := errors.AsType[*extensionpkg.AgentPluginManifestValidationError](err); validationErr != nil {
		path = agentplugin.PackageManifestPath(validationErr.Path)
		manifestIssues = validationErr.Issues
	} else if archiveErr, _ := errors.AsType[*agentplugin.ManifestError](err); archiveErr != nil {
		manifestIssues = archiveErr.Issues
		if archiveErr.ManifestPath != "" {
			path = agentplugin.PackageManifestPath(archiveErr.ManifestPath)
			err = archiveErr
		}
	}
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
		Path: path, Message: message, Severity: contract.IssueSeverityError,
	}}
	if len(manifestIssues) > 0 {
		issues = make([]contract.ValidationIssue, 0, len(manifestIssues))
		for _, item := range manifestIssues {
			issues = append(issues, contract.ValidationIssue{
				Path:     path,
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
