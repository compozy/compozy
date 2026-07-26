package consolidation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// Service evaluates dream gates and coordinates lock-aware consolidation runs.
type Service interface {
	ShouldRun() (bool, error)
	Run(ctx context.Context, spawn memory.SessionSpawner, workspace string) error
}

// ServiceFactory constructs a consolidation service using memory package options.
type ServiceFactory func(opts ...memory.Option) Service

// SessionManager is the session lifecycle surface needed to spawn dream sessions.
type SessionManager interface {
	Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error)
	ListAll(ctx context.Context) ([]*session.Info, error)
	Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error)
	Stop(ctx context.Context, id string) error
}

// Runtime owns dream scheduling, trigger behavior, and session spawning.
type Runtime struct {
	enabled            func() bool
	service            Service
	spawner            memory.SessionSpawner
	logger             *slog.Logger
	interval           time.Duration
	lastConsolidatedAt func() (time.Time, error)

	mu      sync.Mutex
	checkCh chan checkRequest
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

type checkRequest struct {
	reason       string
	workspaceRef string
}

const (
	defaultSessionStopTimeout    = 10 * time.Second
	dreamGatesNotSatisfiedReason = "dream consolidation gates are not satisfied"
)

// SessionRoute is the resolved dream-session route supplied by the daemon role resolver.
type SessionRoute struct {
	Enabled         bool
	AgentName       string
	Provider        string
	Model           string
	ReasoningEffort string
	Fallbacks       []aghconfig.RoleFallback
	BeforeFallback  func(context.Context, int, aghconfig.RoleFallback) error
}

// SessionRouteResolver resolves the dream route for one effective workspace config.
type SessionRouteResolver func(ctx context.Context, workspaceID string) (SessionRoute, error)

// NewRuntime constructs a dream runtime that can be started by the daemon.
func NewRuntime(
	enabled func() bool,
	service Service,
	spawner memory.SessionSpawner,
	interval time.Duration,
	logger *slog.Logger,
	lastConsolidatedAt func() (time.Time, error),
) *Runtime {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runtime{
		enabled:            enabled,
		service:            service,
		spawner:            spawner,
		logger:             logger,
		interval:           interval,
		lastConsolidatedAt: lastConsolidatedAt,
	}
}

// Start launches the background dream check loop when the runtime is configured.
func (r *Runtime) Start(parent context.Context) {
	if r == nil {
		return
	}

	r.mu.Lock()
	if r.service == nil || r.spawner == nil || r.checkCh != nil {
		r.mu.Unlock()
		return
	}

	dreamCtx, cancel := context.WithCancel(parent)
	checkCh := make(chan checkRequest, 1)
	r.cancel = cancel
	r.checkCh = checkCh
	service := r.service
	spawner := r.spawner
	logger := r.logger
	interval := r.interval
	r.wg.Add(1)
	r.mu.Unlock()

	go func() {
		defer r.wg.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-dreamCtx.Done():
				return
			case <-ticker.C:
				r.runCheck(dreamCtx, logger, service, spawner, "ticker", "")
			case request := <-checkCh:
				r.runCheck(dreamCtx, logger, service, spawner, request.reason, request.workspaceRef)
			}
		}
	}()
}

// EnqueueCheck requests a background dream check without blocking.
func (r *Runtime) EnqueueCheck(reason string, workspaceRef string) {
	if r == nil {
		return
	}

	r.mu.Lock()
	checkCh := r.checkCh
	logger := r.logger
	r.mu.Unlock()

	if checkCh == nil {
		return
	}

	select {
	case checkCh <- checkRequest{
		reason:       strings.TrimSpace(reason),
		workspaceRef: strings.TrimSpace(workspaceRef),
	}:
	default:
		logger.Debug("daemon: dream check already queued", "reason", reason, "workspace_ref", workspaceRef)
	}
}

// Shutdown stops the background dream check loop.
func (r *Runtime) Shutdown() {
	if r == nil {
		return
	}

	r.mu.Lock()
	cancel := r.cancel
	r.cancel = nil
	r.checkCh = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
		r.wg.Wait()
	}
}

func (r *Runtime) runCheck(
	ctx context.Context,
	logger *slog.Logger,
	service Service,
	spawner memory.SessionSpawner,
	reason string,
	workspaceRef string,
) {
	if !r.Enabled() || service == nil || spawner == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}

	logger.Debug("daemon: evaluating dream consolidation gates", "reason", reason, "workspace_ref", workspaceRef)
	shouldRun, err := service.ShouldRun()
	if err != nil {
		logger.Warn(
			"daemon: dream gate evaluation failed",
			"reason",
			reason,
			"workspace_ref",
			workspaceRef,
			"error",
			err,
		)
		return
	}
	if !shouldRun {
		logger.Debug("daemon: dream consolidation skipped", "reason", reason, "workspace_ref", workspaceRef)
		return
	}

	logger.Info("daemon: starting dream consolidation", "reason", reason, "workspace_ref", workspaceRef)
	if err := service.Run(ctx, spawner, workspaceRef); err != nil {
		if errors.Is(err, memory.ErrLockUnavailable) {
			logger.Debug("daemon: dream consolidation already running", "reason", reason, "workspace_ref", workspaceRef)
			return
		}
		if errors.Is(err, memory.ErrDreamGateNotSatisfied) {
			logger.Debug("daemon: dream consolidation skipped", "reason", reason, "workspace_ref", workspaceRef)
			return
		}
		if errors.Is(err, memory.ErrDreamRoleDisabled) {
			logger.Debug(
				"daemon: dream consolidation skipped because the workspace role is disabled",
				"reason", reason,
				"workspace_ref", workspaceRef,
			)
			return
		}
		logger.Warn("daemon: dream consolidation failed", "reason", reason, "workspace_ref", workspaceRef, "error", err)
		return
	}
	logger.Info("daemon: dream consolidation completed", "reason", reason, "workspace_ref", workspaceRef)
}

// NewSessionSpawner creates dream sessions against one or more eligible workspaces.
func NewSessionSpawner(
	sessions SessionManager,
	resolver workspacepkg.RuntimeResolver,
	memoryEnabled bool,
	resolveRoute SessionRouteResolver,
) memory.SessionSpawner {
	if !memoryEnabled || sessions == nil || resolver == nil || resolveRoute == nil {
		return nil
	}

	return func(ctx context.Context, goal, prompt, workspace string, lastConsolidatedAt time.Time) error {
		workspaces, err := resolveWorkspaces(ctx, sessions, resolver, lastConsolidatedAt, workspace)
		if err != nil {
			return err
		}

		spawned := false
		for _, workspaceID := range workspaces {
			route, err := resolveRoute(ctx, workspaceID)
			if err != nil {
				return fmt.Errorf("daemon: resolve dream role for workspace %q: %w", workspaceID, err)
			}
			if !route.Enabled {
				continue
			}
			if err := spawnSession(
				ctx,
				sessions,
				route,
				goal,
				prompt,
				workspaceID,
				defaultSessionStopTimeout,
			); err != nil {
				return err
			}
			spawned = true
		}
		if !spawned {
			return memory.ErrDreamRoleDisabled
		}
		return nil
	}
}

func resolveWorkspaces(
	ctx context.Context,
	sessions SessionManager,
	resolver workspacepkg.RuntimeResolver,
	lastConsolidatedAt time.Time,
	explicitWorkspace string,
) ([]string, error) {
	if resolver == nil {
		return nil, errors.New("daemon: workspace resolver is required for dream consolidation")
	}

	if workspaceRef := strings.TrimSpace(explicitWorkspace); workspaceRef != "" {
		resolvedRef, err := resolveWorkspaceRef(ctx, resolver, workspaceRef)
		if err != nil {
			return nil, err
		}
		return []string{resolvedRef}, nil
	}

	infos, err := sessions.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list sessions for dream consolidation: %w", err)
	}

	type workspaceCandidate struct {
		id        string
		updatedAt time.Time
	}

	latestByWorkspace := make(map[string]time.Time, len(infos))
	for _, info := range infos {
		if info == nil || info.Type == session.SessionTypeDream {
			continue
		}

		workspaceID := strings.TrimSpace(info.WorkspaceID)
		if workspaceID == "" {
			continue
		}

		updatedAt := info.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = info.CreatedAt
		}
		if !lastConsolidatedAt.IsZero() && updatedAt.Before(lastConsolidatedAt) {
			continue
		}

		if latest, ok := latestByWorkspace[workspaceID]; !ok || updatedAt.After(latest) {
			latestByWorkspace[workspaceID] = updatedAt
		}
	}

	if len(latestByWorkspace) == 0 {
		return nil, errors.New("daemon: no recent workspaces available for dream consolidation")
	}

	candidates := make([]workspaceCandidate, 0, len(latestByWorkspace))
	for workspaceID, updatedAt := range latestByWorkspace {
		candidates = append(candidates, workspaceCandidate{id: workspaceID, updatedAt: updatedAt})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].updatedAt.Equal(candidates[j].updatedAt) {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].updatedAt.After(candidates[j].updatedAt)
	})

	workspaces := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		workspaces = append(workspaces, candidate.id)
	}
	return workspaces, nil
}

func resolveWorkspaceRef(
	ctx context.Context,
	resolver workspacepkg.RuntimeResolver,
	workspaceRef string,
) (string, error) {
	trimmedRef := strings.TrimSpace(workspaceRef)
	if trimmedRef == "" {
		return "", errors.New("daemon: dream workspace is required")
	}

	var (
		resolved workspacepkg.ResolvedWorkspace
		err      error
	)
	if isPathLikeWorkspaceRef(trimmedRef) {
		normalizedPath, normalizeErr := aghconfig.ResolvePath(trimmedRef)
		if normalizeErr != nil {
			return "", fmt.Errorf("daemon: resolve dream workspace %q: %w", workspaceRef, normalizeErr)
		}
		resolved, err = resolver.ResolveOrRegister(ctx, normalizedPath)
		if err != nil {
			return "", fmt.Errorf("daemon: resolve dream workspace %q: %w", workspaceRef, err)
		}
	} else {
		resolved, err = resolver.Resolve(ctx, trimmedRef)
		if err != nil {
			return "", fmt.Errorf("daemon: resolve dream workspace %q: %w", workspaceRef, err)
		}
	}

	if strings.TrimSpace(resolved.ID) == "" {
		return "", errors.New("daemon: dream workspace id is required")
	}
	return resolved.ID, nil
}

func isPathLikeWorkspaceRef(ref string) bool {
	trimmedRef := strings.TrimSpace(ref)
	return filepath.IsAbs(trimmedRef) ||
		strings.HasPrefix(trimmedRef, ".") ||
		strings.HasPrefix(trimmedRef, "~") ||
		strings.ContainsAny(trimmedRef, "/\\")
}

func spawnSession(
	ctx context.Context,
	sessions SessionManager,
	route SessionRoute,
	goal string,
	prompt string,
	workspace string,
	stopTimeout time.Duration,
) (err error) {
	if ctx == nil {
		return errors.New("daemon: dream session context is required")
	}
	if stopTimeout <= 0 {
		stopTimeout = defaultSessionStopTimeout
	}

	dreamSession, err := createDreamSessionWithFallback(ctx, sessions, route, goal, workspace)
	if dreamSession != nil {
		defer func() {
			stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopTimeout)
			defer cancel()
			stopErr := sessions.Stop(stopCtx, dreamSession.ID)
			if stopErr != nil {
				err = errors.Join(err, fmt.Errorf("daemon: stop dream session %q: %w", dreamSession.ID, stopErr))
			}
		}()
	}
	if err != nil {
		return fmt.Errorf("daemon: create dream session: %w", err)
	}
	if dreamSession == nil {
		return errors.New("daemon: create dream session returned nil")
	}

	events, err := sessions.Prompt(ctx, dreamSession.ID, prompt)
	if err != nil {
		return fmt.Errorf("daemon: prompt dream session %q: %w", dreamSession.ID, err)
	}

	var eventErrs []error
	for event := range events {
		if strings.TrimSpace(event.Error) != "" {
			eventErrs = append(eventErrs, errors.New(event.Error))
		}
	}
	if len(eventErrs) > 0 {
		return fmt.Errorf(
			"daemon: dream session %q reported prompt errors: %w",
			dreamSession.ID,
			errors.Join(eventErrs...),
		)
	}
	return nil
}
