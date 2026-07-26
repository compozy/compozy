package resources

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/store"
)

const (
	defaultMaxSpecBytes       = 256 << 10
	defaultMaxSnapshotRecords = 256
	defaultMaxSnapshotBytes   = 2 << 20

	rawRecordSelectQuery = `SELECT
		kind,
		id,
		version,
		scope_kind,
		scope_id,
		owner_kind,
		owner_id,
		source_kind,
		source_id,
		spec_json,
		created_at,
		updated_at
	FROM resource_records`
	selectRecordByKeyQuery = rawRecordSelectQuery + `
	WHERE kind = ? AND id = ?`
	selectSourceRecordsQuery = rawRecordSelectQuery + `
	WHERE source_kind = ? AND source_id = ?
	ORDER BY kind ASC, id ASC`
	selectSourceStateQuery = `SELECT
		source_kind,
		source_id,
		session_nonce,
		last_snapshot_version,
		updated_at
	FROM resource_source_state
	WHERE source_kind = ? AND source_id = ?`
	resourceRecordOrderBy      = " ORDER BY kind ASC, id ASC"
	deleteRecordByVersionQuery = `
		DELETE FROM resource_records
		WHERE kind = ? AND id = ? AND version = ?`
	deleteStaleSourceRecordQuery = `
		DELETE FROM resource_records
		WHERE kind = ? AND id = ? AND source_kind = ? AND source_id = ?`
	deleteSourceRecordsQuery = `
		DELETE FROM resource_records
		WHERE source_kind = ? AND source_id = ?`
	deleteSourceStateQuery = `
		DELETE FROM resource_source_state
		WHERE source_kind = ? AND source_id = ?`
	activateSourceStateQuery = `INSERT INTO resource_source_state (
		source_kind,
		source_id,
		session_nonce,
		last_snapshot_version,
		updated_at
	) VALUES (?, ?, ?, 0, ?)
	ON CONFLICT(source_kind, source_id)
	DO UPDATE SET
		session_nonce = excluded.session_nonce,
		last_snapshot_version = 0,
		updated_at = excluded.updated_at`
	updateSourceStateQuery = `UPDATE resource_source_state
		SET last_snapshot_version = ?, updated_at = ?
		WHERE source_kind = ? AND source_id = ? AND session_nonce = ?`
)

// Option configures a Kernel instance.
type Option func(*Kernel)

// WithNow overrides the clock used by the kernel.
func WithNow(now func() time.Time) Option {
	return func(k *Kernel) {
		if now != nil {
			k.now = now
		}
	}
}

// WithMaxSpecBytes overrides the per-record payload ceiling.
func WithMaxSpecBytes(limit int) Option {
	return func(k *Kernel) {
		if limit > 0 {
			k.maxSpecBytes = limit
		}
	}
}

// WithMaxSnapshotRecords overrides the per-snapshot record count ceiling.
func WithMaxSnapshotRecords(limit int) Option {
	return func(k *Kernel) {
		if limit > 0 {
			k.maxSnapshotRecords = limit
		}
	}
}

// WithMaxSnapshotBytes overrides the per-snapshot byte ceiling.
func WithMaxSnapshotBytes(limit int) Option {
	return func(k *Kernel) {
		if limit > 0 {
			k.maxSnapshotBytes = limit
		}
	}
}

// Kernel implements the canonical raw desired-state persistence contract.
type Kernel struct {
	db *sql.DB

	now                func() time.Time
	maxSpecBytes       int
	maxSnapshotRecords int
	maxSnapshotBytes   int

	sourceLocksMu sync.Mutex
	sourceLocks   map[string]*sourceLock
}

var _ RawStore = (*Kernel)(nil)
var _ SourceSessionManager = (*Kernel)(nil)

type sourceLock struct {
	mu   sync.Mutex
	refs int
}

// NewKernel constructs a new raw resource persistence kernel over the supplied database.
func NewKernel(db *sql.DB, opts ...Option) (*Kernel, error) {
	if db == nil {
		return nil, errors.New("resources: database is required")
	}

	kernel := &Kernel{
		db:                 db,
		now:                func() time.Time { return time.Now().UTC() },
		maxSpecBytes:       defaultMaxSpecBytes,
		maxSnapshotRecords: defaultMaxSnapshotRecords,
		maxSnapshotBytes:   defaultMaxSnapshotBytes,
		sourceLocks:        make(map[string]*sourceLock),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(kernel)
		}
	}
	if kernel.now == nil {
		return nil, errors.New("resources: clock is required")
	}
	if kernel.maxSpecBytes <= 0 {
		return nil, errors.New("resources: max spec bytes must be positive")
	}
	if kernel.maxSnapshotRecords <= 0 {
		return nil, errors.New("resources: max snapshot records must be positive")
	}
	if kernel.maxSnapshotBytes <= 0 {
		return nil, errors.New("resources: max snapshot bytes must be positive")
	}
	return kernel, nil
}

// ActivateSourceSession registers the active nonce and resets the snapshot version counter for one source.
func (k *Kernel) ActivateSourceSession(
	ctx context.Context,
	actor MutationActor,
	source ResourceSource,
	sessionNonce string,
) error {
	if ctx == nil {
		return errors.New("resources: activate source session context is required")
	}

	normalizedActor, err := normalizeActor(actor)
	if err != nil {
		return err
	}
	if normalizedActor.Kind == MutationActorKindExtension {
		return fmt.Errorf("%w: extension actors cannot activate source sessions", ErrPermissionDenied)
	}

	normalizedSource := source.Normalize()
	if err := normalizedSource.Validate("source"); err != nil {
		return err
	}
	trimmedNonce := strings.TrimSpace(sessionNonce)
	if trimmedNonce == "" {
		return fmt.Errorf("%w: session_nonce is required", ErrValidation)
	}

	unlock := k.lockSource(normalizedSource)
	defer unlock()

	return k.withImmediateTransaction(ctx, "activate source session", func(exec sqlExecutor) error {
		updatedAt := store.FormatTimestamp(k.now())
		if _, err := exec.ExecContext(
			ctx,
			activateSourceStateQuery,
			normalizedSource.Kind,
			normalizedSource.ID,
			trimmedNonce,
			updatedAt,
		); err != nil {
			return fmt.Errorf(
				"resources: activate source session %q/%q: %w",
				normalizedSource.Kind,
				normalizedSource.ID,
				err,
			)
		}
		return nil
	})
}

// ResetSource deletes all source-owned records and source state in one transaction.
func (k *Kernel) ResetSource(ctx context.Context, actor MutationActor, source ResourceSource) error {
	if ctx == nil {
		return errors.New("resources: reset source context is required")
	}

	normalizedActor, err := normalizeActor(actor)
	if err != nil {
		return err
	}
	if normalizedActor.Kind == MutationActorKindExtension {
		return fmt.Errorf("%w: extension actors cannot reset sources", ErrPermissionDenied)
	}

	normalizedSource := source.Normalize()
	if err := normalizedSource.Validate("source"); err != nil {
		return err
	}

	unlock := k.lockSource(normalizedSource)
	defer unlock()

	return k.withImmediateTransaction(ctx, "reset source", func(exec sqlExecutor) error {
		if _, err := exec.ExecContext(
			ctx,
			deleteSourceRecordsQuery,
			normalizedSource.Kind,
			normalizedSource.ID,
		); err != nil {
			return fmt.Errorf(
				"resources: delete source records %q/%q: %w",
				normalizedSource.Kind,
				normalizedSource.ID,
				err,
			)
		}
		if _, err := exec.ExecContext(
			ctx,
			deleteSourceStateQuery,
			normalizedSource.Kind,
			normalizedSource.ID,
		); err != nil {
			return fmt.Errorf(
				"resources: delete source state %q/%q: %w",
				normalizedSource.Kind,
				normalizedSource.ID,
				err,
			)
		}
		return nil
	})
}

// PutRaw creates or updates one raw desired-state record using optimistic concurrency.
func (k *Kernel) PutRaw(ctx context.Context, actor MutationActor, draft RawDraft) (record RawRecord, err error) {
	if ctx == nil {
		return RawRecord{}, errors.New("resources: put raw context is required")
	}

	normalizedActor, normalizedDraft, err := k.preparePutRaw(actor, draft)
	if err != nil {
		return RawRecord{}, err
	}

	if err := store.ExecuteWrite(ctx, k.db, func(_ context.Context, tx *store.WriteTx) error {
		var putErr error
		record, putErr = k.putRawWithExecutor(ctx, tx, normalizedActor, normalizedDraft)
		return putErr
	}); err != nil {
		return RawRecord{}, fmt.Errorf(
			"resources: put record %q/%q: %w",
			normalizedDraft.Kind,
			normalizedDraft.ID,
			err,
		)
	}
	return record, nil
}

// DeleteRaw deletes one raw desired-state record using optimistic concurrency.
func (k *Kernel) DeleteRaw(
	ctx context.Context,
	actor MutationActor,
	kind ResourceKind,
	id string,
	expectedVersion int64,
) (err error) {
	if ctx == nil {
		return errors.New("resources: delete raw context is required")
	}

	normalizedActor, normalizedKind, trimmedID, err := k.prepareDeleteRaw(actor, kind, id)
	if err != nil {
		return err
	}

	if err := store.ExecuteWrite(ctx, k.db, func(_ context.Context, tx *store.WriteTx) error {
		return k.deleteRawWithExecutor(ctx, tx, normalizedActor, normalizedKind, trimmedID, expectedVersion)
	}); err != nil {
		return fmt.Errorf("resources: delete record %q/%q: %w", normalizedKind, trimmedID, err)
	}
	return nil
}

// GetRaw fetches one raw desired-state record under the actor's read boundary.
func (k *Kernel) GetRaw(ctx context.Context, actor MutationActor, kind ResourceKind, id string) (RawRecord, error) {
	if ctx == nil {
		return RawRecord{}, errors.New("resources: get raw context is required")
	}

	normalizedActor, err := normalizeActor(actor)
	if err != nil {
		return RawRecord{}, err
	}
	normalizedKind := kind.Normalize()
	if err := normalizedKind.Validate("kind"); err != nil {
		return RawRecord{}, err
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return RawRecord{}, fmt.Errorf("%w: id is required", ErrValidation)
	}
	if !actorAllowsKind(normalizedActor, normalizedKind) {
		return RawRecord{}, fmt.Errorf("%w: actor cannot read resource kind %q", ErrPermissionDenied, normalizedKind)
	}

	record, found, err := lookupRecordWithExecutor(ctx, k.db, normalizedKind, trimmedID)
	if err != nil {
		return RawRecord{}, err
	}
	if !found {
		return RawRecord{}, ErrNotFound
	}
	if err := validateActorReadAccess(normalizedActor, record); err != nil {
		return RawRecord{}, err
	}
	return record, nil
}

// ListRaw lists raw desired-state records under the actor's read boundary.
func (k *Kernel) ListRaw(ctx context.Context, actor MutationActor, filter ResourceFilter) ([]RawRecord, error) {
	if ctx == nil {
		return nil, errors.New("resources: list raw context is required")
	}

	normalizedActor, normalizedFilter, err := k.prepareListRaw(actor, filter)
	if err != nil {
		return nil, err
	}
	if normalizedActor.Kind == MutationActorKindExtension && k.extensionReadGrantsEmpty(normalizedActor) {
		return []RawRecord{}, nil
	}

	query, args := buildListRawQuery(normalizedActor, normalizedFilter)

	rows, err := k.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("resources: query records: %w", err)
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
		if accessErr := validateActorReadAccess(normalizedActor, record); accessErr != nil {
			if errors.Is(accessErr, ErrPermissionDenied) {
				continue
			}
			return nil, accessErr
		}
		records = append(records, record)
		if normalizedFilter.Limit > 0 && len(records) == normalizedFilter.Limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resources: iterate records: %w", err)
	}
	return records, nil
}

// ApplySourceSnapshotRaw replaces one source's desired-state snapshot under optimistic source sequencing.
func (k *Kernel) ApplySourceSnapshotRaw(ctx context.Context, actor MutationActor, snapshot SourceSnapshot) error {
	if ctx == nil {
		return errors.New("resources: apply snapshot context is required")
	}

	normalizedActor, normalizedSnapshot, normalizedDrafts, err := k.prepareSnapshotApply(actor, snapshot)
	if err != nil {
		return err
	}

	unlock := k.lockSource(normalizedActor.Source)
	defer unlock()

	return k.withImmediateTransaction(ctx, "apply source snapshot", func(exec sqlExecutor) error {
		return k.applySnapshotWithExecutor(
			ctx,
			exec,
			normalizedActor,
			normalizedSnapshot,
			normalizedDrafts,
		)
	})
}

type sourceState struct {
	Source              ResourceSource
	SessionNonce        string
	LastSnapshotVersion int64
	UpdatedAt           time.Time
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
