package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"strings"

	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func (s *Service) prepareWorkspace(ctx context.Context, workspaceRef string) (dreamRunWorkspace, error) {
	trimmedRef := strings.TrimSpace(workspaceRef)
	if trimmedRef == "" {
		return dreamRunWorkspace{id: "", store: s.memStore, scope: memcontract.ScopeGlobal}, nil
	}
	if s.workspaceResolver == nil {
		return dreamRunWorkspace{}, errors.New("memory: workspace resolver is required")
	}

	resolved, err := s.workspaceResolver.Resolve(ctx, trimmedRef)
	if err != nil {
		return dreamRunWorkspace{}, fmt.Errorf("memory: resolve workspace %q: %w", trimmedRef, err)
	}
	workspaceID := strings.TrimSpace(resolved.WorkspaceID)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(resolved.ID)
	}
	if workspaceID == "" {
		return dreamRunWorkspace{}, errors.New("memory: workspace id is required")
	}
	workspaceStore := s.memStore
	if s.memStore != nil {
		workspaceStore = s.memStore.ForWorkspace(resolved.RootDir)
		if err := workspaceStore.EnsureDirs(); err != nil {
			return dreamRunWorkspace{}, fmt.Errorf(
				"memory: ensure workspace memory dirs for %q: %w",
				resolved.RootDir,
				err,
			)
		}
	}

	return dreamRunWorkspace{
		id:    workspaceID,
		store: workspaceStore,
		scope: memcontract.ScopeWorkspace,
	}, nil
}

func (s *Service) validate() error {
	switch {
	case s == nil:
		return errors.New("memory: service is required")
	case s.lock == nil:
		return errors.New("memory: consolidation lock is required")
	case s.logger == nil:
		return errors.New("memory: logger is required")
	case s.now == nil:
		return errors.New("memory: clock is required")
	case s.countSessionsSince == nil:
		return errors.New("memory: session counter is required")
	default:
		return nil
	}
}

func (s *Service) timeGatePasses(lastConsolidatedAt time.Time) bool {
	if lastConsolidatedAt.IsZero() || s.minHours <= 0 {
		return true
	}

	return s.now().Sub(lastConsolidatedAt).Hours() >= s.minHours
}

func (s *Service) scanCompletedSessionsSince(lastConsolidatedAt time.Time) (int, error) {
	if strings.TrimSpace(s.sessionsDir) == "" {
		return 0, errors.New("memory: sessions directory is required")
	}

	entries, err := os.ReadDir(s.sessionsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("memory: read sessions directory %q: %w", s.sessionsDir, err)
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(s.sessionsDir, entry.Name(), "meta.json")
		payload, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			s.logger.Warn("memory: skip unreadable session metadata", "path", path, "error", err)
			continue
		}

		var meta persistedSessionMetadata
		if err := json.Unmarshal(payload, &meta); err != nil {
			s.logger.Warn("memory: skip malformed session metadata", "path", path, "error", err)
			continue
		}

		completedAt, ok := meta.completedAt()
		if !ok {
			continue
		}
		if !lastConsolidatedAt.IsZero() && completedAt.Before(lastConsolidatedAt) {
			continue
		}

		count++
	}

	return count, nil
}

func (m persistedSessionMetadata) completedAt() (time.Time, bool) {
	if m.StoppedAt != nil && !m.StoppedAt.IsZero() {
		return m.StoppedAt.UTC(), true
	}
	if strings.EqualFold(strings.TrimSpace(m.State), "stopped") && !m.UpdatedAt.IsZero() {
		return m.UpdatedAt.UTC(), true
	}
	return time.Time{}, false
}
