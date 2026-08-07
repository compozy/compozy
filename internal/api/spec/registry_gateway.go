package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	specAPIGatewayPath              = "/api/gateway"
	specAPIGatewayStatusPath        = specAPIGatewayPath + "/status"
	specAPIGatewaySurfacesPath      = specAPIGatewayPath + "/surfaces"
	specAPIGatewayProvidersNamePath = specAPIGatewayPath + "/providers/{name}"
	specAPIGatewayPairingsPath      = specAPIGatewayPath + "/pairings"
	specAPIGatewayPairingRedeemPath = specAPIGatewayPairingsPath + "/redeem"
	specAPIGatewayDevicesPath       = specAPIGatewayPath + "/devices"
	specAPIGatewayDevicesIDPath     = specAPIGatewayDevicesPath + "/{id}"
	specAPIGatewayStreamTicketsPath = specAPIGatewayPath + "/stream-tickets"
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
