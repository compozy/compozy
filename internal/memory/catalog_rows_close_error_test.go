package memory

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

var errCatalogRowsClose = errors.New("memory catalog rows close failed")

const catalogRowsCloseErrorDriverName = "compozy-memory-catalog-rows-close-error"

func init() {
	sql.Register(catalogRowsCloseErrorDriverName, catalogRowsCloseErrorDriver{})
}

func TestCatalogListRowsCloseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		list func(context.Context, *catalog) error
	}{
		{
			name: "Should join dream-run scan and rows-close errors",
			list: func(ctx context.Context, catalog *catalog) error {
				_, err := catalog.listDreamRuns(ctx, "", DreamRunListQuery{})
				return err
			},
		},
		{
			name: "Should join daily-log scan and rows-close errors",
			list: func(ctx context.Context, catalog *catalog) error {
				_, err := catalog.listDailyLogs(ctx, "", DailyLogListQuery{})
				return err
			},
		},
		{
			name: "Should join decision scan and rows-close errors",
			list: func(ctx context.Context, catalog *catalog) error {
				_, err := catalog.listDecisions(ctx, "", DecisionListQuery{})
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.list(testutil.Context(t), openCatalogRowsCloseErrorCatalog(t))
			assertCatalogRowsCloseError(t, err)
		})
	}
}

func assertCatalogRowsCloseError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, errCatalogRowsClose) {
		t.Fatalf("list error = %v, want rows close error", err)
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("list error = %T, want joined scan and rows close errors", err)
	}
	var (
		hasCloseError bool
		hasScanError  bool
	)
	for _, candidate := range joined.Unwrap() {
		if errors.Is(candidate, errCatalogRowsClose) {
			hasCloseError = true
			continue
		}
		hasScanError = true
	}
	if !hasCloseError || !hasScanError {
		t.Fatalf("joined error branches = %#v, want independent scan and rows close errors", joined.Unwrap())
	}
}

type catalogRowsCloseErrorDriver struct{}

func (catalogRowsCloseErrorDriver) Open(string) (driver.Conn, error) {
	return catalogRowsCloseErrorConn{}, nil
}

type catalogRowsCloseErrorConn struct{}

func (catalogRowsCloseErrorConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("memory catalog rows close-error driver does not prepare statements")
}

func (catalogRowsCloseErrorConn) Close() error {
	return nil
}

func (catalogRowsCloseErrorConn) Begin() (driver.Tx, error) {
	return nil, errors.New("memory catalog rows close-error driver does not begin transactions")
}

func (catalogRowsCloseErrorConn) QueryContext(
	ctx context.Context,
	query string,
	_ []driver.NamedValue,
) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch {
	case strings.Contains(query, "FROM memory_consolidations"):
		return invalidCatalogRowsWithCloseError(12, 5), nil
	case strings.Contains(query, "FROM memory_events"):
		return invalidCatalogRowsWithCloseError(6, 5), nil
	case strings.Contains(query, "FROM memory_decisions"):
		return invalidCatalogRowsWithCloseError(22, 14), nil
	default:
		return nil, fmt.Errorf("memory catalog rows close-error driver: unexpected query %q", query)
	}
}

type catalogRowsCloseErrorRows struct {
	columns []string
	values  []driver.Value
	yielded bool
}

func (r *catalogRowsCloseErrorRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*catalogRowsCloseErrorRows) Close() error {
	return errCatalogRowsClose
}

func (r *catalogRowsCloseErrorRows) Next(destination []driver.Value) error {
	if r.yielded {
		return io.EOF
	}
	copy(destination, r.values)
	r.yielded = true
	return nil
}

func invalidCatalogRowsWithCloseError(columnCount, invalidColumn int) *catalogRowsCloseErrorRows {
	columns := make([]string, columnCount)
	values := make([]driver.Value, columnCount)
	for index := range columns {
		columns[index] = fmt.Sprintf("column_%d", index)
		values[index] = ""
	}
	values[invalidColumn] = "not-a-number"
	return &catalogRowsCloseErrorRows{columns: columns, values: values}
}

func openCatalogRowsCloseErrorCatalog(t *testing.T) *catalog {
	t.Helper()

	db, err := sql.Open(catalogRowsCloseErrorDriverName, "")
	if err != nil {
		t.Fatalf("sql.Open(memory catalog rows close-error driver) error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("sql.DB.Close(memory catalog rows close-error driver) error = %v", err)
		}
	})
	return &catalog{path: "memory-catalog-close-error.db", db: db}
}
