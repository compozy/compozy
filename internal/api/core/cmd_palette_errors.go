package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/gin-gonic/gin"
)

const runtimeUnavailableErrorCode = "runtime_unavailable"

var errCmdPaletteServiceUnavailable = errors.New("cmd palette service is unavailable")

func (h *BaseHandlers) respondCmdPaletteError(
	c *gin.Context,
	workspaceID cmdpalette.WorkspaceID,
	err error,
) {
	status, payload := cmdPaletteErrorPayload(err)
	if status >= http.StatusInternalServerError && h.Logger != nil {
		h.Logger.Error("cmd palette request failed", "workspace_id", workspaceID, "error", err)
	}
	c.JSON(status, payload)
}

func cmdPaletteErrorPayload(err error) (int, contract.CmdPaletteError) {
	var invalid *cmdpalette.InvalidArgumentsError
	var unavailable *cmdpalette.UnavailableError
	var multiple *cmdpalette.MultipleClientsError
	switch {
	case errors.Is(err, cmdpalette.ErrCommandNotFound):
		return http.StatusNotFound, contract.CmdPaletteError{
			Error: "command_not_found", Message: trimCmdPaletteErrorPrefix(err.Error()),
		}
	case errors.As(err, &invalid):
		return http.StatusUnprocessableEntity, contract.CmdPaletteError{
			Error: "invalid_arguments", Fields: invalid.Fields,
		}
	case errors.As(err, &unavailable):
		return http.StatusPreconditionFailed, contract.CmdPaletteError{
			Error: "command_unavailable", Reason: unavailable.Reason,
		}
	case errors.Is(err, cmdpalette.ErrNoAttachedShell):
		return http.StatusPreconditionFailed, contract.CmdPaletteError{
			Error: "no_attached_shell", Message: "command changes UI state and needs an open CompozyOS shell",
		}
	case errors.As(err, &multiple):
		return http.StatusConflict, contract.CmdPaletteError{
			Error: "multiple_clients", Message: "multiple attached clients; pass client", Clients: multiple.Clients,
		}
	case errors.Is(err, cmdpalette.ErrAlreadyRunning):
		return http.StatusConflict, contract.CmdPaletteError{
			Error: "already_running", Message: trimCmdPaletteErrorPrefix(err.Error()),
		}
	case errors.Is(err, cmdpalette.ErrCannotDeferSecrets):
		return http.StatusUnprocessableEntity, contract.CmdPaletteError{
			Error: "cannot_defer_secrets", Message: "password arguments cannot wait for approval",
		}
	case errors.Is(err, cmdpalette.ErrClientUnauthorized):
		return http.StatusUnauthorized, contract.CmdPaletteError{
			Error: "client_unauthorized", Message: "client attachment token is invalid",
		}
	default:
		return http.StatusServiceUnavailable, contract.CmdPaletteError{
			Error: runtimeUnavailableErrorCode, Message: "cmd palette runtime is unavailable",
		}
	}
}

func trimCmdPaletteErrorPrefix(message string) string {
	return strings.TrimPrefix(message, "cmd palette: ")
}
