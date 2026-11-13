package sessiondb

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"

	"github.com/compozy/compozy/internal/store"
)

// inMemorySessionCandidatePath labels the private candidate a fresh owner builds before publication.
const inMemorySessionCandidatePath = "in-memory candidate"

type sessionMigrationApplier func(context.Context, *sql.DB, store.MigrationStream) error

type sessionMigrationLockEntry struct {
	token chan struct{}
	refs  int
}

var sessionMigrationLocks = struct {
	sync.Mutex
	entries map[string]*sessionMigrationLockEntry
}{entries: make(map[string]*sessionMigrationLockEntry)}

// applySessionMigrations is the single owner of schema upgrades for one events.db.
// Goose lists applied versions before it opens each migration's transaction, so two
// owners of the same database could both plan a migration and the second would replay
// its statements. The per-path lock covers list and apply for every opener in this
// process; the family lease cannot, because each pool connection takes it to open.
func applySessionMigrations(ctx context.Context, db *sql.DB, stream store.MigrationStream, path string) error {
	return applySessionMigrationsWith(ctx, db, stream, path, store.Apply)
}

func applySessionMigrationsWith(
	ctx context.Context,
	db *sql.DB,
	stream store.MigrationStream,
	path string,
	apply sessionMigrationApplier,
) error {
	release, err := acquireSessionMigrationLock(ctx, path)
	if err != nil {
		return err
	}
	defer release()
	applyErr := apply(ctx, db, stream)
	if applyErr == nil {
		return nil
	}
	// Another process can still win the same race. A failed migration rolls back and
	// records nothing, so a stream that now passes every integrity check at the
	// embedded head was completed by that owner. Anything else keeps the original error.
	if currentErr := store.RequireCurrent(ctx, db, stream); currentErr != nil {
		return applyErr
	}
	slog.Default().WarnContext(ctx, "store.migrations.concurrent_apply_settled",
		"stream", stream.Name,
		"path", path,
		"error", applyErr,
	)
	return nil
}

// acquireSessionMigrationLock serializes schema upgrades of one events.db path inside this process.
func acquireSessionMigrationLock(ctx context.Context, path string) (func(), error) {
	if path == inMemorySessionCandidatePath {
		return func() {}, nil
	}
	key, err := normalizeSessionDBFamilyLeasePath(path)
	if err != nil {
		return nil, err
	}
	sessionMigrationLocks.Lock()
	entry := sessionMigrationLocks.entries[key]
	if entry == nil {
		entry = &sessionMigrationLockEntry{token: make(chan struct{}, 1)}
		entry.token <- struct{}{}
		sessionMigrationLocks.entries[key] = entry
	}
	entry.refs++
	sessionMigrationLocks.Unlock()

	drop := func(returnToken bool) {
		sessionMigrationLocks.Lock()
		defer sessionMigrationLocks.Unlock()
		if returnToken {
			entry.token <- struct{}{}
		}
		entry.refs--
		if entry.refs == 0 {
			delete(sessionMigrationLocks.entries, key)
		}
	}
	select {
	case <-ctx.Done():
		drop(false)
		return nil, fmt.Errorf("store: wait for session database migration owner %q: %w", key, ctx.Err())
	case <-entry.token:
	}
	return sync.OnceFunc(func() { drop(true) }), nil
}
