package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

type writeAdmission struct {
	mu          sync.Mutex
	changed     chan struct{}
	busy        bool
	interactive int
	users       int
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
		admission = &writeAdmission{changed: make(chan struct{})}
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
func (a *writeAdmission) acquire(ctx context.Context, background bool) (func(), error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !background {
		a.interactive++
		defer func() {
			a.interactive--
			a.notify()
		}()
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("store: wait for local sqlite writer admission: %w", err)
		}
		if !a.busy && (!background || a.interactive == 0) {
			a.busy = true
			return func() {
				a.mu.Lock()
				defer a.mu.Unlock()
				a.busy = false
				a.notify()
			}, nil
		}
		changed := a.changed
		a.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
		}
		a.mu.Lock()
	}
}

func (a *writeAdmission) notify() {
	close(a.changed)
	a.changed = make(chan struct{})
}

func (a *writeAdmission) interactiveWaiting() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.interactive > 0
}
