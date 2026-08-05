package sessiondb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"modernc.org/sqlite"
)

type sessionSQLiteConnector struct {
	driver   driver.Driver
	dsn      string
	path     string
	owner    store.SessionDBOwner
	readOnly bool
}

var _ driver.Connector = (*sessionSQLiteConnector)(nil)

func (c *sessionSQLiteConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if ctx == nil {
		return nil, errors.New("store: session sqlite connection context is required")
	}
	if c == nil || c.driver == nil {
		return nil, errors.New("store: session sqlite connector is required")
	}
	lease, err := AcquireFamilyLease(ctx, c.path)
	if err != nil {
		return nil, err
	}
	defer lease.Release()
	pinned, cleanPath, err := lease.pinFile(c.path)
	if err != nil {
		return nil, err
	}
	databaseID, err := verifySessionDBOwnerWithImmutableDriver(ctx, c.driver, cleanPath, c.owner)
	if err != nil {
		return nil, pinned.closeWithError(err)
	}
	if err := pinned.RequireCurrent(cleanPath); err != nil {
		return nil, pinned.closeWithError(err)
	}
	familyStamp, err := pinned.FamilyMutationStamp()
	if err != nil {
		return nil, pinned.closeWithError(err)
	}

	connection, err := c.driver.Open(c.dsn)
	if err != nil {
		return nil, pinned.closeWithError(err)
	}
	querier, err := c.verifyOpenedConnection(ctx, connection, pinned, familyStamp, databaseID)
	if err != nil {
		return nil, closePinnedSessionSQLiteDriverConnection(connection, pinned, err)
	}
	if c.readOnly {
		err = configureReadOnlySessionSQLiteConnection(ctx, querier)
	} else {
		err = configureWritableSessionSQLiteConnection(ctx, querier)
	}
	if err != nil {
		return nil, closePinnedSessionSQLiteDriverConnection(connection, pinned, err)
	}
	if err := pinned.Close(); err != nil {
		return nil, closeSessionSQLiteDriverConnection(connection, err)
	}
	return connection, nil
}

func (c *sessionSQLiteConnector) verifyOpenedConnection(
	ctx context.Context,
	connection driver.Conn,
	pinned *pinnedSessionDBFile,
	familyStamp sessionDBFamilyMutationStamp,
	databaseID string,
) (sqlite.ExecQuerierContext, error) {
	// Driver.Open establishes the main-file handle but does not execute schema
	// reads. Reject a substituted path before any query can open or update WAL
	// sidecars for the wrong filesystem family.
	if err := pinned.RequireFamilyUnchanged(familyStamp); err != nil {
		return nil, err
	}
	if err := pinned.RequireCurrent(c.path); err != nil {
		return nil, err
	}
	querier, ok := connection.(sqlite.ExecQuerierContext)
	if !ok {
		return nil, errors.New("store: sqlite driver connection does not support contextual queries")
	}
	if err := verifySessionDBOwnerConnection(ctx, querier, c.owner); err != nil {
		return nil, err
	}
	connectionDatabaseID, err := readSessionDBIdentityConnection(ctx, querier)
	if errors.Is(err, errSessionDBIdentityMissing) {
		connectionDatabaseID = ""
		err = nil
	}
	if err != nil {
		return nil, err
	}
	if connectionDatabaseID != databaseID {
		return nil, fmt.Errorf("%w: physical database identity changed", ErrSessionDBFamilyChanged)
	}
	if err := pinned.RequireFamilyUnchanged(familyStamp); err != nil {
		return nil, err
	}
	if err := pinned.RequireCurrent(c.path); err != nil {
		return nil, err
	}
	return querier, nil
}

func verifySessionDBOwnerWithImmutableDriver(
	ctx context.Context,
	sqliteDriver driver.Driver,
	path string,
	owner store.SessionDBOwner,
) (string, error) {
	connection, err := sqliteDriver.Open(sessionDBOwnerGuardDSN(path))
	if err != nil {
		return "", fmt.Errorf("store: open immutable session database owner guard %q: %w", path, err)
	}
	querier, ok := connection.(sqlite.ExecQuerierContext)
	if !ok {
		return "", closeSessionSQLiteDriverConnection(
			connection,
			errors.New("store: immutable sqlite owner guard does not support contextual queries"),
		)
	}
	if err := verifySessionDBOwnerConnection(ctx, querier, owner); err != nil {
		return "", closeSessionSQLiteDriverConnection(
			connection,
			fmt.Errorf("store: refuse session database %q before operational open: %w", path, err),
		)
	}
	databaseID, err := readSessionDBIdentityConnection(ctx, querier)
	if errors.Is(err, errSessionDBIdentityMissing) {
		databaseID = ""
		err = nil
	}
	if err != nil {
		return "", closeSessionSQLiteDriverConnection(connection, err)
	}
	if err := connection.Close(); err != nil {
		return "", fmt.Errorf("store: close immutable session database owner guard %q: %w", path, err)
	}
	return databaseID, nil
}

func (c *sessionSQLiteConnector) Driver() driver.Driver {
	return c.driver
}

func closeSessionSQLiteDriverConnection(connection driver.Conn, primaryErr error) error {
	if connection == nil {
		return primaryErr
	}
	if err := connection.Close(); err != nil {
		return errors.Join(primaryErr, fmt.Errorf("store: close rejected session sqlite connection: %w", err))
	}
	return primaryErr
}

func closePinnedSessionSQLiteDriverConnection(
	connection driver.Conn,
	pinned *pinnedSessionDBFile,
	primaryErr error,
) error {
	if pinned != nil {
		primaryErr = pinned.closeWithError(primaryErr)
	}
	return closeSessionSQLiteDriverConnection(connection, primaryErr)
}

func openGuardedSessionSQLite(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
	readOnly bool,
) (*sql.DB, error) {
	mode := "rw"
	if readOnly {
		mode = "ro"
	}
	connector := &sessionSQLiteConnector{
		driver:   &sqlite.Driver{},
		dsn:      sessionSQLiteOperationalDSN(path, mode),
		path:     path,
		owner:    owner,
		readOnly: readOnly,
	}
	db := sql.OpenDB(connector)
	if readOnly {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	} else {
		db.SetMaxOpenConns(8)
		db.SetMaxIdleConns(8)
	}
	if err := db.PingContext(ctx); err != nil {
		primaryErr := fmt.Errorf("store: open guarded session sqlite database %q: %w", path, err)
		if closeErr := db.Close(); closeErr != nil {
			primaryErr = errors.Join(
				primaryErr,
				fmt.Errorf("store: close rejected session sqlite database: %w", closeErr),
			)
		}
		return nil, primaryErr
	}
	return db, nil
}

func sessionSQLiteModeDSN(path, mode string) string {
	u := url.URL{Scheme: "file", Path: store.SqliteFilePath(path)}
	query := u.Query()
	query.Set("mode", mode)
	u.RawQuery = query.Encode()
	return u.String()
}

func sessionSQLiteOperationalDSN(path, mode string) string {
	return sessionSQLiteModeDSN(path, mode)
}

func verifySessionDBOwnerConnection(
	ctx context.Context,
	connection sqlite.ExecQuerierContext,
	expected store.SessionDBOwner,
) error {
	values, err := querySessionSQLiteConnectionRow(
		ctx,
		connection,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'session_db_owner'`,
		1,
	)
	if err != nil {
		return fmt.Errorf("store: inspect session database owner table: %w", err)
	}
	tableCount, ok := values[0].(int64)
	if !ok {
		return fmt.Errorf("store: inspect session database owner table count type %T", values[0])
	}
	if tableCount != 1 {
		return ErrSessionDBOwnerMissing
	}

	values, err = querySessionSQLiteConnectionRow(
		ctx,
		connection,
		`SELECT session_id, workspace_id FROM session_db_owner WHERE singleton = 1`,
		2,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSessionDBOwnerMissing
	}
	if err != nil {
		return fmt.Errorf("store: read session database owner: %w", err)
	}
	actual := store.SessionDBOwner{}
	if actual.SessionID, err = sessionSQLiteDriverString(values[0]); err != nil {
		return fmt.Errorf("store: read session database owner session id: %w", err)
	}
	if actual.WorkspaceID, err = sessionSQLiteDriverString(values[1]); err != nil {
		return fmt.Errorf("store: read session database owner workspace id: %w", err)
	}
	if actual != expected {
		return fmt.Errorf(
			"%w: expected session %q workspace %q, found session %q workspace %q",
			ErrSessionDBOwnerMismatch,
			expected.SessionID,
			expected.WorkspaceID,
			actual.SessionID,
			actual.WorkspaceID,
		)
	}
	return nil
}

func readSessionDBIdentityConnection(
	ctx context.Context,
	connection sqlite.ExecQuerierContext,
) (string, error) {
	values, err := querySessionSQLiteConnectionRow(
		ctx,
		connection,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'session_db_identity'`,
		1,
	)
	if err != nil {
		return "", fmt.Errorf("store: inspect session database identity table: %w", err)
	}
	tableCount, ok := values[0].(int64)
	if !ok {
		return "", fmt.Errorf("store: inspect session database identity table count type %T", values[0])
	}
	if tableCount != 1 {
		return "", errSessionDBIdentityMissing
	}
	values, err = querySessionSQLiteConnectionRow(
		ctx,
		connection,
		`SELECT database_id FROM session_db_identity WHERE singleton = 1`,
		1,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errSessionDBIdentityMissing
	}
	if err != nil {
		return "", fmt.Errorf("store: read session database identity: %w", err)
	}
	databaseID, err := sessionSQLiteDriverString(values[0])
	if err != nil {
		return "", fmt.Errorf("store: read session database identity: %w", err)
	}
	if len(databaseID) != 32 {
		return "", fmt.Errorf("store: session database identity is invalid")
	}
	return databaseID, nil
}

func querySessionSQLiteConnectionRow(
	ctx context.Context,
	connection sqlite.ExecQuerierContext,
	query string,
	columnCount int,
) (values []driver.Value, retErr error) {
	rows, err := connection.QueryContext(ctx, query, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("store: close session sqlite driver rows: %w", closeErr))
		}
	}()
	if got := len(rows.Columns()); got != columnCount {
		return nil, fmt.Errorf("store: session sqlite driver column count = %d, want %d", got, columnCount)
	}
	values = make([]driver.Value, columnCount)
	if err := rows.Next(values); errors.Is(err, io.EOF) {
		return nil, sql.ErrNoRows
	} else if err != nil {
		return nil, err
	}
	return values, nil
}

func sessionSQLiteDriverString(value driver.Value) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	default:
		return "", fmt.Errorf("unexpected sqlite string type %T", value)
	}
}

func configureReadOnlySessionSQLiteConnection(
	ctx context.Context,
	connection sqlite.ExecQuerierContext,
) error {
	if err := execSessionSQLiteConnectionPragma(
		ctx,
		connection,
		fmt.Sprintf("PRAGMA busy_timeout = %d", store.DefaultSQLiteBusyTimeoutMS),
	); err != nil {
		return err
	}
	return execSessionSQLiteConnectionPragma(ctx, connection, "PRAGMA query_only = ON")
}

func configureWritableSessionSQLiteConnection(
	ctx context.Context,
	connection sqlite.ExecQuerierContext,
) error {
	if err := execSessionSQLiteConnectionPragma(
		ctx,
		connection,
		fmt.Sprintf("PRAGMA busy_timeout = %d", store.DefaultSQLiteBusyTimeoutMS),
	); err != nil {
		return err
	}
	values, err := querySessionSQLiteConnectionRow(ctx, connection, "PRAGMA journal_mode = WAL", 1)
	if err != nil {
		return err
	}
	mode, err := sessionSQLiteDriverString(values[0])
	if err != nil {
		return err
	}
	if !strings.EqualFold(mode, "wal") {
		return fmt.Errorf("store: sqlite journal_mode = %q, want wal", mode)
	}
	for _, pragma := range []string{
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA wal_autocheckpoint = 0",
	} {
		if err := execSessionSQLiteConnectionPragma(ctx, connection, pragma); err != nil {
			return err
		}
	}
	return nil
}

func execSessionSQLiteConnectionPragma(
	ctx context.Context,
	connection sqlite.ExecQuerierContext,
	pragma string,
) error {
	if _, err := connection.ExecContext(ctx, pragma, nil); err != nil {
		return fmt.Errorf("store: configure session sqlite connection with %q: %w", pragma, err)
	}
	return nil
}
