package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	specAPIGatewayPath               = "/api/gateway"
	specAPIGatewayStatusPath         = specAPIGatewayPath + "/status"
	specAPIGatewaySurfacesPath       = specAPIGatewayPath + "/surfaces"
	specAPIGatewayProvidersNamePath  = specAPIGatewayPath + "/providers/{name}"
	specAPIGatewayPairingsPath       = specAPIGatewayPath + "/pairings"
	specAPIGatewayPairingRedeemPath  = specAPIGatewayPairingsPath + "/redeem"
	specAPIGatewayDevicesPath        = specAPIGatewayPath + "/devices"
	specAPIGatewayDevicesIDPath      = specAPIGatewayDevicesPath + "/{id}"
	specAPIGatewayStreamTicketsPath  = specAPIGatewayPath + "/stream-tickets"
	specAPIGatewayIngressPath        = specAPIGatewayPath + "/ingress-bindings"
	specAPIGatewayIngressSubjectPath = specAPIGatewayIngressPath + "/{subject_kind}/{subject_id}"
	specAPIBridgeCallbackPath        = "/api/bridge-callbacks/{id}"
)

func registryGatewayOperations() []OperationSpec {
	return []OperationSpec{
		gatewayStatusOperation(),
		gatewaySurfaceOperation(),
		gatewayProviderEnableOperation(),
		gatewayProviderDisableOperation(),
		gatewayPairingMintOperation(),
		gatewayPairingRedeemOperation(),
		gatewayDeviceListOperation(),
		gatewayDeviceRenameOperation(),
		gatewayDeviceRevokeOperation(),
		gatewayStreamTicketOperation(),
		gatewayIngressBindOperation(),
		gatewayIngressUnbindOperation(),
		gatewayBridgeCallbackGetOperation(),
		gatewayBridgeCallbackPostOperation(),
		gatewayBridgeCallbackHeadOperation(),
	}
}

func gatewayBridgeCallbackGetOperation() OperationSpec {
	return gatewayBridgeCallbackOperation(httpMethodGet, "probeGatewayBridgeCallback", "Probe one bound bridge callback")
}

func gatewayBridgeCallbackPostOperation() OperationSpec {
	operation := gatewayBridgeCallbackOperation(
		httpMethodPost,
		"deliverGatewayBridgeCallback",
		"Deliver one bound bridge callback",
	)
	operation.RequestBody = map[string]any{}
	return operation
}

func gatewayBridgeCallbackHeadOperation() OperationSpec {
	return gatewayBridgeCallbackOperation(httpMethodHead, "headGatewayBridgeCallback", "Probe one bound bridge callback")
}

func gatewayBridgeCallbackOperation(method string, operationID string, summary string) OperationSpec {
	return OperationSpec{
		Method: method, Path: specAPIBridgeCallbackPath,
		OperationID: operationID, Summary: summary,
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP},
		Parameters: []ParameterSpec{pathParam("id", "Bridge instance id")},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Adapter response", Body: binaryResponse{}, ContentType: "*/*"},
			{Status: 204, Description: "Confirmed callback endpoint"},
			{Status: 400, Description: "Invalid callback request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Bound bridge callback not found", Body: contract.ErrorPayload{}},
			{Status: 413, Description: specPayloadTooLargeDescription, Body: contract.ErrorPayload{}},
			{Status: 429, Description: specRateLimitedDescription, Body: contract.ErrorPayload{}},
			{Status: 502, Description: "Bridge adapter unavailable", Body: contract.ErrorPayload{}},
			{Status: 504, Description: "Bridge adapter timed out", Body: contract.ErrorPayload{}},
			{
				Default: true, Description: "Adapter-owned response",
				Body: binaryResponse{}, ContentType: "*/*",
			},
		},
	}
}

func gatewayIngressBindOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: specAPIGatewayIngressPath,
		OperationID: "bindGatewayIngress", Summary: "Confirm one subject against the verified public endpoint",
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Auth: gatewayManagementAuth(), RequestBody: contract.GatewayIngressBindRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Already confirmed", Body: contract.GatewayIngressBindingResponse{}},
			{Status: 201, Description: specCreatedDescription, Body: contract.GatewayIngressBindingResponse{}},
			{Status: 400, Description: "Invalid ingress binding request", Body: contract.ErrorPayload{}},
			{Status: 403, Description: "Ingress subject is outside the caller workspace", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Ingress subject not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Public ingress is not reachable", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func gatewayIngressUnbindOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodDelete, Path: specAPIGatewayIngressSubjectPath,
		OperationID: "unbindGatewayIngress", Summary: "Remove one public ingress confirmation",
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Auth: gatewayManagementAuth(),
		Parameters: []ParameterSpec{
			pathParam("subject_kind", "Ingress subject kind"),
			pathParam("subject_id", "Ingress subject id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Unbound", Body: contract.GatewayIngressUnbindResponse{}},
			{Status: 400, Description: "Invalid ingress subject", Body: contract.ErrorPayload{}},
			{Status: 403, Description: "Ingress subject is outside the caller workspace", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Ingress subject not found", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func gatewayManagementAuth() map[Transport]OperationAuth {
	return map[Transport]OperationAuth{
		TransportHTTP: OperationAuthDevice,
		TransportUDS:  OperationAuthLocalSocket,
	}
}

func gatewayStatusOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodGet, Path: specAPIGatewayStatusPath,
		OperationID: "getGatewayStatus", Summary: "Get the operator-global gateway posture",
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Auth: gatewayManagementAuth(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.GatewayStatusPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func gatewaySurfaceOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: specAPIGatewaySurfacesPath,
		OperationID: "setGatewaySurface", Summary: "Set durable exposure for one gateway surface",
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Auth:        gatewayManagementAuth(),
		RequestBody: contract.GatewaySurfaceRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: specUpdatedDescription, Body: contract.GatewayStatusPayload{}},
			{Status: 400, Description: "Invalid gateway surface request", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Gateway exposure conflict", Body: contract.ErrorPayload{}},
			{Status: 428, Description: "Gateway consent required", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func gatewayProviderEnableOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: specAPIGatewayProvidersNamePath + "/enable",
		OperationID: "enableGatewayProvider", Summary: "Enable one connectivity provider for a gateway tier",
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Auth:        gatewayManagementAuth(),
		Parameters:  []ParameterSpec{pathParam("name", "Connectivity provider name")},
		RequestBody: contract.GatewayProviderEnableRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Enabled", Body: contract.GatewayStatusPayload{}},
			{Status: 400, Description: "Invalid provider request", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Gateway provider conflict", Body: contract.ErrorPayload{}},
			{Status: 428, Description: "Provider digest confirmation required", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func gatewayProviderDisableOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: specAPIGatewayProvidersNamePath + "/disable",
		OperationID: "disableGatewayProvider", Summary: "Disable one named connectivity provider for a gateway tier",
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Auth: gatewayManagementAuth(),
		Parameters: []ParameterSpec{
			pathParam("name", "Connectivity provider name"),
			queryParam("tier", "Gateway tier", true),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Disabled", Body: contract.GatewayStatusPayload{}},
			{Status: 400, Description: "Invalid gateway tier", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Gateway provider not found", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Gateway provider conflict", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func gatewayPairingMintOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: specAPIGatewayPairingsPath,
		OperationID: "mintGatewayPairing", Summary: "Mint a bounded one-time gateway pairing artifact",
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Auth: gatewayManagementAuth(),
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.GatewayPairingArtifactPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 429, Description: specRateLimitedDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func gatewayPairingRedeemOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: specAPIGatewayPairingRedeemPath,
		OperationID: "redeemGatewayPairing", Summary: "Redeem a one-time gateway pairing artifact",
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Auth: map[Transport]OperationAuth{
			TransportHTTP: OperationAuthPrivatePreAuth,
			TransportUDS:  OperationAuthLocalSocket,
		},
		RequestBody: contract.GatewayPairingRedeemRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Redeemed", Body: contract.GatewayIssuedCredentialPayload{}},
			{Status: 400, Description: "Invalid pairing request", Body: contract.ErrorPayload{}},
			{Status: 401, Description: "Invalid pairing artifact", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Pairing artifact already spent", Body: contract.ErrorPayload{}},
			{Status: 410, Description: "Pairing artifact expired", Body: contract.ErrorPayload{}},
			{Status: 429, Description: specRateLimitedDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func gatewayDeviceListOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodGet, Path: specAPIGatewayDevicesPath,
		OperationID: "listGatewayDevices", Summary: "List the operator-global paired device inventory",
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Auth: gatewayManagementAuth(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.GatewayDevicesResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func gatewayDeviceRenameOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodPatch, Path: specAPIGatewayDevicesIDPath,
		OperationID: "renameGatewayDevice", Summary: "Rename one paired gateway device",
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Auth:        gatewayManagementAuth(),
		Parameters:  []ParameterSpec{pathParam("id", "Gateway device id")},
		RequestBody: contract.GatewayDeviceRenameRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Renamed", Body: contract.GatewayDevicePayload{}},
			{Status: 400, Description: "Invalid gateway device name", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Gateway device not found", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func gatewayDeviceRevokeOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodDelete, Path: specAPIGatewayDevicesIDPath,
		OperationID: "revokeGatewayDevice", Summary: "Revoke one paired gateway device and its live streams",
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Auth:       gatewayManagementAuth(),
		Parameters: []ParameterSpec{pathParam("id", "Gateway device id")},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Revoked", Body: contract.GatewayRevokePayload{}},
			{Status: 404, Description: "Gateway device not found", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func gatewayStreamTicketOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: specAPIGatewayStreamTicketsPath,
		OperationID: "mintGatewayStreamTicket", Summary: "Mint a short-lived single-use stream ticket",
		Tags: []string{specGatewayKey}, Transports: []Transport{TransportHTTP},
		Auth: map[Transport]OperationAuth{TransportHTTP: OperationAuthDevice},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.GatewayStreamTicketPayload{}},
			{Status: 401, Description: "Gateway device authentication required", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
