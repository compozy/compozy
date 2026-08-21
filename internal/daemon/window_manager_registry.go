package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// windowManagerWorkspaceLister enumerates registered workspaces for cross-workspace
// profile sweeps.
type windowManagerWorkspaceLister interface {
	List(ctx context.Context) ([]workspacepkg.Workspace, error)
}

// windowManagerProfileRuntime is one profile's window manager and the repository
// that keys its aggregates.
type windowManagerProfileRuntime struct {
	manager    *windowmanager.Manager
	repository *windowManagerRepository
}

// windowManagerRegistry owns one window manager per profile.
//
// Desks are per-profile working state (US-026), and so is everything derived from
// them: the durable aggregate, the live subscription fan-out, attached clients, and
// pending stack activations. Partitioning at the composition root keeps that whole
// set separate by construction — two profiles share the underlying store and
// nothing else, so neither can ever observe the other's arrangement.
type windowManagerRegistry struct {
	engine     clientstate.Service
	authorizer windowmanager.WorkspaceResolver
	layouts    windowmanager.LayoutResourceRegistry
	workspaces windowManagerWorkspaceLister
	options    []windowmanager.Option
	logger     *slog.Logger

	mu       sync.Mutex
	closed   bool
	defaults windowmanager.Config
	runtimes map[string]*windowManagerProfileRuntime
	deleted  map[string]struct{}
	claims   map[clientClaimKey]*clientClaim
}

var _ core.WindowManagerProvider = (*windowManagerRegistry)(nil)

func newWindowManagerRegistry(
	engine clientstate.Service,
	authorizer windowmanager.WorkspaceResolver,
	layouts windowmanager.LayoutResourceRegistry,
	workspaces windowManagerWorkspaceLister,
	defaults windowmanager.Config,
	logger *slog.Logger,
	options ...windowmanager.Option,
) (*windowManagerRegistry, error) {
	if engine == nil {
		return nil, errors.New("daemon: window-manager client-state service is required")
	}
	if authorizer == nil {
		return nil, errors.New("daemon: window-manager workspace authorizer is required")
	}
	if workspaces == nil {
		return nil, errors.New("daemon: window-manager workspace lister is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &windowManagerRegistry{
		engine:     engine,
		authorizer: authorizer,
		layouts:    layouts,
		workspaces: workspaces,
		options:    options,
		logger:     logger,
		defaults:   defaults,
		runtimes:   make(map[string]*windowManagerProfileRuntime),
		deleted:    make(map[string]struct{}),
		claims:     make(map[clientClaimKey]*clientClaim),
	}, nil
}

// For returns the window manager that owns one profile's desks, creating it on
// first use. The profile is always resolved by the caller's boundary — an absent
// profile is a programming error, never an implicit default.
func (r *windowManagerRegistry) For(profileID string) (*windowmanager.Manager, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil, errors.New("daemon: window-manager profile is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, windowmanager.ErrClosed
	}
	// Profile ids are ULIDs and never reused, so a deleted profile stays refused:
	// nothing may rebuild a runtime whose desks the delete flow is removing.
	if _, deleted := r.deleted[profileID]; deleted {
		return nil, errWindowManagerProfileDeleted
	}
	if runtime, exists := r.runtimes[profileID]; exists {
		return runtime.manager, nil
	}
	runtime, err := r.buildLocked(profileID)
	if err != nil {
		return nil, err
	}
	r.runtimes[profileID] = runtime
	return runtime.manager, nil
}

// WindowManagerFor adapts the registry to the transport port.
func (r *windowManagerRegistry) WindowManagerFor(profileID string) (core.WindowManagerService, error) {
	return r.For(profileID)
}

func (r *windowManagerRegistry) buildLocked(profileID string) (*windowManagerProfileRuntime, error) {
	repository, err := newWindowManagerRepository(
		r.engine,
		profileID,
		withWindowManagerRepositoryLogger(r.logger),
	)
	if err != nil {
		return nil, err
	}
	manager, err := windowmanager.NewService(
		repository,
		r.authorizer,
		r.layouts,
		r.defaults,
		r.options...,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create window manager for profile %q: %w", profileID, err)
	}
	return &windowManagerProfileRuntime{manager: manager, repository: repository}, nil
}

// UpdateDefaults applies runtime configuration to every live profile and to the
// profiles that enter later.
func (r *windowManagerRegistry) UpdateDefaults(defaults windowmanager.Config) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return windowmanager.ErrClosed
	}
	r.defaults = defaults
	live := r.liveRuntimesLocked()
	r.mu.Unlock()
	var errs []error
	for _, runtime := range live {
		if err := runtime.manager.UpdateDefaults(defaults); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ForgetWorkspace tears down transient state for a removed workspace in every profile.
func (r *windowManagerRegistry) ForgetWorkspace(workspaceID windowmanager.WorkspaceID) {
	for _, runtime := range r.liveRuntimes() {
		runtime.manager.ForgetWorkspace(workspaceID)
		runtime.repository.forgetWorkspace(workspaceID)
	}
}

// Close terminates every profile's window manager.
func (r *windowManagerRegistry) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	live := r.liveRuntimesLocked()
	r.runtimes = make(map[string]*windowManagerProfileRuntime)
	r.mu.Unlock()
	var errs []error
	for _, runtime := range live {
		if err := runtime.manager.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (r *windowManagerRegistry) liveRuntimes() []*windowManagerProfileRuntime {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.liveRuntimesLocked()
}

func (r *windowManagerRegistry) liveRuntimesLocked() []*windowManagerProfileRuntime {
	profileIDs := make([]string, 0, len(r.runtimes))
	for profileID := range r.runtimes {
		profileIDs = append(profileIDs, profileID)
	}
	sort.Strings(profileIDs)
	live := make([]*windowManagerProfileRuntime, 0, len(profileIDs))
	for _, profileID := range profileIDs {
		live = append(live, r.runtimes[profileID])
	}
	return live
}

// CountDesktopPartitions reports how many stored arrangements one profile owns
// across every registered workspace. This is the delete preview's number.
func (r *windowManagerRegistry) CountDesktopPartitions(ctx context.Context, profileID string) (int, error) {
	partitions, err := r.desktopPartitions(ctx, profileID)
	if err != nil {
		return 0, err
	}
	return len(partitions), nil
}

// PurgeDesktopPartitions removes every stored arrangement one profile owns.
//
// The runtime is retired *before* anything is enumerated: a command already in
// flight would otherwise commit between the enumeration and the delete and leave
// the profile with an arrangement nobody can reach. Retiring first — tombstone,
// seal, close — makes the removal final without waiting on in-flight callers.
// Idempotent, so forward-only lifecycle recovery can re-run it after a crash.
func (r *windowManagerRegistry) PurgeDesktopPartitions(ctx context.Context, profileID string) error {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return errors.New("daemon: window-manager profile is required")
	}
	r.retireRuntime(profileID)
	partitions, err := r.desktopPartitions(ctx, profileID)
	if err != nil {
		return err
	}
	for _, partition := range partitions {
		if _, err := r.engine.Apply(
			ctx,
			partition.workspaceID,
			windowManagerStateDomain,
			[]clientstate.Op{{
				Kind:  clientstate.OpDelete,
				Key:   windowManagerSnapshotKey(partition.profileID),
				IfRev: partition.revision,
			}},
			clientstate.ApplyOptions{Origin: "window-manager.profile.delete"},
		); err != nil && !errors.Is(err, clientstate.ErrNotFound) &&
			!errors.Is(err, clientstate.ErrWorkspaceNotFound) {
			return fmt.Errorf(
				"daemon: remove desktop partition for profile %q in workspace %q: %w",
				profileID,
				partition.workspaceID,
				err,
			)
		}
	}
	return nil
}

type windowManagerDesktopPartition struct {
	workspaceID clientstate.WorkspaceID
	profileID   string
	revision    uint64
}

func (r *windowManagerRegistry) desktopPartitions(
	ctx context.Context,
	profileID string,
) ([]windowManagerDesktopPartition, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil, errors.New("daemon: window-manager profile is required")
	}
	workspaces, err := r.workspaces.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list workspaces for desktop partitions: %w", err)
	}
	partitions := make([]windowManagerDesktopPartition, 0, len(workspaces))
	for index := range workspaces {
		workspaceID := clientstate.WorkspaceID(workspaces[index].ID)
		entries, err := r.engine.List(ctx, workspaceID, windowManagerStateDomain)
		if errors.Is(err, clientstate.ErrWorkspaceNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf(
				"daemon: list desktop partitions in workspace %q: %w",
				workspaceID,
				err,
			)
		}
		for _, entry := range entries {
			owner, partitioned := windowManagerSnapshotProfile(entry.Key)
			if !partitioned || owner != profileID {
				continue
			}
			partitions = append(partitions, windowManagerDesktopPartition{
				workspaceID: workspaceID, profileID: owner, revision: entry.Rev,
			})
		}
	}
	return partitions, nil
}

// retireRuntime permanently closes one profile's window manager and refuses every
// later durable write for it.
func (r *windowManagerRegistry) retireRuntime(profileID string) {
	r.mu.Lock()
	r.deleted[profileID] = struct{}{}
	runtime, exists := r.runtimes[profileID]
	if exists {
		delete(r.runtimes, profileID)
	}
	r.mu.Unlock()
	if !exists {
		return
	}
	// Sealed before the close so the coalescer's closing flush cannot land either;
	// it reads the partition as absent and drops what it was holding.
	runtime.repository.seal()
	if err := runtime.manager.Close(); err != nil {
		r.logger.Warn(
			"closing window manager for a deleted profile failed",
			"profile_id", profileID,
			"error", err,
		)
	}
}
