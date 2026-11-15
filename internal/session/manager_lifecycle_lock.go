package session

import (
	"context"
	"sync"
)

// lifecycleMutex serializes manager mutations while allowing bounded delete admission.
// The zero value is ready to use, like the mutex it replaces.
type lifecycleMutex struct {
	once  sync.Once
	token chan struct{}
}

func (m *lifecycleMutex) init() {
	m.once.Do(func() {
		m.token = make(chan struct{}, 1)
		m.token <- struct{}{}
	})
}

func (m *lifecycleMutex) Lock() {
	m.init()
	<-m.token
}

func (m *lifecycleMutex) LockContext(ctx context.Context) error {
	m.init()
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.token:
		if err := ctx.Err(); err != nil {
			m.Unlock()
			return err
		}
		return nil
	}
}

func (m *lifecycleMutex) Unlock() {
	m.init()
	select {
	case m.token <- struct{}{}:
	default:
		panic("session: unlock of unlocked lifecycle mutex")
	}
}
