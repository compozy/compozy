package sessionprocess

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

const normalInterval = 5 * time.Second
const toolInterval = 250 * time.Millisecond

type Service struct {
	runCtx  context.Context
	workMu  sync.Mutex
	closing bool
	work    sync.WaitGroup

	source       Source
	store        Store
	logger       *slog.Logger
	scanMu       sync.Mutex
	observations map[Identity]Observation
	mu           sync.RWMutex
	snapshot     Snapshot
	summaries    map[string]Summary
	clients      atomic.Int64
	cancel       context.CancelFunc
	done         chan struct{}
	lastFull     time.Time
}

func New(ctx context.Context, source Source, persistence Store, logger *slog.Logger) (*Service, error) {
	if source == nil || persistence == nil {
		return nil, errors.New("session processes: source and store are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	saved, err := persistence.LoadProcessObservations(ctx)
	if err != nil {
		return nil, fmt.Errorf("load process observations: %w", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	s := &Service{
		runCtx:       runCtx,
		source:       source,
		store:        persistence,
		logger:       logger,
		observations: make(map[Identity]Observation),
		summaries:    make(map[string]Summary),
		cancel:       cancel,
		done:         make(chan struct{}),
	}
	for _, o := range saved {
		s.observations[o.Identity] = o
	}
	// The initial scan re-verifies every saved identity before making it visible.
	if _, err := s.Refresh(runCtx); err != nil {
		cancel()
		return nil, err
	}
	go s.run(runCtx)
	return s, nil
}

func (s *Service) Shutdown() {
	s.workMu.Lock()
	s.closing = true
	s.cancel()
	s.workMu.Unlock()
	<-s.done
	s.work.Wait()
}
func (s *Service) Acquire() func() {
	s.clients.Add(1)
	return sync.OnceFunc(func() { s.clients.Add(-1) })
}
func (s *Service) Summary(sessionID string) Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summary := s.summaries[sessionID]
	summary.Supported = s.snapshot.Supported
	summary.Ports = slices.Clone(summary.Ports)
	if summary.Ports == nil {
		summary.Ports = []int{}
	}
	return summary
}

func (s *Service) run(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(toolInterval)
	defer ticker.Stop()
	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		owners, err := s.source.Owners(ctx)
		if err != nil {
			s.logger.Warn("Read session process demand", "error", err)
			continue
		}
		demand := s.clients.Load() > 0
		busy := false
		for _, o := range owners {
			demand = demand || o.Live
			busy = busy || (o.Live && o.ToolBusy)
		}
		if !demand || (!busy && time.Since(last) < normalInterval) {
			continue
		}
		if _, err := s.refresh(ctx, owners, false); err != nil && ctx.Err() == nil {
			s.logger.Warn("Scan session processes", "error", err)
		}
		last = time.Now()
	}
}

func (s *Service) Refresh(ctx context.Context) (Snapshot, error) {
	owners, err := s.source.Owners(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	return s.refresh(ctx, owners, true)
}

func (s *Service) refresh(ctx context.Context, owners []Owner, force bool) (Snapshot, error) {
	s.scanMu.Lock()
	snap, changed, err := s.scan(ctx, owners, force)
	s.scanMu.Unlock()
	if err != nil {
		return Snapshot{}, err
	}
	for _, id := range changed {
		s.source.ProcessesChanged(ctx, id)
	}
	return snap, nil
}

func (s *Service) scan(ctx context.Context, owners []Owner, force bool) (Snapshot, []string, error) {
	full := force || time.Since(s.lastFull) >= normalInterval
	var raw procutil.ProcessSnapshot
	var err error
	if full {
		raw, err = procutil.ScanProcesses(ctx)
	} else {
		raw, err = procutil.ScanProcessTree(ctx)
	}
	if err != nil {
		return Snapshot{}, nil, err
	}
	if !raw.Supported {
		snapshot := Snapshot{Supported: false, ScannedAt: raw.ScannedAt, Processes: []Process{}}
		s.mu.Lock()
		s.snapshot = snapshot
		s.mu.Unlock()
		return snapshot, nil, nil
	}
	next, rows, err := s.reconcile(ctx, raw, owners, full)
	if err != nil {
		return Snapshot{}, nil, err
	}
	s.enrich(ctx, rows, full)
	updates, deletes := observationChanges(s.observations, next)
	if len(updates)+len(deletes) > 0 {
		if err := s.store.SaveProcessObservations(ctx, updates, deletes); err != nil {
			return Snapshot{}, nil, fmt.Errorf("persist process observations: %w", err)
		}
	}
	s.observations = next
	snapshot := Snapshot{
		Supported: raw.Supported,
		ScannedAt: raw.ScannedAt,
		Duration:  time.Since(raw.ScannedAt),
		Processes: rows,
	}
	summaries := summarize(snapshot)
	s.mu.Lock()
	changed := changedProcessSessions(s.snapshot.Processes, snapshot.Processes)
	s.snapshot = snapshot
	s.summaries = summaries
	s.mu.Unlock()
	return snapshot, changed, nil
}

func (s *Service) enrich(ctx context.Context, rows []Process, full bool) {
	if full {
		enriched := make([]procutil.ScannedProcess, len(rows))
		for i := range rows {
			enriched[i] = rows[i].ScannedProcess
		}
		procutil.EnrichProcesses(ctx, enriched)
		for i := range rows {
			rows[i].ScannedProcess = enriched[i]
		}
		s.lastFull = time.Now()
	} else {
		s.mu.RLock()
		previous := make(map[Identity]Process, len(s.snapshot.Processes))
		for _, p := range s.snapshot.Processes {
			previous[identity(p.ScannedProcess)] = p
		}
		s.mu.RUnlock()
		for i, p := range rows {
			if old, ok := previous[identity(p.ScannedProcess)]; ok {
				rows[i].Ports = old.Ports
				rows[i].CWD = old.CWD
				rows[i].Argv0 = old.Argv0
				rows[i].Command = old.Command
				rows[i].Home = old.Home
			}
		}
	}
}

func summarize(s Snapshot) map[string]Summary {
	out := make(map[string]Summary)
	for _, p := range s.Processes {
		summary := out[p.Owner.SessionID]
		summary.Supported = s.Supported
		switch p.Kind {
		case "background":
			summary.Background++
		case "terminal":
			summary.Terminals++
		}
		summary.Ports = append(summary.Ports, p.Ports...)
		slices.Sort(summary.Ports)
		summary.Ports = slices.Compact(summary.Ports)
		out[p.Owner.SessionID] = summary
	}
	return out
}

// Catalog notifications also refresh open process lists, even when their counts stay unchanged.
func changedProcessSessions(before, after []Process) []string {
	previous := make(map[Identity]Process, len(before))
	for _, p := range before {
		previous[identity(p.ScannedProcess)] = p
	}
	changed := make(map[string]bool)
	for _, p := range after {
		id := identity(p.ScannedProcess)
		old, exists := previous[id]
		if !exists || !sameProcessView(old, p) {
			changed[p.Owner.SessionID] = true
			if exists {
				changed[old.Owner.SessionID] = true
			}
		}
		delete(previous, id)
	}
	for _, p := range previous {
		changed[p.Owner.SessionID] = true
	}
	return slices.Sorted(maps.Keys(changed))
}

func sameProcessView(a, b Process) bool {
	return a.Owner.SessionID == b.Owner.SessionID && a.Kind == b.Kind && a.Attribution == b.Attribution &&
		a.TerminalID == b.TerminalID && a.PGID == b.PGID && a.Argv0 == b.Argv0 &&
		a.Command == b.Command && a.CWD == b.CWD && slices.Equal(a.Ports, b.Ports)
}
