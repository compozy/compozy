package core

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/modelcatalog"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

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
	case errors.Is(err, agentidentity.ErrIdentityUnauthorized):
		return http.StatusForbidden
	case errors.Is(err, agentidentity.ErrIdentityRequired),
		errors.Is(err, agentidentity.ErrIdentityMismatch),
		errors.Is(err, agentidentity.ErrIdentityStale):
		return http.StatusUnauthorized
	case errors.Is(err, workspacepkg.ErrWorkspaceNotFound):
		return http.StatusNotFound
	case errors.Is(err, workspacepkg.ErrWorkspaceResolverUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
