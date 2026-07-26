package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/soul"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

var (
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
	name := aghconfig.NormalizeAgentName(c.Param("name"))
	requestedName := aghconfig.NormalizeAgentName(req.Agent.Name)
	if err := aghconfig.ValidateAuthoredAgentName(requestedName); err != nil {
		wrapped := errors.Join(errAgentDefinitionInvalid, err)
		h.respondError(c, statusForAgentDefinitionError(wrapped), wrapped)
		return
	}
	if requestedName != name {
		h.respondError(c, http.StatusBadRequest, errors.Join(
			errAgentDefinitionInvalid,
			fmt.Errorf("agent.name must equal route name %q", name),
		))
		return
	}
	if strings.TrimSpace(req.ExpectedDigest) == "" {
		h.respondError(c, http.StatusBadRequest, errors.Join(
			errAgentDefinitionInvalid,
			errors.New("expected_digest is required"),
		))
		return
	}
	if h.AgentDefinitionSync == nil {
		h.respondError(c, http.StatusServiceUnavailable, errAgentDefinitionSyncUnavailable)
		return
	}
	resolved, err := h.resolveAgentDefinition(c.Request.Context(), req.Workspace, name)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	currentDigest, err := aghconfig.AgentDefinitionDigest(resolved.Entry.Def)
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
	agent, err := aghconfig.CreateAgentDefFile(resolved.Entry.Def.SourcePath, draft, true)
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
	h.logAgentMutation("update", entry, startedAt, syncDuration)
	c.JSON(http.StatusOK, contract.AgentResponse{
		Agent: AgentPayloadFromEntryWithConfig(entry, &resolved.Config),
	})
}

// DeleteAgent durably removes the effective authored directory and its scoped revision history.
func (h *BaseHandlers) DeleteAgent(c *gin.Context) {
	startedAt := time.Now()
	if h.AgentDefinitionSync == nil {
		h.respondError(c, http.StatusServiceUnavailable, errAgentDefinitionSyncUnavailable)
		return
	}
	name := aghconfig.NormalizeAgentName(c.Param("name"))
	resolved, err := h.resolveAgentDefinition(c.Request.Context(), c.Query("workspace"), name)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
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
	if err := aghconfig.DeleteAgentDefinition(agentsRoot, resolved.Entry.Def.SourcePath); err != nil {
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
	h.logAgentDeleteMutation(resolved.Entry, startedAt, syncDuration, response.UnshadowedOrigin != "")
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
	if err := aghconfig.ValidateAuthoredAgentName(aghconfig.NormalizeAgentName(req.Name)); err != nil {
		wrapped := errors.Join(errAgentDefinitionInvalid, err)
		h.respondError(c, statusForAgentDefinitionError(wrapped), wrapped)
		return
	}
	source, err := h.resolveAgentDefinition(c.Request.Context(), req.Workspace, c.Param("name"))
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	target, err := h.duplicateAgentTarget(c.Request.Context(), req, &source)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	draft, err := duplicateAgentDraft(source.Entry.Def, req)
	if err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	if err := validateAgentDraftRuntime(draft, &target.Config); err != nil {
		h.respondError(c, statusForAgentDefinitionError(err), err)
		return
	}
	agent, err := aghconfig.DuplicateAgentDefinition(source.Entry.Def, target.Path, draft)
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
	h.logAgentMutation("duplicate", entry, startedAt, syncDuration)
	c.JSON(http.StatusCreated, contract.AgentResponse{
		Agent: AgentPayloadFromEntryWithConfig(entry, &target.Config),
	})
}

func updateAgentDraft(
	payload contract.CreateAgentPayload,
	current aghconfig.AgentDef,
) (aghconfig.AgentDefinitionDraft, error) {
	draft, err := createAgentDraftFromRequest(contract.CreateAgentRequest{Agent: payload})
	if err != nil {
		return aghconfig.AgentDefinitionDraft{}, err
	}
	currentDraft, err := authoredAgentDraftFromDef(current)
	if err != nil {
		return aghconfig.AgentDefinitionDraft{}, err
	}
	draft.MCPServers = currentDraft.MCPServers
	draft.Hooks = currentDraft.Hooks
	return draft, nil
}

func duplicateAgentDraft(
	source aghconfig.AgentDef,
	req contract.DuplicateAgentRequest,
) (aghconfig.AgentDefinitionDraft, error) {
	draft, err := authoredAgentDraftFromDef(source)
	if err != nil {
		return aghconfig.AgentDefinitionDraft{}, err
	}
	draft.Name = aghconfig.NormalizeAgentName(req.Name)
	if req.Overrides == nil {
		return draft, nil
	}
	overrides := req.Overrides
	if overrides.Provider != "" {
		draft.Provider = overrides.Provider
	}
	if overrides.Command != "" {
		draft.Command = overrides.Command
	}
	if overrides.Model != "" {
		draft.Model = overrides.Model
	}
	if overrides.ReasoningEffort != "" {
		draft.ReasoningEffort = string(overrides.ReasoningEffort)
	}
	if len(overrides.Tools) > 0 {
		draft.Tools = append([]string(nil), overrides.Tools...)
	}
	if len(overrides.Toolsets) > 0 {
		draft.Toolsets = append([]string(nil), overrides.Toolsets...)
	}
	if len(overrides.DenyTools) > 0 {
		draft.DenyTools = append([]string(nil), overrides.DenyTools...)
	}
	if overrides.Permissions != "" {
		draft.Permissions = string(overrides.Permissions)
	}
	if len(overrides.CategoryPath) > 0 {
		draft.CategoryPath = append([]string(nil), overrides.CategoryPath...)
	}
	if overrides.Skills != nil {
		draft.Skills.Disabled = append([]string(nil), overrides.Skills.Disabled...)
	}
	if overrides.Prompt != "" {
		draft.Prompt = overrides.Prompt
	}
	return draft, nil
}

func authoredAgentDraftFromDef(agent aghconfig.AgentDef) (aghconfig.AgentDefinitionDraft, error) {
	contents, err := os.ReadFile(agent.SourcePath)
	if err != nil {
		return aghconfig.AgentDefinitionDraft{}, fmt.Errorf(
			"api: read authored agent definition %q: %w",
			agent.SourcePath,
			err,
		)
	}
	authored, err := aghconfig.ParseAgentDef(contents)
	if err != nil {
		return aghconfig.AgentDefinitionDraft{}, fmt.Errorf(
			"api: parse authored agent definition %q: %w",
			agent.SourcePath,
			err,
		)
	}
	return aghconfig.AgentDefinitionDraftFromDef(authored), nil
}

func (h *BaseHandlers) purgeAgentDefinitionHistory(
	ctx context.Context,
	resolved *resolvedAgentDefinition,
) error {
	workspaceID := strings.TrimSpace(resolved.OperationWorkspace)
	if workspaceID == "" {
		return nil
	}
	if h.SoulHistoryPurger == nil || h.HeartbeatHistoryPurger == nil {
		return errors.New("api: agent definition history purgers are unavailable")
	}
	name := resolved.Entry.Def.Name
	sourcePath := resolved.Entry.Def.SourcePath
	if err := h.SoulHistoryPurger.PurgeAgentHistory(
		ctx,
		soul.WorkspaceRef{WorkspaceID: workspaceID},
		name,
		sourcePath,
	); err != nil {
		return err
	}
	if err := h.HeartbeatHistoryPurger.PurgeAgentHistory(
		ctx,
		heartbeat.WorkspaceRef{WorkspaceID: workspaceID},
		name,
		sourcePath,
	); err != nil {
		return err
	}
	return nil
}

func (h *BaseHandlers) logAgentMutation(
	operation string,
	entry AgentCatalogEntry,
	startedAt time.Time,
	syncDuration time.Duration,
) {
	attributes := []any{
		handlersOperationKey, "agent." + operation,
		"agent_name", entry.Def.Name,
		"origin", entry.Origin,
		"workspace_id", entry.WorkspaceID,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"sync_duration_ms", syncDuration.Milliseconds(),
		"outcome", "success",
	}
	h.Logger.Info("api: agent definition mutation completed", attributes...)
}

func (h *BaseHandlers) logAgentDeleteMutation(
	entry AgentCatalogEntry,
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

func statusForAgentDefinitionError(err error) int {
	switch {
	case errors.Is(err, aghconfig.ErrAgentNameReserved):
		return http.StatusUnprocessableEntity
	case errors.Is(err, errAgentDefinitionInvalid),
		errors.Is(err, errCreateAgentRequestInvalid),
		errors.Is(err, aghconfig.ErrInvalidAgentDefinition):
		return http.StatusBadRequest
	case errors.Is(err, errAgentDefinitionConflict),
		errors.Is(err, aghconfig.ErrAgentDefinitionExists):
		return http.StatusConflict
	case errors.Is(err, aghconfig.ErrAgentDefinitionNotFound),
		errors.Is(err, workspacepkg.ErrAgentNotAvailable),
		errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, errAgentDefinitionSyncUnavailable):
		return http.StatusServiceUnavailable
	default:
		return StatusForWorkspaceError(err)
	}
}
