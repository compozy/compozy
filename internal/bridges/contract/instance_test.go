package contract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBridgeInstanceContractValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize and validate a complete workspace instance", func(t *testing.T) {
		t.Parallel()

		instance := validContractBridgeInstance()
		instance.ID = " brg-1 "
		instance.Scope = " WORKSPACE "
		instance.WorkspaceID = " ws-1 "
		instance.Platform = " slack "
		instance.ExtensionName = " slack-adapter "
		instance.DisplayName = " Support "
		instance.Source = " PACKAGE "
		instance.Status = " DEGRADED "
		instance.DMPolicy = " PAIRING "
		instance.ProviderConfig = json.RawMessage(` {"mode":"bot"} `)
		instance.DeliveryDefaults = json.RawMessage(` {"mode":"reply"} `)
		instance.Degradation = &BridgeDegradation{
			Reason: " RATE_LIMITED ", Message: " retry later ",
		}

		if err := instance.Validate(); err != nil {
			t.Fatalf("BridgeInstance.Validate() error = %v", err)
		}
		normalized := NormalizeBridgeInstance(instance)
		if normalized.ID != "brg-1" || normalized.Scope != ScopeWorkspace || normalized.WorkspaceID != "ws-1" ||
			normalized.Source != BridgeInstanceSourcePackage || normalized.Status != BridgeStatusDegraded ||
			normalized.DMPolicy != BridgeDMPolicyPairing {
			t.Fatalf("NormalizeBridgeInstance() = %#v, want canonical identity and enums", normalized)
		}
		if got, want := string(normalized.ProviderConfig), `{"mode":"bot"}`; got != want {
			t.Fatalf("normalized.ProviderConfig = %q, want %q", got, want)
		}
		if normalized.Degradation == nil || normalized.Degradation.Message != "retry later" {
			t.Fatalf("normalized.Degradation = %#v, want trimmed degradation", normalized.Degradation)
		}

		normalized.ProviderConfig[0] = '['
		normalized.DeliveryDefaults[0] = '['
		normalized.Degradation.Message = "mutated"
		if got, want := string(instance.ProviderConfig), ` {"mode":"bot"} `; got != want {
			t.Fatalf("source.ProviderConfig = %q after normalized mutation, want %q", got, want)
		}
		if got, want := string(instance.DeliveryDefaults), ` {"mode":"reply"} `; got != want {
			t.Fatalf("source.DeliveryDefaults = %q after normalized mutation, want %q", got, want)
		}
		if got, want := instance.Degradation.Message, " retry later "; got != want {
			t.Fatalf("source.Degradation.Message = %q after normalized mutation, want %q", got, want)
		}

		empty := NormalizeBridgeInstance(BridgeInstance{})
		if empty.ProviderConfig != nil || empty.DeliveryDefaults != nil || empty.Degradation != nil {
			t.Fatalf("NormalizeBridgeInstance(zero) = %#v, want nil optional values", empty)
		}
	})

	t.Run("Should normalize provider configuration into owned compact JSON", func(t *testing.T) {
		t.Parallel()

		source := json.RawMessage(` { "mode": "bot" } `)
		normalized, err := NormalizeProviderConfigJSON(source)
		if err != nil {
			t.Fatalf("NormalizeProviderConfigJSON() error = %v", err)
		}
		if got, want := string(normalized), `{"mode":"bot"}`; got != want {
			t.Fatalf("NormalizeProviderConfigJSON() = %q, want %q", got, want)
		}
		normalized[0] = '['
		if got, want := string(source), ` { "mode": "bot" } `; got != want {
			t.Fatalf("source provider config = %q after normalized mutation, want %q", got, want)
		}
		nilConfig, err := NormalizeProviderConfigJSON(nil)
		if err != nil || nilConfig != nil {
			t.Fatalf("NormalizeProviderConfigJSON(nil) = (%v, %v), want (nil, nil)", nilConfig, err)
		}
	})

	t.Run("Should reject invalid instance fields and lifecycle combinations", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			mutate func(*BridgeInstance)
			want   string
		}{
			{name: "Should reject a missing id", mutate: func(v *BridgeInstance) { v.ID = " " }, want: "instance id"},
			{
				name:   "Should reject a missing workspace",
				mutate: func(v *BridgeInstance) { v.WorkspaceID = "" },
				want:   "requires workspace",
			},
			{
				name:   "Should reject a missing platform",
				mutate: func(v *BridgeInstance) { v.Platform = "" },
				want:   "platform",
			},
			{
				name:   "Should reject a missing extension",
				mutate: func(v *BridgeInstance) { v.ExtensionName = "" },
				want:   "extension name",
			},
			{
				name:   "Should reject a missing display name",
				mutate: func(v *BridgeInstance) { v.DisplayName = "" },
				want:   "display name",
			},
			{
				name:   "Should reject an invalid source",
				mutate: func(v *BridgeInstance) { v.Source = "legacy" },
				want:   "source",
			},
			{
				name:   "Should reject an invalid status",
				mutate: func(v *BridgeInstance) { v.Status = "unknown" },
				want:   "status",
			},
			{
				name:   "Should reject disabled status while enabled",
				mutate: func(v *BridgeInstance) { v.Status = BridgeStatusDisabled },
				want:   "enabled",
			},
			{
				name:   "Should reject ready status while disabled",
				mutate: func(v *BridgeInstance) { v.Enabled = false },
				want:   "disabled",
			},
			{
				name:   "Should reject an invalid dm policy",
				mutate: func(v *BridgeInstance) { v.DMPolicy = "closed" },
				want:   "dm policy",
			},
			{
				name:   "Should reject an invalid routing policy",
				mutate: func(v *BridgeInstance) { v.RoutingPolicy = RoutingPolicy{IncludeThread: true} },
				want:   "thread",
			},
			{
				name:   "Should reject invalid provider JSON",
				mutate: func(v *BridgeInstance) { v.ProviderConfig = json.RawMessage(`{`) },
				want:   "valid JSON",
			},
			{
				name:   "Should reject a provider config array",
				mutate: func(v *BridgeInstance) { v.ProviderConfig = json.RawMessage(`[]`) },
				want:   "JSON object",
			},
			{
				name:   "Should reject a scalar provider config",
				mutate: func(v *BridgeInstance) { v.ProviderConfig = json.RawMessage(`"bot"`) },
				want:   "JSON object",
			},
			{name: "Should reject an operator api base url", mutate: func(v *BridgeInstance) {
				v.ProviderConfig = json.RawMessage(`{"api_base_url":"https://operator.test"}`)
			}, want: "operator-owned"},
			{name: "Should reject an operator oauth token url", mutate: func(v *BridgeInstance) {
				v.ProviderConfig = json.RawMessage(`{"oauth_token_url":"https://operator.test"}`)
			}, want: "operator-owned"},
			{name: "Should reject an operator service url", mutate: func(v *BridgeInstance) {
				v.ProviderConfig = json.RawMessage(`{"service_url":"https://operator.test"}`)
			}, want: "operator-owned"},
			{name: "Should reject an operator openid metadata url", mutate: func(v *BridgeInstance) {
				v.ProviderConfig = json.RawMessage(`{"openid_metadata_url":"https://operator.test"}`)
			}, want: "operator-owned"},
			{name: "Should reject a nested operator token url", mutate: func(v *BridgeInstance) {
				v.ProviderConfig = json.RawMessage(`{"nested":[{"token_url":"https://operator.test"}]}`)
			}, want: "operator-owned"},
			{name: "Should reject truncated delivery defaults", mutate: func(v *BridgeInstance) {
				v.DeliveryDefaults = json.RawMessage(`{`)
			}, want: "valid JSON"},
			{name: "Should reject invalid delivery defaults", mutate: func(v *BridgeInstance) {
				v.DeliveryDefaults = json.RawMessage(`{"progress":{"tool_progress":"sometimes"}}`)
			}, want: "progress tool_progress"},
			{
				name:   "Should reject a missing degradation reason",
				mutate: func(v *BridgeInstance) { v.Degradation = &BridgeDegradation{Message: "broken"} },
				want:   "reason",
			},
			{name: "Should reject degradation on a ready instance", mutate: func(v *BridgeInstance) {
				v.Status = BridgeStatusReady
				v.Degradation = &BridgeDegradation{Reason: BridgeDegradationReasonAuthFailed}
			}, want: "requires degraded"},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				instance := validContractBridgeInstance()
				test.mutate(&instance)
				err := instance.Validate()
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("BridgeInstance.Validate() error = %v, want substring %q", err, test.want)
				}
			})
		}
	})

	t.Run("Should validate scope and workspace ownership", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			scope       Scope
			workspaceID string
			wantErr     bool
		}{
			{name: "Should accept global scope without a workspace", scope: ScopeGlobal},
			{name: "Should accept workspace scope with an id", scope: ScopeWorkspace, workspaceID: "ws-1"},
			{name: "Should reject workspace scope without an id", scope: ScopeWorkspace, wantErr: true},
			{
				name:        "Should reject global scope with a workspace",
				scope:       ScopeGlobal,
				workspaceID: "ws-1",
				wantErr:     true,
			},
			{name: "Should reject unsupported scope", scope: "tenant", wantErr: true},
			{name: "Should reject empty scope", scope: "", wantErr: true},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateScopeWorkspaceID(test.scope, test.workspaceID)
				if test.wantErr && err == nil {
					t.Fatal("ValidateScopeWorkspaceID() error = nil, want non-nil")
				}
				if !test.wantErr && err != nil {
					t.Fatalf("ValidateScopeWorkspaceID() error = %v, want nil", err)
				}
			})
		}
	})

	t.Run("Should validate the closed instance source contract", func(t *testing.T) {
		t.Parallel()

		sources := []struct {
			name  string
			value BridgeInstanceSource
		}{
			{name: "Should accept dynamic source", value: BridgeInstanceSourceDynamic},
			{name: "Should accept package source", value: BridgeInstanceSourcePackage},
		}
		for _, test := range sources {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.value.Validate(); err != nil {
					t.Fatalf("BridgeInstanceSource(%q).Validate() error = %v", test.value, err)
				}
			})
		}
		if err := (BridgeInstanceSource("unknown")).Validate(); err == nil {
			t.Fatal("BridgeInstanceSource(unknown).Validate() error = nil")
		}
		if err := (BridgeInstanceSource("")).Validate(); err == nil {
			t.Fatal("BridgeInstanceSource(empty).Validate() error = nil")
		}
	})

	t.Run("Should validate the closed bridge status contract", func(t *testing.T) {
		t.Parallel()

		statuses := []struct {
			name  string
			value BridgeStatus
		}{
			{name: "Should accept disabled status", value: BridgeStatusDisabled},
			{name: "Should accept starting status", value: BridgeStatusStarting},
			{name: "Should accept ready status", value: BridgeStatusReady},
			{name: "Should accept degraded status", value: BridgeStatusDegraded},
			{name: "Should accept auth-required status", value: BridgeStatusAuthRequired},
			{name: "Should accept error status", value: BridgeStatusError},
		}
		for _, test := range statuses {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.value.Validate(); err != nil {
					t.Fatalf("BridgeStatus(%q).Validate() error = %v", test.value, err)
				}
			})
		}
		if err := (BridgeStatus("unknown")).Validate(); err == nil {
			t.Fatal("BridgeStatus(unknown).Validate() error = nil")
		}
	})

	t.Run("Should validate the closed direct-message policy contract", func(t *testing.T) {
		t.Parallel()

		policies := []struct {
			name  string
			value BridgeDMPolicy
		}{
			{name: "Should accept the default dm policy", value: ""},
			{name: "Should accept open dm policy", value: BridgeDMPolicyOpen},
			{name: "Should accept allowlist dm policy", value: BridgeDMPolicyAllowlist},
			{name: "Should accept pairing dm policy", value: BridgeDMPolicyPairing},
		}
		for _, test := range policies {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.value.Validate(); err != nil {
					t.Fatalf("BridgeDMPolicy(%q).Validate() error = %v", test.value, err)
				}
			})
		}
		if err := (BridgeDMPolicy("closed")).Validate(); err == nil {
			t.Fatal("BridgeDMPolicy(closed).Validate() error = nil")
		}
	})

	t.Run("Should validate the closed degradation reason contract", func(t *testing.T) {
		t.Parallel()

		reasons := []struct {
			name  string
			value BridgeDegradationReason
		}{
			{name: "Should accept auth-failed degradation", value: BridgeDegradationReasonAuthFailed},
			{name: "Should accept rate-limited degradation", value: BridgeDegradationReasonRateLimited},
			{name: "Should accept webhook-invalid degradation", value: BridgeDegradationReasonWebhookInvalid},
			{name: "Should accept provider-timeout degradation", value: BridgeDegradationReasonProviderTimeout},
			{
				name:  "Should accept tenant-config-invalid degradation",
				value: BridgeDegradationReasonTenantConfigInvalid,
			},
		}
		for _, test := range reasons {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if err := test.value.Validate(); err != nil {
					t.Fatalf("BridgeDegradationReason(%q).Validate() error = %v", test.value, err)
				}
			})
		}
		if err := (BridgeDegradationReason("unknown")).Validate(); err == nil {
			t.Fatal("BridgeDegradationReason(unknown).Validate() error = nil")
		}
		if !(BridgeDegradation{}).IsZero() {
			t.Fatal("BridgeDegradation{}.IsZero() = false")
		}
		normalized := NormalizeBridgeDegradation(BridgeDegradation{
			Reason: " RATE_LIMITED ", Message: " retry later ",
		})
		if normalized.Reason != BridgeDegradationReasonRateLimited || normalized.Message != "retry later" {
			t.Fatalf("NormalizeBridgeDegradation() = %#v, want canonical degradation", normalized)
		}
	})

	t.Run("Should validate enabled and status lifecycle combinations", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			enabled bool
			status  BridgeStatus
			wantErr bool
		}{
			{name: "Should accept a disabled instance", status: BridgeStatusDisabled},
			{name: "Should accept an enabled ready instance", enabled: true, status: BridgeStatusReady},
			{name: "Should reject disabled with ready status", status: BridgeStatusReady, wantErr: true},
			{
				name:    "Should reject enabled with disabled status",
				enabled: true,
				status:  BridgeStatusDisabled,
				wantErr: true,
			},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				err := ValidateBridgeInstanceLifecycle(test.enabled, test.status)
				if test.wantErr && err == nil {
					t.Fatal("ValidateBridgeInstanceLifecycle() error = nil, want non-nil")
				}
				if !test.wantErr && err != nil {
					t.Fatalf("ValidateBridgeInstanceLifecycle() error = %v, want nil", err)
				}
			})
		}
	})

	t.Run("Should require a routing anchor when thread routing is enabled", func(t *testing.T) {
		t.Parallel()

		if err := (RoutingPolicy{IncludePeer: true, IncludeThread: true}).Validate(); err != nil {
			t.Fatalf("RoutingPolicy.Validate(valid) error = %v", err)
		}
		if err := (RoutingPolicy{IncludeThread: true}).Validate(); err == nil {
			t.Fatal("RoutingPolicy.Validate(thread only) error = nil")
		}
	})
}

func TestRoutingContractBuildSerializeAndHash(t *testing.T) {
	t.Parallel()

	t.Run("Should build every selected dimension and hash canonical identity", func(t *testing.T) {
		t.Parallel()

		instance := validContractBridgeInstance()
		key, err := BuildRoutingKey(instance, RoutingDimensions{
			PeerID: " peer-1 ", ThreadID: " thread-1 ", GroupID: " group-1 ",
		})
		if err != nil {
			t.Fatalf("BuildRoutingKey() error = %v", err)
		}
		if key.PeerID != "peer-1" || key.ThreadID != "thread-1" || key.GroupID != "group-1" ||
			key.Scope != ScopeWorkspace || key.WorkspaceID != "ws-1" {
			t.Fatalf("BuildRoutingKey() = %#v, want canonical selected dimensions", key)
		}
		serialized, err := key.Serialize()
		if err != nil {
			t.Fatalf("RoutingKey.Serialize() error = %v", err)
		}
		if !strings.Contains(serialized, `"workspace_id":"ws-1"`) {
			t.Fatalf("RoutingKey.Serialize() = %q, want workspace identity", serialized)
		}
		firstHash, err := key.Hash()
		if err != nil {
			t.Fatalf("RoutingKey.Hash() error = %v", err)
		}
		key.PeerID = " peer-1 "
		secondHash, err := key.Hash()
		if err != nil || firstHash != secondHash {
			t.Fatalf("RoutingKey.Hash() = (%q, %v), want stable %q", secondHash, err, firstHash)
		}

		normalized := NormalizeRoutingKey(RoutingKey{
			Scope: " WORKSPACE ", WorkspaceID: " ws-1 ", BridgeInstanceID: " brg-1 ",
			PeerID: " peer-1 ", ThreadID: " thread-1 ", GroupID: " group-1 ",
		})
		if normalized.Scope != ScopeWorkspace || normalized.WorkspaceID != "ws-1" ||
			normalized.BridgeInstanceID != "brg-1" || normalized.PeerID != "peer-1" ||
			normalized.ThreadID != "thread-1" || normalized.GroupID != "group-1" {
			t.Fatalf("NormalizeRoutingKey() = %#v, want canonical routing identity", normalized)
		}
	})

	t.Run("Should reject missing required dimensions and invalid base identity", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name       string
			dimensions RoutingDimensions
		}{
			{
				name:       "Should require a peer dimension",
				dimensions: RoutingDimensions{ThreadID: "thread-1", GroupID: "group-1"},
			},
			{
				name:       "Should require a thread dimension",
				dimensions: RoutingDimensions{PeerID: "peer-1", GroupID: "group-1"},
			},
			{
				name:       "Should require a group dimension",
				dimensions: RoutingDimensions{PeerID: "peer-1", ThreadID: "thread-1"},
			},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if _, err := BuildRoutingKey(validContractBridgeInstance(), test.dimensions); err == nil {
					t.Fatalf("BuildRoutingKey(%#v) error = nil, want required dimension error", test.dimensions)
				}
			})
		}
		instance := validContractBridgeInstance()
		instance.ID = ""
		if _, err := BuildRoutingKey(instance, RoutingDimensions{}); err == nil {
			t.Fatal("BuildRoutingKey(invalid instance) error = nil")
		}
		if err := (RoutingKey{Scope: ScopeGlobal}).Validate(); err == nil {
			t.Fatal("RoutingKey.Validate(missing bridge id) error = nil")
		}
		if _, err := (RoutingKey{}).Serialize(); err == nil {
			t.Fatal("RoutingKey.Serialize(invalid) error = nil")
		}
	})

	t.Run("Should discard dimensions excluded by a peer-only policy", func(t *testing.T) {
		t.Parallel()

		instance := validContractBridgeInstance()
		instance.RoutingPolicy = RoutingPolicy{IncludePeer: true}
		key, err := BuildRoutingKey(instance, RoutingDimensions{
			PeerID: "peer-1", ThreadID: "thread-ignored", GroupID: "group-ignored",
		})
		if err != nil {
			t.Fatalf("BuildRoutingKey() error = %v", err)
		}
		if key.PeerID != "peer-1" || key.ThreadID != "" || key.GroupID != "" {
			t.Fatalf("BuildRoutingKey() = %#v, want only the selected peer dimension", key)
		}
	})

	t.Run("Should produce distinct hashes for distinct selected threads", func(t *testing.T) {
		t.Parallel()

		instance := validContractBridgeInstance()
		instance.RoutingPolicy = RoutingPolicy{IncludePeer: true, IncludeThread: true}
		first, err := BuildRoutingKey(instance, RoutingDimensions{PeerID: "peer-1", ThreadID: "thread-1"})
		if err != nil {
			t.Fatalf("BuildRoutingKey(first) error = %v", err)
		}
		second, err := BuildRoutingKey(instance, RoutingDimensions{PeerID: "peer-1", ThreadID: "thread-2"})
		if err != nil {
			t.Fatalf("BuildRoutingKey(second) error = %v", err)
		}
		firstHash, err := first.Hash()
		if err != nil {
			t.Fatalf("first.Hash() error = %v", err)
		}
		secondHash, err := second.Hash()
		if err != nil {
			t.Fatalf("second.Hash() error = %v", err)
		}
		if firstHash == secondHash {
			t.Fatalf("Hash() = %q for both threads, want distinct identities", firstHash)
		}
	})
}

func validContractBridgeInstance() BridgeInstance {
	return BridgeInstance{
		ID: "brg-1", Scope: ScopeWorkspace, WorkspaceID: "ws-1", Platform: "slack",
		ExtensionName: "slack-adapter", DisplayName: "Support", Source: BridgeInstanceSourcePackage,
		Enabled: true, Status: BridgeStatusDegraded, DMPolicy: BridgeDMPolicyPairing,
		RoutingPolicy:    RoutingPolicy{IncludePeer: true, IncludeThread: true, IncludeGroup: true},
		ProviderConfig:   json.RawMessage(`{"mode":"bot"}`),
		DeliveryDefaults: json.RawMessage(`{"mode":"reply"}`),
		Degradation:      &BridgeDegradation{Reason: BridgeDegradationReasonRateLimited, Message: "retry later"},
		CreatedAt:        time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 7, 13, 12, 1, 0, 0, time.UTC),
	}
}
