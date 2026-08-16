package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

type spawnGovernance struct {
	depth  int
	rootID string
}

type spawnGovernanceLookup func(string) (*Info, bool, error)

func (m *Manager) spawnGovernanceForParent(ctx context.Context, parent *Info) (spawnGovernance, error) {
	return resolveSpawnGovernance(parent, func(sessionID string) (*Info, bool, error) {
		info, err := m.Status(ctx, sessionID)
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				return nil, false, nil
			}
			return nil, false, err
		}
		return info, true, nil
	})
}

func resolveSpawnGovernance(parent *Info, lookup spawnGovernanceLookup) (spawnGovernance, error) {
	if parent == nil || strings.TrimSpace(parent.ID) == "" {
		return spawnGovernance{}, fmt.Errorf("%w: spawn parent identity is unavailable", ErrSpawnValidation)
	}
	if normalizeSessionType(parent.Type) != SessionTypeSpawned {
		return spawnGovernance{rootID: parent.ID}, nil
	}

	current := parent
	seen := map[string]struct{}{}
	depth := 0
	for normalizeSessionType(current.Type) == SessionTypeSpawned {
		currentID := strings.TrimSpace(current.ID)
		if _, ok := seen[currentID]; ok {
			return spawnGovernance{}, fmt.Errorf("%w: spawned session lineage contains a cycle", ErrSpawnValidation)
		}
		seen[currentID] = struct{}{}
		depth++
		lineage := store.NormalizeSessionLineage(current.ID, current.Lineage)
		parentID := strings.TrimSpace(lineage.ParentSessionID)
		if parentID == "" {
			return spawnGovernance{depth: depth, rootID: currentID}, nil
		}
		ancestor, found, err := lookup(parentID)
		if err != nil {
			return spawnGovernance{}, fmt.Errorf("%w: resolve spawned ancestor: %w", ErrSpawnValidation, err)
		}
		if !found {
			return spawnGovernance{depth: depth, rootID: currentID}, nil
		}
		if normalizeSessionType(ancestor.Type) != SessionTypeSpawned {
			return spawnGovernance{depth: depth, rootID: ancestor.ID}, nil
		}
		current = ancestor
	}
	return spawnGovernance{depth: depth, rootID: current.ID}, nil
}
