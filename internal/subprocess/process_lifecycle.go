package subprocess

import (
	"context"
	"errors"
	"fmt"

	"os"

	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/toolruntime"
)

func (p *Process) checkpointShutdownRequested(ctx context.Context) {
	if p == nil || p.processRecord == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := p.processRecord.Checkpoint(context.WithoutCancel(ctx), toolruntime.ProcessCheckpoint{
		State: toolruntime.ProcessStateInterrupting,
		Error: "subprocess shutdown requested",
	}); err != nil && p.logger != nil {
		p.logger.Warn("subprocess: checkpoint process interrupt", "pid", p.PID(), "error", err)
	}
}

func (p *Process) waitForExit(ctx context.Context) {
	waitErr := p.cmd.Wait()
	p.cancelLifecycle()

	var groupWaitErr error
	if p.stopWasRequested() {
		groupWaitErr = forceManagedProcessGroupExit(p.cmd, defaultProcessGroupWait)
	}

	if p.stopWasRequested() {
		waitErr = nil
		if groupWaitErr != nil {
			waitErr = fmt.Errorf("subprocess: wait for process tree exit: %w", groupWaitErr)
		}
	} else if waitErr != nil {
		waitErr = fmt.Errorf("subprocess: process exited: %w", attachStderr(waitErr, p.Stderr()))
	}

	if p.transport != nil {
		p.transport.shutdown(waitErr)
	}
	p.stopHealthMonitor()
	p.setState(processStateStopped)

	transportErr := p.currentTransportError()
	if p.stopWasRequested() && isBenignTransportShutdownError(transportErr) {
		transportErr = nil
	}
	if waitErr == nil && transportErr != nil {
		waitErr = transportErr
	} else if waitErr != nil && transportErr != nil {
		waitErr = errors.Join(waitErr, transportErr)
	}

	p.waitMu.Lock()
	p.waitErr = waitErr
	p.waitMu.Unlock()
	if p.processRecord != nil {
		if err := p.processRecord.Complete(ctx, toolruntime.ProcessCompletion{Err: waitErr}); err != nil &&
			p.logger != nil {
			p.logger.Warn("subprocess: complete process record", "pid", p.PID(), "error", err)
		}
	}

	close(p.done)
}

func (p *Process) waitWithContext(ctx context.Context, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = time.Millisecond
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-p.Done():
		return nil
	case <-waitCtx.Done():
		return waitCtx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Process) closeInput() error {
	p.closeInputMu.Lock()
	defer p.closeInputMu.Unlock()
	if p.inputClosed || p.stdin == nil {
		return nil
	}
	p.inputClosed = true
	if err := p.stdin.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return err
	}
	return nil
}

func (p *Process) currentState() processState {
	p.stateMu.RLock()
	defer p.stateMu.RUnlock()
	return p.state
}

func (p *Process) setState(state processState) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	if p.state == processStateStopped {
		return
	}
	p.state = state
}

func (p *Process) markReady() {
	p.setState(processStateReady)
}

func (p *Process) markStopRequested() {
	p.stopMu.Lock()
	defer p.stopMu.Unlock()
	p.stopRequested = true
}

func (p *Process) stopWasRequested() bool {
	p.stopMu.RLock()
	defer p.stopMu.RUnlock()
	return p.stopRequested
}

func (p *Process) recordTransportError(err error) {
	if err == nil {
		return
	}
	p.transportErrMu.Lock()
	defer p.transportErrMu.Unlock()
	if p.transportErr == nil {
		p.transportErr = err
	}
}

func (p *Process) currentTransportError() error {
	p.transportErrMu.RLock()
	defer p.transportErrMu.RUnlock()
	return p.transportErr
}

func attachStderr(err error, stderr string) error {
	if stderr == "" {
		return err
	}
	return fmt.Errorf("%w: stderr=%s", err, stderr)
}

func isBenignTransportShutdownError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrClosed) {
		return true
	}
	return strings.Contains(err.Error(), "file already closed")
}

func (cfg LaunchConfig) defaultShutdownReason() string {
	if cfg.ShutdownReason != "" {
		return cfg.ShutdownReason
	}
	return "daemon_shutdown"
}

type boundedBuffer struct {
	mu    sync.Mutex
	buf   []byte
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.limit <= 0 {
		return len(p), nil
	}

	if len(p) >= b.limit {
		b.buf = append(b.buf[:0], p[len(p)-b.limit:]...)
		return len(p), nil
	}

	if overflow := len(b.buf) + len(p) - b.limit; overflow > 0 {
		copy(b.buf, b.buf[overflow:])
		b.buf = b.buf[:len(b.buf)-overflow]
	}

	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}
