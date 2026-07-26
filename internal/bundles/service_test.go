package bundles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"path/filepath"
	"strings"
	"testing"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func discardBundleTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type memoryStore struct {
	activations   map[string]Activation
	inventory     map[string][]InventoryItem
	bundles       []resources.Record[BundleResourceSpec]
	agents        []resources.Record[aghconfig.AgentDef]
	applied       []BundleActivationResourcePlan
	applyErr      error
	applyAfterErr error

	createBundleActivationHook func(Activation) error
	updateBundleActivationHook func(Activation) error
	deleteBundleActivationHook func(string) error
	listBundleActivationsHook  func() ([]Activation, error)
	listBundleInventoryHook    func(string) ([]InventoryItem, error)
	listBundleResourcesHook    func() ([]resources.Record[BundleResourceSpec], error)
	listAgentResourcesHook     func() ([]resources.Record[aghconfig.AgentDef], error)
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		activations: make(map[string]Activation),
		inventory:   make(map[string][]InventoryItem),
	}
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	next := make(map[string]string, len(values))
	maps.Copy(next, values)
	return next
}

func (s *memoryStore) CreateBundleActivation(_ context.Context, activation Activation) error {
	if s.createBundleActivationHook != nil {
		if err := s.createBundleActivationHook(activation); err != nil {
			return err
		}
	}
	if _, exists := s.activations[activation.ID]; exists {
		return errors.New("duplicate activation")
	}
	activation.Version = 1
	s.activations[activation.ID] = activation
	return nil
}

func (s *memoryStore) UpdateBundleActivation(_ context.Context, activation Activation) error {
	if s.updateBundleActivationHook != nil {
		if err := s.updateBundleActivationHook(activation); err != nil {
			return err
		}
	}
	current, exists := s.activations[activation.ID]
	if !exists {
		return ErrActivationNotFound
	}
	if activation.Version != current.Version {
		return fmt.Errorf("%w: expected version %d", resources.ErrConflict, activation.Version)
	}
	activation.Version++
	s.activations[activation.ID] = activation
	return nil
}

func (s *memoryStore) DeleteBundleActivation(_ context.Context, id string) error {
	if s.deleteBundleActivationHook != nil {
		if err := s.deleteBundleActivationHook(id); err != nil {
			return err
		}
	}
	if _, exists := s.activations[id]; !exists {
		return ErrActivationNotFound
	}
	delete(s.activations, id)
	delete(s.inventory, id)
	return nil
}

func (s *memoryStore) GetBundleActivation(_ context.Context, id string) (Activation, error) {
	activation, exists := s.activations[id]
	if !exists {
		return Activation{}, ErrActivationNotFound
	}
	return activation, nil
}

func (s *memoryStore) ListBundleActivations(_ context.Context) ([]Activation, error) {
	if s.listBundleActivationsHook != nil {
		return s.listBundleActivationsHook()
	}
	items := make([]Activation, 0, len(s.activations))
	for _, activation := range s.activations {
		items = append(items, activation)
	}
	return items, nil
}

func (s *memoryStore) ListBundleActivationInventory(_ context.Context, activationID string) ([]InventoryItem, error) {
	if s.listBundleInventoryHook != nil {
		return s.listBundleInventoryHook(activationID)
	}
	return append([]InventoryItem(nil), s.inventory[activationID]...), nil
}

func (s *memoryStore) ListBundleResources(
	_ context.Context,
) ([]resources.Record[BundleResourceSpec], error) {
	if s.listBundleResourcesHook != nil {
		return s.listBundleResourcesHook()
	}
	return append([]resources.Record[BundleResourceSpec](nil), s.bundles...), nil
}

func (s *memoryStore) ListAgentResources(
	_ context.Context,
) ([]resources.Record[aghconfig.AgentDef], error) {
	if s.listAgentResourcesHook != nil {
		return s.listAgentResourcesHook()
	}
	return append([]resources.Record[aghconfig.AgentDef](nil), s.agents...), nil
}

func (s *memoryStore) ApplyBundleActivationResources(
	_ context.Context,
	plan BundleActivationResourcePlan,
) error {
	if s.applyErr != nil {
		return s.applyErr
	}
	next := plan
	next.activeActivationIDs = cloneStringSet(plan.activeActivationIDs)
	next.desiredLayouts = cloneOwnedLayoutResources(plan.desiredLayouts)
	next.desiredAgents = cloneOwnedAgentResources(plan.desiredAgents)
	next.desiredSouls = cloneOwnedSoulResources(plan.desiredSouls)
	next.desiredHeartbeats = cloneOwnedHeartbeatResources(plan.desiredHeartbeats)
	next.desiredJobs = cloneJobsForBundle(plan.desiredJobs)
	next.desiredTriggers = cloneTriggersForBundle(plan.desiredTriggers)
	next.desiredBridges = cloneBridgeInstancesForBundle(plan.desiredBridges)
	next.layoutOwners = copyStringMap(plan.layoutOwners)
	next.agentOwners = copyStringMap(plan.agentOwners)
	next.soulOwners = copyStringMap(plan.soulOwners)
	next.heartbeatOwners = copyStringMap(plan.heartbeatOwners)
	next.jobOwners = copyStringMap(plan.jobOwners)
	next.triggerOwners = copyStringMap(plan.triggerOwners)
	next.bridgeOwners = copyStringMap(plan.bridgeOwners)
	next.declaredChannels = append([]DeclaredChannel(nil), plan.declaredChannels...)
	s.applied = append(s.applied, next)
	s.inventory = make(map[string][]InventoryItem)
	s.applyDesiredAgentRecords(plan)
	for _, layout := range plan.desiredLayouts {
		activationID := strings.TrimSpace(plan.layoutOwners[strings.TrimSpace(layout.ID)])
		s.inventory[activationID] = append(s.inventory[activationID], InventoryItem{
			ActivationID: activationID,
			ResourceKind: string(windowmanager.WindowLayoutResourceKind),
			ResourceID:   layout.ID,
			ResourceName: layout.Spec.DisplayName,
		})
	}
	for _, agent := range plan.desiredAgents {
		activationID := strings.TrimSpace(plan.agentOwners[strings.TrimSpace(agent.ID)])
		s.inventory[activationID] = append(s.inventory[activationID], InventoryItem{
			ActivationID: activationID,
			ResourceKind: string(aghconfig.AgentResourceKind),
			ResourceID:   agent.ID,
			ResourceName: agent.Spec.Name,
		})
	}
	for _, spec := range plan.desiredSouls {
		activationID := strings.TrimSpace(plan.soulOwners[strings.TrimSpace(spec.ID)])
		s.inventory[activationID] = append(s.inventory[activationID], InventoryItem{
			ActivationID: activationID,
			ResourceKind: string(soul.ResourceKind),
			ResourceID:   spec.ID,
			ResourceName: spec.Spec.AgentName,
		})
	}
	for _, spec := range plan.desiredHeartbeats {
		activationID := strings.TrimSpace(plan.heartbeatOwners[strings.TrimSpace(spec.ID)])
		s.inventory[activationID] = append(s.inventory[activationID], InventoryItem{
			ActivationID: activationID,
			ResourceKind: string(heartbeat.ResourceKind),
			ResourceID:   spec.ID,
			ResourceName: spec.Spec.AgentName,
		})
	}
	for _, job := range plan.desiredJobs {
		activationID := strings.TrimSpace(plan.jobOwners[strings.TrimSpace(job.ID)])
		s.inventory[activationID] = append(s.inventory[activationID], InventoryItem{
			ActivationID: activationID,
			ResourceKind: string(automationpkg.JobResourceKind),
			ResourceID:   job.ID,
			ResourceName: job.Name,
		})
	}
	for _, trigger := range plan.desiredTriggers {
		activationID := strings.TrimSpace(plan.triggerOwners[strings.TrimSpace(trigger.ID)])
		s.inventory[activationID] = append(s.inventory[activationID], InventoryItem{
			ActivationID: activationID,
			ResourceKind: string(automationpkg.TriggerResourceKind),
			ResourceID:   trigger.ID,
			ResourceName: trigger.Name,
		})
	}
	for _, instance := range plan.desiredBridges {
		activationID := strings.TrimSpace(plan.bridgeOwners[strings.TrimSpace(instance.ID)])
		s.inventory[activationID] = append(s.inventory[activationID], InventoryItem{
			ActivationID: activationID,
			ResourceKind: string(bridgepkg.BridgeInstanceResourceKind),
			ResourceID:   instance.ID,
			ResourceName: instance.DisplayName,
		})
	}
	if s.applyAfterErr != nil {
		err := s.applyAfterErr
		s.applyAfterErr = nil
		return err
	}
	return nil
}

func (s *memoryStore) applyDesiredAgentRecords(plan BundleActivationResourcePlan) {
	preserved := make([]resources.Record[aghconfig.AgentDef], 0, len(s.agents)+len(plan.desiredAgents))
	for _, record := range s.agents {
		if record.Owner.Kind.Normalize() == BundleActivationOwnerKind {
			continue
		}
		preserved = append(preserved, record)
	}
	for _, agent := range plan.desiredAgents {
		activationID := strings.TrimSpace(plan.agentOwners[strings.TrimSpace(agent.ID)])
		preserved = append(preserved, resources.Record[aghconfig.AgentDef]{
			Kind:  aghconfig.AgentResourceKind,
			ID:    strings.TrimSpace(agent.ID),
			Scope: agent.Scope.Normalize(),
			Owner: ownerForActivation(activationID),
			Spec:  aghconfig.CloneAgentDef(agent.Spec),
		})
	}
	s.agents = preserved
}

type staticExtensionLister struct {
	items []extensionpkg.ExtensionInfo
}

func (l staticExtensionLister) List() ([]extensionpkg.ExtensionInfo, error) {
	return append([]extensionpkg.ExtensionInfo(nil), l.items...), nil
}

type memoryWorkspaceResolver struct {
	resolveFn           func(context.Context, string) (workspacepkg.ResolvedWorkspace, error)
	resolveOrRegisterFn func(context.Context, string) (workspacepkg.ResolvedWorkspace, error)
}

func (r memoryWorkspaceResolver) Resolve(ctx context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
	if r.resolveFn != nil {
		return r.resolveFn(ctx, idOrPath)
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (r memoryWorkspaceResolver) ResolveOrRegister(
	ctx context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	if r.resolveOrRegisterFn != nil {
		return r.resolveOrRegisterFn(ctx, path)
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func newMarketingExtension() *extensionpkg.Extension {
	return &extensionpkg.Extension{
		Info: extensionpkg.ExtensionInfo{Name: "marketing-team"},
		Manifest: &extensionpkg.Manifest{
			Name: "marketing-team",
			Bridge: extensionpkg.BridgeConfig{
				Platform:    "telegram",
				DisplayName: "Telegram",
			},
		},
		Bundles: []extensionpkg.BundleSpec{{
			Name:        "marketing",
			Description: "Marketing team bundle",
			Profiles: []extensionpkg.BundleProfile{{
				Name: "default",
				Channels: extensionpkg.BundleChannelsConfig{
					Primary: "marketing",
					Items: []extensionpkg.BundleChannel{{
						Name:        "marketing",
						Description: "Primary marketing channel",
					}},
				},
				Layouts: []extensionpkg.BundleLayout{{
					Path:   "layouts/marketing.json",
					Layout: bundleTestLayoutResource("marketing-two-up", "Marketing two-up"),
				}},
				Agents: []extensionpkg.BundleAgent{{
					Path: "agents/marketer",
					Agent: aghconfig.AgentDef{
						Name:   "marketer",
						Prompt: "Run marketing workflows.",
						Model:  "sonnet",
					},
					Soul: &extensionpkg.BundleAgentSidecar{
						SourcePath: "agents/marketer/SOUL.md",
						Body:       "Lead with campaign context.",
					},
					Heartbeat: &extensionpkg.BundleAgentSidecar{
						SourcePath: "agents/marketer/HEARTBEAT.md",
						Body:       "Inspect campaign status and use AGH task APIs.",
					},
				}},
				Jobs: []extensionpkg.BundleJob{{
					Name:      "daily-sync",
					AgentName: "marketer",
					Prompt:    "Summarize campaign status",
					Schedule:  automationpkg.ScheduleSpec{Mode: automationpkg.ScheduleModeEvery, Interval: "1h"},
					Enabled:   true,
					Retry:     automationpkg.DefaultRetryConfig(),
					FireLimit: automationpkg.DefaultFireLimitConfig(),
				}},
				Triggers: []extensionpkg.BundleTrigger{{
					Name:      "session-opened",
					AgentName: "marketer",
					Prompt:    "React to session open",
					Event:     "session.created",
					Enabled:   true,
					Retry:     automationpkg.DefaultRetryConfig(),
					FireLimit: automationpkg.DefaultFireLimitConfig(),
				}},
				Bridges: []extensionpkg.BundleBridgePreset{{
					Name:        "telegram-main",
					DisplayName: "Marketing Telegram",
					RoutingPolicy: bridgepkg.RoutingPolicy{
						IncludePeer: true,
					},
					SecretSlots: []extensionpkg.BundleBridgeSecretSlot{{
						Name: "bot_token",
						Kind: "token",
					}},
				}},
			}},
		}},
	}
}

func bundleTestLayoutResource(id string, displayName string) windowmanager.LayoutResource {
	return windowmanager.LayoutResource{
		Version:        windowmanager.LayoutResourceVersion,
		ID:             id,
		DisplayName:    displayName,
		AspectVariant:  windowmanager.LayoutAspectAny,
		OverflowPolicy: windowmanager.LayoutOverflowStack,
		Document: windowmanager.LayoutDocument{
			Version: windowmanager.SnapshotVersion,
			Desktops: []windowmanager.Desktop{{
				ID:       "desktop-default",
				Name:     "Desktop 1",
				Purpose:  windowmanager.DesktopPurposeStandard,
				Groups:   []windowmanager.LayoutGroup{},
				Floating: []windowmanager.WindowID{},
			}},
			Windows: map[windowmanager.WindowID]windowmanager.Window{},
		},
	}
}

func newMarketingService(store *memoryStore, opts ...Option) *Service {
	ext := newMarketingExtension()
	if len(store.agents) == 0 {
		store.agents = []resources.Record[aghconfig.AgentDef]{plannerAgentRecord()}
	}
	return newServiceForExtensions(store, []*extensionpkg.Extension{ext}, opts...)
}

func newServiceForExtensions(
	store *memoryStore,
	extensions []*extensionpkg.Extension,
	opts ...Option,
) *Service {
	byName := make(map[string]*extensionpkg.Extension, len(extensions))
	infos := make([]extensionpkg.ExtensionInfo, 0, len(extensions))
	bundles := make([]resources.Record[BundleResourceSpec], 0, len(extensions))
	for _, ext := range extensions {
		if ext == nil {
			continue
		}
		byName[ext.Info.Name] = ext
		infos = append(infos, extensionpkg.ExtensionInfo{Name: ext.Info.Name})
		for _, bundle := range ext.Bundles {
			ownerBridgePlatform := ""
			ownerProvidesBridgeAdapter := false
			if ext.Manifest != nil {
				ownerBridgePlatform = ext.Manifest.Bridge.Platform
				ownerProvidesBridgeAdapter = strings.TrimSpace(ownerBridgePlatform) != ""
			}
			bundles = append(bundles, resources.Record[BundleResourceSpec]{
				Kind:  BundleResourceKind,
				ID:    BundleResourceID(ext.Info.Name, bundle.Name),
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				Spec: BundleResourceSpec{
					ExtensionName:              ext.Info.Name,
					Bundle:                     bundle,
					OwnerBridgePlatform:        ownerBridgePlatform,
					OwnerProvidesBridgeAdapter: ownerProvidesBridgeAdapter,
				},
			})
		}
	}
	store.bundles = bundles
	options := []Option{
		WithNow(func() time.Time {
			return time.Date(2026, 4, 14, 22, 0, 0, 0, time.UTC)
		}),
	}
	options = append(options, opts...)
	return NewService(
		store,
		staticExtensionLister{items: infos},
		func(_ context.Context, name string) (*extensionpkg.Extension, error) {
			ext, ok := byName[name]
			if !ok {
				return nil, extensionpkg.ErrExtensionNotFound
			}
			return ext, nil
		},
		options...,
	)
}

func plannerAgentRecord() resources.Record[aghconfig.AgentDef] {
	return resources.Record[aghconfig.AgentDef]{
		Kind:  aghconfig.AgentResourceKind,
		ID:    "agent_planner",
		Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		Spec: aghconfig.AgentDef{
			Name:   "planner",
			Prompt: "Plan the work.",
		},
	}
}

func bundleAgentRecord(
	id string,
	name string,
	scope resources.ResourceScope,
) resources.Record[aghconfig.AgentDef] {
	return resources.Record[aghconfig.AgentDef]{
		Kind:  aghconfig.AgentResourceKind,
		ID:    id,
		Scope: scope,
		Spec: aghconfig.AgentDef{
			Name:   name,
			Prompt: "Existing agent.",
		},
	}
}

func marketingExtensionNamed(name string) *extensionpkg.Extension {
	ext := newMarketingExtension()
	ext.Info.Name = name
	if ext.Manifest != nil {
		ext.Manifest.Name = name
	}
	return ext
}

func TestServiceActivateMaterializesManagedResources(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	service := newMarketingService(store)

	preview, err := service.Activate(testutil.Context(t), ActivateRequest{
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
	})
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	if got, want := len(preview.Inventory), 7; got != want {
		t.Fatalf("len(preview.Inventory) = %d, want %d", got, want)
	}
	if got, want := len(store.applied), 1; got != want {
		t.Fatalf("len(applied plans) = %d, want %d", got, want)
	}
	plan := store.applied[0]
	if got, want := len(plan.desiredLayouts), 1; got != want {
		t.Fatalf("len(plan.desiredLayouts) = %d, want %d", got, want)
	}
	layout := plan.desiredLayouts[0]
	if layout.ID != layout.Spec.ID {
		t.Fatalf("layout record/spec IDs = %q/%q, want identical", layout.ID, layout.Spec.ID)
	}
	if got, want := layout.ID,
		stableID("lay", preview.Activation.ID, "marketing-two-up"); got != want {
		t.Fatalf("layout ID = %q, want %q", got, want)
	}
	if got, want := len(plan.desiredAgents), 1; got != want {
		t.Fatalf("len(plan.desiredAgents) = %d, want %d", got, want)
	}
	if got, want := plan.desiredAgents[0].Spec.Name, "marketer"; got != want {
		t.Fatalf("plan.desiredAgents[0].Spec.Name = %q, want %q", got, want)
	}
	if got, want := len(plan.desiredSouls), 1; got != want {
		t.Fatalf("len(plan.desiredSouls) = %d, want %d", got, want)
	}
	if got, want := len(plan.desiredHeartbeats), 1; got != want {
		t.Fatalf("len(plan.desiredHeartbeats) = %d, want %d", got, want)
	}
	if got, want := len(plan.desiredJobs), 1; got != want {
		t.Fatalf("len(plan.desiredJobs) = %d, want %d", got, want)
	}
	if got, want := plan.desiredJobs[0].Source, automationpkg.JobSourcePackage; got != want {
		t.Fatalf("plan.desiredJobs[0].Source = %q, want %q", got, want)
	}
	if got, want := len(plan.desiredTriggers), 1; got != want {
		t.Fatalf("len(plan.desiredTriggers) = %d, want %d", got, want)
	}
	if got, want := len(plan.desiredBridges), 1; got != want {
		t.Fatalf("len(plan.desiredBridges) = %d, want %d", got, want)
	}
	if strings.TrimSpace(preview.Activation.SpecContentHash) == "" {
		t.Fatal("preview.Activation.SpecContentHash = empty, want persisted hash")
	}

	settings, err := service.NetworkSettings(testutil.Context(t))
	if err != nil {
		t.Fatalf("NetworkSettings() error = %v", err)
	}
	if got, want := len(settings.DeclaredChannels), 1; got != want {
		t.Fatalf("len(DeclaredChannels) = %d, want %d", got, want)
	}
	if !settings.DeclaredChannels[0].Primary {
		t.Fatal("DeclaredChannels[0].Primary = false, want true")
	}
}

func TestServiceCatalogPreviewListAndGetUseCanonicalResources(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	service := newMarketingService(store, WithLogger(discardBundleTestLogger()))

	catalog, err := service.Catalog(testutil.Context(t))
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if got, want := len(catalog), 1; got != want {
		t.Fatalf("len(Catalog()) = %d, want %d", got, want)
	}
	preview, err := service.PreviewActivation(testutil.Context(t), ActivateRequest{
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
	})
	if err != nil {
		t.Fatalf("PreviewActivation() error = %v", err)
	}
	if got, want := len(preview.Inventory), 7; got != want {
		t.Fatalf("len(preview.Inventory) = %d, want %d", got, want)
	}
	activated, err := service.Activate(testutil.Context(t), ActivateRequest{
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
	})
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	listed, err := service.ListActivations(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListActivations() error = %v", err)
	}
	if got, want := len(listed), 1; got != want {
		t.Fatalf("len(ListActivations()) = %d, want %d", got, want)
	}
	loaded, err := service.GetActivation(testutil.Context(t), activated.Activation.ID)
	if err != nil {
		t.Fatalf("GetActivation() error = %v", err)
	}
	if got, want := len(loaded.Inventory), 7; got != want {
		t.Fatalf("len(GetActivation().Inventory) = %d, want %d", got, want)
	}
}

func TestServiceActivationSpecDriftUsesContentHashAndClearsOnReapply(t *testing.T) {
	t.Parallel()

	t.Run("Should expose drift on list and get then clear it after update reapply", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		service := newMarketingService(store, WithLogger(discardBundleTestLogger()))
		activated, err := service.Activate(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("Activate() error = %v", err)
		}
		if activated.SpecDrift {
			t.Fatal("Activate() SpecDrift = true, want false for a new activation")
		}

		storedBefore := store.activations[activated.Activation.ID]
		store.bundles[0].Spec.Bundle.Profiles[0].Description = "Updated profile contract"

		listed, err := service.ListActivations(testutil.Context(t))
		if err != nil {
			t.Fatalf("ListActivations() error = %v", err)
		}
		if len(listed) != 1 || !listed[0].SpecDrift {
			t.Fatalf("ListActivations() = %#v, want one drifted activation", listed)
		}
		loaded, err := service.GetActivation(testutil.Context(t), activated.Activation.ID)
		if err != nil {
			t.Fatalf("GetActivation() error = %v", err)
		}
		if !loaded.SpecDrift {
			t.Fatal("GetActivation() SpecDrift = false, want true")
		}

		reapplied, err := service.UpdateActivation(testutil.Context(t), UpdateActivationRequest{
			ID:              activated.Activation.ID,
			ExpectedVersion: activated.Activation.Version,
		})
		if err != nil {
			t.Fatalf("UpdateActivation() error = %v", err)
		}
		if reapplied.SpecDrift {
			t.Fatal("UpdateActivation() SpecDrift = true, want false after reapply")
		}
		storedAfter := store.activations[activated.Activation.ID]
		if storedAfter.SpecContentHash == storedBefore.SpecContentHash {
			t.Fatalf("UpdateActivation() SpecContentHash = %q, want refreshed hash", storedAfter.SpecContentHash)
		}
	})

	t.Run("Should require confirmation when reapply introduces a live network requirement", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		store.agents = []resources.Record[aghconfig.AgentDef]{plannerAgentRecord()}
		ext := newMarketingExtension()
		service := newServiceForExtensions(
			store,
			[]*extensionpkg.Extension{ext},
			WithLogger(discardBundleTestLogger()),
		)
		activated, err := service.Activate(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("Activate() error = %v", err)
		}

		ext.Manifest.NetworkParticipation = &extensionpkg.NetworkParticipationRequirement{
			Required:      true,
			Mode:          "live",
			ChannelScopes: []string{"workspace"},
		}
		_, err = service.UpdateActivation(testutil.Context(t), UpdateActivationRequest{
			ID:              activated.Activation.ID,
			ExpectedVersion: activated.Activation.Version,
		})
		if !errors.Is(err, ErrNetworkRequirementConfirmationRequired) {
			t.Fatalf("UpdateActivation() error = %v, want confirmation required", err)
		}
		storedAfterRejection := store.activations[activated.Activation.ID]
		if storedAfterRejection.NetworkRequirementDigest != "" || storedAfterRejection.ConfirmedBy != "" {
			t.Fatalf("rejected UpdateActivation() persisted confirmation = %#v", storedAfterRejection)
		}

		reapplied, err := service.UpdateActivation(testutil.Context(t), UpdateActivationRequest{
			ID:                        activated.Activation.ID,
			ExpectedVersion:           activated.Activation.Version,
			ConfirmNetworkRequirement: true,
		})
		if err != nil {
			t.Fatalf("confirmed UpdateActivation() error = %v", err)
		}
		if reapplied.SpecDrift {
			t.Fatal("confirmed UpdateActivation() SpecDrift = true, want false")
		}
		if reapplied.Activation.NetworkRequirementDigest == "" ||
			reapplied.Activation.ConfirmedBy != networkRequirementConfirmedByOperator {
			t.Fatalf("confirmed UpdateActivation() activation = %#v", reapplied.Activation)
		}
	})

	t.Run("Should treat a missing stored hash as drift regardless of timestamps", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		service := newMarketingService(store, WithLogger(discardBundleTestLogger()))
		activated, err := service.Activate(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("Activate() error = %v", err)
		}
		stored := store.activations[activated.Activation.ID]
		stored.SpecContentHash = ""
		stored.UpdatedAt = time.Now().UTC().Add(24 * time.Hour)
		store.activations[stored.ID] = stored

		loaded, err := service.GetActivation(testutil.Context(t), stored.ID)
		if err != nil {
			t.Fatalf("GetActivation() error = %v", err)
		}
		if !loaded.SpecDrift {
			t.Fatal("GetActivation() SpecDrift = false, want true for missing stored hash")
		}
	})
}

func TestServiceAgentValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a reserved bundle agent before activation", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		ext := newMarketingExtension()
		profile := &ext.Bundles[0].Profiles[0]
		profile.Agents[0].Agent.Name = "coordinator"
		profile.Jobs[0].AgentName = "coordinator"
		profile.Triggers[0].AgentName = "coordinator"
		service := newServiceForExtensions(store, []*extensionpkg.Extension{ext}, WithLogger(discardBundleTestLogger()))

		_, err := service.Activate(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		})
		if !errors.Is(err, aghconfig.ErrAgentNameReserved) {
			t.Fatalf("Activate() error = %v, want ErrAgentNameReserved", err)
		}
		if !strings.Contains(err.Error(), "/agents/coordinator/AGENT.md") {
			t.Fatalf("Activate() error = %v, want reserved bundle path", err)
		}
		if len(store.activations) != 0 || len(store.agents) != 0 || len(store.applied) != 0 {
			t.Fatalf(
				"reserved activation mutated state: activations=%d agents=%d applied=%d",
				len(store.activations),
				len(store.agents),
				len(store.applied),
			)
		}
	})

	t.Run("Should materialize an ordinary bundle agent", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		ext := newMarketingExtension()
		profile := &ext.Bundles[0].Profiles[0]
		profile.Agents[0].Agent.Name = "code_implementer"
		profile.Jobs[0].AgentName = "code_implementer"
		profile.Triggers[0].AgentName = "code_implementer"
		service := newServiceForExtensions(store, []*extensionpkg.Extension{ext}, WithLogger(discardBundleTestLogger()))

		if _, err := service.Activate(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		}); err != nil {
			t.Fatalf("Activate() error = %v", err)
		}
		if len(store.agents) != 1 || store.agents[0].Spec.Name != "code_implementer" {
			t.Fatalf("materialized agents = %#v, want code_implementer", store.agents)
		}
	})

	t.Run("Should reject conflicting visible agent name", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		store.agents = []resources.Record[aghconfig.AgentDef]{
			bundleAgentRecord("agent_existing_marketer", "marketer", resources.ResourceScope{
				Kind: resources.ResourceScopeKindGlobal,
			}),
		}
		service := newMarketingService(store, WithLogger(discardBundleTestLogger()))

		_, err := service.PreviewActivation(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		})
		if !errors.Is(err, ErrAgentConflict) {
			t.Fatalf("PreviewActivation() error = %v, want ErrAgentConflict", err)
		}
	})

	t.Run("Should reject unresolved job agent reference", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		ext := newMarketingExtension()
		ext.Bundles[0].Profiles[0].Jobs[0].AgentName = "missing-agent"
		ext.Bundles[0].Profiles[0].Triggers = nil
		service := newServiceForExtensions(store, []*extensionpkg.Extension{ext}, WithLogger(discardBundleTestLogger()))

		_, err := service.PreviewActivation(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		})
		if !errors.Is(err, ErrAgentReferenceNotFound) {
			t.Fatalf("PreviewActivation() error = %v, want ErrAgentReferenceNotFound", err)
		}
	})

	t.Run("Should reject unresolved trigger agent reference", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		ext := newMarketingExtension()
		ext.Bundles[0].Profiles[0].Jobs = nil
		ext.Bundles[0].Profiles[0].Triggers[0].AgentName = "missing-agent"
		service := newServiceForExtensions(store, []*extensionpkg.Extension{ext}, WithLogger(discardBundleTestLogger()))

		_, err := service.PreviewActivation(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		})
		if !errors.Is(err, ErrAgentReferenceNotFound) {
			t.Fatalf("PreviewActivation() error = %v, want ErrAgentReferenceNotFound", err)
		}
	})

	t.Run("Should reject cross activation agent name overlap", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		left := marketingExtensionNamed("marketing-team-left")
		right := marketingExtensionNamed("marketing-team-right")
		resolver := memoryWorkspaceResolver{
			resolveFn: func(_ context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{
						ID:      idOrPath,
						Name:    idOrPath,
						RootDir: filepath.Join(t.TempDir(), idOrPath),
					},
				}, nil
			},
		}
		service := newServiceForExtensions(
			store,
			[]*extensionpkg.Extension{left, right},
			WithLogger(discardBundleTestLogger()),
			WithWorkspaceResolver(resolver),
		)
		leftActivation := Activation{
			ID:            ActivationResourceID(left.Info.Name, "marketing", "default", ScopeWorkspace, "ws-marketing"),
			ExtensionName: left.Info.Name,
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeWorkspace,
			WorkspaceID:   "ws-marketing",
		}
		rightActivation := Activation{
			ID: ActivationResourceID(
				right.Info.Name,
				"marketing",
				"default",
				ScopeWorkspace,
				"ws-marketing",
			),
			ExtensionName: right.Info.Name,
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeWorkspace,
			WorkspaceID:   "ws-marketing",
		}
		store.activations[leftActivation.ID] = leftActivation
		store.activations[rightActivation.ID] = rightActivation

		err := service.Reconcile(testutil.Context(t))
		if !errors.Is(err, ErrAgentConflict) {
			t.Fatalf("Reconcile() error = %v, want ErrAgentConflict", err)
		}
		if got := len(store.applied); got != 0 {
			t.Fatalf("len(applied plans) = %d, want 0 after validation failure", got)
		}
	})
}

func TestServiceReadMethodsValidateInputs(t *testing.T) {
	t.Parallel()

	var nilService *Service
	if _, err := nilService.NetworkSettings(testutil.Context(t)); err == nil {
		t.Fatal("nil NetworkSettings() error = nil, want failure")
	}
	service := newMarketingService(newMemoryStore())
	var nilCtx context.Context
	if _, err := service.NetworkSettings(nilCtx); err == nil {
		t.Fatal("NetworkSettings(nil) error = nil, want failure")
	}
	if _, err := service.GetActivation(testutil.Context(t), ""); err == nil {
		t.Fatal("GetActivation(empty) error = nil, want failure")
	}
	if _, err := service.ListActivations(nilCtx); err == nil {
		t.Fatal("ListActivations(nil) error = nil, want failure")
	}
}

func TestServiceListActivationsWrapsErrorsWithContext(t *testing.T) {
	t.Parallel()

	t.Run("ShouldWrapBundleResourceListError", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		store.activations["activation-1"] = Activation{ID: "activation-1"}
		resourceErr := errors.New("resource store offline")
		store.listBundleResourcesHook = func() ([]resources.Record[BundleResourceSpec], error) {
			return nil, resourceErr
		}
		service := newMarketingService(store, WithLogger(discardBundleTestLogger()))

		_, err := service.ListActivations(testutil.Context(t))
		if err == nil {
			t.Fatal("ListActivations() error = nil, want non-nil")
		}
		if !errors.Is(err, resourceErr) {
			t.Fatalf("ListActivations() error = %v, want wrapped %v", err, resourceErr)
		}
		if !strings.Contains(err.Error(), "list bundle resources for activations") {
			t.Fatalf("ListActivations() error = %v, want wrapped bundle resource context", err)
		}
	})

	t.Run("ShouldWrapActivationInventoryError", func(t *testing.T) {
		t.Parallel()

		store := newMemoryStore()
		service := newMarketingService(store, WithLogger(discardBundleTestLogger()))

		preview, err := service.Activate(testutil.Context(t), ActivateRequest{
			ExtensionName: "marketing-team",
			BundleName:    "marketing",
			ProfileName:   "default",
			Scope:         ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("Activate() error = %v", err)
		}

		inventoryErr := errors.New("inventory unavailable")
		store.listBundleInventoryHook = func(activationID string) ([]InventoryItem, error) {
			if activationID != preview.Activation.ID {
				t.Fatalf("ListBundleActivationInventory() id = %q, want %q", activationID, preview.Activation.ID)
			}
			return nil, inventoryErr
		}

		_, err = service.ListActivations(testutil.Context(t))
		if !errors.Is(err, inventoryErr) {
			t.Fatalf("ListActivations() error = %v, want %v", err, inventoryErr)
		}
		if !strings.Contains(err.Error(), `list activation inventory for "`+preview.Activation.ID+`"`) {
			t.Fatalf("ListActivations() error = %v, want wrapped inventory context", err)
		}
	})
}

func TestServiceListActivationsReturnsEmptyWithoutLoadingBundleResources(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	store.listBundleResourcesHook = func() ([]resources.Record[BundleResourceSpec], error) {
		return nil, errors.New("bundle resources should not be loaded for empty activations")
	}
	service := newMarketingService(store, WithLogger(discardBundleTestLogger()))

	got, err := service.ListActivations(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListActivations() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(ListActivations()) = %d, want 0", len(got))
	}
	if got == nil {
		t.Fatal("ListActivations() = nil, want empty slice")
	}
}

func TestFindBundleResourceRecordIndexedNormalizesLookupKeys(t *testing.T) {
	t.Parallel()

	record := resources.Record[BundleResourceSpec]{
		ID: BundleResourceID("Marketing-Team", "Launch"),
		Spec: BundleResourceSpec{
			ExtensionName: " Marketing-Team ",
			Bundle: extensionpkg.BundleSpec{
				Name: " Launch ",
			},
		},
	}
	lookup := newBundleRecordLookup([]resources.Record[BundleResourceSpec]{record})

	got, ok := findBundleResourceRecordIndexed(lookup, "marketing-team", "launch")
	if !ok {
		t.Fatal("findBundleResourceRecordIndexed() ok = false, want true")
	}
	if got.ID != record.ID {
		t.Fatalf("findBundleResourceRecordIndexed().ID = %q, want %q", got.ID, record.ID)
	}

	_, ok = findBundleResourceRecordIndexed(lookup, "", "launch")
	if ok {
		t.Fatal("findBundleResourceRecordIndexed() ok = true for empty extension name, want false")
	}
}

func TestFindBundleResourceRecordIndexedPreservesFirstNormalizedRecord(t *testing.T) {
	t.Parallel()

	t.Run("Should keep the first record for duplicate normalized keys", func(t *testing.T) {
		t.Parallel()

		first := resources.Record[BundleResourceSpec]{
			ID: BundleResourceID("Marketing-Team", "Launch"),
			Spec: BundleResourceSpec{
				ExtensionName: "Marketing-Team",
				Bundle: extensionpkg.BundleSpec{
					Name: "Launch",
				},
			},
		}
		second := resources.Record[BundleResourceSpec]{
			ID: BundleResourceID(" marketing-team ", " launch "),
			Spec: BundleResourceSpec{
				ExtensionName: " marketing-team ",
				Bundle: extensionpkg.BundleSpec{
					Name: " launch ",
				},
			},
		}
		lookup := newBundleRecordLookup([]resources.Record[BundleResourceSpec]{first, second})

		got, ok := findBundleResourceRecordIndexed(lookup, "marketing-team", "launch")
		if !ok {
			t.Fatal("findBundleResourceRecordIndexed() ok = false, want true")
		}
		if got.ID != first.ID {
			t.Fatalf("findBundleResourceRecordIndexed().ID = %q, want %q", got.ID, first.ID)
		}
	})
}

func TestServiceDeactivateCleansUpManagedResources(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	service := newMarketingService(store)

	preview, err := service.Activate(testutil.Context(t), ActivateRequest{
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
	})
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	if err := service.Deactivate(testutil.Context(t), preview.Activation.ID); err != nil {
		t.Fatalf("Deactivate() error = %v", err)
	}

	if got := len(store.activations); got != 0 {
		t.Fatalf("len(store.activations) = %d, want 0", got)
	}
	if got := len(store.inventory); got != 0 {
		t.Fatalf("len(store.inventory) = %d, want 0", got)
	}
	if got, want := len(store.applied), 2; got != want {
		t.Fatalf("len(applied plans) = %d, want %d", got, want)
	}
	last := store.applied[len(store.applied)-1]
	if got := len(last.activeActivationIDs); got != 0 {
		t.Fatalf("len(last.activeActivationIDs) = %d, want 0", got)
	}
	if got := len(last.desiredLayouts); got != 0 {
		t.Fatalf("len(last.desiredLayouts) after deactivate = %d, want 0", got)
	}
	if got := len(last.desiredAgents); got != 0 {
		t.Fatalf("len(last.desiredAgents) after deactivate = %d, want 0", got)
	}
	if got := len(last.desiredSouls); got != 0 {
		t.Fatalf("len(last.desiredSouls) after deactivate = %d, want 0", got)
	}
	if got := len(last.desiredHeartbeats); got != 0 {
		t.Fatalf("len(last.desiredHeartbeats) after deactivate = %d, want 0", got)
	}
	if got := len(last.desiredJobs); got != 0 {
		t.Fatalf("len(last.desiredJobs) after deactivate = %d, want 0", got)
	}
	if got := len(last.desiredTriggers); got != 0 {
		t.Fatalf("len(last.desiredTriggers) after deactivate = %d, want 0", got)
	}
	if got := len(last.desiredBridges); got != 0 {
		t.Fatalf("len(last.desiredBridges) after deactivate = %d, want 0", got)
	}
}

func TestServiceDeactivateReturnsRollbackFailureWhenRestoreFails(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	service := newMarketingService(store)

	preview, err := service.Activate(testutil.Context(t), ActivateRequest{
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
	})
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	syncErr := errors.New("sync failed")
	recreateErr := errors.New("recreate failed")
	store.applyErr = syncErr
	store.createBundleActivationHook = func(activation Activation) error {
		if activation.ID == preview.Activation.ID {
			return recreateErr
		}
		return nil
	}

	err = service.Deactivate(testutil.Context(t), preview.Activation.ID)
	if err == nil {
		t.Fatal("Deactivate() error = nil, want reconcile + rollback failure")
	}
	if !errors.Is(err, syncErr) {
		t.Fatalf("Deactivate() error = %v, want sync failure", err)
	}
	if !errors.Is(err, recreateErr) {
		t.Fatalf("Deactivate() error = %v, want rollback failure", err)
	}
}

func TestServiceReconcileReturnsBeforeSyncWhenActivationResolutionFails(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	goodActivation := Activation{
		ID:            stableID("act", "marketing-team", "marketing", "default", string(ScopeGlobal), ""),
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
	}
	badActivation := Activation{
		ID:            stableID("act", "broken-team", "marketing", "default", string(ScopeGlobal), ""),
		ExtensionName: "broken-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
	}
	store.activations[goodActivation.ID] = goodActivation
	store.activations[badActivation.ID] = badActivation
	marketing := newMarketingExtension()
	store.bundles = []resources.Record[BundleResourceSpec]{{
		Kind:  BundleResourceKind,
		ID:    BundleResourceID(marketing.Info.Name, marketing.Bundles[0].Name),
		Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		Spec: BundleResourceSpec{
			ExtensionName:              marketing.Info.Name,
			Bundle:                     marketing.Bundles[0],
			OwnerBridgePlatform:        marketing.Manifest.Bridge.Platform,
			OwnerProvidesBridgeAdapter: true,
		},
	}}
	service := NewService(
		store,
		staticExtensionLister{items: []extensionpkg.ExtensionInfo{{Name: "marketing-team"}, {Name: "broken-team"}}},
		func(_ context.Context, name string) (*extensionpkg.Extension, error) {
			switch name {
			case "marketing-team":
				return marketing, nil
			default:
				return nil, extensionpkg.ErrExtensionNotFound
			}
		},
	)

	err := service.Reconcile(testutil.Context(t))
	if !errors.Is(err, ErrBundleNotFound) {
		t.Fatalf("Reconcile() error = %v, want ErrBundleNotFound", err)
	}
	if got := len(store.applied); got != 0 {
		t.Fatalf("len(applied plans) = %d, want 0 after failed resolve", got)
	}
}

func TestServicePreviewRejectsWebhookTriggers(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	ext := &extensionpkg.Extension{
		Info: extensionpkg.ExtensionInfo{Name: "webhook-team"},
		Bundles: []extensionpkg.BundleSpec{{
			Name: "webhook",
			Profiles: []extensionpkg.BundleProfile{{
				Name: "default",
				Triggers: []extensionpkg.BundleTrigger{{
					Name:      "webhook",
					AgentName: "planner",
					Prompt:    "Handle webhook",
					Event:     "webhook",
					Enabled:   true,
					Retry:     automationpkg.DefaultRetryConfig(),
					FireLimit: automationpkg.DefaultFireLimitConfig(),
				}},
			}},
		}},
	}
	store.bundles = []resources.Record[BundleResourceSpec]{{
		Kind:  BundleResourceKind,
		ID:    BundleResourceID(ext.Info.Name, ext.Bundles[0].Name),
		Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		Spec: BundleResourceSpec{
			ExtensionName: ext.Info.Name,
			Bundle:        ext.Bundles[0],
		},
	}}
	service := NewService(
		store,
		staticExtensionLister{items: []extensionpkg.ExtensionInfo{{Name: ext.Info.Name}}},
		func(_ context.Context, name string) (*extensionpkg.Extension, error) {
			if name != ext.Info.Name {
				return nil, extensionpkg.ErrExtensionNotFound
			}
			return ext, nil
		},
	)

	_, err := service.PreviewActivation(testutil.Context(t), ActivateRequest{
		ExtensionName: ext.Info.Name,
		BundleName:    "webhook",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
	})
	if !errors.Is(err, ErrWebhookUnsupported) {
		t.Fatalf("PreviewActivation() error = %v, want ErrWebhookUnsupported", err)
	}
}

func TestServiceMaterializesExternalBridgeProviderPlatform(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	consumer := &extensionpkg.Extension{
		Info: extensionpkg.ExtensionInfo{Name: "consumer-team"},
		Manifest: &extensionpkg.Manifest{
			Name: "consumer-team",
		},
		Bundles: []extensionpkg.BundleSpec{{
			Name: "consumer",
			Profiles: []extensionpkg.BundleProfile{{
				Name: "default",
				Bridges: []extensionpkg.BundleBridgePreset{{
					Name:          "external",
					ExtensionName: "provider-team",
					DisplayName:   "Provider Bridge",
					RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
				}},
			}},
		}},
	}
	provider := &extensionpkg.Extension{
		Info: extensionpkg.ExtensionInfo{Name: "provider-team"},
		Manifest: &extensionpkg.Manifest{
			Name: "provider-team",
			Bridge: extensionpkg.BridgeConfig{
				Platform:    "slack",
				DisplayName: "Slack",
			},
		},
	}
	store.bundles = []resources.Record[BundleResourceSpec]{{
		Kind:  BundleResourceKind,
		ID:    BundleResourceID(consumer.Info.Name, consumer.Bundles[0].Name),
		Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		Spec: BundleResourceSpec{
			ExtensionName: consumer.Info.Name,
			Bundle:        consumer.Bundles[0],
		},
	}}
	service := NewService(
		store,
		staticExtensionLister{items: []extensionpkg.ExtensionInfo{{Name: consumer.Info.Name}}},
		func(_ context.Context, name string) (*extensionpkg.Extension, error) {
			switch name {
			case consumer.Info.Name:
				return consumer, nil
			case provider.Info.Name:
				return provider, nil
			default:
				return nil, extensionpkg.ErrExtensionNotFound
			}
		},
	)

	preview, err := service.PreviewActivation(testutil.Context(t), ActivateRequest{
		ExtensionName: consumer.Info.Name,
		BundleName:    "consumer",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
	})
	if err != nil {
		t.Fatalf("PreviewActivation() error = %v", err)
	}
	if got, want := len(preview.Inventory), 1; got != want {
		t.Fatalf("len(preview.Inventory) = %d, want %d", got, want)
	}
	plan := store.applied
	if len(plan) != 0 {
		t.Fatalf("preview applied plans = %d, want 0", len(plan))
	}
}

func TestServiceActivateWorkspaceScopedResources(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	resolver := memoryWorkspaceResolver{
		resolveFn: func(_ context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
			return workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID:      "ws-marketing",
					RootDir: filepath.Join(t.TempDir(), "workspace"),
					Name:    idOrPath,
				},
			}, nil
		},
	}
	service := newMarketingService(store, WithWorkspaceResolver(resolver))

	preview, err := service.Activate(testutil.Context(t), ActivateRequest{
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeWorkspace,
		Workspace:     "marketing-workspace",
	})
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	if got, want := preview.Activation.WorkspaceID, "ws-marketing"; got != want {
		t.Fatalf("Activation.WorkspaceID = %q, want %q", got, want)
	}
	plan := store.applied[0]
	if got, want := plan.desiredLayouts[0].Scope.Kind,
		resources.ResourceScopeKindWorkspace; got != want {
		t.Fatalf("layout scope kind = %q, want %q", got, want)
	}
	if got, want := plan.desiredLayouts[0].Scope.ID, "ws-marketing"; got != want {
		t.Fatalf("layout scope id = %q, want %q", got, want)
	}
	if plan.desiredLayouts[0].ID != plan.desiredLayouts[0].Spec.ID {
		t.Fatalf(
			"layout record/spec IDs = %q/%q, want identical",
			plan.desiredLayouts[0].ID,
			plan.desiredLayouts[0].Spec.ID,
		)
	}
	if got, want := plan.desiredAgents[0].Scope.Kind, resources.ResourceScopeKindWorkspace; got != want {
		t.Fatalf("agent scope kind = %q, want %q", got, want)
	}
	if got, want := plan.desiredAgents[0].Scope.ID, "ws-marketing"; got != want {
		t.Fatalf("agent scope id = %q, want %q", got, want)
	}
	if got, want := plan.desiredJobs[0].Scope, automationpkg.AutomationScopeWorkspace; got != want {
		t.Fatalf("job scope = %q, want %q", got, want)
	}
	if got, want := plan.desiredJobs[0].WorkspaceID, "ws-marketing"; got != want {
		t.Fatalf("job workspace = %q, want %q", got, want)
	}

	storedBridge := plan.desiredBridges[0]
	if got, want := storedBridge.Scope, bridgepkg.ScopeWorkspace; got != want {
		t.Fatalf("bridge scope = %q, want %q", got, want)
	}
	if got, want := storedBridge.WorkspaceID, "ws-marketing"; got != want {
		t.Fatalf("bridge workspace = %q, want %q", got, want)
	}
}

func TestServiceActivateWorkspacePathUsesResolveOrRegister(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	called := false
	resolver := memoryWorkspaceResolver{
		resolveFn: func(_ context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
			if idOrPath != "ws-path" {
				return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
			}
			return workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID:      "ws-path",
					RootDir: filepath.Join(t.TempDir(), "workspace"),
					Name:    "path-workspace",
				},
			}, nil
		},
		resolveOrRegisterFn: func(_ context.Context, path string) (workspacepkg.ResolvedWorkspace, error) {
			called = true
			return workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID:      "ws-path",
					RootDir: path,
					Name:    "path-workspace",
				},
			}, nil
		},
	}
	service := newMarketingService(store, WithWorkspaceResolver(resolver))

	preview, err := service.Activate(testutil.Context(t), ActivateRequest{
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeWorkspace,
		Workspace:     filepath.Join(t.TempDir(), "workspace"),
	})
	if err != nil {
		t.Fatalf("Activate(path workspace) error = %v", err)
	}
	if !called {
		t.Fatal("ResolveOrRegister was not called for path-like workspace ref")
	}
	if got, want := preview.Activation.WorkspaceID, "ws-path"; got != want {
		t.Fatalf("WorkspaceID = %q, want %q", got, want)
	}
}
