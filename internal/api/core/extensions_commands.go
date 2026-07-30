package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

// ExtensionCommandService exposes the global contributed-command projection.
type ExtensionCommandService interface {
	Commands(context.Context, string) (contract.ExtensionCommandsResponse, error)
}

// ExtensionScopedCommandService exposes the trusted-workspace command projection.
type ExtensionScopedCommandService interface {
	CommandsScoped(
		context.Context,
		string,
		taskpkg.ActorContext,
	) (contract.ExtensionCommandsResponse, error)
}

// ExtensionCommands returns active command leaves and presentation groups.
func (h *BaseHandlers) ExtensionCommands(c *gin.Context) {
	service, ok := h.extensionService(c)
	if !ok {
		return
	}
	commands, ok := service.(ExtensionCommandService)
	if !ok {
		h.respondExtensionError(
			c,
			http.StatusServiceUnavailable,
			errors.New("api: extension service is not configured"),
		)
		return
	}
	filter := strings.TrimSpace(c.Query("extension"))
	var response contract.ExtensionCommandsResponse
	var err error
	if scoped, supportsScope := service.(ExtensionScopedCommandService); supportsScope && hasExtensionScopeRequest(c) {
		actor, actorOK := h.extensionScopedActorContext(c, "commands", false)
		if !actorOK {
			return
		}
		response, err = scoped.CommandsScoped(c.Request.Context(), filter, actor)
	} else {
		response, err = commands.Commands(c.Request.Context(), filter)
	}
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, response)
}
