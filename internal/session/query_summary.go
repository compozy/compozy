package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// StatusSummary combines durable counts with authoritative live snapshots without rich hydration.
func (m *Manager) StatusSummary(ctx context.Context) (store.SessionStatusSummary, error) {
	if ctx == nil {
		return store.SessionStatusSummary{}, errors.New("session: status summary context is required")
	}
	live := m.List()
	ids := make([]string, 0, len(live))
	for _, info := range live {
		if info != nil {
			ids = append(ids, info.ID)
		}
	}
	reader, ok := m.sessionCatalog.(store.SessionStatusRowsReader)
	if !ok {
		return store.SessionStatusSummary{}, errors.New("session: status summary catalog is required")
	}
	stored, err := reader.SessionStatusRows(ctx, store.ReadScope{AllProfiles: true})
	if err != nil {
		return store.SessionStatusSummary{}, fmt.Errorf("session: read status rows: %w", err)
	}
	rows := make(sessionInventory, len(stored))
	for i := range stored {
		rows[stored[i].ID] = &stored[i]
	}
	population, err := m.statusMetadataEntries(ctx, rows, ids)
	if err != nil {
		return store.SessionStatusSummary{}, fmt.Errorf("session: read status metadata: %w", err)
	}
	result := store.SessionStatusSummary{ByStatus: make(map[string]int), ByBadge: make(map[string]int)}
	for _, info := range append(live, population...) {
		if info == nil {
			continue
		}
		state := strings.TrimSpace(string(info.State))
		if state == "" {
			state = "unknown"
		}
		result.Total++
		result.ByStatus[state]++
		result.ByBadge[string(BadgeForInfo(info))]++
		if info.State == StateActive {
			result.Active++
		}
	}
	return result, nil
}

// Keep ListAll's metadata population without hydrating histories, workspace roots or MCP authority.
func (m *Manager) statusMetadataEntries(
	ctx context.Context, rows sessionInventory, liveIDs []string,
) ([]*Info, error) {
	entries, err := os.ReadDir(m.homePaths.SessionsDir)
	if errors.Is(err, os.ErrNotExist) {
		return []*Info{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("session: read status metadata: %w", err)
	}
	metadataIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			metadataIDs = append(metadataIDs, entry.Name())
		}
	}
	m.pruneMetadataCache(metadataIDs)
	population := make([]*Info, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), sessionDeleteTombstonePrefix) ||
			slices.Contains(liveIDs, entry.Name()) {
			continue
		}
		item, err := m.statusMetadataEntry(ctx, entry.Name(), rows)
		if errors.Is(err, errInventoryHydration) {
			return nil, err
		}
		if err == nil {
			population = append(population, item)
		}
	}
	return population, nil
}

func (m *Manager) statusMetadataEntry(
	ctx context.Context, id string, rows sessionInventory,
) (*Info, error) {
	meta, err := m.readInventoryMetadata(id)
	if err != nil {
		return nil, err
	}
	if !m.isPending(id) {
		owner, err := store.LookupSessionOwner(ctx, rows, m.creationStore, id)
		if err != nil {
			return nil, err
		}
		if err := owner.BindMetadata(&meta); err != nil {
			return nil, err
		}
	}
	// Metadata owns these badge inputs; attention remains catalog-owned.
	info := &Info{
		ID: id, State: State(meta.State), StopVerificationFailed: meta.StopVerificationFailed,
		Failure: meta.Failure, Liveness: meta.Liveness, RuntimeRecovery: meta.RuntimeRecoveryValue(),
	}
	if row := rows[id]; row != nil && row.Attention != nil {
		applyAttentionToInfo(info, *row.Attention)
	} else if err := m.hydrateSessionInfoAttention(ctx, info); err != nil {
		return nil, errors.Join(errInventoryHydration, err)
	}
	return info, nil
}
