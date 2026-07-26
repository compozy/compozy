package bridges_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/testutil"
)

func TestBridgeInstanceResourceCodecRejectsInvalidPayloads(t *testing.T) {
	t.Parallel()

	codec, err := bridgepkg.NewBridgeInstanceResourceCodec(nil)
	if err != nil {
		t.Fatalf("NewBridgeInstanceResourceCodec() error = %v", err)
	}

	tests := []struct {
		name      string
		scope     resources.ResourceScope
		raw       []byte
		wantIs    error
		wantError string
	}{
		{
			name:      "invalid scope binding",
			scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"},
			wantIs:    resources.ErrInvalidScopeBinding,
			wantError: `bridge.scope "global" does not match resource scope "workspace"`,
			raw: []byte(`{
				"scope":"global",
				"platform":"telegram",
				"extension_name":"ext-telegram",
				"display_name":"Support",
				"enabled":true,
				"dm_policy":"open",
				"routing_policy":{"include_peer":true}
			}`),
		},
		{
			name:      "malformed provider config",
			scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			wantError: "bridge instance provider config must be a JSON object or null",
			raw: []byte(`{
				"scope":"global",
				"platform":"telegram",
				"extension_name":"ext-telegram",
				"display_name":"Support",
				"enabled":true,
				"dm_policy":"open",
				"routing_policy":{"include_peer":true},
				"provider_config":["not","object"]
			}`),
		},
		{
			name:      "invalid dm policy",
			scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			wantError: `unsupported dm policy "invite-everyone"`,
			raw: []byte(`{
				"scope":"global",
				"platform":"telegram",
				"extension_name":"ext-telegram",
				"display_name":"Support",
				"enabled":true,
				"dm_policy":"invite-everyone",
				"routing_policy":{"include_peer":true}
			}`),
		},
		{
			name:      "invalid delivery defaults field type",
			scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			wantError: `bridge instance delivery defaults field "thread_id" must be a string`,
			raw: []byte(`{
					"scope":"global",
					"platform":"telegram",
					"extension_name":"ext-telegram",
					"display_name":"Support",
					"enabled":true,
					"dm_policy":"open",
					"routing_policy":{"include_peer":true},
					"delivery_defaults":{"thread_id":{"nested":"bad"}}
				}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := codec.DecodeAndValidate(testutil.Context(t), tt.scope, tt.raw)
			if err == nil {
				t.Fatalf("DecodeAndValidate() error = nil, want validation failure")
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("DecodeAndValidate() error = %v, want errors.Is(..., %v)", err, tt.wantIs)
			}
			if tt.wantError != "" && !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("DecodeAndValidate() error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestBridgeInstanceResourceCodecAllowsProviderSpecificDeliveryDefaults(t *testing.T) {
	t.Parallel()

	codec, err := bridgepkg.NewBridgeInstanceResourceCodec(nil)
	if err != nil {
		t.Fatalf("NewBridgeInstanceResourceCodec() error = %v", err)
	}

	spec, err := codec.DecodeAndValidate(
		testutil.Context(t),
		resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		[]byte(`{
			"scope":"global",
			"platform":"telegram",
			"extension_name":"ext-telegram",
			"display_name":"Support",
			"enabled":true,
			"dm_policy":"open",
			"routing_policy":{"include_peer":true},
			"delivery_defaults":{"parse_mode":"markdown","thread_id":"thread-marketing"}
		}`),
	)
	if err != nil {
		t.Fatalf("DecodeAndValidate() error = %v", err)
	}

	if got, want := string(
		spec.DeliveryDefaults,
	), `{"parse_mode":"markdown","thread_id":"thread-marketing"}`; got != want {
		t.Fatalf("spec.DeliveryDefaults = %s, want %s", got, want)
	}
}

func TestBridgeProgressConfigValidationAndResolution(t *testing.T) {
	t.Parallel()

	t.Run("Should validate a typed progress block alongside provider defaults", func(t *testing.T) {
		t.Parallel()

		raw := []byte(`{
			"scope":"global",
			"platform":"slack",
			"extension_name":"ext-slack",
			"display_name":"Support",
			"enabled":true,
			"dm_policy":"open",
			"routing_policy":{"include_peer":true},
			"delivery_defaults":{
				"thread_id":"thread-support",
				"progress":{"tool_progress":"all","grouping":"separate","typing":true,"reactions":true}
			}
		}`)
		codec, err := bridgepkg.NewBridgeInstanceResourceCodec(nil)
		if err != nil {
			t.Fatalf("NewBridgeInstanceResourceCodec() error = %v", err)
		}
		spec, err := codec.DecodeAndValidate(
			testutil.Context(t),
			resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			raw,
		)
		if err != nil {
			t.Fatalf("DecodeAndValidate() error = %v", err)
		}
		instance := bridgepkg.BridgeInstance{Platform: "slack", DeliveryDefaults: spec.DeliveryDefaults}
		got := bridgepkg.ResolveProgressConfig(&instance, instance.Platform)
		if got.ToolProgress != bridgepkg.ProgressModeAll ||
			got.Grouping != bridgepkg.ProgressGroupingSeparate ||
			!got.Typing || !got.Reactions {
			t.Fatalf("ResolveProgressConfig(instance) = %#v, want explicit override", got)
		}
	})
}

func TestBridgeInstanceResourceCodecEnforcesProviderManifestMetadata(t *testing.T) {
	t.Parallel()

	lookup := func(_ context.Context, extensionName string) (bridgepkg.BridgeProvider, bool, error) {
		if strings.TrimSpace(extensionName) != "ext-telegram" {
			return bridgepkg.BridgeProvider{}, false, nil
		}
		return bridgepkg.BridgeProvider{
			Platform:      "telegram",
			ExtensionName: "ext-telegram",
			DisplayName:   "Telegram",
			SecretSlots: []bridgepkg.BridgeSecretSlot{
				{Name: "bot_token", Required: true},
				{Name: "signing_secret"},
			},
			ConfigSchema: &bridgepkg.BridgeProviderConfigSchema{
				Schema:  "telegram.bot",
				Version: "v1",
			},
		}, true, nil
	}
	codec, err := bridgepkg.NewBridgeInstanceResourceCodec(lookup)
	if err != nil {
		t.Fatalf("NewBridgeInstanceResourceCodec() error = %v", err)
	}

	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
	spec, err := codec.DecodeAndValidate(scopeContext(t), scope, []byte(`{
		"scope":"global",
		"platform":"telegram",
		"extension_name":"ext-telegram",
		"display_name":"Support",
		"enabled":true,
		"dm_policy":"pairing",
		"routing_policy":{"include_peer":true},
		"provider_config":{"tenant":"acme"},
		"delivery_defaults":{"peer_id":"peer-1","mode":"reply"}
	}`))
	if err != nil {
		t.Fatalf("DecodeAndValidate(valid) error = %v", err)
	}
	if got, want := len(spec.SecretSlots), 2; got != want {
		t.Fatalf("len(spec.SecretSlots) = %d, want %d", got, want)
	}
	if spec.ConfigSchema == nil || spec.ConfigSchema.Schema != "telegram.bot" ||
		spec.ConfigSchema.Version != "v1" {
		t.Fatalf("spec.ConfigSchema = %#v, want manifest schema", spec.ConfigSchema)
	}
	if got, want := string(spec.ProviderConfig), `{"tenant":"acme"}`; got != want {
		t.Fatalf("spec.ProviderConfig = %s, want %s", got, want)
	}

	for _, tc := range []struct {
		name      string
		raw       []byte
		wantError string
	}{
		{
			name: "platform mismatch",
			raw: []byte(`{
			"scope":"global",
			"platform":"slack",
			"extension_name":"ext-telegram",
			"display_name":"Support",
			"enabled":true,
			"dm_policy":"pairing",
			"routing_policy":{"include_peer":true}
		}`),
			wantError: `bridge provider "ext-telegram" platform "telegram" does not match resource platform "slack"`,
		},
		{
			name: "secret slot mismatch",
			raw: []byte(`{
			"scope":"global",
			"platform":"telegram",
			"extension_name":"ext-telegram",
			"display_name":"Support",
			"enabled":true,
			"dm_policy":"pairing",
			"routing_policy":{"include_peer":true},
			"secret_slots":[{"name":"wrong"}]
		}`),
			wantError: "secret_slots metadata does not match manifest",
		},
		{
			name: "config schema mismatch",
			raw: []byte(`{
			"scope":"global",
			"platform":"telegram",
			"extension_name":"ext-telegram",
			"display_name":"Support",
			"enabled":true,
			"dm_policy":"pairing",
			"routing_policy":{"include_peer":true},
			"config_schema":{"schema":"different","version":"v1"}
		}`),
			wantError: "config_schema metadata does not match manifest",
		},
		{
			name: "unknown provider",
			raw: []byte(`{
			"scope":"global",
			"platform":"telegram",
			"extension_name":"missing-provider",
			"display_name":"Support",
			"enabled":true,
			"dm_policy":"pairing",
			"routing_policy":{"include_peer":true}
		}`),
			wantError: `bridge provider "missing-provider" is not installed`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := codec.DecodeAndValidate(scopeContext(t), scope, tc.raw)
			if err == nil {
				t.Fatalf("DecodeAndValidate(%s) error = nil, want validation failure", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("DecodeAndValidate(%s) error = %v, want substring %q", tc.name, err, tc.wantError)
			}
		})
	}
}

func TestBridgeResourceBuildComputesDeltaWithoutApplyingSideEffects(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	store := &projectionStore{
		instances: []bridgepkg.BridgeInstance{{
			ID:            "brg-existing",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "ext-telegram",
			DisplayName:   "Existing",
			Source:        bridgepkg.BridgeInstanceSourceDynamic,
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			DMPolicy:      bridgepkg.BridgeDMPolicyOpen,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			CreatedAt:     now.Add(-time.Hour),
			UpdatedAt:     now.Add(-time.Hour),
		}},
	}

	records := []resources.Record[bridgepkg.BridgeInstanceSpec]{{
		ID:        "brg-existing",
		Version:   7,
		Scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		Spec:      resourceSpec("Updated", true),
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now,
	}}
	plan, err := bridgepkg.BuildResourceState(testutil.Context(t), store, records, func() time.Time { return now })
	if err != nil {
		t.Fatalf("BuildResourceState() error = %v", err)
	}
	if got, want := plan.Revision(), int64(7); got != want {
		t.Fatalf("plan.Revision() = %d, want %d", got, want)
	}
	if got, want := plan.OperationCount(), 1; got != want {
		t.Fatalf("plan.OperationCount() = %d, want %d", got, want)
	}
	if got := len(store.replacements); got != 0 {
		t.Fatalf("BuildResourceState applied %d replacements, want 0", got)
	}
	next := plan.NextInstances()
	if len(next) != 1 || next[0].DisplayName != "Updated" || next[0].Status != bridgepkg.BridgeStatusReady {
		t.Fatalf("plan.NextInstances() = %#v, want updated desired fields with preserved status", next)
	}
}

func TestBridgeResourceProjectionRemovesLegacyRowsWhenSnapshotIsEmpty(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	store := &projectionStore{
		instances: []bridgepkg.BridgeInstance{{
			ID:            "brg-legacy",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "ext-telegram",
			DisplayName:   "Legacy",
			Source:        bridgepkg.BridgeInstanceSourceDynamic,
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			DMPolicy:      bridgepkg.BridgeDMPolicyOpen,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			CreatedAt:     now,
			UpdatedAt:     now,
		}},
	}

	plan, err := bridgepkg.BuildResourceState(testutil.Context(t), store, nil, func() time.Time { return now })
	if err != nil {
		t.Fatalf("BuildResourceState(empty) error = %v", err)
	}
	if got, want := plan.OperationCount(), 1; got != want {
		t.Fatalf("plan.OperationCount() = %d, want %d", got, want)
	}
	if err := bridgepkg.ApplyResourceState(testutil.Context(t), store, plan); err != nil {
		t.Fatalf("ApplyResourceState(empty) error = %v", err)
	}
	if got := len(store.instances); got != 0 {
		t.Fatalf("len(store.instances) = %d, want legacy rows removed", got)
	}
}

func TestBridgeResourceApplyReturnsReplaceFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("replace failed")
	store := &projectionStore{}
	plan, err := bridgepkg.BuildResourceState(
		testutil.Context(t),
		store,
		[]resources.Record[bridgepkg.BridgeInstanceSpec]{{
			ID:        "brg-fail",
			Version:   1,
			Scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			Spec:      resourceSpec("Failing", true),
			CreatedAt: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
		}},
		time.Now,
	)
	if err != nil {
		t.Fatalf("BuildResourceState() error = %v", err)
	}
	store.replaceErr = wantErr
	err = bridgepkg.ApplyResourceState(testutil.Context(t), store, plan)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ApplyResourceState() error = %v, want %v", err, wantErr)
	}
}

func TestBridgeResourceApplyRejectsTypedNilPlanWithoutReplacingInstances(t *testing.T) {
	t.Parallel()

	newStore := func() *projectionStore {
		now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
		return &projectionStore{
			instances: []bridgepkg.BridgeInstance{{
				ID:            "brg-existing",
				Scope:         bridgepkg.ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "ext-telegram",
				DisplayName:   "Existing",
				Source:        bridgepkg.BridgeInstanceSourceDynamic,
				Enabled:       true,
				Status:        bridgepkg.BridgeStatusReady,
				DMPolicy:      bridgepkg.BridgeDMPolicyOpen,
				RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
				CreatedAt:     now,
				UpdatedAt:     now,
			}},
		}
	}

	t.Run("Should reject typed nil plan without replacing instances", func(t *testing.T) {
		t.Parallel()

		store := newStore()
		var plan *bridgepkg.ResourceProjectionPlan

		err := bridgepkg.ApplyResourceState(testutil.Context(t), store, plan)
		if err == nil {
			t.Fatal("ApplyResourceState(typed nil) error = nil, want validation failure")
		}
		if got, want := err.Error(), "bridges: bridge resource plan is required"; got != want {
			t.Fatalf("ApplyResourceState(typed nil) error = %q, want %q", got, want)
		}
		if got := len(store.replacements); got != 0 {
			t.Fatalf("len(store.replacements) = %d, want no replacement", got)
		}
		if got := len(store.instances); got != 1 {
			t.Fatalf("len(store.instances) = %d, want existing instance preserved", got)
		}
	})

	t.Run("Should reject untyped nil interface without replacing instances", func(t *testing.T) {
		t.Parallel()

		store := newStore()
		var plan resources.ProjectionPlan

		err := bridgepkg.ApplyResourceState(testutil.Context(t), store, plan)
		if err == nil {
			t.Fatal("ApplyResourceState(untyped nil) error = nil, want validation failure")
		}
		if got, want := err.Error(), "bridges: bridge resource plan has type <nil>"; got != want {
			t.Fatalf("ApplyResourceState(untyped nil) error = %q, want %q", got, want)
		}
		if got := len(store.replacements); got != 0 {
			t.Fatalf("len(store.replacements) = %d, want no replacement", got)
		}
		if got := len(store.instances); got != 1 {
			t.Fatalf("len(store.instances) = %d, want existing instance preserved", got)
		}
	})
}

func TestBridgeResourceProjectionPlanAccessorsAndRollback(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	store := &projectionStore{
		instances: []bridgepkg.BridgeInstance{{
			ID:            "brg-accessor",
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "ext-old",
			DisplayName:   "Before",
			Source:        bridgepkg.BridgeInstanceSourceDynamic,
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusDegraded,
			DMPolicy:      bridgepkg.BridgeDMPolicyOpen,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			ProviderConfig: []byte(`{
				"tenant":"acme"
			}`),
			Degradation: &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
				Message: "waiting for adapter refresh",
			},
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Minute),
		}},
	}

	spec := resourceSpec("After", true)
	spec.ExtensionName = "ext-new"
	plan, err := bridgepkg.BuildResourceState(
		testutil.Context(t),
		store,
		[]resources.Record[bridgepkg.BridgeInstanceSpec]{{
			ID:        "brg-accessor",
			Version:   17,
			Scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			Spec:      spec,
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now,
		}},
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("BuildResourceState() error = %v", err)
	}

	if got, want := plan.Kind(), bridgepkg.BridgeInstanceResourceKind; got != want {
		t.Fatalf("plan.Kind() = %q, want %q", got, want)
	}
	if got, want := plan.ChangedExtensions(), []string{
		"ext-new",
		"ext-old",
	}; strings.Join(
		got,
		",",
	) != strings.Join(
		want,
		",",
	) {
		t.Fatalf("plan.ChangedExtensions() = %#v, want %#v", got, want)
	}
	previous := plan.PreviousInstances()
	if len(previous) != 1 || previous[0].Degradation == nil {
		t.Fatalf("plan.PreviousInstances() = %#v, want preserved degradation", previous)
	}

	rollback := plan.RollbackPlan()
	if rollback == nil {
		t.Fatalf("plan.RollbackPlan() = nil, want rollback plan")
	}
	next := rollback.NextInstances()
	if len(next) != 1 || next[0].DisplayName != "Before" || next[0].ExtensionName != "ext-old" {
		t.Fatalf("rollback.NextInstances() = %#v, want previous bridge state", next)
	}
}

func TestBridgeResourceProjectionIgnoresSemanticallyEquivalentJSON(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	store := &projectionStore{
		instances: []bridgepkg.BridgeInstance{{
			ID:               "brg-json",
			Scope:            bridgepkg.ScopeGlobal,
			Platform:         "telegram",
			ExtensionName:    "ext-telegram",
			DisplayName:      "JSON Bridge",
			Source:           bridgepkg.BridgeInstanceSourceDynamic,
			Enabled:          true,
			Status:           bridgepkg.BridgeStatusReady,
			DMPolicy:         bridgepkg.BridgeDMPolicyOpen,
			RoutingPolicy:    bridgepkg.RoutingPolicy{IncludePeer: true},
			ProviderConfig:   []byte(`{"tenant":"acme","features":{"beta":true}}`),
			DeliveryDefaults: []byte(`{"peer_id":"peer-1","mode":"reply"}`),
			CreatedAt:        now.Add(-time.Hour),
			UpdatedAt:        now.Add(-time.Minute),
		}},
	}

	spec := resourceSpec("JSON Bridge", true)
	spec.ProviderConfig = []byte("{\n  \"features\": {\"beta\": true},\n  \"tenant\": \"acme\"\n}")
	spec.DeliveryDefaults = []byte(`{"mode":"reply","peer_id":"peer-1"}`)
	plan, err := bridgepkg.BuildResourceState(
		testutil.Context(t),
		store,
		[]resources.Record[bridgepkg.BridgeInstanceSpec]{{
			ID:        "brg-json",
			Version:   9,
			Scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			Spec:      spec,
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now,
		}},
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("BuildResourceState() error = %v", err)
	}
	if got, want := plan.OperationCount(), 0; got != want {
		t.Fatalf("plan.OperationCount() = %d, want %d", got, want)
	}
	if got := plan.ChangedExtensions(); len(got) != 0 {
		t.Fatalf("plan.ChangedExtensions() = %#v, want no changed extensions", got)
	}
}

func TestBridgeResourceProjectionDetectsLargeJSONNumberChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		previous []byte
		next     []byte
	}{
		{
			name:     "Should detect top-level large integer change",
			previous: []byte(`{"cursor":9007199254740992}`),
			next:     []byte(`{"cursor":9007199254740993}`),
		},
		{
			name:     "Should detect nested large integer change",
			previous: []byte(`{"meta":{"cursor":9007199254740992}}`),
			next:     []byte(`{"meta":{"cursor":9007199254740993}}`),
		},
		{
			name:     "Should distinguish large number from string",
			previous: []byte(`{"cursor":9007199254740992}`),
			next:     []byte(`{"cursor":"9007199254740992"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
			store := &projectionStore{
				instances: []bridgepkg.BridgeInstance{{
					ID:             "brg-json-number",
					Scope:          bridgepkg.ScopeGlobal,
					Platform:       "telegram",
					ExtensionName:  "ext-telegram",
					DisplayName:    "JSON Number Bridge",
					Source:         bridgepkg.BridgeInstanceSourceDynamic,
					Enabled:        true,
					Status:         bridgepkg.BridgeStatusReady,
					DMPolicy:       bridgepkg.BridgeDMPolicyOpen,
					RoutingPolicy:  bridgepkg.RoutingPolicy{IncludePeer: true},
					ProviderConfig: tt.previous,
					CreatedAt:      now.Add(-time.Hour),
					UpdatedAt:      now.Add(-time.Minute),
				}},
			}
			spec := resourceSpec("JSON Number Bridge", true)
			spec.ProviderConfig = tt.next

			plan, err := bridgepkg.BuildResourceState(
				testutil.Context(t),
				store,
				[]resources.Record[bridgepkg.BridgeInstanceSpec]{{
					ID:        "brg-json-number",
					Version:   10,
					Scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
					Spec:      spec,
					CreatedAt: now.Add(-time.Hour),
					UpdatedAt: now,
				}},
				func() time.Time { return now },
			)
			if err != nil {
				t.Fatalf("BuildResourceState() error = %v", err)
			}
			if got, want := plan.OperationCount(), 1; got != want {
				t.Fatalf("plan.OperationCount() = %d, want %d", got, want)
			}
			if got, want := strings.Join(plan.ChangedExtensions(), ","), "ext-telegram"; got != want {
				t.Fatalf("plan.ChangedExtensions() = %q, want %q", got, want)
			}
		})
	}
}

func TestBridgeInstanceSpecFromCreateRequestBindsWorkspaceScope(t *testing.T) {
	t.Parallel()

	request := bridgepkg.CreateInstanceRequest{
		Scope:            bridgepkg.ScopeWorkspace,
		WorkspaceID:      "ws-alpha",
		Platform:         "telegram",
		ExtensionName:    "ext-telegram",
		DisplayName:      "Workspace Telegram",
		Source:           bridgepkg.BridgeInstanceSourceDynamic,
		Enabled:          true,
		Status:           bridgepkg.BridgeStatusReady,
		DMPolicy:         bridgepkg.BridgeDMPolicyOpen,
		RoutingPolicy:    bridgepkg.RoutingPolicy{IncludePeer: true},
		ProviderConfig:   []byte(`{"tenant":"acme"}`),
		DeliveryDefaults: []byte(`{"peer_id":"peer-1","mode":"reply"}`),
	}

	id, spec, err := bridgepkg.BridgeInstanceSpecFromCreateRequest(request, func() time.Time {
		return time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("BridgeInstanceSpecFromCreateRequest() error = %v", err)
	}
	if strings.TrimSpace(id) == "" {
		t.Fatalf("BridgeInstanceSpecFromCreateRequest() id is empty")
	}
	scope := bridgepkg.ResourceScopeForBridge(spec.Scope, spec.WorkspaceID)
	if got, want := scope.Kind, resources.ResourceScopeKindWorkspace; got != want {
		t.Fatalf("scope.Kind = %q, want %q", got, want)
	}
	if got, want := scope.ID, "ws-alpha"; got != want {
		t.Fatalf("scope.ID = %q, want %q", got, want)
	}
	if got, want := string(spec.ProviderConfig), `{"tenant":"acme"}`; got != want {
		t.Fatalf("spec.ProviderConfig = %s, want %s", got, want)
	}
}

func scopeContext(t *testing.T) context.Context {
	t.Helper()
	return testutil.Context(t)
}

func resourceSpec(displayName string, enabled bool) bridgepkg.BridgeInstanceSpec {
	return bridgepkg.BridgeInstanceSpec{
		Scope:            bridgepkg.ScopeGlobal,
		Platform:         "telegram",
		ExtensionName:    "ext-telegram",
		DisplayName:      displayName,
		Source:           bridgepkg.BridgeInstanceSourceDynamic,
		Enabled:          enabled,
		DMPolicy:         bridgepkg.BridgeDMPolicyOpen,
		RoutingPolicy:    bridgepkg.RoutingPolicy{IncludePeer: true},
		ProviderConfig:   []byte(`{"tenant":"acme"}`),
		DeliveryDefaults: []byte(`{"peer_id":"peer-1","mode":"reply"}`),
	}
}

type projectionStore struct {
	instances    []bridgepkg.BridgeInstance
	replacements [][]bridgepkg.BridgeInstance
	replaceErr   error
}

func (s *projectionStore) ListBridgeInstances(context.Context) ([]bridgepkg.BridgeInstance, error) {
	instances := make([]bridgepkg.BridgeInstance, 0, len(s.instances))
	for _, instance := range s.instances {
		instances = append(instances, cloneBridgeInstanceForTest(instance))
	}
	return instances, nil
}

func (s *projectionStore) ReplaceBridgeInstances(_ context.Context, instances []bridgepkg.BridgeInstance) error {
	if s.replaceErr != nil {
		return s.replaceErr
	}
	next := make([]bridgepkg.BridgeInstance, 0, len(instances))
	for _, instance := range instances {
		next = append(next, cloneBridgeInstanceForTest(instance))
	}
	s.replacements = append(s.replacements, next)
	s.instances = next
	return nil
}
