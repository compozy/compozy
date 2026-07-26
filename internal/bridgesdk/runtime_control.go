package bridgesdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

func (r *Runtime) handleCheck(ctx context.Context, raw json.RawMessage) (any, error) {
	session, err := r.requireControlSession(bridgepkg.ControlMethodCheck)
	if err != nil {
		return nil, err
	}
	var request bridgepkg.BridgeCheckRequest
	if err := decodeParams(raw, &request); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, invalidControlParams(err)
	}
	if err := validateControlInstanceRequest(session, request.BridgeInstanceID); err != nil {
		return nil, invalidControlParams(err)
	}
	response, err := r.config.Check(ctx, session, request)
	if err != nil {
		return nil, err
	}
	if err := response.Validate(); err != nil {
		return nil, invalidControlParams(err)
	}
	return response, nil
}

func (r *Runtime) handleRegisterWebhook(ctx context.Context, raw json.RawMessage) (any, error) {
	session, err := r.requireControlSession(bridgepkg.ControlMethodWebhookRegister)
	if err != nil {
		return nil, err
	}
	var request bridgepkg.BridgeWebhookRegistrationRequest
	if err := decodeParams(raw, &request); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, invalidControlParams(err)
	}
	if err := validateControlInstanceRequest(session, request.BridgeInstanceID); err != nil {
		return nil, invalidControlParams(err)
	}
	if r.config.RegisterWebhook == nil {
		return nil, subprocess.NewRPCError(bridgeSDKRPCCodeInternal, "Internal error", map[string]string{
			runtimeErrorKey: "bridge webhook registration is not implemented",
		})
	}
	response, err := r.config.RegisterWebhook(ctx, session, request)
	if err != nil {
		return nil, err
	}
	if err := response.Validate(); err != nil {
		return nil, invalidControlParams(err)
	}
	return response, nil
}

func (r *Runtime) requireControlSession(method bridgepkg.ControlMethod) (*Session, error) {
	session, err := r.requireSession()
	if err != nil {
		return nil, err
	}
	runtime := session.BridgeRuntime()
	if runtime == nil || runtime.Purpose != subprocess.BridgeRuntimePurposeControl {
		return nil, subprocess.NewRPCError(bridgeSDKRPCCodeInvalidParams, "Invalid params", map[string]string{
			runtimeErrorKey: "bridge control method is not available in service mode",
		})
	}
	if len(runtime.AllowedMethods) != 1 || runtime.AllowedMethods[0] != string(method) {
		return nil, subprocess.NewRPCError(bridgeSDKRPCCodeInvalidParams, "Invalid params", map[string]string{
			runtimeErrorKey: fmt.Sprintf("bridge control method %q is not allowed", method),
		})
	}
	return session, nil
}

func (r *Runtime) requireServiceSession(operation string) (*Session, error) {
	session, err := r.requireSession()
	if err != nil {
		return nil, err
	}
	r.mu.RLock()
	shutdownState := r.shutdownState
	r.mu.RUnlock()
	switch shutdownState {
	case runtimeShutdownRunning:
		return nil, subprocess.NewRPCError(
			bridgeSDKRPCCodeShutdownRunning,
			"Shutdown running",
			map[string]string{runtimeErrorKey: strings.TrimSpace(operation) + " is unavailable during shutdown"},
		)
	case runtimeShutdownSucceeded:
		return nil, subprocess.NewRPCError(
			bridgeSDKRPCCodeShutdownRunning,
			"Shutdown complete",
			map[string]string{runtimeErrorKey: strings.TrimSpace(operation) + " is unavailable after shutdown"},
		)
	}
	if sessionIsControl(session) {
		return nil, subprocess.NewRPCError(bridgeSDKRPCCodeInvalidParams, "Invalid params", map[string]string{
			runtimeErrorKey: strings.TrimSpace(operation) + " is not available in control mode",
		})
	}
	return session, nil
}

func (r *Runtime) validateControlHandler(runtime *subprocess.InitializeBridgeRuntime) error {
	if runtime == nil || runtime.Purpose != subprocess.BridgeRuntimePurposeControl || len(runtime.AllowedMethods) != 1 {
		return errors.New("bridgesdk: valid control runtime is required")
	}
	method := bridgepkg.ControlMethod(runtime.AllowedMethods[0])
	switch method {
	case bridgepkg.ControlMethodCheck:
		if r.config.Check == nil {
			return errors.New("bridgesdk: runtime check handler is required")
		}
	case bridgepkg.ControlMethodWebhookRegister:
		if r.config.RegisterWebhook == nil {
			return errors.New("bridgesdk: runtime webhook registration handler is required")
		}
	default:
		return fmt.Errorf("bridgesdk: unsupported control method %q", method)
	}
	return nil
}

func (r *Runtime) initializeResponse(request subprocess.InitializeRequest) subprocess.InitializeResponse {
	if request.Runtime.Bridge != nil && request.Runtime.Bridge.Purpose == subprocess.BridgeRuntimePurposeControl {
		implemented := []string{request.Runtime.Bridge.AllowedMethods[0], runtimeShutdownMethod}
		slices.Sort(implemented)
		return subprocess.InitializeResponse{
			ProtocolVersion:    request.ProtocolVersion,
			ExtensionInfo:      r.config.ExtensionInfo,
			ImplementedMethods: implemented,
		}
	}

	implemented := []string{
		string(extensionprotocol.ExtensionServiceMethodBridgesDeliver),
		string(extensionprotocol.ExtensionServiceMethodBridgeTargets),
		"health_check",
		runtimeShutdownMethod,
	}
	slices.Sort(implemented)
	return subprocess.InitializeResponse{
		ProtocolVersion: request.ProtocolVersion,
		ExtensionInfo:   r.config.ExtensionInfo,
		AcceptedCapabilities: subprocess.AcceptedCapabilities{
			Provides: append([]string(nil), request.Capabilities.Provides...),
			Actions:  append([]extensionprotocol.HostAPIMethod(nil), request.Capabilities.GrantedActions...),
			Security: append([]string(nil), request.Capabilities.GrantedSecurity...),
		},
		ImplementedMethods: implemented,
		Supports: subprocess.InitializeSupports{
			HealthCheck: true,
		},
	}
}

func validateControlInstanceRequest(session *Session, bridgeInstanceID string) error {
	if strings.TrimSpace(bridgeInstanceID) == "" {
		return errors.New("bridge instance id is required")
	}
	runtime := session.BridgeRuntime()
	if runtime == nil {
		return errors.New("bridge control runtime is required")
	}
	managed, err := runtime.SingleManagedInstance()
	if err != nil {
		return err
	}
	if strings.TrimSpace(managed.Instance.ID) != strings.TrimSpace(bridgeInstanceID) {
		return fmt.Errorf("bridge instance %q is outside the control runtime grant", bridgeInstanceID)
	}
	return nil
}

func invalidControlParams(err error) error {
	return subprocess.NewRPCError(bridgeSDKRPCCodeInvalidParams, "Invalid params", map[string]string{
		runtimeErrorKey: err.Error(),
	})
}

func sessionIsControl(session *Session) bool {
	if session == nil {
		return false
	}
	runtime := session.BridgeRuntime()
	return runtime != nil && runtime.Purpose == subprocess.BridgeRuntimePurposeControl
}
