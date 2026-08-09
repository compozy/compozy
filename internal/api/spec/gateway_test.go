package spec

import (
	"slices"
	"testing"
)

func TestGatewayOperationRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Should register every gateway route with its exact transport and response contract", func(t *testing.T) {
		t.Parallel()

		type expectedOperation struct {
			operationID string
			transports  []Transport
			auth        map[Transport]OperationAuth
			statuses    []int
		}
		managementAuth := gatewayManagementAuth()
		expected := map[string]expectedOperation{
			"GET /api/gateway/audit": {
				operationID: "getGatewayAudit", transports: []Transport{TransportHTTP, TransportUDS},
				auth: managementAuth, statuses: []int{200, 500},
			},
			"GET /api/gateway/status": {
				operationID: "getGatewayStatus", transports: []Transport{TransportHTTP, TransportUDS},
				auth:     managementAuth,
				statuses: []int{200, 500},
			},
			"POST /api/gateway/surfaces": {
				operationID: "setGatewaySurface", transports: []Transport{TransportHTTP, TransportUDS},
				auth:     managementAuth,
				statuses: []int{200, 400, 409, 428, 500},
			},
			"POST /api/gateway/providers/{name}/enable": {
				operationID: "enableGatewayProvider", transports: []Transport{TransportHTTP, TransportUDS},
				auth:     managementAuth,
				statuses: []int{200, 400, 409, 428, 500},
			},
			"POST /api/gateway/providers/{name}/disable": {
				operationID: "disableGatewayProvider", transports: []Transport{TransportHTTP, TransportUDS},
				auth:     managementAuth,
				statuses: []int{200, 400, 404, 409, 500},
			},
			"POST /api/gateway/pairings": {
				operationID: "mintGatewayPairing", transports: []Transport{TransportHTTP, TransportUDS},
				auth:     managementAuth,
				statuses: []int{201, 403, 429, 500},
			},
			"POST /api/gateway/pairings/redeem": {
				operationID: "redeemGatewayPairing", transports: []Transport{TransportHTTP, TransportUDS},
				auth: map[Transport]OperationAuth{
					TransportHTTP: OperationAuthPrivatePreAuth,
					TransportUDS:  OperationAuthLocalSocket,
				},
				statuses: []int{200, 400, 401, 403, 409, 410, 429, 500},
			},
			"GET /api/gateway/devices": {
				operationID: "listGatewayDevices", transports: []Transport{TransportHTTP, TransportUDS},
				auth:     managementAuth,
				statuses: []int{200, 500},
			},
			"PATCH /api/gateway/devices/{id}": {
				operationID: "renameGatewayDevice", transports: []Transport{TransportHTTP, TransportUDS},
				auth:     managementAuth,
				statuses: []int{200, 400, 404, 500},
			},
			"DELETE /api/gateway/devices/{id}": {
				operationID: "revokeGatewayDevice", transports: []Transport{TransportHTTP, TransportUDS},
				auth:     managementAuth,
				statuses: []int{200, 404, 500},
			},
			"POST /api/gateway/stream-tickets": {
				operationID: "mintGatewayStreamTicket", transports: []Transport{TransportHTTP},
				auth:     map[Transport]OperationAuth{TransportHTTP: OperationAuthDevice},
				statuses: []int{201, 401, 500},
			},
			"POST /api/gateway/ingress-bindings": {
				operationID: "bindGatewayIngress", transports: []Transport{TransportHTTP, TransportUDS},
				auth: managementAuth, statuses: []int{200, 201, 400, 403, 404, 409, 500},
			},
			"DELETE /api/gateway/ingress-bindings/{subject_kind}/{subject_id}": {
				operationID: "unbindGatewayIngress", transports: []Transport{TransportHTTP, TransportUDS},
				auth: managementAuth, statuses: []int{200, 400, 403, 404, 500},
			},
			"GET /api/bridge-callbacks/{id}": {
				operationID: "probeGatewayBridgeCallback", transports: []Transport{TransportHTTP},
				statuses: []int{200, 204, 400, 404, 413, 429, 502, 504},
			},
			"POST /api/bridge-callbacks/{id}": {
				operationID: "deliverGatewayBridgeCallback", transports: []Transport{TransportHTTP},
				statuses: []int{200, 204, 400, 404, 413, 429, 502, 504},
			},
			"HEAD /api/bridge-callbacks/{id}": {
				operationID: "headGatewayBridgeCallback", transports: []Transport{TransportHTTP},
				statuses: []int{200, 204, 400, 404, 413, 429, 502, 504},
			},
		}

		actual := make(map[string]OperationSpec)
		for _, operation := range Operations() {
			if !slices.Contains(operation.Tags, specGatewayKey) {
				continue
			}
			actual[operation.Method+" "+operation.Path] = operation
		}
		if len(actual) != len(expected) {
			t.Fatalf("gateway operation count = %d, want %d; operations = %#v", len(actual), len(expected), actual)
		}
		for route, want := range expected {
			operation, ok := actual[route]
			if !ok {
				t.Fatalf("gateway operation %q is absent", route)
			}
			if operation.OperationID != want.operationID || !slices.Equal(operation.Transports, want.transports) ||
				!gatewayAuthEqual(operation.Auth, want.auth) ||
				!slices.Equal(gatewayResponseStatuses(operation), want.statuses) {
				t.Fatalf("gateway operation %q = %#v, want %#v", route, operation, want)
			}
		}
	})

	t.Run("Should bind provider disable to both provider name and tier", func(t *testing.T) {
		t.Parallel()

		operation := gatewayOperationByID(t, "disableGatewayProvider")
		if len(operation.Parameters) != 2 {
			t.Fatalf("disable parameters = %#v, want name path and tier query", operation.Parameters)
		}
		name := operation.Parameters[0]
		tier := operation.Parameters[1]
		if name.Name != "name" || name.In != "path" || !name.Required ||
			tier.Name != "tier" || tier.In != "query" || !tier.Required {
			t.Fatalf("disable parameters = %#v", operation.Parameters)
		}
	})

	t.Run("Should document adapter-owned callback statuses and media types", func(t *testing.T) {
		t.Parallel()
		for _, operationID := range []string{
			"probeGatewayBridgeCallback",
			"deliverGatewayBridgeCallback",
			"headGatewayBridgeCallback",
		} {
			operation := gatewayOperationByID(t, operationID)
			found := false
			for _, response := range operation.Responses {
				if response.Default && response.ContentType == "*/*" {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("operation %q has no wildcard default adapter response", operationID)
			}
		}
	})

	t.Run("Should emit auth metadata into the OpenAPI operation extension", func(t *testing.T) {
		t.Parallel()

		document, err := Document()
		if err != nil {
			t.Fatalf("Document() error = %v", err)
		}
		operation := document.Paths.Find("/api/gateway/pairings/redeem").Post
		raw, ok := operation.Extensions["x-compozy-auth"]
		if !ok {
			t.Fatal("pairing redeem is missing x-compozy-auth")
		}
		auth, ok := raw.(map[string]string)
		if !ok || auth[string(TransportHTTP)] != string(OperationAuthPrivatePreAuth) ||
			auth[string(TransportUDS)] != string(OperationAuthLocalSocket) {
			t.Fatalf("x-compozy-auth = %#v, want transport auth matrix", raw)
		}
	})
}

func gatewayAuthEqual(left, right map[Transport]OperationAuth) bool {
	if len(left) != len(right) {
		return false
	}
	for transport, mode := range right {
		if left[transport] != mode {
			return false
		}
	}
	return true
}

func gatewayOperationByID(t *testing.T, operationID string) OperationSpec {
	t.Helper()
	for _, operation := range Operations() {
		if operation.OperationID == operationID {
			return operation
		}
	}
	t.Fatalf("operation %q is absent", operationID)
	return OperationSpec{}
}

func gatewayResponseStatuses(operation OperationSpec) []int {
	statuses := make([]int, 0, len(operation.Responses))
	for _, response := range operation.Responses {
		if response.Default {
			continue
		}
		statuses = append(statuses, response.Status)
	}
	return statuses
}
