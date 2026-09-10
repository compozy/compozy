package core

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/observe"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

type attentionNotificationObserver interface {
	notifications.AttentionStore
	TaskAttentionItems(context.Context, observe.OverviewQuery) ([]observe.OverviewAttentionItem, error)
}

// AttentionNotifications lists unread occurrences across all workspaces and source profiles.
func (h *BaseHandlers) AttentionNotifications(c *gin.Context) {
	if !h.requireOperatorSurface(c, "notification inbox") {
		return
	}
	observer, scope, ok := h.attentionNotificationScope(c, "bell")
	if !ok {
		return
	}
	items, err := h.bellNotificationItems(c, observer)
	if err != nil {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	snapshot, unread, err := observer.CaptureAttentionSnapshot(c.Request.Context(), scope, ids)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	visible := make(map[string]bool, len(unread))
	for _, id := range unread {
		visible[id] = true
	}
	items = slices.DeleteFunc(items, func(item contract.AttentionNotificationPayload) bool { return !visible[item.ID] })
	slices.SortFunc(items, func(a, b contract.AttentionNotificationPayload) int {
		if order := b.OccurredAt.Compare(a.OccurredAt); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	response := contract.AttentionNotificationsResponse{Snapshot: snapshot, Total: len(items)}
	for _, item := range items {
		if item.Finished {
			response.Finished++
		} else {
			response.NeedsYou++
		}
	}
	response.Items = items[:min(len(items), 100)]
	c.JSON(http.StatusOK, response)
}

// AcknowledgeAttentionNotifications acknowledges exactly the frozen bell or Home population.
func (h *BaseHandlers) AcknowledgeAttentionNotifications(c *gin.Context) {
	if !h.requireOperatorSurface(c, "notification acknowledgement") {
		return
	}
	population := c.Query("surface")
	if population == "" {
		population = "bell"
	}
	if population != "bell" && population != "home" {
		h.respondError(c, http.StatusBadRequest, errors.New("api: notification surface must be bell or home"))
		return
	}
	observer, scope, ok := h.attentionNotificationScope(c, population)
	if !ok {
		return
	}
	var request contract.AcknowledgeAttentionRequest
	if err := decodeStrictBridgeJSON(c, &request); err != nil || request.Snapshot == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("api: valid notification snapshot is required"))
		return
	}
	if err := observer.AcknowledgeAttentionSnapshot(
		c.Request.Context(),
		scope,
		request.Snapshot,
		request.ID,
	); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, notifications.ErrAttentionSnapshotUnavailable) {
			status = http.StatusConflict
		}
		h.respondError(c, status, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) attentionNotificationScope(
	c *gin.Context, population string,
) (attentionNotificationObserver, notifications.AttentionScope, bool) {
	observer, ok := h.Observer.(attentionNotificationObserver)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: notification inbox is unavailable"))
		return nil, notifications.AttentionScope{}, false
	}
	query, ok := h.attentionOverviewQuery(c)
	if !ok {
		return nil, notifications.AttentionScope{}, false
	}
	scope := observe.OverviewAttentionScope(query)
	if population == "bell" {
		scope.Population = "bell"
	}
	return observer, scope, true
}

func (h *BaseHandlers) attentionOverviewQuery(c *gin.Context) (observe.OverviewQuery, bool) {
	actor, err := h.taskActorContext(c, taskActionOverview)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return observe.OverviewQuery{}, false
	}
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return observe.OverviewQuery{}, false
	}
	query := observe.OverviewQuery{ReadScope: readScope, Actor: actor.Actor}
	query.AcknowledgementProfileID = readScope.ProfileID
	if readScope.AllProfiles {
		query.AcknowledgementProfileID = store.DefaultProfileID
	}
	if name := c.Query("receipt_profile"); name != "" {
		if !h.requireOperatorSurface(c, "notification receipt profile") {
			return observe.OverviewQuery{}, false
		}
		if h.Profiles == nil {
			if name != profileDefaultName {
				h.respondError(c, http.StatusServiceUnavailable, errors.New("api: profile service is unavailable"))
				return observe.OverviewQuery{}, false
			}
			query.AcknowledgementProfileID = store.DefaultProfileID
		} else {
			resolved, err := h.profileService().Resolve(c.Request.Context(), profilepkg.ResolveInput{
				Flag: name, Lens: profilepkg.Lens{Kind: profilepkg.SelectionLensGlobal},
			})
			if err != nil {
				h.respondProfileReadScopeError(c, err)
				return observe.OverviewQuery{}, false
			}
			query.AcknowledgementProfileID = resolved.Profile.ID
		}
	}
	if !h.resolveOverviewWorkspace(c, &query) {
		return observe.OverviewQuery{}, false
	}
	return query, true
}
