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
}

func LinkedWorkErrorCode(err error) string {
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
	return contract.SessionLinkedWorkItem{Repository: item.Repository, Branch: item.Branch, PRNumber: item.PRNumber, WorktreeID: item.WorktreeID, WorktreeName: item.WorktreeName, WorktreeRemoved: item.WorktreeRemoved, Provenance: item.Provenance, RunsHere: item.RunsHere, SourceSessionID: item.SourceSessionID, SourceSessionTitle: item.SourceSessionTitle, PullRequest: pr, PullRequestLookup: lookup}
}

func DecorateSessionLinkedWork(ctx context.Context, reader any, infos []*session.Info, payloads []contract.SessionPayload) error {
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
		summary := &contract.SessionLinkedWorkSummary{Total: view.Summary.Total, NeedsYou: view.Summary.NeedsYou}
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

func (h *BaseHandlers) ListSessionLinkedWork(c *gin.Context) { h.sessionLinkedWork(c, false, false) }
func (h *BaseHandlers) AddSessionWorkLink(c *gin.Context)    { h.sessionLinkedWork(c, true, false) }
func (h *BaseHandlers) RemoveSessionWorkLink(c *gin.Context) { h.sessionLinkedWork(c, true, true) }

func (h *BaseHandlers) sessionLinkedWork(c *gin.Context, mutate, unlink bool) {
	_, id, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	if mutate {
		scope, err := h.resolveProfileMutationScope(c)
		if err != nil {
			h.respondProfileReadScopeError(c, err)
			return
		}
		if !h.requireSessionInProfile(c, info, scope) {
			return
		}
	} else {
		scope, err := h.resolveProfileReadScope(c)
		if err != nil {
			h.respondProfileReadScopeError(c, err)
			return
		}
		if !h.requireSessionInProfile(c, info, scope) {
			return
		}
	}
	manager, ok := h.Sessions.(SessionLinkedWorkManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("session linked work unavailable"))
		return
	}
	if mutate {
		var request contract.SessionWorkLinkRequest
		if err := decodeStrictJSONBody(c, &request); err != nil {
			h.respondError(c, http.StatusBadRequest, err)
			return
		}
		var err error
		if unlink {
			err = manager.UnlinkSessionWork(c.Request.Context(), id, request.Repository, request.Branch)
		} else {
			err = manager.LinkSessionWork(c.Request.Context(), id, store.SessionWorkLink{Repository: request.Repository, Branch: request.Branch, PRNumber: request.PRNumber, Provenance: "user"}, false)
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, contract.ErrorPayload{Code: LinkedWorkErrorCode(err), Error: err.Error()})
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
		h.respondError(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, response)
}
