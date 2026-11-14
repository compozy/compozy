package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/sessioncleanup"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

const (
	historyCleanupSurface = "session history cleanup"
	// maxHistoryCleanupDays bounds older_than_days to a century.
	maxHistoryCleanupDays = 36500
)

var errHistoryCleanupUnavailable = errors.New("api: session history cleanup is unavailable")

// SessionHistoryCleanupManager plans, runs, and stops archived-history cleanups.
type SessionHistoryCleanupManager interface {
	PreviewHistoryCleanup(ctx context.Context, request sessioncleanup.Request) (sessioncleanup.Plan, error)
	StartHistoryCleanup(
		ctx context.Context,
		request sessioncleanup.Request,
	) (sessioncleanup.Run, sessioncleanup.Plan, error)
	HistoryCleanupRun(profileID string) (sessioncleanup.Run, bool, error)
	// StopHistoryCleanup stops the profile's running cleanup; a non-empty runID stops only that run.
	StopHistoryCleanup(ctx context.Context, profileID, runID string) (sessioncleanup.Run, error)
}

// PlanSessionHistoryCleanup previews a cleanup of the selected profile's
// archived sessions with dry_run, or starts it in the background (202).
func (h *BaseHandlers) PlanSessionHistoryCleanup(c *gin.Context) {
	cleaner, profileID, ok := h.historyCleanupScope(c)
	if !ok {
		return
	}
	var body contract.SessionHistoryCleanupRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("api: decode session history cleanup: %w", err))
		return
	}
	request, status, err := h.historyCleanupRequest(c.Request.Context(), profileID, body, h.nowUTC())
	if err != nil {
		h.respondError(c, status, err)
		return
	}
	if body.DryRun {
		plan, err := cleaner.PreviewHistoryCleanup(c.Request.Context(), request)
		if err != nil {
			h.respondError(c, statusForHistoryCleanupError(err), err)
			return
		}
		c.JSON(http.StatusOK, contract.SessionHistoryCleanupPreviewResponse{
			Plan: HistoryCleanupPlanPayload(plan, request.ArchivedBefore),
		})
		return
	}
	run, plan, err := cleaner.StartHistoryCleanup(c.Request.Context(), request)
	if err != nil {
		h.respondError(c, statusForHistoryCleanupError(err), err)
		return
	}
	c.JSON(http.StatusAccepted, contract.SessionHistoryCleanupStartResponse{
		Plan: HistoryCleanupPlanPayload(plan, request.ArchivedBefore),
		Run:  HistoryCleanupRunPayload(run),
	})
}

// GetSessionHistoryCleanup reads the selected profile's running or most recent cleanup.
func (h *BaseHandlers) GetSessionHistoryCleanup(c *gin.Context) {
	cleaner, profileID, ok := h.historyCleanupScope(c)
	if !ok {
		return
	}
	run, found, err := cleaner.HistoryCleanupRun(profileID)
	if err != nil {
		h.respondError(c, statusForHistoryCleanupError(err), err)
		return
	}
	response := contract.SessionHistoryCleanupRunResponse{}
	if found {
		payload := HistoryCleanupRunPayload(run)
		response.Run = &payload
	}
	c.JSON(http.StatusOK, response)
}

// StopSessionHistoryCleanup stops the selected profile's running cleanup after
// the deletes in flight; what it deleted stays deleted.
func (h *BaseHandlers) StopSessionHistoryCleanup(c *gin.Context) {
	cleaner, profileID, ok := h.historyCleanupScope(c)
	if !ok {
		return
	}
	run, err := cleaner.StopHistoryCleanup(c.Request.Context(), profileID, "")
	if err != nil {
		h.respondError(c, statusForHistoryCleanupError(err), err)
		return
	}
	payload := HistoryCleanupRunPayload(run)
	c.JSON(http.StatusOK, contract.SessionHistoryCleanupRunResponse{Run: &payload})
}

// historyCleanupScope admits operators only and resolves the one profile the
// cleanup belongs to; agents clean up through the approved native tool.
func (h *BaseHandlers) historyCleanupScope(c *gin.Context) (SessionHistoryCleanupManager, string, bool) {
	if !h.requireOperatorSurface(c, historyCleanupSurface) {
		return nil, "", false
	}
	cleaner, ok := h.Sessions.(SessionHistoryCleanupManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errHistoryCleanupUnavailable)
		return nil, "", false
	}
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return nil, "", false
	}
	return cleaner, scope.ProfileID, true
}

// historyCleanupRequest resolves the body's workspace and range into a plan request.
func (h *BaseHandlers) historyCleanupRequest(
	ctx context.Context,
	profileID string,
	body contract.SessionHistoryCleanupRequest,
	now time.Time,
) (sessioncleanup.Request, int, error) {
	request := sessioncleanup.Request{
		Scope: sessioncleanup.Scope{ProfileID: profileID, AllWorkspaces: body.AllWorkspaces},
		Selection: sessioncleanup.Selection{
			Everything: body.Delete.Everything,
			Children:   body.Delete.Children,
			Subagents:  body.Delete.Subagents,
		},
		RootIDs: body.SessionIDs,
	}
	workspaceRef := strings.TrimSpace(body.WorkspaceID)
	if (workspaceRef != "") == body.AllWorkspaces {
		return request, http.StatusBadRequest, errors.New("choose exactly one workspace_id or all_workspaces")
	}
	switch {
	case workspaceRef == workspaceaccess.UserSelector:
		request.Scope.WorkspaceID = workspaceaccess.UserSelector
	case workspaceRef != "":
		workspaceID, err := h.lookupWorkspaceID(ctx, workspaceRef)
		if err != nil {
			return request, StatusForWorkspaceError(err), err
		}
		request.Scope.WorkspaceID = workspaceID
	}
	// A confirmed count fixes the range where the dry run resolved it.
	if body.CountedAt != nil {
		countedAt := body.CountedAt.UTC()
		request.CountedAt = &countedAt
		now = countedAt
	}
	cutoff, err := historyCleanupCutoff(body, now)
	if err != nil {
		return request, http.StatusBadRequest, err
	}
	request.ArchivedBefore = cutoff
	return request, http.StatusOK, nil
}

// historyCleanupCutoff turns the range into the instant a thread's archive
// date must precede: older_than_days counts whole 24-hour days back from now.
func historyCleanupCutoff(body contract.SessionHistoryCleanupRequest, now time.Time) (*time.Time, error) {
	if body.OlderThanDays != nil && body.ArchivedBefore != nil {
		return nil, errors.New("choose at most one of older_than_days and archived_before")
	}
	if body.ArchivedBefore != nil {
		cutoff := body.ArchivedBefore.UTC()
		return &cutoff, nil
	}
	if body.OlderThanDays == nil {
		return nil, nil
	}
	days := *body.OlderThanDays
	if days < 1 || days > maxHistoryCleanupDays {
		return nil, fmt.Errorf("older_than_days must be between 1 and %d", maxHistoryCleanupDays)
	}
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	return &cutoff, nil
}

func statusForHistoryCleanupError(err error) int {
	switch {
	case errors.Is(err, sessioncleanup.ErrInvalidRequest):
		return http.StatusBadRequest
	case errors.Is(err, sessioncleanup.ErrRunActive):
		return http.StatusConflict
	case errors.Is(err, sessioncleanup.ErrNoActiveRun):
		return http.StatusNotFound
	case errors.Is(err, sessioncleanup.ErrShuttingDown), errors.Is(err, session.ErrSessionArchiveUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// HistoryCleanupPlanPayload maps a plan to its wire shape.
func HistoryCleanupPlanPayload(plan sessioncleanup.Plan, archivedBefore *time.Time) contract.SessionHistoryCleanupPlan {
	workspaces := make([]contract.SessionHistoryCleanupWorkspace, 0, len(plan.Workspaces))
	for _, item := range plan.Workspaces {
		workspaces = append(workspaces, contract.SessionHistoryCleanupWorkspace{
			WorkspaceID: item.WorkspaceID,
			Total:       item.Total,
		})
	}
	return contract.SessionHistoryCleanupPlan{
		ArchivedBefore: archivedBefore,
		CountedAt:      plan.CountedAt.UTC(),
		Counts: contract.SessionHistoryCleanupCounts{
			Sessions:  plan.Counts.Sessions,
			Children:  plan.Counts.Children,
			Subagents: plan.Counts.Subagents,
			Total:     plan.Counts.Total(),
		},
		KeptLive: plan.KeptLive,
		Choices: contract.SessionHistoryCleanupChoices{
			Everything: plan.Choices.Everything,
			Children:   plan.Choices.Children,
			Subagents:  plan.Choices.Subagents,
		},
		Workspaces: workspaces,
		Threads:    len(plan.Threads),
	}
}

// HistoryCleanupRunPayload maps a run snapshot to its wire shape.
func HistoryCleanupRunPayload(run sessioncleanup.Run) contract.SessionHistoryCleanupRun {
	failures := make([]contract.SessionHistoryCleanupFailure, 0, len(run.Failures))
	for _, failure := range run.Failures {
		failures = append(failures, contract.SessionHistoryCleanupFailure{
			SessionID:   failure.RootID,
			WorkspaceID: failure.WorkspaceID,
			Title:       failure.Title,
			Code:        string(failure.Code),
			Message:     failure.Message,
		})
	}
	return contract.SessionHistoryCleanupRun{
		ID:            run.ID,
		ProfileID:     run.ProfileID,
		WorkspaceID:   run.Scope.WorkspaceID,
		AllWorkspaces: run.Scope.AllWorkspaces,
		Delete: contract.SessionHistoryCleanupSelection{
			Everything: run.Selection.Everything,
			Children:   run.Selection.Children,
			Subagents:  run.Selection.Subagents,
		},
		ArchivedBefore: run.ArchivedBefore,
		CountedAt:      run.CountedAt.UTC(),
		State:          string(run.State),
		Total:          run.Total,
		Done:           run.Done(),
		Deleted:        run.Deleted,
		Skipped:        run.Skipped,
		FailedThreads:  run.FailedThreads,
		Failures:       failures,
		StartedAt:      run.StartedAt,
		FinishedAt:     run.FinishedAt,
	}
}
