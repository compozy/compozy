package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/resources"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestExtensionInventoryAndEnablePreview(t *testing.T) {
	t.Parallel()

	t.Run("Should report shipped resources as not live while disabled", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		ext := inventoryTestExtension(false)
		resourceID := ext.AutomationJobs[0].ID
		specJSON, err := json.Marshal(map[string]string{"name": ext.AutomationJobs[0].Name})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		service := &daemonExtensionService{
			registry: extensionpkg.NewRegistry(db.DB()),
			runtime:  &inventoryExtensionRuntime{ext: ext},
			resourceStore: inventoryRawStore{records: []resources.RawRecord{{
				Kind: automationpkg.JobResourceKind, ID: resourceID, SpecJSON: specJSON,
				Owner: *extensionOwner("kit"),
			}}},
			logger: discardLogger(),
		}

		inventory, err := service.Inventory(context.Background(), "kit")
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
		service, ok := newDaemonExtensionService(
			extensionpkg.NewRegistry(db.DB()),
			runtime,
			nil,
			nil,
			nil,
			nil,
			testHomePaths(t),
			discardLogger(),
			func() time.Time { return time.Date(2026, 8, 2, 14, 0, 0, 0, time.UTC) },
			withDaemonExtensionAutomation(automation),
		).(*daemonExtensionService)
		if !ok {
			t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
		}
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "cli")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}

		preview, err := service.Preview(context.Background(), "kit")
		if err != nil {
			t.Fatalf("Preview() error = %v", err)
		}
		wantStarting := []string{"kit/alpha", "kit/beta"}
		if !reflect.DeepEqual(preview.AutomationStarting, wantStarting) {
			t.Fatalf("Preview().AutomationStarting = %#v, want %#v", preview.AutomationStarting, wantStarting)
		}
		result, err := service.Enable(context.Background(), "kit", contract.EnableExtensionRequest{}, actor)
		if err != nil {
			t.Fatalf("Enable() error = %v", err)
		}
		if !reflect.DeepEqual(result.AutomationStarted, preview.AutomationStarting) {
			t.Fatalf(
				"Enable().AutomationStarted = %#v, want preview plan %#v",
				result.AutomationStarted,
				preview.AutomationStarting,
			)
		}

		enabledPreview, err := service.Preview(context.Background(), "kit")
		if err != nil {
			t.Fatalf("Preview(enabled) error = %v", err)
		}
		if len(enabledPreview.AutomationStarting) != 0 {
			t.Fatalf("Preview(enabled).AutomationStarting = %#v, want empty", enabledPreview.AutomationStarting)
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
			resourceStore: &inventoryMutationSpyStore{mutationCalls: &mutationCalls},
			secretVault:   secretVault,
			eventWriter:   writer,
			logger:        discardLogger(),
			now:           time.Now,
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
			{Agent: compozyconfig.AgentDef{Name: "writer"}},
			{Agent: compozyconfig.AgentDef{Name: builtinNames[0]}},
		}
		writerSpec, err := json.Marshal(map[string]string{"name": "writer"})
		if err != nil {
			t.Fatalf("json.Marshal(writer spec) error = %v", err)
		}
		runtime := &inventoryExtensionRuntime{ext: ext}
		store := inventoryRawStore{records: []resources.RawRecord{{
			Kind:     compozyconfig.AgentResourceKind,
			ID:       "authored/agent/writer",
			SpecJSON: writerSpec,
			Owner:    resources.ResourceOwner{Kind: resources.ResourceOwnerKind("operator"), ID: "operator"},
		}}}
		service, ok := newDaemonExtensionService(
			extensionpkg.NewRegistry(db.DB()),
			runtime,
			nil,
			nil,
			nil,
			nil,
			testHomePaths(t),
			discardLogger(),
			time.Now,
			withDaemonExtensionResources(store, resources.MutationActor{}),
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
		if !reflect.DeepEqual(preview.AgentConflicts, wantConflicts) {
			t.Fatalf("Preview().AgentConflicts = %#v, want %#v", preview.AgentConflicts, wantConflicts)
		}
		_, enableErr := service.Enable(t.Context(), "kit", contract.EnableExtensionRequest{}, actor)
		if !errors.Is(enableErr, extensionpkg.ErrExtensionAgentConflict) {
			t.Fatalf("Enable() error = %v, want agent conflict", enableErr)
		}
		var conflictErr *extensionpkg.AgentConflictError
		if !errors.As(enableErr, &conflictErr) || !reflect.DeepEqual(conflictErr.Agents, preview.AgentConflicts) {
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

	t.Run("Should join content changes by kind and name and split renames", func(t *testing.T) {
		t.Parallel()

		kind := automationpkg.JobResourceKind
		desired := []extensionpkg.KitItem{{
			Kind: kind, ID: "extension/kit/automation.job/daily", Name: "kit/daily",
		}}
		changedSpec, err := json.Marshal(map[string]any{"name": "kit/daily", "schedule": "changed"})
		if err != nil {
			t.Fatalf("json.Marshal(changed spec) error = %v", err)
		}
		changed := mergeExtensionKitInventory(desired, []resources.RawRecord{{
			Kind: kind, ID: desired[0].ID, SpecJSON: changedSpec,
		}})
		if len(changed) != 1 || !changed[0].Live || changed[0].Name != "kit/daily" {
			t.Fatalf("content-changed inventory = %#v, want one joined live item", changed)
		}

		renamedSpec, err := json.Marshal(map[string]any{"name": "kit/old"})
		if err != nil {
			t.Fatalf("json.Marshal(renamed spec) error = %v", err)
		}
		renamed := mergeExtensionKitInventory(desired, []resources.RawRecord{{
			Kind: kind, ID: "extension/kit/automation.job/old", SpecJSON: renamedSpec,
		}})
		if len(renamed) != 2 || renamed[0].Name != "kit/daily" || renamed[0].Live ||
			renamed[1].Name != "kit/old" || !renamed[1].Live {
			t.Fatalf("renamed inventory = %#v, want shipped add plus live removal", renamed)
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
			{ID: "extension/kit/automation.job/alpha", Name: "kit/alpha", Enabled: true},
			{ID: "extension/kit/automation.job/zeta", Name: "kit/zeta", Enabled: true},
		},
		AutomationTriggers: []automationpkg.Trigger{
			{ID: "extension/kit/automation.trigger/beta", Name: "kit/beta", Enabled: true},
		},
	}
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
	*s.mutationCalls++
	return resources.RawRecord{}, nil
}

func (s *inventoryMutationSpyStore) DeleteRaw(
	context.Context,
	resources.MutationActor,
	resources.ResourceKind,
	string,
	int64,
) error {
	*s.mutationCalls++
	return nil
}

func (s *inventoryMutationSpyStore) ApplySourceSnapshotRaw(
	context.Context,
	resources.MutationActor,
	resources.SourceSnapshot,
) error {
	*s.mutationCalls++
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
