package sessioncleanup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store"

	"uuid"
)

const (
	// defaultConcurrency bounds the threads deleted at once. Each delete takes
	// the session manager's lifecycle lock for its catalog and directory step,
	// so a second worker only overlaps the per-session preparation; more would
	// queue on that lock and crowd out the daemon's other lifecycle work.
	defaultConcurrency = 2
	// defaultProgressInterval throttles progress wakes on the catalog stream.
	defaultProgressInterval = 300 * time.Millisecond
	maxRootIDs              = 1000
)

var (
	// ErrInvalidRequest wraps every refused cleanup request.
	ErrInvalidRequest = errors.New("sessioncleanup: invalid request")
	// ErrRunActive refuses a second run for a profile whose cleanup is still running.
	ErrRunActive = errors.New("sessioncleanup: a cleanup is already running for this profile")
	// ErrNoActiveRun reports a stop request with nothing running.
	ErrNoActiveRun = errors.New("sessioncleanup: no cleanup is running for this profile")
	// ErrShuttingDown refuses new runs while the daemon stops.
	ErrShuttingDown = errors.New("sessioncleanup: service is shutting down")
)

// Lister reads the archived history a cleanup plans from.
type Lister interface {
	ListSessionHistoryCleanupRows(
		ctx context.Context,
		query store.SessionHistoryCleanupQuery,
	) ([]store.SessionHistoryCleanupRow, error)
}

// Deleter deletes one archived session through the normal session delete path.
// It must return store.ErrSessionNotFound for a session already gone and
// store.ErrSessionNotArchived for one restored or moved since the plan.
type Deleter interface {
	DeleteArchived(ctx context.Context, target store.ArchivedSessionDelete) error
}

// Option configures a Service.
type Option func(*Service)

// WithPublisher wakes clients of a profile when its run progresses or ends.
func WithPublisher(publish func(profileID string)) Option {
	return func(s *Service) { s.publish = publish }
}

// WithLogger sets the service logger.
func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) { s.logger = logger }
}

// WithClock sets the service clock.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// WithConcurrency bounds the threads deleted at once.
func WithConcurrency(workers int) Option {
	return func(s *Service) { s.concurrency = workers }
}

// WithProgressInterval sets the minimum gap between progress wakes.
func WithProgressInterval(interval time.Duration) Option {
	return func(s *Service) { s.progressInterval = interval }
}

// Service plans cleanups and runs at most one per profile in the background.
type Service struct {
	lister           Lister
	deleter          Deleter
	publish          func(profileID string)
	logger           *slog.Logger
	now              func() time.Time
	concurrency      int
	progressInterval time.Duration

	mu          sync.Mutex
	runs        map[string]*cleanupRun
	closing     bool
	ownerCtx    context.Context
	ownerCancel context.CancelFunc
	work        sync.WaitGroup
}

// NewService builds a cleanup service over the archived history and the delete path.
func NewService(lister Lister, deleter Deleter, opts ...Option) (*Service, error) {
	if lister == nil || deleter == nil {
		return nil, errors.New("sessioncleanup: lister and deleter are required")
	}
	ownerCtx, ownerCancel := context.WithCancel(context.Background())
	s := &Service{
		lister:           lister,
		deleter:          deleter,
		logger:           slog.Default(),
		now:              func() time.Time { return time.Now().UTC() },
		concurrency:      defaultConcurrency,
		progressInterval: defaultProgressInterval,
		runs:             make(map[string]*cleanupRun),
		ownerCtx:         ownerCtx,
		ownerCancel:      ownerCancel,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	if s.concurrency < 1 {
		s.concurrency = 1
	}
	return s, nil
}

// Preview plans a cleanup without deleting anything.
func (s *Service) Preview(ctx context.Context, request Request) (Plan, error) {
	request, err := normalizeRequest(request)
	if err != nil {
		return Plan{}, err
	}
	rows, err := s.lister.ListSessionHistoryCleanupRows(ctx, store.SessionHistoryCleanupQuery{
		ProfileID:     request.Scope.ProfileID,
		WorkspaceID:   request.Scope.WorkspaceID,
		AllWorkspaces: request.Scope.AllWorkspaces,
	})
	if err != nil {
		return Plan{}, err
	}
	return BuildPlan(rows, request), nil
}

// Start plans a cleanup and deletes it in the background. The plan is frozen:
// sessions archived after it are never picked up by this run.
func (s *Service) Start(ctx context.Context, request Request) (Run, Plan, error) {
	request, err := normalizeRequest(request)
	if err != nil {
		return Run{}, Plan{}, err
	}
	if err := s.admit(request.Scope.ProfileID); err != nil {
		return Run{}, Plan{}, err
	}
	plan, err := s.Preview(ctx, request)
	if err != nil {
		return Run{}, Plan{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.admitLocked(request.Scope.ProfileID); err != nil {
		return Run{}, Plan{}, err
	}
	runCtx, cancel := context.WithCancel(s.ownerCtx)
	run := &cleanupRun{
		cancel: cancel,
		done:   make(chan struct{}),
		run: Run{
			ID:             uuid.NewV4().String(),
			ProfileID:      request.Scope.ProfileID,
			Scope:          request.Scope,
			Selection:      request.Selection,
			ArchivedBefore: request.ArchivedBefore,
			State:          RunRunning,
			Total:          plan.Counts.Total(),
			StartedAt:      s.now(),
		},
	}
	s.runs[request.Scope.ProfileID] = run
	s.work.Go(func() {
		defer close(run.done)
		defer cancel()
		s.execute(runCtx, run, plan.Threads)
	})
	snapshot := run.snapshot()
	s.notify(snapshot.ProfileID)
	return snapshot, plan, nil
}

// Current returns the profile's running or most recent run.
func (s *Service) Current(profileID string) (Run, bool) {
	s.mu.Lock()
	run := s.runs[strings.TrimSpace(profileID)]
	s.mu.Unlock()
	if run == nil {
		return Run{}, false
	}
	return run.snapshot(), true
}

// Stop asks the profile's running cleanup to stop after the deletes in flight
// and waits for it to settle; what it deleted stays deleted. A non-empty runID
// stops only that run, so a caller that checked one run never stops another
// that replaced it.
func (s *Service) Stop(ctx context.Context, profileID, runID string) (Run, error) {
	s.mu.Lock()
	run := s.runs[strings.TrimSpace(profileID)]
	s.mu.Unlock()
	if run == nil || !run.running() {
		return Run{}, ErrNoActiveRun
	}
	if runID = strings.TrimSpace(runID); runID != "" && run.run.ID != runID {
		return Run{}, ErrNoActiveRun
	}
	run.cancel()
	select {
	case <-run.done:
		return run.snapshot(), nil
	case <-ctx.Done():
		return run.snapshot(), ctx.Err()
	}
}

// Shutdown stops admitting runs, stops the running ones, and waits for them.
func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closing = true
	s.mu.Unlock()
	s.ownerCancel()
	done := make(chan struct{})
	go func() {
		s.work.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) admit(profileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.admitLocked(profileID)
}

func (s *Service) admitLocked(profileID string) error {
	if s.closing {
		return ErrShuttingDown
	}
	if current := s.runs[profileID]; current != nil && current.running() {
		return ErrRunActive
	}
	return nil
}

// execute deletes the planned threads with bounded concurrency. A thread stops
// at its first failure, deepest first, so its remaining sessions stay reachable
// from its main session for a retry. Each delete outlives a stop request, which
// takes effect between deletes.
func (s *Service) execute(ctx context.Context, run *cleanupRun, threads []Thread) {
	scope := run.snapshot().Scope
	queue := make(chan Thread)
	var workers sync.WaitGroup
	for range min(s.concurrency, len(threads)) {
		workers.Go(func() {
			for thread := range queue {
				s.deleteThread(ctx, run, scope, thread)
			}
		})
	}
	interrupted := false
	for _, thread := range threads {
		if ctx.Err() != nil {
			interrupted = true
			break
		}
		select {
		case queue <- thread:
		case <-ctx.Done():
			interrupted = true
		}
		if interrupted {
			break
		}
	}
	close(queue)
	workers.Wait()
	if ctx.Err() != nil && run.snapshot().Done() < run.run.Total {
		interrupted = true
	}
	run.finish(s.now(), interrupted)
	final := run.snapshot()
	s.logger.Info(
		"sessioncleanup: run finished",
		"run_id", final.ID, "state", final.State,
		"deleted", final.Deleted, "skipped", final.Skipped, "failed_threads", final.FailedThreads,
	)
	s.notify(final.ProfileID)
}

func (s *Service) deleteThread(ctx context.Context, run *cleanupRun, scope Scope, thread Thread) {
	for _, sessionID := range thread.Delete {
		if ctx.Err() != nil {
			return
		}
		err := s.deleter.DeleteArchived(context.WithoutCancel(ctx), store.ArchivedSessionDelete{
			SessionID:   sessionID,
			ProfileID:   scope.ProfileID,
			WorkspaceID: thread.WorkspaceID,
		})
		if err != nil && !errors.Is(err, store.ErrSessionNotFound) && !errors.Is(err, store.ErrSessionNotArchived) {
			s.logger.Warn(
				"sessioncleanup: delete archived session",
				"run_id", run.run.ID, "session_id", sessionID, "root_id", thread.RootID, "error", err,
			)
			run.fail(thread, err)
			s.notify(scope.ProfileID)
			return
		}
		if run.settle(err, s.now(), s.progressInterval) {
			s.notify(scope.ProfileID)
		}
	}
}

func (s *Service) notify(profileID string) {
	if s.publish != nil {
		s.publish(profileID)
	}
}

// normalizeRequest trims the request and refuses one that cannot be planned.
func normalizeRequest(request Request) (Request, error) {
	request.Scope.ProfileID = strings.TrimSpace(request.Scope.ProfileID)
	request.Scope.WorkspaceID = strings.TrimSpace(request.Scope.WorkspaceID)
	if request.Scope.ProfileID == "" {
		return Request{}, fmt.Errorf("%w: a profile is required", ErrInvalidRequest)
	}
	if (request.Scope.WorkspaceID != "") == request.Scope.AllWorkspaces {
		return Request{}, fmt.Errorf("%w: choose one workspace or all workspaces", ErrInvalidRequest)
	}
	if request.Selection.Empty() {
		return Request{}, fmt.Errorf("%w: choose what to delete", ErrInvalidRequest)
	}
	if len(request.RootIDs) > maxRootIDs {
		return Request{}, fmt.Errorf("%w: at most %d session ids", ErrInvalidRequest, maxRootIDs)
	}
	ids := make([]string, 0, len(request.RootIDs))
	for _, id := range request.RootIDs {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	// A list of only blank ids names nothing; read as "no limit" it would plan every thread.
	if len(request.RootIDs) > 0 && len(ids) == 0 {
		return Request{}, fmt.Errorf("%w: session ids must not be blank", ErrInvalidRequest)
	}
	request.RootIDs = ids
	return request, nil
}
