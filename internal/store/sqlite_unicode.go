package store

import (
	"database/sql/driver"
	"errors"
	"sync"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"modernc.org/sqlite"
)

// SQLite's built-in lower is ASCII-only. Catalog visible-text search uses the
// same Unicode lowercasing as active Go rows, without hydrating durable rows.
var registerSQLiteUnicodeSearch = sync.OnceValue(func() error {
	return sqlite.RegisterDeterministicScalarFunction("compozy_unicode_lower", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if args[0] == nil {
				return nil, nil
			}
			value, ok := args[0].(string)
			if !ok {
				return nil, errors.New("store: Unicode search requires text")
			}
			return LowerSessionCatalogSearchText(value), nil
		})
})

// LowerSessionCatalogSearchText preserves the desktop's full Unicode lowercase
// semantics. Casers are stateful, so each operation owns its transformer.
func LowerSessionCatalogSearchText(value string) string {
	return cases.Lower(language.Und).String(value)
}
