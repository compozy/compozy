package subprocess

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	bridges "github.com/compozy/agh/internal/bridges/contract"
)

func TestInitializeBridgeRuntimeValidateRejectsInvalidProviderScopedPayload(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	validManaged := InitializeBridgeManagedInstance{
		Instance: bridges.BridgeInstance{
			ID:            "brg-1",
			Scope:         bridges.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-reference",
			DisplayName:   "Telegram",
			Enabled:       true,
			Status:        bridges.BridgeStatusReady,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		BoundSecrets: []InitializeBridgeBoundSecret{
			{BindingName: "bot_token", Kind: "token", Value: "secret"},
		},
	}

	tests := []struct {
		name    string
		runtime InitializeBridgeRuntime
		want    string
	}{
		{
			name: "missing provider identity",
			runtime: InitializeBridgeRuntime{
				RuntimeVersion:   InitializeBridgeRuntimeVersion2,
				Purpose:          BridgeRuntimePurposeService,
				Platform:         "telegram",
				ManagedInstances: []InitializeBridgeManagedInstance{validManaged},
			},
			want: "provider is required",
		},
		{
			name: "invalid managed instance snapshot",
			runtime: InitializeBridgeRuntime{
				RuntimeVersion: InitializeBridgeRuntimeVersion2,
				Purpose:        BridgeRuntimePurposeService,
				Provider:       "telegram-reference",
				Platform:       "telegram",
				ManagedInstances: []InitializeBridgeManagedInstance{{
					Instance: bridges.BridgeInstance{
						ID:            "brg-invalid",
						Scope:         bridges.ScopeGlobal,
						Platform:      "telegram",
						DisplayName:   "Telegram",
						Enabled:       true,
						Status:        bridges.BridgeStatusReady,
						RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
						CreatedAt:     now,
						UpdatedAt:     now,
					},
				}},
			},
			want: "bridge instance extension name",
		},
		{
			name: "platform mismatch",
			runtime: InitializeBridgeRuntime{
				RuntimeVersion:   InitializeBridgeRuntimeVersion2,
				Purpose:          BridgeRuntimePurposeService,
				Provider:         "telegram-reference",
				Platform:         "slack",
				ManagedInstances: []InitializeBridgeManagedInstance{validManaged},
			},
			want: "does not match runtime platform",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.runtime.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestCloneInitializeBridgeRuntimeDoesNotAliasManagedInstanceState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 5, 0, 0, time.UTC)
	src := &InitializeBridgeRuntime{
		RuntimeVersion: InitializeBridgeRuntimeVersion2,
		Purpose:        BridgeRuntimePurposeService,
		Provider:       "telegram-reference",
		Platform:       "telegram",
		ManagedInstances: []InitializeBridgeManagedInstance{{
			Instance: bridges.BridgeInstance{
				ID:               "brg-1",
				Scope:            bridges.ScopeWorkspace,
				WorkspaceID:      "ws-1",
				Platform:         "telegram",
				ExtensionName:    "telegram-reference",
				DisplayName:      "Telegram",
				Enabled:          true,
				Status:           bridges.BridgeStatusDegraded,
				RoutingPolicy:    bridges.RoutingPolicy{IncludePeer: true},
				ProviderConfig:   json.RawMessage(`{"mode":"bot"}`),
				DeliveryDefaults: json.RawMessage(`{"peer_id":"peer-1"}`),
				Degradation: &bridges.BridgeDegradation{
					Reason:  bridges.BridgeDegradationReasonAuthFailed,
					Message: "token expired",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
			BoundSecrets: []InitializeBridgeBoundSecret{
				{BindingName: "bot_token", Kind: "token", Value: "secret-1"},
			},
		}},
	}

	cloned := CloneInitializeBridgeRuntime(src)
	if cloned == nil {
		t.Fatal("CloneInitializeBridgeRuntime() = nil, want non-nil")
		return
	}

	src.ManagedInstances[0].Instance.ID = "mutated"
	src.ManagedInstances[0].Instance.ProviderConfig[0] = '['
	src.ManagedInstances[0].Instance.DeliveryDefaults[0] = '['
	src.ManagedInstances[0].Instance.Degradation.Message = "mutated"
	src.ManagedInstances[0].BoundSecrets[0].Value = "mutated"
	src.ManagedInstances = append(src.ManagedInstances, InitializeBridgeManagedInstance{
		Instance: bridges.BridgeInstance{
			ID:            "brg-2",
			Scope:         bridges.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-reference",
			DisplayName:   "Telegram 2",
			Enabled:       true,
			Status:        bridges.BridgeStatusReady,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	})

	if got, want := cloned.ManagedInstances[0].Instance.ID, "brg-1"; got != want {
		t.Fatalf("cloned managed instance id = %q, want %q", got, want)
	}
	if got, want := string(cloned.ManagedInstances[0].Instance.ProviderConfig), `{"mode":"bot"}`; got != want {
		t.Fatalf("cloned provider config = %q, want %q", got, want)
	}
	if got, want := string(cloned.ManagedInstances[0].Instance.DeliveryDefaults), `{"peer_id":"peer-1"}`; got != want {
		t.Fatalf("cloned delivery defaults = %q, want %q", got, want)
	}
	if got, want := cloned.ManagedInstances[0].Instance.Degradation.Message, "token expired"; got != want {
		t.Fatalf("cloned degradation message = %q, want %q", got, want)
	}
	if got, want := cloned.ManagedInstances[0].BoundSecrets[0].Value, "secret-1"; got != want {
		t.Fatalf("cloned bound secret value = %q, want %q", got, want)
	}
	if got, want := len(cloned.ManagedInstances), 1; got != want {
		t.Fatalf("len(cloned.ManagedInstances) = %d, want %d", got, want)
	}
}

func TestInitializeBridgeRuntimeManagedInstanceHelpers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 10, 0, 0, time.UTC)
	singleRuntime := InitializeBridgeRuntime{
		RuntimeVersion: InitializeBridgeRuntimeVersion2,
		Purpose:        BridgeRuntimePurposeService,
		Provider:       "telegram-reference",
		Platform:       "telegram",
		ManagedInstances: []InitializeBridgeManagedInstance{{
			Instance: bridges.BridgeInstance{
				ID:            " brg-1 ",
				Scope:         bridges.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "telegram-reference",
				DisplayName:   "Telegram",
				Enabled:       true,
				Status:        bridges.BridgeStatusReady,
				RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		}},
	}

	managed, err := singleRuntime.SingleManagedInstance()
	if err != nil {
		t.Fatalf("SingleManagedInstance() error = %v", err)
	}
	if got, want := managed.Instance.ID, " brg-1 "; got != want {
		t.Fatalf("SingleManagedInstance().Instance.ID = %q, want %q", got, want)
	}

	managed.Instance.ID = "mutated"
	reloaded, ok := singleRuntime.ManagedInstance("brg-1")
	if !ok {
		t.Fatal("ManagedInstance(brg-1) = missing, want managed instance")
	}
	if got, want := reloaded.Instance.ID, " brg-1 "; got != want {
		t.Fatalf("ManagedInstance(brg-1).Instance.ID = %q, want %q", got, want)
	}
	if _, ok := singleRuntime.ManagedInstance(" "); ok {
		t.Fatal("ManagedInstance(blank) = found, want false")
	}
	if _, ok := singleRuntime.ManagedInstance("missing"); ok {
		t.Fatal("ManagedInstance(missing) = found, want false")
	}
	if got, want := singleRuntime.ManagedBridgeInstanceIDs(), []string{
		"brg-1",
	}; len(got) != len(want) ||
		got[0] != want[0] {
		t.Fatalf("ManagedBridgeInstanceIDs() = %#v, want %#v", got, want)
	}

	if _, err := (InitializeBridgeRuntime{}).SingleManagedInstance(); err == nil ||
		!strings.Contains(err.Error(), "is required") {
		t.Fatalf("SingleManagedInstance() empty error = %v, want required error", err)
	}

	multiRuntime := singleRuntime
	multiRuntime.ManagedInstances = append(multiRuntime.ManagedInstances, InitializeBridgeManagedInstance{
		Instance: bridges.BridgeInstance{
			ID:            " brg-2 ",
			Scope:         bridges.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-reference",
			DisplayName:   "Telegram 2",
			Enabled:       true,
			Status:        bridges.BridgeStatusReady,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	})
	if _, err := multiRuntime.SingleManagedInstance(); err == nil ||
		!strings.Contains(err.Error(), "explicit managed instance selection") {
		t.Fatalf("SingleManagedInstance() multi error = %v, want explicit selection error", err)
	}
	if got, want := multiRuntime.ManagedBridgeInstanceIDs(), []string{
		"brg-1",
		"brg-2",
	}; len(got) != len(want) || got[0] != want[0] ||
		got[1] != want[1] {
		t.Fatalf("ManagedBridgeInstanceIDs() multi = %#v, want %#v", got, want)
	}
}

func TestInitializeBridgeManagedInstanceValidateRejectsDuplicateSecretBindings(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 15, 0, 0, time.UTC)
	managed := InitializeBridgeManagedInstance{
		Instance: bridges.BridgeInstance{
			ID:            "brg-dup",
			Scope:         bridges.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-reference",
			DisplayName:   "Telegram",
			Enabled:       true,
			Status:        bridges.BridgeStatusReady,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		BoundSecrets: []InitializeBridgeBoundSecret{
			{BindingName: "bot_token", Kind: "token", Value: "secret-1"},
			{BindingName: " bot_token ", Kind: "token", Value: "secret-2"},
		},
	}

	err := managed.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("managed.Validate() error = %v, want duplicated secret error", err)
	}
}

func TestInitializeBridgeRuntimeValidateRejectsDuplicateManagedInstances(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 20, 0, 0, time.UTC)
	managed := InitializeBridgeManagedInstance{
		Instance: bridges.BridgeInstance{
			ID:            "brg-dup",
			Scope:         bridges.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-reference",
			DisplayName:   "Telegram",
			Enabled:       true,
			Status:        bridges.BridgeStatusReady,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}

	runtime := InitializeBridgeRuntime{
		RuntimeVersion:   InitializeBridgeRuntimeVersion2,
		Purpose:          BridgeRuntimePurposeService,
		Provider:         "telegram-reference",
		Platform:         "telegram",
		ManagedInstances: []InitializeBridgeManagedInstance{managed, managed},
	}

	err := runtime.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("runtime.Validate() error = %v, want duplicated managed instance error", err)
	}
}

func TestInitializeBridgeBoundSecretValidateRejectsMissingFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		secret InitializeBridgeBoundSecret
		want   string
	}{
		{
			name:   "missing binding name",
			secret: InitializeBridgeBoundSecret{Kind: "token", Value: "secret"},
			want:   "binding_name",
		},
		{
			name:   "missing kind",
			secret: InitializeBridgeBoundSecret{BindingName: "bot_token", Value: "secret"},
			want:   "kind",
		},
		{
			name:   "missing value",
			secret: InitializeBridgeBoundSecret{BindingName: "bot_token", Kind: "token"},
			want:   "value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.secret.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("secret.Validate() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}
