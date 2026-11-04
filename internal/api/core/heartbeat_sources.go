package core

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat/source"
	"github.com/gin-gonic/gin"
)

type heartbeatScriptStore interface {
	source.ApprovalReader
	ApproveScript(context.Context, string, string, string, string, string, time.Time) error
	RevokeScript(context.Context, string, string, string) error
}

func (h *BaseHandlers) heartbeatSourceTarget(c *gin.Context) (authoredAgentTarget, heartbeatScriptStore, bool) {
	target, err := h.resolveAuthoredAgentTargetForRequest(c, authoredWorkspaceRefFromQuery(c), pathAgentName(c))
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return target, nil, false
	}
	if _, err := os.Stat(target.agentPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = source.Failure("agent_not_found", "agent definition was not found")
		}
		h.respondError(c, 404, err)
		return target, nil, false
	}
	if h.AgentInstances == nil {
		h.respondError(c, 503, errors.New("source storage unavailable"))
		return target, nil, false
	}
	storage, ok := h.AgentInstances.Store.(heartbeatScriptStore)
	if !ok {
		h.respondError(c, 503, errors.New("source storage unavailable"))
		return target, nil, false
	}
	return target, storage, true
}
func (h *BaseHandlers) GetAgentHeartbeatSources(c *gin.Context) {
	target, storage, ok := h.heartbeatSourceTarget(c)
	if !ok {
		return
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(target.agentPath))
	if err != nil {
		h.respondError(c, 404, err)
		return
	}
	result, err := HeartbeatSourceCatalog(
		c.Request.Context(),
		target.agentName,
		dir,
		target.packageOwned,
		source.CatalogScope{ProfileID: target.profileID, WorkspaceID: target.workspaceID},
		target.heartbeatConfig.Sources,
		storage,
		h.AgentInstances.SourceExtensions,
	)
	if err != nil {
		h.respondError(c, 500, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func HeartbeatSourceCatalog(
	ctx context.Context,
	agent, dir string,
	packageOwned bool,
	scope source.CatalogScope,
	cfg compozyconfig.HeartbeatSourcesConfig,
	storage source.ApprovalReader,
	poller source.ExtensionPoller,
) (contract.HeartbeatSourceCatalogPayload, error) {
	var err error
	result := contract.HeartbeatSourceCatalogPayload{
		Agent:          agent,
		ScriptsEnabled: cfg.ScriptsEnabled,
		Limits:         contract.HeartbeatSourceLimitsPayload{MaxPerAgent: cfg.MaxPerAgent, MaxItems: cfg.MaxItems},
		Kinds:          []source.KindDescriptor{},
		Scripts:        []contract.HeartbeatSourceScriptPayload{},
	}
	if poller != nil {
		result.Kinds, err = poller.SourceKinds(ctx, scope)
		if err != nil {
			return result, err
		}
	}
	if packageOwned {
		return result, nil
	}
	entries, err := os.ReadDir(filepath.Join(dir, "sources"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !source.ValidScript(entry.Name()) {
			continue
		}
		_, hash, readErr := source.InspectScript(dir, entry.Name())
		if readErr != nil {
			continue
		}
		approved, err := storage.ApprovedHash(ctx, scope.ProfileID, dir, entry.Name())
		if err != nil {
			return result, err
		}
		info, err := os.Stat(filepath.Join(dir, "sources", entry.Name()))
		if err != nil {
			continue
		}
		row := contract.HeartbeatSourceScriptPayload{
			Name:       entry.Name(),
			Executable: info.Mode()&0111 != 0,
			SHA256:     hash,
			Approved:   hash == approved,
		}
		if approved != "" {
			row.ApprovedSHA256 = new(approved)
		}
		result.Scripts = append(result.Scripts, row)
	}
	return result, nil
}

func (h *BaseHandlers) ApproveAgentHeartbeatScript(c *gin.Context) { h.mutateHeartbeatScript(c, false) }
func (h *BaseHandlers) RevokeAgentHeartbeatScript(c *gin.Context)  { h.mutateHeartbeatScript(c, true) }
func (h *BaseHandlers) mutateHeartbeatScript(c *gin.Context, revoke bool) {
	if hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		h.respondError(
			c,
			403,
			source.Failure("source_approval_operator_only", "only the operator can approve or revoke source scripts"),
		)
		return
	}
	target, storage, ok := h.heartbeatSourceTarget(c)
	if !ok {
		return
	}
	if target.packageOwned {
		h.respondError(c, 403, source.Failure("source_script_invalid", "extension scripts cannot be approved"))
		return
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(target.agentPath))
	if err != nil {
		h.respondError(c, 422, err)
		return
	}
	name := c.Param("script")
	if !source.ValidScript(name) {
		h.respondError(c, 422, source.Failure("source_script_invalid", "invalid script name"))
		return
	}
	if revoke {
		if err := storage.RevokeScript(c.Request.Context(), target.profileID, dir, name); err != nil {
			h.respondError(c, 500, err)
			return
		}
		c.Status(204)
		return
	}
	var req contract.HeartbeatScriptApprovalRequest
	if err := decodeAuthoredJSONBody(c, &req, false); err != nil {
		h.respondError(c, 422, err)
		return
	}
	_, hash, err := source.ReadScript(dir, name)
	if err != nil {
		h.respondError(c, 422, err)
		return
	}
	if req.SHA256 != hash {
		h.respondError(
			c,
			409,
			source.Failure("source_script_changed", "script contents changed; review and approve its current hash"),
		)
		return
	}
	now := time.Now().UTC()
	via := string(taskOriginKindForTransport(h.TransportName))
	if err := storage.ApproveScript(c.Request.Context(), target.profileID, dir, name, hash, via, now); err != nil {
		h.respondError(c, 500, err)
		return
	}
	c.JSON(200, contract.HeartbeatScriptApprovalPayload{Script: name, SHA256: hash, ApprovedAt: now})
}
