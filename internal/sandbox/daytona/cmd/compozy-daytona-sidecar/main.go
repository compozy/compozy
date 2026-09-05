package main

import (
	"bytes"
	"context"

	"errors"

	"fmt"
	"io"
	"os"

	"os/exec"

	"sync"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

const (
	version               = "compozy-daytona-launcher-sidecar-v2"
	serverStdoutFrame     = 0x01
	serverStderrFrame     = 0x02
	serverExitFrame       = 0x03
	serverErrorFrame      = 0x04
	clientStdinFrame      = 0x01
	clientCloseStdinFrame = 0x02
	clientStopFrame       = 0x03
	stopTimeout           = 5 * time.Second
	stdoutBufferLimit     = 4 * 1024 * 1024
	stderrBufferLimit     = 1024 * 1024
	stderrTruncatedMarker = "\n[stderr truncated]\n"
)

var errOutputBufferExceeded = errors.New("sidecar output buffer exceeded")

type launchRequest struct {
	Command string `json:"command"`
	ID      string `json:"id,omitempty"`
}

type launchResponse struct {
	ID string `json:"id"`
}

type healthResponse struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
}

type exitPayload struct {
	ExitCode int    `json:"exitCode"`
	Stderr   string `json:"stderr"`
}

type frameWriter func([]byte) error

type chunkQueue struct {
	mu            sync.Mutex
	cond          *sync.Cond
	chunks        [][]byte
	bufferedBytes int
	maxBytes      int
	closed        bool
}

func newChunkQueue() *chunkQueue {
	q := &chunkQueue{maxBytes: stdoutBufferLimit}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *chunkQueue) Push(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	copied := append([]byte(nil), chunk...)
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil
	}
	if q.maxBytes > 0 && q.bufferedBytes+len(copied) > q.maxBytes {
		q.closed = true
		q.cond.Broadcast()
		return fmt.Errorf("%w: stdout buffer exceeds %d bytes", errOutputBufferExceeded, q.maxBytes)
	}
	q.chunks = append(q.chunks, copied)
	q.bufferedBytes += len(copied)
	q.cond.Signal()
	return nil
}

func (q *chunkQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	q.cond.Broadcast()
}

func (q *chunkQueue) Pop() ([]byte, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.chunks) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.chunks) == 0 {
		return nil, false
	}
	chunk := q.chunks[0]
	q.chunks[0] = nil
	q.chunks = q.chunks[1:]
	q.bufferedBytes -= len(chunk)
	return chunk, true
}

type managedProcess struct {
	id              string
	command         string
	cmd             *exec.Cmd
	cancel          context.CancelFunc
	stdin           io.WriteCloser
	stdinMu         sync.Mutex
	stdout          *chunkQueue
	stderr          bytes.Buffer
	stderrMu        sync.Mutex
	stderrTruncated bool
	done            chan struct{}
	exitCode        int
	exitVerified    bool
	stopMu          sync.Mutex
	stopRun         *managedStopRun
	streamMu        sync.Mutex
	streamClaimed   bool
}

func newManagedProcess(command string) (*managedProcess, error) {
	processID, err := randomID()
	if err != nil {
		return nil, err
	}
	return startManagedProcess(command, processID)
}

func startManagedProcess(command, processID string) (*managedProcess, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "/bin/sh", "-lc", command)
	procutil.ConfigureCommandProcessGroup(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start command: %w", err)
	}
	if err := procutil.RegisterCommandProcessGroup(cmd); err != nil {
		cancel()
		return nil, errors.Join(
			fmt.Errorf("register command process group: %w", err),
			cleanupStartedManagedCommand(cmd),
		)
	}
	process := &managedProcess{
		id:       processID,
		command:  command,
		cmd:      cmd,
		cancel:   cancel,
		stdin:    stdin,
		stdout:   newChunkQueue(),
		done:     make(chan struct{}),
		exitCode: -1,
	}
	go process.captureStdout(stdout)
	go process.captureStderr(stderr)
	go process.wait()
	return process, nil
}

func cleanupStartedManagedCommand(cmd *exec.Cmd) error {
	var errs []error
	if err := procutil.SignalCommandProcessGroup(cmd, syscall.SIGKILL); err != nil {
		errs = append(errs, fmt.Errorf("signal command process group: %w", err))
	}
	if err := cmd.Wait(); err != nil {
		errs = append(errs, fmt.Errorf("wait after cleanup: %w", err))
	}
	if err := procutil.KillCommandProcessGroupAndWait(cmd, stopTimeout); err != nil {
		errs = append(errs, fmt.Errorf("wait for command process group exit: %w", err))
	}
	return errors.Join(errs...)
}

func (p *managedProcess) captureStdout(stdout io.ReadCloser) {
	defer p.stdout.Close()
	defer p.reportPipeClose("stdout", stdout)
	buf := make([]byte, 64*1024)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			if pushErr := p.stdout.Push(buf[:n]); pushErr != nil {
				p.appendStderr(pushErr.Error() + "\n")
				if stopErr := p.Stop(); stopErr != nil {
					p.appendStderr(fmt.Sprintf("stop after stdout buffer overflow: %v\n", stopErr))
				}
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				p.appendStderr(fmt.Sprintf("stdout read error: %v\n", err))
			}
			return
		}
	}
}

func (p *managedProcess) captureStderr(stderr io.ReadCloser) {
	defer p.reportPipeClose("stderr", stderr)
	buf := make([]byte, 64*1024)
	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			p.appendStderr(string(buf[:n]))
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				p.appendStderr(fmt.Sprintf("stderr read error: %v\n", err))
			}
			return
		}
	}
}

func (p *managedProcess) reportPipeClose(name string, pipe io.Closer) {
	if err := pipe.Close(); err != nil {
		p.appendStderr(fmt.Sprintf("%s close error: %v\n", name, err))
	}
}

func (p *managedProcess) wait() {
	defer close(p.done)
	waitErr := p.cmd.Wait()
	if waitErr != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](waitErr); ok {
			p.exitCode = exitErr.ExitCode()
		} else {
			p.appendStderr(fmt.Sprintf("wait error: %v\n", waitErr))
			p.exitCode = 1
		}
	} else {
		p.exitCode = 0
	}
	if err := procutil.KillCommandProcessGroupAndWait(p.cmd, stopTimeout); err != nil {
		p.appendStderr(fmt.Sprintf("release process group error: %v\n", err))
		if p.exitCode == 0 {
			p.exitCode = 1
		}
	} else {
		p.exitVerified = true
	}
}

func (p *managedProcess) WriteStdin(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	p.stdinMu.Lock()
	stdin := p.stdin
	p.stdinMu.Unlock()
	if stdin == nil {
		return errors.New("stdin is closed")
	}
	if _, err := stdin.Write(data); err != nil {
		return err
	}
	return nil
}

func (p *managedProcess) CloseStdin() error {
	p.stdinMu.Lock()
	stdin := p.stdin
	p.stdin = nil
	p.stdinMu.Unlock()
	if stdin == nil {
		return nil
	}
	err := stdin.Close()
	// exec.Cmd.Wait also closes its stdin pipe; closing it again is already satisfied.
	if errors.Is(err, os.ErrClosed) {
		return nil
	}
	return err
}

type managedStopRun struct {
	done chan struct{}
	err  error
}

func (p *managedProcess) Stop() error {
	p.stopMu.Lock()
	run := p.stopRun
	if run != nil && !p.canRetryStop(run) {
		p.stopMu.Unlock()
		<-run.done
		return run.err
	}
	run = &managedStopRun{done: make(chan struct{})}
	p.stopRun = run
	p.stopMu.Unlock()
	run.err = p.stopAttempt()
	close(run.done)
	return run.err
}

func (p *managedProcess) canRetryStop(run *managedStopRun) bool {
	select {
	case <-run.done:
		if run.err == nil || p.cmd.Process == nil {
			return false
		}
	default:
		return false
	}
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

func (p *managedProcess) stopAttempt() (stopErr error) {
	if err := p.CloseStdin(); err != nil {
		stopErr = errors.Join(stopErr, err)
	}
	select {
	case <-p.done:
		if !p.exitVerified {
			stopErr = errors.Join(stopErr, errors.New("completed process group exit remains unverified"))
		}
		if p.cancel != nil {
			p.cancel()
		}
		return stopErr
	default:
	}
	if p.cmd.Process == nil {
		if p.cancel != nil {
			p.cancel()
		}
		return stopErr
	}
	if err := procutil.SignalCommandProcessGroup(p.cmd, syscall.SIGTERM); err != nil {
		stopErr = errors.Join(stopErr, err)
	}
	select {
	case <-p.done:
		if err := procutil.WaitForCommandProcessGroupExit(p.cmd, stopTimeout); err != nil {
			stopErr = errors.Join(stopErr, err)
		}
		if p.cancel != nil {
			p.cancel()
		}
		return stopErr
	case <-time.After(stopTimeout):
	}
	if err := procutil.KillCommandProcessGroupAndWait(p.cmd, stopTimeout); err != nil {
		stopErr = errors.Join(stopErr, err)
	}
	if p.cancel != nil {
		p.cancel()
	}
	select {
	case <-p.done:
	case <-time.After(stopTimeout):
		stopErr = errors.Join(stopErr, fmt.Errorf("wait for process exit observer: %w", context.DeadlineExceeded))
	}
	return stopErr
}

func (p *managedProcess) appendStderr(text string) {
	if text == "" {
		return
	}
	p.stderrMu.Lock()
	defer p.stderrMu.Unlock()
	if p.stderr.Len() >= stderrBufferLimit {
		p.stderrTruncated = true
		return
	}
	remaining := stderrBufferLimit - p.stderr.Len()
	if len(text) > remaining {
		p.stderr.WriteString(text[:remaining])
		p.stderrTruncated = true
		return
	}
	p.stderr.WriteString(text)
}

func (p *managedProcess) stderrText() string {
	p.stderrMu.Lock()
	defer p.stderrMu.Unlock()
	text := p.stderr.String()
	if p.stderrTruncated {
		return text + stderrTruncatedMarker
	}
	return text
}
