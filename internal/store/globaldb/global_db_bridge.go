package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var (
	_ bridges.BridgeTaskSubscriptionStore = (*BridgeRepo)(nil)
	_ bridges.TargetDirectoryStore        = (*BridgeRepo)(nil)
)

// InsertBridgeInstance creates a new persisted bridge instance row.
func (g *BridgeRepo) InsertBridgeInstance(ctx context.Context, instance bridges.BridgeInstance) error {
	if err := g.checkReady(ctx, "insert bridge instance"); err != nil {
		return err
	}

	normalized,
		routingPolicyJSON,
		providerConfig,
		deliveryDefaults,
		degradationReason,
		degradationMessage,
		err := normalizeBridgeInstanceRecord(instance)
	if err != nil {
		return err
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}

	params, err := bridgeInstanceInsertParams(bridgeInstanceRecord{
		instance: normalized, routingPolicyJSON: routingPolicyJSON, providerConfig: providerConfig,
		deliveryDefaults: deliveryDefaults, degradationReason: degradationReason,
		degradationMessage: degradationMessage,
	})
	if err != nil {
		return err
	}
	if err := g.queries.InsertBridgeInstance(ctx, params); err != nil {
		return fmt.Errorf("store: insert bridge instance %q: %w", normalized.ID, mapBridgeInstanceConstraintError(err))
	}

	return nil
}

// UpdateBridgeInstance updates an existing persisted bridge instance row.
func (g *BridgeRepo) UpdateBridgeInstance(ctx context.Context, instance bridges.BridgeInstance) error {
	if err := g.checkReady(ctx, "update bridge instance"); err != nil {
		return err
	}

	normalized,
		routingPolicyJSON,
		providerConfig,
		deliveryDefaults,
		degradationReason,
		degradationMessage,
		err := normalizeBridgeInstanceRecord(instance)
	if err != nil {
		return err
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = g.now()
	}

	params, err := bridgeInstanceUpdateParams(bridgeInstanceRecord{
		instance: normalized, routingPolicyJSON: routingPolicyJSON, providerConfig: providerConfig,
		deliveryDefaults: deliveryDefaults, degradationReason: degradationReason,
		degradationMessage: degradationMessage,
	})
	if err != nil {
		return err
	}
	affected, err := g.queries.UpdateBridgeInstance(ctx, params)
	if err != nil {
		return fmt.Errorf("store: update bridge instance %q: %w", normalized.ID, mapBridgeInstanceConstraintError(err))
	}

	if affected == 0 {
		return fmt.Errorf("store: bridge instance %q: %w", normalized.ID, bridges.ErrBridgeInstanceNotFound)
	}

	return nil
}

// DeleteBridgeInstance removes a persisted bridge instance row.
func (g *BridgeRepo) DeleteBridgeInstance(ctx context.Context, id string) error {
	if err := g.checkReady(ctx, "delete bridge instance"); err != nil {
		return err
	}

	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return errors.New("store: bridge instance id is required")
	}

	affected, err := g.queries.DeleteBridgeInstance(ctx, trimmedID)
	if err != nil {
		return fmt.Errorf("store: delete bridge instance %q: %w", trimmedID, err)
	}

	if affected == 0 {
		return fmt.Errorf("store: bridge instance %q: %w", trimmedID, bridges.ErrBridgeInstanceNotFound)
	}

	return nil
}

// GetBridgeInstance loads one persisted bridge instance by primary key.
func (g *BridgeRepo) GetBridgeInstance(ctx context.Context, id string) (bridges.BridgeInstance, error) {
	if err := g.checkReady(ctx, "get bridge instance"); err != nil {
		return bridges.BridgeInstance{}, err
	}

	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return bridges.BridgeInstance{}, errors.New("store: bridge instance id is required")
	}

	row, err := g.queries.GetBridgeInstance(ctx, trimmedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bridges.BridgeInstance{}, bridges.ErrBridgeInstanceNotFound
		}
		return bridges.BridgeInstance{}, err
	}
	return bridgeInstanceFromGenerated(row)
}

// ReplaceBridgeInstances atomically swaps the daemon-visible bridge instance projection.
func (g *BridgeRepo) ReplaceBridgeInstances(ctx context.Context, instances []bridges.BridgeInstance) (err error) {
	if err := g.checkReady(ctx, "replace bridge instances"); err != nil {
		return err
	}

	prepared := make([]bridgeInstanceRecord, 0, len(instances))
	seen := make(map[string]struct{}, len(instances))
	for _, instance := range instances {
		record, normalizeErr := prepareBridgeInstanceRecord(instance)
		if normalizeErr != nil {
			return normalizeErr
		}
		if _, exists := seen[record.instance.ID]; exists {
			return fmt.Errorf("store: duplicate bridge instance %q in replacement set", record.instance.ID)
		}
		seen[record.instance.ID] = struct{}{}
		prepared = append(prepared, record)
	}

	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin bridge instance replacement transaction: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, rollbackTx(tx, "bridge instance replacement"))
		}
	}()

	for _, record := range prepared {
		if err := upsertPreparedBridgeInstance(ctx, tx, record, g.now); err != nil {
			return err
		}
	}
	queries := sqlcgen.New(tx)
	rows, err := queries.ListBridgeInstanceIDs(ctx)
	if err != nil {
		return fmt.Errorf("store: query stale bridge instances during replacement: %w", err)
	}
	var staleIDs []string
	for _, id := range rows {
		if _, keep := seen[id]; !keep {
			staleIDs = append(staleIDs, id)
		}
	}
	for _, id := range staleIDs {
		if _, err := queries.DeleteBridgeInstance(ctx, id); err != nil {
			return fmt.Errorf("store: delete stale bridge instance %q during replacement: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit bridge instance replacement transaction: %w", err)
	}
	return nil
}

type bridgeInstanceRecord struct {
	instance           bridges.BridgeInstance
	routingPolicyJSON  string
	providerConfig     any
	deliveryDefaults   any
	degradationReason  any
	degradationMessage any
}

func prepareBridgeInstanceRecord(instance bridges.BridgeInstance) (bridgeInstanceRecord, error) {
	normalized,
		routingPolicyJSON,
		providerConfig,
		deliveryDefaults,
		degradationReason,
		degradationMessage,
		err := normalizeBridgeInstanceRecord(instance)
	if err != nil {
		return bridgeInstanceRecord{}, err
	}

	return bridgeInstanceRecord{
		instance:           normalized,
		routingPolicyJSON:  routingPolicyJSON,
		providerConfig:     providerConfig,
		deliveryDefaults:   deliveryDefaults,
		degradationReason:  degradationReason,
		degradationMessage: degradationMessage,
	}, nil
}

func upsertPreparedBridgeInstance(
	ctx context.Context,
	execer globalSQLExecutor,
	record bridgeInstanceRecord,
	now func() time.Time,
) error {
	clock := now
	if clock == nil {
		clock = time.Now
	}
	normalized := record.instance
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = clock().UTC()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}

	record.instance = normalized
	params, err := bridgeInstanceUpsertParams(record)
	if err != nil {
		return err
	}
	if err := sqlcgen.New(execer).UpsertBridgeInstance(ctx, params); err != nil {
		return fmt.Errorf("store: replace bridge instance %q: %w", normalized.ID, mapBridgeInstanceConstraintError(err))
	}
	return nil
}

// PutBridgeSecretBinding inserts or refreshes a persisted secret binding row.
