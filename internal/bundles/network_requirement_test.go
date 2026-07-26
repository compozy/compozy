package bundles

import (
	"context"
	"errors"
	"strings"
	"testing"

	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestActivationResourceSpecNetworkRequirementFields(t *testing.T) {
	t.Parallel()

	t.Run("Should round-trip confirmation fields on ActivationResourceSpec", func(t *testing.T) {
		t.Parallel()

		activation := Activation{
			ID:                       "act_live",
			Version:                  7,
			ExtensionName:            "ext-live",
			BundleName:               "bundle",
			ProfileName:              "default",
			Scope:                    ScopeGlobal,
			NetworkRequirementDigest: "digest-v1",
			ConfirmedBy:              "operator",
			ConfirmedAt:              "2026-07-14T12:00:00Z",
		}
		spec := activationResourceSpecFromActivation(activation)
		if spec.NetworkRequirementDigest != "digest-v1" {
			t.Fatalf("digest = %q, want digest-v1", spec.NetworkRequirementDigest)
		}
		if spec.ConfirmedBy != "operator" || spec.ConfirmedAt != "2026-07-14T12:00:00Z" {
			t.Fatalf("confirmation = (%q, %q)", spec.ConfirmedBy, spec.ConfirmedAt)
		}
		restored := activationFromResourceRecord(resources.Record[ActivationResourceSpec]{
			ID:      activation.ID,
			Kind:    BundleActivationResourceKind,
			Version: activation.Version,
			Spec:    spec,
		})
		if restored.Version != activation.Version ||
			restored.NetworkRequirementDigest != "digest-v1" ||
			restored.ConfirmedBy != "operator" ||
			restored.ConfirmedAt != "2026-07-14T12:00:00Z" {
			t.Fatalf("restored = %#v", restored)
		}
	})

	t.Run("Should clear confirmation preview when digest is empty", func(t *testing.T) {
		t.Parallel()

		got := previewNetworkRequirement(
			Activation{
				NetworkRequirementDigest: "stale",
				ConfirmedBy:              "operator",
				ConfirmedAt:              "2026-07-14T12:00:00Z",
			},
			"",
		)
		if got.NetworkRequirementDigest != "" || got.ConfirmedBy != "" || got.ConfirmedAt != "" {
			t.Fatalf("preview = %#v, want cleared confirmation", got)
		}
	})

	t.Run("Should invalidate changed digests and reject a stale confirmation", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		ext := newMarketingExtension()
		ext.Manifest.NetworkParticipation = &extensionpkg.NetworkParticipationRequirement{
			Required:      true,
			Mode:          string(participation.ModeLive),
			ChannelScopes: []string{"builders"},
		}
		service := newServiceForExtensions(store, []*extensionpkg.Extension{ext})
		activated, err := service.Activate(testutil.Context(t), ActivateRequest{
			ExtensionName:             ext.Info.Name,
			BundleName:                "marketing",
			ProfileName:               "default",
			Scope:                     ScopeGlobal,
			ConfirmNetworkRequirement: true,
		})
		if err != nil {
			t.Fatalf("Activate() error = %v", err)
		}
		staleVersion := activated.Activation.Version
		oldDigest := activated.Activation.NetworkRequirementDigest

		ext.Manifest.NetworkParticipation.ChannelScopes = []string{"operators"}
		if err := service.Reconcile(testutil.Context(t)); !errors.Is(err, ErrNetworkRequirementConfirmationRequired) {
			t.Fatalf(
				"Reconcile(changed digest) error = %v, want ErrNetworkRequirementConfirmationRequired",
				err,
			)
		}
		changed, err := service.GetActivation(testutil.Context(t), activated.Activation.ID)
		if err != nil {
			t.Fatalf("GetActivation(changed digest) error = %v", err)
		}
		if changed.Activation.Version <= staleVersion ||
			changed.Activation.NetworkRequirementDigest == oldDigest ||
			changed.Activation.ConfirmedBy != "" ||
			changed.Activation.ConfirmedAt != "" {
			t.Fatalf("changed activation = %#v, want bumped unconfirmed digest", changed.Activation)
		}

		_, err = service.ConfirmNetworkRequirement(
			testutil.Context(t),
			activated.Activation.ID,
			staleVersion,
		)
		if !errors.Is(err, resources.ErrConflict) {
			t.Fatalf("ConfirmNetworkRequirement(stale) error = %v, want ErrConflict", err)
		}
		confirmed, err := service.ConfirmNetworkRequirement(
			testutil.Context(t),
			activated.Activation.ID,
			changed.Activation.Version,
		)
		if err != nil {
			t.Fatalf("ConfirmNetworkRequirement(current) error = %v", err)
		}
		if confirmed.Activation.Version <= changed.Activation.Version ||
			confirmed.Activation.ConfirmedBy != networkRequirementConfirmedByOperator ||
			confirmed.Activation.ConfirmedAt != "2026-04-14T22:00:00Z" {
			t.Fatalf("confirmed activation = %#v", confirmed.Activation)
		}
	})

	t.Run("Should reject duplicate activation instead of updating without CAS", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		ext := newMarketingExtension()
		ext.Manifest.NetworkParticipation = &extensionpkg.NetworkParticipationRequirement{
			Required: true,
			Mode:     string(participation.ModeLive),
		}
		service := newServiceForExtensions(store, []*extensionpkg.Extension{ext})
		request := ActivateRequest{
			ExtensionName:             ext.Info.Name,
			BundleName:                "marketing",
			ProfileName:               "default",
			Scope:                     ScopeGlobal,
			ConfirmNetworkRequirement: true,
		}
		created, err := service.Activate(testutil.Context(t), request)
		if err != nil {
			t.Fatalf("Activate(create) error = %v", err)
		}
		if _, err := service.Activate(testutil.Context(t), request); !errors.Is(err, resources.ErrConflict) {
			t.Fatalf("Activate(duplicate) error = %v, want ErrConflict", err)
		}
		current, err := service.GetActivation(testutil.Context(t), created.Activation.ID)
		if err != nil {
			t.Fatalf("GetActivation() error = %v", err)
		}
		if current.Activation.Version != created.Activation.Version {
			t.Fatalf(
				"duplicate activation version = %d, want unchanged %d",
				current.Activation.Version,
				created.Activation.Version,
			)
		}
	})

	t.Run("Should reject missing confirmation before registering a workspace", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		ext := newMarketingExtension()
		ext.Manifest.NetworkParticipation = &extensionpkg.NetworkParticipationRequirement{
			Required: true,
			Mode:     string(participation.ModeLive),
		}
		registerCalls := 0
		service := newServiceForExtensions(
			store,
			[]*extensionpkg.Extension{ext},
			WithWorkspaceResolver(memoryWorkspaceResolver{
				resolveOrRegisterFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
					registerCalls++
					return workspacepkg.ResolvedWorkspace{}, nil
				},
			}),
		)
		_, err := service.Activate(testutil.Context(t), ActivateRequest{
			ExtensionName: ext.Info.Name,
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeWorkspace,
			Workspace:     t.TempDir(),
		})
		if !errors.Is(err, ErrNetworkRequirementConfirmationRequired) {
			t.Fatalf("Activate() error = %v, want ErrNetworkRequirementConfirmationRequired", err)
		}
		if registerCalls != 0 {
			t.Fatalf("ResolveOrRegister calls = %d, want 0", registerCalls)
		}
	})

	t.Run("Should fail closed when the extension manifest is unavailable", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		ext := newMarketingExtension()
		ext.Manifest = nil
		service := newServiceForExtensions(store, []*extensionpkg.Extension{ext})
		_, err := service.Activate(testutil.Context(t), ActivateRequest{
			ExtensionName: ext.Info.Name,
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		})
		if err == nil || !strings.Contains(err.Error(), "manifest is unavailable") {
			t.Fatalf("Activate() error = %v, want unavailable manifest failure", err)
		}
		if len(store.activations) != 0 {
			t.Fatalf("activations = %#v, want no persisted activation", store.activations)
		}
	})

	t.Run("Should invalidate changed digest before projector materialization", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		ext := newMarketingExtension()
		ext.Manifest.NetworkParticipation = &extensionpkg.NetworkParticipationRequirement{
			Required:      true,
			Mode:          string(participation.ModeLive),
			ChannelScopes: []string{"builders"},
		}
		service := newServiceForExtensions(store, []*extensionpkg.Extension{ext})
		created, err := service.Activate(testutil.Context(t), ActivateRequest{
			ExtensionName:             ext.Info.Name,
			BundleName:                "marketing",
			ProfileName:               "default",
			Scope:                     ScopeGlobal,
			ConfirmNetworkRequirement: true,
		})
		if err != nil {
			t.Fatalf("Activate() error = %v", err)
		}
		stale := resources.Record[ActivationResourceSpec]{
			Kind:    BundleActivationResourceKind,
			ID:      created.Activation.ID,
			Version: created.Activation.Version,
			Scope:   resourceScopeForActivation(created.Activation),
			Spec:    activationResourceSpecFromActivation(created.Activation),
		}
		ext.Manifest.NetworkParticipation.ChannelScopes = []string{"operators"}

		if _, err := service.Build(
			testutil.Context(t),
			[]resources.Record[ActivationResourceSpec]{stale},
			store.bundles,
		); !errors.Is(
			err,
			ErrNetworkRequirementConfirmationRequired,
		) {
			t.Fatalf("Build(changed digest) error = %v, want confirmation required", err)
		}
		invalidated, err := store.GetBundleActivation(testutil.Context(t), created.Activation.ID)
		if err != nil {
			t.Fatalf("GetBundleActivation() error = %v", err)
		}
		if invalidated.Version <= stale.Version || invalidated.ConfirmedBy != "" || invalidated.ConfirmedAt != "" {
			t.Fatalf("invalidated activation = %#v, want advanced unconfirmed state", invalidated)
		}

		if _, err := service.Build(
			testutil.Context(t),
			[]resources.Record[ActivationResourceSpec]{stale},
			store.bundles,
		); !errors.Is(
			err,
			resources.ErrConflict,
		) {
			t.Fatalf("Build(stale replay) error = %v, want ErrConflict", err)
		}
		current, err := store.GetBundleActivation(testutil.Context(t), created.Activation.ID)
		if err != nil {
			t.Fatalf("GetBundleActivation(stale replay) error = %v", err)
		}
		if current.Version != invalidated.Version || current.ConfirmedBy != "" || current.ConfirmedAt != "" {
			t.Fatalf("stale projector restored confirmation: %#v", current)
		}
	})
}
