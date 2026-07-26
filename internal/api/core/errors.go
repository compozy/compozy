package core

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/compozy/agh/internal/admission"
	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	diagnosticspkg "github.com/compozy/agh/internal/diagnostics"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/network"
	presetspkg "github.com/compozy/agh/internal/notifications/presets"
	"github.com/compozy/agh/internal/resources"
	skillmarketplace "github.com/compozy/agh/internal/skills/marketplace"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/vault"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

const (
	errorsInternalServerErrorValue = "internal server error"
	errorsUnknownErrorValue        = "unknown error"
)

type normalizedErrorStatus struct {
	status             int
	err                error
	maskInternalErrors bool
}

// ErrRequestBodyTooLarge is the shared transport sentinel for request bodies
// rejected by HTTP MaxBytesReader enforcement.
var ErrRequestBodyTooLarge = errors.New("request body too large")

// RespondError writes a transport error response, optionally masking internal error details.
func RespondError(c *gin.Context, status int, err error, maskInternalErrors bool) {
	normalized := normalizeErrorStatus(status, err, maskInternalErrors)
	c.JSON(
		normalized.status,
		errorPayloadForNormalizedStatus(
			normalized.status,
			normalized.err,
			normalized.maskInternalErrors,
		),
	)
}

// ErrorPayloadForError builds a redacted error payload for stream frames and other non-status envelopes.
func ErrorPayloadForError(err error) contract.ErrorPayload {
	message := errorsUnknownErrorValue
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	return errorPayloadForMessage(message, err)
}

// ErrorPayloadForStatus builds a redacted HTTP/UDS error payload for status.
func ErrorPayloadForStatus(status int, err error, maskInternalErrors bool) contract.ErrorPayload {
	normalized := normalizeErrorStatus(status, err, maskInternalErrors)
	return errorPayloadForNormalizedStatus(
		normalized.status,
		normalized.err,
		normalized.maskInternalErrors,
	)
}

func normalizeErrorStatus(status int, err error, maskInternalErrors bool) normalizedErrorStatus {
	if maxBytesErr, ok := errors.AsType[*http.MaxBytesError](err); ok && maxBytesErr != nil {
		status = http.StatusRequestEntityTooLarge
		err = ErrRequestBodyTooLarge
		maskInternalErrors = false
	}
	if errors.Is(err, admission.ErrDraining) {
		maskInternalErrors = false
	}
	return normalizedErrorStatus{
		status:             status,
		err:                err,
		maskInternalErrors: maskInternalErrors,
	}
}

func errorPayloadForNormalizedStatus(
	status int,
	err error,
	maskInternalErrors bool,
) contract.ErrorPayload {
	message := http.StatusText(status)
	switch {
	case maskInternalErrors && status >= http.StatusInternalServerError:
		if strings.TrimSpace(message) == "" {
			message = errorsInternalServerErrorValue
		}
	case err != nil && strings.TrimSpace(err.Error()) != "":
		message = err.Error()
	case strings.TrimSpace(message) == "":
		message = errorsUnknownErrorValue
	}

	return errorPayloadForMessage(message, err)
}

// StatusForSessionError maps session and workspace-domain errors to transport statuses.
func StatusForSessionError(err error) int {
	return statusForSessionError(err)
}

// StatusForWorkspaceError maps workspace-domain errors to transport statuses.
func StatusForWorkspaceError(err error) int {
	return statusForWorkspaceError(err)
}

// NewMemoryValidationError wraps a memory validation failure with the shared sentinel.
func NewMemoryValidationError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", memory.ErrValidation, err)
}

// StatusForVaultError maps vault-domain failures to transport statuses.
func StatusForVaultError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, vault.ErrSecretNotFound):
		return http.StatusNotFound
	case errors.Is(err, vault.ErrUnsupportedSecretRef),
		errors.Is(err, vault.ErrMissingSecret):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// StatusForMemoryError maps memory-domain errors to transport statuses.
func StatusForMemoryError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrMemoryUnsupported):
		return http.StatusNotImplemented
	case errors.Is(err, ErrMemoryRejected):
		return http.StatusUnprocessableEntity
	case errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, memory.ErrValidation):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// StatusForResourceError maps desired-state resource failures to transport statuses.
func StatusForResourceError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, resources.ErrPermissionDenied),
		errors.Is(err, resources.ErrDirectMutationNotAllowed):
		return http.StatusForbidden
	case errors.Is(err, resources.ErrConflict),
		errors.Is(err, resources.ErrSessionNotActive),
		errors.Is(err, resources.ErrStaleSourceVersion):
		return http.StatusConflict
	case errors.Is(err, resources.ErrPayloadTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, resources.ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, resources.ErrNotFound), errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, resources.ErrValidation),
		errors.Is(err, resources.ErrInvalidScopeBinding),
		errors.Is(err, resources.ErrCodecNotFound),
		errors.Is(err, resources.ErrCodecTypeMismatch):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// StatusForBridgeError maps bridge-domain and workspace-domain errors to transport statuses.
func StatusForBridgeError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, contract.ErrBridgeInstanceMismatch),
		errors.Is(err, bridgepkg.ErrInvalidBridgeSecretBinding),
		errors.Is(err, bridgepkg.ErrInvalidBridgeTaskSubscription),
		errors.Is(err, bridgepkg.ErrInvalidBridgeTarget):
		return http.StatusBadRequest
	case errors.Is(err, bridgepkg.ErrBridgeInstanceNotFound),
		errors.Is(err, bridgepkg.ErrBridgeTargetUnknown),
		errors.Is(err, bridgepkg.ErrBridgeSecretBindingNotFound),
		errors.Is(err, bridgepkg.ErrBridgeTaskSubscriptionNotFound),
		errors.Is(err, bridgepkg.ErrBridgeRouteNotFound),
		errors.Is(err, bridgepkg.ErrDeliveryNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceNotFound):
		return http.StatusNotFound
	case errors.Is(err, bridgepkg.ErrBridgeInstanceUnavailable),
		errors.Is(err, bridgepkg.ErrInvalidBridgeStateTransition),
		errors.Is(err, bridgepkg.ErrBridgeInstanceReadOnly):
		return http.StatusConflict
	case errors.Is(err, bridgepkg.ErrBridgeTargetAmbiguous):
		return http.StatusUnprocessableEntity
	case errors.Is(err, bridgepkg.ErrDeliveryQueueSaturated),
		errors.Is(err, bridgepkg.ErrDeliveryTransportUnavailable),
		errors.Is(err, bridgepkg.ErrBridgeControlTransportUnavailable),
		errors.Is(err, bridgepkg.ErrBridgeTargetDirectoryUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// StatusForNotificationPresetError maps notification preset failures to transport statuses.
func StatusForNotificationPresetError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, presetspkg.ErrInvalidPreset):
		return http.StatusBadRequest
	case errors.Is(err, presetspkg.ErrPresetNotFound):
		return http.StatusNotFound
	case errors.Is(err, presetspkg.ErrPresetDuplicateName):
		return http.StatusConflict
	case errors.Is(err, presetspkg.ErrPresetBuiltIn):
		return http.StatusConflict
	case errors.Is(err, bridgepkg.ErrBridgeTargetAmbiguous):
		return http.StatusUnprocessableEntity
	case errors.Is(err, bridgepkg.ErrBridgeInstanceUnavailable),
		errors.Is(err, bridgepkg.ErrDeliveryQueueSaturated),
		errors.Is(err, bridgepkg.ErrDeliveryTransportUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// ErrSkillNotFound is the sentinel for a missing skill.
var ErrSkillNotFound = errors.New("skill not found")

// ErrSkillValidation is the sentinel for skill request validation failures.
var ErrSkillValidation = errors.New("skill validation error")

// ErrSkillUnprocessable is the sentinel for semantically invalid skill layers.
var ErrSkillUnprocessable = errors.New("skill unprocessable")

// ErrAutomationValidation is the sentinel for automation request validation failures.
var ErrAutomationValidation = errors.New("automation validation error")

// ErrNetworkValidation is the sentinel for malformed network control-plane requests.
var ErrNetworkValidation = errors.New("network validation error")

// ErrModelCatalogValidation is the sentinel for malformed model catalog requests.
var ErrModelCatalogValidation = errors.New("model catalog validation error")

// ErrModelCatalogUnavailable reports that the daemon model catalog surface is not configured.
var ErrModelCatalogUnavailable = errors.New("model catalog service unavailable")

// StatusForSkillError maps skill-domain errors to transport statuses.
func StatusForSkillError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrSkillNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrSkillValidation):
		return http.StatusBadRequest
	case errors.Is(err, ErrSkillUnprocessable):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// StatusForSkillMarketplaceError maps skill marketplace lifecycle failures to transport statuses.
func StatusForSkillMarketplaceError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, skillmarketplace.ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, skillmarketplace.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, skillmarketplace.ErrNotMarketplace):
		return http.StatusUnprocessableEntity
	case errors.Is(err, skillmarketplace.ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, skillmarketplace.ErrNotConfigured):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// NewAutomationValidationError wraps an automation validation failure with the shared sentinel.
func NewAutomationValidationError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrAutomationValidation, err)
}

// StatusForAutomationError maps automation-domain failures to transport statuses.
func StatusForAutomationError(err error) int {
	var maxBytesErr *http.MaxBytesError
	switch {
	case err == nil:
		return http.StatusOK
	case errors.As(err, &maxBytesErr):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, ErrAutomationValidation):
		return http.StatusBadRequest
	case errors.Is(err, automationpkg.ErrDaemonLifecycleCommandBlocked):
		return http.StatusBadRequest
	case errors.Is(err, automationpkg.ErrListCursorInvalid):
		return http.StatusBadRequest
	case errors.Is(err, looppkg.ErrValidation):
		return http.StatusUnprocessableEntity
	case errors.Is(err, automationpkg.ErrWebhookSecretRequired):
		return http.StatusBadRequest
	case errors.Is(err, automationpkg.ErrWebhookEndpointInvalid):
		return http.StatusBadRequest
	case errors.Is(err, automationpkg.ErrJobNotFound),
		errors.Is(err, automationpkg.ErrSuggestionNotFound),
		errors.Is(err, automationpkg.ErrTriggerNotFound),
		errors.Is(err, automationpkg.ErrRunNotFound),
		errors.Is(err, automationpkg.ErrWebhookTriggerNotRegistered),
		errors.Is(err, automationpkg.ErrJobOverlayNotFound),
		errors.Is(err, automationpkg.ErrTriggerOverlayNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, looppkg.ErrDefinitionNotFound):
		return http.StatusNotFound
	case errors.Is(err, automationpkg.ErrJobNameTaken),
		errors.Is(err, automationpkg.ErrSuggestionResolved),
		errors.Is(err, automationpkg.ErrSuggestionPendingCap),
		errors.Is(err, automationpkg.ErrTriggerNameTaken),
		errors.Is(err, automationpkg.ErrTriggerWebhookIDTaken),
		errors.Is(err, automationpkg.ErrConcurrencyLimitReached),
		errors.Is(err, automationpkg.ErrFireLimitReached),
		errors.Is(err, automationpkg.ErrDefinitionReadOnly),
		errors.Is(err, automationpkg.ErrOverlayRequiresConfigSource),
		errors.Is(err, automationpkg.ErrWebhookReplayDetected):
		return http.StatusConflict
	case errors.Is(err, automationpkg.ErrWebhookSignatureInvalid),
		errors.Is(err, automationpkg.ErrWebhookTimestampInvalid):
		return http.StatusUnauthorized
	case errors.Is(err, automationpkg.ErrManagerNotRunning):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// NewNetworkValidationError wraps a network request validation failure with the shared sentinel.
func NewNetworkValidationError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrNetworkValidation, err)
}

// StatusForNetworkError maps network-domain errors to transport statuses.
func StatusForNetworkError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrNetworkValidation):
		return http.StatusBadRequest
	case errors.Is(err, store.ErrNetworkCursorInvalid):
		return http.StatusBadRequest
	case errors.Is(err, network.ErrLocalPeerNotFound),
		errors.Is(err, network.ErrTargetPeerNotFound),
		errors.Is(err, store.ErrNetworkConversationNotFound),
		errors.Is(err, sql.ErrNoRows):
		return http.StatusNotFound
	case errors.Is(err, store.ErrNetworkDirectRoomCollision),
		errors.Is(err, store.ErrNetworkWorkContainerMismatch),
		errors.Is(err, store.ErrNetworkWorkClosed):
		return http.StatusConflict
	case errors.Is(err, network.ErrMissingField),
		errors.Is(err, network.ErrInvalidField),
		errors.Is(err, network.ErrInvalidKind),
		errors.Is(err, network.ErrInvalidBody),
		errors.Is(err, network.ErrExpired),
		errors.Is(err, network.ErrReplayTooOld),
		errors.Is(err, network.ErrLegacyFieldRejected):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// NewModelCatalogValidationError wraps a model catalog request validation failure.
func NewModelCatalogValidationError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrModelCatalogValidation, err)
}

// StatusForModelCatalogError maps model catalog failures to transport statuses.
func StatusForModelCatalogError(err error) int {
	var maxBytesErr *http.MaxBytesError
	switch {
	case err == nil:
		return http.StatusOK
	case errors.As(err, &maxBytesErr):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, ErrModelCatalogValidation),
		errors.Is(err, modelcatalog.ErrSourceNotRegistered):
		return http.StatusBadRequest
	case errors.Is(err, ErrModelCatalogUnavailable),
		errors.Is(err, modelcatalog.ErrAllSourcesFailed):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// RespondOpenAIError writes an OpenAI-compatible error response envelope.
func RespondOpenAIError(c *gin.Context, status int, err error, maskInternalErrors bool) {
	message := http.StatusText(status)
	switch {
	case maskInternalErrors && status >= http.StatusInternalServerError:
		if strings.TrimSpace(message) == "" {
			message = errorsInternalServerErrorValue
		}
	case err != nil && strings.TrimSpace(err.Error()) != "":
		message = err.Error()
	case strings.TrimSpace(message) == "":
		message = errorsUnknownErrorValue
	}
	message = diagnosticspkg.Redact(taskpkg.RedactClaimTokens(message))
	c.JSON(status, contract.OpenAIErrorResponse{
		Error: contract.OpenAIErrorPayload{
			Message: message,
			Type:    openAIErrorTypeForStatus(status),
			Param:   nil,
			Code:    openAIErrorCodeForStatus(status),
		},
	})
}

func openAIErrorTypeForStatus(status int) string {
	if status >= http.StatusInternalServerError {
		return "server_error"
	}
	return "invalid_request_error"
}

func openAIErrorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusRequestEntityTooLarge:
		return "request_too_large"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		if status >= http.StatusInternalServerError {
			return "internal_error"
		}
		return "api_error"
	}
}
