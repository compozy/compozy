package contract_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func TestCreateBridgeRequestValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  contract.CreateBridgeRequest
	}{
		{
			name: "workspace scope requires workspace id",
			req: contract.CreateBridgeRequest{
				Scope:         bridgepkg.ScopeWorkspace,
				Platform:      "telegram",
				ExtensionName: "ext-telegram",
				DisplayName:   "Support",
				Enabled:       true,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			},
		},
		{
			name: "global scope rejects workspace id",
			req: contract.CreateBridgeRequest{
				Scope:         bridgepkg.ScopeGlobal,
				WorkspaceID:   "ws-alpha",
				Platform:      "telegram",
				ExtensionName: "ext-telegram",
				DisplayName:   "Support",
				Enabled:       true,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			},
		},
		{
			name: "routing policy rejects thread without peer or group",
			req: contract.CreateBridgeRequest{
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "ext-telegram",
				DisplayName:   "Support",
				Enabled:       true,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludeThread: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := tt.req.ToCreateInstanceRequest(); err == nil {
				t.Fatal("ToCreateInstanceRequest() error = nil, want non-nil")
			}
		})
	}
}

func TestCreateBridgeRequestPreservesNormalizedFieldsAndDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve normalized bridge create fields and clone raw payloads", func(t *testing.T) {
		t.Parallel()

		req := contract.CreateBridgeRequest{
			Scope:            bridgepkg.ScopeWorkspace,
			WorkspaceID:      " ws-alpha ",
			Platform:         " telegram ",
			ExtensionName:    " ext-telegram ",
			DisplayName:      " Support ",
			Enabled:          true,
			DMPolicy:         bridgepkg.BridgeDMPolicyPairing,
			RoutingPolicy:    bridgepkg.RoutingPolicy{IncludePeer: true},
			ProviderConfig:   contract.BridgeProviderConfigPayload(`{"mode":"bot","tenant":"acme"}`),
			DeliveryDefaults: contract.BridgeDeliveryDefaultsPayload(`{"mode":"reply","peer_id":"peer-1"}`),
		}

		mapped, err := req.ToCreateInstanceRequest()
		if err != nil {
			t.Fatalf("ToCreateInstanceRequest() error = %v", err)
		}
		if mapped.WorkspaceID != "ws-alpha" || mapped.Platform != "telegram" ||
			mapped.ExtensionName != "ext-telegram" || mapped.DisplayName != "Support" {
			t.Fatalf("mapped request = %#v", mapped)
		}
		if mapped.DMPolicy != bridgepkg.BridgeDMPolicyPairing {
			t.Fatalf("mapped.DMPolicy = %q, want %q", mapped.DMPolicy, bridgepkg.BridgeDMPolicyPairing)
		}
		if string(mapped.ProviderConfig) != `{"mode":"bot","tenant":"acme"}` {
			t.Fatalf("mapped.ProviderConfig = %s", string(mapped.ProviderConfig))
		}
		if string(mapped.DeliveryDefaults) != `{"mode":"reply","peer_id":"peer-1"}` {
			t.Fatalf("mapped.DeliveryDefaults = %s", string(mapped.DeliveryDefaults))
		}
		if mapped.Status != bridgepkg.BridgeStatusStarting {
			t.Fatalf("mapped.Status = %q, want %q", mapped.Status, bridgepkg.BridgeStatusStarting)
		}

		req.DeliveryDefaults[0] = '['
		req.ProviderConfig[0] = '['
		if string(mapped.DeliveryDefaults) != `{"mode":"reply","peer_id":"peer-1"}` {
			t.Fatalf("mapped.DeliveryDefaults mutated with source slice = %s", string(mapped.DeliveryDefaults))
		}
		if string(mapped.ProviderConfig) != `{"mode":"bot","tenant":"acme"}` {
			t.Fatalf("mapped.ProviderConfig mutated with source slice = %s", string(mapped.ProviderConfig))
		}
	})
}

func TestCreateBridgeRequestInitialStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		enabled bool
		want    bridgepkg.BridgeStatus
	}{
		{
			name:    "Should map enabled create requests to starting status",
			enabled: true,
			want:    bridgepkg.BridgeStatusStarting,
		},
		{
			name:    "Should map disabled create requests to disabled status",
			enabled: false,
			want:    bridgepkg.BridgeStatusDisabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mapped, err := contract.CreateBridgeRequest{
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "ext-telegram",
				DisplayName:   "Support",
				Enabled:       tt.enabled,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			}.ToCreateInstanceRequest()
			if err != nil {
				t.Fatalf("ToCreateInstanceRequest() error = %v", err)
			}
			if mapped.Status != tt.want {
				t.Fatalf("mapped.Status = %q, want %q", mapped.Status, tt.want)
			}
		})
	}
}

func TestBridgeRoutesResponseJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve route target fields in JSON", func(t *testing.T) {
		t.Parallel()

		payload := contract.BridgeRoutesResponse{
			Routes: []bridgepkg.BridgeRoute{
				{
					RoutingKeyHash:   "hash-1",
					Scope:            bridgepkg.ScopeWorkspace,
					WorkspaceID:      "ws-alpha",
					BridgeInstanceID: "brg-1",
					PeerID:           "peer-1",
					ThreadID:         "thread-1",
					GroupID:          "group-1",
					SessionID:        "sess-1",
					AgentName:        "coder",
				},
			},
		}

		var got map[string]any
		marshalJSON(t, payload, &got)

		routes, ok := got["routes"].([]any)
		if !ok || len(routes) != 1 {
			t.Fatalf("routes JSON = %#v", got["routes"])
		}
		route, ok := routes[0].(map[string]any)
		if !ok {
			t.Fatalf("route JSON type = %T, want object", routes[0])
		}
		if route["peer_id"] != "peer-1" || route["thread_id"] != "thread-1" || route["group_id"] != "group-1" {
			t.Fatalf("route JSON = %#v", route)
		}
	})
}

func TestBridgeTestDeliveryRequestPreservesTypedTargetShape(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve typed delivery target mapping and response JSON", func(t *testing.T) {
		t.Parallel()

		req := contract.BridgeTestDeliveryRequest{
			Message: "hello",
			Target: contract.BridgeDeliveryTargetInput{
				PeerID:   "peer-1",
				ThreadID: "thread-1",
				GroupID:  "group-1",
				Mode:     bridgepkg.DeliveryModeReply,
			},
		}

		mapped, err := req.ToResolveDeliveryTargetRequest("brg-1")
		if err != nil {
			t.Fatalf("ToResolveDeliveryTargetRequest() error = %v", err)
		}
		if mapped.BridgeInstanceID != "brg-1" || mapped.PeerID != "peer-1" || mapped.ThreadID != "thread-1" ||
			mapped.GroupID != "group-1" ||
			mapped.Mode != bridgepkg.DeliveryModeReply {
			t.Fatalf("mapped target = %#v", mapped)
		}

		data, err := json.Marshal(contract.BridgeTestDeliveryResponse{
			Status:  "resolved",
			Message: "hello",
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-1",
				PeerID:           "peer-1",
				ThreadID:         "thread-1",
				GroupID:          "group-1",
				Mode:             bridgepkg.DeliveryModeReply,
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal(response) error = %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("json.Unmarshal(response) error = %v", err)
		}
		target, ok := got["delivery_target"].(map[string]any)
		if !ok {
			t.Fatalf("delivery_target type = %T, want object", got["delivery_target"])
		}
		if target["peer_id"] != "peer-1" || target["thread_id"] != "thread-1" || target["group_id"] != "group-1" ||
			target["mode"] != string(bridgepkg.DeliveryModeReply) {
			t.Fatalf("delivery_target JSON = %#v", target)
		}
	})
}

func TestBridgeTestDeliveryRequestRejectsMismatchedInstanceID(t *testing.T) {
	t.Parallel()

	t.Run("Should reject mismatched body and path instance ids", func(t *testing.T) {
		t.Parallel()

		req := contract.BridgeTestDeliveryRequest{
			Target: contract.BridgeDeliveryTargetInput{
				BridgeInstanceID: "brg-2",
				PeerID:           "peer-1",
			},
		}

		if _, err := req.ToResolveDeliveryTargetRequest("brg-1"); !errors.Is(err, contract.ErrBridgeInstanceMismatch) {
			t.Fatalf("ToResolveDeliveryTargetRequest() error = %v, want %v", err, contract.ErrBridgeInstanceMismatch)
		}
	})
}

func TestBridgeTestDeliveryRequestAcceptsExplicitMatchingInstanceID(t *testing.T) {
	t.Parallel()

	t.Run("Should accept explicit matching instance id", func(t *testing.T) {
		t.Parallel()

		req := contract.BridgeTestDeliveryRequest{
			Target: contract.BridgeDeliveryTargetInput{
				BridgeInstanceID: " brg-1 ",
				PeerID:           " peer-1 ",
				Mode:             "direct",
			},
		}

		mapped, err := req.ToResolveDeliveryTargetRequest(" brg-1 ")
		if err != nil {
			t.Fatalf("ToResolveDeliveryTargetRequest() error = %v", err)
		}
		if mapped.BridgeInstanceID != "brg-1" || mapped.PeerID != "peer-1" ||
			mapped.Mode != bridgepkg.DeliveryModeDirectSend {
			t.Fatalf("mapped target = %#v", mapped)
		}
	})
}

func TestBridgeTestDeliveryRequestRejectsBlankInstanceID(t *testing.T) {
	t.Parallel()

	t.Run("Should reject blank path instance id", func(t *testing.T) {
		t.Parallel()

		req := contract.BridgeTestDeliveryRequest{
			Target: contract.BridgeDeliveryTargetInput{PeerID: "peer-1"},
		}

		if _, err := req.ToResolveDeliveryTargetRequest("   "); err == nil {
			t.Fatal("ToResolveDeliveryTargetRequest() error = nil, want non-nil")
		}
	})
}

func TestUpdateBridgeRequestPreservesOptionalFields(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve optional update fields and clone raw payloads", func(t *testing.T) {
		t.Parallel()

		displayName := "Support Escalations"
		dmPolicy := bridgepkg.BridgeDMPolicyAllowlist
		rawProviderConfig := contract.BridgeProviderConfigPayload(`{"mode":"bot","tenant":"ws-alpha"}`)
		rawDefaults := contract.BridgeDeliveryDefaultsPayload(`{"mode":"reply","peer_id":"peer-1"}`)
		req := contract.UpdateBridgeRequest{
			DisplayName:      &displayName,
			DMPolicy:         &dmPolicy,
			RoutingPolicy:    &bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
			ProviderConfig:   &rawProviderConfig,
			DeliveryDefaults: &rawDefaults,
			Degradation: &bridgepkg.BridgeDegradation{
				Reason: bridgepkg.BridgeDegradationReasonAuthFailed,
			},
		}

		mapped, err := req.ToUpdateInstanceRequest("brg-1")
		if err != nil {
			t.Fatalf("ToUpdateInstanceRequest() error = %v", err)
		}
		if mapped.ID != "brg-1" {
			t.Fatalf("mapped.ID = %q, want brg-1", mapped.ID)
		}
		if mapped.DisplayName == nil || *mapped.DisplayName != displayName {
			t.Fatalf("mapped.DisplayName = %#v", mapped.DisplayName)
		}
		if mapped.DMPolicy == nil || *mapped.DMPolicy != bridgepkg.BridgeDMPolicyAllowlist {
			t.Fatalf("mapped.DMPolicy = %#v", mapped.DMPolicy)
		}
		if mapped.RoutingPolicy == nil || !mapped.RoutingPolicy.IncludePeer || !mapped.RoutingPolicy.IncludeThread {
			t.Fatalf("mapped.RoutingPolicy = %#v", mapped.RoutingPolicy)
		}
		if mapped.ProviderConfig == nil || string(*mapped.ProviderConfig) != string(rawProviderConfig) {
			t.Fatalf(
				"mapped.ProviderConfig = %s, want %s",
				stringValue(mapped.ProviderConfig),
				string(rawProviderConfig),
			)
		}
		if mapped.DeliveryDefaults == nil || string(*mapped.DeliveryDefaults) != string(rawDefaults) {
			t.Fatalf("mapped.DeliveryDefaults = %s, want %s", stringValue(mapped.DeliveryDefaults), string(rawDefaults))
		}
		if mapped.Degradation == nil || mapped.Degradation.Reason != bridgepkg.BridgeDegradationReasonAuthFailed {
			t.Fatalf("mapped.Degradation = %#v", mapped.Degradation)
		}

		rawProviderConfig[0] = '['
		rawDefaults[0] = '['
		if string(*mapped.ProviderConfig) != `{"mode":"bot","tenant":"ws-alpha"}` {
			t.Fatalf("mapped.ProviderConfig mutated with source slice = %s", string(*mapped.ProviderConfig))
		}
		if string(*mapped.DeliveryDefaults) != `{"mode":"reply","peer_id":"peer-1"}` {
			t.Fatalf("mapped.DeliveryDefaults mutated with source slice = %s", string(*mapped.DeliveryDefaults))
		}
	})
}

func TestBridgeRequestsKeepProviderConfigDistinctFromDeliveryDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should keep provider config distinct from delivery defaults", func(t *testing.T) {
		t.Parallel()

		createReq := contract.CreateBridgeRequest{
			Scope:            bridgepkg.ScopeGlobal,
			Platform:         "telegram",
			ExtensionName:    "ext-telegram",
			DisplayName:      "Support",
			Enabled:          true,
			RoutingPolicy:    bridgepkg.RoutingPolicy{IncludePeer: true},
			ProviderConfig:   contract.BridgeProviderConfigPayload(`{"mode":"bot","tenant":"acme"}`),
			DeliveryDefaults: contract.BridgeDeliveryDefaultsPayload(`{"peer_id":"peer-default","mode":"reply"}`),
		}

		createMapped, err := createReq.ToCreateInstanceRequest()
		if err != nil {
			t.Fatalf("ToCreateInstanceRequest() error = %v", err)
		}
		if got, want := string(createMapped.ProviderConfig), `{"mode":"bot","tenant":"acme"}`; got != want {
			t.Fatalf("createMapped.ProviderConfig = %s, want %s", got, want)
		}
		if got, want := string(
			createMapped.DeliveryDefaults,
		), `{"mode":"reply","peer_id":"peer-default"}`; got != want {
			t.Fatalf("createMapped.DeliveryDefaults = %s, want %s", got, want)
		}

		updateProviderConfig := contract.BridgeProviderConfigPayload(`{"mode":"comments"}`)
		updateDeliveryDefaults := contract.BridgeDeliveryDefaultsPayload(`{"group_id":"ops","mode":"direct-send"}`)
		updateReq := contract.UpdateBridgeRequest{
			ProviderConfig:   &updateProviderConfig,
			DeliveryDefaults: &updateDeliveryDefaults,
		}

		updateMapped, err := updateReq.ToUpdateInstanceRequest("brg-1")
		if err != nil {
			t.Fatalf("ToUpdateInstanceRequest() error = %v", err)
		}
		if updateMapped.ProviderConfig == nil || string(*updateMapped.ProviderConfig) != `{"mode":"comments"}` {
			t.Fatalf("updateMapped.ProviderConfig = %s", stringValue(updateMapped.ProviderConfig))
		}
		if updateMapped.DeliveryDefaults == nil ||
			string(*updateMapped.DeliveryDefaults) != `{"group_id":"ops","mode":"direct-send"}` {
			t.Fatalf("updateMapped.DeliveryDefaults = %s", stringValue(updateMapped.DeliveryDefaults))
		}
	})
}

func TestBridgeRequestsRejectUnsupportedProviderConfigAndDeliveryDefaultsShapes(t *testing.T) {
	t.Parallel()

	t.Run("Should reject scalar provider config", func(t *testing.T) {
		t.Parallel()

		badProviderConfig := contract.CreateBridgeRequest{
			Scope:          bridgepkg.ScopeGlobal,
			Platform:       "telegram",
			ExtensionName:  "ext-telegram",
			DisplayName:    "Support",
			Enabled:        true,
			RoutingPolicy:  bridgepkg.RoutingPolicy{IncludePeer: true},
			ProviderConfig: contract.BridgeProviderConfigPayload(`"bot"`),
		}
		if _, err := badProviderConfig.ToCreateInstanceRequest(); err == nil {
			t.Fatal("ToCreateInstanceRequest(provider_config string) error = nil, want non-nil")
		}
	})

	t.Run("Should preserve provider-specific delivery defaults fields", func(t *testing.T) {
		t.Parallel()

		providerDefaults := contract.UpdateBridgeRequest{
			DeliveryDefaults: new(contract.BridgeDeliveryDefaultsPayload(
				`{"mode":"reply","parse_mode":"markdown","thread_id":"thread-marketing"}`,
			)),
		}
		mapped, err := providerDefaults.ToUpdateInstanceRequest("brg-1")
		if err != nil {
			t.Fatalf("ToUpdateInstanceRequest(provider delivery defaults) error = %v", err)
		}
		if mapped.DeliveryDefaults == nil ||
			string(*mapped.DeliveryDefaults) !=
				`{"mode":"reply","parse_mode":"markdown","thread_id":"thread-marketing"}` {
			t.Fatalf(
				"mapped.DeliveryDefaults = %s, want provider field preserved",
				stringValue(mapped.DeliveryDefaults),
			)
		}
	})
}

func TestBridgeProviderConfigRejectsUnsafeSecretMaterial(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unsafe provider config during unmarshal", func(t *testing.T) {
		t.Parallel()

		var payload contract.BridgeProviderConfigPayload
		if err := payload.UnmarshalJSON([]byte(`{"provider_token":"sk-secret"}`)); !errors.Is(
			err,
			contract.ErrUnsafeBridgeProviderConfigPayload,
		) {
			t.Fatalf(
				"ProviderConfig.UnmarshalJSON() error = %v, want ErrUnsafeBridgeProviderConfigPayload",
				err,
			)
		}
	})

	t.Run("Should reject unsafe provider config during create mapping", func(t *testing.T) {
		t.Parallel()

		req := contract.CreateBridgeRequest{
			Scope:          bridgepkg.ScopeGlobal,
			Platform:       "telegram",
			ExtensionName:  "ext-telegram",
			DisplayName:    "Support",
			Enabled:        true,
			RoutingPolicy:  bridgepkg.RoutingPolicy{IncludePeer: true},
			ProviderConfig: contract.BridgeProviderConfigPayload(`{"access_token":"sk-secret"}`),
		}
		if _, err := req.ToCreateInstanceRequest(); !errors.Is(err, contract.ErrUnsafeBridgeProviderConfigPayload) {
			t.Fatalf("ToCreateInstanceRequest() error = %v, want ErrUnsafeBridgeProviderConfigPayload", err)
		}
	})

	t.Run("Should reject unsafe provider config during update mapping", func(t *testing.T) {
		t.Parallel()

		providerConfig := contract.BridgeProviderConfigPayload(
			`{"client_secret_ref":"vault:bridges/brg-1/client-secret"}`,
		)
		req := contract.UpdateBridgeRequest{ProviderConfig: &providerConfig}
		if _, err := req.ToUpdateInstanceRequest("brg-1"); !errors.Is(
			err,
			contract.ErrUnsafeBridgeProviderConfigPayload,
		) {
			t.Fatalf("ToUpdateInstanceRequest() error = %v, want ErrUnsafeBridgeProviderConfigPayload", err)
		}
	})

	t.Run("Should reject unsafe provider config during response marshal", func(t *testing.T) {
		t.Parallel()

		payload := contract.BridgePayload{
			ProviderConfig: contract.BridgeProviderConfigPayload(
				`{"tenant":"acme","debug":"vault:bridges/brg-1/token"}`,
			),
		}
		if _, err := json.Marshal(payload); !errors.Is(err, contract.ErrUnsafeBridgeProviderConfigPayload) {
			t.Fatalf("json.Marshal(BridgePayload) error = %v, want ErrUnsafeBridgeProviderConfigPayload", err)
		}
	})
}

func TestUpdateBridgeRequestRejectsBlankDisplayName(t *testing.T) {
	t.Parallel()

	t.Run("Should reject blank display name", func(t *testing.T) {
		t.Parallel()

		displayName := "   "
		req := contract.UpdateBridgeRequest{DisplayName: &displayName}

		if _, err := req.ToUpdateInstanceRequest("brg-1"); err == nil {
			t.Fatal("ToUpdateInstanceRequest() error = nil, want non-nil")
		}
	})
}

func TestBridgeInstanceMismatchErrorSupportsErrorsIs(t *testing.T) {
	t.Parallel()

	t.Run("Should support errors Is", func(t *testing.T) {
		t.Parallel()

		err := contract.ErrBridgeInstanceMismatch
		if err.Error() != "bridge instance id must match request path" {
			t.Fatalf("Error() = %q", err.Error())
		}
		if !errors.Is(err, contract.ErrBridgeInstanceMismatch) {
			t.Fatal("expected errors.Is to match ErrBridgeInstanceMismatch")
		}
	})
}

func TestBridgeJSONPayloadsMarshalAndUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		validate func(*testing.T, []byte)
	}{
		{
			name: "provider config object round-trips compactly",
			raw:  "{\n  \"tenant\": \"acme\",\n  \"mode\": \"bot\"\n}",
			validate: func(t *testing.T, encoded []byte) {
				t.Helper()
				if got, want := string(encoded), `{"tenant":"acme","mode":"bot"}`; got != want {
					t.Fatalf("provider config encoded = %s, want %s", got, want)
				}
			},
		},
		{
			name: "provider config blank marshals to null",
			raw:  "",
			validate: func(t *testing.T, encoded []byte) {
				t.Helper()
				if got, want := string(encoded), "null"; got != want {
					t.Fatalf("provider config encoded = %s, want %s", got, want)
				}
			},
		},
		{
			name: "delivery defaults object round-trips compactly",
			raw:  "{\n  \"peer_id\": \"peer-1\",\n  \"mode\": \"reply\",\n  \"parse_mode\": \"markdown\"\n}",
			validate: func(t *testing.T, encoded []byte) {
				t.Helper()
				got := string(encoded)
				want := `{"mode":"reply","parse_mode":"markdown","peer_id":"peer-1"}`
				if got != want {
					t.Fatalf("delivery defaults encoded = %s, want %s", got, want)
				}
			},
		},
		{
			name: "delivery defaults null stays null",
			raw:  "null",
			validate: func(t *testing.T, encoded []byte) {
				t.Helper()
				if got, want := string(encoded), "null"; got != want {
					t.Fatalf("delivery defaults encoded = %s, want %s", got, want)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var providerPayload contract.BridgeProviderConfigPayload
			if err := providerPayload.UnmarshalJSON([]byte(tt.raw)); err == nil {
				encoded, marshalErr := providerPayload.MarshalJSON()
				if marshalErr != nil {
					t.Fatalf("provider MarshalJSON() error = %v", marshalErr)
				}
				if tt.name == "provider config object round-trips compactly" ||
					tt.name == "provider config blank marshals to null" {
					tt.validate(t, encoded)
				}
			}

			var deliveryPayload contract.BridgeDeliveryDefaultsPayload
			if err := deliveryPayload.UnmarshalJSON([]byte(tt.raw)); err == nil {
				encoded, marshalErr := deliveryPayload.MarshalJSON()
				if marshalErr != nil {
					t.Fatalf("delivery MarshalJSON() error = %v", marshalErr)
				}
				if tt.name == "delivery defaults object round-trips compactly" ||
					tt.name == "delivery defaults null stays null" {
					tt.validate(t, encoded)
				}
			}
		})
	}
}

func TestBridgeJSONPayloadsRejectInvalidShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  string
		raw     string
		wantErr string
	}{
		{
			name:    "provider config rejects scalar",
			target:  "provider",
			raw:     `"bot"`,
			wantErr: "bridge provider config must be a JSON object or null",
		},
		{
			name:    "provider config rejects invalid json",
			target:  "provider",
			raw:     "{not-json",
			wantErr: "bridge provider config must be valid JSON",
		},
		{
			name:    "delivery defaults rejects non-string field",
			target:  "delivery",
			raw:     `{"peer_id":7}`,
			wantErr: `bridge instance delivery defaults field "peer_id" must be a string`,
		},
		{
			name:    "Should reject unsupported progress mode",
			target:  "delivery",
			raw:     `{"progress":{"tool_progress":"sometimes","grouping":"accumulate","typing":true,"reactions":true}}`,
			wantErr: `unsupported progress tool_progress "sometimes"`,
		},
		{
			name:    "Should reject unsupported progress grouping",
			target:  "delivery",
			raw:     `{"progress":{"tool_progress":"all","grouping":"threaded","typing":true,"reactions":true}}`,
			wantErr: `unsupported progress grouping "threaded"`,
		},
		{
			name:    "Should reject non-boolean progress typing",
			target:  "delivery",
			raw:     `{"progress":{"tool_progress":"all","grouping":"accumulate","typing":"yes","reactions":true}}`,
			wantErr: `cannot unmarshal string into Go struct field ProgressConfig.typing of type bool`,
		},
		{
			name:    "Should reject non-boolean progress reactions",
			target:  "delivery",
			raw:     `{"progress":{"tool_progress":"all","grouping":"accumulate","typing":true,"reactions":1}}`,
			wantErr: `cannot unmarshal number into Go struct field ProgressConfig.reactions of type bool`,
		},
		{
			name:    "Should reject unknown progress fields",
			target:  "delivery",
			raw:     `{"progress":{"tool_progress":"all","grouping":"accumulate","typing":true,"reactions":true,"style":"compact"}}`,
			wantErr: `unknown field "style"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			switch tt.target {
			case "provider":
				var payload contract.BridgeProviderConfigPayload
				if err := payload.UnmarshalJSON(
					[]byte(tt.raw),
				); err == nil ||
					!strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("provider UnmarshalJSON(%s) error = %v, want substring %q", tt.raw, err, tt.wantErr)
				}
			case "delivery":
				var payload contract.BridgeDeliveryDefaultsPayload
				if err := payload.UnmarshalJSON(
					[]byte(tt.raw),
				); err == nil ||
					!strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("delivery UnmarshalJSON(%s) error = %v, want substring %q", tt.raw, err, tt.wantErr)
				}
			default:
				t.Fatalf("unknown target %q", tt.target)
			}
		})
	}
}

func TestBridgeDeliveryDefaultsExposeTypedProgress(t *testing.T) {
	t.Parallel()

	t.Run("Should round-trip typed progress with existing target defaults", func(t *testing.T) {
		t.Parallel()

		var payload contract.BridgeDeliveryDefaultsPayload
		if err := payload.UnmarshalJSON([]byte(
			`{"peer_id":"peer-1","parse_mode":"MarkdownV2","mode":" REPLY_SEND ","progress":{"tool_progress":" ALL ","grouping":" Separate ","typing":true,"reactions":false}}`,
		)); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		encoded, err := payload.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var decoded struct {
			PeerID    string                               `json:"peer_id"`
			ParseMode string                               `json:"parse_mode"`
			Mode      bridgepkg.DeliveryMode               `json:"mode"`
			Progress  contract.BridgeProgressConfigPayload `json:"progress"`
		}
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.PeerID != "peer-1" || decoded.ParseMode != "MarkdownV2" ||
			decoded.Mode != bridgepkg.DeliveryModeReply {
			t.Fatalf("decoded target defaults = %#v", decoded)
		}
		if decoded.Progress.ToolProgress != bridgepkg.ProgressModeAll ||
			decoded.Progress.Grouping != bridgepkg.ProgressGroupingSeparate ||
			!decoded.Progress.Typing || decoded.Progress.Reactions {
			t.Fatalf("decoded progress = %#v", decoded.Progress)
		}
	})
}

func TestPutBridgeSecretBindingRequestValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize binding fields and keep write-only value on request", func(t *testing.T) {
		t.Parallel()

		secretValue := "telegram-token"
		req := contract.PutBridgeSecretBindingRequest{
			SecretRef:   " vault:bridges/brg-1/bot_token ",
			Kind:        " token ",
			SecretValue: &secretValue,
		}

		binding, err := req.ToBridgeSecretBinding(" brg-1 ", " bot_token ")
		if err != nil {
			t.Fatalf("ToBridgeSecretBinding() error = %v", err)
		}
		if binding.BridgeInstanceID != "brg-1" || binding.BindingName != "bot_token" ||
			binding.SecretRef != "vault:bridges/brg-1/bot_token" ||
			binding.Kind != "token" {
			t.Fatalf("binding = %#v", binding)
		}
		if req.SecretValue == nil || *req.SecretValue != "telegram-token" {
			t.Fatalf("SecretValue = %v, want write-only value", req.SecretValue)
		}
	})

	t.Run("Should reject blank binding kind", func(t *testing.T) {
		t.Parallel()

		req := contract.PutBridgeSecretBindingRequest{
			SecretRef: "vault:bridges/brg-1/bot_token",
			Kind:      "   ",
		}
		if _, err := req.ToBridgeSecretBinding("brg-1", "bot_token"); err == nil {
			t.Fatal("ToBridgeSecretBinding(blank kind) error = nil, want non-nil")
		}
	})
}

func stringValue(payload *json.RawMessage) string {
	if payload == nil {
		return "<nil>"
	}
	return string(*payload)
}
