package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	bridgecontract "github.com/compozy/agh/internal/bridges/contract"

	"github.com/compozy/agh/internal/subprocess"
)

func (h *HostAPIHandler) prepareBridgeIngress(
	ctx context.Context,
	raw json.RawMessage,
) (hostAPIBridgeIngressContext, error) {
	var wireParams bridgecontract.InboundMessageEnvelope
	if err := decodeHostAPIParams(raw, &wireParams); err != nil {
		return hostAPIBridgeIngressContext{}, err
	}
	if err := wireParams.Validate(); err != nil {
		return hostAPIBridgeIngressContext{}, invalidParamsRPCError(err)
	}
	params := bridgeInboundEnvelopeDomain(wireParams)

	instance, err := h.authorizedBridgeInstance(ctx, params.BridgeInstanceID)
	if err != nil {
		return hostAPIBridgeIngressContext{}, err
	}
	if err := validateBridgeIngressInstance(*instance); err != nil {
		return hostAPIBridgeIngressContext{}, err
	}

	routingKey, err := h.bridges.BuildRoutingKey(ctx, bridgepkg.RoutingKey{
		Scope:            params.Scope,
		WorkspaceID:      params.WorkspaceID,
		BridgeInstanceID: params.BridgeInstanceID,
		PeerID:           params.PeerID,
		ThreadID:         params.ThreadID,
		GroupID:          params.GroupID,
	})
	if err != nil {
		return hostAPIBridgeIngressContext{}, mapBridgeRoutingError(instance.ID, err)
	}

	lockKey, err := routingKey.Hash()
	if err != nil {
		return hostAPIBridgeIngressContext{}, fmt.Errorf("extension: hash bridge routing key: %w", err)
	}

	return hostAPIBridgeIngressContext{
		params:     params,
		instance:   instance,
		routingKey: routingKey,
		lockKey:    lockKey,
	}, nil
}

func (h *HostAPIHandler) authorizedBridgeInstances(
	ctx context.Context,
) (*subprocess.InitializeBridgeRuntime, []bridgepkg.BridgeInstance, error) {
	runtime, extName, err := h.authorizedBridgeRuntime(ctx)
	if err != nil {
		return nil, nil, err
	}

	managedIDs := runtime.ManagedBridgeInstanceIDs()
	if len(managedIDs) == 0 {
		return runtime, nil, nil
	}

	instances := make([]bridgepkg.BridgeInstance, 0, len(managedIDs))
	for _, instanceID := range managedIDs {
		managed, ok := runtime.ManagedInstance(instanceID)
		if !ok {
			return nil, nil, notFoundRPCError(
				"bridge_instance",
				instanceID,
				fmt.Errorf("bridge instance %q is not assigned to this extension", instanceID),
			)
		}
		if strings.TrimSpace(managed.Instance.ExtensionName) != extName {
			return nil, nil, notFoundRPCError(
				"bridge_instance",
				instanceID,
				fmt.Errorf(
					"bridge runtime instance belongs to extension %q",
					strings.TrimSpace(managed.Instance.ExtensionName),
				),
			)
		}

		instance, err := h.bridges.GetInstance(ctx, instanceID)
		if err != nil {
			return nil, nil, mapBridgeLookupError(instanceID, err)
		}
		if strings.TrimSpace(instance.ExtensionName) != extName {
			return nil, nil, notFoundRPCError(
				"bridge_instance",
				instance.ID,
				fmt.Errorf("bridge instance %q is not owned by extension %q", instance.ID, extName),
			)
		}

		instances = append(instances, *instance)
	}

	return runtime, instances, nil
}

func (h *HostAPIHandler) authorizedBridgeInstance(
	ctx context.Context,
	bridgeInstanceID string,
) (*bridgepkg.BridgeInstance, error) {
	runtime, extName, err := h.authorizedBridgeRuntime(ctx)
	if err != nil {
		return nil, err
	}

	trimmedID, err := requireBridgeInstanceID(bridgeInstanceID)
	if err != nil {
		return nil, err
	}

	managed, ok := runtime.ManagedInstance(trimmedID)
	if !ok {
		return nil, notFoundRPCError(
			"bridge_instance",
			trimmedID,
			fmt.Errorf("bridge instance %q is not assigned to this extension", trimmedID),
		)
	}

	instanceID := strings.TrimSpace(managed.Instance.ID)
	if instanceID == "" {
		return nil, unavailableRPCError(errors.New("bridge runtime instance id is required"))
	}
	if strings.TrimSpace(managed.Instance.ExtensionName) != extName {
		return nil, notFoundRPCError(
			"bridge_instance",
			instanceID,
			fmt.Errorf(
				"bridge runtime instance belongs to extension %q",
				strings.TrimSpace(managed.Instance.ExtensionName),
			),
		)
	}

	instance, err := h.bridges.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, mapBridgeLookupError(instanceID, err)
	}
	if strings.TrimSpace(instance.ExtensionName) != extName {
		return nil, notFoundRPCError(
			"bridge_instance",
			instance.ID,
			fmt.Errorf("bridge instance %q is not owned by extension %q", instance.ID, extName),
		)
	}

	return instance, nil
}

func (h *HostAPIHandler) authorizedBridgeRuntime(
	ctx context.Context,
) (*subprocess.InitializeBridgeRuntime, string, error) {
	if h.bridges == nil {
		return nil, "", unavailableRPCError(errors.New("bridge registry is not configured"))
	}

	runtime := hostAPIBridgeRuntimeFromContext(ctx)
	if runtime == nil {
		return nil, "", unavailableRPCError(errors.New("bridge runtime is not configured"))
	}

	extName := hostAPIExtensionNameFromContext(ctx)
	if extName == "" {
		return nil, "", unavailableRPCError(errors.New("bridge extension name is not available"))
	}

	return runtime, extName, nil
}

func requireBridgeInstanceID(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", invalidParamsRPCError(errors.New("bridge_instance_id is required"))
	}
	return trimmed, nil
}
