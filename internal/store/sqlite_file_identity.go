package store

import (
	"database/sql"
	"database/sql/driver"
	"os"
)

// File identity, rather than a pool pointer or path spelling, joins separately
// opened handles for one SQLite file. The connector owns this immutable value;
// admission registries retain handles only while a writer is registered.
type sqliteFileDriver struct {
	driver.Driver
	info os.FileInfo
}

func (d *sqliteFileDriver) SQLiteFileIdentity() os.FileInfo { return d.info }

type sqliteFileConnector struct {
	driver.Connector
	identity *sqliteFileDriver
}

func (c *sqliteFileConnector) Driver() driver.Driver { return c.identity }

func sameWriteDatabase(first, second *sql.DB) bool {
	if first == second {
		return true
	}
	if first == nil || second == nil {
		return false
	}
	type identified interface{ SQLiteFileIdentity() os.FileInfo }
	left, leftOK := first.Driver().(identified)
	right, rightOK := second.Driver().(identified)
	if !leftOK || !rightOK {
		return false
	}
	a, b := left.SQLiteFileIdentity(), right.SQLiteFileIdentity()
	return a != nil && b != nil && os.SameFile(a, b)
}
