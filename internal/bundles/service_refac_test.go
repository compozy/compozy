package bundles

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/resources"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestServiceRefacs(t *testing.T) {
	t.Parallel()

	t.Run("Should reject canceled context before store access", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		store.listBundleResourcesHook = func() ([]resources.Record[BundleResourceSpec], error) {
			t.Fatal("ListBundleResources() should not be called with a canceled context")
			return nil, nil
		}
		service := newMarketingService(store, WithLogger(discardBundleTestLogger()))

		ctx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		_, err := service.Catalog(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Catalog(canceled) error = %v, want context.Canceled", err)
		}
	})

	t.Run("Should keep public previews defensively cloned", func(t *testing.T) {
		t.Parallel()

		service := newBundleServiceWithMutableProfile(t)

		first, err := service.PreviewActivation(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("PreviewActivation() error = %v", err)
		}
		first.Bundle.Profiles[0].Jobs[0].Task.Owner.Ref = "mutated"
		first.Bundle.Profiles[0].Triggers[0].Filter["kind"] = "mutated"
		first.Bundle.Profiles[0].Bridges[0].DeliveryDefaults[0] = '['
		first.Bundle.Profiles[0].Bridges[0].SecretSlots[0].Name = "mutated"
		first.Profile.Triggers[0].Filter["extra"] = "mutated"

		second, err := service.PreviewActivation(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("PreviewActivation() second error = %v", err)
		}
		if got, want := second.Bundle.Profiles[0].Jobs[0].Task.Owner.Ref, "triage"; got != want {
			t.Fatalf("second bundle task owner ref = %q, want %q", got, want)
		}
		if got, want := second.Bundle.Profiles[0].Triggers[0].Filter["kind"], "session.created"; got != want {
			t.Fatalf("second bundle trigger filter kind = %q, want %q", got, want)
		}
		if _, ok := second.Profile.Triggers[0].Filter["extra"]; ok {
			t.Fatal("second profile trigger filter kept mutation from previous preview")
		}
		gotDefaults := string(second.Bundle.Profiles[0].Bridges[0].DeliveryDefaults)
		if wantDefaults := `{"priority":"normal"}`; gotDefaults != wantDefaults {
			t.Fatalf("second bridge delivery defaults = %q, want %q", gotDefaults, wantDefaults)
		}
		if got, want := second.Bundle.Profiles[0].Bridges[0].SecretSlots[0].Name, "bot_token"; got != want {
			t.Fatalf("second bridge secret slot name = %q, want %q", got, want)
		}
	})

	t.Run("Should build plans isolated from bundle record input mutation", func(t *testing.T) {
		t.Parallel()

		service := newBundleServiceWithMutableProfile(t)
		activation := Activation{
			ID:            ActivationResourceID("marketing-team", "marketing", "default", ScopeGlobal, ""),
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		}
		store := service.store.(*memoryStore)
		bundleRecords := append([]resources.Record[BundleResourceSpec](nil), store.bundles...)
		plan, err := service.Build(
			testutil.Context(t),
			[]resources.Record[ActivationResourceSpec]{{
				Kind:  BundleActivationResourceKind,
				ID:    activation.ID,
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				Spec:  activationResourceSpecFromActivation(activation),
			}},
			bundleRecords,
		)
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		typed, ok := plan.(*BundleActivationResourcePlan)
		if !ok {
			t.Fatalf("Build() plan type = %T, want *BundleActivationResourcePlan", plan)
		}

		bundleRecords[0].Spec.Bundle.Profiles[0].Jobs[0].Task.Owner.Ref = "mutated"
		bundleRecords[0].Spec.Bundle.Profiles[0].Triggers[0].Filter["kind"] = "mutated"
		bundleRecords[0].Spec.Bundle.Profiles[0].Bridges[0].DeliveryDefaults[0] = '['

		if got, want := typed.desiredJobs[0].Task.Owner.Ref, "triage"; got != want {
			t.Fatalf("plan job task owner ref = %q, want %q", got, want)
		}
		if got, want := typed.desiredTriggers[0].Filter["kind"], "session.created"; got != want {
			t.Fatalf("plan trigger filter kind = %q, want %q", got, want)
		}
		if got, want := string(typed.desiredBridges[0].DeliveryDefaults), `{"priority":"normal"}`; got != want {
			t.Fatalf("plan bridge delivery defaults = %q, want %q", got, want)
		}
	})

	t.Run("Should resolve activation ids through the canonical helper", func(t *testing.T) {
		t.Parallel()

		service := newMarketingService(
			newMemoryStore(),
			WithLogger(discardBundleTestLogger()),
			WithWorkspaceResolver(memoryWorkspaceResolver{
				resolveFn: func(_ context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{ID: idOrPath, Name: idOrPath},
					}, nil
				},
			}),
		)

		preview, err := service.PreviewActivation(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeWorkspace,
			Workspace:     "workspace-1",
		})
		if err != nil {
			t.Fatalf("PreviewActivation() error = %v", err)
		}
		want := ActivationResourceID("marketing-team", "marketing", "default", ScopeWorkspace, "workspace-1")
		if preview.Activation.ID != want {
			t.Fatalf("preview activation id = %q, want %q", preview.Activation.ID, want)
		}
	})

	t.Run("Should preview path-like workspace refs without registration", func(t *testing.T) {
		t.Parallel()

		workspacePath := t.TempDir()
		service := newMarketingService(
			newMemoryStore(),
			WithLogger(discardBundleTestLogger()),
			WithWorkspaceResolver(memoryWorkspaceResolver{
				resolveFn: func(_ context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
					if idOrPath != workspacePath && idOrPath != "ws-preview" {
						t.Fatalf("Resolve() idOrPath = %q, want %q or ws-preview", idOrPath, workspacePath)
					}
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{
							ID:      "ws-preview",
							Name:    "Preview Workspace",
							RootDir: workspacePath,
						},
					}, nil
				},
				resolveOrRegisterFn: func(_ context.Context, path string) (workspacepkg.ResolvedWorkspace, error) {
					t.Fatalf("ResolveOrRegister() path = %q, want preview to stay read-only", path)
					return workspacepkg.ResolvedWorkspace{}, nil
				},
			}),
		)

		preview, err := service.PreviewActivation(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeWorkspace,
			Workspace:     workspacePath,
		})
		if err != nil {
			t.Fatalf("PreviewActivation() error = %v", err)
		}
		if got, want := preview.Activation.WorkspaceID, "ws-preview"; got != want {
			t.Fatalf("preview workspace id = %q, want %q", got, want)
		}
	})

	t.Run("Should keep network settings unchanged when reconcile sync fails", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		service := newMarketingService(store, WithLogger(discardBundleTestLogger()))
		activationID := ActivationResourceID("marketing-team", "marketing", "default", ScopeGlobal, "")
		store.activations[activationID] = Activation{
			ID:            activationID,
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		}
		syncErr := errors.New("sync failed")
		store.applyErr = syncErr

		err := service.Reconcile(testutil.Context(t))
		if !errors.Is(err, syncErr) {
			t.Fatalf("Reconcile() error = %v, want sync failure", err)
		}
		settings, err := service.NetworkSettings(testutil.Context(t))
		if err != nil {
			t.Fatalf("NetworkSettings() error = %v", err)
		}
		if got := len(settings.DeclaredChannels); got != 0 {
			t.Fatalf("len(DeclaredChannels) after failed reconcile = %d, want 0", got)
		}
	})

	t.Run("Should materialize canonical scopes from validated activations", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		service := newMarketingService(
			store,
			WithLogger(discardBundleTestLogger()),
			WithWorkspaceResolver(memoryWorkspaceResolver{
				resolveFn: func(_ context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
					return workspacepkg.ResolvedWorkspace{
						Workspace: workspacepkg.Workspace{ID: strings.TrimSpace(idOrPath), Name: idOrPath},
					}, nil
				},
			}),
		)

		resolved, err := service.resolveActivation(testutil.Context(t), Activation{
			ID:            ActivationResourceID("marketing-team", "marketing", "default", ScopeWorkspace, "ws-1"),
			ExtensionName: " marketing-team ",
			BundleName:    " marketing ",
			ProfileName:   " default ",
			Scope:         Scope(" WORKSPACE "),
			WorkspaceID:   " ws-1 ",
		})
		if err != nil {
			t.Fatalf("resolveActivation() error = %v", err)
		}
		if got, want := resolved.activation.Scope, ScopeWorkspace; got != want {
			t.Fatalf("activation scope = %q, want %q", got, want)
		}
		if got, want := resolved.activation.WorkspaceID, "ws-1"; got != want {
			t.Fatalf("activation workspace = %q, want %q", got, want)
		}
		if got, want := resolved.agents[0].Scope.Kind, resources.ResourceScopeKindWorkspace; got != want {
			t.Fatalf("agent scope kind = %q, want %q", got, want)
		}
		if got, want := resolved.jobs[0].Scope, automationpkg.AutomationScopeWorkspace; got != want {
			t.Fatalf("job scope = %q, want %q", got, want)
		}
		if got, want := resolved.bridges[0].Scope, bridgepkg.ScopeWorkspace; got != want {
			t.Fatalf("bridge scope = %q, want %q", got, want)
		}
	})
}

func TestStableIDGoldenValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
		parts  []string
		want   string
	}{
		{
			name:   "Should preserve bundle id hash input trimming",
			prefix: "bun",
			parts:  []string{" marketing-team ", " marketing "},
			want:   "bun_71abf3d73c5a934f",
		},
		{
			name:   "Should preserve workspace activation id hash input",
			prefix: "act",
			parts:  []string{"marketing-team", "marketing", "default", "workspace", "ws-001"},
			want:   "act_05c52a0fc3434e04",
		},
		{
			name:   "Should preserve job id hash input",
			prefix: "job",
			parts:  []string{"act_abc", "daily-sync"},
			want:   "job_44e1af1b7cb594ac",
		},
		{
			name:   "Should preserve agent id hash input",
			prefix: "agt",
			parts:  []string{"act_abc", "marketer"},
			want:   "agt_2d25fbd1cf05ff42",
		},
		{
			name:   "Should preserve bridge id hash input",
			prefix: "bri",
			parts:  []string{"act_abc", "telegram-main"},
			want:   "bri_879455ae8928252c",
		},
		{
			name:   "Should preserve empty part separators",
			prefix: "act",
			parts:  []string{"", " spaced ", "", "global", ""},
			want:   "act_7958a67dc6e347a1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := stableID(tt.prefix, tt.parts...); got != tt.want {
				t.Fatalf("stableID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func newBundleServiceWithMutableProfile(t *testing.T) *Service {
	t.Helper()

	ext := newMarketingExtension()
	profile := &ext.Bundles[0].Profiles[0]
	profile.Jobs[0].Task = &automationpkg.JobTaskConfig{
		Title: "Campaign triage",
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindPool,
			Ref:  "triage",
		},
	}
	profile.Triggers[0].Filter = map[string]string{"kind": "session.created"}
	profile.Bridges[0].DeliveryDefaults = json.RawMessage(`{"priority":"normal"}`)
	profile.Bridges[0].SecretSlots = []extensionpkg.BundleBridgeSecretSlot{{
		Name: "bot_token",
		Kind: "token",
	}}

	return newServiceForExtensions(
		newMemoryStore(),
		[]*extensionpkg.Extension{ext},
		WithLogger(discardBundleTestLogger()),
	)
}
