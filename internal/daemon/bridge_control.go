package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/deadentity"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/subprocess"
)

var (
	_ extensionpkg.BridgeControlRuntimeResolver = (*bridgeRuntime)(nil)
	_ bridgepkg.BridgeControlTransport          = (*bridgeRuntime)(nil)
)

// ResolveBridgeControlRuntime returns one fresh disabled-or-enabled instance snapshot without mutating lifecycle state.
func (r *bridgeRuntime) ResolveBridgeControlRuntime(
	ctx context.Context,
	extensionName string,
	bridgeInstanceID string,
	method bridgepkg.ControlMethod,
) (*subprocess.InitializeBridgeRuntime, error) {
	if r == nil {
		return nil, errors.New("daemon: bridge runtime is required")
	}
	if ctx == nil {
		return nil, errors.New("daemon: bridge control runtime context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := method.Validate(); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(extensionName)
	instanceID := strings.TrimSpace(bridgeInstanceID)
	if name == "" {
		return nil, errors.New("daemon: bridge control extension name is required")
	}
	if instanceID == "" {
		return nil, errors.New("daemon: bridge control instance id is required")
	}

	ctx, unlock, err := r.lockBridgeControlLifecycleContext(ctx, name, instanceID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	instance, err := r.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve bridge control instance %q: %w", instanceID, err)
	}
	if strings.TrimSpace(instance.ExtensionName) != name {
		return nil, fmt.Errorf(
			"daemon: bridge instance %q belongs to extension %q, not %q",
			instanceID,
			instance.ExtensionName,
			name,
		)
	}
	boundSecrets, err := r.resolveBoundSecrets(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve bridge control secrets for %q: %w", instanceID, err)
	}
	runtime := subprocess.InitializeBridgeRuntime{
		RuntimeVersion: subprocess.InitializeBridgeRuntimeVersion2,
		Purpose:        subprocess.BridgeRuntimePurposeControl,
		Provider:       name,
		Platform:       strings.TrimSpace(instance.Platform),
		AllowedMethods: []string{string(method)},
		ManagedInstances: []subprocess.InitializeBridgeManagedInstance{{
			Instance:     bridgepkg.BridgeInstanceToContract(*instance),
			BoundSecrets: boundSecrets,
		}},
	}
	if err := runtime.Validate(); err != nil {
		return nil, fmt.Errorf("daemon: build bridge control runtime for %q: %w", instanceID, err)
	}
	return subprocess.CloneInitializeBridgeRuntime(&runtime), nil
}

// CheckBridge holds the instance lifecycle lock until the transient provider process is reaped.
func (r *bridgeRuntime) CheckBridge(
	ctx context.Context,
	extensionName string,
	req bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	if err := req.Validate(); err != nil {
		return bridgepkg.BridgeCheckResponse{}, err
	}
	key, tracked, err := r.bridgeDeadEntityKey(ctx, req.BridgeInstanceID)
	if err != nil {
		return bridgepkg.BridgeCheckResponse{}, err
	}
	if tracked {
		decision, err := r.deadEntities.BeforeProbe(ctx, key)
		if err != nil {
			return bridgepkg.BridgeCheckResponse{}, fmt.Errorf("daemon: admit bridge recovery check: %w", err)
		}
		if !decision.Allowed {
			return bridgepkg.BridgeCheckResponse{}, fmt.Errorf(
				"%w: bridge %q is marked dead: %s",
				bridgepkg.ErrBridgeInstanceUnavailable,
				req.BridgeInstanceID,
				decision.Reason,
			)
		}
	}
	var response bridgepkg.BridgeCheckResponse
	err = r.withBridgeControlTransport(
		ctx,
		extensionName,
		req.BridgeInstanceID,
		func(callCtx context.Context, transport bridgepkg.BridgeControlTransport, name string) error {
			var callErr error
			response, callErr = transport.CheckBridge(callCtx, name, req)
			if callErr != nil {
				return callErr
			}
			return response.Validate()
		},
	)
	if err != nil {
		r.recordBridgeCheckFailure(ctx, key, tracked, bridgeCheckErrorClass(err), err.Error())
		return bridgepkg.BridgeCheckResponse{}, err
	}
	if reason, failed := permanentBridgeCheckFailure(response); failed {
		r.recordBridgeCheckFailure(ctx, key, tracked, deadentity.FailurePermanent, reason)
	} else if tracked {
		if err := r.deadEntities.RecordSuccess(ctx, key); err != nil {
			return bridgepkg.BridgeCheckResponse{}, fmt.Errorf("daemon: record bridge check recovery: %w", err)
		}
	}
	return response, nil
}

func (r *bridgeRuntime) bridgeDeadEntityKey(
	ctx context.Context,
	instanceID string,
) (store.DeadEntityKey, bool, error) {
	if r == nil || r.deadEntities == nil {
		return store.DeadEntityKey{}, false, nil
	}
	instance, err := r.GetInstance(ctx, strings.TrimSpace(instanceID))
	if err != nil {
		return store.DeadEntityKey{}, false, fmt.Errorf("daemon: load bridge reliability scope: %w", err)
	}
	if instance.Scope != bridgepkg.ScopeWorkspace {
		return store.DeadEntityKey{}, false, nil
	}
	key := store.DeadEntityKey{
		WorkspaceID: instance.WorkspaceID,
		Kind:        store.DeadEntityKindBridge,
		EntityID:    instance.ID,
	}.Normalize()
	if err := key.Validate(); err != nil {
		return store.DeadEntityKey{}, false, fmt.Errorf("daemon: bridge reliability key: %w", err)
	}
	return key, true, nil
}

func (r *bridgeRuntime) recordBridgeCheckFailure(
	ctx context.Context,
	key store.DeadEntityKey,
	tracked bool,
	class deadentity.FailureClass,
	reason string,
) {
	if !tracked || r == nil || r.deadEntities == nil {
		return
	}
	if err := r.deadEntities.RecordFailure(ctx, key, class, reason); err != nil {
		r.logger.Warn(
			"daemon: record bridge reliability failure failed open",
			"workspace_id", key.WorkspaceID,
			"bridge_instance_id", key.EntityID,
			"error", err,
		)
	}
}

func permanentBridgeCheckFailure(response bridgepkg.BridgeCheckResponse) (string, bool) {
	reasons := make([]string, 0)
	for _, check := range response.Checks {
		if check.Status != bridgepkg.BridgeCheckStatusFail {
			continue
		}
		reason := strings.TrimSpace(check.Check)
		if remediation := strings.TrimSpace(check.Remediation); remediation != "" {
			reason += ": " + remediation
		}
		reasons = append(reasons, reason)
	}
	return strings.Join(reasons, "; "), len(reasons) > 0
}

func bridgeCheckErrorClass(err error) deadentity.FailureClass {
	if errors.Is(err, bridgepkg.ErrBridgeControlTransportUnavailable) ||
		errors.Is(err, bridgepkg.ErrBridgeInstanceUnavailable) {
		return deadentity.FailurePermanent
	}
	return deadentity.FailureTransient
}

// RegisterBridgeWebhook holds the instance lifecycle lock through provider registration and process reap.
func (r *bridgeRuntime) RegisterBridgeWebhook(
	ctx context.Context,
	extensionName string,
	req bridgepkg.BridgeWebhookRegistrationRequest,
) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
	if err := req.Validate(); err != nil {
		return bridgepkg.BridgeWebhookRegistrationResponse{}, err
	}
	var response bridgepkg.BridgeWebhookRegistrationResponse
	err := r.withBridgeControlTransport(
		ctx,
		extensionName,
		req.BridgeInstanceID,
		func(callCtx context.Context, transport bridgepkg.BridgeControlTransport, name string) error {
			var callErr error
			response, callErr = transport.RegisterBridgeWebhook(callCtx, name, req)
			if callErr != nil {
				return callErr
			}
			return response.Validate()
		},
	)
	if err != nil {
		return bridgepkg.BridgeWebhookRegistrationResponse{}, err
	}
	return response, nil
}

type bridgeControlTransportCall func(context.Context, bridgepkg.BridgeControlTransport, string) error

func (r *bridgeRuntime) withBridgeControlTransport(
	ctx context.Context,
	extensionName string,
	bridgeInstanceID string,
	call bridgeControlTransportCall,
) error {
	if r == nil {
		return errors.New("daemon: bridge runtime is required")
	}
	if ctx == nil {
		return errors.New("daemon: bridge control context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	name := strings.TrimSpace(extensionName)
	instanceID := strings.TrimSpace(bridgeInstanceID)
	if name == "" {
		return errors.New("daemon: bridge control extension name is required")
	}

	ctx, unlock, err := r.lockBridgeControlLifecycleContext(ctx, name, instanceID)
	if err != nil {
		return err
	}
	defer unlock()
	instance, err := r.GetInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("daemon: load bridge control instance %q: %w", instanceID, err)
	}
	if strings.TrimSpace(instance.ExtensionName) != name {
		return fmt.Errorf(
			"daemon: bridge instance %q belongs to extension %q, not %q",
			instanceID,
			instance.ExtensionName,
			name,
		)
	}
	transport, ok := r.extensionRuntime().(bridgepkg.BridgeControlTransport)
	if !ok || transport == nil {
		return bridgepkg.ErrBridgeControlTransportUnavailable
	}
	return call(ctx, transport, name)
}
