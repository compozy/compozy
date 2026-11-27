package globaldb

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *MigrationNoticeRepo) ListMigrationNotices(ctx context.Context) ([]store.MigrationNotice, error) {
	if err := g.checkReady(ctx, "list migration notices"); err != nil {
		return nil, err
	}
	rows, err := g.readQueries.ListMigrationNotices(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list migration notices: %w", err)
	}
	notices := make([]store.MigrationNotice, 0, len(rows))
	for _, row := range rows {
		createdAt, err := store.ParseTimestamp(row.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("store: parse migration notice timestamp: %w", err)
		}
		notice := store.MigrationNotice{Key: row.Key, CreatedAt: createdAt}
		if err := json.Unmarshal([]byte(row.Payload), &notice.Payload); err != nil {
			return nil, fmt.Errorf("store: decode migration notice: %w", err)
		}
		notices = append(notices, notice)
	}
	return notices, nil
}

func (g *MigrationNoticeRepo) DismissMigrationNotice(ctx context.Context, key string, now time.Time) error {
	return store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		return sqlcgen.New(tx).DismissMigrationNotice(ctx, sqlcgen.DismissMigrationNoticeParams{
			Key: key, DismissedAt: nullableTime(new(now)),
		})
	})
}

func (g *MigrationNoticeRepo) RecordGatewayRemovalProfiles(ctx context.Context, count int, now time.Time) error {
	return store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		return sqlcgen.New(tx).RecordGatewayRemovalProfiles(ctx, sqlcgen.RecordGatewayRemovalProfilesParams{
			Profiles: count, CreatedAt: store.FormatTimestamp(now),
		})
	})
}
