package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

const (
	archiveStoppedSurface            = "archive stopped sessions"
	archiveStoppedInvalidRequestCode = "invalid_request"
	archiveStoppedWorkspaceCode      = "workspace_not_found"
)

var errArchiveStoppedUnavailable = errors.New("api: archive stopped sessions is unavailable")

// SessionArchiveStoppedManager archives every eligible stopped thread of one scope.
type SessionArchiveStoppedManager interface {
	ArchiveStopped(ctx context.Context, req session.ArchiveStoppedRequest) (session.ArchiveStoppedReport, error)
}

// ArchiveStoppedSessions archives, or previews with dry_run, every eligible
// stopped thread of the selected profile in one workspace or all of them.
// Agents reach it through the native tool, not this operator surface.
func (h *BaseHandlers) ArchiveStoppedSessions(c *gin.Context) {
	if !h.requireOperatorSurface(c, archiveStoppedSurface) {
		return
	}
	archiver, ok := h.Sessions.(SessionArchiveStoppedManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errArchiveStoppedUnavailable)
		return
	}
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	var body contract.SessionArchiveStoppedRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		respondArchiveStoppedInvalid(c, fmt.Errorf("decode archive stopped request: %w", err))
		return
	}
	workspaceRef := strings.TrimSpace(body.WorkspaceID)
	if (workspaceRef != "") == body.AllWorkspaces {
		respondArchiveStoppedInvalid(c, errors.New("choose exactly one of workspace_id and all_workspaces"))
		return
	}
	request := session.ArchiveStoppedRequest{
		ProfileID:     scope.ProfileID,
		AllWorkspaces: body.AllWorkspaces,
		DryRun:        body.DryRun,
	}
	if workspaceRef != "" {
		workspaceID, status, err := h.archiveStoppedWorkspaceID(c.Request.Context(), workspaceRef)
		if err != nil {
			h.respondArchiveStoppedWorkspaceError(c, status, err)
			return
		}
		request.WorkspaceID = workspaceID
	}
	report, err := archiver.ArchiveStopped(c.Request.Context(), request)
	if err != nil {
		h.respondError(c, statusForArchiveStoppedError(err), err)
		return
	}
	c.JSON(http.StatusOK, ArchiveStoppedPayload(&report))
}

// archiveStoppedWorkspaceID resolves an unarchived workspace; an archived one
// is not found, as All projects leaves it out too.
func (h *BaseHandlers) archiveStoppedWorkspaceID(ctx context.Context, ref string) (string, int, error) {
	if h.Workspaces == nil {
		return "", http.StatusServiceUnavailable, errArchiveStoppedUnavailable
	}
	workspace, err := h.Workspaces.Get(ctx, ref)
	if err != nil {
		return "", StatusForWorkspaceError(err), err
	}
	if workspace.ArchivedAt != nil {
		return "", http.StatusNotFound, fmt.Errorf("%w: %s is archived", workspacepkg.ErrWorkspaceNotFound, ref)
	}
	return strings.TrimSpace(workspace.ID), http.StatusOK, nil
}

func (h *BaseHandlers) respondArchiveStoppedWorkspaceError(c *gin.Context, status int, err error) {
	if status == http.StatusNotFound {
		c.JSON(http.StatusNotFound, contract.ErrorPayload{Error: err.Error(), Code: archiveStoppedWorkspaceCode})
		return
	}
	h.respondError(c, status, err)
}

func respondArchiveStoppedInvalid(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, contract.ErrorPayload{Error: err.Error(), Code: archiveStoppedInvalidRequestCode})
}

func statusForArchiveStoppedError(err error) int {
	switch {
	case errors.Is(err, session.ErrArchiveStoppedInvalidRequest):
		return http.StatusBadRequest
	case errors.Is(err, session.ErrSessionArchiveUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// ArchiveStoppedPayload maps an Archive stopped report to its wire shape.
func ArchiveStoppedPayload(report *session.ArchiveStoppedReport) contract.SessionArchiveStoppedResponse {
	response := contract.SessionArchiveStoppedResponse{
		DryRun:               report.DryRun,
		Archived:             make([]contract.SessionArchiveStoppedThread, 0, len(report.Archived)),
		ArchivedThreadCount:  len(report.Archived),
		ArchivedSessionCount: report.ArchivedSessionCount(),
		SkipCounts:           make(map[string]int, len(report.SkipCounts)),
		Skipped:              make([]contract.SessionArchiveStoppedSkip, 0, len(report.Skipped)),
		SkippedTruncated:     report.SkippedTruncated,
		Failures:             make([]contract.SessionArchiveStoppedFailure, 0, len(report.Failures)),
		FailuresTruncated:    report.FailuresTruncated,
	}
	for _, thread := range report.Archived {
		response.Archived = append(response.Archived, contract.SessionArchiveStoppedThread{
			RootID:      thread.RootID,
			WorkspaceID: thread.WorkspaceID,
			Title:       thread.Title,
			SessionIDs:  append([]string{}, thread.SessionIDs...),
		})
	}
	for reason, count := range report.SkipCounts {
		response.SkipCounts[string(reason)] = count
	}
	for _, skip := range report.Skipped {
		response.Skipped = append(response.Skipped, contract.SessionArchiveStoppedSkip{
			RootID:      skip.RootID,
			WorkspaceID: skip.WorkspaceID,
			Title:       skip.Title,
			Reason:      string(skip.Reason),
		})
	}
	for _, failure := range report.Failures {
		response.Failures = append(response.Failures, contract.SessionArchiveStoppedFailure{
			RootID:      failure.RootID,
			WorkspaceID: failure.WorkspaceID,
			Title:       failure.Title,
			Code:        failure.Code,
			Message:     failure.Message,
		})
	}
	return response
}
