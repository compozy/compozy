package extensionpkg

import (
	"context"

	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/session"
)

func (h *HostAPIHandler) maybeCleanupBridgeIngestDedup(ctx context.Context) error {
	if h.dedupStore == nil {
		return unavailableRPCError(errors.New("bridge ingest dedup store is not configured"))
	}

	now := h.now()

	h.bridgeCleanupMu.Lock()
	shouldRun := h.bridgeLastCleanup.IsZero() || now.Sub(h.bridgeLastCleanup) >= h.bridgeCleanupInterval
	if shouldRun {
		h.bridgeLastCleanup = now
	}
	h.bridgeCleanupMu.Unlock()

	if !shouldRun {
		return nil
	}

	if err := retrySQLiteBusy(ctx, func() error {
		_, err := h.dedupStore.DeleteExpiredBridgeIngestDedup(ctx, now)
		return err
	}); err != nil {
		return fmt.Errorf("extension: cleanup expired bridge ingest dedup: %w", err)
	}
	return nil
}

func (h *HostAPIHandler) suppressedBridgeIngressRoute(
	ctx context.Context,
	routingKey bridgepkg.RoutingKey,
	instance bridgepkg.BridgeInstance,
	idempotencyKey string,
) (*bridgepkg.BridgeRoute, bool, error) {
	record, err := h.dedupStore.GetBridgeIngestDedup(ctx, idempotencyKey, h.now())
	switch {
	case err == nil:
		if strings.TrimSpace(record.BridgeInstanceID) != instance.ID {
			return nil, false, invalidParamsRPCError(
				fmt.Errorf(
					"idempotency key %q is already assigned to bridge instance %q",
					idempotencyKey,
					record.BridgeInstanceID,
				),
			)
		}

		route, routeErr := h.bridges.ResolveRoute(ctx, routingKey)
		if routeErr == nil {
			return route, true, nil
		}
		if errors.Is(routeErr, bridgepkg.ErrBridgeRouteNotFound) {
			return nil, false, nil
		}
		return nil, false, mapBridgeRouteError(instance.ID, routeErr)
	case errors.Is(err, bridgepkg.ErrIngestDedupRecordNotFound):
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("extension: get bridge ingest dedup %q: %w", idempotencyKey, err)
	}
}

func (h *HostAPIHandler) resolveBridgeIngressRoute(
	ctx context.Context,
	instance bridgepkg.BridgeInstance,
	routingKey bridgepkg.RoutingKey,
) (*bridgepkg.BridgeRoute, bool, error) {
	existing, err := h.bridges.ResolveRoute(ctx, routingKey)
	switch {
	case err == nil:
		refreshed, _, resolveErr := h.resolveOrCreateBridgeRoute(
			ctx,
			bridgeRouteForRoutingKey(routingKey, existing.SessionID, existing.AgentName, h.now()),
		)
		if resolveErr != nil {
			return nil, false, mapBridgeRouteError(instance.ID, resolveErr)
		}
		return refreshed, false, nil
	case !errors.Is(err, bridgepkg.ErrBridgeRouteNotFound):
		return nil, false, mapBridgeRouteError(instance.ID, err)
	}

	createdSession, err := h.createBridgeSession(ctx, instance)
	if err != nil {
		return nil, false, err
	}
	createdInfo := createdSession.Info()

	route, created, routeErr := h.resolveOrCreateBridgeRoute(
		ctx,
		bridgeRouteForRoutingKey(routingKey, createdInfo.ID, createdInfo.AgentName, h.now()),
	)
	if routeErr != nil {
		cleanupErr := h.stopBridgeSession(ctx, createdInfo.ID)
		mapped := mapBridgeRouteError(instance.ID, routeErr)
		if cleanupErr != nil {
			return nil, false, errors.Join(mapped, cleanupErr)
		}
		return nil, false, mapped
	}

	if !created && route.SessionID != createdInfo.ID {
		if cleanupErr := h.stopBridgeSession(ctx, createdInfo.ID); cleanupErr != nil {
			return nil, false, cleanupErr
		}
	}

	return route, created, nil
}

func (h *HostAPIHandler) promptBridgeRoute(
	ctx context.Context,
	instance bridgepkg.BridgeInstance,
	routingKey bridgepkg.RoutingKey,
	route *bridgepkg.BridgeRoute,
	envelope bridgepkg.InboundMessageEnvelope,
	message string,
) (*bridgepkg.BridgeRoute, error) {
	if route == nil {
		return nil, unavailableRPCError(errors.New("bridge route is required"))
	}

	submission, err := h.submitBridgePrompt(ctx, instance, routingKey, route.SessionID, envelope, message)
	if err == nil {
		if registerErr := h.registerPromptDeliveryAfterSubmission(
			context.WithoutCancel(ctx),
			instance,
			routingKey,
			route.SessionID,
			submission,
		); registerErr != nil {
			return nil, registerErr
		}
		return route, nil
	} else if !errors.Is(err, session.ErrSessionNotFound) && !errors.Is(err, session.ErrSessionNotActive) {
		return nil, err
	}

	replacement, err := h.createBridgeSession(ctx, instance)
	if err != nil {
		return nil, err
	}
	replacementInfo := replacement.Info()

	rebound, err := h.upsertBridgeRoute(
		ctx,
		bridgeRouteForRoutingKey(routingKey, replacementInfo.ID, replacementInfo.AgentName, h.now()),
	)
	if err != nil {
		cleanupErr := h.stopBridgeSession(ctx, replacementInfo.ID)
		mapped := mapBridgeRouteError(instance.ID, err)
		if cleanupErr != nil {
			return nil, errors.Join(mapped, cleanupErr)
		}
		return nil, mapped
	}

	submission, err = h.submitBridgePrompt(ctx, instance, routingKey, rebound.SessionID, envelope, message)
	if err != nil {
		return nil, err
	}
	if err := h.registerPromptDeliveryAfterSubmission(
		context.WithoutCancel(ctx),
		instance,
		routingKey,
		rebound.SessionID,
		submission,
	); err != nil {
		return nil, err
	}
	return rebound, nil
}
