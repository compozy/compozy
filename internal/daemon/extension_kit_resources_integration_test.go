//go:build integration

// Suite: Extension kit resource lifecycle integration
// Invariant: An enabled extension publishes one owner-attributed automation/layout kit, and update or disable converges every consumer.
// Boundary IN: daemon source sync, resource reconciliation, automation scheduling, and layout projection.
// Boundary OUT: HTTP, UDS, CLI, native-tool transport parity, and non-kit extension capabilities.

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/jonboulle/clockwork"
)

func TestExtensionKitResourcePublicationLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should publish update and remove automation and layouts as one owned kit", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		kernel, err := resources.NewKernel(db.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}
		jobCodec, err := automationpkg.NewJobResourceCodec()
		if err != nil {
			t.Fatalf("automation.NewJobResourceCodec() error = %v", err)
		}
		jobStore, err := resources.NewStore(kernel, jobCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(job) error = %v", err)
		}
		triggerCodec, err := automationpkg.NewTriggerResourceCodec()
		if err != nil {
			t.Fatalf("automation.NewTriggerResourceCodec() error = %v", err)
		}
		triggerStore, err := resources.NewStore(kernel, triggerCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(trigger) error = %v", err)
		}
		layoutCodec, err := windowmanager.NewLayoutResourceCodec()
		if err != nil {
			t.Fatalf("windowmanager.NewLayoutResourceCodec() error = %v", err)
		}
		layoutStore, err := resources.NewStore(kernel, layoutCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(layout) error = %v", err)
		}

		fakeClock := clockwork.NewFakeClockAt(time.Date(2030, 1, 1, 11, 59, 0, 0, time.UTC))
		automationManager := newExtensionKitAutomationManager(
			t,
			db,
			jobStore,
			triggerStore,
			fakeClock,
		)
		layoutCatalog := newResourceCatalog(windowmanager.CloneLayoutResource)
		driver := newExtensionKitIntegrationDriver(
			t,
			kernel,
			jobCodec,
			triggerCodec,
			layoutCodec,
			automationManager,
			layoutCatalog,
		)
		registry, runtime := installExtensionKitIntegrationFixture(t, db)
		syncer := &extensionKitSourceSyncer{
			jobs: jobStore, jobCodec: jobCodec,
			triggers: triggerStore, triggerCodec: triggerCodec,
			layouts: layoutStore, layoutCodec: layoutCodec,
			actor: extensionKitSyncActor(), registry: registry,
			runtime: func() extensionRuntime { return runtime }, logger: discardLogger(),
			trigger: func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
				_, err := driver.Trigger(ctx, kind, reason)
				return err
			},
		}

		if err := syncer.Sync(ctx); err != nil {
			t.Fatalf("Sync(installed disabled) error = %v", err)
		}
		assertExtensionKitIntegrationCounts(t, ctx, jobStore, triggerStore, layoutStore, 0, 0, 0)
		seedExtensionKitUnownedJob(t, ctx, jobStore, runtime.extension.AutomationJobs[0])

		if err := registry.Enable(runtime.extension.Info.Name); err != nil {
			t.Fatalf("registry.Enable() error = %v", err)
		}
		if err := syncer.Sync(ctx); err != nil {
			t.Fatalf("Sync(enabled) error = %v", err)
		}
		if err := driver.RunBoot(ctx); err != nil {
			t.Fatalf("RunBoot(enabled) error = %v", err)
		}
		assertExtensionKitIntegrationCounts(t, ctx, jobStore, triggerStore, layoutStore, 2, 1, 1)
		assertExtensionKitIntegrationOwners(t, ctx, jobStore, triggerStore, layoutStore, runtime.extension.Info.Name)
		projectedJobs, err := automationManager.Jobs(ctx)
		if err != nil {
			t.Fatalf("automation Jobs() error = %v", err)
		}
		projectedTriggers, err := automationManager.Triggers(ctx)
		if err != nil {
			t.Fatalf("automation Triggers() error = %v", err)
		}
		if len(projectedJobs) != 2 || len(projectedTriggers) != 1 {
			t.Fatalf("automation projection = jobs:%#v triggers:%#v", projectedJobs, projectedTriggers)
		}
		enabledJobID := "extension/kit-roundtrip/automation.job/enabled"
		disabledJobID := "extension/kit-roundtrip/automation.job/disabled"
		triggerID := "extension/kit-roundtrip/automation.trigger/session-stop"
		status, err := automationManager.Status(ctx)
		if err != nil {
			t.Fatalf("automation Status() error = %v", err)
		}
		if len(status.ScheduledJobs) != 1 || status.ScheduledJobs[0].JobID != enabledJobID {
			t.Fatalf(
				"scheduled jobs = %#v, want enabled %q only and disabled %q unregistered",
				status.ScheduledJobs,
				enabledJobID,
				disabledJobID,
			)
		}
		if _, err := automationManager.UpdateJob(ctx, projectedJobs[0]); !errors.Is(
			err,
			automationpkg.ErrDefinitionReadOnly,
		) {
			t.Fatalf("UpdateJob(package definition) error = %v, want read-only", err)
		}
		if err := fakeClock.BlockUntilContext(ctx, 1); err != nil {
			t.Fatalf("fake clock wait for package schedule error = %v", err)
		}
		fakeClock.Advance(time.Minute)
		waitForExtensionKitAutomationRun(t, ctx, db, store.DefaultProfileID, enabledJobID)
		if _, err := automationManager.SetJobEnabled(ctx, enabledJobID, false); err != nil {
			t.Fatalf("SetJobEnabled(package overlay) error = %v", err)
		}
		if _, err := automationManager.SetTriggerEnabled(ctx, triggerID, false); err != nil {
			t.Fatalf("SetTriggerEnabled(package overlay) error = %v", err)
		}
		jobOverlay, err := db.GetJobEnabledOverlay(ctx, enabledJobID)
		if err != nil || jobOverlay.EnabledOverride {
			t.Fatalf("job enabled overlay = %#v, error = %v", jobOverlay, err)
		}
		triggerOverlay, err := db.GetTriggerEnabledOverlay(ctx, triggerID)
		if err != nil || triggerOverlay.EnabledOverride {
			t.Fatalf("trigger enabled overlay = %#v, error = %v", triggerOverlay, err)
		}
		layouts, err := newWindowManagerLayoutRegistry(layoutCatalog).List(ctx, "workspace-a")
		if err != nil {
			t.Fatalf("layout registry List() error = %v", err)
		}
		if len(layouts) != 1 || layouts[0].ID != "extension/kit-roundtrip/window_layout/focus" ||
			layouts[0].Document.WorkspaceID != "workspace-a" {
			t.Fatalf("layout registry List() = %#v", layouts)
		}
		layoutService, err := windowmanager.NewService(
			windowmanager.NewMemoryRepository(),
			windowmanager.NewMemoryWorkspaceResolver("workspace-a"),
			newWindowManagerLayoutRegistry(layoutCatalog),
			windowmanager.DefaultConfig(),
		)
		if err != nil {
			t.Fatalf("windowmanager.NewService() error = %v", err)
		}
		t.Cleanup(func() {
			if err := layoutService.Close(); err != nil {
				t.Errorf("window manager Close() error = %v", err)
			}
		})
		arranged, err := layoutService.Execute(ctx, windowmanager.CommandRequest{
			WorkspaceID: "workspace-a",
			Payload: windowmanager.ArrangeLayoutCommand{
				ResourceID: "extension/kit-roundtrip/window_layout/focus",
			},
		})
		if err != nil || !arranged.Applied {
			t.Fatalf("Execute(extension layout) = %#v, error = %v", arranged, err)
		}

		runtime.extension.AutomationJobs = []automationpkg.Job{
			extensionKitIntegrationJob("replacement", true),
		}
		runtime.extension.AutomationTriggers = nil
		runtime.extension.Layouts = nil
		if err := syncer.Sync(ctx); err != nil {
			t.Fatalf("Sync(updated kit) error = %v", err)
		}
		if err := driver.RunBoot(ctx); err != nil {
			t.Fatalf("RunBoot(updated kit) error = %v", err)
		}
		assertExtensionKitIntegrationCounts(t, ctx, jobStore, triggerStore, layoutStore, 1, 0, 0)
		projectedJobs, err = automationManager.Jobs(ctx)
		if err != nil {
			t.Fatalf("automation Jobs(updated) error = %v", err)
		}
		projectedTriggers, err = automationManager.Triggers(ctx)
		if err != nil {
			t.Fatalf("automation Triggers(updated) error = %v", err)
		}
		if len(projectedJobs) != 1 || projectedJobs[0].Name != "kit-roundtrip/replacement" ||
			len(projectedTriggers) != 0 {
			t.Fatalf("updated automation projection = jobs:%#v triggers:%#v", projectedJobs, projectedTriggers)
		}
		if _, err := db.GetJobEnabledOverlay(ctx, enabledJobID); !errors.Is(err, automationpkg.ErrJobOverlayNotFound) {
			t.Fatalf("GetJobEnabledOverlay(retired job) error = %v, want overlay GC", err)
		}
		if _, err := db.GetTriggerEnabledOverlay(ctx, triggerID); !errors.Is(
			err,
			automationpkg.ErrTriggerOverlayNotFound,
		) {
			t.Fatalf("GetTriggerEnabledOverlay(retired trigger) error = %v, want overlay GC", err)
		}

		if err := registry.Disable(runtime.extension.Info.Name); err != nil {
			t.Fatalf("registry.Disable() error = %v", err)
		}
		if err := syncer.Sync(ctx); err != nil {
			t.Fatalf("Sync(disabled) error = %v", err)
		}
		if err := driver.RunBoot(ctx); err != nil {
			t.Fatalf("RunBoot(disabled) error = %v", err)
		}
		assertExtensionKitIntegrationCounts(t, ctx, jobStore, triggerStore, layoutStore, 0, 0, 0)
		projectedJobs, err = automationManager.Jobs(ctx)
		if err != nil {
			t.Fatalf("automation Jobs(disabled) error = %v", err)
		}
		projectedTriggers, err = automationManager.Triggers(ctx)
		if err != nil {
			t.Fatalf("automation Triggers(disabled) error = %v", err)
		}
		if len(projectedJobs) != 0 || len(projectedTriggers) != 0 || len(layoutCatalog.Snapshot()) != 0 {
			t.Fatalf(
				"disabled projection = jobs:%#v triggers:%#v layouts:%#v",
				projectedJobs,
				projectedTriggers,
				layoutCatalog.Snapshot(),
			)
		}
		_, err = layoutService.Execute(ctx, windowmanager.CommandRequest{
			WorkspaceID:      "workspace-a",
			ExpectedRevision: arranged.Snapshot.Revision,
			Payload: windowmanager.ArrangeLayoutCommand{
				ResourceID: "extension/kit-roundtrip/window_layout/focus",
			},
		})
		if !errors.Is(err, windowmanager.ErrLayoutResourceNotFound) {
			t.Fatalf("Execute(disabled extension layout) error = %v, want resource not found", err)
		}
	})
}

func newExtensionKitAutomationManager(
	t *testing.T,
	db *globaldb.GlobalDB,
	jobStore resources.Store[automationpkg.Job],
	triggerStore resources.Store[automationpkg.Trigger],
	fakeClock *clockwork.FakeClock,
) *automationpkg.Manager {
	t.Helper()
	workspaceRoot := t.TempDir()
	resolver := &daemonExtensionWorkspaceResolverStub{resolved: workspacepkg.ResolvedWorkspace{
		ID: "workspace-a", Name: "kit-automation", RootDir: workspaceRoot,
		WorkspaceID: "workspace-a",
	}}
	manager, err := automationpkg.New(
		automationpkg.WithStore(db),
		automationpkg.WithSessions(newNativeAutomationSessionManager()),
		automationpkg.WithWorkspaceResolver(resolver),
		automationpkg.WithConfig(compozyconfig.AutomationConfig{
			Enabled:           true,
			Timezone:          automationpkg.DefaultTimezone,
			MaxConcurrentJobs: automationpkg.DefaultMaxConcurrentJobs,
			DefaultFireLimit:  automationpkg.DefaultFireLimitConfig(),
		}),
		automationpkg.WithLogger(discardLogger()),

		automationpkg.WithResourceDefinitions(
			jobStore,
			triggerStore,
			resourceReconcileActor(),
			nil,
		),
		automationpkg.WithManagerNow(fakeClock.Now),
		automationpkg.WithDispatcherOptions(automationpkg.WithDispatcherNow(fakeClock.Now)),
		automationpkg.WithSchedulerOptions(automationpkg.WithSchedulerClock(fakeClock)),
	)
	if err != nil {
		t.Fatalf("automation.New() error = %v", err)
	}
	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("automation manager Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("automation manager Shutdown() error = %v", err)
		}
	})
	return manager
}

func waitForExtensionKitAutomationRun(
	t *testing.T,
	ctx context.Context,
	db *globaldb.GlobalDB,
	profileID string,
	jobID string,
) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var lastRuns []automationpkg.Run
	for time.Now().Before(deadline) {
		runs, err := db.ListRuns(ctx, automationpkg.RunQuery{
			ReadScope: store.ReadScope{ProfileID: profileID},
			JobID:     jobID,
		})
		if err != nil {
			t.Fatalf("ListRuns(%q) error = %v", jobID, err)
		}
		if len(runs) == 1 && runs[0].Status == automationpkg.RunCompleted {
			return
		}
		lastRuns = runs
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("automation job %q did not complete under the fake clock; runs=%#v", jobID, lastRuns)
}

type extensionKitProjectionPlan struct {
	kind     resources.ResourceKind
	revision int64
	jobs     []resources.Record[automationpkg.Job]
	triggers []resources.Record[automationpkg.Trigger]
}

func (p extensionKitProjectionPlan) Kind() resources.ResourceKind { return p.kind }
func (p extensionKitProjectionPlan) Revision() int64              { return p.revision }
func (p extensionKitProjectionPlan) OperationCount() int          { return len(p.jobs) + len(p.triggers) }

type extensionKitProjectionTarget struct {
	jobs     []resources.Record[automationpkg.Job]
	triggers []resources.Record[automationpkg.Trigger]
}

func (t *extensionKitProjectionTarget) BuildJobResourceState(
	_ context.Context,
	records []resources.Record[automationpkg.Job],
) (resources.ProjectionPlan, error) {
	return extensionKitProjectionPlan{
		kind: automationpkg.JobResourceKind, revision: extensionKitRevision(records),
		jobs: append([]resources.Record[automationpkg.Job](nil), records...),
	}, nil
}

func (t *extensionKitProjectionTarget) ApplyJobResourceState(
	_ context.Context,
	plan resources.ProjectionPlan,
) error {
	t.jobs = append([]resources.Record[automationpkg.Job](nil), plan.(extensionKitProjectionPlan).jobs...)
	return nil
}

func (t *extensionKitProjectionTarget) BuildTriggerResourceState(
	_ context.Context,
	records []resources.Record[automationpkg.Trigger],
) (resources.ProjectionPlan, error) {
	return extensionKitProjectionPlan{
		kind: automationpkg.TriggerResourceKind, revision: extensionKitRevision(records),
		triggers: append([]resources.Record[automationpkg.Trigger](nil), records...),
	}, nil
}

func (t *extensionKitProjectionTarget) ApplyTriggerResourceState(
	_ context.Context,
	plan resources.ProjectionPlan,
) error {
	t.triggers = append([]resources.Record[automationpkg.Trigger](nil), plan.(extensionKitProjectionPlan).triggers...)
	return nil
}

func extensionKitRevision[T any](records []resources.Record[T]) int64 {
	var revision int64
	for _, record := range records {
		if record.Version > revision {
			revision = record.Version
		}
	}
	return revision
}

func newExtensionKitIntegrationDriver(
	t *testing.T,
	kernel resources.RawStore,
	jobCodec resources.KindCodec[automationpkg.Job],
	triggerCodec resources.KindCodec[automationpkg.Trigger],
	layoutCodec resources.KindCodec[windowmanager.LayoutResource],
	target automationResourceProjectorTarget,
	layoutCatalog *resourceCatalog[windowmanager.LayoutResource],
) resources.ReconcileDriver {
	t.Helper()
	jobRegistration, err := resources.NewTypedProjectorRegistration(jobCodec, newAutomationJobProjector(target))
	if err != nil {
		t.Fatalf("NewTypedProjectorRegistration(job) error = %v", err)
	}
	triggerRegistration, err := resources.NewTypedProjectorRegistration(
		triggerCodec,
		newAutomationTriggerProjector(target),
	)
	if err != nil {
		t.Fatalf("NewTypedProjectorRegistration(trigger) error = %v", err)
	}
	layoutRegistration, err := resources.NewTypedProjectorRegistration(
		layoutCodec,
		newWindowLayoutProjector(layoutCatalog),
	)
	if err != nil {
		t.Fatalf("NewTypedProjectorRegistration(layout) error = %v", err)
	}
	driver, err := resources.NewReconcileDriver(
		kernel,
		resourceReconcileActor(),
		[]resources.ProjectorRegistration{jobRegistration, triggerRegistration, layoutRegistration},
		resources.WithReconcileLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("resources.NewReconcileDriver() error = %v", err)
	}
	t.Cleanup(func() {
		if err := driver.Close(testutil.Context(t)); err != nil {
			t.Errorf("driver.Close() error = %v", err)
		}
	})
	return driver
}

type extensionKitIntegrationRuntime struct {
	extension *extensionpkg.Extension
}

func (r *extensionKitIntegrationRuntime) Start(context.Context) error  { return nil }
func (r *extensionKitIntegrationRuntime) Stop(context.Context) error   { return nil }
func (r *extensionKitIntegrationRuntime) Reload(context.Context) error { return nil }
func (r *extensionKitIntegrationRuntime) HookDeclarations(context.Context) ([]hookspkg.HookDecl, error) {
	return nil, nil
}
func (r *extensionKitIntegrationRuntime) Get(name string) (*extensionpkg.Extension, error) {
	if r.extension == nil || r.extension.Info.Name != name {
		return nil, extensionpkg.ErrExtensionNotFound
	}
	return r.extension, nil
}

func (r *extensionKitIntegrationRuntime) InspectPackageResources(
	_ context.Context,
	name string,
) (*extensionpkg.Extension, error) {
	return r.Get(name)
}

func installExtensionKitIntegrationFixture(
	t *testing.T,
	db extensionDBSource,
) (*extensionpkg.Registry, *extensionKitIntegrationRuntime) {
	t.Helper()
	dir := t.TempDir()
	manifestText := `[extension]
name = "kit-roundtrip"
version = "1.0.0"
min_compozy_version = "0.5.0"

[capabilities]
provides = []

[permissions]
requires = []
`
	if err := os.WriteFile(filepath.Join(dir, "extension.toml"), []byte(manifestText), 0o644); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}
	manifest, err := extensionpkg.LoadManifest(dir)
	if err != nil {
		t.Fatalf("extension.LoadManifest() error = %v", err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("extension.ComputeDirectoryChecksum() error = %v", err)
	}
	registry := extensionpkg.NewRegistry(db.DB())
	if err := registry.Install(manifest, dir, checksum, extensionpkg.WithInstallEnabled(false)); err != nil {
		t.Fatalf("registry.Install() error = %v", err)
	}
	info, err := registry.Get(manifest.Name)
	if err != nil {
		t.Fatalf("registry.Get() error = %v", err)
	}
	extension := &extensionpkg.Extension{
		Info: *info, Manifest: manifest, RootDir: dir,
		AutomationJobs: []automationpkg.Job{
			extensionKitIntegrationJob("enabled", true),
			extensionKitIntegrationJob("disabled", false),
		},
		AutomationTriggers: []automationpkg.Trigger{extensionKitIntegrationTrigger("session-stop")},
		Layouts:            []windowmanager.LayoutResource{extensionKitIntegrationLayout("focus")},
		Status: extensionpkg.ExtensionStatus{
			Name: info.Name, Version: info.Version, Source: info.Source, Registered: true,
		},
	}
	return registry, &extensionKitIntegrationRuntime{extension: extension}
}

func extensionKitIntegrationJob(name string, enabled bool) automationpkg.Job {
	return automationpkg.Job{
		ID:    "extension/kit-roundtrip/automation.job/" + name,
		Scope: automationpkg.AutomationScopeGlobal, Name: "kit-roundtrip/" + name,
		AgentName: "writer", Prompt: "Review repository",
		Schedule: &automationpkg.ScheduleSpec{
			Mode: automationpkg.ScheduleModeAt,
			Time: time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
		Enabled: enabled, Retry: automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(), Source: automationpkg.JobSourcePackage,
	}
}

func extensionKitIntegrationTrigger(name string) automationpkg.Trigger {
	return automationpkg.Trigger{
		ID:    "extension/kit-roundtrip/automation.trigger/" + name,
		Scope: automationpkg.AutomationScopeGlobal, Name: "kit-roundtrip/" + name,
		AgentName: "writer", Prompt: "Review stopped session", Event: "session.stopped", Enabled: true,
		Retry: automationpkg.DefaultRetryConfig(), FireLimit: automationpkg.DefaultFireLimitConfig(),
		Source: automationpkg.JobSourcePackage,
	}
}

func extensionKitIntegrationLayout(id string) windowmanager.LayoutResource {
	return windowmanager.LayoutResource{
		Version: windowmanager.LayoutResourceVersion, ID: id, DisplayName: "Focus",
		AspectVariant: windowmanager.LayoutAspectAny, OverflowPolicy: windowmanager.LayoutOverflowStack,
		Document: daemonTestLayoutDocument("Main"),
	}
}

func seedExtensionKitUnownedJob(
	t *testing.T,
	ctx context.Context,
	store resources.Store[automationpkg.Job],
	job automationpkg.Job,
) {
	t.Helper()
	if _, err := store.Put(ctx, extensionKitSyncActor(), resources.Draft[automationpkg.Job]{
		ID: job.ID, Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser}, Spec: job,
	}); err != nil {
		t.Fatalf("seed unowned job error = %v", err)
	}
}

func assertExtensionKitIntegrationCounts(
	t *testing.T,
	ctx context.Context,
	jobs resources.Store[automationpkg.Job],
	triggers resources.Store[automationpkg.Trigger],
	layouts resources.Store[windowmanager.LayoutResource],
	wantJobs int,
	wantTriggers int,
	wantLayouts int,
) {
	t.Helper()
	actor := extensionKitSyncActor()
	jobRecords, err := jobs.List(ctx, actor, resources.ResourceFilter{})
	if err != nil {
		t.Fatalf("jobs.List() error = %v", err)
	}
	triggerRecords, err := triggers.List(ctx, actor, resources.ResourceFilter{})
	if err != nil {
		t.Fatalf("triggers.List() error = %v", err)
	}
	layoutRecords, err := layouts.List(ctx, actor, resources.ResourceFilter{})
	if err != nil {
		t.Fatalf("layouts.List() error = %v", err)
	}
	if len(jobRecords) != wantJobs || len(triggerRecords) != wantTriggers || len(layoutRecords) != wantLayouts {
		t.Fatalf(
			"resource counts = jobs:%d triggers:%d layouts:%d, want %d/%d/%d",
			len(jobRecords), len(triggerRecords), len(layoutRecords), wantJobs, wantTriggers, wantLayouts,
		)
	}
}

func assertExtensionKitIntegrationOwners(
	t *testing.T,
	ctx context.Context,
	jobs resources.Store[automationpkg.Job],
	triggers resources.Store[automationpkg.Trigger],
	layouts resources.Store[windowmanager.LayoutResource],
	extensionName string,
) {
	t.Helper()
	want := extensionOwner(extensionName).Normalize()
	actor := extensionKitSyncActor()
	jobRecords, err := jobs.List(ctx, actor, resources.ResourceFilter{})
	if err != nil {
		t.Fatalf("jobs.List() error = %v", err)
	}
	triggerRecords, err := triggers.List(ctx, actor, resources.ResourceFilter{})
	if err != nil {
		t.Fatalf("triggers.List() error = %v", err)
	}
	layoutRecords, err := layouts.List(ctx, actor, resources.ResourceFilter{})
	if err != nil {
		t.Fatalf("layouts.List() error = %v", err)
	}
	for _, owner := range append(
		[]resources.ResourceOwner{jobRecords[0].Owner, jobRecords[1].Owner, triggerRecords[0].Owner},
		layoutRecords[0].Owner,
	) {
		if owner.Normalize() != want {
			t.Fatalf("resource owner = %#v, want %#v", owner, want)
		}
	}
}

// Invariant: boot upgrades saved layouts once, retains failures, and admits legacy extension files.
func TestLegacyWindowLayoutResources(t *testing.T) {
	t.Run(
		"Should rewrite stored v5 profiles and boot with an unconvertible profile intact (IT-013)",
		func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db := openDaemonTestGlobalDB(t)
			kernel, err := resources.NewKernel(db.DB())
			if err != nil {
				t.Fatal(err)
			}
			codecs := resources.NewCodecRegistry()
			if err := registerDaemonResourceCodecs(codecs); err != nil {
				t.Fatal(err)
			}
			state := &bootState{resourceKernel: kernel, resourceCodecs: codecs, logger: discardLogger()}
			actor := resources.MutationActor{
				Kind: resources.MutationActorKindOperator, ID: "operator",
				Source:   resources.ResourceSource{Kind: "dynamic", ID: "operator"},
				MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			}
			original, err := kernel.PutRaw(ctx, actor, resources.RawDraft{
				Kind: windowmanager.WindowLayoutResourceKind, ID: "legacy", Scope: actor.MaxScope,
				SpecJSON: []byte(legacyWindowLayoutResourceJSON),
			})
			if err != nil {
				t.Fatal(err)
			}
			brokenJSON := bytes.ReplaceAll(
				[]byte(legacyWindowLayoutResourceJSON),
				[]byte(`"id":"legacy"`),
				[]byte(`"id":"broken"`),
			)
			brokenJSON = bytes.ReplaceAll(brokenJSON, []byte(`"floating":["one","two"]`), []byte(`"floating":["one"]`))
			broken, err := kernel.PutRaw(ctx, actor, resources.RawDraft{
				Kind: windowmanager.WindowLayoutResourceKind, ID: "broken", Scope: actor.MaxScope, SpecJSON: brokenJSON,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := reconcileStoredWindowLayouts(ctx, state); err != nil {
				t.Fatal(err)
			}
			stored, err := kernel.GetRaw(ctx, actor, windowmanager.WindowLayoutResourceKind, "legacy")
			if err != nil {
				t.Fatal(err)
			}
			var header struct {
				Version  uint32 `json:"version"`
				Document struct {
					Version uint32 `json:"version"`
				} `json:"document"`
			}
			if err := json.Unmarshal(stored.SpecJSON, &header); err != nil {
				t.Fatal(err)
			}
			if header.Version != 1 || header.Document.Version != 6 || stored.Version != original.Version+1 ||
				stored.Source != original.Source || stored.Owner != original.Owner || stored.Scope != original.Scope {
				t.Fatalf("rewritten record = %+v; header = %+v", stored, header)
			}
			if err := reconcileStoredWindowLayouts(ctx, state); err != nil {
				t.Fatal(err)
			}
			again, err := kernel.GetRaw(ctx, actor, windowmanager.WindowLayoutResourceKind, "legacy")
			if err != nil || !reflect.DeepEqual(again, stored) {
				t.Fatalf("second rewrite = %+v, error = %v; want %+v", again, err, stored)
			}
			retained, err := kernel.GetRaw(ctx, actor, windowmanager.WindowLayoutResourceKind, "broken")
			if err != nil || !reflect.DeepEqual(retained, broken) {
				t.Fatalf("unconvertible record = %+v, error = %v; want original %+v", retained, err, broken)
			}

			catalog := newResourceCatalog(windowmanager.CloneLayoutResource)
			daemon := &Daemon{}
			daemon.applyResourceReconcileDriverFactoryDefault()
			driver, err := daemon.newResourceReconcile(ctx, resourceReconcileDriverDeps{
				ResourceStore: kernel, CodecRegistry: codecs, WindowLayoutCatalog: catalog, Logger: discardLogger(),
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := driver.Close(context.Background()); err != nil {
					t.Errorf("Close() error = %v", err)
				}
			})
			if err := driver.RunBoot(ctx); err != nil {
				t.Fatalf("RunBoot() with an unconvertible profile: %v", err)
			}
			registry := newWindowManagerLayoutRegistry(catalog)
			visible, err := registry.List(ctx, "workspace-a")
			if err != nil || len(visible) != 1 || visible[0].ID != "legacy" ||
				!reflect.DeepEqual(visible[0].ParticipantSlots, []windowmanager.WindowID{"one", "two", "virtual"}) ||
				visible[0].AspectVariant != windowmanager.LayoutAspectLandscape ||
				visible[0].OverflowPolicy != windowmanager.LayoutOverflowReject {
				t.Fatalf("List() after boot = %+v, error = %v", visible, err)
			}
			service, err := windowmanager.NewService(windowmanager.NewMemoryRepository(),
				windowmanager.NewMemoryWorkspaceResolver("workspace-a"), registry, windowmanager.DefaultConfig())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := service.Close(); err != nil {
					t.Errorf("Close() error = %v", err)
				}
			})
			applied, err := service.Execute(ctx, windowmanager.CommandRequest{
				WorkspaceID: "workspace-a", Payload: windowmanager.ArrangeLayoutCommand{ResourceID: "legacy"},
			})
			if err != nil || !applied.Applied || applied.Snapshot.Version != 6 ||
				len(applied.Snapshot.Desktops[0].Tabs) != 2 || len(applied.Snapshot.Windows) != 2 ||
				applied.Snapshot.Windows["one"].Route.Pathname != "/sessions/one" ||
				len(applied.Snapshot.Windows["one"].NavStack) != 1 ||
				applied.Snapshot.Windows["two"].Route.Pathname != "/terminal/two" ||
				applied.Snapshot.Overrides.FocusWrap == nil || !*applied.Snapshot.Overrides.FocusWrap {
				t.Fatalf("apply saved profile = %+v, error = %v", applied, err)
			}
		},
	)

	// Invariant: boot persists v4 app upgrades before registered-app reconciliation can prune windows.
	t.Run("Should rewrite retired apps in a stored v4 profile without losing windows (IT-013)", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openDaemonTestGlobalDB(t)
		kernel, err := resources.NewKernel(db.DB())
		if err != nil {
			t.Fatal(err)
		}
		codecs := resources.NewCodecRegistry()
		if err := registerDaemonResourceCodecs(codecs); err != nil {
			t.Fatal(err)
		}
		state := &bootState{resourceKernel: kernel, resourceCodecs: codecs, logger: discardLogger()}
		actor := resources.MutationActor{
			Kind: resources.MutationActorKindOperator, ID: "operator",
			Source:   resources.ResourceSource{Kind: "dynamic", ID: "operator"},
			MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
		}
		raw := []byte(legacyWindowLayoutResourceJSON)
		for _, replacement := range [][2]string{
			{`"version":5`, `"version":4`},
			{`"app":"session"`, `"app":"jobs"`},
			{`"app":"terminal"`, `"app":"triggers"`},
			{`/sessions/one`, `/jobs/one`},
			{`/sessions`, `/jobs`},
			{`/terminal/two`, `/triggers/two`},
		} {
			raw = bytes.ReplaceAll(raw, []byte(replacement[0]), []byte(replacement[1]))
		}
		if _, err := kernel.PutRaw(ctx, actor, resources.RawDraft{
			Kind: windowmanager.WindowLayoutResourceKind, ID: "legacy", Scope: actor.MaxScope, SpecJSON: raw,
		}); err != nil {
			t.Fatal(err)
		}
		if err := reconcileStoredWindowLayouts(ctx, state); err != nil {
			t.Fatal(err)
		}
		stored, err := kernel.GetRaw(ctx, actor, windowmanager.WindowLayoutResourceKind, "legacy")
		if err != nil {
			t.Fatal(err)
		}
		// Inspect stored window fields directly so a second document decode cannot mask a missed rewrite.
		var persisted struct {
			Document struct {
				Version uint32                          `json:"version"`
				Windows map[string]windowmanager.Window `json:"windows"`
			} `json:"document"`
			ParticipantSlots []windowmanager.WindowID `json:"participant_slots"`
		}
		if err := json.Unmarshal(stored.SpecJSON, &persisted); err != nil {
			t.Fatal(err)
		}
		job, trigger := persisted.Document.Windows["one"], persisted.Document.Windows["two"]
		if persisted.Document.Version != 6 || len(persisted.Document.Windows) != 2 ||
			job.App != "automations" || job.Route.Pathname != "/automations/jobs/one" ||
			trigger.App != "automations" || trigger.Route.Pathname != "/automations/triggers/two" ||
			len(job.NavStack) != 1 ||
			!reflect.DeepEqual(persisted.ParticipantSlots, []windowmanager.WindowID{"one", "two", "virtual"}) {
			t.Fatalf("persisted v4 upgrade = %+v", persisted)
		}
		if job.NavStack[0].Pathname != "/automations" || string(job.NavStack[0].Search["start"]) != `"schedule"` {
			t.Fatalf("persisted navigation stack = %+v", job.NavStack)
		}
	})

	t.Run("Should load a static v5 extension layout as v6 tabs (IT-013)", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(root, "legacy.json"),
			[]byte(legacyWindowLayoutResourceJSON),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		layouts, err := extensionpkg.LoadLayoutResources(t.Context(), root, []string{"legacy.json"})
		if err != nil || len(layouts) != 1 || layouts[0].Version != 1 || layouts[0].Document.Version != 6 ||
			len(layouts[0].Document.Desktops[0].Tabs) != 2 || len(layouts[0].Document.Windows) != 2 {
			t.Fatalf("LoadLayoutResources() = %+v, error = %v", layouts, err)
		}
	})
}

const legacyWindowLayoutResourceJSON = `{
	"version":1,"id":"legacy","display_name":"Saved","aspect_variant":"landscape",
	"participant_slots":["one","two","virtual"],"overflow_policy":"reject",
	"document":{
		"version":5,"workspace_id":"",
		"desktops":[{"id":"saved","name":"Saved","order":0,"groups":[],"floating":["one","two"],"floating_stacks":[]}],
		"windows":{
			"one":{"id":"one","app":"session","desktop_id":"saved","placement":"floating","route":{"pathname":"/sessions/one","search":{}},"nav_stack":[{"pathname":"/sessions","search":{}}],"pinned":true,"floating_rect":{"x":0.1,"y":0.1,"width":0.4,"height":0.6}},
			"two":{"id":"two","app":"terminal","desktop_id":"saved","placement":"floating","route":{"pathname":"/terminal/two","search":{}},"nav_stack":[],"minimized":true,"floating_rect":{"x":0.5,"y":0.1,"width":0.4,"height":0.6}}
		},"overrides":{"focus_wrap":true}
	}
}`
