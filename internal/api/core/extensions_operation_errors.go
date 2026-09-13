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
	case extensionErrorMCPNameTaken:
		payload.Code = "mcp_server_name_taken"
	case extensionErrorNameConflict,
		extensionErrorSourceChanged,
		extensionErrorSourceUnreachable,
		extensionErrorMarketplaceSourceNotFound,
		extensionErrorInputsRequired,
		extensionErrorInputInvalid:
		payload, _ = ExtensionAcquisitionErrorPayload(err)
	case extensionErrorNetworkConfirmationRequired:
		confirmationErr, ok := errors.AsType[*extensionpkg.NetworkConfirmationRequiredError](err)
		if ok && confirmationErr != nil {
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
		if conflictErr, ok := errors.AsType[*extensionpkg.AgentConflictError](err); ok && conflictErr != nil {
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
		if bindingErr, ok := errors.AsType[*extensionpkg.EnvBindingValidationError](err); ok && bindingErr != nil {
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
	case extensionErrorAgentPluginNotManifest,
		extensionErrorAgentPluginSchemaUnsupported:
		payload.Code = ExtensionAgentPluginErrorCode(err)
	default:
		return contract.ExtensionOperationErrorPayload{}, false
	}
	payload.Error = message
	return payload, true
}

// ExtensionAgentPluginErrorCode returns the stable branch key for one portable-package failure.
func ExtensionAgentPluginErrorCode(err error) string {
	switch classifyExtensionError(err) {
	case extensionErrorAgentPluginNotManifest:
		return diagnosticcontract.CodeExtensionAgentPluginNotManifest
	case extensionErrorAgentPluginSchemaUnsupported:
		return diagnosticcontract.CodeExtensionAgentPluginSchemaUnsupported
	case extensionErrorAgentPluginManifestInvalid:
		return diagnosticcontract.CodeExtensionAgentPluginManifestInvalid
	default:
		return ""
	}
}

func extensionOperationDiagnostic(id, code, title, message, command string) *contract.DiagnosticItem {
	options := []diagnosticspkg.ItemOption{}
	if strings.TrimSpace(command) != "" {
		options = append(options, diagnosticspkg.WithSuggestedCommand(command))
	}
	item := diagnosticspkg.NewItem(diagnosticspkg.ItemSpec{
		ID:            id,
		Code:          code,
		Category:      diagnosticcontract.CategoryExtension,
		Title:         title,
		Message:       message,
		Severity:      diagnosticcontract.SeverityError,
		DataFreshness: diagnosticcontract.FreshnessLive,
	},
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

// ExtensionAcquisitionErrorPayload maps candidate failures consistently across public transports.
func ExtensionAcquisitionErrorPayload(err error) (contract.ExtensionOperationErrorPayload, bool) {
	payload := contract.ExtensionOperationErrorPayload{}
	switch classifyExtensionError(err) {
	case extensionErrorMarketplaceSourceNotFound:
		payload.Code = "marketplace_source_not_found"
	case extensionErrorSourceUnreachable:
		payload.Code = "source_unreachable"
	case extensionErrorNameConflict:
		payload.Code = diagnosticcontract.CodeExtensionNameConflict
		if conflict, ok := errors.AsType[*extensionpkg.ExtensionNameConflictError](err); ok {
			payload.InstalledOrigin = &contract.MarketplaceOriginPayload{
				Source: conflict.SourceName, SourceRef: conflict.InstalledOrigin.SourceRef,
				EntryID: conflict.InstalledOrigin.EntryID,
			}
		}
	case extensionErrorSourceChanged:
		payload.Code = diagnosticcontract.CodeExtensionSourceChanged
		if changed, ok := errors.AsType[*extensionpkg.SourceChangedError](err); ok {
			payload.ListedDigest, payload.FetchedDigest = changed.ListedDigest, changed.FetchedDigest
		}
	case extensionErrorInputsRequired:
		payload.Code = diagnosticcontract.CodeExtensionInputsRequired
		if required, ok := errors.AsType[*extensionpkg.InputsRequiredError](err); ok {
			payload.Inputs, payload.MissingEnv = slices.Clone(required.MissingInputs), slices.Clone(required.MissingEnv)
			for _, input := range required.InputDefinitions {
				payload.InputDefinitions = append(payload.InputDefinitions, contract.MarketplaceInputPayload{
					ID:       input.ID,
					Prompt:   input.Prompt,
					Type:     input.Type,
					Required: input.Required,
					Default:  slices.Clone(input.Default),
					Binding: contract.MarketplaceInputBindingPayload{
						Type: input.Binding.Type,
						Name: input.Binding.Name,
					},
				})
			}
		}
	case extensionErrorInputInvalid:
		payload.Code = diagnosticcontract.CodeExtensionInputInvalid
		if invalid, ok := errors.AsType[*extensionpkg.InputValidationError](err); ok {
			payload.InputID = invalid.InputID
		}
	default:
		return payload, false
	}
	payload.Error = err.Error()
	return payload, true
}
