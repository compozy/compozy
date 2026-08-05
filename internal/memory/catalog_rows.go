package memory

import (
	"database/sql"
	"errors"
	"fmt"
)

func closeCatalogRows(rows *sql.Rows, operation string, target *error) {
	if closeErr := rows.Close(); closeErr != nil {
		*target = errors.Join(
			*target,
			fmt.Errorf("memory: close %s rows: %w", operation, closeErr),
		)
	}
}
