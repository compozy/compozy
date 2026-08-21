package settings

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/testutil"
)

var errConfigApplyRecordRowsClose = errors.New("config apply record rows close failed")

const configApplyRecordRowsCloseErrorDriverName = "compozy-config-apply-record-rows-close-error"

func init() {
	sql.Register(configApplyRecordRowsCloseErrorDriverName, configApplyRecordRowsCloseErrorDriver{})
}

func TestConfigApplyRecordRepositoryListApplyRecordsRowsCloseErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should join apply-record scan and rows-close errors", func(t *testing.T) {
		t.Parallel()

		_, err := openConfigApplyRecordRowsCloseErrorRepository(t).ListApplyRecords(
			testutil.Context(t),
			ApplyRecordFilter{},
		)
		assertConfigApplyRecordRowsCloseError(t, err)
	})
}

func assertConfigApplyRecordRowsCloseError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, errConfigApplyRecordRowsClose) {
		t.Fatalf("ListApplyRecords() error = %v, want rows close error", err)
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("ListApplyRecords() error = %T, want joined scan and rows close errors", err)
	}
	var (
		hasCloseError bool
		hasScanError  bool
	)
	for _, candidate := range joined.Unwrap() {
		if errors.Is(candidate, errConfigApplyRecordRowsClose) {
			hasCloseError = true
			continue
		}
		hasScanError = true
	}
	if !hasCloseError || !hasScanError {
		t.Fatalf("joined error branches = %#v, want independent scan and rows close errors", joined.Unwrap())
	}
}

type configApplyRecordRowsCloseErrorDriver struct{}

func (configApplyRecordRowsCloseErrorDriver) Open(string) (driver.Conn, error) {
	return configApplyRecordRowsCloseErrorConn{}, nil
}

type configApplyRecordRowsCloseErrorConn struct{}

func (configApplyRecordRowsCloseErrorConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("config apply record rows close-error driver does not prepare statements")
}

func (configApplyRecordRowsCloseErrorConn) Close() error {
	return nil
}

func (configApplyRecordRowsCloseErrorConn) Begin() (driver.Tx, error) {
	return nil, errors.New("config apply record rows close-error driver does not begin transactions")
}

func (configApplyRecordRowsCloseErrorConn) QueryContext(
	ctx context.Context,
	query string,
	_ []driver.NamedValue,
) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !strings.Contains(query, "FROM config_apply_records") {
		return nil, fmt.Errorf("config apply record rows close-error driver: unexpected query %q", query)
	}
	return &configApplyRecordRowsCloseErrorRows{}, nil
}

type configApplyRecordRowsCloseErrorRows struct {
	yielded bool
}

func (*configApplyRecordRowsCloseErrorRows) Columns() []string {
	return []string{
		"id",
		"desired_config_hash",
		"active_config_hash",
		"generation",
		"actor",
		"write_target",
		"write_path",
		"diff_class",
		"status",
		"diagnostic_json",
		"created_at",
		"applied_at",
		"updated_at",
	}
}

func (*configApplyRecordRowsCloseErrorRows) Close() error {
	return errConfigApplyRecordRowsClose
}

func (r *configApplyRecordRowsCloseErrorRows) Next(destination []driver.Value) error {
	if r.yielded {
		return io.EOF
	}
	copy(destination, []driver.Value{
		"record-id",
		"desired-hash",
		"active-hash",
		"not-a-number",
		"runtime",
		"global-config",
		"/tmp/config.toml",
		"restart_required",
		"pending_apply",
		"",
		"2026-01-01T00:00:00Z",
		"",
		"2026-01-01T00:00:00Z",
	})
	r.yielded = true
	return nil
}

func openConfigApplyRecordRowsCloseErrorRepository(t *testing.T) *configApplyRecordRepository {
	t.Helper()

	db, err := sql.Open(configApplyRecordRowsCloseErrorDriverName, "")
	if err != nil {
		t.Fatalf("sql.Open(config apply record rows close-error driver) error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("sql.DB.Close(config apply record rows close-error driver) error = %v", err)
		}
	})
	return &configApplyRecordRepository{db: db}
}
