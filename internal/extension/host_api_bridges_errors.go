package extensionpkg

import (
	"context"

	"errors"
	"fmt"
	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/subprocess"
)

func validateBridgeIngressInstance(instance bridgepkg.BridgeInstance) error {
	if !instance.Enabled || instance.Status.Normalize() == bridgepkg.BridgeStatusDisabled {
		return unavailableRPCError(fmt.Errorf("bridge instance %q is disabled", instance.ID))
	}

	switch instance.Status.Normalize() {
	case bridgepkg.BridgeStatusReady, bridgepkg.BridgeStatusDegraded:
		return nil
	case bridgepkg.BridgeStatusStarting,
		bridgepkg.BridgeStatusAuthRequired,
		bridgepkg.BridgeStatusError:
		return unavailableRPCError(
			fmt.Errorf("bridge instance %q status %q cannot ingest messages", instance.ID, instance.Status.Normalize()),
		)
	default:
		return unavailableRPCError(
			fmt.Errorf("bridge instance %q status %q is unavailable", instance.ID, instance.Status.Normalize()),
		)
	}
}

func bridgeRouteForRoutingKey(
	routingKey bridgepkg.RoutingKey,
	sessionID string,
	agentName string,
	activityAt time.Time,
) bridgepkg.BridgeRoute {
	return bridgepkg.BridgeRoute{
		Scope:            routingKey.Scope,
		WorkspaceID:      routingKey.WorkspaceID,
		BridgeInstanceID: routingKey.BridgeInstanceID,
		PeerID:           routingKey.PeerID,
		ThreadID:         routingKey.ThreadID,
		GroupID:          routingKey.GroupID,
		SessionID:        strings.TrimSpace(sessionID),
		AgentName:        strings.TrimSpace(agentName),
		LastActivityAt:   activityAt,
		UpdatedAt:        activityAt,
	}
}

func mapBridgeLookupError(instanceID string, err error) error {
	switch {
	case errors.Is(err, bridgepkg.ErrBridgeInstanceNotFound):
		return notFoundRPCError("bridge_instance", strings.TrimSpace(instanceID), err)
	case errors.Is(err, bridgepkg.ErrBridgeInstanceUnavailable):
		return unavailableRPCError(err)
	default:
		return err
	}
}

func mapBridgeRoutingError(instanceID string, err error) error {
	switch {
	case errors.Is(err, bridgepkg.ErrBridgeInstanceNotFound):
		return notFoundRPCError("bridge_instance", strings.TrimSpace(instanceID), err)
	case errors.Is(err, bridgepkg.ErrBridgeInstanceUnavailable):
		return unavailableRPCError(err)
	default:
		return invalidParamsRPCError(err)
	}
}

func mapBridgeRouteError(instanceID string, err error) error {
	switch {
	case errors.Is(err, bridgepkg.ErrBridgeInstanceNotFound):
		return notFoundRPCError("bridge_instance", strings.TrimSpace(instanceID), err)
	case errors.Is(err, bridgepkg.ErrBridgeInstanceUnavailable):
		return unavailableRPCError(err)
	default:
		return err
	}
}

func mapBridgeStateUpdateError(instanceID string, err error) error {
	switch {
	case errors.Is(err, bridgepkg.ErrBridgeInstanceNotFound):
		return notFoundRPCError("bridge_instance", strings.TrimSpace(instanceID), err)
	case errors.Is(err, bridgepkg.ErrInvalidBridgeStateTransition):
		return invalidParamsRPCError(err)
	default:
		return invalidParamsRPCError(err)
	}
}

func hostAPIBridgeRuntimeFromContext(ctx context.Context) *subprocess.InitializeBridgeRuntime {
	if ctx == nil {
		return nil
	}
	runtime, ok := ctx.Value(hostAPIBridgeRuntimeContextKey).(*subprocess.InitializeBridgeRuntime)
	if !ok {
		return nil
	}
	return subprocess.CloneInitializeBridgeRuntime(runtime)
}
