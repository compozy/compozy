package pty

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/procutil"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

type pipeProc struct {
	reader     *os.File
	input      *os.File
	cancel     context.CancelFunc
	waiter     processWaiter
	mu         sync.RWMutex
	command    *exec.Cmd
	pid        int
	startedAt  time.Time
	exit       *Exit
	ready      chan struct{}
	readyOnce  sync.Once
	startErr   error
	killSignal Signal
	closeOnce  sync.Once
	closeErr   error
}

func startPipe(parent context.Context, spec ProcSpec) (Proc, error) {
	if len(spec.Argv) == 0 || strings.TrimSpace(spec.Argv[0]) == "" {
		return nil, errors.New("terminal pipe: command is required")
	}
	environmentList := expand.ListEnviron(environment(spec.Env)...)
	path, err := interp.LookPathDir(spec.Cwd, environmentList, spec.Argv[0])
	if err != nil {
		return nil, fmt.Errorf("terminal pipe: resolve %q: %w", spec.Argv[0], err)
	}
	argv := append([]string(nil), spec.Argv...)
	argv[0] = path
	program, err := syntax.NewParser().Parse(strings.NewReader(`"$@"`), "terminal-exec")
	if err != nil {
		return nil, fmt.Errorf("terminal pipe: parse runner program: %w", err)
	}
	inputReader, inputWriter, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("terminal pipe: open input pipe: %w", err)
	}
	outputReader, outputWriter, err := os.Pipe()
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("terminal pipe: open output pipe: %w", err),
			inputReader.Close(),
			inputWriter.Close(),
		)
	}
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	proc := &pipeProc{
		reader: outputReader,
		input:  inputWriter,
		cancel: cancel,
		waiter: newProcessWaiter(),
		ready:  make(chan struct{}),
	}
	runner, err := interp.New(
		interp.Dir(spec.Cwd),
		interp.Env(environmentList),
		interp.Params(append([]string{"--"}, argv...)...),
		interp.StdIO(inputReader, outputWriter, outputWriter),
		interp.ExecHandlers(func(interp.ExecHandlerFunc) interp.ExecHandlerFunc {
			return proc.execHandler(spec.Env)
		}),
	)
	if err != nil {
		cancel()
		return nil, errors.Join(
			fmt.Errorf("terminal pipe: construct interpreter: %w", err),
			inputReader.Close(),
			inputWriter.Close(),
			outputReader.Close(),
			outputWriter.Close(),
		)
	}
	proc.waiter.start(func() waitResult {
		runErr := runner.Run(ctx, program)
		proc.signalReady(runErr)
		closeErr := errors.Join(inputReader.Close(), outputWriter.Close())
		exit := proc.pipeExit(runErr)
		if _, ok := runErr.(interp.ExitStatus); ok {
			runErr = nil
		}
		return waitResult{exit: exit, err: errors.Join(runErr, closeErr)}
	})
	<-proc.ready
	proc.mu.RLock()
	startErr := proc.startErr
	proc.mu.RUnlock()
	if startErr != nil {
		closeErr := proc.Close()
		_, waitErr := proc.Wait(context.Background())
		return nil, errors.Join(fmt.Errorf("terminal pipe: start %q: %w", spec.Argv[0], startErr), closeErr, waitErr)
	}
	return proc, nil
}

func (p *pipeProc) execHandler(env map[string]string) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		handler := interp.HandlerCtx(ctx)
		path, err := interp.LookPathDir(handler.Dir, handler.Env, args[0])
		if err != nil {
			return fmt.Errorf("terminal pipe: resolve %q: %w", args[0], err)
		}
		command := exec.CommandContext(ctx, path, args[1:]...)
		command.Dir = handler.Dir
		command.Env = environment(env)
		command.Stdin = handler.Stdin
		command.Stdout = handler.Stdout
		command.Stderr = handler.Stderr
		configurePipeCommand(command)
		if err := command.Start(); err != nil {
			return fmt.Errorf("terminal pipe: start %q: %w", args[0], err)
		}
		startedAt, err := procutil.StartedAt(command.Process.Pid)
		if err != nil {
			killErr := forcePipeCommand(command)
			waitErr := command.Wait()
			return errors.Join(
				fmt.Errorf("terminal pipe: observe %q start time: %w", args[0], err),
				killErr,
				normalizeExecWaitError(waitErr),
			)
		}
		if err := registerPipeCommand(command); err != nil {
			killErr := forcePipeCommand(command)
			waitErr := command.Wait()
			return errors.Join(
				fmt.Errorf("terminal pipe: register %q: %w", args[0], err),
				killErr,
				normalizeExecWaitError(waitErr),
			)
		}
		p.mu.Lock()
		p.command = command
		p.pid = command.Process.Pid
		p.startedAt = startedAt
		p.mu.Unlock()
		p.signalReady(nil)
		cancellationDone := make(chan struct{})
		var cancellationErr error
		stopCancellation := context.AfterFunc(ctx, func() {
			defer close(cancellationDone)
			cancellationErr = forcePipeCommand(command)
		})
		waitErr := command.Wait()
		commandExit := classifyExit(waitErr, command)
		if !stopCancellation() {
			<-cancellationDone
		}
		descendantErr := escalatePipeCommand(command, nil)
		p.mu.Lock()
		if p.command == command {
			p.command = nil
		}
		p.exit = &commandExit
		p.mu.Unlock()
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			if ctx.Err() != nil {
				return errors.Join(ctx.Err(), cancellationErr, descendantErr)
			}
			if descendantErr != nil {
				return descendantErr
			}
			exitCode := exitErr.ExitCode()
			if exitCode < 0 || exitCode > math.MaxUint8 {
				exitCode = 1
			}
			return interp.ExitStatus(exitCode)
		}
		if waitErr != nil {
			return errors.Join(
				fmt.Errorf("terminal pipe: wait %q: %w", args[0], waitErr), cancellationErr, descendantErr,
			)
		}
		return errors.Join(cancellationErr, descendantErr)
	}
}

func (p *pipeProc) Reader() io.Reader { return p.reader }

func (p *pipeProc) PID() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pid
}

func (p *pipeProc) ProcessGroupID() int { return p.PID() }

func (p *pipeProc) StartedAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.startedAt
}

func (p *pipeProc) Write(input []byte) (int, error) {
	written, err := p.input.Write(input)
	if err != nil {
		return written, fmt.Errorf("terminal pipe: write input: %w", err)
	}
	return written, nil
}

func (p *pipeProc) Resize(uint16, uint16) error { return ErrInteractiveUnavailable }

func (p *pipeProc) Wait(ctx context.Context) (Exit, error) {
	exit, err := p.waiter.wait(ctx)
	if err != nil {
		return exit, err
	}
	p.mu.RLock()
	signal := p.killSignal
	p.mu.RUnlock()
	return reportedExitForSignal(signal, exit), nil
}

func (p *pipeProc) Kill(signal Signal) error {
	p.mu.Lock()
	command := p.command
	if command == nil {
		p.mu.Unlock()
		if signal == SignalKILL || signal == SignalTERM || signal == SignalHUP {
			p.cancel()
			return nil
		}
		return errors.New("terminal pipe: no running external command")
	}
	previousSignal := p.killSignal
	p.killSignal = signal
	p.mu.Unlock()
	if err := signalPipeCommand(command, signal); err != nil {
		p.mu.Lock()
		if p.command == command && p.killSignal == signal {
			p.killSignal = previousSignal
		}
		p.mu.Unlock()
		return fmt.Errorf("terminal pipe: signal: %w", err)
	}
	if signal == SignalHUP || signal == SignalTERM {
		if err := escalatePipeCommand(command, func() {
			p.mu.Lock()
			p.killSignal = SignalKILL
			p.mu.Unlock()
		}); err != nil {
			return fmt.Errorf("terminal pipe: escalate process group: %w", err)
		}
	}
	return nil
}

func (p *pipeProc) Close() error {
	p.closeOnce.Do(func() {
		p.cancel()
		p.closeErr = errors.Join(p.input.Close(), p.reader.Close())
	})
	return p.closeErr
}

func (p *pipeProc) pipeExit(err error) Exit {
	p.mu.RLock()
	if p.exit != nil {
		exit := *p.exit
		p.mu.RUnlock()
		return exit
	}
	p.mu.RUnlock()
	if status, ok := errors.AsType[interp.ExitStatus](err); ok {
		code := int(status)
		return Exit{Cause: exitCauseExited, Code: &code}
	}
	if err == nil {
		code := 0
		return Exit{Cause: exitCauseExited, Code: &code}
	}
	return Exit{Cause: exitCauseUnknown}
}

func (p *pipeProc) signalReady(err error) {
	p.readyOnce.Do(func() {
		p.mu.Lock()
		p.startErr = err
		p.mu.Unlock()
		close(p.ready)
	})
}

func normalizeExecWaitError(err error) error {
	if errors.As(err, new(*exec.ExitError)) {
		return nil
	}
	return err
}
