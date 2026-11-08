package core

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/gin-gonic/gin"
)

type heartbeatAuthorityStore interface {
	ApprovedAuthority(context.Context, string, string) (string, error)
	ApproveAuthority(context.Context, string, string, string, string, time.Time) error
	RevokeAuthority(context.Context, string, string) error
}

func (h *BaseHandlers) heartbeatAuthorityStorage() (heartbeatAuthorityStore, error) {
	if h.AgentInstances != nil {
		if repo, ok := h.AgentInstances.Store.(heartbeatAuthorityStore); ok {
			return repo, nil
		}
	}
	return nil, errors.New("heartbeat authority storage unavailable")
}

func (h *BaseHandlers) authorityPayload(
	ctx context.Context,
	target authoredAgentTarget,
	policy heartbeat.ResolvedPolicy,
) (contract.HeartbeatAuthorityPayload, error) {
	digest, err := heartbeat.AuthorityDigest(
		policy.Frontmatter.CheckIns,
		heartbeat.AuthorityActions(policy.Frontmatter.Actions), target.readOnlyTools,
	)
	if err != nil {
		return contract.HeartbeatAuthorityPayload{}, err
	}
	result := contract.HeartbeatAuthorityPayload{
		ReadOnlyTools: target.readOnlyTools,
		Digest:        digest,
		Status:        "none",
		CheckIns:      policy.Frontmatter.CheckIns,
		Actions:       policy.Frontmatter.Actions,
	}
	if digest == "" {
		return result, nil
	}
	result.Status = "unapproved"
	repo, err := h.heartbeatAuthorityStorage()
	if err != nil {
		return result, err
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(target.agentPath))
	if err != nil {
		return result, err
	}
	approved, err := repo.ApprovedAuthority(ctx, target.profileID, dir)
	if approved == digest {
		result.Status = "approved"
	}
	return result, err
}
func (h *BaseHandlers) GetAgentHeartbeatAuthority(c *gin.Context) {
	h.mutateHeartbeatAuthority(c, "show")
}
func (h *BaseHandlers) ApproveAgentHeartbeatAuthority(c *gin.Context) {
	h.mutateHeartbeatAuthority(c, "approve")
}
func (h *BaseHandlers) RevokeAgentHeartbeatAuthority(c *gin.Context) {
	h.mutateHeartbeatAuthority(c, "revoke")
}
func (h *BaseHandlers) mutateHeartbeatAuthority(c *gin.Context, action string) {
	if action != "show" && hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		h.respondError(
			c,
			403,
			source.Failure(
				"heartbeat_authority_operator_only",
				"only the operator can approve or revoke check-in authority",
			),
		)
		return
	}
	target, err := h.resolveAuthoredAgentTargetForRequest(c, authoredWorkspaceRefFromQuery(c), pathAgentName(c))
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	policy, err := resolveHeartbeatAuthorityPolicy(c.Request.Context(), target)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	payload, err := h.authorityPayload(c.Request.Context(), target, policy)
	if err != nil {
		h.respondError(c, 503, err)
		return
	}
	if action == "show" {
		c.JSON(200, payload)
		return
	}
	repo, err := h.heartbeatAuthorityStorage()
	if err != nil {
		h.respondError(c, 503, err)
		return
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(target.agentPath))
	if err != nil {
		h.respondError(c, 422, err)
		return
	}
	if action == "revoke" {
		err = repo.RevokeAuthority(c.Request.Context(), target.profileID, dir)
	} else {
		var req contract.HeartbeatScriptApprovalRequest
		if err := decodeAuthoredJSONBody(c, &req, false); err != nil {
			h.respondError(c, 422, err)
			return
		}
		if req.SHA256 != payload.Digest {
			h.respondError(
				c,
				409,
				source.Failure("heartbeat_authority_changed", "check-in authority changed; review the current block"),
			)
			return
		}
		via := string(taskOriginKindForTransport(h.TransportName))
		if c.Request.UserAgent() == "compozy-cli" {
			via = "cli"
		} else if c.GetHeader("X-Compozy-Client-ID") != "" {
			via = "web"
		}
		err = repo.ApproveAuthority(c.Request.Context(), target.profileID, dir, payload.Digest, via, time.Now().UTC())
	}
	if err != nil {
		h.respondError(c, 500, err)
		return
	}
	c.Status(204)
}

func (h *BaseHandlers) heartbeatSelectorWarnings(
	c *gin.Context,
	target authoredAgentTarget,
	policy heartbeat.CheckInsPolicy,
) []contract.AuthoredContextDiagnosticPayload {
	if h.Tools == nil || len(policy.AllowWrites) == 0 {
		return nil
	}
	views, err := h.Tools.List(
		c.Request.Context(),
		toolspkg.Scope{
			Operator:  true,
			ProfileID: target.profileID,
			// The registry keys MCP sources by the registered workspace, not the identity ID.
			WorkspaceID: target.storageWorkspaceID(),
			AgentName:   target.agentName,
		},
	)
	if err != nil {
		return nil
	}
	access := make(map[string]bool, len(views))
	for _, view := range views {
		access[string(view.Descriptor.ID)] = toolspkg.AccessOf(view.Descriptor) == toolspkg.AccessRead
	}
	var result []contract.AuthoredContextDiagnosticPayload
	for _, diagnostic := range heartbeat.CheckInSelectorWarnings(policy, access) {
		result = append(
			result,
			contract.AuthoredContextDiagnosticPayload{
				Code:     diagnostic.Code,
				Severity: "warning",
				Message:  diagnostic.Message,
				Field:    diagnostic.Field,
			},
		)
	}
	return result
}

func resolveHeartbeatAuthorityPolicy(
	ctx context.Context,
	target authoredAgentTarget,
) (heartbeat.ResolvedPolicy, error) {
	policy, err := heartbeat.Resolve(
		ctx,
		heartbeat.ResolveRequest{
			AgentPath:     target.agentPath,
			WorkspaceRoot: target.authoringRoot(),
			Config:        target.heartbeatConfig,
			CheckInLimits: target.checkInLimits,
		},
	)
	if target.packageOwned {
		policy, err = target.resolvePackageOwnedHeartbeat(ctx, nil)
	}
	return policy, err
}

func (h *BaseHandlers) validateHeartbeatWriteAuthority(
	c *gin.Context,
	target authoredAgentTarget,
	req contract.HeartbeatPutByPathRequest,
) bool {
	if req.ApproveAuthoritySHA256 != "" {
		if hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
			h.respondError(
				c,
				403,
				source.Failure("heartbeat_authority_operator_only", "only the operator can approve check-in authority"),
			)
			return false
		}
		body := req.Body
		checked, err := h.HeartbeatAuthoring.Validate(
			c.Request.Context(),
			heartbeat.ValidateRequest{Target: target.heartbeatAuthoringTarget(), Body: &body},
		)
		if err != nil {
			h.respondError(c, StatusForHeartbeatError(err), err)
			return false
		}
		digest, err := heartbeat.AuthorityDigest(
			checked.Policy.Frontmatter.CheckIns,
			heartbeat.AuthorityActions(checked.Policy.Frontmatter.Actions), target.readOnlyTools,
		)
		if err != nil {
			h.respondError(c, 500, err)
			return false
		}
		if digest != req.ApproveAuthoritySHA256 {
			h.respondError(
				c,
				409,
				source.Failure("heartbeat_authority_changed", "check-in authority changed; review the current block"),
			)
			return false
		}
	}
	return true
}
func (h *BaseHandlers) approveSavedHeartbeatAuthority(c *gin.Context, target authoredAgentTarget, hash string) bool {
	if hash != "" {
		repo, storageErr := h.heartbeatAuthorityStorage()
		if storageErr != nil {
			h.respondError(c, 503, storageErr)
			return false
		}
		dir, pathErr := filepath.EvalSymlinks(filepath.Dir(target.agentPath))
		if pathErr != nil {
			h.respondError(c, 422, pathErr)
			return false
		}
		via := string(taskOriginKindForTransport(h.TransportName))
		if c.Request.UserAgent() == "compozy-cli" {
			via = "cli"
		} else if c.GetHeader("X-Compozy-Client-ID") != "" {
			via = "web"
		}
		if approvalErr := repo.ApproveAuthority(
			c.Request.Context(),
			target.profileID,
			dir,
			hash,
			via,
			time.Now().UTC(),
		); approvalErr != nil {
			h.respondError(c, 500, approvalErr)
			return false
		}
	}
	return true
}
