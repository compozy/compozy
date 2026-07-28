package daemon

import "database/sql"

type extensionDBSource interface {
	DB() *sql.DB
}
