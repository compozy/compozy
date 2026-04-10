package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
)

// RuntimeManager captures the lifecycle hook the planner/executor needs from an
// extension-aware runtime manager.
type RuntimeManager interface {
	Start(context.Context) error
	Shutdown(context.Context) error
}

// RunScope captures the pre-planning runtime resources allocated for one run.
type RunScope interface {
	RunArtifacts() RunArtifacts
	RunJournal() *journal.Journal
	RunEventBus() *events.Bus[events.Event]
	RunManager() RuntimeManager
	Close(context.Context) error
}

// OpenRunScopeOptions controls whether executable extensions should be
// initialized as part of the early run-scope bootstrap.
type OpenRunScopeOptions struct {
	EnableExecutableExtensions bool
}

// BaseRunScope is the neutral run scope used by legacy and non-extension paths.
type BaseRunScope struct {
	Artifacts RunArtifacts
	Journal   *journal.Journal
	EventBus  *events.Bus[events.Event]
}

var _ RunScope = (*BaseRunScope)(nil)

type openRunScopeFunc func(context.Context, *RuntimeConfig, OpenRunScopeOptions) (RunScope, error)

var (
	openRunScopeMu      sync.RWMutex
	openRunScopeFactory openRunScopeFunc = openBaseRunScope
)

// RegisterOpenRunScopeFactory installs the current run-scope bootstrap hook.
func RegisterOpenRunScopeFactory(fn func(context.Context, *RuntimeConfig, OpenRunScopeOptions) (RunScope, error)) {
	openRunScopeMu.Lock()
	defer openRunScopeMu.Unlock()

	if fn == nil {
		openRunScopeFactory = openBaseRunScope
		return
	}
	openRunScopeFactory = fn
}

// OpenRunScope allocates the runtime resources required before planning begins.
func OpenRunScope(
	ctx context.Context,
	cfg *RuntimeConfig,
	opts OpenRunScopeOptions,
) (RunScope, error) {
	openRunScopeMu.RLock()
	factory := openRunScopeFactory
	openRunScopeMu.RUnlock()

	return factory(ctx, cfg, opts)
}

// OpenBaseRunScope allocates the non-extension runtime resources for one run.
func OpenBaseRunScope(ctx context.Context, cfg *RuntimeConfig) (*BaseRunScope, error) {
	scope, err := openBaseRunScope(ctx, cfg, OpenRunScopeOptions{})
	if err != nil {
		return nil, err
	}

	baseScope, ok := scope.(*BaseRunScope)
	if !ok {
		return nil, fmt.Errorf("open base run scope: unexpected scope type %T", scope)
	}
	return baseScope, nil
}

// Artifacts reports the run artifact paths owned by the scope.
func (s *BaseRunScope) RunArtifacts() RunArtifacts {
	if s == nil {
		return RunArtifacts{}
	}
	return s.Artifacts
}

// Journal reports the run journal owned by the scope.
func (s *BaseRunScope) RunJournal() *journal.Journal {
	if s == nil {
		return nil
	}
	return s.Journal
}

// EventBus reports the run-scoped event bus.
func (s *BaseRunScope) RunEventBus() *events.Bus[events.Event] {
	if s == nil {
		return nil
	}
	return s.EventBus
}

// RuntimeManager reports the optional extension runtime manager. Base scopes do
// not carry one.
func (*BaseRunScope) RunManager() RuntimeManager {
	return nil
}

// Close tears down the base runtime resources in journal-then-bus order.
func (s *BaseRunScope) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}

	cleanupCtx := withoutCancelContext(ctx)
	var closeErr error

	if s.Journal != nil {
		if err := s.Journal.Close(cleanupCtx); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}
	if s.EventBus != nil {
		if err := s.EventBus.Close(cleanupCtx); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}

	return closeErr
}

func openBaseRunScope(
	_ context.Context,
	cfg *RuntimeConfig,
	_ OpenRunScopeOptions,
) (RunScope, error) {
	if cfg == nil {
		return nil, fmt.Errorf("open run scope: missing runtime config")
	}

	runArtifacts, err := allocateRunArtifacts(cfg)
	if err != nil {
		return nil, err
	}
	bus := events.New[events.Event](0)
	runJournal, err := journal.Open(runArtifacts.EventsPath, bus, 0)
	if err != nil {
		if closeErr := bus.Close(context.Background()); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, fmt.Errorf("open run journal: %w", err)
	}

	return &BaseRunScope{
		Artifacts: runArtifacts,
		Journal:   runJournal,
		EventBus:  bus,
	}, nil
}

func allocateRunArtifacts(cfg *RuntimeConfig) (RunArtifacts, error) {
	runArtifacts := NewRunArtifacts(cfg.WorkspaceRoot, buildRunID(cfg))
	if err := os.MkdirAll(runArtifacts.JobsDir, 0o755); err != nil {
		return RunArtifacts{}, fmt.Errorf("mkdir run artifacts: %w", err)
	}
	return runArtifacts, nil
}

func buildRunID(cfg *RuntimeConfig) string {
	if cfg != nil && strings.TrimSpace(cfg.RunID) != "" {
		return cfg.RunID
	}
	label := runLabel(cfg)
	timestamp := time.Now().UTC().Format("20060102-150405-000000000")
	return fmt.Sprintf("%s-%s", label, timestamp)
}

func runLabel(cfg *RuntimeConfig) string {
	if cfg.Mode == ExecutionModeExec {
		return "exec"
	}
	if cfg.Mode == ExecutionModePRDTasks {
		return "tasks-" + safeScopeLabel(cfg.Name)
	}

	scope := cfg.Name
	if strings.TrimSpace(scope) == "" {
		scope = "pr-" + cfg.PR
	}
	return fmt.Sprintf("reviews-%s-round-%03d", safeScopeLabel(scope), cfg.Round)
}

func safeScopeLabel(value string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	runes := make([]rune, 0, len(normalized))
	for _, r := range normalized {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.',
			r == '_',
			r == '-':
			runes = append(runes, r)
		default:
			runes = append(runes, '_')
		}
	}

	base := string(runes)
	if strings.TrimSpace(base) == "" {
		base = "run"
	}
	sum := sha256.Sum256([]byte(normalized))
	hash := hex.EncodeToString(sum[:])[:6]
	return fmt.Sprintf("%s-%s", base, hash)
}

func withoutCancelContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}
