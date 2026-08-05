package daemon

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/resources"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/windowmanager"
)

func TestExtensionInventoryAndEnablePreview(t *testing.T) {
	t.Parallel()

	t.Run("Should report shipped resources as not live while disabled", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		ext := inventoryTestExtension(false)
		codecs := inventoryTestResourceCodecs(t)
		resourceID := ext.AutomationJobs[0].ID
		specJSON := inventoryTestJobSpec(t, codecs, ext.AutomationJobs[0])
		service := &daemonExtensionService{
			registry: extensionpkg.NewRegistry(db.DB()),
			runtime:  &inventoryExtensionRuntime{ext: ext},
			resourceStore: inventoryRawStore{records: []resources.RawRecord{{
				Kind: automationpkg.JobResourceKind, ID: resourceID, SpecJSON: specJSON,
				Owner: *extensionOwner("kit"),
			}}},
			logger:         discardLogger(),
			resourceCodecs: codecs,
		}

		inventory, err := service.Inventory(t.Context(), "kit")
		if err != nil {
			t.Fatalf("Inventory() error = %v", err)
		}
		if inventory.Enabled {
			t.Fatal("Inventory().Enabled = true, want false")
		}
		if len(inventory.Items) != 3 {
			t.Fatalf("Inventory().Items = %#v, want three shipped automation items", inventory.Items)
		}
		for _, item := range inventory.Items {
			if item.Live {
				t.Fatalf("disabled inventory item = %#v, want Live=false", item)
			}
		}
	})

	t.Run("Should use one desired automation plan for preview and enable", func(t *testing.T) {
		// This ordered lifecycle assertion owns one mutable registry instance.
		db := openDaemonTestGlobalDB(t)
		installDaemonTestExtension(t, db, "kit", daemonTestExtensionOptions{}, false)
		ext := inventoryTestExtension(false)
		runtime := &inventoryExtensionRuntime{ext: ext}
		automation := inventoryAutomationPreviewer{
			jobs:     slices.Clone(ext.AutomationJobs),
			triggers: slices.Clone(ext.AutomationTriggers),
		}
		service, ok := newDaemonExtensionService(daemonExtensionServiceDeps{
			Registry:  extensionpkg.NewRegistry(db.DB()),
			Runtime:   runtime,
			HomePaths: testHomePaths(t),
			Logger:    discardLogger(),
			Now:       func() time.Time { return time.Date(2026, 8, 2, 14, 0, 0, 0, time.UTC) },
		},
			withDaemonExtensionAutomation(automation),
			withDaemonExtensionResources(nil, resources.MutationActor{}, inventoryTestResourceCodecs(t)),
		).(*daemonExtensionService)
		if !ok {
			t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
		}
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "cli")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}

		preview, err := service.Preview(t.Context(), "kit")
		if err != nil {
			t.Fatalf("Preview() error = %v", err)
		}
		wantStarting := []string{"kit/alpha", "kit/beta"}
		if !slices.Equal(preview.AutomationStarting, wantStarting) {
			t.Fatalf("Preview().AutomationStarting = %#v, want %#v", preview.AutomationStarting, wantStarting)
		}
		result, err := service.Enable(t.Context(), "kit", contract.EnableExtensionRequest{}, actor)
		if err != nil {
			t.Fatalf("Enable() error = %v", err)
		}
		if !slices.Equal(result.AutomationStarted, preview.AutomationStarting) {
			t.Fatalf(
				"Enable().AutomationStarted = %#v, want preview plan %#v",
				result.AutomationStarted,
				preview.AutomationStarting,
			)
		}
	})

	t.Run("Should preview without mutating runtime storage vault or events", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		installDaemonTestExtension(t, db, "kit", daemonTestExtensionOptions{}, false)
		registry := extensionpkg.NewRegistry(db.DB())
		before, err := registry.Get("kit")
		if err != nil {
			t.Fatalf("registry.Get(before preview) error = %v", err)
		}
		runtime := &inventoryExtensionRuntime{ext: inventoryTestExtension(false)}
		mutationCalls := 0
		writer := &extensionEventRecorder{}
		secretVault := newExtensionSecretVaultFake()
		service := &daemonExtensionService{
			registry: registry, runtime: runtime,
			resourceStore:  &inventoryMutationSpyStore{mutationCalls: &mutationCalls},
			secretVault:    secretVault,
			eventWriter:    writer,
			logger:         discardLogger(),
			now:            time.Now,
			automation:     inventoryAutomationPreviewer{},
			resourceCodecs: inventoryTestResourceCodecs(t),
		}

		if _, err := service.Preview(t.Context(), "kit"); err != nil {
			t.Fatalf("Preview() error = %v", err)
		}
		after, err := registry.Get("kit")
		if err != nil {
			t.Fatalf("registry.Get(after preview) error = %v", err)
		}
		if !reflect.DeepEqual(after, before) {
			t.Fatalf("registry state changed during preview: before=%#v after=%#v", before, after)
		}
		if runtime.reloadCalls != 0 || mutationCalls != 0 || len(secretVault.operations()) != 0 ||
			len(writer.snapshot()) != 0 {
			t.Fatalf(
				"preview mutations = reload:%d resources:%d vault:%#v events:%#v",
				runtime.reloadCalls,
				mutationCalls,
				secretVault.operations(),
				writer.snapshot(),
			)
		}
	})

	t.Run("Should refuse the exact authored and reserved agent conflicts shown by preview", func(t *testing.T) {
		// This ordered lifecycle assertion owns one mutable registry instance.
		db := openDaemonTestGlobalDB(t)
		installDaemonTestExtension(t, db, "kit", daemonTestExtensionOptions{}, false)
		builtinNames := compozyconfig.BuiltinAgentNames()
		if len(builtinNames) == 0 {
			t.Fatal("BuiltinAgentNames() = empty")
		}
		ext := inventoryTestExtension(false)
		ext.StaticAgents = []extensionpkg.StaticAgent{
			{Agent: compozyconfig.AgentDef{Name: "writer", Prompt: "Write clearly"}},
			{Agent: compozyconfig.AgentDef{Name: builtinNames[0], Prompt: "Assist clearly"}},
		}
		codecs := inventoryTestResourceCodecs(t)
		writerCodec, err := resources.ResolveCodec[compozyconfig.AgentDef](
			codecs,
			compozyconfig.AgentResourceKind,
		)
		if err != nil {
			t.Fatalf("ResolveCodec(agent) error = %v", err)
		}
		_, writerSpec, err := validateAndEncodeResource(
			t.Context(),
			writerCodec,
			resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			compozyconfig.AgentDef{Name: "writer", Prompt: "Existing writer"},
		)
		if err != nil {
			t.Fatalf("validateAndEncodeResource(writer) error = %v", err)
		}
		runtime := &inventoryExtensionRuntime{ext: ext}
		store := inventoryRawStore{records: []resources.RawRecord{{
			Kind:     compozyconfig.AgentResourceKind,
			ID:       "authored/agent/writer",
			Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			SpecJSON: writerSpec,
			Owner:    resources.ResourceOwner{Kind: resources.ResourceOwnerKind("operator"), ID: "operator"},
		}}}
		service, ok := newDaemonExtensionService(daemonExtensionServiceDeps{
			Registry:  extensionpkg.NewRegistry(db.DB()),
			Runtime:   runtime,
			HomePaths: testHomePaths(t),
			Logger:    discardLogger(),
			Now:       time.Now,
		},
			withDaemonExtensionAutomation(inventoryAutomationPreviewer{}),
			withDaemonExtensionResources(store, resources.MutationActor{}, codecs),
		).(*daemonExtensionService)
		if !ok {
			t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
		}
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "cli")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		preview, err := service.Preview(t.Context(), "kit")
		if err != nil {
			t.Fatalf("Preview() error = %v", err)
		}
		wantConflicts := []string{builtinNames[0], "writer"}
		slices.Sort(wantConflicts)
		if !slices.Equal(preview.AgentConflicts, wantConflicts) {
			t.Fatalf("Preview().AgentConflicts = %#v, want %#v", preview.AgentConflicts, wantConflicts)
		}
		_, enableErr := service.Enable(t.Context(), "kit", contract.EnableExtensionRequest{}, actor)
		if !errors.Is(enableErr, extensionpkg.ErrExtensionAgentConflict) {
			t.Fatalf("Enable() error = %v, want agent conflict", enableErr)
		}
		conflictErr, conflictErrMatched := errors.AsType[*extensionpkg.AgentConflictError](enableErr)
		if !conflictErrMatched || !slices.Equal(conflictErr.Agents, preview.AgentConflicts) {
			t.Fatalf("Enable() conflict = %#v, want preview conflicts %#v", conflictErr, preview.AgentConflicts)
		}
		info, err := service.registry.Get("kit")
		if err != nil {
			t.Fatalf("registry.Get(after refusal) error = %v", err)
		}
		if info.Enabled || runtime.reloadCalls != 0 {
			t.Fatalf(
				"enable refusal mutated registry/runtime: enabled=%t reloads=%d",
				info.Enabled,
				runtime.reloadCalls,
			)
		}
	})

	t.Run("Should omit unchanged canonical content", func(t *testing.T) {
		t.Parallel()

		codecs := inventoryTestResourceCodecs(t)
		desiredJob := inventoryTestJob("daily", true)
		desired := []extensionpkg.KitItem{inventoryTestJobItem(t, codecs, desiredJob)}
		semanticallyIdenticalSpec := append([]byte("{\n  "), desired[0].SpecJSON[1:]...)
		service := &daemonExtensionService{resourceCodecs: codecs}
		changes, err := service.extensionKitChanges(t.Context(), desired, []resources.RawRecord{{
			Kind:     automationpkg.JobResourceKind,
			ID:       desiredJob.ID,
			Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			SpecJSON: semanticallyIdenticalSpec,
		}})
		if err != nil {
			t.Fatalf("extensionKitChanges() error = %v", err)
		}
		if len(changes) != 0 {
			t.Fatalf("extensionKitChanges() = %#v, want unchanged resource omitted", changes)
		}
	})

	t.Run("Should omit an unchanged materialized layout", func(t *testing.T) {
		t.Parallel()

		codecs := inventoryTestResourceCodecs(t)
		ext := inventoryTestExtension(true)
		ext.Layouts = []windowmanager.LayoutResource{{
			Version: windowmanager.LayoutResourceVersion, ID: "two-up", DisplayName: "Two up",
			Document: windowmanager.LayoutDocument{
				Version: windowmanager.SnapshotVersion,
				Desktops: []windowmanager.Desktop{{
					ID: "desktop-default", Name: "Desktop 1", Purpose: windowmanager.DesktopPurposeStandard,
					Groups: []windowmanager.LayoutGroup{}, Floating: []windowmanager.WindowID{},
				}},
				Windows: map[windowmanager.WindowID]windowmanager.Window{},
			},
		}}
		desired, err := projectExtensionKitItems(t.Context(), ext, codecs, nil)
		if err != nil {
			t.Fatalf("projectExtensionKitItems() error = %v", err)
		}
		service := &daemonExtensionService{resourceCodecs: codecs}
		changes, err := service.extensionKitChanges(t.Context(), desired, inventoryTestRawRecords(desired))
		if err != nil {
			t.Fatalf("extensionKitChanges() error = %v", err)
		}
		if len(changes) != 0 {
			t.Fatalf("extensionKitChanges() = %#v, want unchanged materialized layout omitted", changes)
		}
	})

	t.Run("Should report changed canonical content", func(t *testing.T) {
		t.Parallel()

		codecs := inventoryTestResourceCodecs(t)
		desiredJob := inventoryTestJob("daily", true)
		desired := []extensionpkg.KitItem{inventoryTestJobItem(t, codecs, desiredJob)}
		liveJob := desiredJob
		liveJob.Prompt = "Previous prompt"
		service := &daemonExtensionService{resourceCodecs: codecs}
		changes, err := service.extensionKitChanges(t.Context(), desired, []resources.RawRecord{{
			Kind:     automationpkg.JobResourceKind,
			ID:       liveJob.ID,
			Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			SpecJSON: inventoryTestJobSpec(t, codecs, liveJob),
		}})
		if err != nil {
			t.Fatalf("extensionKitChanges() error = %v", err)
		}
		want := []contract.ExtensionKitChangePayload{{
			Kind: automationpkg.JobResourceKind, ID: desiredJob.ID,
			Name: desiredJob.Name, Change: contract.ExtensionKitChangeChanged,
		}}
		if !reflect.DeepEqual(changes, want) {
			t.Fatalf("extensionKitChanges() = %#v, want %#v", changes, want)
		}
	})

	t.Run("Should split a rename into removal and addition", func(t *testing.T) {
		t.Parallel()

		codecs := inventoryTestResourceCodecs(t)
		desiredJob := inventoryTestJob("daily", true)
		desired := []extensionpkg.KitItem{inventoryTestJobItem(t, codecs, desiredJob)}
		liveJob := inventoryTestJob("previous", true)
		service := &daemonExtensionService{resourceCodecs: codecs}
		changes, err := service.extensionKitChanges(t.Context(), desired, []resources.RawRecord{{
			Kind:     automationpkg.JobResourceKind,
			ID:       liveJob.ID,
			Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			SpecJSON: inventoryTestJobSpec(t, codecs, liveJob),
		}})
		if err != nil {
			t.Fatalf("extensionKitChanges() error = %v", err)
		}
		want := []contract.ExtensionKitChangePayload{
			{
				Kind: automationpkg.JobResourceKind, ID: desiredJob.ID,
				Name: desiredJob.Name, Change: contract.ExtensionKitChangeAdded,
			},
			{
				Kind: automationpkg.JobResourceKind, ID: liveJob.ID,
				Name: liveJob.Name, Change: contract.ExtensionKitChangeRemoved,
			},
		}
		if !reflect.DeepEqual(changes, want) {
			t.Fatalf("extensionKitChanges() = %#v, want %#v", changes, want)
		}
	})

	t.Run("Should report only enabled automation deltas for an enabled extension", func(t *testing.T) {
		// This preview reads one stable installed-extension snapshot.
		db := openDaemonTestGlobalDB(t)
		installDaemonTestExtension(t, db, "kit", daemonTestExtensionOptions{}, true)
		ext := inventoryTestExtension(true)
		codecs := inventoryTestResourceCodecs(t)
		desired, err := projectExtensionKitItems(t.Context(), ext, codecs, nil)
		if err != nil {
			t.Fatalf("projectExtensionKitItems() error = %v", err)
		}
		live := inventoryTestRawRecords(desired)
		for index := range live {
			if live[index].Kind != automationpkg.JobResourceKind ||
				extensionResourceNameFromID(live[index]) != "alpha" {
				continue
			}
			changedJob := ext.AutomationJobs[0]
			changedJob.Prompt = "Previous prompt"
			live[index].SpecJSON = inventoryTestJobSpec(t, codecs, changedJob)
		}
		automation := inventoryAutomationPreviewer{
			jobs: slices.Clone(ext.AutomationJobs), triggers: slices.Clone(ext.AutomationTriggers),
		}
		service, ok := newDaemonExtensionService(daemonExtensionServiceDeps{
			Registry: extensionpkg.NewRegistry(db.DB()), Runtime: &inventoryExtensionRuntime{ext: ext},
			HomePaths: testHomePaths(t), Logger: discardLogger(), Now: time.Now,
		},
			withDaemonExtensionAutomation(automation),
			withDaemonExtensionResources(inventoryRawStore{records: live}, resources.MutationActor{}, codecs),
		).(*daemonExtensionService)
		if !ok {
			t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
		}

		preview, err := service.Preview(t.Context(), "kit")
		if err != nil {
			t.Fatalf("Preview() error = %v", err)
		}
		if want := []string{"kit/alpha"}; !slices.Equal(preview.AutomationStarting, want) {
			t.Fatalf("Preview().AutomationStarting = %#v, want %#v", preview.AutomationStarting, want)
		}
		if len(preview.Changes) != 1 || preview.Changes[0].Name != "kit/alpha" ||
			preview.Changes[0].Change != contract.ExtensionKitChangeChanged {
			t.Fatalf("Preview().Changes = %#v, want only changed kit/alpha", preview.Changes)
		}
	})

	t.Run("Should reject preview without the required automation dependency", func(t *testing.T) {
		db := openDaemonTestGlobalDB(t)
		installDaemonTestExtension(t, db, "kit", daemonTestExtensionOptions{}, false)
		service := &daemonExtensionService{
			registry: extensionpkg.NewRegistry(db.DB()),
			runtime:  &inventoryExtensionRuntime{ext: inventoryTestExtension(false)},
			logger:   discardLogger(), now: time.Now,
			resourceCodecs: inventoryTestResourceCodecs(t),
		}

		_, err := service.Preview(t.Context(), "kit")
		if err == nil || !strings.Contains(err.Error(), "extension automation previewer is required") {
			t.Fatalf("Preview() error = %v, want required automation dependency error", err)
		}
	})
}

func inventoryTestExtension(enabled bool) *extensionpkg.Extension {
	return &extensionpkg.Extension{
		Info: extensionpkg.ExtensionInfo{Name: "kit", Version: "1.0.0", Enabled: enabled},
		Manifest: &extensionpkg.Manifest{
			Name: "kit", Version: "1.0.0",
		},
		Status: extensionpkg.ExtensionStatus{
			Name: "kit", Version: "1.0.0", Enabled: enabled, Registered: true,
		},
		AutomationJobs: []automationpkg.Job{
			inventoryTestJob("alpha", true),
			inventoryTestJob("zeta", true),
		},
		AutomationTriggers: []automationpkg.Trigger{
			inventoryTestTrigger("beta", true),
		},
	}
}

func inventoryTestJob(name string, enabled bool) automationpkg.Job {
	return automationpkg.Job{
		ID: "extension/kit/automation.job/" + name, Scope: automationpkg.AutomationScopeGlobal,
		Name: "kit/" + name, TargetKind: automationpkg.TargetKindAgent,
		AgentName: "writer", Prompt: "Review repository",
		Schedule: &automationpkg.ScheduleSpec{
			Mode: automationpkg.ScheduleModeAt,
			Time: time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
		Enabled: enabled, Retry: automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(), Source: automationpkg.JobSourcePackage,
	}
}

func inventoryTestTrigger(name string, enabled bool) automationpkg.Trigger {
	return automationpkg.Trigger{
		ID: "extension/kit/automation.trigger/" + name, Scope: automationpkg.AutomationScopeGlobal,
		Name: "kit/" + name, TargetKind: automationpkg.TargetKindAgent,
		AgentName: "writer", Prompt: "Review stopped session", Event: "session.stopped",
		Enabled: enabled, Retry: automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(), Source: automationpkg.JobSourcePackage,
	}
}

func inventoryTestResourceCodecs(t *testing.T) *resources.CodecRegistry {
	t.Helper()
	codecs := resources.NewCodecRegistry()
	if err := registerDaemonResourceCodecs(codecs, nil); err != nil {
		t.Fatalf("registerDaemonResourceCodecs() error = %v", err)
	}
	return codecs
}

func inventoryTestJobSpec(
	t *testing.T,
	codecs *resources.CodecRegistry,
	job automationpkg.Job,
) []byte {
	t.Helper()
	codec, err := resources.ResolveCodec[automationpkg.Job](codecs, automationpkg.JobResourceKind)
	if err != nil {
		t.Fatalf("ResolveCodec(automation job) error = %v", err)
	}
	_, encoded, err := validateAndEncodeResource(
		t.Context(),
		codec,
		resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		job,
	)
	if err != nil {
		t.Fatalf("validateAndEncodeResource(automation job) error = %v", err)
	}
	return encoded
}

func inventoryTestJobItem(
	t *testing.T,
	codecs *resources.CodecRegistry,
	job automationpkg.Job,
) extensionpkg.KitItem {
	t.Helper()
	return extensionpkg.KitItem{
		Kind: automationpkg.JobResourceKind, ID: job.ID, Name: job.Name,
		SpecJSON: inventoryTestJobSpec(t, codecs, job),
	}
}

func inventoryTestRawRecords(items []extensionpkg.KitItem) []resources.RawRecord {
	records := make([]resources.RawRecord, 0, len(items))
	for _, item := range items {
		records = append(records, resources.RawRecord{
			Kind: item.Kind, ID: item.ID,
			Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			Owner: *extensionOwner("kit"), SpecJSON: slices.Clone(item.SpecJSON),
		})
	}
	return records
}

type inventoryExtensionRuntime struct {
	extensionRuntime
	ext         *extensionpkg.Extension
	reloadCalls int
}

func (r *inventoryExtensionRuntime) Get(string) (*extensionpkg.Extension, error) { return r.ext, nil }

func (r *inventoryExtensionRuntime) Reload(context.Context) error {
	r.reloadCalls++
	r.ext.Info.Enabled = true
	r.ext.Status.Enabled = true
	r.ext.Status.Registered = true
	return nil
}

type inventoryMutationSpyStore struct {
	resources.RawStore
	mutationCalls *int
}

func (s *inventoryMutationSpyStore) PutRaw(
	context.Context,
	resources.MutationActor,
	resources.RawDraft,
) (resources.RawRecord, error) {
	(*s.mutationCalls)++
	return resources.RawRecord{}, nil
}

func (s *inventoryMutationSpyStore) DeleteRaw(
	context.Context,
	resources.MutationActor,
	resources.ResourceKind,
	string,
	int64,
) error {
	(*s.mutationCalls)++
	return nil
}

func (s *inventoryMutationSpyStore) ApplySourceSnapshotRaw(
	context.Context,
	resources.MutationActor,
	resources.SourceSnapshot,
) error {
	(*s.mutationCalls)++
	return nil
}

func (*inventoryMutationSpyStore) ListRaw(
	context.Context,
	resources.MutationActor,
	resources.ResourceFilter,
) ([]resources.RawRecord, error) {
	return nil, nil
}

func (r *inventoryExtensionRuntime) InspectPackageResources(
	context.Context,
	string,
) (*extensionpkg.Extension, error) {
	return r.ext, nil
}

type inventoryRawStore struct {
	resources.RawStore
	records []resources.RawRecord
}

func (s inventoryRawStore) ListRaw(
	context.Context,
	resources.MutationActor,
	resources.ResourceFilter,
) ([]resources.RawRecord, error) {
	return slices.Clone(s.records), nil
}

type inventoryAutomationPreviewer struct {
	jobs     []automationpkg.Job
	triggers []automationpkg.Trigger
}

func (s inventoryAutomationPreviewer) Jobs(context.Context) ([]automationpkg.Job, error) {
	return slices.Clone(s.jobs), nil
}

func (s inventoryAutomationPreviewer) Triggers(context.Context) ([]automationpkg.Trigger, error) {
	return slices.Clone(s.triggers), nil
}

func (s inventoryAutomationPreviewer) EffectivePackageAutomation(
	_ context.Context,
	jobs []automationpkg.Job,
	triggers []automationpkg.Trigger,
) ([]automationpkg.Job, []automationpkg.Trigger, error) {
	jobs = slices.Clone(jobs)
	triggers = slices.Clone(triggers)
	for index := range jobs {
		if jobs[index].Name == "kit/zeta" {
			jobs[index].Enabled = false
		}
	}
	return jobs, triggers, nil
}
