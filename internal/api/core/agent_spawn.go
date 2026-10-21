package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

const agentActionSpawn = "agent.spawn"

type agentSpawner interface {
	Spawn(ctx context.Context, opts session.SpawnOpts) (*session.Session, error)
}

// AgentSpawn creates a bounded child session for the validated agent caller.
func (h *BaseHandlers) AgentSpawn(c *gin.Context) {
	spawner, ok := h.Sessions.(agentSpawner)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: spawn service is not configured"))
		return
	}

	caller, ok := h.requireAgentCaller(c, agentActionSpawn)
	if !ok {
		return
	}

	req := contract.AgentSpawnRequest{AutoStopOnParent: true}
	if err := decodeStrictAgentSpawnRequest(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusUnprocessableEntity,
			fmt.Errorf("%w: %s: decode agent spawn request: %w", session.ErrSpawnValidation, h.transportName(), err),
		)
		return
	}

	ctx := c.Request.Context()
	worktreeTarget, err := h.resolveAgentSpawnWorktree(ctx, caller.Session, req)
	if err != nil {
		h.respondError(c, statusForAgentSpawnWorktreeError(err), err)
		return
	}
	opts := spawnOptsFromAgentRequest(req, caller.Session.ID)
	opts.IsolatedWorktreeID = worktreeTarget.ID()
	child, err := spawner.Spawn(ctx, opts)
	if err != nil {
		err = errors.Join(err, RollbackSpawnWorktree(ctx, h.Worktrees, worktreeTarget))
		h.respondError(c, statusForAgentSpawnError(err), err)
		return
	}

	payload, err := agentSpawnPayloadFromSession(child.Info())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	payload.Worktree = SpawnWorktreePayloadFromTarget(worktreeTarget)
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		result, promptErr := sendAgentSpawnPrompt(ctx, h.Sessions, session.SpawnPromptRequest{
			ParentSessionID: caller.Session.ID,
			ParentAgentName: caller.Session.AgentName,
			ChildSessionID:  child.ID,
			Message:         prompt,
			Source:          agentActionSpawn,
		})
		if promptErr != nil {
			h.respondError(c, StatusForSessionError(promptErr), fmt.Errorf(
				"api: spawned child %q but its first prompt failed: %w", child.ID, promptErr,
			))
			return
		}
		payload.Prompt = SpawnPromptPayloadFromResult(result)
	}
	sessionPayload, err := h.sessionPayloadWithOptionalHealth(ctx, child.Info(), false)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	payload.Session = sessionPayload
	c.JSON(http.StatusCreated, contract.AgentSpawnResponse{Spawn: payload})
}

type agentSpawnPrompter interface {
	SendSpawnPrompt(ctx context.Context, req session.SpawnPromptRequest) (session.SendPromptResult, error)
}

func sendAgentSpawnPrompt(
	ctx context.Context,
	sessions any,
	req session.SpawnPromptRequest,
) (session.SendPromptResult, error) {
	prompter, ok := sessions.(agentSpawnPrompter)
	if !ok {
		return session.SendPromptResult{}, errors.New("api: spawn prompt service is not configured")
	}
	return prompter.SendSpawnPrompt(ctx, req)
}

func (h *BaseHandlers) resolveAgentSpawnWorktree(
	ctx context.Context,
	caller agentidentity.SessionSnapshot,
	req contract.AgentSpawnRequest,
) (SpawnWorktreeTarget, error) {
	if strings.TrimSpace(req.Worktree) == "" && req.NewWorktree == nil {
		return SpawnWorktreeTarget{}, nil
	}
	// Authorize the target workspace before creating anything in it.
	workspaceID, err := ResolveSpawnWorkspace(ctx, h.Sessions, caller.ID, req.Workspace)
	if err != nil {
		return SpawnWorktreeTarget{}, err
	}
	return ResolveSpawnWorktree(ctx, h.Worktrees, workspaceID, caller.ProfileID, req.Worktree, req.NewWorktree)
}

type spawnWorkspaceResolver interface {
	ResolveSpawnWorkspace(ctx context.Context, parentSessionID, workspaceRef string) (string, error)
}

// ResolveSpawnWorkspace authorizes the child workspace of a spawn before any side effect in it.
func ResolveSpawnWorkspace(ctx context.Context, sessions any, parentSessionID, workspaceRef string) (string, error) {
	resolver, ok := sessions.(spawnWorkspaceResolver)
	if !ok {
		return "", errors.New("api: spawn workspace resolver is not configured")
	}
	return resolver.ResolveSpawnWorkspace(ctx, parentSessionID, workspaceRef)
}

func statusForAgentSpawnWorktreeError(err error) int {
	if errors.Is(err, session.ErrSpawnValidation) {
		return http.StatusUnprocessableEntity
	}
	if errors.Is(err, session.ErrSpawnPermissionDenied) {
		return http.StatusForbidden
	}
	if WorktreeErrorCode(err) != "" {
		return StatusForWorktreeError(err)
	}
	return StatusForSessionError(err)
}

func decodeStrictAgentSpawnRequest(c *gin.Context, req *contract.AgentSpawnRequest) error {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return io.EOF
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(req); err != nil {
		return err
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return err
	} else if err == nil {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func spawnOptsFromAgentRequest(req contract.AgentSpawnRequest, parentSessionID string) session.SpawnOpts {
	return session.SpawnOpts{
		ParentSessionID:    strings.TrimSpace(parentSessionID),
		AgentName:          strings.TrimSpace(req.AgentName),
		Provider:           strings.TrimSpace(req.Provider),
		Model:              strings.TrimSpace(req.Model),
		ReasoningEffort:    strings.TrimSpace(string(req.ReasoningEffort)),
		Speed:              req.Speed,
		ACPOptions:         contract.ACPOptionSelectionsFromPayload(req.ACPOptions),
		Name:               strings.TrimSpace(req.Name),
		Workspace:          strings.TrimSpace(req.Workspace),
		PromptOverlay:      strings.TrimSpace(req.PromptOverlay),
		SpawnRole:          strings.TrimSpace(req.SpawnRole),
		TTL:                time.Duration(req.TTLSeconds) * time.Second,
		AutoStopOnParent:   req.AutoStopOnParent,
		NotifyCreator:      req.NotifyCreator != nil && *req.NotifyCreator,
		NotifyCreatorSet:   req.NotifyCreator != nil,
		PermissionPolicy:   sessionPermissionPolicyFromPayload(req.Permissions),
		InheritPermissions: req.InheritPermissions,
		PermissionNarrowing: SpawnPermissionNarrowing(
			req.Permissions.Tools, req.Permissions.Skills, req.Permissions.MCPServers, req.Permissions.WorkspacePaths,
		),
		Toolsets:       trimSpawnAtoms(req.Toolsets),
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
	}
}

func agentSpawnPayloadFromSession(info *session.Info) (contract.AgentSpawnPayload, error) {
	if info == nil {
		return contract.AgentSpawnPayload{}, errors.New("api: spawn returned an empty session")
	}
	lineage := contract.SessionLineagePayloadFromStore(info.Lineage)
	if lineage == nil {
		return contract.AgentSpawnPayload{}, fmt.Errorf("api: spawned session %q is missing lineage", info.ID)
	}
	return contract.AgentSpawnPayload{
		Session:     SessionPayloadFromInfo(info),
		Lineage:     *lineage,
		Permissions: contract.NormalizeSpawnPermissionPolicyPayload(lineage.PermissionPolicy),
	}, nil
}

func sessionPermissionPolicyFromPayload(payload contract.SpawnPermissionPolicyPayload) store.SessionPermissionPolicy {
	normalized := contract.NormalizeSpawnPermissionPolicyPayload(payload)
	return store.NormalizeSessionPermissionPolicy(store.SessionPermissionPolicy{
		Tools:          append([]string(nil), normalized.Tools...),
		Skills:         append([]string(nil), normalized.Skills...),
		MCPServers:     append([]string(nil), normalized.MCPServers...),
		WorkspacePaths: append([]string(nil), normalized.WorkspacePaths...),
	})
}

func statusForAgentSpawnError(err error) int {
	switch {
	case err == nil:
		return http.StatusCreated
	case errors.Is(err, session.ErrSpawnPermissionDenied):
		return http.StatusForbidden
	case errors.Is(err, session.ErrSpawnLimitExceeded):
		return http.StatusConflict
	case errors.Is(err, session.ErrSpawnValidation):
		return http.StatusUnprocessableEntity
	default:
		return StatusForSessionError(err)
	}
}
