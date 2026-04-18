package store

import "time"

const (
	sqliteDriverName     = "sqlite"
	defaultBusyTimeoutMS = 5000
	defaultMaxOpenConns  = 8
	defaultMaxIdleConns  = 8

	timestampLayout = "2006-01-02T15:04:05.000000000Z"
)

// DefaultDrainTimeout is reserved for future writer-loop stores.
const DefaultDrainTimeout = 5 * time.Second
