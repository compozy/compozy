package pty

import (
	"context"
	"errors"
	"io"
)

var ErrInteractiveUnavailable = errors.New("interactive terminal unavailable")

type Mode string

const (
	ModePTY  Mode = "pty"
	ModePipe Mode = "pipe"
)

type Signal string

const (
	SignalINT  Signal = "INT"
	SignalTERM Signal = "TERM"
	SignalKILL Signal = "KILL"
	SignalHUP  Signal = "HUP"
)

type ProcSpec struct {
	Argv        []string
	Cwd         string
	Env         map[string]string
	Cols        uint16
	Rows        uint16
	Mode        Mode
	Title       string
	MarkerNonce string
}

type Exit struct {
	Cause  string
	Code   *int
	Signal *string
}

type PTY interface {
	Start(ctx context.Context, spec ProcSpec) (Proc, error)
}

type Proc interface {
	Reader() io.Reader
	Write(input []byte) (int, error)
	Resize(cols, rows uint16) error
	Wait(ctx context.Context) (Exit, error)
	Kill(signal Signal) error
	Close() error
}

type Platform struct{}

func New() *Platform {
	return &Platform{}
}

func (p *Platform) Start(ctx context.Context, spec ProcSpec) (Proc, error) {
	if ctx == nil {
		return nil, errors.New("terminal pty: start context is required")
	}
	if spec.Mode == ModePipe {
		return startPipe(ctx, spec)
	}
	return startInteractive(ctx, spec)
}

var _ PTY = (*Platform)(nil)
