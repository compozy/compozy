package core

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) writeInstanceCatalogEvent(c *gin.Context, writer FlushWriter, event session.CatalogEvent, actor workspaceaccess.ActorRef) error {
	api := h.AgentInstances
	if api == nil {
		return nil
	}
	id := event.InstanceID
	if id == "" && event.SessionID != "" && api.Authority.Callers != nil {
		caller, err := api.Authority.Callers.InstanceCaller(c.Request.Context(), event.SessionID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		id = caller.InstanceID
	}
	if id == "" {
		return nil
	}
	instance, err := api.Store.Get(c.Request.Context(), id)
	if errors.Is(err, agentinstance.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	// A descendant can belong to another workspace; its stream must not reveal the owner's instance.
	if instance.ProfileID != event.ProfileID || instance.WorkspaceID != event.WorkspaceID {
		return nil
	}
	if err := api.Authority.Authorize(c.Request.Context(), actor, instance, "list"); err != nil {
		if errors.Is(err, agentinstance.ErrForbidden) {
			return nil
		}
		return err
	}
	payload, err := api.Payload(c.Request.Context(), instance)
	if err != nil {
		return err
	}
	return WriteSSE(writer, SSEMessage{ID: strconv.FormatInt(event.Sequence, 10), Name: "agent_instance.updated", Data: payload})
}
