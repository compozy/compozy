package workflow

import (
	"context"

	"github.com/compozy/compozy/engine/stmanager"
	"github.com/nats-io/nats.go"
)

type Executor struct {
	stm *stmanager.Manager
	nc  *nats.Conn
}

func NewExecutor(nc *nats.Conn, stm *stmanager.Manager) *Executor {
	return &Executor{
		nc:  nc,
		stm: stm,
	}
}

func (e *Executor) Start(_ context.Context) error {
	return nil
}
