package subprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// PID returns the operating-system process identifier.
func (p *Process) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// Stdin exposes the subprocess stdin writer for callers that disable the shared transport.
func (p *Process) Stdin() io.WriteCloser {
	if p == nil {
		return nil
	}
	return p.stdin
}

// Stdout exposes the subprocess stdout reader for callers that disable the shared transport.
func (p *Process) Stdout() io.ReadCloser {
	if p == nil {
		return nil
	}
	return p.stdout
}

// Done closes when the subprocess exits.
func (p *Process) Done() <-chan struct{} {
	if p == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return p.done
}

// Wait blocks until the subprocess exits and returns its final error state.
func (p *Process) Wait() error {
	if p == nil {
		return nil
	}
	<-p.Done()
	p.waitMu.RLock()
	defer p.waitMu.RUnlock()
	return p.waitErr
}

// Stderr returns the captured stderr tail for diagnostics.
func (p *Process) Stderr() string {
	if p == nil || p.stderr == nil {
		return ""
	}
	return p.stderr.String()
}

// HandleMethod registers an inbound JSON-RPC request handler.
func (p *Process) HandleMethod(method string, handler HandlerFunc) error {
	if p == nil {
		return errors.New("subprocess: process is required")
	}
	if p.transport == nil {
		return ErrTransportDisabled
	}
	return p.transport.handleMethod(method, handler)
}

// Call sends an outbound JSON-RPC request and decodes the response.
func (p *Process) Call(ctx context.Context, method string, params, result any) error {
	if ctx == nil {
		return errors.New("subprocess: context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if p == nil {
		return errors.New("subprocess: process is required")
	}
	if p.transport == nil {
		return ErrTransportDisabled
	}

	switch p.currentState() {
	case processStateStarting:
		if method != initializeMethod {
			return fmt.Errorf("subprocess: call %q: %w", method, ErrNotInitialized)
		}
	case processStateDraining:
		if method != shutdownMethod {
			return fmt.Errorf("subprocess: call %q: %w", method, ErrShutdownInProgress)
		}
	case processStateStopped:
		if waitErr := p.Wait(); waitErr != nil {
			return waitErr
		}
		return errors.New("subprocess: process already stopped")
	}

	return p.transport.call(ctx, method, params, result)
}

// Shutdown performs cooperative shutdown when transport is enabled, then escalates signals if needed.
func (p *Process) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("subprocess: context is required")
	}
	if p == nil {
		return errors.New("subprocess: process is required")
	}

	select {
	case <-p.Done():
		return p.Wait()
	default:
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	p.markStopRequested()
	p.checkpointShutdownRequested(ctx)
	p.setState(processStateDraining)

	var errs []error
	var stopCtxErr error
	if p.transport != nil && p.currentState() != processStateStopped {
		shutdownCtx, cancel := context.WithTimeout(ctx, p.shutdownTimeout)
		defer cancel()

		var response ShutdownResponse
		err := p.Call(shutdownCtx, shutdownMethod, ShutdownRequest{
			Reason:     p.shutdownReason,
			DeadlineMS: p.shutdownTimeout.Milliseconds(),
		}, &response)
		if err != nil && !cooperativeShutdownTimedOut(ctx, shutdownCtx, err) {
			errs = append(errs, fmt.Errorf("subprocess: cooperative shutdown: %w", err))
		}
	}

	if err := p.closeInput(); err != nil {
		errs = append(errs, fmt.Errorf("subprocess: close stdin: %w", err))
	}

	if waitErr := p.waitWithContext(ctx, p.shutdownTimeout); waitErr == nil {
		stopCtxErr = shutdownContextError(ctx, stopCtxErr)
		return joinShutdownResult(errs, p.Wait(), stopCtxErr)
	} else if shutdownWaitInterruptedByCaller(ctx, waitErr) {
		stopCtxErr = shutdownContextError(ctx, stopCtxErr)
	} else if !errors.Is(waitErr, context.DeadlineExceeded) {
		stopCtxErr = shutdownContextError(ctx, stopCtxErr)
		return joinShutdownResult(errs, waitErr, stopCtxErr)
	}
	stopCtxErr = shutdownContextError(ctx, stopCtxErr)

	if err := terminateManagedProcess(p.cmd); err != nil {
		errs = append(errs, fmt.Errorf("subprocess: terminate process tree: %w", err))
	}

	waitCtx := shutdownWaitContext(ctx, stopCtxErr)
	if waitErr := p.waitWithContext(waitCtx, p.postSignalGrace); waitErr == nil {
		stopCtxErr = shutdownContextError(ctx, stopCtxErr)
		return joinShutdownResult(errs, p.Wait(), stopCtxErr)
	} else if shutdownWaitInterruptedByCaller(ctx, waitErr) {
		stopCtxErr = shutdownContextError(ctx, stopCtxErr)
		waitCtx = shutdownWaitContext(ctx, stopCtxErr)
	} else if !errors.Is(waitErr, context.DeadlineExceeded) {
		stopCtxErr = shutdownContextError(ctx, stopCtxErr)
		return joinShutdownResult(errs, waitErr, stopCtxErr)
	}
	stopCtxErr = shutdownContextError(ctx, stopCtxErr)

	if err := forceManagedProcessGroupExit(p.cmd, defaultProcessGroupWait); err != nil {
		errs = append(errs, fmt.Errorf("subprocess: kill process tree: %w", err))
	}

	waitErr := p.waitWithContext(waitCtx, defaultProcessGroupWait)
	if waitErr == nil {
		stopCtxErr = shutdownContextError(ctx, stopCtxErr)
		return joinShutdownResult(errs, p.Wait(), stopCtxErr)
	}
	stopCtxErr = shutdownContextError(ctx, stopCtxErr)
	return joinShutdownResult(errs, waitErr, stopCtxErr)
}

func joinShutdownResult(errs []error, waitErr error, stopCtxErr error) error {
	joined := errors.Join(append(errs, waitErr)...)
	if stopCtxErr == nil {
		return joined
	}
	return errors.Join(joined, stopCtxErr)
}

func shutdownContextError(ctx context.Context, existing error) error {
	if existing != nil || ctx == nil {
		return existing
	}
	return ctx.Err()
}

func shutdownWaitInterruptedByCaller(ctx context.Context, waitErr error) bool {
	if ctx == nil || waitErr == nil {
		return false
	}
	ctxErr := ctx.Err()
	return ctxErr != nil && errors.Is(waitErr, ctxErr)
}

func shutdownWaitContext(ctx context.Context, stopCtxErr error) context.Context {
	if ctx == nil {
		return context.Background()
	}
	if stopCtxErr != nil {
		return context.WithoutCancel(ctx)
	}
	return ctx
}

func cooperativeShutdownTimedOut(ctx context.Context, shutdownCtx context.Context, err error) bool {
	if err == nil || shutdownCtx == nil || shutdownCtx.Err() == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}
