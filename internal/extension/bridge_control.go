package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/subprocess"
)

// BridgeControlRuntimeResolver resolves one fresh, single-instance control grant.
type BridgeControlRuntimeResolver interface {
	ResolveBridgeControlRuntime(
		ctx context.Context,
		extensionName string,
		bridgeInstanceID string,
		method bridgepkg.ControlMethod,
	) (*subprocess.InitializeBridgeRuntime, error)
}

var _ bridgepkg.BridgeControlTransport = (*Manager)(nil)

// CheckBridge runs provider checks in one isolated, short-lived extension process.
func (m *Manager) CheckBridge(
	ctx context.Context,
	extensionName string,
	req bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	if err := req.Validate(); err != nil {
		return bridgepkg.BridgeCheckResponse{}, err
	}
	req.BridgeInstanceID = strings.TrimSpace(req.BridgeInstanceID)
	var response bridgepkg.BridgeCheckResponse
	err := m.runBridgeControl(
		ctx,
		extensionName,
		req.BridgeInstanceID,
		bridgepkg.ControlMethodCheck,
		func(callCtx context.Context, process processHandle) error {
			if err := process.Call(callCtx, string(bridgepkg.ControlMethodCheck), req, &response); err != nil {
				return err
			}
			return response.Validate()
		},
	)
	if err != nil {
		return bridgepkg.BridgeCheckResponse{}, err
	}
	return response, nil
}

// RegisterBridgeWebhook asks the adapter to register its configured webhook in an isolated process.
func (m *Manager) RegisterBridgeWebhook(
	ctx context.Context,
	extensionName string,
	req bridgepkg.BridgeWebhookRegistrationRequest,
) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
	if err := req.Validate(); err != nil {
		return bridgepkg.BridgeWebhookRegistrationResponse{}, err
	}
	req.BridgeInstanceID = strings.TrimSpace(req.BridgeInstanceID)
	var response bridgepkg.BridgeWebhookRegistrationResponse
	err := m.runBridgeControl(
		ctx,
		extensionName,
		req.BridgeInstanceID,
		bridgepkg.ControlMethodWebhookRegister,
		func(callCtx context.Context, process processHandle) error {
			if err := process.Call(
				callCtx,
				string(bridgepkg.ControlMethodWebhookRegister),
				req,
				&response,
			); err != nil {
				return err
			}
			return response.Validate()
		},
	)
	if err != nil {
		return bridgepkg.BridgeWebhookRegistrationResponse{}, err
	}
	return response, nil
}

type bridgeControlProcessCall func(context.Context, processHandle) error

func (m *Manager) runBridgeControl(
	ctx context.Context,
	extensionName string,
	bridgeInstanceID string,
	method bridgepkg.ControlMethod,
	call bridgeControlProcessCall,
) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if m == nil {
		return ErrManagerRequired
	}
	name := strings.TrimSpace(extensionName)
	if name == "" {
		return errors.New("extension: bridge control extension name is required")
	}
	instanceID := strings.TrimSpace(bridgeInstanceID)
	if instanceID == "" {
		return errors.New("extension: bridge control instance id is required")
	}
	if err := method.Validate(); err != nil {
		return err
	}
	if call == nil {
		return errors.New("extension: bridge control process call is required")
	}

	controlCtx, extension, releaseLifecycle, err := m.admitBridgeControl(ctx, name)
	if err != nil {
		return err
	}
	defer releaseLifecycle()

	releaseInstance, err := bridgeControlInstanceGates.acquire(controlCtx, name+"\x00"+instanceID)
	if err != nil {
		return err
	}
	defer releaseInstance()
	releaseGlobal, err := acquireBridgeControlGlobal(controlCtx)
	if err != nil {
		return err
	}
	defer releaseGlobal()
	if err := controlCtx.Err(); err != nil {
		return err
	}
	resolver, ok := m.bridgeRuntimeResolver.(BridgeControlRuntimeResolver)
	if !ok || resolver == nil {
		return fmt.Errorf("%w for bridge control", ErrBridgeRuntimeResolverRequired)
	}
	runtime, err := resolver.ResolveBridgeControlRuntime(controlCtx, name, instanceID, method)
	if err != nil {
		return fmt.Errorf("extension: resolve bridge control runtime for %q: %w", name, err)
	}
	if err := validateBridgeControlRuntime(runtime, name, instanceID, method); err != nil {
		return err
	}

	return m.executeBridgeControl(controlCtx, extension, runtime, method, call)
}

func validateBridgeControlRuntime(
	runtime *subprocess.InitializeBridgeRuntime,
	extensionName string,
	bridgeInstanceID string,
	method bridgepkg.ControlMethod,
) error {
	if runtime == nil {
		return errors.New("extension: bridge control runtime is required")
	}
	if err := runtime.Validate(); err != nil {
		return fmt.Errorf("extension: invalid bridge control runtime: %w", err)
	}
	if runtime.Purpose != subprocess.BridgeRuntimePurposeControl {
		return fmt.Errorf("extension: bridge control runtime purpose = %q, want control", runtime.Purpose)
	}
	if strings.TrimSpace(runtime.Provider) != strings.TrimSpace(extensionName) {
		return fmt.Errorf(
			"extension: bridge control runtime provider %q, want %q",
			runtime.Provider,
			extensionName,
		)
	}
	if len(runtime.AllowedMethods) != 1 || runtime.AllowedMethods[0] != string(method) {
		return fmt.Errorf("extension: bridge control runtime method does not match %q", method)
	}
	managed, err := runtime.SingleManagedInstance()
	if err != nil {
		return fmt.Errorf("extension: bridge control runtime instance: %w", err)
	}
	if strings.TrimSpace(managed.Instance.ID) != strings.TrimSpace(bridgeInstanceID) {
		return fmt.Errorf(
			"extension: bridge control runtime instance %q, want %q",
			managed.Instance.ID,
			bridgeInstanceID,
		)
	}
	return nil
}
