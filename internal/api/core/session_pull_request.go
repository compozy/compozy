package core

import (
	"context"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) RefreshSessionPullRequest(c *gin.Context) {
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	info, err := h.Sessions.Status(c.Request.Context(), c.Param("session_id"))
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	if info == nil || info.Type == session.SessionTypeTriage || !scope.Matches(info.ProfileID) {
		c.JSON(http.StatusNotFound, contract.ErrorPayload{Code: "session_not_found", Error: "session not found"})
		return
	}
	refresher, ok := h.Sessions.(interface {
		RefreshSessionPullRequest(context.Context, *session.Info) (session.PullRequestProjection, error)
	})
	var view session.PullRequestProjection
	if ok {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		view, err = refresher.RefreshSessionPullRequest(ctx, info)
	}
	if view.Branch == "" && err == nil {
		c.JSON(
			http.StatusConflict,
			sessionPullRequestError(
				"session_branch_unavailable",
				"session has no branch (default branch, detached HEAD, or no git repository)",
				"",
			),
		)
		return
	}
	if err != nil && view.Lookup.ObservedAt.IsZero() {
		c.JSON(
			http.StatusBadGateway,
			sessionPullRequestError("forge_unavailable", "pull request refresh failed", view.Lookup.Cause),
		)
		return
	}
	payload := contract.SessionPullRequestResponse{SessionID: info.ID, Branch: view.Branch}
	payload.PullRequest, payload.PullRequestLookup = pullRequestPayload(view.PullRequestView)
	c.JSON(http.StatusOK, payload)
}

// DecorateSessionPullRequests performs one batch projection for an already-authorized page.
func DecorateSessionPullRequests(
	ctx context.Context,
	reader any,
	infos []*session.Info,
	payloads []contract.SessionPayload,
) error {
	projection, ok := reader.(interface {
		SessionPullRequests(context.Context, []*session.Info) (map[string]session.PullRequestProjection, error)
	})
	if !ok {
		return nil
	}
	views, err := projection.SessionPullRequests(ctx, infos)
	if err != nil {
		return err
	}
	for i := range payloads {
		view := views[payloads[i].ID]
		payloads[i].Branch = view.Branch
		if view.Branch == "" {
			continue
		}
		payloads[i].PullRequest, payloads[i].PullRequestLookup = pullRequestPayload(view.PullRequestView)
		var state session.PullRequestState
		if view.PullRequest != nil {
			state = view.PullRequest.State
		}
		payloads[i].BoardColumn = session.BoardColumnFor(payloads[i].Badge, state)
	}
	return nil
}

func pullRequestPayload(
	view session.PullRequestView,
) (*contract.SessionPullRequestPayload, *contract.SessionPullRequestLookupPayload) {
	lookup := &contract.SessionPullRequestLookupPayload{
		State: view.Lookup.State,
		Cause: view.Lookup.Cause,
		Stale: view.Lookup.Stale,
	}
	if lookup.State == "" {
		lookup.State = session.LookupPending
		lookup.Stale = true
	}
	if !view.Lookup.ObservedAt.IsZero() {
		lookup.ObservedAt = &view.Lookup.ObservedAt
	}
	pr := view.PullRequest
	if pr == nil {
		return nil, lookup
	}
	names := append([]string{}, pr.Checks.FailedNames...)
	return &contract.SessionPullRequestPayload{
		Provider: pr.Provider,
		Number:   pr.Number,
		URL:      pr.URL,
		State:    pr.State,
		Draft:    pr.Draft,
		HeadSHA:  pr.HeadSHA,
		Checks: contract.SessionPullRequestChecksPayload{
			State:       pr.Checks.State,
			Passed:      pr.Checks.Passed,
			Failed:      pr.Checks.Failed,
			Pending:     pr.Checks.Pending,
			FailedNames: names,
		},
		ReviewDecision: pr.ReviewDecision,
		Mergeable:      pr.Mergeable,
		ReadyToMerge:   pr.ReadyToMerge,
	}, lookup
}

func sessionPullRequestError(code, message, cause string) contract.SessionPullRequestErrorPayload {
	return contract.SessionPullRequestErrorPayload{Code: code, Error: message, Message: message, Cause: cause}
}
