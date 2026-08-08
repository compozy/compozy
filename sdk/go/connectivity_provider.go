package compozysdk

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/sdk/go/contracts"
)

// ConnectivityEstablishRequest is the daemon request to start one connectivity tier.
type ConnectivityEstablishRequest = contracts.ConnectivityEstablishRequest

// ConnectivityStatusRequest is the daemon request to inspect one connectivity tier.
type ConnectivityStatusRequest = contracts.ConnectivityStatusRequest

// ConnectivityTeardownRequest is the daemon request to stop one connectivity tier.
type ConnectivityTeardownRequest = contracts.ConnectivityTeardownRequest

// ConnectivityReachability reports the provider's current endpoints and health.
type ConnectivityReachability = contracts.ConnectivityReachability

// ConnectivityAdvertisedEndpoint is one endpoint claimed by a connectivity provider.
type ConnectivityAdvertisedEndpoint = contracts.ConnectivityAdvertisedEndpoint

// ConnectivityTeardownResponse reports whether a connectivity tier stopped.
type ConnectivityTeardownResponse = contracts.ConnectivityTeardownResponse

// ConnectivityProviderHandlers implements the three daemon-initiated provider services.
type ConnectivityProviderHandlers struct {
	Establish func(context.Context, ExtensionContext, ConnectivityEstablishRequest) (ConnectivityReachability, error)
	Status    func(context.Context, ExtensionContext, ConnectivityStatusRequest) (ConnectivityReachability, error)
	Teardown  func(context.Context, ExtensionContext, ConnectivityTeardownRequest) (ConnectivityTeardownResponse, error)
}

// ConnectivityProvider registers one public reachability provider and advertises
// CapabilityProvideConnectivityProvider on the extension runtime.
func ConnectivityProvider(extension *Extension, handlers ConnectivityProviderHandlers) error {
	if extension == nil {
		return NewInternalError("extension is required")
	}
	if handlers.Establish == nil || handlers.Status == nil || handlers.Teardown == nil {
		return NewInvalidParamsError("connectivity provider handlers are required", nil)
	}
	registrations := []struct {
		method  string
		handler ExtensionHandler
	}{
		{ExtensionServiceMethodConnectivityEstablish, typedConnectivityHandler(handlers.Establish)},
		{ExtensionServiceMethodConnectivityStatus, typedConnectivityHandler(handlers.Status)},
		{ExtensionServiceMethodConnectivityTeardown, typedConnectivityHandler(handlers.Teardown)},
	}
	extension.mu.Lock()
	defer extension.mu.Unlock()
	if extension.initialized {
		return NewInvalidParamsError("extension registration is closed after initialize", nil)
	}
	if extension.connectivityProvider {
		return NewInvalidParamsError("connectivity provider is already registered", nil)
	}
	for _, registration := range registrations {
		if _, exists := extension.handlers[registration.method]; exists {
			return NewInvalidParamsError(registration.method+" is reserved by ConnectivityProvider", nil)
		}
	}
	for _, registration := range registrations {
		extension.handlers[registration.method] = registration.handler
		extension.transport.Handle(registration.method, extension.dispatch)
	}
	extension.connectivityProvider = true
	extension.definition.Capabilities.Provides = normalizeStrings(append(
		extension.definition.Capabilities.Provides,
		CapabilityProvideConnectivityProvider,
	))
	return nil
}

func typedConnectivityHandler[Request any, Response any](
	handler func(context.Context, ExtensionContext, Request) (Response, error),
) ExtensionHandler {
	return func(ctx context.Context, extensionContext ExtensionContext, raw json.RawMessage) (any, error) {
		var request Request
		if len(raw) == 0 {
			return nil, NewInvalidParamsError("connectivity request is required", nil)
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, NewInvalidParamsError("connectivity request is invalid", map[string]any{
				"error": err.Error(),
			})
		}
		response, err := handler(ctx, extensionContext, request)
		if err != nil {
			return nil, err
		}
		return response, nil
	}
}

func isConnectivityProviderMethod(method string) bool {
	switch strings.TrimSpace(method) {
	case ExtensionServiceMethodConnectivityEstablish,
		ExtensionServiceMethodConnectivityStatus,
		ExtensionServiceMethodConnectivityTeardown:
		return true
	default:
		return false
	}
}
