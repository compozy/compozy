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

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
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

	child, err := spawner.Spawn(c.Request.Context(), spawnOptsFromAgentRequest(req, caller.Session.ID))
	if err != nil {
		h.respondError(c, statusForAgentSpawnError(err), err)
		return
	}

	payload, err := agentSpawnPayloadFromSession(child.Info())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, contract.AgentSpawnResponse{Spawn: payload})
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
		ParentSessionID:  strings.TrimSpace(parentSessionID),
		AgentName:        strings.TrimSpace(req.AgentName),
		Provider:         strings.TrimSpace(req.Provider),
		Name:             strings.TrimSpace(req.Name),
		PromptOverlay:    strings.TrimSpace(req.PromptOverlay),
		SpawnRole:        strings.TrimSpace(req.SpawnRole),
		TTL:              time.Duration(req.TTLSeconds) * time.Second,
		AutoStopOnParent: req.AutoStopOnParent,
		PermissionPolicy: sessionPermissionPolicyFromPayload(req.Permissions),
		IdempotencyKey:   strings.TrimSpace(req.IdempotencyKey),
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
		Tools:           append([]string(nil), normalized.Tools...),
		Skills:          append([]string(nil), normalized.Skills...),
		MCPServers:      append([]string(nil), normalized.MCPServers...),
		WorkspacePaths:  append([]string(nil), normalized.WorkspacePaths...),
		NetworkChannels: append([]string(nil), normalized.NetworkChannels...),
		SandboxProfiles: append([]string(nil), normalized.SandboxProfiles...),
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
