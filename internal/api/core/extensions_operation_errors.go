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
	if dependencyErrorContext(err) {
		payload, _ = ExtensionAcquisitionErrorPayload(err)
		payload.Error = message
		return payload, true
	}

	kind := classifyExtensionError(err)
	switch kind {
	case extensionErrorMCPNameTaken:
		payload.Code = "mcp_server_name_taken"
	case extensionErrorNameConflict,
		extensionErrorSourceChanged,
		extensionErrorSourceUnreachable,
		extensionErrorMarketplaceSourceNotFound,
		extensionErrorInputsRequired,
		extensionErrorInputInvalid,
		extensionErrorInputOverwriteDenied,
		extensionErrorDependenciesRequired,
		extensionErrorDependencyGraphInvalid:
		payload, _ = ExtensionAcquisitionErrorPayload(err)
	case extensionErrorGatewayConfirmationRequired:
		confirmationErr, ok := errors.AsType[*extensionpkg.GatewayConfirmationRequiredError](err)
		if ok && confirmationErr != nil {
			payload.CurrentDigest = strings.TrimSpace(confirmationErr.CurrentDigest)
		}
		payload.Code = diagnosticcontract.CodeExtensionGatewayConfirmRequired
		payload.Diagnostic = extensionOperationDiagnostic(
			"extension.gateway_confirmation_required",
			payload.Code,
			"Gateway confirmation is required",
			message,
			extensionGatewayRetryCommand(name, payload.CurrentDigest),
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
		var ok bool
		if payload, ok = extensionBuiltinOperationPayload(kind, message, name); !ok {
			return contract.ExtensionOperationErrorPayload{}, false
		}
	}
	payload.Error = message
	return payload, true
}

// extensionBuiltinOperationPayload maps the built-in lifecycle conflicts (not a built-in, not removed,
// existing copy, failed restore, failed removal compensation) onto their stable codes. It reports false for
// every other kind.
func extensionBuiltinOperationPayload(
	kind extensionErrorKind,
	message string,
	name string,
) (contract.ExtensionOperationErrorPayload, bool) {
	payload := contract.ExtensionOperationErrorPayload{}
	switch kind {
	case extensionErrorNotBuiltin:
		payload.Code = diagnosticcontract.CodeExtensionNotBuiltin
		payload.Diagnostic = extensionOperationDiagnostic(
			"extension.not_builtin",
			payload.Code,
			"Extension is not a built-in",
			message,
			"",
		)
	case extensionErrorBuiltinNotRemoved:
		payload.Code = diagnosticcontract.CodeExtensionBuiltinNotRemoved
		payload.Diagnostic = extensionOperationDiagnostic(
			"extension.builtin_not_removed",
			payload.Code,
			"Built-in extension is not permanently removed",
			message,
			"",
		)
	case extensionErrorExists:
		payload.Code = diagnosticcontract.CodeExtensionExists
		payload.Diagnostic = extensionOperationDiagnostic(
			"extension.exists",
			payload.Code,
			"Extension is already installed",
			message,
			extensionRemoveCommandFor(name),
		)
	case extensionErrorRestoreFailed:
		payload.Code = diagnosticcontract.CodeExtensionRestoreFailed
		payload.Diagnostic = extensionOperationDiagnostic(
			"extension.restore_failed",
			payload.Code,
			"Built-in extension restore failed",
			message,
			"compozy doctor",
		)
	case extensionErrorBuiltinRemovalCompensationFailed:
		// The tombstone is still recorded, so restore is the action that brings the built-in back.
		payload.Code = diagnosticcontract.CodeExtensionRemovalCompensationFailed
		payload.Diagnostic = extensionOperationDiagnostic(
			"extension.removal_compensation_failed",
			payload.Code,
			"Built-in extension removal could not be compensated",
			message,
			extensionRestoreCommandFor(name),
		)
	case extensionErrorRemovalCompensationFailed:
		payload.Code = diagnosticcontract.CodeExtensionRemovalCompensationFailed
		payload.Diagnostic = extensionOperationDiagnostic(
			"extension.removal_compensation_failed",
			payload.Code,
			"Extension removal could not be compensated",
			message,
			"compozy doctor",
		)
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

// extensionRemoveCommandFor suggests removing an installed copy so a restore can proceed. It is empty
// when the request carries no extension name, such as an install.
func extensionRemoveCommandFor(name string) string {
	if name = strings.TrimSpace(name); name == "" {
		return ""
	}
	return "compozy extension remove " + name
}

// extensionRestoreCommandFor suggests restoring a permanently removed built-in. It is empty when the request
// carries no extension name.
func extensionRestoreCommandFor(name string) string {
	if name = strings.TrimSpace(name); name == "" {
		return ""
	}
	return "compozy extension restore " + name
}

func extensionGatewayRetryCommand(name, digest string) string {
	name = strings.TrimSpace(name)
	digest = strings.TrimSpace(digest)
	if name == "" || digest == "" {
		return ""
	}
	return fmt.Sprintf("compozy extension enable %s --confirm-gateway-requirement %s", name, digest)
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

// ExtensionAcquisitionErrorPayload maps candidate failures consistently across public transports. A failure that
// belongs to a dependency, or follows completed dependency installs, always maps and names them.
func ExtensionAcquisitionErrorPayload(err error) (contract.ExtensionOperationErrorPayload, bool) {
	payload, ok := extensionAcquisitionKindPayload(err)
	if !dependencyErrorContext(err) {
		return payload, ok
	}
	if !ok {
		payload = extensionDependencyGatePayload(err)
	}
	if gate, gated := errors.AsType[*extensionpkg.DependencyGateError](err); gated {
		payload.Dependency = gate.Dependency
	}
	if installed, partial := errors.AsType[*extensionpkg.DependenciesInstalledError](err); partial {
		payload.DependenciesInstalled = slices.Clone(installed.Installed)
	}
	return payload, true
}

func dependencyErrorContext(err error) bool {
	_, gated := errors.AsType[*extensionpkg.DependencyGateError](err)
	_, partial := errors.AsType[*extensionpkg.DependenciesInstalledError](err)
	return gated || partial
}

// extensionDependencyGatePayload maps the trust and gateway gates a dependency artifact can fail.
func extensionDependencyGatePayload(err error) contract.ExtensionOperationErrorPayload {
	payload := contract.ExtensionOperationErrorPayload{Error: err.Error()}
	switch {
	case errors.Is(err, extensionpkg.ErrExtensionGatewayConfirmationRequired):
		payload.Code = diagnosticcontract.CodeExtensionGatewayConfirmRequired
		if confirmation, ok := errors.AsType[*extensionpkg.GatewayConfirmationRequiredError](err); ok {
			payload.CurrentDigest = strings.TrimSpace(confirmation.CurrentDigest)
		}
	case errors.Is(err, extensionpkg.ErrExtensionUnverifiedPolicyBlocked):
		payload.Code = diagnosticcontract.CodeExtensionUnverifiedPolicyBlocked
	case errors.Is(err, extensionpkg.ErrExtensionChecksumUnverified):
		payload.Code = diagnosticcontract.CodeExtensionChecksumUnverified
	case errors.Is(err, extensionpkg.ErrExtensionArchiveDigestMismatch):
		payload.Code = diagnosticcontract.CodeExtensionArchiveDigestMismatch
	default:
		payload.Code = diagnosticcontract.CodeExtensionInstallFailed
	}
	return payload
}

func extensionAcquisitionKindPayload(err error) (contract.ExtensionOperationErrorPayload, bool) {
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
		addExtensionInputsRequiredFields(&payload, err)
	case extensionErrorInputInvalid:
		payload.Code = diagnosticcontract.CodeExtensionInputInvalid
		if invalid, ok := errors.AsType[*extensionpkg.InputValidationError](err); ok {
			payload.InputID = invalid.InputID
		}
	case extensionErrorInputOverwriteDenied:
		payload.Code = diagnosticcontract.CodeExtensionInputOverwriteDenied
		if denied, ok := errors.AsType[*extensionpkg.InputOverwriteDeniedError](err); ok {
			payload.InputIDs = slices.Clone(denied.InputIDs)
		}
	case extensionErrorDependenciesRequired:
		payload.Code = diagnosticcontract.CodeExtensionDependenciesRequired
		if required, ok := errors.AsType[*extensionpkg.DependenciesRequiredError](err); ok {
			payload.Dependencies = slices.Clone(required.Dependencies)
			payload.RetryCommand = dependencyRetryCommand(required.Dependencies)
		}
	case extensionErrorDependencyGraphInvalid:
		payload.Code = diagnosticcontract.CodeExtensionDependencyGraphInvalid
		if invalid, ok := errors.AsType[*extensionpkg.DependencyGraphInvalidError](err); ok {
			payload.Reason, payload.Path = invalid.Reason, slices.Clone(invalid.Path)
			payload.Parents = slices.Clone(invalid.Parents)
		}
	default:
		return payload, false
	}
	payload.Error = err.Error()
	return payload, true
}

// dependencyRetryCommand names the update that unblocks the first version_unmet dependency; nothing auto-updates.
func dependencyRetryCommand(dependencies []contract.ExtensionDependencyPayload) string {
	for _, dependency := range dependencies {
		if dependency.RetryCommand != "" {
			return dependency.RetryCommand
		}
	}
	return ""
}

func addExtensionInputsRequiredFields(payload *contract.ExtensionOperationErrorPayload, err error) {
	payload.Code = diagnosticcontract.CodeExtensionInputsRequired
	if required, ok := errors.AsType[*extensionpkg.InputsRequiredError](err); ok {
		payload.Inputs, payload.MissingEnv = slices.Clone(required.MissingInputs), slices.Clone(required.MissingEnv)
		payload.Invalid = slices.Clone(required.InvalidInputs)
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
}
