package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	"github.com/compozy/agh/internal/vault"
)

var _ vault.Store = (*VaultRepo)(nil)
var _ vault.Store = transactionVaultStore{}

type transactionVaultStore struct {
	owner *VaultRepo
	exec  globalSQLExecutor
}

func (s transactionVaultStore) PutVaultSecret(ctx context.Context, record vault.Record) error {
	if s.owner == nil {
		return errors.New("store: global database is required")
	}
	if s.exec == nil {
		return errors.New("store: vault transaction is required")
	}
	normalized, err := s.owner.normalizeVaultRecord(record)
	if err != nil {
		return err
	}
	return putVaultSecretWithExecutor(ctx, s.exec, normalized)
}

func (s transactionVaultStore) GetVaultSecret(ctx context.Context, ref string) (vault.Record, error) {
	if s.exec == nil {
		return vault.Record{}, errors.New("store: vault transaction is required")
	}
	return getVaultSecretWithExecutor(ctx, s.exec, ref)
}

func (s transactionVaultStore) ListVaultSecrets(ctx context.Context, prefix string) ([]vault.Record, error) {
	if s.exec == nil {
		return nil, errors.New("store: vault transaction is required")
	}
	return listVaultSecretsWithExecutor(ctx, s.exec, prefix)
}

func (s transactionVaultStore) DeleteVaultSecret(ctx context.Context, ref string) error {
	if s.exec == nil {
		return errors.New("store: vault transaction is required")
	}
	return deleteVaultSecretWithExecutor(ctx, s.exec, ref)
}

// PutVaultSecret stores one encrypted vault secret record.
func (g *VaultRepo) PutVaultSecret(ctx context.Context, record vault.Record) error {
	if err := g.checkReady(ctx, "put vault secret"); err != nil {
		return err
	}
	normalized, err := g.normalizeVaultRecord(record)
	if err != nil {
		return err
	}
	if err := putVaultSecretWithExecutor(ctx, g.db, normalized); err != nil {
		return err
	}
	return nil
}

func putVaultSecretWithExecutor(ctx context.Context, exec globalSQLExecutor, record vault.Record) error {
	err := sqlcgen.New(exec).UpsertVaultSecret(ctx, sqlcgen.UpsertVaultSecretParams{
		Ref: record.Ref, Kind: record.Kind, EncryptedValue: record.EncryptedValue,
		CreatedAt: store.FormatTimestamp(record.CreatedAt), UpdatedAt: store.FormatTimestamp(record.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("store: put vault secret %q: %w", record.Ref, err)
	}
	return nil
}

// GetVaultSecret returns one encrypted vault secret record.
func (g *VaultRepo) GetVaultSecret(ctx context.Context, ref string) (vault.Record, error) {
	if err := g.checkReady(ctx, "get vault secret"); err != nil {
		return vault.Record{}, err
	}
	normalized := vault.NormalizeRef(ref)
	if normalized == "" {
		return vault.Record{}, errors.New("store: vault secret ref is required")
	}
	return getVaultSecretWithExecutor(ctx, g.db, normalized)
}

func getVaultSecretWithExecutor(ctx context.Context, exec globalSQLExecutor, ref string) (vault.Record, error) {
	normalized := vault.NormalizeRef(ref)
	if normalized == "" {
		return vault.Record{}, errors.New("store: vault secret ref is required")
	}
	row, err := sqlcgen.New(exec).GetVaultSecret(ctx, normalized)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return vault.Record{}, vault.ErrSecretNotFound
		}
		return vault.Record{}, fmt.Errorf("store: get vault secret %q: %w", normalized, err)
	}
	return vaultRecordFromGenerated(row)
}

// ListVaultSecrets returns encrypted vault secret records filtered by ref prefix.
func (g *VaultRepo) ListVaultSecrets(ctx context.Context, prefix string) (_ []vault.Record, err error) {
	if err := g.checkReady(ctx, "list vault secrets"); err != nil {
		return nil, err
	}
	return listVaultSecretsWithExecutor(ctx, g.db, prefix)
}

func listVaultSecretsWithExecutor(
	ctx context.Context,
	exec globalSQLExecutor,
	prefix string,
) (_ []vault.Record, err error) {
	normalizedPrefix := strings.TrimSpace(prefix)
	// dynamic-sql: exact-ref and prefix-range modes require distinct predicates and argument counts.
	query := `SELECT ref, kind, encrypted_value, created_at, updated_at FROM vault_secrets`
	args := make([]any, 0, 3)
	if normalizedPrefix != "" {
		if strings.HasSuffix(normalizedPrefix, "/") {
			query += ` WHERE ref >= ? AND ref < ?`
			args = append(args, normalizedPrefix, vaultPrefixRangeEnd(normalizedPrefix))
		} else {
			childPrefix := normalizedPrefix + "/"
			query += ` WHERE ref = ? OR (ref >= ? AND ref < ?)`
			args = append(args, normalizedPrefix, childPrefix, vaultPrefixRangeEnd(childPrefix))
		}
	}
	query += ` ORDER BY ref ASC`

	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list vault secrets: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinCleanupError(&err, fmt.Errorf("store: close vault secret rows: %w", closeErr))
		}
	}()

	records := make([]vault.Record, 0)
	for rows.Next() {
		record, scanErr := scanVaultSecret(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate vault secrets: %w", err)
	}
	return records, nil
}

func vaultPrefixRangeEnd(prefix string) string {
	if prefix == "" {
		return ""
	}
	next := []byte(prefix)
	for i, value := range slices.Backward(next) {
		if value < 0xff {
			next[i]++
			return string(next[:i+1])
		}
	}
	return prefix + "\x00"
}

// DeleteVaultSecret removes one encrypted vault secret record.
func (g *VaultRepo) DeleteVaultSecret(ctx context.Context, ref string) error {
	if err := g.checkReady(ctx, "delete vault secret"); err != nil {
		return err
	}
	normalized := vault.NormalizeRef(ref)
	if normalized == "" {
		return errors.New("store: vault secret ref is required")
	}
	return deleteVaultSecretWithExecutor(ctx, g.db, normalized)
}

func deleteVaultSecretWithExecutor(ctx context.Context, exec globalSQLExecutor, ref string) error {
	normalized := vault.NormalizeRef(ref)
	if normalized == "" {
		return errors.New("store: vault secret ref is required")
	}
	affected, err := sqlcgen.New(exec).DeleteVaultSecret(ctx, normalized)
	if err != nil {
		return fmt.Errorf("store: delete vault secret %q: %w", normalized, err)
	}
	if affected == 0 {
		return vault.ErrSecretNotFound
	}
	return nil
}

func (g *VaultRepo) normalizeVaultRecord(record vault.Record) (vault.Record, error) {
	record.Ref = vault.NormalizeRef(record.Ref)
	record.Kind = strings.TrimSpace(record.Kind)
	record.EncryptedValue = strings.TrimSpace(record.EncryptedValue)
	if record.Ref == "" {
		return vault.Record{}, errors.New("store: vault secret ref is required")
	}
	if err := vault.ValidateSecretRef(record.Ref); err != nil {
		return vault.Record{}, err
	}
	if record.EncryptedValue == "" {
		return vault.Record{}, errors.New("store: vault encrypted value is required")
	}
	now := g.now()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}
	return record, nil
}

func scanVaultSecret(scanner rowScanner) (vault.Record, error) {
	var (
		record       vault.Record
		createdAtRaw string
		updatedAtRaw string
	)
	if err := scanner.Scan(
		&record.Ref,
		&record.Kind,
		&record.EncryptedValue,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		return vault.Record{}, fmt.Errorf("store: scan vault secret: %w", err)
	}
	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return vault.Record{}, err
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return vault.Record{}, err
	}
	record.CreatedAt = createdAt
	record.UpdatedAt = updatedAt
	return record, nil
}

func vaultRecordFromGenerated(row sqlcgen.VaultSecret) (vault.Record, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return vault.Record{}, err
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return vault.Record{}, err
	}
	return vault.Record{
		Ref: row.Ref, Kind: row.Kind, EncryptedValue: row.EncryptedValue,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}
