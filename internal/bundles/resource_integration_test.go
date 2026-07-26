//go:build integration

package bundles

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
	storepkg "github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/windowmanager"
)

type bundleResourceIntegrationHarness struct {
	ctx            context.Context
	db             *sql.DB
	kernel         *resources.Kernel
	codecs         *resources.CodecRegistry
	actor          resources.MutationActor
	resourceStore  *ResourceStore
	service        *Service
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

func TestBundleResourceIntegrationActivationFanoutWritesCanonicalOwnedRecords(t *testing.T) {
	t.Parallel()

	h := newBundleResourceIntegrationHarness(t)
	h.putMarketingBundle(t)

	preview, err := h.service.Activate(h.ctx, ActivateRequest{
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
	})
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	owner := ownerForActivation(preview.Activation.ID)
	layouts := h.listOwnedLayouts(t, owner)
	if got, want := len(layouts), 1; got != want {
		t.Fatalf("len(owned layouts) = %d, want %d", got, want)
	}
	if layouts[0].ID != layouts[0].Spec.ID {
		t.Fatalf("layout record/spec IDs = %q/%q, want identical", layouts[0].ID, layouts[0].Spec.ID)
	}
	jobs := h.listOwnedJobs(t, owner)
	if got, want := len(jobs), 1; got != want {
		t.Fatalf("len(owned jobs) = %d, want %d", got, want)
	}
	if jobs[0].Owner != owner {
		t.Fatalf("job owner = %#v, want %#v", jobs[0].Owner, owner)
	}
	agents := h.listOwnedAgents(t, owner)
	if got, want := len(agents), 1; got != want {
		t.Fatalf("len(owned agents) = %d, want %d", got, want)
	}
	souls := h.listOwnedSouls(t, owner)
	if got, want := len(souls), 1; got != want {
		t.Fatalf("len(owned soul resources) = %d, want %d", got, want)
	}
	heartbeats := h.listOwnedHeartbeats(t, owner)
	if got, want := len(heartbeats), 1; got != want {
		t.Fatalf("len(owned heartbeat resources) = %d, want %d", got, want)
	}
	triggers := h.listOwnedTriggers(t, owner)
	if got, want := len(triggers), 1; got != want {
		t.Fatalf("len(owned triggers) = %d, want %d", got, want)
	}
	bridges := h.listOwnedBridges(t, owner)
	if got, want := len(bridges), 1; got != want {
		t.Fatalf("len(owned bridges) = %d, want %d", got, want)
	}
	for _, kind := range []resources.ResourceKind{
		windowmanager.WindowLayoutResourceKind,
		aghconfig.AgentResourceKind,
		soul.ResourceKind,
		heartbeat.ResourceKind,
		automationpkg.JobResourceKind,
		automationpkg.TriggerResourceKind,
		bridgepkg.BridgeInstanceResourceKind,
	} {
		if !slices.Contains(h.triggeredKinds, kind) {
			t.Fatalf("triggered kinds = %#v, want %q", h.triggeredKinds, kind)
		}
	}
	if err := h.service.Deactivate(h.ctx, preview.Activation.ID); err != nil {
		t.Fatalf("Deactivate() error = %v", err)
	}
	if got := len(h.listOwnedLayouts(t, owner)); got != 0 {
		t.Fatalf("len(owned layouts after deactivate) = %d, want 0", got)
	}
}

func TestBundleResourceIntegrationCleanupIsActivationScoped(t *testing.T) {
	t.Parallel()

	h := newBundleResourceIntegrationHarness(t)
	removeJob := integrationJob("job-remove", "remove")
	keepJob := integrationJob("job-keep", "keep")
	unownedJob := integrationJob("job-unowned", "unowned")

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

func TestBundleResourceIntegrationBootRebuildUsesResourcesWithoutInventoryTable(t *testing.T) {
	t.Parallel()

	h := newBundleResourceIntegrationHarness(t)
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

	registration, err := resources.NewBundleActivationProjectorRegistration[
		ActivationResourceSpec,
		BundleResourceSpec,
	](h.codecs, h.service)
	if err != nil {
		t.Fatalf("NewBundleActivationProjectorRegistration() error = %v", err)
	}
	driver, err := resources.NewReconcileDriver(h.kernel, h.actor, []resources.ProjectorRegistration{registration})
	if err != nil {
		t.Fatalf("NewReconcileDriver() error = %v", err)
	}
	t.Cleanup(func() {
		if err := driver.Close(h.ctx); err != nil {
			t.Fatalf("driver.Close() error = %v", err)
		}
	})

	if err := driver.RunBoot(h.ctx); err != nil {
		t.Fatalf("RunBoot() error = %v", err)
	}
	owner := ownerForActivation(activation.ID)
	if got, want := len(h.listOwnedJobs(t, owner)), 1; got != want {
		t.Fatalf("len(boot rebuilt owned jobs) = %d, want %d", got, want)
	}
	if got, want := len(h.listOwnedLayouts(t, owner)), 1; got != want {
		t.Fatalf("len(boot rebuilt owned layouts) = %d, want %d", got, want)
	}
	if got, want := len(h.listOwnedAgents(t, owner)), 1; got != want {
		t.Fatalf("len(boot rebuilt owned agents) = %d, want %d", got, want)
	}
	if got, want := len(h.listOwnedSouls(t, owner)), 1; got != want {
		t.Fatalf("len(boot rebuilt owned soul resources) = %d, want %d", got, want)
	}
	if got, want := len(h.listOwnedHeartbeats(t, owner)), 1; got != want {
		t.Fatalf("len(boot rebuilt owned heartbeat resources) = %d, want %d", got, want)
	}
	assertNoLegacyBundleActivationTable(t, h.db)
}

func newBundleResourceIntegrationHarness(t *testing.T) *bundleResourceIntegrationHarness {
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
	kernel, err := resources.NewKernel(db, resources.WithNow(func() time.Time {
		return time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
	}))
	if err != nil {
		t.Fatalf("NewKernel() error = %v", err)
	}
	actor := bundleIntegrationActor()
	codecs := resources.NewCodecRegistry()
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
	bridgeCodec, err := bridgepkg.NewBridgeInstanceResourceCodec(marketingBridgeProviderLookup)
	if err != nil {
		t.Fatalf("NewBridgeInstanceResourceCodec() error = %v", err)
	}
	for _, register := range []func(*resources.CodecRegistry) error{
		func(registry *resources.CodecRegistry) error { return resources.RegisterCodec(registry, bundleCodec) },
		func(registry *resources.CodecRegistry) error {
			return resources.RegisterCodec(registry, activationCodec)
		},
		func(registry *resources.CodecRegistry) error { return resources.RegisterCodec(registry, layoutCodec) },
		func(registry *resources.CodecRegistry) error { return resources.RegisterCodec(registry, agentCodec) },
		func(registry *resources.CodecRegistry) error { return resources.RegisterCodec(registry, soulCodec) },
		func(registry *resources.CodecRegistry) error {
			return resources.RegisterCodec(registry, heartbeatCodec)
		},
		func(registry *resources.CodecRegistry) error { return resources.RegisterCodec(registry, jobCodec) },
		func(registry *resources.CodecRegistry) error { return resources.RegisterCodec(registry, triggerCodec) },
		func(registry *resources.CodecRegistry) error { return resources.RegisterCodec(registry, bridgeCodec) },
	} {
		if err := register(codecs); err != nil {
			t.Fatalf("RegisterCodec() error = %v", err)
		}
	}

	bundleStore := mustNewTypedStore(t, kernel, bundleCodec)
	activationStore := mustNewTypedStore(t, kernel, activationCodec)
	layoutStore := mustNewTypedStore(t, kernel, layoutCodec)
	agentStore := mustNewTypedStore(t, kernel, agentCodec)
	soulStore := mustNewTypedStore(t, kernel, soulCodec)
	heartbeatStore := mustNewTypedStore(t, kernel, heartbeatCodec)
	jobStore := mustNewTypedStore(t, kernel, jobCodec)
	triggerStore := mustNewTypedStore(t, kernel, triggerCodec)
	bridgeStore := mustNewTypedStore(t, kernel, bridgeCodec)

	h := &bundleResourceIntegrationHarness{
		ctx:         ctx,
		db:          db,
		kernel:      kernel,
		codecs:      codecs,
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
		Now: func() time.Time {
			return time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewResourceStore() error = %v", err)
	}
	h.resourceStore = resourceStore
	h.service = NewService(
		resourceStore,
		staticExtensionLister{},
		func(_ context.Context, name string) (*extensionpkg.Extension, error) {
			if name != "marketing-team" {
				return nil, extensionpkg.ErrExtensionNotFound
			}
			return newMarketingExtension(), nil
		},
		WithNow(func() time.Time {
			return time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
		}),
	)
	if h.service == nil {
		t.Fatal("NewService() = nil, want service")
	}
	return h
}

func mustNewTypedStore[T any](
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

func (h *bundleResourceIntegrationHarness) putMarketingBundle(t *testing.T) {
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

func (h *bundleResourceIntegrationHarness) putJob(
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

func (h *bundleResourceIntegrationHarness) listOwnedJobs(
	t *testing.T,
	owner resources.ResourceOwner,
) []resources.Record[automationpkg.Job] {
	t.Helper()

	records, err := h.jobs.List(h.ctx, h.actor, resources.ResourceFilter{
		Kind:  automationpkg.JobResourceKind,
		Owner: &owner,
	})
	if err != nil {
		t.Fatalf("List(owned jobs) error = %v", err)
	}
	return records
}

func (h *bundleResourceIntegrationHarness) listOwnedLayouts(
	t *testing.T,
	owner resources.ResourceOwner,
) []resources.Record[windowmanager.LayoutResource] {
	t.Helper()

	records, err := h.layouts.List(h.ctx, h.actor, resources.ResourceFilter{
		Kind:  windowmanager.WindowLayoutResourceKind,
		Owner: &owner,
	})
	if err != nil {
		t.Fatalf("List(owned layouts) error = %v", err)
	}
	return records
}

func (h *bundleResourceIntegrationHarness) listOwnedAgents(
	t *testing.T,
	owner resources.ResourceOwner,
) []resources.Record[aghconfig.AgentDef] {
	t.Helper()

	records, err := h.agents.List(h.ctx, h.actor, resources.ResourceFilter{
		Kind:  aghconfig.AgentResourceKind,
		Owner: &owner,
	})
	if err != nil {
		t.Fatalf("List(owned agents) error = %v", err)
	}
	return records
}

func (h *bundleResourceIntegrationHarness) listOwnedSouls(
	t *testing.T,
	owner resources.ResourceOwner,
) []resources.Record[soul.ResourceSpec] {
	t.Helper()

	records, err := h.souls.List(h.ctx, h.actor, resources.ResourceFilter{
		Kind:  soul.ResourceKind,
		Owner: &owner,
	})
	if err != nil {
		t.Fatalf("List(owned souls) error = %v", err)
	}
	return records
}

func (h *bundleResourceIntegrationHarness) listOwnedHeartbeats(
	t *testing.T,
	owner resources.ResourceOwner,
) []resources.Record[heartbeat.ResourceSpec] {
	t.Helper()

	records, err := h.heartbeats.List(h.ctx, h.actor, resources.ResourceFilter{
		Kind:  heartbeat.ResourceKind,
		Owner: &owner,
	})
	if err != nil {
		t.Fatalf("List(owned heartbeats) error = %v", err)
	}
	return records
}

func (h *bundleResourceIntegrationHarness) listOwnedTriggers(
	t *testing.T,
	owner resources.ResourceOwner,
) []resources.Record[automationpkg.Trigger] {
	t.Helper()

	records, err := h.triggers.List(h.ctx, h.actor, resources.ResourceFilter{
		Kind:  automationpkg.TriggerResourceKind,
		Owner: &owner,
	})
	if err != nil {
		t.Fatalf("List(owned triggers) error = %v", err)
	}
	return records
}

func (h *bundleResourceIntegrationHarness) listOwnedBridges(
	t *testing.T,
	owner resources.ResourceOwner,
) []resources.Record[bridgepkg.BridgeInstanceSpec] {
	t.Helper()

	records, err := h.bridges.List(h.ctx, h.actor, resources.ResourceFilter{
		Kind:  bridgepkg.BridgeInstanceResourceKind,
		Owner: &owner,
	})
	if err != nil {
		t.Fatalf("List(owned bridges) error = %v", err)
	}
	return records
}

func bundleIntegrationActor() resources.MutationActor {
	return resources.MutationActor{
		Kind:     resources.MutationActorKindDaemon,
		ID:       "bundle-integration",
		Source:   resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "bundle-integration"},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}

func marketingBridgeProviderLookup(
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

func integrationJob(id string, name string) automationpkg.Job {
	return automationpkg.Job{
		ID:        id,
		Scope:     automationpkg.AutomationScopeGlobal,
		Name:      name,
		AgentName: "planner",
		Prompt:    "Run " + name,
		Schedule:  &automationpkg.ScheduleSpec{Mode: automationpkg.ScheduleModeEvery, Interval: "1h"},
		Enabled:   true,
		Retry:     automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(),
		Source:    automationpkg.JobSourcePackage,
		CreatedAt: time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC),
	}
}

func assertNoLegacyBundleActivationTable(t *testing.T, db *sql.DB) {
	t.Helper()

	for _, table := range []string{"bundle_activations", "bundle_activation_inventory"} {
		var count int
		if err := db.QueryRowContext(
			testutil.Context(t),
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&count); err != nil {
			t.Fatalf("query sqlite_master for %s error = %v", table, err)
		}
		if count != 0 {
			t.Fatalf("legacy table %s exists, want absent", table)
		}
	}
}
