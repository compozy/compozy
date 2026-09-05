package terminal

import (
	"context"

	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
)

type ProcSpec = terminalpty.ProcSpec
type PTY = terminalpty.PTY
type Proc = terminalpty.Proc

type Subscription interface {
	Frames() <-chan Frame
	Err() error
	Ack(bytes int)
	Resize(cols, rows uint16) error
	Close() error
}

type Handle interface {
	Info() Info
	MarkerNonce() string
	Attach(ctx context.Context, options AttachOptions) (Subscription, error)
	Write(ctx context.Context, actor Actor, input []byte) error
	Screen(ctx context.Context, options ReadOptions) (*ReadResult, error)
	Wait(ctx context.Context, condition WaitCondition) (*WaitResult, error)
	RequestInput(ctx context.Context, actor Actor, request InputRequest) (*InputOutcome, error)
	AnswerInput(ctx context.Context, actor Actor, id InputRequestID, answer InputAnswer) (*InputOutcome, error)
	RejectInput(ctx context.Context, actor Actor, id InputRequestID, reason string) error
	PendingInput(id InputRequestID) (*PendingInputRequest, error)
	Signal(ctx context.Context, actor Actor, signal Signal) error
	StartRecording(ctx context.Context, actor Actor) (RecordingRef, error)
	StopRecording(ctx context.Context, actor Actor) (RecordingRef, error)
}
