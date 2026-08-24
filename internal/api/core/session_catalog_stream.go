package core

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

const sessionCatalogChangedEvent = "session_catalog_changed"

// StreamSessionCatalog emits workspace-identified wake signals for catalog reconciliation.
func (h *BaseHandlers) StreamSessionCatalog(c *gin.Context) {
	if !h.requireOperatorSurface(c, "session catalog stream") {
		return
	}
	subscriber, ok := h.Sessions.(SessionCatalogEventSubscriber)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session catalog stream is required"))
		return
	}
	scope, err := h.parseSessionCatalogScope(c)
	if err != nil {
		if isProfileDomainError(err) {
			respondProfileError(c, err)
		} else {
			h.respondError(c, http.StatusBadRequest, err)
		}
		return
	}
	events, cancel, err := subscriber.SubscribeSessionCatalogEvents(c.Request.Context(), scope)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	defer cancel()
	owners, err := h.profileOwnerIdentities(c.Request.Context())
	if err != nil {
		respondProfileError(c, err)
		return
	}

	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	if err := WriteSSEComment(writer, "session catalog stream ready"); err != nil {
		h.logSSEWriteFailure(sessionCatalogChangedEvent, err)
		return
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case event, open := <-events:
			if !open {
				return
			}
			owner, found := owners[event.ProfileID]
			if !found {
				owners, err = h.profileOwnerIdentities(c.Request.Context())
				if err != nil {
					h.logSSEWriteFailure(sessionCatalogChangedEvent, err)
					return
				}
				owner, found = owners[event.ProfileID]
			}
			if !found {
				h.logSSEWriteFailure(
					sessionCatalogChangedEvent,
					fmt.Errorf("profile owner %q not found", event.ProfileID),
				)
				return
			}
			event.ProfileName = owner.Name
			name, data := sessionCatalogSSEMessage(event)
			if err := WriteSSE(writer, SSEMessage{
				ID:   strconv.FormatInt(event.Sequence, 10),
				Name: name,
				Data: data,
			}); err != nil {
				h.logSSEWriteFailure(name, err)
				return
			}
		}
	}
}

func (h *BaseHandlers) parseSessionCatalogScope(c *gin.Context) (session.CatalogScope, error) {
	readScope, allWorkspaces, workspaceID, err := h.parseSessionCatalogReadScope(c)
	if err != nil {
		return session.CatalogScope{}, err
	}
	afterSequence, err := parseLastEventID(c.GetHeader("Last-Event-ID"), h.transportName())
	if err != nil {
		return session.CatalogScope{}, err
	}
	replay := strings.TrimSpace(c.GetHeader("Last-Event-ID")) != ""
	if allWorkspaces {
		return session.CatalogScope{
			ReadScope:     readScope,
			AllWorkspaces: true,
			Replay:        replay,
			ReplayAfter:   afterSequence,
		}, nil
	}
	return session.CatalogScope{
		ReadScope:   readScope,
		WorkspaceID: workspaceID,
		Replay:      replay,
		ReplayAfter: afterSequence,
	}, nil
}

func (h *BaseHandlers) parseSessionCatalogReadScope(
	c *gin.Context,
) (store.ReadScope, bool, string, error) {
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		return store.ReadScope{}, false, "", err
	}
	allWorkspaces, workspaceID, err := h.parseSessionCatalogWorkspace(c)
	if err != nil {
		return store.ReadScope{}, false, "", err
	}
	return readScope, allWorkspaces, workspaceID, nil
}

func (h *BaseHandlers) parseSessionCatalogWorkspace(c *gin.Context) (bool, string, error) {
	allWorkspaces, err := parseBoolQuery(c, "all_workspaces")
	if err != nil {
		return false, "", err
	}
	workspaceRef := strings.TrimSpace(c.Query("workspace_id"))
	if (workspaceRef != "") == allWorkspaces {
		return false, "", fmt.Errorf(
			"%w: choose exactly one workspace_id or all_workspaces=true",
			session.ErrCatalogScopeInvalid,
		)
	}
	if allWorkspaces {
		return true, "", nil
	}
	workspaceID, err := h.lookupWorkspaceID(c.Request.Context(), workspaceRef)
	if err != nil {
		return false, "", fmt.Errorf("api: resolve session catalog workspace: %w", err)
	}
	return false, workspaceID, nil
}

func sessionCatalogSSEMessage(event session.CatalogEvent) (string, any) {
	if event.Name == session.CatalogEventNameOperatorNotification && event.OperatorNotification != nil {
		notification := event.OperatorNotification
		return string(session.CatalogEventNameOperatorNotification), contract.OperatorNotificationEventPayload{
			NotificationID: notification.NotificationID,
			SessionID:      notification.SessionID, ProfileID: event.ProfileID, ProfileName: event.ProfileName,
			WorkspaceID: notification.WorkspaceID,
			Title:       notification.Title, Body: notification.Body, At: notification.At,
		}
	}
	if event.Name == session.CatalogEventNameAttention && event.Attention != nil {
		return string(session.CatalogEventNameAttention), contract.SessionAttentionEventPayload{
			SessionID: event.Attention.SessionID, ProfileID: event.ProfileID, ProfileName: event.ProfileName,
			WorkspaceID: event.Attention.WorkspaceID,
			From:        event.Attention.From, To: event.Attention.To,
			Class: event.Attention.Class, At: event.Attention.At,
		}
	}
	return sessionCatalogChangedEvent, sessionCatalogEventPayload(event)
}

func sessionCatalogEventPayload(event session.CatalogEvent) contract.SessionCatalogEventPayload {
	return contract.SessionCatalogEventPayload{
		Kind:        string(event.Kind),
		ProfileID:   event.ProfileID,
		ProfileName: event.ProfileName,
		WorkspaceID: event.WorkspaceID,
		SessionID:   event.SessionID,
	}
}
