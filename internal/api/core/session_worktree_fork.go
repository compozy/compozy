package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

var (
	errWorktreeForkConfirmationRequired = errors.New("api: worktree fork confirmation is required")
	errWorktreeForkTargetRequired       = errors.New("api: exactly one worktree fork target is required")
)

type sessionPromptStateReader interface {
	IsPrompting(string) bool
}

// ForkSessionToWorktree creates a fresh child session without changing the
// origin session or copying its transcript.
func (h *BaseHandlers) ForkSessionToWorktree(c *gin.Context) {
	var req contract.ForkSessionWorktreeRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("%s: decode worktree fork request: %w", h.transportName(), err))
		return
	}
	if err := validateWorktreeForkRequest(req); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	scope, sessionID, origin, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	promptState, ok := h.Sessions.(sessionPromptStateReader)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session prompt state is unavailable"))
		return
	}
	if h.SessionAcceptance == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: asynchronous session acceptance is unavailable"))
		return
	}
	forkAcceptance, ok := h.SessionAcceptance.(SessionWorktreeForkAcceptanceManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: atomic worktree fork acceptance is unavailable"))
		return
	}
	if err := validateWorktreeForkAvailability(origin, sessionID, promptState); err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	worktreeTarget, err := h.resolveWorktreeForkTarget(c.Request.Context(), scope.RegistryID, req)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	if err := validateWorktreeForkAvailability(origin, sessionID, promptState); err != nil {
		status := StatusForSessionError(err)
		err = errors.Join(err, h.rollbackMaterializedSessionWorktree(c.Request.Context(), worktreeTarget))
		h.respondError(c, status, err)
		return
	}

	created, err := forkAcceptance.CreateWorktreeForkAccepted(c.Request.Context(), origin.ID, session.CreateAcceptedOpts{
		Session: session.CreateOpts{
			AgentName: origin.AgentName,
			Workspace: scope.SessionWorkspaceID(),
			Worktree:  worktreeTarget.ID,
			Type:      session.SessionTypeUser,
			Lineage:   &store.SessionLineage{ParentSessionID: origin.ID},
		},
	})
	if err != nil {
		if errors.Is(err, session.ErrPromptInProgress) {
			err = fmt.Errorf("%w: %s", worktree.ErrSessionActive, origin.ID)
		}
		status := StatusForSessionError(err)
		err = errors.Join(err, h.rollbackMaterializedSessionWorktree(c.Request.Context(), worktreeTarget))
		h.respondError(c, status, err)
		return
	}
	c.JSON(http.StatusCreated, contract.SessionResponse{Session: SessionPayloadFromInfo(created)})
}

func validateWorktreeForkRequest(req contract.ForkSessionWorktreeRequest) error {
	if !req.Confirmed {
		return errWorktreeForkConfirmationRequired
	}
	hasExisting := strings.TrimSpace(req.Worktree) != ""
	hasNew := req.NewWorktree != nil
	if hasExisting == hasNew {
		return errWorktreeForkTargetRequired
	}
	return nil
}

func validateWorktreeForkAvailability(
	origin *session.Info,
	sessionID string,
	promptState sessionPromptStateReader,
) error {
	if origin == nil || origin.State != session.StateActive {
		return session.ErrSessionNotActive
	}
	if promptState.IsPrompting(sessionID) {
		return worktree.ErrSessionActive
	}
	return nil
}

func (h *BaseHandlers) resolveWorktreeForkTarget(
	ctx context.Context,
	workspaceID string,
	req contract.ForkSessionWorktreeRequest,
) (materializedSessionWorktree, error) {
	if target := strings.TrimSpace(req.Worktree); target != "" {
		return materializedSessionWorktree{ID: target}, nil
	}
	if h.Worktrees == nil {
		return materializedSessionWorktree{}, errors.New("api: worktree creation is unavailable")
	}
	created, err := h.Worktrees.CreateReady(ctx, workspaceID, worktree.CreateOptions{
		Name:   strings.TrimSpace(req.NewWorktree.Name),
		Origin: worktree.OriginManual,
	})
	if err != nil {
		return materializedSessionWorktree{}, err
	}
	return materializedSessionWorktree{ID: created.ID, WorkspaceID: workspaceID, Created: true}, nil
}
