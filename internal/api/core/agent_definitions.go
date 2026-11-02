package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentmemory"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/soul"
	"github.com/gin-gonic/gin"
)

var (
	errAgentDefinitionPackageDelete = errors.New(
		"api: package-owned agent definitions cannot be deleted; disable the extension instead",
	)
	errAgentDefinitionConflict        = errors.New("api: agent definition conflict")
	errAgentDefinitionInvalid         = errors.New("api: invalid agent definition request")
	errAgentDefinitionSyncUnavailable = errors.New("api: agent definition sync is unavailable")
)

// UpdateAgent replaces the effective authored definition after a digest CAS.
func (h *BaseHandlers) UpdateAgent(c *gin.Context) {
	startedAt := time.Now()
	var req contract.UpdateAgentRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode update agent request: %w", h.transportName(), err),
		)
		return
	}
	if !h.allowWorkspaceAccessWrite(c, req.Agent.WorkspaceAccess) {
		return
	}
	name := compozyconfig.NormalizeAgentName(c.Param("name"))
	if err := validateUpdateAgentRequest(name, req); err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	if h.AgentDefinitionSync == nil {
		h.respondError(c, http.StatusServiceUnavailable, errAgentDefinitionSyncUnavailable)
		return
	}
	profileScope, profileName, err := h.agentResourceProfile(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	resolved, err := h.resolveAgentDefinition(
		c.Request.Context(), req.Workspace, name, profileScope, profileName,
	)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	currentDigest, err := compozyconfig.AgentDefinitionDigest(resolved.Entry.Def)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	if currentDigest != strings.TrimSpace(req.ExpectedDigest) {
		h.respondError(c, http.StatusConflict, errors.Join(
			errAgentDefinitionConflict,
			fmt.Errorf("expected digest %q does not match current digest", req.ExpectedDigest),
		))
		return
	}
	draft, err := updateAgentDraft(req.Agent, resolved.Entry.Def)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	if err := validateAgentDraftRuntime(draft, &resolved.Config); err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	agent, err := compozyconfig.CreateAgentDefFile(resolved.Entry.Def.SourcePath, draft, true)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	syncStartedAt := time.Now()
	if err := h.AgentDefinitionSync.Sync(c.Request.Context()); err != nil {
		syncErr := fmt.Errorf("api: sync updated agent definition: %w", err)
		h.logAgentMutationFailure("update", agent.SourcePath, startedAt, time.Since(syncStartedAt), syncErr)
		h.respondError(c, http.StatusInternalServerError, syncErr)
		return
	}
	syncDuration := time.Since(syncStartedAt)
	entry := h.agentCatalogEntryFromDef(agent, resolved.OperationWorkspace)
	h.logAgentMutation("update", &entry, startedAt, syncDuration)
	c.JSON(http.StatusOK, contract.AgentResponse{
		Agent: AgentPayloadFromEntryWithConfig(&entry, &resolved.Config),
	})
}

func validateUpdateAgentRequest(routeName string, req contract.UpdateAgentRequest) error {
	requestedName := compozyconfig.NormalizeAgentName(req.Agent.Name)
	if err := compozyconfig.ValidateAuthoredAgentName(requestedName); err != nil {
		return errors.Join(errAgentDefinitionInvalid, err)
	}
	if requestedName != routeName {
		return errors.Join(
			errAgentDefinitionInvalid,
			fmt.Errorf("agent.name must equal route name %q", routeName),
		)
	}
	if strings.TrimSpace(req.ExpectedDigest) == "" {
		return errors.Join(errAgentDefinitionInvalid, errors.New("expected_digest is required"))
	}
	return nil
}

// DeleteAgent durably removes the effective authored directory and its scoped revision history.
func (h *BaseHandlers) DeleteAgent(c *gin.Context) {
	startedAt := time.Now()
	if h.AgentDefinitionSync == nil {
		h.respondError(c, http.StatusServiceUnavailable, errAgentDefinitionSyncUnavailable)
		return
	}
	name := compozyconfig.NormalizeAgentName(c.Param("name"))
	profileScope, profileName, err := h.agentResourceProfile(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	resolved, err := h.resolveAgentDefinition(
		c.Request.Context(), agentWorkspaceRef(c), name, profileScope, profileName,
	)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	if resolved.Entry.PackageOwned {
		h.respondError(c, http.StatusForbidden, errAgentDefinitionPackageDelete)
		return
	}
	if resolved.OperationWorkspace != "" &&
		(h.SoulHistoryPurger == nil || h.HeartbeatHistoryPurger == nil) {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			errors.New("api: agent definition history purgers are unavailable"),
		)
		return
	}
	if !h.allowAgentDefinitionDeletion(c, profileScope.ProfileID, resolved.OperationWorkspace, name) {
		return
	}
	unshadowGlobal := h.globalAgentTwinExists(name, resolved.Entry.Def.SourcePath)
	agentsRoot, err := h.agentDefinitionAgentsRoot(&resolved)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	if err := h.purgeAgentDefinitionHistory(c.Request.Context(), &resolved); err != nil {
		h.logAgentMutationFailure("delete", resolved.Entry.Def.SourcePath, startedAt, 0, err)
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	if err := h.retireAgentDefinitionMemory(&resolved, profileScope.ProfileID, name, startedAt); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	if err := compozyconfig.DeleteAgentDefinition(agentsRoot, resolved.Entry.Def.SourcePath); err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	var mutationErr error
	syncStartedAt := time.Now()
	if syncErr := h.AgentDefinitionSync.Sync(c.Request.Context()); syncErr != nil {
		mutationErr = errors.Join(mutationErr, fmt.Errorf("api: sync deleted agent definition: %w", syncErr))
	}
	syncDuration := time.Since(syncStartedAt)
	if mutationErr != nil {
		h.logAgentMutationFailure(
			"delete",
			resolved.Entry.Def.SourcePath,
			startedAt,
			syncDuration,
			mutationErr,
		)
		h.respondError(c, http.StatusInternalServerError, mutationErr)
		return
	}
	response := contract.DeleteAgentResponse{Name: name, Origin: resolved.Entry.Origin}
	if resolved.Entry.Origin == contract.AgentOriginWorkspace && unshadowGlobal {
		response.UnshadowedOrigin = contract.AgentOriginGlobal
	}
	h.logAgentDeleteMutation(&resolved.Entry, startedAt, syncDuration, response.UnshadowedOrigin != "")
	c.JSON(http.StatusOK, response)
}

// DuplicateAgent clones the effective authored directory under a new name.
func (h *BaseHandlers) DuplicateAgent(c *gin.Context) {
	startedAt := time.Now()
	var req contract.DuplicateAgentRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode duplicate agent request: %w", h.transportName(), err),
		)
		return
	}
	if h.AgentDefinitionSync == nil {
		h.respondError(c, http.StatusServiceUnavailable, errAgentDefinitionSyncUnavailable)
		return
	}
	if err := compozyconfig.ValidateAuthoredAgentName(compozyconfig.NormalizeAgentName(req.Name)); err != nil {
		wrapped := errors.Join(errAgentDefinitionInvalid, err)
		h.respondError(c, statusForAgentDefinitionError(wrapped), wrapped)
		return
	}
	profileScope, profileName, err := h.agentResourceProfile(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	source, err := h.resolveAgentDefinition(
		c.Request.Context(), req.Workspace, c.Param("name"), profileScope, profileName,
	)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	target, err := h.duplicateAgentTarget(c.Request.Context(), req, &source, profileName)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	draft, err := duplicateAgentDraft(source.Entry.Def, req)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	if !h.allowWorkspaceAccessWrite(c, draft.WorkspaceAccess) {
		return
	}
	if err := validateAgentDraftRuntime(draft, &target.Config); err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	agent, err := compozyconfig.DuplicateAgentDefinition(source.Entry.Def, target.Path, draft)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	syncStartedAt := time.Now()
	if err := h.AgentDefinitionSync.Sync(c.Request.Context()); err != nil {
		syncErr := fmt.Errorf("api: sync duplicated agent definition: %w", err)
		h.logAgentMutationFailure("duplicate", agent.SourcePath, startedAt, time.Since(syncStartedAt), syncErr)
		h.respondError(c, http.StatusInternalServerError, syncErr)
		return
	}
	syncDuration := time.Since(syncStartedAt)
	entry := AgentCatalogEntry{Def: agent, Origin: target.Origin, WorkspaceID: target.WorkspaceID}
	h.logAgentMutation("duplicate", &entry, startedAt, syncDuration)
	c.JSON(http.StatusCreated, contract.AgentResponse{
		Agent: AgentPayloadFromEntryWithConfig(&entry, &target.Config),
	})
}

func updateAgentDraft(
	payload contract.CreateAgentPayload,
	current compozyconfig.AgentDef,
) (compozyconfig.AgentDefinitionDraft, error) {
	draft, err := createAgentDraftFromRequest(contract.CreateAgentRequest{Agent: payload})
	if err != nil {
		return compozyconfig.AgentDefinitionDraft{}, err
	}
	currentDraft, err := authoredAgentDraftFromDef(current)
	if err != nil {
		return compozyconfig.AgentDefinitionDraft{}, err
	}
	if payload.DisplayName == nil {
		draft.DisplayName = currentDraft.DisplayName
	}
	if payload.Description == nil {
		draft.Description = currentDraft.Description
	}
	if payload.Kind == "" {
		draft.Kind = currentDraft.Kind
	}
	if (compozyconfig.AgentDef{Kind: draft.Kind}).EffectiveKind() != current.EffectiveKind() {
		return compozyconfig.AgentDefinitionDraft{}, compozyconfig.ErrAgentKindImmutable
	}
	if payload.Appearance == nil {
		draft.Appearance = currentDraft.Appearance
	}
	// Structural comparison permits unchanged future appearance blocks to round-trip.
	if !compozyconfig.EqualAgentAppearance(draft.Appearance, currentDraft.Appearance) {
		if err := compozyconfig.ValidateAgentAppearance(draft.Appearance); err != nil {
			return compozyconfig.AgentDefinitionDraft{}, errors.Join(compozyconfig.ErrInvalidAgentDefinition, err)
		}
	}
	draft.MCPServers = currentDraft.MCPServers
	draft.Hooks = currentDraft.Hooks
	return draft, nil
}

func authoredAgentDraftFromDef(agent compozyconfig.AgentDef) (compozyconfig.AgentDefinitionDraft, error) {
	contents, err := os.ReadFile(agent.SourcePath)
	if err != nil {
		return compozyconfig.AgentDefinitionDraft{}, fmt.Errorf(
			"api: read authored agent definition %q: %w",
			agent.SourcePath,
			err,
		)
	}
	authored, err := compozyconfig.ParseAgentDef(contents)
	if err != nil {
		return compozyconfig.AgentDefinitionDraft{}, fmt.Errorf(
			"api: parse authored agent definition %q: %w",
			agent.SourcePath,
			err,
		)
	}
	return compozyconfig.AgentDefinitionDraftFromDef(authored), nil
}

func (h *BaseHandlers) purgeAgentDefinitionHistory(
	ctx context.Context,
	resolved *resolvedAgentDefinition,
) error {
	workspaceID := strings.TrimSpace(resolved.OperationWorkspace)
	if workspaceID == "" && resolved.ProfileID == "" {
		return nil
	}
	if h.SoulHistoryPurger == nil || h.HeartbeatHistoryPurger == nil {
		return errors.New("api: agent definition history purgers are unavailable")
	}
	name := resolved.Entry.Def.Name
	sourcePath := resolved.Entry.Def.SourcePath
	if err := h.SoulHistoryPurger.PurgeAgentHistory(
		ctx,
		soul.WorkspaceRef{WorkspaceID: workspaceID, ProfileID: resolved.ProfileID},
		name,
		sourcePath,
	); err != nil {
		return err
	}
	if err := h.HeartbeatHistoryPurger.PurgeAgentHistory(
		ctx,
		heartbeat.WorkspaceRef{WorkspaceID: workspaceID, ProfileID: resolved.ProfileID},
		name,
		sourcePath,
	); err != nil {
		return err
	}
	return nil
}

func (h *BaseHandlers) logAgentMutation(
	operation string,
	entry *AgentCatalogEntry,
	startedAt time.Time,
	syncDuration time.Duration,
) {
	attributes := []any{
		handlersOperationKey, "agent." + operation,
		"agent_name", entry.Def.Name,
		"origin", entry.Origin,
		workspaceIDKey, entry.WorkspaceID,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"sync_duration_ms", syncDuration.Milliseconds(),
		"outcome", "success",
	}
	h.Logger.Info("api: agent definition mutation completed", attributes...)
}

func (h *BaseHandlers) logAgentDeleteMutation(
	entry *AgentCatalogEntry,
	startedAt time.Time,
	syncDuration time.Duration,
	unshadowed bool,
) {
	h.Logger.Info(
		"api: agent definition mutation completed",
		handlersOperationKey, "agent.delete",
		"agent_name", entry.Def.Name,
		"origin", entry.Origin,
		"workspace_id", entry.WorkspaceID,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"sync_duration_ms", syncDuration.Milliseconds(),
		"unshadowed", unshadowed,
		"outcome", "success",
	)
}

func (h *BaseHandlers) logAgentMutationFailure(
	operation string,
	authoredPath string,
	startedAt time.Time,
	syncDuration time.Duration,
	err error,
) {
	h.Logger.Error(
		"api: agent definition mutation failed after authored state changed",
		handlersOperationKey, "agent."+operation,
		"authored_path", authoredPath,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"sync_duration_ms", syncDuration.Milliseconds(),
		"outcome", "error",
		"error", err,
	)
}

func (h *BaseHandlers) retireAgentDefinitionMemory(
	resolved *resolvedAgentDefinition,
	profileID, name string,
	at time.Time,
) error {
	if h.AgentMemory == nil {
		return nil
	}
	origin := resolved.Entry.MemoryOrigin
	if origin == "" {
		origin = "user"
		if resolved.Entry.Origin == contract.AgentOriginWorkspace {
			origin = "workspace-" + resolved.OperationWorkspace
		}
	}
	if err := h.AgentMemory.Retire(agentmemory.Key{ProfileID: profileID, Origin: origin, Agent: name}, at); err != nil {
		return fmt.Errorf("api: retire agent memory: %w", err)
	}
	return nil
}
