package postgres

import (
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

type row struct{ N int }

func TestScanHelpers(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("mock: %v", err)
	}
	defer mock.Close()
	ctx := t.Context()

	// scanOne
	mock.ExpectQuery("SELECT 1").WillReturnRows(
		mock.NewRows([]string{"n"}).AddRow(1),
	)
	var one row
	if err := scanOne(ctx, mock, &one, "SELECT 1"); err != nil {
		t.Fatalf("scanOne: %v", err)
	}
	if one.N != 1 {
		t.Fatalf("unexpected: %v", one.N)
	}

	// scanAll
	mock.ExpectQuery("SELECT 1 UNION SELECT 2").WillReturnRows(
		mock.NewRows([]string{"n"}).AddRow(1).AddRow(2),
	)
	var all []row
	if err := scanAll(ctx, mock, &all, "SELECT 1 UNION SELECT 2"); err != nil {
		t.Fatalf("scanAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("unexpected len: %d", len(all))
	}
}
