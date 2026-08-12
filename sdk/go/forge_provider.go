package compozysdk

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/sdk/go/contracts"
)

// ForgeCapabilitiesRequest asks which remotes a provider serves.
type ForgeCapabilitiesRequest = contracts.ForgeCapabilitiesRequest

// ForgeCapabilitiesResponse describes provider vocabulary and availability.
type ForgeCapabilitiesResponse = contracts.ForgeCapabilitiesResponse

// ForgeStatusRequest asks for pull-request state by branch.
type ForgeStatusRequest = contracts.ForgeStatusRequest

// ForgeStatusResponse reports the current pull-request state.
type ForgeStatusResponse = contracts.ForgeStatusResponse

// ForgePRCreateRequest asks a provider to create or find one pull request.
type ForgePRCreateRequest = contracts.ForgePRCreateRequest

// ForgePRCreateResponse reports the created or existing pull request.
type ForgePRCreateResponse = contracts.ForgePRCreateResponse

// ForgeProviderHandlers implements the daemon-initiated forge provider services.
type ForgeProviderHandlers struct {
	Capabilities func(context.Context, ExtensionContext, ForgeCapabilitiesRequest) (ForgeCapabilitiesResponse, error)
	Status       func(context.Context, ExtensionContext, ForgeStatusRequest) (ForgeStatusResponse, error)
	CreatePR     func(context.Context, ExtensionContext, ForgePRCreateRequest) (ForgePRCreateResponse, error)
}

// ForgeProvider registers one public forge provider atomically.
func ForgeProvider(extension *Extension, handlers ForgeProviderHandlers) error {
	if extension == nil {
		return NewInternalError("extension is required")
	}
	if handlers.Capabilities == nil || handlers.Status == nil || handlers.CreatePR == nil {
		return NewInvalidParamsError("forge provider handlers are required", nil)
	}
	registrations := []struct {
		method  string
		handler ExtensionHandler
	}{
		{ExtensionServiceMethodForgeCapabilities, typedForgeHandler(handlers.Capabilities)},
		{ExtensionServiceMethodForgeStatus, typedForgeHandler(handlers.Status)},
		{ExtensionServiceMethodForgePRCreate, typedForgeHandler(handlers.CreatePR)},
	}
	extension.mu.Lock()
	defer extension.mu.Unlock()
	if extension.initialized {
		return NewInvalidParamsError("extension registration is closed after initialize", nil)
	}
	if extension.forgeProvider {
		return NewInvalidParamsError("forge provider is already registered", nil)
	}
	for _, registration := range registrations {
		if _, exists := extension.handlers[registration.method]; exists {
			return NewInvalidParamsError(registration.method+" is reserved by ForgeProvider", nil)
		}
	}
	for _, registration := range registrations {
		extension.handlers[registration.method] = registration.handler
		extension.transport.Handle(registration.method, extension.dispatch)
	}
	extension.forgeProvider = true
	extension.definition.Capabilities.Provides = normalizeStrings(append(
		extension.definition.Capabilities.Provides,
		CapabilityProvideForgeProvider,
	))
	return nil
}

func typedForgeHandler[Request any, Response any](
	handler func(context.Context, ExtensionContext, Request) (Response, error),
) ExtensionHandler {
	return func(ctx context.Context, extensionContext ExtensionContext, raw json.RawMessage) (any, error) {
		var request Request
		if len(raw) == 0 {
			return nil, NewInvalidParamsError("forge request is required", nil)
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, NewInvalidParamsError("forge request is invalid", map[string]any{"error": err.Error()})
		}
		return handler(ctx, extensionContext, request)
	}
}

func isForgeProviderMethod(method string) bool {
	switch strings.TrimSpace(method) {
	case ExtensionServiceMethodForgeCapabilities,
		ExtensionServiceMethodForgeStatus,
		ExtensionServiceMethodForgePRCreate:
		return true
	default:
		return false
	}
}
