package daytona

import (
	"context"
	"io"
	"time"
)

type sandboxInfo struct {
	ID                 string
	APIURL             string
	SSHHost            string
	SSHAccessExpiresAt *time.Time
}

type transport interface {
	Dial(ctx context.Context, sandbox sandboxInfo, command string) (transportSession, error)
}

type transportSession interface {
	io.ReadWriteCloser
	CloseWrite() error
	Done() <-chan struct{}
	Wait() error
	Stop(ctx context.Context) error
	Stderr() string
}
