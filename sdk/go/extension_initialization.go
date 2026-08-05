package compozysdk

import (
	"encoding/json"

	"slices"
)

func (e *Extension) handleInitialize(params json.RawMessage) (InitializeResponse, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.initialized {
		return InitializeResponse{}, NewInvalidParamsError("initialize may only be called once", nil)
	}
	if err := e.definition.validate(); err != nil {
		return InitializeResponse{}, err
	}
	var request InitializeRequest
	if err := json.Unmarshal(params, &request); err != nil {
		return InitializeResponse{}, NewInvalidParamsError(
			"initialize params must be an object",
			map[string]any{extensionErrorKey: err.Error()},
		)
	}
	if err := validateInitializeRequest(request); err != nil {
		return InitializeResponse{}, err
	}
	if request.ProtocolVersion != ProtocolVersion ||
		!slices.Contains(request.SupportedProtocolVersions, ProtocolVersion) {
		return InitializeResponse{}, NewInvalidParamsError("unsupported protocol version", map[string]any{
			"protocol_version": request.ProtocolVersion,
		})
	}
	requestedProvides := normalizeStrings(e.definition.Capabilities.Provides)
	requestedPermissions := normalizeHostMethods(e.definition.Permissions.Requires)
	grantedPermissions := hostMethodsToStrings(request.Capabilities.GrantedPermissions)
	if err := ensureSubset("provides", requestedProvides, request.Capabilities.Provides); err != nil {
		return InitializeResponse{}, err
	}
	if err := ensureSubset(
		"permissions",
		hostMethodsToStrings(requestedPermissions),
		grantedPermissions,
	); err != nil {
		return InitializeResponse{}, err
	}
	implemented := e.implementedMethodsLocked()
	if err := validateProvidedMethodCoverage(requestedProvides, implemented); err != nil {
		return InitializeResponse{}, err
	}
	response := InitializeResponse{
		ProtocolVersion: ProtocolVersion,
		ExtensionInfo: InitializeExtensionInfo{
			Name:       e.definition.Name,
			Version:    e.definition.Version,
			SDKName:    SDKName,
			SDKVersion: e.sdkVersion,
		},
		AcceptedCapabilities: AcceptedCapabilities{
			Provides:    requestedProvides,
			Permissions: requestedPermissions,
		},
		ImplementedMethods:  implemented,
		SupportedHookEvents: normalizeStrings(e.definition.SupportedHookEvents),
		WatchSourceKinds:    e.watchSourceKindsLocked(),
		Supports:            InitializeSupports{HealthCheck: true},
	}
	session := ExtensionSession{
		InitializeRequest:    request,
		InitializeResponse:   response,
		Runtime:              request.Runtime,
		AcceptedCapabilities: response.AcceptedCapabilities,
	}
	e.initialized = true
	e.session = &session
	e.startReadyCallbacksLocked(e.session)
	return response, nil
}
