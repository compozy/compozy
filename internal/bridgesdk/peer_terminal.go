package bridgesdk

import (
	"context"
	"errors"
	"fmt"
)

// ErrPeerClosed is the terminal cause observed by calls made after the peer
// reaches clean EOF. Serve normalizes that clean shutdown to nil.
var ErrPeerClosed = errors.New("bridgesdk: peer closed")

var errPeerAlreadyServing = errors.New("bridgesdk: peer already serving")

type peerPhase uint8

const (
	peerPhaseOpen peerPhase = iota
	peerPhaseTerminating
	peerPhaseTerminal
)

func (p *Peer) beginServe(ctx context.Context) error {
	p.stateMu.Lock()
	if p.serveStarted {
		p.stateMu.Unlock()
		return errPeerAlreadyServing
	}
	if p.phase == peerPhaseOpen {
		p.serveStarted = true
		p.stateMu.Unlock()
		return nil
	}
	done := p.terminalDone
	phase := p.phase
	p.stateMu.Unlock()

	if phase == peerPhaseTerminating {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return p.currentTerminalError()
}

func (p *Peer) admitCall(ctx context.Context, id string, responseCh chan rpcResult) error {
	p.stateMu.Lock()
	if p.phase == peerPhaseOpen {
		if err := ctx.Err(); err != nil {
			p.stateMu.Unlock()
			return err
		}
		p.pending[id] = responseCh
		p.stateMu.Unlock()
		return nil
	}
	done := p.terminalDone
	phase := p.phase
	p.stateMu.Unlock()

	if phase == peerPhaseTerminating {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return p.currentTerminalError()
}

func (p *Peer) cancelPendingCall(id string) <-chan struct{} {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	if p.phase == peerPhaseOpen {
		delete(p.pending, id)
		return nil
	}
	return p.terminalDone
}

func (p *Peer) startDispatch(ctx context.Context, envelope rpcEnvelope) bool {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	if p.phase != peerPhaseOpen {
		return false
	}
	p.wg.Go(func() {
		p.dispatchRequest(ctx, envelope)
	})
	return true
}

func (p *Peer) terminate(cause error) error {
	p.publishTerminal(cause)
	return p.currentTerminalError()
}

func (p *Peer) failTransport(ctx context.Context, cause error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		cause = ctxErr
	}
	p.publishTerminal(cause)
}

func (p *Peer) publishTerminal(cause error) {
	if cause == nil {
		cause = ErrPeerClosed
	}

	p.stateMu.Lock()
	switch p.phase {
	case peerPhaseTerminal:
		p.stateMu.Unlock()
		return
	case peerPhaseTerminating:
		done := p.terminalDone
		p.stateMu.Unlock()
		<-done
		return
	case peerPhaseOpen:
		p.phase = peerPhaseTerminating
	}
	pending := p.pending
	p.pending = make(map[string]chan rpcResult)
	stdin := p.stdin
	p.stateMu.Unlock()

	terminalErr := cause
	if err := stdin.Close(); err != nil {
		terminalErr = errors.Join(
			cause,
			fmt.Errorf("bridgesdk: close peer input: %w", err),
		)
	}

	p.stateMu.Lock()
	p.terminalErr = terminalErr
	p.stateMu.Unlock()

	for _, responseCh := range pending {
		close(responseCh)
	}

	p.stateMu.Lock()
	p.phase = peerPhaseTerminal
	close(p.terminalDone)
	p.stateMu.Unlock()
}

func (p *Peer) currentTerminalError() error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	if p.terminalErr == nil {
		return ErrPeerClosed
	}
	return p.terminalErr
}
