package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

type writeAdmission struct {
	busy  chan struct{}
	users int
}

var writeAdmissions = struct {
	sync.Mutex
	byDB map[*sql.DB]*writeAdmission
}{byDB: make(map[*sql.DB]*writeAdmission)}

// Waiting local writers must not reserve pooled connections: the active
// transaction's commit fence may need a separate read connection from that pool.
func registerWriteAdmission(db *sql.DB) (*writeAdmission, func()) {
	writeAdmissions.Lock()
	admission := writeAdmissions.byDB[db]
	if admission == nil {
		admission = &writeAdmission{busy: make(chan struct{}, 1)}
		writeAdmissions.byDB[db] = admission
	}
	admission.users++
	writeAdmissions.Unlock()
	return admission, func() {
		writeAdmissions.Lock()
		defer writeAdmissions.Unlock()
		admission.users--
		if admission.users == 0 {
			delete(writeAdmissions.byDB, db)
		}
	}
}

// acquire waits for the local transaction owner without reserving a database
// connection or spending the retry budget for actual SQLite BUSY responses.
func (a *writeAdmission) acquire(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("store: wait for local sqlite writer admission: %w", err)
	}
	select {
	case a.busy <- struct{}{}:
		return func() { <-a.busy }, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("store: wait for local sqlite writer admission: %w", ctx.Err())
	}
}
