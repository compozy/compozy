package bridges

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

func TestBridgeInstanceToContract(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve every field, slice nilness, and copy ownership", func(t *testing.T) {
		t.Parallel()

		createdAt := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
		updatedAt := createdAt.Add(time.Minute)
		source := BridgeInstance{
			ID: "brg-contract", Scope: ScopeWorkspace, WorkspaceID: "ws-contract",
			Platform: "slack", ExtensionName: "ext-contract", DisplayName: "Contract",
			Source: BridgeInstanceSourcePackage, Enabled: true, Status: BridgeStatusDegraded,
			DMPolicy:             BridgeDMPolicyPairing,
			RoutingPolicy:        RoutingPolicy{IncludePeer: true, IncludeThread: true, IncludeGroup: true},
			ProviderConfig:       json.RawMessage(`{"mode":"bot"}`),
			DeliveryDefaults:     json.RawMessage(`{"peer_id":"peer-1"}`),
			NotificationSuppress: true,
			Degradation: &BridgeDegradation{
				Reason: BridgeDegradationReasonRateLimited, Message: "retry later",
			},
			CreatedAt: createdAt, UpdatedAt: updatedAt,
		}
		want := bridgecontract.BridgeInstance{
			ID: "brg-contract", Scope: bridgecontract.ScopeWorkspace, WorkspaceID: "ws-contract",
			Platform: "slack", ExtensionName: "ext-contract", DisplayName: "Contract",
			Source: bridgecontract.BridgeInstanceSourcePackage, Enabled: true,
			Status: bridgecontract.BridgeStatusDegraded, DMPolicy: bridgecontract.BridgeDMPolicyPairing,
			RoutingPolicy: bridgecontract.RoutingPolicy{
				IncludePeer: true, IncludeThread: true, IncludeGroup: true,
			},
			ProviderConfig:       json.RawMessage(`{"mode":"bot"}`),
			DeliveryDefaults:     json.RawMessage(`{"peer_id":"peer-1"}`),
			NotificationSuppress: true,
			Degradation: &bridgecontract.BridgeDegradation{
				Reason: bridgecontract.BridgeDegradationReasonRateLimited, Message: "retry later",
			},
			CreatedAt: createdAt, UpdatedAt: updatedAt,
		}

		converted := BridgeInstanceToContract(source)
		if !reflect.DeepEqual(converted, want) {
			t.Fatalf("BridgeInstanceToContract() = %#v, want %#v", converted, want)
		}

		converted.ProviderConfig[0] = '['
		converted.DeliveryDefaults[0] = '['
		converted.Degradation.Message = "mutated"
		if got, expected := string(source.ProviderConfig), `{"mode":"bot"}`; got != expected {
			t.Fatalf("source.ProviderConfig = %q after contract mutation, want %q", got, expected)
		}
		if got, expected := string(source.DeliveryDefaults), `{"peer_id":"peer-1"}`; got != expected {
			t.Fatalf("source.DeliveryDefaults = %q after contract mutation, want %q", got, expected)
		}
		if got, expected := source.Degradation.Message, "retry later"; got != expected {
			t.Fatalf("source.Degradation.Message = %q after contract mutation, want %q", got, expected)
		}

		nilSlices := BridgeInstanceToContract(BridgeInstance{})
		if nilSlices.ProviderConfig != nil || nilSlices.DeliveryDefaults != nil {
			t.Fatalf("BridgeInstanceToContract(nil slices) = %#v, want nil slices", nilSlices)
		}
		emptySlices := BridgeInstanceToContract(BridgeInstance{
			ProviderConfig: json.RawMessage{}, DeliveryDefaults: json.RawMessage{},
		})
		if emptySlices.ProviderConfig == nil || emptySlices.DeliveryDefaults == nil {
			t.Fatalf("BridgeInstanceToContract(empty slices) = %#v, want non-nil empty slices", emptySlices)
		}
	})
}

func TestBridgeSecretBindingValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should accept vault bindings and reject malformed references", func(t *testing.T) {
		t.Parallel()

		valid := BridgeSecretBinding{
			BridgeInstanceID: "brg-1",
			BindingName:      "bot_token",
			SecretRef:        "vault:bridges/brg-1/bot-token",
			Kind:             "token",
		}
		if err := valid.Validate(); err != nil {
			t.Fatalf("BridgeSecretBinding.Validate(valid) error = %v", err)
		}

		invalidName := valid
		invalidName.BindingName = " "
		if err := invalidName.Validate(); err == nil {
			t.Fatal("BridgeSecretBinding.Validate(empty name) error = nil, want non-nil")
		}

		invalidVault := valid
		invalidVault.SecretRef = ""
		if err := invalidVault.Validate(); err == nil {
			t.Fatal("BridgeSecretBinding.Validate(empty secret ref) error = nil, want non-nil")
		}

		envRef := valid
		envRef.SecretRef = "env:TG_TOKEN"
		if err := envRef.Validate(); err == nil {
			t.Fatal("BridgeSecretBinding.Validate(env ref) error = nil, want non-nil")
		}
	})
}

func TestBridgeRouteCanonicalizeAndDedupValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should canonicalize routes and reject non-increasing dedup expiry", func(t *testing.T) {
		t.Parallel()

		route := BridgeRoute{
			Scope:            ScopeWorkspace,
			WorkspaceID:      "ws-1",
			BridgeInstanceID: "brg-1",
			PeerID:           "peer-1",
			SessionID:        "sess-1",
			AgentName:        "coder",
		}
		canonical, err := route.Canonicalize()
		if err != nil {
			t.Fatalf("BridgeRoute.Canonicalize() error = %v", err)
		}
		if canonical.RoutingKeyHash == "" {
			t.Fatal("BridgeRoute.Canonicalize() routing key hash = empty, want non-empty")
		}

		record := IngestDedupRecord{
			IdempotencyKey:   "idem-1",
			BridgeInstanceID: "brg-1",
			ReceivedAt:       time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC),
			ExpiresAt:        time.Date(2026, 4, 10, 10, 5, 0, 0, time.UTC),
		}
		if err := record.Validate(); err != nil {
			t.Fatalf("IngestDedupRecord.Validate(valid) error = %v", err)
		}

		record.ExpiresAt = record.ReceivedAt
		if err := record.Validate(); err == nil {
			t.Fatal("IngestDedupRecord.Validate(equal expiry) error = nil, want non-nil")
		}
	})
}

func TestBridgeSecretSlotAndConfigSchemaValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should validate provider secret slots and optional schema hints", func(t *testing.T) {
		t.Parallel()

		slot := BridgeSecretSlot{Name: "bot_token", Description: "Bot token", Required: true}
		if err := slot.Validate(); err != nil {
			t.Fatalf("BridgeSecretSlot.Validate(valid) error = %v", err)
		}
		if err := (BridgeSecretSlot{}).Validate(); err == nil {
			t.Fatal("BridgeSecretSlot.Validate(empty) error = nil, want non-nil")
		}

		schema := BridgeProviderConfigSchema{Schema: "agh.bridge.slack", Version: "v1"}
		if err := schema.Validate(); err != nil {
			t.Fatalf("BridgeProviderConfigSchema.Validate(valid) error = %v", err)
		}
		if err := (BridgeProviderConfigSchema{}).Validate(); err != nil {
			t.Fatalf("BridgeProviderConfigSchema.Validate(zero) error = %v", err)
		}
	})
}

func TestBridgeRouteValidateHashMismatch(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a route whose persisted hash disagrees with its identity", func(t *testing.T) {
		t.Parallel()

		route := BridgeRoute{
			RoutingKeyHash:   "wrong",
			Scope:            ScopeGlobal,
			BridgeInstanceID: "brg-1",
			PeerID:           "peer-1",
			SessionID:        "sess-1",
			AgentName:        "coder",
		}
		if err := route.Validate(); err == nil {
			t.Fatal("BridgeRoute.Validate(hash mismatch) error = nil, want non-nil")
		}
	})
}

func TestDeliveryAckWireSequencePresence(t *testing.T) {
	t.Parallel()

	event := DeliveryEvent{DeliveryID: "del-zero", Seq: 0}
	tests := []struct {
		name          string
		payload       string
		wantDecodeErr bool
		wantValid     bool
	}{
		{
			name:      "Should accept an explicit zero sequence",
			payload:   `{"delivery_id":"del-zero","seq":0}`,
			wantValid: true,
		},
		{
			name:    "Should reject an omitted sequence",
			payload: `{"delivery_id":"del-zero"}`,
		},
		{
			name:    "Should reject a null sequence",
			payload: `{"delivery_id":"del-zero","seq":null}`,
		},
		{
			name:          "Should reject a string sequence during decoding",
			payload:       `{"delivery_id":"del-zero","seq":"0"}`,
			wantDecodeErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var ack DeliveryAck
			err := json.Unmarshal([]byte(test.payload), &ack)
			if test.wantDecodeErr {
				if err == nil {
					t.Fatal("json.Unmarshal(DeliveryAck) error = nil, want non-nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("json.Unmarshal(DeliveryAck) error = %v", err)
			}
			err = ack.ValidateFor(event)
			if test.wantValid && err != nil {
				t.Fatalf("DeliveryAck.ValidateFor() error = %v, want nil", err)
			}
			if !test.wantValid && (err == nil || !strings.Contains(err.Error(), "sequence is required")) {
				t.Fatalf("DeliveryAck.ValidateFor() error = %v, want required sequence error", err)
			}
		})
	}
}
