package core

import (
	"errors"
	"net/http"
	"os"

	compozyconfig "github.com/compozy/compozy/internal/config"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func statusForAgentDefinitionError(err error) int {
	switch {
	case errors.Is(err, compozyconfig.ErrAgentKindImmutable):
		return http.StatusConflict
	case errors.Is(err, compozyconfig.ErrAgentNameReserved):
		return http.StatusUnprocessableEntity
	case errors.Is(err, errAgentDefinitionInvalid),
		errors.Is(err, errCreateAgentRequestInvalid),
		errors.Is(err, compozyconfig.ErrInvalidAgentDefinition):
		return http.StatusBadRequest
	case errors.Is(err, errAgentDefinitionConflict),
		errors.Is(err, compozyconfig.ErrAgentDefinitionExists):
		return http.StatusConflict
	case errors.Is(err, compozyconfig.ErrAgentDefinitionNotFound),
		errors.Is(err, workspacepkg.ErrAgentNotAvailable),
		errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, errAgentDefinitionSyncUnavailable):
		return http.StatusServiceUnavailable
	default:
		return StatusForWorkspaceError(err)
	}
}
