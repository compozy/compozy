package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

const sessionProfileMoveCodePrefix = "session_move_to_profile_"

// SessionProfileMover copies a stopped user session into another profile and archives the original.
type SessionProfileMover interface {
	MoveToProfile(ctx context.Context, opts session.MoveToProfileOpts) (session.MoveToProfileResult, error)
}

// SessionProfileMoveErrorCode returns the stable error code of a move-to-profile refusal.
func SessionProfileMoveErrorCode(err error) string {
	refusal, ok := errors.AsType[*session.ProfileMoveError](err)
	if !ok || refusal == nil {
		return ""
	}
	return sessionProfileMoveCodePrefix + string(refusal.Reason)
}

// StatusForSessionProfileMoveError maps a refusal to 409 and other failures as session errors.
func StatusForSessionProfileMoveError(err error) int {
	if SessionProfileMoveErrorCode(err) != "" {
		return http.StatusConflict
	}
	return StatusForSessionError(err)
}

// MoveSessionToProfile copies a stopped user session into another profile and archives the original.
func (h *BaseHandlers) MoveSessionToProfile(c *gin.Context) {
	mover, ok := h.Sessions.(SessionProfileMover)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session profile move is unavailable"))
		return
	}
	scope, sessionID, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	profileScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.requireSessionInProfile(c, info, profileScope) {
		return
	}
	var req contract.MoveSessionToProfileRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c, http.StatusBadRequest, fmt.Errorf("%s: decode move to profile request: %w", h.transportName(), err),
		)
		return
	}
	target, err := h.resolveProfileMoveTarget(c, req.TargetProfile)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.bindProfileMoveActor(c, scope.SessionWorkspaceID(), target.ID) {
		return
	}
	result, err := mover.MoveToProfile(c.Request.Context(), session.MoveToProfileOpts{
		WorkspaceID: scope.SessionWorkspaceID(), ProfileID: info.ProfileID,
		SessionID: sessionID, TargetProfileID: target.ID,
	})
	if err != nil {
		h.respondError(c, StatusForSessionProfileMoveError(err), err)
		return
	}
	response, err := h.sessionProfileMoveResponse(c.Request.Context(), result)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// resolveProfileMoveTarget resolves the target by name to an active, available profile.
func (h *BaseHandlers) resolveProfileMoveTarget(c *gin.Context, name string) (profilepkg.Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return profilepkg.Profile{}, &profilepkg.Error{
			Code: "profile_invalid", Message: "target_profile is required",
			Action: "name the profile to move the session to", Cause: profilepkg.ErrInvalidInput,
		}
	}
	resolved, err := h.profileService().Resolve(c.Request.Context(), profilepkg.ResolveInput{
		Flag: name, Lens: profilepkg.Lens{Kind: profilepkg.SelectionLensGlobal},
	})
	if err != nil {
		return profilepkg.Profile{}, err
	}
	return resolved.Profile, nil
}

// bindProfileMoveActor admits an agent caller only through the workspace-access policy for a
// write to the target profile, and records it so the manager refuses its own session.
func (h *BaseHandlers) bindProfileMoveActor(c *gin.Context, workspaceID, targetProfileID string) bool {
	if !hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		return true
	}
	caller, ok := h.requireAgentCaller(c, "session.move_to_profile")
	if !ok {
		return false
	}
	if err := h.authorizeGrantedProfile(c, caller, workspaceID, targetProfileID, false); err != nil {
		h.respondError(c, StatusForAgentIdentityError(err), err)
		return false
	}
	c.Request = c.Request.WithContext(session.WithActingSession(c.Request.Context(), caller.Session.ID))
	return true
}

func (h *BaseHandlers) sessionProfileMoveResponse(
	ctx context.Context,
	result session.MoveToProfileResult,
) (contract.MoveSessionToProfileResponse, error) {
	copyPayload, err := h.sessionPayloadWithOptionalHealth(ctx, result.Copy, false)
	if err != nil {
		return contract.MoveSessionToProfileResponse{}, err
	}
	originalPayload, err := h.sessionPayloadWithOptionalHealth(ctx, result.Original, false)
	if err != nil {
		return contract.MoveSessionToProfileResponse{}, err
	}
	return contract.MoveSessionToProfileResponse{
		Session: copyPayload, Original: originalPayload, WorktreeID: result.WorktreeID, Branch: result.Branch,
	}, nil
}
