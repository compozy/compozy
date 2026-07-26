package extensionpkg

import (
	"context"

	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
)

func (h *HostAPIHandler) createBridgeSession(
	ctx context.Context,
	instance bridgepkg.BridgeInstance,
) (*session.Session, error) {
	if h.sessions == nil {
		return nil, unavailableRPCError(errors.New("session manager is not configured"))
	}
	if h.workspaces == nil {
		return nil, unavailableRPCError(errors.New("workspace resolver is not configured"))
	}
	if strings.TrimSpace(instance.WorkspaceID) == "" {
		return nil, unavailableRPCError(errors.New("bridge instance workspace is required to create a session"))
	}

	resolved, err := h.workspaces.Resolve(ctx, instance.WorkspaceID)
	if err != nil {
		return nil, unavailableRPCError(fmt.Errorf("resolve workspace %q: %w", instance.WorkspaceID, err))
	}

	agentName, err := aghconfig.ResolveAgentName("", resolved.Config.Defaults)
	if err != nil {
		return nil, unavailableRPCError(
			fmt.Errorf("resolve default agent for workspace %q: %w", resolved.WorkspaceID, err),
		)
	}
	workspaceID, err := hostAPIResolvedWorkspaceRegistrationID(&resolved)
	if err != nil {
		return nil, unavailableRPCError(err)
	}

	created, err := h.sessions.Create(ctx, session.CreateOpts{
		AgentName: agentName,
		Provider:  "",
		Workspace: workspaceID,
		Type:      session.SessionTypeUser,
	})
	if err != nil {
		return nil, unavailableRPCError(fmt.Errorf("create bridge session: %w", err))
	}
	return created, nil
}

func (h *HostAPIHandler) stopBridgeSession(ctx context.Context, sessionID string) error {
	if h.sessions == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if err := h.sessions.Stop(ctx, sessionID); err != nil && !errors.Is(err, session.ErrSessionNotFound) {
		return fmt.Errorf("extension: stop orphaned bridge session %q: %w", sessionID, err)
	}
	return nil
}

func (h *HostAPIHandler) resolveOrCreateBridgeRoute(
	ctx context.Context,
	route bridgepkg.BridgeRoute,
) (*bridgepkg.BridgeRoute, bool, error) {
	var (
		resolved *bridgepkg.BridgeRoute
		created  bool
	)

	err := retrySQLiteBusy(ctx, func() error {
		var callErr error
		resolved, created, callErr = h.bridges.ResolveOrCreateRoute(ctx, route)
		return callErr
	})
	return resolved, created, err
}

func (h *HostAPIHandler) upsertBridgeRoute(
	ctx context.Context,
	route bridgepkg.BridgeRoute,
) (*bridgepkg.BridgeRoute, error) {
	var updated *bridgepkg.BridgeRoute
	err := retrySQLiteBusy(ctx, func() error {
		var callErr error
		updated, callErr = h.bridges.UpsertRoute(ctx, route)
		return callErr
	})
	return updated, err
}

func (h *HostAPIHandler) recordBridgeIngressDedup(
	ctx context.Context,
	envelope bridgepkg.InboundMessageEnvelope,
	instance bridgepkg.BridgeInstance,
) error {
	dedupBaseTime := h.now()
	if dedupBaseTime.Before(envelope.ReceivedAt) {
		dedupBaseTime = envelope.ReceivedAt
	}

	record := bridgepkg.IngestDedupRecord{
		IdempotencyKey:   strings.TrimSpace(envelope.IdempotencyKey),
		BridgeInstanceID: instance.ID,
		ReceivedAt:       envelope.ReceivedAt,
		ExpiresAt:        dedupBaseTime.Add(h.bridgeIngestDedupTTL),
	}
	if err := retrySQLiteBusy(ctx, func() error {
		return h.dedupStore.PutBridgeIngestDedup(ctx, record)
	}); err != nil {
		return fmt.Errorf("extension: put bridge ingest dedup %q: %w", record.IdempotencyKey, err)
	}
	return nil
}
