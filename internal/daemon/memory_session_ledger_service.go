package daemon

import (
	"context"

	"errors"
	"fmt"

	"os"
	"path/filepath"
	"slices"
	"strings"

	"time"

	"github.com/compozy/agh/internal/api/contract"
)

type daemonMemorySessionLedgerService struct {
	rootDir          string
	unboundPartition string
	now              func() time.Time
}

func newDaemonMemorySessionLedgerService(state *bootState, now func() time.Time) *daemonMemorySessionLedgerService {
	if state == nil || !state.cfg.Memory.Enabled {
		return nil
	}
	root := strings.TrimSpace(state.cfg.Memory.Session.LedgerRoot)
	if root == "" {
		return nil
	}
	unbound := strings.TrimSpace(state.cfg.Memory.Session.UnboundPartition)
	if unbound == "" {
		unbound = "_unbound"
	}
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &daemonMemorySessionLedgerService{rootDir: root, unboundPartition: unbound, now: now}
}

func (s *daemonMemorySessionLedgerService) Get(
	ctx context.Context,
	sessionID string,
) (contract.MemorySessionLedgerResponse, error) {
	path, err := s.locate(ctx, sessionID)
	if err != nil {
		return contract.MemorySessionLedgerResponse{}, err
	}
	return readSessionLedger(path)
}

func (s *daemonMemorySessionLedgerService) Replay(
	ctx context.Context,
	sessionID string,
	req contract.MemorySessionReplayRequest,
) (contract.MemorySessionReplayResponse, error) {
	ledger, err := s.Get(ctx, sessionID)
	if err != nil {
		return contract.MemorySessionReplayResponse{}, err
	}
	events := make([]contract.MemorySessionLedgerEntryPayload, 0, len(ledger.Events))
	for _, event := range ledger.Events {
		if !req.IncludeToolEvents && isToolLedgerEvent(event.EventType) {
			continue
		}
		if !req.IncludeMemory && strings.Contains(strings.ToLower(event.EventType), "memory") {
			continue
		}
		events = append(events, event)
	}
	return contract.MemorySessionReplayResponse{SessionID: ledger.Meta.SessionID, Events: events}, nil
}

func (s *daemonMemorySessionLedgerService) Prune(
	ctx context.Context,
	req contract.MemorySessionsPruneRequest,
) (contract.MemorySessionsPruneResponse, error) {
	if req.OlderThanHours <= 0 {
		return contract.MemorySessionsPruneResponse{}, errors.New("older_than_hours must be positive")
	}
	paths, err := s.listPaths(ctx)
	if err != nil {
		return contract.MemorySessionsPruneResponse{}, err
	}
	cutoff := s.now().UTC().Add(-time.Duration(req.OlderThanHours) * time.Hour)
	response := contract.MemorySessionsPruneResponse{DryRun: req.DryRun}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return response, fmt.Errorf("daemon: prune memory session ledgers: %w", err)
		}
		ledger, err := readSessionLedger(path)
		if err != nil {
			return response, err
		}
		stopTime := ledger.Meta.CreatedAt
		if ledger.Meta.StoppedAt != nil {
			stopTime = *ledger.Meta.StoppedAt
		}
		if !stopTime.Before(cutoff) {
			continue
		}
		response.PrunedSessions++
		response.PrunedEvents += len(ledger.Events)
		if req.DryRun {
			continue
		}
		if err := os.RemoveAll(filepath.Dir(path)); err != nil {
			return response, fmt.Errorf("daemon: prune ledger %q: %w", path, err)
		}
	}
	return response, nil
}

func (s *daemonMemorySessionLedgerService) Repair(
	context.Context,
) (contract.MemorySessionsRepairResponse, error) {
	return contract.MemorySessionsRepairResponse{CompletedAt: s.now().UTC()}, nil
}

func (s *daemonMemorySessionLedgerService) locate(ctx context.Context, sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", errors.New("session_id is required")
	}
	paths, err := s.listPaths(ctx)
	if err != nil {
		return "", err
	}
	for _, path := range paths {
		if filepath.Base(filepath.Dir(path)) == sessionID {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w: memory session ledger %q", os.ErrNotExist, sessionID)
}

func (s *daemonMemorySessionLedgerService) listPaths(ctx context.Context) ([]string, error) {
	if s == nil || strings.TrimSpace(s.rootDir) == "" {
		return nil, errors.New("daemon: memory session ledger root is not configured")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("daemon: list memory session ledgers: %w", err)
	}
	pattern := filepath.Join(s.rootDir, "*", "*", "ledger.jsonl")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("daemon: glob memory session ledgers: %w", err)
	}
	slices.Sort(paths)
	return paths, nil
}
