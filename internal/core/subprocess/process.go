package subprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const defaultSignalGracePeriod = 100 * time.Millisecond

// LaunchConfig describes how to start and manage a subprocess.
type LaunchConfig struct {
	Command           []string
	Env               []string
	WorkingDir        string
	WaitDelay         time.Duration
	WaitErrorPrefix   string
	SignalGracePeriod time.Duration
}

// Process manages a spawned subprocess and its lifetime.
type Process struct {
	cmd               *exec.Cmd
	stdin             io.WriteCloser
	stdout            io.ReadCloser
	stderr            *LockedBuffer
	waitErrorPrefix   string
	signalGracePeriod time.Duration

	mu        sync.Mutex
	waitDone  chan struct{}
	waitErr   error
	forced    bool
	forceOnce sync.Once
	closeOnce sync.Once
}

// Launch starts a subprocess with the provided configuration.
func Launch(ctx context.Context, cfg LaunchConfig) (*Process, error) {
	if len(cfg.Command) == 0 {
		return nil, fmt.Errorf("missing subprocess command")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// #nosec G204 -- this package exists to launch configured subprocess commands.
	cmd := exec.CommandContext(ctx, cfg.Command[0], cfg.Command[1:]...)
	cmd.Env = append([]string(nil), cfg.Env...)
	cmd.Dir = strings.TrimSpace(cfg.WorkingDir)
	cmd.WaitDelay = cfg.WaitDelay
	cmd.Cancel = func() error {
		return forceTerminateProcess(cmd)
	}
	if err := configureCommand(cmd); err != nil {
		return nil, err
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create subprocess stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("create subprocess stdout pipe: %w", err)
	}

	stderr := &LockedBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}

	process := &Process{
		cmd:               cmd,
		stdin:             stdin,
		stdout:            stdout,
		stderr:            stderr,
		waitErrorPrefix:   cfg.WaitErrorPrefix,
		signalGracePeriod: cfg.SignalGracePeriod,
		waitDone:          make(chan struct{}),
	}
	if process.signalGracePeriod <= 0 {
		process.signalGracePeriod = defaultSignalGracePeriod
	}

	go process.waitForExit()

	return process, nil
}

// Stdin returns the subprocess stdin pipe.
func (p *Process) Stdin() io.WriteCloser {
	if p == nil {
		return nil
	}
	return p.stdin
}

// Stdout returns the subprocess stdout pipe.
func (p *Process) Stdout() io.ReadCloser {
	if p == nil {
		return nil
	}
	return p.stdout
}

// StderrBuffer returns the subprocess stderr capture buffer.
func (p *Process) StderrBuffer() *LockedBuffer {
	if p == nil {
		return nil
	}
	return p.stderr
}

// Done returns a channel that closes when the subprocess exits.
func (p *Process) Done() <-chan struct{} {
	if p == nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return p.waitDone
}

// Forced reports whether shutdown escalated beyond cooperative stdin closure.
func (p *Process) Forced() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.forced
}

// Wait blocks until the subprocess exits and returns the normalized wait error.
func (p *Process) Wait() error {
	if p == nil {
		return nil
	}
	<-p.waitDone

	p.mu.Lock()
	defer p.mu.Unlock()
	return p.waitErr
}

// CloseInput closes the subprocess stdin pipe.
func (p *Process) CloseInput() {
	if p == nil {
		return
	}
	p.closeOnce.Do(func() {
		if p.stdin != nil {
			_ = p.stdin.Close()
		}
	})
}

// Shutdown attempts cooperative exit first, then escalates to SIGTERM and SIGKILL.
func (p *Process) Shutdown(timeout time.Duration) error {
	if p == nil {
		return nil
	}

	p.CloseInput()
	if p.waitWithin(timeout) {
		return suppressWaitDelay(p.Wait())
	}

	p.markForced()
	if err := terminateProcess(p.cmd); err != nil {
		return err
	}
	if p.waitWithin(p.signalGracePeriod) {
		return suppressWaitDelay(p.Wait())
	}
	if err := p.forceExit(); err != nil {
		return err
	}
	return suppressWaitDelay(p.Wait())
}

// Kill force-terminates the subprocess immediately.
func (p *Process) Kill() error {
	if p == nil {
		return nil
	}

	p.CloseInput()
	p.markForced()
	if err := p.forceExit(); err != nil {
		return err
	}
	return suppressWaitDelay(p.Wait())
}

func (p *Process) waitForExit() {
	err := p.cmd.Wait()

	p.mu.Lock()
	p.waitErr = NormalizeWaitError(p.waitErrorPrefix, err)
	if p.forced {
		p.waitErr = nil
	}
	close(p.waitDone)
	p.mu.Unlock()
}

func (p *Process) markForced() {
	p.mu.Lock()
	p.forced = true
	p.mu.Unlock()
}

func (p *Process) forceExit() error {
	var result error
	p.forceOnce.Do(func() {
		if p.cmd == nil {
			return
		}
		if p.cmd.Cancel != nil {
			result = p.cmd.Cancel()
			return
		}
		result = forceTerminateProcess(p.cmd)
	})
	return result
}

func (p *Process) waitWithin(timeout time.Duration) bool {
	if timeout <= 0 {
		select {
		case <-p.waitDone:
			return true
		default:
			return false
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-p.waitDone:
		return true
	case <-timer.C:
		return false
	}
}

// MergeEnvironment returns the current environment with base and extra assignments appended.
func MergeEnvironment(base map[string]string, extra map[string]string) []string {
	env := append([]string(nil), os.Environ()...)
	for key, value := range base {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	for key, value := range extra {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

// NormalizeWaitError wraps subprocess wait errors with a caller-provided prefix.
func NormalizeWaitError(prefix string, err error) error {
	if err == nil {
		return nil
	}
	if strings.TrimSpace(prefix) == "" {
		return fmt.Errorf("wait for subprocess: %w", err)
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

// LockedBuffer is a concurrency-safe stderr capture buffer.
type LockedBuffer struct {
	mu  sync.Mutex
	buf []byte
}

// Write appends bytes to the buffer.
func (b *LockedBuffer) Write(p []byte) (int, error) {
	if b == nil {
		return 0, errors.New("write locked buffer: nil receiver")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

// String returns the current buffer contents as a string.
func (b *LockedBuffer) String() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

func suppressWaitDelay(err error) error {
	if errors.Is(err, exec.ErrWaitDelay) {
		return nil
	}
	return err
}
