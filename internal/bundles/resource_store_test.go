package bundles

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
	storepkg "github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/windowmanager"
)

type bundleResourceUnitHarness struct {
	ctx            context.Context
	db             *sql.DB
	kernel         *resources.Kernel
	actor          resources.MutationActor
	resourceStore  *ResourceStore
	bundles        resources.Store[BundleResourceSpec]
	activations    resources.Store[ActivationResourceSpec]
	layouts        resources.Store[windowmanager.LayoutResource]
	agents         resources.Store[aghconfig.AgentDef]
	souls          resources.Store[soul.ResourceSpec]
	heartbeats     resources.Store[heartbeat.ResourceSpec]
	jobs           resources.Store[automationpkg.Job]
	triggers       resources.Store[automationpkg.Trigger]
	bridges        resources.Store[bridgepkg.BridgeInstanceSpec]
	triggeredKinds []resources.ResourceKind
}

type failingJobStore struct {
	base   resources.Store[automationpkg.Job]
	putErr error
}

type failingLayoutStore struct {
	base   resources.Store[windowmanager.LayoutResource]
	putErr error
}

type deadlineObservingAgentStore struct {
	base            resources.Store[aghconfig.AgentDef]
	listSawDeadline bool
}

func (s failingJobStore) Put(
	_ context.Context,
	_ resources.MutationActor,
	_ resources.Draft[automationpkg.Job],
) (resources.Record[automationpkg.Job], error) {
	return resources.Record[automationpkg.Job]{}, s.putErr
}

func (s failingJobStore) Delete(
	ctx context.Context,
	actor resources.MutationActor,
	id string,
	expectedVersion int64,
) error {
	return s.base.Delete(ctx, actor, id, expectedVersion)
}

func (s failingJobStore) Get(
	ctx context.Context,
	actor resources.MutationActor,
	id string,
) (resources.Record[automationpkg.Job], error) {
	return s.base.Get(ctx, actor, id)
}

func (s failingJobStore) List(
	ctx context.Context,
	actor resources.MutationActor,
	filter resources.ResourceFilter,
) ([]resources.Record[automationpkg.Job], error) {
	return s.base.List(ctx, actor, filter)
}

func (s failingLayoutStore) Put(
	ctx context.Context,
	actor resources.MutationActor,
	draft resources.Draft[windowmanager.LayoutResource],
) (resources.Record[windowmanager.LayoutResource], error) {
	record, err := s.base.Put(ctx, actor, draft)
	if err != nil {
		return resources.Record[windowmanager.LayoutResource]{}, err
	}
	return record, s.putErr
}

func (s failingLayoutStore) Delete(
	ctx context.Context,
	actor resources.MutationActor,
	id string,
	expectedVersion int64,
) error {
	return s.base.Delete(ctx, actor, id, expectedVersion)
}

func (s failingLayoutStore) Get(
	ctx context.Context,
	actor resources.MutationActor,
	id string,
) (resources.Record[windowmanager.LayoutResource], error) {
	return s.base.Get(ctx, actor, id)
}

func (s failingLayoutStore) List(
	ctx context.Context,
	actor resources.MutationActor,
	filter resources.ResourceFilter,
) ([]resources.Record[windowmanager.LayoutResource], error) {
	return s.base.List(ctx, actor, filter)
}

func (s *deadlineObservingAgentStore) Put(
	ctx context.Context,
	actor resources.MutationActor,
	draft resources.Draft[aghconfig.AgentDef],
) (resources.Record[aghconfig.AgentDef], error) {
	return s.base.Put(ctx, actor, draft)
}

func (s *deadlineObservingAgentStore) Delete(
	ctx context.Context,
	actor resources.MutationActor,
	id string,
	expectedVersion int64,
) error {
	return s.base.Delete(ctx, actor, id, expectedVersion)
}

func (s *deadlineObservingAgentStore) Get(
	ctx context.Context,
	actor resources.MutationActor,
	id string,
) (resources.Record[aghconfig.AgentDef], error) {
	return s.base.Get(ctx, actor, id)
}

func (s *deadlineObservingAgentStore) List(
	ctx context.Context,
	actor resources.MutationActor,
	filter resources.ResourceFilter,
) ([]resources.Record[aghconfig.AgentDef], error) {
	_, s.listSawDeadline = ctx.Deadline()
	return s.base.List(ctx, actor, filter)
}

func TestBundleResourceCodecsValidateAndNormalize(t *testing.T) {
	t.Parallel()

	ext := newMarketingExtension()
	bundleCodec, err := NewBundleResourceCodec()
	if err != nil {
		t.Fatalf("NewBundleResourceCodec() error = %v", err)
	}
	decodedBundle, err := bundleCodec.DecodeAndValidate(
		testutil.Context(t),
		resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		mustEncodeJSON(t, BundleResourceSpec{
			ExtensionName:       " marketing-team ",
			Bundle:              ext.Bundles[0],
			OwnerBridgePlatform: " telegram ",
		}),
	)
	if err != nil {
		t.Fatalf("DecodeAndValidate(bundle) error = %v", err)
	}
	if got, want := decodedBundle.ExtensionName, "marketing-team"; got != want {
		t.Fatalf("decodedBundle.ExtensionName = %q, want %q", got, want)
	}
	if !decodedBundle.OwnerProvidesBridgeAdapter {
		t.Fatal("decodedBundle.OwnerProvidesBridgeAdapter = false, want true")
	}
	if got, want := decodedBundle.Bundle.Profiles[0].Layouts[0].Layout.AspectVariant,
		windowmanager.LayoutAspectAny; got != want {
		t.Fatalf("decoded layout aspect = %q, want %q", got, want)
	}

	invalidBundle := cloneBundleSpec(ext.Bundles[0])
	invalidBundle.Profiles[0].Layouts[0].Layout.ParticipantSlots = []windowmanager.WindowID{
		"duplicate",
		"duplicate",
	}
	if _, err := bundleCodec.DecodeAndValidate(
		testutil.Context(t),
		resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		mustEncodeJSON(t, BundleResourceSpec{
			ExtensionName: "marketing-team",
			Bundle:        invalidBundle,
		}),
	); !errors.Is(err, resources.ErrValidation) {
		t.Fatalf("DecodeAndValidate(invalid embedded layout) error = %v, want ErrValidation", err)
	}

	activationCodec, err := NewActivationResourceCodec()
	if err != nil {
		t.Fatalf("NewActivationResourceCodec() error = %v", err)
	}
	decodedActivation, err := activationCodec.DecodeAndValidate(
		testutil.Context(t),
		resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"},
		mustEncodeJSON(t, ActivationResourceSpec{
			ExtensionName:   " marketing-team ",
			BundleName:      " marketing ",
			ProfileName:     " default ",
			SpecContentHash: " hash ",
		}),
	)
	if err != nil {
		t.Fatalf("DecodeAndValidate(activation) error = %v", err)
	}
	if got, want := decodedActivation.SpecContentHash, "hash"; got != want {
		t.Fatalf("decodedActivation.SpecContentHash = %q, want %q", got, want)
	}
	if _, err := activationCodec.DecodeAndValidate(
		testutil.Context(t),
		resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		mustEncodeJSON(t, ActivationResourceSpec{ExtensionName: "marketing-team"}),
	); err == nil {
		t.Fatal("DecodeAndValidate(invalid activation) error = nil, want validation failure")
	}
}

func TestResourceStoreActivationCRUDInventoryAndApply(t *testing.T) {
	t.Parallel()

	h := newBundleResourceUnitHarness(t)
	h.putMarketingBundle(t)
	activation := Activation{
		ID:            ActivationResourceID("marketing-team", "marketing", "default", ScopeGlobal, ""),
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
	}
	if err := h.resourceStore.CreateBundleActivation(h.ctx, activation); err != nil {
		t.Fatalf("CreateBundleActivation() error = %v", err)
	}
	loaded, err := h.resourceStore.GetBundleActivation(h.ctx, activation.ID)
	if err != nil {
		t.Fatalf("GetBundleActivation() error = %v", err)
	}
	if got, want := loaded.ID, activation.ID; got != want {
		t.Fatalf("loaded.ID = %q, want %q", got, want)
	}
	if got, want := loaded.Version, int64(1); got != want {
		t.Fatalf("loaded.Version = %d, want %d", got, want)
	}
	loaded.SpecContentHash = "updated-hash"
	if err := h.resourceStore.UpdateBundleActivation(h.ctx, loaded); err != nil {
		t.Fatalf("UpdateBundleActivation() error = %v", err)
	}
	updated, err := h.resourceStore.GetBundleActivation(h.ctx, activation.ID)
	if err != nil {
		t.Fatalf("GetBundleActivation(updated) error = %v", err)
	}
	if got, want := updated.SpecContentHash, "updated-hash"; got != want {
		t.Fatalf("updated.SpecContentHash = %q, want %q", got, want)
	}
	if got, want := updated.Version, int64(2); got != want {
		t.Fatalf("updated.Version = %d, want %d", got, want)
	}
	loaded.SpecContentHash = "stale-overwrite"
	if err := h.resourceStore.UpdateBundleActivation(h.ctx, loaded); !errors.Is(err, resources.ErrConflict) {
		t.Fatalf("UpdateBundleActivation(stale) error = %v, want ErrConflict", err)
	}
	activations, err := h.resourceStore.ListBundleActivations(h.ctx)
	if err != nil {
		t.Fatalf("ListBundleActivations() error = %v", err)
	}
	if got, want := len(activations), 1; got != want {
		t.Fatalf("len(activations) = %d, want %d", got, want)
	}
	bundles, err := h.resourceStore.ListBundleResources(h.ctx)
	if err != nil {
		t.Fatalf("ListBundleResources() error = %v", err)
	}
	if got, want := len(bundles), 1; got != want {
		t.Fatalf("len(bundles) = %d, want %d", got, want)
	}

	layout := bundleTestLayoutResource("layout-owned", "Owned Layout")
	job := unitJob("job-owned", "owned")
	trigger := unitTrigger("trigger-owned", "owned-trigger")
	bridge := unitBridge("bridge-owned", "Owned Bridge")
	err = h.resourceStore.ApplyBundleActivationResources(h.ctx, BundleActivationResourcePlan{
		activeActivationIDs: map[string]struct{}{activation.ID: {}},
		desiredLayouts: []ownedLayoutResource{{
			ID: layout.ID, Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}, Spec: layout,
		}},
		desiredJobs:     []automationpkg.Job{job},
		desiredTriggers: []automationpkg.Trigger{trigger},
		desiredBridges:  []bridgepkg.BridgeInstance{bridge},
		layoutOwners:    map[string]string{layout.ID: activation.ID},
		jobOwners:       map[string]string{job.ID: activation.ID},
		triggerOwners:   map[string]string{trigger.ID: activation.ID},
		bridgeOwners:    map[string]string{bridge.ID: activation.ID},
	})
	if err != nil {
		t.Fatalf("ApplyBundleActivationResources() error = %v", err)
	}
	for _, kind := range []resources.ResourceKind{
		windowmanager.WindowLayoutResourceKind,
		automationpkg.JobResourceKind,
		automationpkg.TriggerResourceKind,
		bridgepkg.BridgeInstanceResourceKind,
	} {
		if !slices.Contains(h.triggeredKinds, kind) {
			t.Fatalf("triggered kinds = %#v, want %q", h.triggeredKinds, kind)
		}
	}
	h.triggeredKinds = nil
	if err := h.resourceStore.ApplyBundleActivationResources(h.ctx, BundleActivationResourcePlan{
		activeActivationIDs: map[string]struct{}{activation.ID: {}},
		desiredLayouts: []ownedLayoutResource{{
			ID: layout.ID, Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}, Spec: layout,
		}},
		desiredJobs:     []automationpkg.Job{job},
		desiredTriggers: []automationpkg.Trigger{trigger},
		desiredBridges:  []bridgepkg.BridgeInstance{bridge},
		layoutOwners:    map[string]string{layout.ID: activation.ID},
		jobOwners:       map[string]string{job.ID: activation.ID},
		triggerOwners:   map[string]string{trigger.ID: activation.ID},
		bridgeOwners:    map[string]string{bridge.ID: activation.ID},
	}); err != nil {
		t.Fatalf("ApplyBundleActivationResources(unchanged) error = %v", err)
	}
	if len(h.triggeredKinds) != 0 {
		t.Fatalf("triggered kinds after unchanged apply = %#v, want none", h.triggeredKinds)
	}
	inventory, err := h.resourceStore.ListBundleActivationInventory(h.ctx, activation.ID)
	if err != nil {
		t.Fatalf("ListBundleActivationInventory() error = %v", err)
	}
	if got, want := len(inventory), 4; got != want {
		t.Fatalf("len(inventory) = %d, want %d", got, want)
	}
	layoutRecord, err := h.layouts.Get(h.ctx, h.actor, layout.ID)
	if err != nil {
		t.Fatalf("Get(layout) error = %v", err)
	}
	if layoutRecord.ID != layoutRecord.Spec.ID {
		t.Fatalf("layout record/spec IDs = %q/%q, want identical", layoutRecord.ID, layoutRecord.Spec.ID)
	}
	if got, want := layoutRecord.Scope.Kind, resources.ResourceScopeKindGlobal; got != want {
		t.Fatalf("layout scope = %q, want %q", got, want)
	}
	if err := h.resourceStore.DeleteBundleActivation(h.ctx, activation.ID); err != nil {
		t.Fatalf("DeleteBundleActivation() error = %v", err)
	}
	if _, err := h.resourceStore.GetBundleActivation(h.ctx, activation.ID); !errors.Is(err, ErrActivationNotFound) {
		t.Fatalf("GetBundleActivation(after delete) error = %v, want ErrActivationNotFound", err)
	}
}

func TestResourceStoreWorkspaceActivationAndNotFoundErrors(t *testing.T) {
	t.Parallel()

	h := newBundleResourceUnitHarness(t)
	workspaceActivation := Activation{
		ID:            ActivationResourceID("marketing-team", "marketing", "default", ScopeWorkspace, "ws-1"),
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeWorkspace,
		WorkspaceID:   "ws-1",
	}
	if err := h.resourceStore.CreateBundleActivation(h.ctx, workspaceActivation); err != nil {
		t.Fatalf("CreateBundleActivation(workspace) error = %v", err)
	}
	loaded, err := h.resourceStore.GetBundleActivation(h.ctx, workspaceActivation.ID)
	if err != nil {
		t.Fatalf("GetBundleActivation(workspace) error = %v", err)
	}
	if got, want := loaded.WorkspaceID, "ws-1"; got != want {
		t.Fatalf("WorkspaceID = %q, want %q", got, want)
	}
	if _, err := h.resourceStore.GetBundleActivation(h.ctx, "missing"); !errors.Is(err, ErrActivationNotFound) {
		t.Fatalf("GetBundleActivation(missing) error = %v, want ErrActivationNotFound", err)
	}
	if err := h.resourceStore.DeleteBundleActivation(h.ctx, ""); err == nil {
		t.Fatal("DeleteBundleActivation(empty) error = nil, want validation failure")
	}
	missing := workspaceActivation
	missing.ID = "missing"
	missing.Version = loaded.Version
	if err := h.resourceStore.UpdateBundleActivation(h.ctx, missing); !errors.Is(err, ErrActivationNotFound) {
		t.Fatalf("UpdateBundleActivation(missing) error = %v, want ErrActivationNotFound", err)
	}
}

func TestResourceStoreApplyRollsBackEarlierKindsOnLaterFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should roll back earlier kinds on later failure", func(t *testing.T) {
		t.Parallel()

		h := newBundleResourceUnitHarness(t)
		putErr := errors.New("put layout failed")
		h.resourceStore.layouts = failingLayoutStore{
			base:   h.layouts,
			putErr: putErr,
		}

		scope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		agent := ownedAgentResource{
			ID:    "agent_owned_atomic",
			Scope: scope,
			Spec:  bundleAgentRecord("agent_owned_atomic", "atomic-agent", scope).Spec,
		}
		layout := bundleTestLayoutResource("layout-owned-atomic", "Atomic layout")
		activationID := "act-atomic"

		err := h.resourceStore.ApplyBundleActivationResources(h.ctx, BundleActivationResourcePlan{
			activeActivationIDs: map[string]struct{}{activationID: {}},
			desiredLayouts:      []ownedLayoutResource{{ID: layout.ID, Scope: scope, Spec: layout}},
			desiredAgents:       []ownedAgentResource{agent},
			layoutOwners:        map[string]string{layout.ID: activationID},
			agentOwners:         map[string]string{agent.ID: activationID},
		})
		if !errors.Is(err, putErr) {
			t.Fatalf("ApplyBundleActivationResources() error = %v, want %v", err, putErr)
		}
		if _, err := h.agents.Get(h.ctx, h.actor, agent.ID); !errors.Is(err, resources.ErrNotFound) {
			t.Fatalf("Get(agent) error = %v, want ErrNotFound after rollback", err)
		}
		if _, err := h.layouts.Get(h.ctx, h.actor, layout.ID); !errors.Is(err, resources.ErrNotFound) {
			t.Fatalf("Get(layout) error = %v, want ErrNotFound after rollback", err)
		}
		if len(h.triggeredKinds) != 0 {
			t.Fatalf("triggered kinds after rollback = %#v, want none", h.triggeredKinds)
		}
		inventory, err := h.resourceStore.ListBundleActivationInventory(h.ctx, activationID)
		if err != nil {
			t.Fatalf("ListBundleActivationInventory() error = %v", err)
		}
		if len(inventory) != 0 {
			t.Fatalf("inventory after rollback = %#v, want empty", inventory)
		}
	})

	t.Run("Should preserve the caller deadline during rollback", func(t *testing.T) {
		t.Parallel()

		h := newBundleResourceUnitHarness(t)
		observingAgents := &deadlineObservingAgentStore{base: h.agents}
		h.resourceStore.agents = observingAgents
		putErr := errors.New("put job failed")
		h.resourceStore.jobs = failingJobStore{
			base:   h.jobs,
			putErr: putErr,
		}

		scope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		agent := ownedAgentResource{
			ID:    "agent_owned_deadline",
			Scope: scope,
			Spec:  bundleAgentRecord("agent_owned_deadline", "deadline-agent", scope).Spec,
		}
		job := unitJob("job-owned-deadline", "deadline-job")
		ctx, cancel := context.WithTimeout(h.ctx, time.Second)
		defer cancel()

		err := h.resourceStore.ApplyBundleActivationResources(ctx, BundleActivationResourcePlan{
			activeActivationIDs: map[string]struct{}{"act-deadline": {}},
			desiredAgents:       []ownedAgentResource{agent},
			desiredJobs:         []automationpkg.Job{job},
			agentOwners:         map[string]string{agent.ID: "act-deadline"},
			jobOwners:           map[string]string{job.ID: "act-deadline"},
		})
		if !errors.Is(err, putErr) {
			t.Fatalf("ApplyBundleActivationResources() error = %v, want %v", err, putErr)
		}
		if !observingAgents.listSawDeadline {
			t.Fatal("rollback agent store list did not observe caller deadline")
		}
	})
}

func TestResourceStoreCleanupDeletesOnlyOwnedInactiveActivationRecords(t *testing.T) {
	t.Parallel()

	h := newBundleResourceUnitHarness(t)
	removeJob := unitJob("job-remove", "remove")
	keepJob := unitJob("job-keep", "keep")
	unownedJob := unitJob("job-unowned", "unowned")
	h.putJob(t, activationResourceActor(h.actor, "act-remove"), removeJob)
	h.putJob(t, activationResourceActor(h.actor, "act-keep"), keepJob)
	h.putJob(t, h.actor, unownedJob)

	err := h.resourceStore.ApplyBundleActivationResources(h.ctx, BundleActivationResourcePlan{
		activeActivationIDs: map[string]struct{}{"act-keep": {}},
		desiredJobs:         []automationpkg.Job{keepJob},
		jobOwners:           map[string]string{keepJob.ID: "act-keep"},
	})
	if err != nil {
		t.Fatalf("ApplyBundleActivationResources() error = %v", err)
	}
	if _, err := h.jobs.Get(h.ctx, h.actor, removeJob.ID); !errors.Is(err, resources.ErrNotFound) {
		t.Fatalf("Get(removeJob) error = %v, want ErrNotFound", err)
	}
	if _, err := h.jobs.Get(h.ctx, h.actor, keepJob.ID); err != nil {
		t.Fatalf("Get(keepJob) error = %v", err)
	}
	if _, err := h.jobs.Get(h.ctx, h.actor, unownedJob.ID); err != nil {
		t.Fatalf("Get(unownedJob) error = %v", err)
	}
}

func TestResourceStoreRejectsMissingOwnedResourceOwner(t *testing.T) {
	t.Parallel()

	h := newBundleResourceUnitHarness(t)
	job := unitJob("job-missing-owner", "missing-owner")
	err := h.resourceStore.ApplyBundleActivationResources(h.ctx, BundleActivationResourcePlan{
		activeActivationIDs: map[string]struct{}{"act-missing": {}},
		desiredJobs:         []automationpkg.Job{job},
	})
	if err == nil {
		t.Fatal("ApplyBundleActivationResources() error = nil, want missing owner failure")
	}
}

func TestResourceStoreOrderingComparators(t *testing.T) {
	t.Parallel()

	base := Activation{
		ID:            "act-a",
		ExtensionName: "ext-a",
		BundleName:    "bundle-a",
		ProfileName:   "profile-a",
		Scope:         ScopeGlobal,
	}
	activationCases := []struct {
		name  string
		left  Activation
		right Activation
	}{
		{
			name:  "extension name",
			left:  base,
			right: activationWith(base, func(next *Activation) { next.ExtensionName = "ext-b" }),
		},
		{
			name:  "bundle name",
			left:  base,
			right: activationWith(base, func(next *Activation) { next.BundleName = "bundle-b" }),
		},
		{
			name:  "profile name",
			left:  base,
			right: activationWith(base, func(next *Activation) { next.ProfileName = "profile-b" }),
		},
		{
			name:  "scope",
			left:  base,
			right: activationWith(base, func(next *Activation) { next.Scope = ScopeWorkspace }),
		},
		{
			name:  "workspace id",
			left:  activationWith(base, func(next *Activation) { next.WorkspaceID = "ws-a" }),
			right: activationWith(base, func(next *Activation) { next.WorkspaceID = "ws-b" }),
		},
		{
			name:  "id",
			left:  base,
			right: activationWith(base, func(next *Activation) { next.ID = "act-b" }),
		},
	}
	for _, tc := range activationCases {
		t.Run("Should activation/"+tc.name, func(t *testing.T) {
			t.Parallel()

			if compareActivations(tc.left, tc.right) >= 0 {
				t.Fatalf("compareActivations(%s) >= 0, want left before right", tc.name)
			}
		})
	}

	inventoryCases := []struct {
		name  string
		left  InventoryItem
		right InventoryItem
	}{
		{
			name:  "kind",
			left:  InventoryItem{ResourceKind: "automation.job", ResourceName: "same", ResourceID: "same"},
			right: InventoryItem{ResourceKind: "bridge.instance", ResourceName: "same", ResourceID: "same"},
		},
		{
			name:  "name",
			left:  InventoryItem{ResourceKind: "automation.job", ResourceName: "alpha", ResourceID: "same"},
			right: InventoryItem{ResourceKind: "automation.job", ResourceName: "beta", ResourceID: "same"},
		},
		{
			name:  "id",
			left:  InventoryItem{ResourceKind: "automation.job", ResourceName: "same", ResourceID: "id-a"},
			right: InventoryItem{ResourceKind: "automation.job", ResourceName: "same", ResourceID: "id-b"},
		},
	}
	for _, tc := range inventoryCases {
		t.Run("Should inventory/"+tc.name, func(t *testing.T) {
			t.Parallel()

			if compareInventoryItems(tc.left, tc.right) >= 0 {
				t.Fatalf("compareInventoryItems(%s) >= 0, want left before right", tc.name)
			}
		})
	}
}

func activationWith(base Activation, mutate func(*Activation)) Activation {
	next := base
	mutate(&next)
	return next
}

func TestNewResourceStoreAppliesDefaultActor(t *testing.T) {
	t.Parallel()

	h := newBundleResourceUnitHarness(t)
	validConfig := ResourceStoreConfig{
		Bundles:         h.bundles,
		BundleCodec:     h.resourceStore.bundleCodec,
		Activations:     h.activations,
		ActivationCodec: h.resourceStore.activationCodec,
		Layouts:         h.layouts,
		LayoutCodec:     h.resourceStore.layoutCodec,
		Agents:          h.agents,
		AgentCodec:      h.resourceStore.agentCodec,
		Souls:           h.souls,
		SoulCodec:       h.resourceStore.soulCodec,
		Heartbeats:      h.heartbeats,
		HeartbeatCodec:  h.resourceStore.heartbeatCodec,
		Jobs:            h.jobs,
		JobCodec:        h.resourceStore.jobCodec,
		Triggers:        h.triggers,
		TriggerCodec:    h.resourceStore.triggerCodec,
		Bridges:         h.bridges,
		BridgeCodec:     h.resourceStore.bridgeCodec,
	}
	store, err := NewResourceStore(validConfig)
	if err != nil {
		t.Fatalf("NewResourceStore(default actor) error = %v", err)
	}
	if got, want := store.actor.ID, "bundle-resource"; got != want {
		t.Fatalf("store.actor.ID = %q, want %q", got, want)
	}
	if _, err := NewResourceStore(ResourceStoreConfig{}); err == nil {
		t.Fatal("NewResourceStore(empty config) error = nil, want validation failure")
	}

	testCases := []struct {
		name   string
		mutate func(*ResourceStoreConfig)
	}{
		{name: "bundle codec", mutate: func(cfg *ResourceStoreConfig) { cfg.BundleCodec = nil }},
		{name: "activations", mutate: func(cfg *ResourceStoreConfig) { cfg.Activations = nil }},
		{name: "activation codec", mutate: func(cfg *ResourceStoreConfig) { cfg.ActivationCodec = nil }},
		{name: "layout store", mutate: func(cfg *ResourceStoreConfig) { cfg.Layouts = nil }},
		{name: "layout codec", mutate: func(cfg *ResourceStoreConfig) { cfg.LayoutCodec = nil }},
		{name: "agent store", mutate: func(cfg *ResourceStoreConfig) { cfg.Agents = nil }},
		{name: "agent codec", mutate: func(cfg *ResourceStoreConfig) { cfg.AgentCodec = nil }},
		{name: "soul store", mutate: func(cfg *ResourceStoreConfig) { cfg.Souls = nil }},
		{name: "soul codec", mutate: func(cfg *ResourceStoreConfig) { cfg.SoulCodec = nil }},
		{name: "heartbeat store", mutate: func(cfg *ResourceStoreConfig) { cfg.Heartbeats = nil }},
		{name: "heartbeat codec", mutate: func(cfg *ResourceStoreConfig) { cfg.HeartbeatCodec = nil }},
		{name: "job store", mutate: func(cfg *ResourceStoreConfig) { cfg.Jobs = nil }},
		{name: "trigger store", mutate: func(cfg *ResourceStoreConfig) { cfg.Triggers = nil }},
		{name: "bridge store", mutate: func(cfg *ResourceStoreConfig) { cfg.Bridges = nil }},
	}
	for _, tc := range testCases {
		t.Run("Should reject missing "+tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := validConfig
			tc.mutate(&cfg)
			if _, err := NewResourceStore(cfg); err == nil {
				t.Fatalf("NewResourceStore(%s) error = nil, want validation failure", tc.name)
			}
		})
	}
}

func newBundleResourceUnitHarness(t *testing.T) *bundleResourceUnitHarness {
	t.Helper()

	ctx := testutil.Context(t)
	db, err := storepkg.OpenSQLiteDatabase(
		ctx,
		filepath.Join(t.TempDir(), storepkg.GlobalDatabaseName),
		func(ctx context.Context, db *sql.DB) error {
			return storepkg.Apply(ctx, db, globaldb.MigrationStream())
		},
	)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("db.Close() error = %v", err)
		}
	})
	kernel, err := resources.NewKernel(db, resources.WithNow(unitNow))
	if err != nil {
		t.Fatalf("NewKernel() error = %v", err)
	}

	bundleCodec, err := NewBundleResourceCodec()
	if err != nil {
		t.Fatalf("NewBundleResourceCodec() error = %v", err)
	}
	activationCodec, err := NewActivationResourceCodec()
	if err != nil {
		t.Fatalf("NewActivationResourceCodec() error = %v", err)
	}
	layoutCodec, err := windowmanager.NewLayoutResourceCodec()
	if err != nil {
		t.Fatalf("NewLayoutResourceCodec() error = %v", err)
	}
	agentCodec, err := aghconfig.NewAgentResourceCodec()
	if err != nil {
		t.Fatalf("NewAgentResourceCodec() error = %v", err)
	}
	soulCodec, err := soul.NewResourceCodec()
	if err != nil {
		t.Fatalf("NewSoulResourceCodec() error = %v", err)
	}
	heartbeatCodec, err := heartbeat.NewResourceCodec()
	if err != nil {
		t.Fatalf("NewHeartbeatResourceCodec() error = %v", err)
	}
	jobCodec, err := automationpkg.NewJobResourceCodec()
	if err != nil {
		t.Fatalf("NewJobResourceCodec() error = %v", err)
	}
	triggerCodec, err := automationpkg.NewTriggerResourceCodec()
	if err != nil {
		t.Fatalf("NewTriggerResourceCodec() error = %v", err)
	}
	bridgeCodec, err := bridgepkg.NewBridgeInstanceResourceCodec(unitBridgeProviderLookup)
	if err != nil {
		t.Fatalf("NewBridgeInstanceResourceCodec() error = %v", err)
	}

	bundleStore := mustNewUnitTypedStore(t, kernel, bundleCodec)
	activationStore := mustNewUnitTypedStore(t, kernel, activationCodec)
	layoutStore := mustNewUnitTypedStore(t, kernel, layoutCodec)
	agentStore := mustNewUnitTypedStore(t, kernel, agentCodec)
	soulStore := mustNewUnitTypedStore(t, kernel, soulCodec)
	heartbeatStore := mustNewUnitTypedStore(t, kernel, heartbeatCodec)
	jobStore := mustNewUnitTypedStore(t, kernel, jobCodec)
	triggerStore := mustNewUnitTypedStore(t, kernel, triggerCodec)
	bridgeStore := mustNewUnitTypedStore(t, kernel, bridgeCodec)
	actor := unitResourceActor()
	h := &bundleResourceUnitHarness{
		ctx:         ctx,
		db:          db,
		kernel:      kernel,
		actor:       actor,
		bundles:     bundleStore,
		activations: activationStore,
		layouts:     layoutStore,
		agents:      agentStore,
		souls:       soulStore,
		heartbeats:  heartbeatStore,
		jobs:        jobStore,
		triggers:    triggerStore,
		bridges:     bridgeStore,
	}
	resourceStore, err := NewResourceStore(ResourceStoreConfig{
		Bundles:         bundleStore,
		BundleCodec:     bundleCodec,
		Activations:     activationStore,
		ActivationCodec: activationCodec,
		Layouts:         layoutStore,
		LayoutCodec:     layoutCodec,
		Agents:          agentStore,
		AgentCodec:      agentCodec,
		Souls:           soulStore,
		SoulCodec:       soulCodec,
		Heartbeats:      heartbeatStore,
		HeartbeatCodec:  heartbeatCodec,
		Jobs:            jobStore,
		JobCodec:        jobCodec,
		Triggers:        triggerStore,
		TriggerCodec:    triggerCodec,
		Bridges:         bridgeStore,
		BridgeCodec:     bridgeCodec,
		Actor:           actor,
		Trigger: func(_ context.Context, kind resources.ResourceKind, _ resources.ReconcileReason) error {
			h.triggeredKinds = append(h.triggeredKinds, kind)
			return nil
		},
		Now: unitNow,
	})
	if err != nil {
		t.Fatalf("NewResourceStore() error = %v", err)
	}
	h.resourceStore = resourceStore
	return h
}

func mustNewUnitTypedStore[T any](
	t *testing.T,
	raw resources.RawStore,
	codec resources.KindCodec[T],
) resources.Store[T] {
	t.Helper()

	store, err := resources.NewStore(raw, codec)
	if err != nil {
		t.Fatalf("NewStore(%q) error = %v", codec.Kind(), err)
	}
	return store
}

func (h *bundleResourceUnitHarness) putMarketingBundle(t *testing.T) {
	t.Helper()

	ext := newMarketingExtension()
	_, err := h.bundles.Put(h.ctx, h.actor, resources.Draft[BundleResourceSpec]{
		ID:    BundleResourceID(ext.Info.Name, ext.Bundles[0].Name),
		Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		Spec: BundleResourceSpec{
			ExtensionName:              ext.Info.Name,
			Bundle:                     ext.Bundles[0],
			OwnerBridgePlatform:        ext.Manifest.Bridge.Platform,
			OwnerProvidesBridgeAdapter: true,
		},
	})
	if err != nil {
		t.Fatalf("Put(bundle) error = %v", err)
	}
}

func (h *bundleResourceUnitHarness) putJob(
	t *testing.T,
	actor resources.MutationActor,
	job automationpkg.Job,
) {
	t.Helper()

	_, err := h.jobs.Put(h.ctx, actor, resources.Draft[automationpkg.Job]{
		ID:    job.ID,
		Scope: automationpkg.ResourceScopeForAutomation(job.Scope, job.WorkspaceID),
		Spec:  job,
	})
	if err != nil {
		t.Fatalf("Put(job %s) error = %v", job.ID, err)
	}
}

func unitResourceActor() resources.MutationActor {
	return resources.MutationActor{
		Kind:     resources.MutationActorKindDaemon,
		ID:       "bundle-unit",
		Source:   resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "bundle-unit"},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}

func unitBridgeProviderLookup(
	_ context.Context,
	extensionName string,
) (bridgepkg.BridgeProvider, bool, error) {
	if extensionName != "marketing-team" {
		return bridgepkg.BridgeProvider{}, false, nil
	}
	return bridgepkg.BridgeProvider{
		Platform:      "telegram",
		ExtensionName: "marketing-team",
		DisplayName:   "Telegram",
		Enabled:       true,
	}, true, nil
}

func unitJob(id string, name string) automationpkg.Job {
	return automationpkg.Job{
		ID:        id,
		Scope:     automationpkg.AutomationScopeGlobal,
		Name:      name,
		AgentName: "planner",
		Prompt:    "Run " + name,
		Schedule: &automationpkg.ScheduleSpec{
			Mode:     automationpkg.ScheduleModeEvery,
			Interval: "1h",
		},
		Enabled:   true,
		Retry:     automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(),
		Source:    automationpkg.JobSourcePackage,
		CreatedAt: unitNow(),
		UpdatedAt: unitNow(),
	}
}

func unitTrigger(id string, name string) automationpkg.Trigger {
	return automationpkg.Trigger{
		ID:        id,
		Scope:     automationpkg.AutomationScopeGlobal,
		Name:      name,
		AgentName: "planner",
		Prompt:    "Handle " + name,
		Event:     "session.created",
		Enabled:   true,
		Retry:     automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(),
		Source:    automationpkg.JobSourcePackage,
		CreatedAt: unitNow(),
		UpdatedAt: unitNow(),
	}
}

func unitBridge(id string, name string) bridgepkg.BridgeInstance {
	return bridgepkg.BridgeInstance{
		ID:            id,
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "marketing-team",
		DisplayName:   name,
		Source:        bridgepkg.BridgeInstanceSourcePackage,
		Enabled:       false,
		Status:        bridgepkg.BridgeStatusDisabled,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		CreatedAt:     unitNow(),
		UpdatedAt:     unitNow(),
	}
}

func unitNow() time.Time {
	return time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC)
}

func mustEncodeJSON(t *testing.T, value any) []byte {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}
