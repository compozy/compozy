// Package journal owns terminal command persistence and retained byte artifacts.
package journal

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/store/workspacedb"
	"github.com/compozy/compozy/internal/terminal"
)

const (
	defaultPageLimit = 50
	maximumPageLimit = 500
)

// Options configures the durable terminal journal.
type Options struct {
	Databases *workspacedb.Pool
	HomeDir   string
	Logger    *slog.Logger
	Now       func() time.Time
}

// Service implements the terminal journal over per-workspace SQLite stores.
type Service struct {
	databases   *workspacedb.Pool
	homeDir     string
	logger      *slog.Logger
	now         func() time.Time
	laneCtx     context.Context
	cancelLanes context.CancelCauseFunc

	mu                sync.Mutex
	lanes             map[string]*terminalLane
	writeFailures     atomic.Uint64
	artifactMu        sync.Mutex
	liveTailMu        sync.Mutex
	liveTails         map[string][]terminal.OutputSegment
	liveTailTerminals map[string]string
	liveTailOrder     []string
}

var (
	_ terminal.Journal        = (*Service)(nil)
	_ terminal.MarkerConsumer = (*Service)(nil)
)

// New constructs a durable journal.
func New(ctx context.Context, options Options) (*Service, error) {
	if ctx == nil {
		return nil, errors.New("terminal journal: owner context is required")
	}
	if options.Databases == nil {
		return nil, errors.New("terminal journal: workspace databases are required")
	}
	if strings.TrimSpace(options.HomeDir) == "" {
		return nil, errors.New("terminal journal: home directory is required")
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	laneCtx, cancelLanes := context.WithCancelCause(ctx)
	return &Service{
		databases:         options.Databases,
		homeDir:           options.HomeDir,
		logger:            options.Logger,
		now:               options.Now,
		laneCtx:           laneCtx,
		cancelLanes:       cancelLanes,
		lanes:             make(map[string]*terminalLane),
		liveTails:         make(map[string][]terminal.OutputSegment),
		liveTailTerminals: make(map[string]string),
	}, nil
}

// RemoveWorkspace closes and removes the workspace database during unregister.
func (s *Service) RemoveWorkspace(ctx context.Context, workspaceID string) error {
	if s == nil {
		return nil
	}
	preparation, err := s.PrepareWorkspaceRemoval(ctx, workspaceID)
	if err != nil {
		return err
	}
	if err := preparation.BeforeDelete(ctx); err != nil {
		return errors.Join(err, preparation.Rollback(context.WithoutCancel(ctx)))
	}
	return preparation.Commit(ctx)
}

// Shutdown closes every open workspace database.
func (s *Service) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	laneErr := s.closeLanes(ctx, func(*terminalLane) bool { return true })
	if laneErr != nil {
		return laneErr
	}
	s.cancelLanes(nil)
	closeErr := s.databases.Close(ctx)
	s.clearLiveTails()
	return closeErr
}

// WriteFailureCount reports durable append failures observed by retry workers.
func (s *Service) WriteFailureCount() uint64 {
	if s == nil {
		return 0
	}
	return s.writeFailures.Load()
}

// PendingCount reports retained, not-yet-durable rows for one terminal.
func (s *Service) PendingCount(info terminal.Info) int64 {
	if s == nil {
		return 0
	}
	lane := s.lane(info)
	if lane == nil {
		return 0
	}
	return lane.pending.Load()
}
