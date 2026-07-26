package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

const (
	coordinationActionRead          = "network_coordination.read"
	coordinationActionSet           = "network_coordination.set"
	coordinationActionSetInvitation = "network_coordination.set_invitation"
)

// GetNetworkCoordination returns one scope-explicit setting, invitation, and eligibility view.
func (h *BaseHandlers) GetNetworkCoordination(c *gin.Context) {
	workspaceID, ok := h.requireRouteWorkspaceID(c)
	if !ok {
		return
	}
	commands, ok := h.requireCoordinationCommands(c)
	if !ok {
		return
	}
	ref, err := coordinationRefFromValues(
		workspaceID,
		c.Query("scope"),
		c.Query("task_id"),
		c.Query("run_id"),
	)
	if err != nil {
		h.respondError(c, statusForNetworkCoordinationError(err), err)
		return
	}
	actor, err := h.taskActorContextForWorkspace(c, coordinationActionRead, workspaceID)
	if err != nil {
		h.respondError(c, statusForNetworkCoordinationError(err), err)
		return
	}
	view, err := commands.Get(c.Request.Context(), ref, actor)
	if err != nil {
		h.respondError(c, statusForNetworkCoordinationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkCoordinationResponse{Coordination: coordinationPayloadFromView(view)})
}

// PutNetworkCoordination compare-and-sets one future-run coordination scope.
func (h *BaseHandlers) PutNetworkCoordination(c *gin.Context) {
	workspaceID, ok := h.requireRouteWorkspaceID(c)
	if !ok {
		return
	}
	commands, ok := h.requireCoordinationCommands(c)
	if !ok {
		return
	}
	var req contract.PutNetworkCoordinationRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode coordination request: %w", h.transportName(), err),
		)
		return
	}
	if req.Enabled == nil {
		h.respondError(c, http.StatusBadRequest, errors.New("api: enabled is required"))
		return
	}
	if req.ExpectedRevision == nil {
		h.respondError(c, http.StatusBadRequest, errors.New("api: expected_revision is required"))
		return
	}
	ref, err := coordinationRefFromValues(workspaceID, req.Scope, req.TaskID, req.RunID)
	if err != nil {
		h.respondError(c, statusForNetworkCoordinationError(err), err)
		return
	}
	actor, err := h.taskActorContextForWorkspace(c, coordinationActionSet, workspaceID)
	if err != nil {
		h.respondError(c, statusForNetworkCoordinationError(err), err)
		return
	}
	view, err := commands.Set(c.Request.Context(), workspacepkg.SetCoordination{
		Ref:              ref,
		Enabled:          *req.Enabled,
		ExpectedRevision: *req.ExpectedRevision,
	}, actor)
	if err != nil {
		h.respondError(c, statusForNetworkCoordinationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkCoordinationResponse{Coordination: coordinationPayloadFromView(view)})
}

// PutNetworkCoordinationInvitation compare-and-sets one durable invitation disposition.
func (h *BaseHandlers) PutNetworkCoordinationInvitation(c *gin.Context) {
	workspaceID, ok := h.requireRouteWorkspaceID(c)
	if !ok {
		return
	}
	commands, ok := h.requireCoordinationCommands(c)
	if !ok {
		return
	}
	var req contract.PutNetworkCoordinationInvitationRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode invitation request: %w", h.transportName(), err),
		)
		return
	}
	if req.Dismissed == nil {
		h.respondError(c, http.StatusBadRequest, errors.New("api: dismissed is required"))
		return
	}
	if req.ExpectedRevision == nil {
		h.respondError(c, http.StatusBadRequest, errors.New("api: expected_revision is required"))
		return
	}
	ref, err := coordinationRefFromValues(workspaceID, req.Scope, req.TaskID, req.RunID)
	if err != nil {
		h.respondError(c, statusForNetworkCoordinationError(err), err)
		return
	}
	actor, err := h.taskActorContextForWorkspace(c, coordinationActionSetInvitation, workspaceID)
	if err != nil {
		h.respondError(c, statusForNetworkCoordinationError(err), err)
		return
	}
	view, err := commands.SetInvitation(c.Request.Context(), workspacepkg.SetInvitation{
		Ref:              ref,
		Dismissed:        *req.Dismissed,
		ExpectedRevision: *req.ExpectedRevision,
	}, actor)
	if err != nil {
		h.respondError(c, statusForNetworkCoordinationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkCoordinationResponse{Coordination: coordinationPayloadFromView(view)})
}

func (h *BaseHandlers) requireCoordinationCommands(
	c *gin.Context,
) (workspacepkg.CoordinationCommands, bool) {
	if h.Coordination == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: coordination service is unavailable"))
		return nil, false
	}
	return h.Coordination, true
}

func coordinationRefFromValues(
	workspaceID string,
	scope string,
	taskID string,
	runID string,
) (workspacepkg.CoordinationRef, error) {
	return workspacepkg.NormalizeCoordinationRef(workspacepkg.CoordinationRef{
		WorkspaceID: workspaceID,
		ScopeKind:   scope,
		TaskID:      taskID,
		RunID:       runID,
	})
}

func (h *BaseHandlers) requireRouteWorkspaceID(c *gin.Context) (string, bool) {
	workspaceRef := strings.TrimSpace(c.Param("workspace_id"))
	if workspaceRef == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("api: workspace_id is required"))
		return "", false
	}
	if h.Workspaces != nil {
		workspace, err := h.Workspaces.Get(c.Request.Context(), workspaceRef)
		if err != nil {
			h.respondError(c, StatusForWorkspaceError(err), err)
			return "", false
		}
		workspaceID := strings.TrimSpace(workspace.ID)
		if workspaceID == "" {
			h.respondError(c, http.StatusInternalServerError, errors.New("api: resolved workspace id is required"))
			return "", false
		}
		return workspaceID, true
	}
	return workspaceRef, true
}

func coordinationPayloadFromView(view workspacepkg.CoordinationView) contract.NetworkCoordinationPayload {
	setting := view.Setting
	payload := contract.NetworkCoordinationPayload{
		WorkspaceID: setting.WorkspaceID,
		Scope:       setting.ScopeKind,
		TaskID:      invitationTaskID(setting.ScopeKind, setting.ScopeID),
		Enabled:     setting.Enabled,
		Revision:    setting.Revision,
		UpdatedBy:   setting.UpdatedBy,
		Invitation:  invitationPayloadFromRow(view.Invitation),
		Eligibility: contract.NetworkCoordinationEligibilityPayload{
			Eligible:    view.Eligibility.Eligible,
			Reason:      view.Eligibility.Reason,
			RunID:       view.Eligibility.RunID,
			Coordinator: view.Eligibility.Coordinator,
			WorkerCount: view.Eligibility.WorkerCount,
		},
	}
	if !setting.UpdatedAt.IsZero() {
		updatedAt := setting.UpdatedAt.UTC()
		payload.UpdatedAt = &updatedAt
	}
	return payload
}

func invitationPayloadFromRow(
	row workspacepkg.CoordinationInvitation,
) *contract.NetworkCoordinationInvitationPayload {
	payload := &contract.NetworkCoordinationInvitationPayload{
		Scope:       row.ScopeKind,
		TaskID:      invitationTaskID(row.ScopeKind, row.ScopeID),
		Dismissed:   row.Dismissed,
		DismissedBy: row.DismissedBy,
	}
	if row.Dismissed && !row.DismissedAt.IsZero() {
		dismissedAt := row.DismissedAt.UTC()
		payload.DismissedAt = &dismissedAt
	}
	return payload
}

func invitationTaskID(scopeKind string, scopeID string) string {
	if scopeKind == workspacepkg.InvitationScopeTask {
		return scopeID
	}
	return ""
}

func statusForNetworkCoordinationError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, workspacepkg.ErrCoordinationScopeInvalid),
		errors.Is(err, taskpkg.ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, taskpkg.ErrPermissionDenied),
		errors.Is(err, agentidentity.ErrIdentityUnauthorized):
		return http.StatusForbidden
	case errors.Is(err, agentidentity.ErrIdentityRequired),
		errors.Is(err, agentidentity.ErrIdentityMismatch),
		errors.Is(err, agentidentity.ErrIdentityStale):
		return http.StatusUnauthorized
	case errors.Is(err, participation.ErrUnavailable),
		errors.Is(err, workspacepkg.ErrCoordinationConflict):
		return http.StatusConflict
	case errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, taskpkg.ErrTaskNotFound),
		errors.Is(err, taskpkg.ErrTaskRunNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
