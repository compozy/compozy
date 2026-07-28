package clientstate

import (
	"context"
	"sync"
)

type sequenceRequest struct {
	ctx      context.Context
	execute  func(context.Context) error
	response chan error
}

type workspaceSequencer struct {
	requests chan sequenceRequest
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	hub      *subscriptionHub
}

func newWorkspaceSequencer(hub *subscriptionHub) *workspaceSequencer {
	sequencer := &workspaceSequencer{
		requests: make(chan sequenceRequest),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		hub:      hub,
	}
	go sequencer.run()
	return sequencer
}

func (s *workspaceSequencer) submit(
	ctx context.Context,
	execute func(context.Context) error,
) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	request := sequenceRequest{
		ctx:      ctx,
		execute:  execute,
		response: make(chan error, 1),
	}
	select {
	case s.requests <- request:
	case <-s.stop:
		return ErrClosed
	case <-ctx.Done():
		return contextError(ctx)
	}
	return <-request.response
}

func (s *workspaceSequencer) shutdown() {
	s.stopOnce.Do(func() {
		close(s.stop)
	})
	<-s.done
}

func (s *workspaceSequencer) run() {
	defer close(s.done)
	defer s.hub.closeAll(nil)
	for {
		select {
		case request := <-s.requests:
			if err := contextError(request.ctx); err != nil {
				request.response <- err
				continue
			}
			request.response <- request.execute(request.ctx)
		case <-s.stop:
			s.failWaitingRequests()
			return
		}
	}
}

func (s *workspaceSequencer) failWaitingRequests() {
	for {
		select {
		case request := <-s.requests:
			request.response <- ErrClosed
		default:
			return
		}
	}
}
