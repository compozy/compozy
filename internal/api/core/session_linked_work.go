package core

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

type SessionLinkedWorkManager interface {
	SessionLinkedWork(context.Context, []*session.Info) (map[string]session.LinkedWorkProjection, error)
	LinkSessionWork(context.Context, string, store.SessionWorkLink, bool) error
	UnlinkSessionWork(context.Context, string, string, string) error
	RefreshSessionWorkLink(context.Context, string, string, string) error
}

type linkedWorkAction int

const (
	linkedWorkList linkedWorkAction = iota
	linkedWorkAdd
	linkedWorkRemove
	linkedWorkRefresh
)

func LinkedWorkErrorCode(err error) string {
	if errors.Is(err, session.ErrLinkedWorkForkUnsupported) {
		return "session_link_fork_unsupported"
	}
	if errors.Is(err, session.ErrLinkedWorkRepository) {
		return "session_link_repository_mismatch"
	}
	if errors.Is(err, session.ErrLinkedWorkInvalid) {
		return "session_link_invalid"
	}
	return "session_link_failed"
}

func linkedWorkItemPayload(item session.LinkedWorkItem) contract.SessionLinkedWorkItem {
	pr, lookup := pullRequestPayload(item.PullRequestView)
	return contract.SessionLinkedWorkItem{
		Repository:         item.Repository,
		Branch:             item.Branch,
		PRNumber:           item.PRNumber,
		WorktreeID:         item.WorktreeID,
		WorktreeName:       item.WorktreeName,
		WorktreeRemoved:    item.WorktreeRemoved,
		Provenance:         item.Provenance,
		RunsHere:           item.RunsHere,
		SourceSessionID:    item.SourceSessionID,
		SourceSessionTitle: item.SourceSessionTitle,
		PullRequest:        pr,
		PullRequestLookup:  lookup,
	}
}

func DecorateSessionLinkedWork(
	ctx context.Context,
	reader any,
	infos []*session.Info,
	payloads []contract.SessionPayload,
) error {
	manager, ok := reader.(SessionLinkedWorkManager)
	if !ok {
		return nil
	}
	views, err := manager.SessionLinkedWork(ctx, infos)
	if err != nil {
		return err
	}
	for i := range payloads {
		p := &payloads[i]
		view := views[p.ID]
		summary := &contract.SessionLinkedWorkSummary{
			Total:        view.Summary.Total,
			PullRequests: view.Summary.PullRequests,
			NeedsYou:     view.Summary.NeedsYou,
		}
		if view.Summary.Lead != nil {
			lead := linkedWorkItemPayload(*view.Summary.Lead)
			summary.Lead = &lead
		}
		p.LinkedWork = summary
		var state session.PullRequestState
		if p.PullRequest != nil {
			state = p.PullRequest.State
		}
		p.BoardColumn = session.BoardColumnWithLinkedWork(p.Badge, state, view.DirectOpen, view.Pending)
	}
	return nil
}

func (h *BaseHandlers) ListSessionLinkedWork(c *gin.Context) { h.sessionLinkedWork(c, linkedWorkList) }
func (h *BaseHandlers) AddSessionWorkLink(c *gin.Context)    { h.sessionLinkedWork(c, linkedWorkAdd) }
func (h *BaseHandlers) RemoveSessionWorkLink(c *gin.Context) {
	h.sessionLinkedWork(c, linkedWorkRemove)
}
func (h *BaseHandlers) RefreshSessionWorkLink(c *gin.Context) {
	h.sessionLinkedWork(c, linkedWorkRefresh)
}

func (h *BaseHandlers) sessionLinkedWork(c *gin.Context, action linkedWorkAction) {
	_, id, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	resolveScope := h.resolveProfileMutationScope
	if action == linkedWorkList {
		resolveScope = h.resolveProfileReadScope
	}
	scope, err := resolveScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.requireSessionInProfile(c, info, scope) {
		return
	}
	manager, ok := h.Sessions.(SessionLinkedWorkManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("session linked work unavailable"))
		return
	}
	if action != linkedWorkList {
		var request contract.SessionWorkLinkRequest
		if err := decodeStrictJSONBody(c, &request); err != nil {
			h.respondError(c, http.StatusBadRequest, err)
			return
		}
		if err := applyLinkedWorkAction(c, manager, id, action, request); err != nil {
			h.respondLinkedWorkError(c, err)
			return
		}
	}
	views, err := manager.SessionLinkedWork(c.Request.Context(), []*session.Info{info})
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	response := contract.SessionLinkedWorkResponse{Items: []contract.SessionLinkedWorkItem{}}
	for _, item := range views[id].Items {
		response.Items = append(response.Items, linkedWorkItemPayload(item))
	}
	c.JSON(http.StatusOK, response)
}

func applyLinkedWorkAction(
	c *gin.Context,
	manager SessionLinkedWorkManager,
	id string,
	action linkedWorkAction,
	request contract.SessionWorkLinkRequest,
) error {
	ctx := c.Request.Context()
	switch action {
	case linkedWorkRemove:
		return manager.UnlinkSessionWork(ctx, id, request.Repository, request.Branch)
	case linkedWorkRefresh:
		return manager.RefreshSessionWorkLink(ctx, id, request.Repository, request.Branch)
	default:
		return manager.LinkSessionWork(ctx, id, store.SessionWorkLink{
			Repository: request.Repository,
			Branch:     request.Branch,
			PRNumber:   request.PRNumber,
			Provenance: userScopeValue,
		}, false)
	}
}

// respondLinkedWorkError keeps 400 for refused input; anything else is the daemon's failure.
func (h *BaseHandlers) respondLinkedWorkError(c *gin.Context, err error) {
	if errors.Is(err, session.ErrLinkedWorkForkUnsupported) ||
		errors.Is(err, session.ErrLinkedWorkRepository) ||
		errors.Is(err, session.ErrLinkedWorkInvalid) {
		c.JSON(http.StatusBadRequest, contract.ErrorPayload{Code: LinkedWorkErrorCode(err), Error: err.Error()})
		return
	}
	h.respondError(c, StatusForSessionError(err), err)
}

func (h *BaseHandlers) SearchSessionWorkCandidates(c *gin.Context) {
	_, id, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.requireSessionInProfile(c, info, scope) {
		return
	}
	reader, ok := h.Sessions.(interface {
		SessionWorkCandidates(context.Context, string, string, string, int, int) (session.WorkCandidates, error)
	})
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("linked work discovery unavailable"))
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	response, err := reader.SessionWorkCandidates(c.Request.Context(), id, c.Query("q"), c.Query("kind"), page, limit)
	if err != nil {
		if errors.Is(err, session.ErrLinkedWorkRepository) || errors.Is(err, session.ErrLinkedWorkInvalid) {
			c.JSON(http.StatusBadRequest, contract.ErrorPayload{Code: LinkedWorkErrorCode(err), Error: err.Error()})
			return
		}
		h.respondError(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, response)
}
