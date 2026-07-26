package core

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	aghconfig "github.com/compozy/agh/internal/config"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	diagnosticspkg "github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/modelcatalog"
	settingspkg "github.com/compozy/agh/internal/settings"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

var (
	// ErrSettingsValidation is the sentinel for settings request validation failures.
	ErrSettingsValidation = errors.New("settings validation error")
	// ErrSettingsNotFound is the sentinel for missing settings resources.
	ErrSettingsNotFound = errors.New("settings not found")
	// ErrSettingsConflict is the sentinel for conflicting settings mutations or scope combinations.
	ErrSettingsConflict = errors.New("settings conflict")
	// ErrSettingsForbidden is the sentinel for settings operations rejected by transport policy.
	ErrSettingsForbidden = errors.New("settings forbidden")
)

// NewSettingsValidationError wraps a settings validation failure with the shared sentinel.
func NewSettingsValidationError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrSettingsValidation, err)
}

// NewSettingsNotFoundError wraps a missing settings resource failure with the shared sentinel.
func NewSettingsNotFoundError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrSettingsNotFound, err)
}

// NewSettingsConflictError wraps a settings conflict with the shared sentinel.
func NewSettingsConflictError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrSettingsConflict, err)
}

// NewSettingsForbiddenError wraps a settings forbidden failure with the shared sentinel.
func NewSettingsForbiddenError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrSettingsForbidden, err)
}

// StatusForSettingsError maps settings-domain failures to transport statuses.
func StatusForSettingsError(err error) int {
	if item, ok := diagnosticspkg.ItemFromError(err); ok {
		switch item.Code {
		case diagnosticcontract.CodeModelNotFound,
			diagnosticcontract.CodeReasoningEffortUnsupported:
			return http.StatusUnprocessableEntity
		}
	}
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrSettingsForbidden),
		errors.Is(err, settingspkg.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrSettingsValidation),
		errors.Is(err, settingspkg.ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, settingspkg.ErrUnprocessable):
		return http.StatusUnprocessableEntity
	case errors.Is(err, settingspkg.ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, ErrSettingsNotFound),
		errors.Is(err, settingspkg.ErrNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceRootMissing),
		errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, ErrSettingsConflict),
		errors.Is(err, settingspkg.ErrConflict),
		errors.Is(err, aghconfig.ErrUnsupportedTOMLMutation):
		return http.StatusConflict
	case errors.Is(err, modelcatalog.ErrAllSourcesFailed):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
