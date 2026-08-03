package core

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/gin-gonic/gin"
)

func extensionOperationErrorPayload(
	c *gin.Context,
	status int,
	err error,
	maskInternal bool,
) (contract.ExtensionOperationErrorPayload, bool) {
	payload := contract.ExtensionOperationErrorPayload{}
	message := ErrorPayloadForStatus(status, err, maskInternal).Error
	name := strings.TrimSpace(c.Param("name"))

	kind := classifyExtensionError(err)
	switch kind {
	case extensionErrorNetworkConfirmationRequired:
		var confirmationErr *extensionpkg.NetworkConfirmationRequiredError
		if errors.As(err, &confirmationErr) && confirmationErr != nil {
			payload.CurrentDigest = strings.TrimSpace(confirmationErr.CurrentDigest)
		}
		payload.Code = diagnosticcontract.CodeExtensionNetworkConfirmRequired
		payload.Diagnostic = extensionOperationDiagnostic(
			"extension.network_confirmation_required",
			payload.Code,
			"Network confirmation is required",
			message,
			extensionNetworkRetryCommand(name, payload.CurrentDigest),
		)
	case extensionErrorAgentConflict:
		var conflictErr *extensionpkg.AgentConflictError
		if errors.As(err, &conflictErr) && conflictErr != nil {
			payload.Agents = slices.Clone(conflictErr.Agents)
			slices.Sort(payload.Agents)
		}
		payload.Code = diagnosticcontract.CodeExtensionAgentConflict
		payload.Diagnostic = extensionOperationDiagnostic(
			"extension.agent_conflict",
			payload.Code,
			"Extension agents conflict",
			message,
			"",
		)
	case extensionErrorEnvBindingUndeclared, extensionErrorEnvBindingDangling, extensionErrorEnvBindingInvalid:
		var bindingErr *extensionpkg.EnvBindingValidationError
		if errors.As(err, &bindingErr) && bindingErr != nil {
			payload.EnvName = strings.TrimSpace(bindingErr.EnvName)
			payload.DeclaredEnv = slices.Clone(bindingErr.Declared)
			slices.Sort(payload.DeclaredEnv)
		}
		payload.Code = extensionEnvBindingErrorCode(kind)
		payload.Diagnostic = extensionOperationDiagnostic(
			"extension.env_binding_invalid",
			payload.Code,
			"Extension secret binding is invalid",
			message,
			"",
		)
	default:
		return contract.ExtensionOperationErrorPayload{}, false
	}
	payload.Error = message
	return payload, true
}

func extensionOperationDiagnostic(id, code, title, message, command string) *contract.DiagnosticItem {
	options := []diagnosticspkg.ItemOption{}
	if strings.TrimSpace(command) != "" {
		options = append(options, diagnosticspkg.WithSuggestedCommand(command))
	}
	item := diagnosticspkg.NewItem(
		id,
		code,
		diagnosticcontract.CategoryExtension,
		title,
		message,
		diagnosticcontract.SeverityError,
		diagnosticcontract.FreshnessLive,
		options...,
	)
	return &item
}

func extensionNetworkRetryCommand(name, digest string) string {
	name = strings.TrimSpace(name)
	digest = strings.TrimSpace(digest)
	if name == "" || digest == "" {
		return ""
	}
	return fmt.Sprintf("compozy extension enable %s --confirm-network-requirement %s", name, digest)
}

func extensionEnvBindingErrorCode(kind extensionErrorKind) string {
	switch kind {
	case extensionErrorEnvBindingUndeclared:
		return diagnosticcontract.CodeExtensionEnvBindingUndeclared
	case extensionErrorEnvBindingDangling:
		return diagnosticcontract.CodeExtensionEnvBindingDangling
	default:
		return diagnosticcontract.CodeExtensionEnvBindingInvalid
	}
}
