package globaldb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

var errQueryRowsClose = errors.New("query rows close failed")

const queryRowsCloseErrorDriverName = "agh-query-rows-close-error"

func init() {
	sql.Register(queryRowsCloseErrorDriverName, queryRowsCloseErrorDriver{})
}

type queryRowsCloseErrorDriver struct{}

func (queryRowsCloseErrorDriver) Open(string) (driver.Conn, error) {
	return queryRowsCloseErrorConn{}, nil
}

type queryRowsCloseErrorConn struct{}

func (queryRowsCloseErrorConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("query rows close-error driver does not prepare statements")
}

func (queryRowsCloseErrorConn) Close() error {
	return nil
}

func (queryRowsCloseErrorConn) Begin() (driver.Tx, error) {
	return nil, errors.New("query rows close-error driver does not begin transactions")
}

func (queryRowsCloseErrorConn) QueryContext(
	ctx context.Context,
	query string,
	_ []driver.NamedValue,
) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch {
	case strings.Contains(query, "FROM task_runs"):
		return invalidRowsWithCloseError(len(strings.Split(taskRunSelectColumnsSQL, ","))), nil
	case strings.Contains(query, "FROM loop_runs"):
		return invalidRowsWithCloseError(len(strings.Split(loopRunSelectColumnsSQL, ","))), nil
	case strings.Contains(query, "FROM model_catalog_reasoning_efforts"):
		return invalidRowsWithCloseError(4), nil
	default:
		return nil, fmt.Errorf("query rows close-error driver: unexpected query %q", query)
	}
}

type queryRowsCloseErrorRows struct {
	columns []string
	values  []driver.Value
	yielded bool
}

func (r *queryRowsCloseErrorRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*queryRowsCloseErrorRows) Close() error {
	return errQueryRowsClose
}

func (r *queryRowsCloseErrorRows) Next(destination []driver.Value) error {
	if r.yielded {
		return io.EOF
	}
	copy(destination, r.values)
	r.yielded = true
	return nil
}

func invalidRowsWithCloseError(columnCount int) *queryRowsCloseErrorRows {
	columns := make([]string, columnCount)
	values := make([]driver.Value, columnCount)
	for index := range columns {
		columns[index] = fmt.Sprintf("column_%d", index)
		values[index] = ""
	}
	values[0] = nil
	return &queryRowsCloseErrorRows{columns: columns, values: values}
}

func openQueryRowsCloseErrorGlobalDB(t *testing.T) *GlobalDB {
	t.Helper()

	db, err := sql.Open(queryRowsCloseErrorDriverName, "")
	if err != nil {
		t.Fatalf("sql.Open(query rows close-error driver) error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("sql.DB.Close(query rows close-error driver) error = %v", err)
		}
	})
	globalDB := &GlobalDB{db: db, now: func() time.Time { return time.Unix(0, 0).UTC() }}
	globalDB.initializeRepositories(openConfig{})
	return globalDB
}
