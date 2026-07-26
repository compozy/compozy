package bundles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"slices"
	"strings"
	"sync"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	modelpkg "github.com/compozy/agh/internal/bundles/model"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

var (
	ErrActivationNotFound     = modelpkg.ErrActivationNotFound
	ErrBundleNotFound         = errors.New("bundles: bundle not found")
	ErrProfileNotFound        = errors.New("bundles: profile not found")
	ErrWebhookUnsupported     = errors.New("bundles: bundle webhook triggers are not supported")
	ErrAgentConflict          = errors.New("bundles: bundle agent conflicts with an existing agent")
	ErrAgentReferenceNotFound = errors.New("bundles: bundle automation references an unavailable agent")
)

type Scope = modelpkg.Scope

const (
	ScopeGlobal    = modelpkg.ScopeGlobal
	ScopeWorkspace = modelpkg.ScopeWorkspace
)

type Activation = modelpkg.Activation
type InventoryItem = modelpkg.InventoryItem

type DeclaredChannel struct {
	ActivationID  string
	ExtensionName string
	BundleName    string
	ProfileName   string
	WorkspaceID   string
	Name          string
	Description   string
	Primary       bool
}

type NetworkSettings struct {
	DeclaredChannels []DeclaredChannel
}

type CatalogEntry struct {
	ExtensionName string
	Bundle        extensionpkg.BundleSpec
}

type ActivationPreview struct {
	Activation Activation
	Bundle     extensionpkg.BundleSpec
	Profile    extensionpkg.BundleProfile
	Inventory  []InventoryItem
	SpecDrift  bool
}

type Store interface {
	CreateBundleActivation(ctx context.Context, activation Activation) error
	UpdateBundleActivation(ctx context.Context, activation Activation) error
	DeleteBundleActivation(ctx context.Context, id string) error
	GetBundleActivation(ctx context.Context, id string) (Activation, error)
	ListBundleActivations(ctx context.Context) ([]Activation, error)
	ListBundleActivationInventory(ctx context.Context, activationID string) ([]InventoryItem, error)
	ListBundleResources(ctx context.Context) ([]resources.Record[BundleResourceSpec], error)
	ListAgentResources(ctx context.Context) ([]resources.Record[aghconfig.AgentDef], error)
	ApplyBundleActivationResources(ctx context.Context, plan BundleActivationResourcePlan) error
}

type ExtensionInfoLister interface {
	List() ([]extensionpkg.ExtensionInfo, error)
}

type ExtensionLoader func(ctx context.Context, name string) (*extensionpkg.Extension, error)

type Service struct {
	store             Store
	extensions        ExtensionInfoLister
	loadExtension     ExtensionLoader
	workspaceResolver workspacepkg.RuntimeResolver
	logger            *slog.Logger
	now               func() time.Time

	opMu       sync.Mutex
	settingsMu sync.RWMutex
	settings   NetworkSettings
}

type Option func(*Service)

func WithWorkspaceResolver(resolver workspacepkg.RuntimeResolver) Option {
	return func(s *Service) {
		s.workspaceResolver = resolver
	}
}

func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		s.now = now
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func NewService(store Store, extensions ExtensionInfoLister, loadExtension ExtensionLoader, opts ...Option) *Service {
	if store == nil || extensions == nil || loadExtension == nil {
		return nil
	}

	service := &Service{
		store:         store,
		extensions:    extensions,
		loadExtension: loadExtension,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

type ActivateRequest struct {
	ExtensionName             string
	BundleName                string
	ProfileName               string
	Scope                     Scope
	Workspace                 string
	ConfirmNetworkRequirement bool
}

type UpdateActivationRequest struct {
	ID                        string
	ExpectedVersion           int64
	ConfirmNetworkRequirement bool
}

func (s *Service) Catalog(ctx context.Context) ([]CatalogEntry, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}

	records, err := s.store.ListBundleResources(ctx)
	if err != nil {
		return nil, err
	}

	entries := make([]CatalogEntry, 0, len(records))
	for _, record := range records {
		entries = append(entries, CatalogEntry{
			ExtensionName: strings.TrimSpace(record.Spec.ExtensionName),
			Bundle:        cloneBundleSpec(record.Spec.Bundle),
		})
	}
	slices.SortFunc(entries, func(left, right CatalogEntry) int {
		if cmp := strings.Compare(left.ExtensionName, right.ExtensionName); cmp != 0 {
			return cmp
		}
		return strings.Compare(left.Bundle.Name, right.Bundle.Name)
	})
	return entries, nil
}

func (s *Service) NetworkSettings(ctx context.Context) (NetworkSettings, error) {
	if err := s.checkReady(ctx); err != nil {
		return NetworkSettings{}, err
	}

	s.settingsMu.RLock()
	settings := cloneNetworkSettings(s.settings)
	s.settingsMu.RUnlock()
	return settings, nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	if err := s.checkReady(ctx); err != nil {
		return err
	}

	s.opMu.Lock()
	defer s.opMu.Unlock()

	return s.reconcileLocked(ctx)
}

func (s *Service) rollbackActivationAndReconcileLocked(
	ctx context.Context,
	reconcileErr error,
	rollback func(context.Context) error,
	action string,
	activationID string,
) error {
	rollbackCtx := context.WithoutCancel(ctx)
	rollbackErr := rollback(rollbackCtx)
	if rollbackErr == nil {
		if compensateErr := s.reconcileLocked(rollbackCtx); compensateErr != nil {
			rollbackErr = fmt.Errorf("compensate bundle activation resources: %w", compensateErr)
		}
	}
	return s.joinRollbackFailure(ctx, reconcileErr, rollbackErr, action, activationID)
}

// reconcileLocked recomputes and syncs bundle-managed resources.
func (s *Service) reconcileLocked(ctx context.Context) error {
	activations, err := s.store.ListBundleActivations(ctx)
	if err != nil {
		return err
	}
	state, err := s.collectDesiredState(ctx, activations)
	if err != nil {
		return err
	}
	changed, err := s.reconcileNetworkRequirementDigests(ctx, activations)
	if err != nil {
		return err
	}
	if changed {
		activations, err = s.store.ListBundleActivations(ctx)
		if err != nil {
			return err
		}
		state, err = s.collectDesiredState(ctx, activations)
		if err != nil {
			return err
		}
	}

	owners := ownedResourceMaps(state.inventoryByActivation)
	if syncErr := s.store.ApplyBundleActivationResources(ctx, BundleActivationResourcePlan{
		activeActivationIDs: cloneStringSet(state.activeActivationIDs),
		desiredLayouts:      cloneOwnedLayoutResources(state.desiredLayouts),
		desiredAgents:       cloneOwnedAgentResources(state.desiredAgents),
		desiredSouls:        cloneOwnedSoulResources(state.desiredSouls),
		desiredHeartbeats:   cloneOwnedHeartbeatResources(state.desiredHeartbeats),
		desiredJobs:         cloneJobsForBundle(state.desiredJobs),
		desiredTriggers:     cloneTriggersForBundle(state.desiredTriggers),
		desiredBridges:      cloneBridgeInstancesForBundle(state.desiredBridges),
		layoutOwners:        owners.layouts,
		agentOwners:         owners.agents,
		soulOwners:          owners.souls,
		heartbeatOwners:     owners.heartbeats,
		jobOwners:           owners.jobs,
		triggerOwners:       owners.triggers,
		bridgeOwners:        owners.bridges,
	}); syncErr != nil {
		return syncErr
	}
	s.applyNetworkSettings(state.declaredChannels)
	return nil
}

type resolvedActivation struct {
	activation      Activation
	bundleRecord    resources.Record[BundleResourceSpec]
	bundle          extensionpkg.BundleSpec
	profile         extensionpkg.BundleProfile
	specContentHash string
	layouts         []ownedLayoutResource
	agents          []ownedAgentResource
	souls           []ownedSoulResource
	heartbeats      []ownedHeartbeatResource
	jobs            []automationpkg.Job
	triggers        []automationpkg.Trigger
	bridges         []bridgepkg.BridgeInstance
	channels        []DeclaredChannel
	inventory       []InventoryItem
}

type activationDefinition struct {
	bundleRecord    resources.Record[BundleResourceSpec]
	bundle          extensionpkg.BundleSpec
	profile         extensionpkg.BundleProfile
	specContentHash string
}

type reconcileState struct {
	activeActivationIDs   map[string]struct{}
	desiredLayouts        []ownedLayoutResource
	desiredAgents         []ownedAgentResource
	desiredSouls          []ownedSoulResource
	desiredHeartbeats     []ownedHeartbeatResource
	desiredJobs           []automationpkg.Job
	desiredTriggers       []automationpkg.Trigger
	desiredBridges        []bridgepkg.BridgeInstance
	inventoryByActivation map[string][]InventoryItem
	declaredChannels      []DeclaredChannel
}

type materializedActivationResources struct {
	layouts    []ownedLayoutResource
	agents     []ownedAgentResource
	souls      []ownedSoulResource
	heartbeats []ownedHeartbeatResource
	jobs       []automationpkg.Job
	triggers   []automationpkg.Trigger
	bridges    []bridgepkg.BridgeInstance
	inventory  []InventoryItem
}

type ownedAgentResource struct {
	ID    string
	Scope resources.ResourceScope
	Spec  aghconfig.AgentDef
}

type ownedSoulResource struct {
	ID    string
	Scope resources.ResourceScope
	Spec  soul.ResourceSpec
}

type ownedHeartbeatResource struct {
	ID    string
	Scope resources.ResourceScope
	Spec  heartbeat.ResourceSpec
}

type workspaceResolutionMode uint8

const (
	workspaceResolutionReadOnly workspaceResolutionMode = iota
	workspaceResolutionRegisterPaths
)
