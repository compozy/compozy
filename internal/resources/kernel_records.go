package resources

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
)

func insertRecord(ctx context.Context, exec sqlExecutor, record RawRecord) error {
	if _, err := exec.ExecContext(
		ctx,
		`INSERT INTO resource_records (
			kind, id, version, scope_kind, scope_id, owner_kind,
			owner_id, source_kind, source_id, spec_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Kind,
		record.ID,
		record.Version,
		record.Scope.Kind,
		nullableScopeID(record.Scope),
		record.Owner.Kind,
		record.Owner.ID,
		record.Source.Kind,
		record.Source.ID,
		string(record.SpecJSON),
		store.FormatTimestamp(record.CreatedAt),
		store.FormatTimestamp(record.UpdatedAt),
	); err != nil {
		return fmt.Errorf("resources: insert record %q/%q: %w", record.Kind, record.ID, err)
	}
	return nil
}

func updateRecord(ctx context.Context, exec sqlExecutor, record RawRecord, expectedVersion int64) error {
	result, err := exec.ExecContext(
		ctx,
		`UPDATE resource_records
		 SET version = ?, scope_kind = ?, scope_id = ?, owner_kind = ?, owner_id = ?,
		     source_kind = ?, source_id = ?, spec_json = ?, updated_at = ?
		 WHERE kind = ? AND id = ? AND version = ?`,
		record.Version,
		record.Scope.Kind,
		nullableScopeID(record.Scope),
		record.Owner.Kind,
		record.Owner.ID,
		record.Source.Kind,
		record.Source.ID,
		string(record.SpecJSON),
		store.FormatTimestamp(record.UpdatedAt),
		record.Kind,
		record.ID,
		expectedVersion,
	)
	if err != nil {
		return fmt.Errorf("resources: update record %q/%q: %w", record.Kind, record.ID, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("resources: rows affected for update %q/%q: %w", record.Kind, record.ID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%w: expected version %d", ErrConflict, expectedVersion)
	}
	return nil
}

func lookupRecordWithExecutor(
	ctx context.Context,
	exec sqlExecutor,
	kind ResourceKind,
	id string,
) (RawRecord, bool, error) {
	row := exec.QueryRowContext(ctx, selectRecordByKeyQuery, kind, id)
	record, err := scanRawRecord(row)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return RawRecord{}, false, nil
	case err != nil:
		return RawRecord{}, false, err
	default:
		return record, true, nil
	}
}

func listSourceRecordsWithExecutor(ctx context.Context, exec sqlExecutor, source ResourceSource) ([]RawRecord, error) {
	rows, err := exec.QueryContext(
		ctx,
		selectSourceRecordsQuery,
		source.Kind,
		source.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("resources: query source records %q/%q: %w", source.Kind, source.ID, err)
	}
	defer func() {
		// Scan and iteration errors own the query result; Close only releases an already-consumed read cursor.
		_ = rows.Close()
	}()

	records := make([]RawRecord, 0)
	for rows.Next() {
		record, scanErr := scanRawRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resources: iterate source records %q/%q: %w", source.Kind, source.ID, err)
	}
	return records, nil
}

func lookupSourceState(ctx context.Context, exec sqlExecutor, source ResourceSource) (sourceState, bool, error) {
	var (
		sourceKind          string
		sourceID            string
		sessionNonce        string
		lastSnapshotVersion int64
		updatedAtRaw        string
	)
	if err := exec.QueryRowContext(
		ctx,
		selectSourceStateQuery,
		source.Kind,
		source.ID,
	).Scan(&sourceKind, &sourceID, &sessionNonce, &lastSnapshotVersion, &updatedAtRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sourceState{}, false, nil
		}
		return sourceState{}, false, fmt.Errorf("resources: query source state %q/%q: %w", source.Kind, source.ID, err)
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return sourceState{}, false, fmt.Errorf(
			"resources: parse source state timestamp %q/%q: %w",
			source.Kind,
			source.ID,
			err,
		)
	}
	return sourceState{
		Source: ResourceSource{
			Kind: ResourceSourceKind(sourceKind),
			ID:   sourceID,
		},
		SessionNonce:        sessionNonce,
		LastSnapshotVersion: lastSnapshotVersion,
		UpdatedAt:           updatedAt,
	}, true, nil
}

type rawRecordScanner interface {
	Scan(dest ...any) error
}

func scanRawRecord(scanner rawRecordScanner) (RawRecord, error) {
	var (
		record       RawRecord
		scopeKind    string
		scopeID      sql.NullString
		ownerKind    string
		ownerID      string
		sourceKind   string
		sourceID     string
		specJSON     string
		createdAtRaw string
		updatedAtRaw string
	)
	if err := scanner.Scan(
		&record.Kind,
		&record.ID,
		&record.Version,
		&scopeKind,
		&scopeID,
		&ownerKind,
		&ownerID,
		&sourceKind,
		&sourceID,
		&specJSON,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		return RawRecord{}, err
	}

	record.Scope = ResourceScope{
		Kind: ResourceScopeKind(scopeKind),
		ID:   strings.TrimSpace(scopeID.String),
	}
	record.Owner = ResourceOwner{
		Kind: ResourceOwnerKind(ownerKind),
		ID:   ownerID,
	}
	record.Source = ResourceSource{
		Kind: ResourceSourceKind(sourceKind),
		ID:   sourceID,
	}
	record.SpecJSON = []byte(specJSON)

	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return RawRecord{}, fmt.Errorf("resources: parse created_at for %q/%q: %w", record.Kind, record.ID, err)
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return RawRecord{}, fmt.Errorf("resources: parse updated_at for %q/%q: %w", record.Kind, record.ID, err)
	}
	record.CreatedAt = createdAt
	record.UpdatedAt = updatedAt
	return record, nil
}
