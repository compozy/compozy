package main

import (
	"bytes"
	"context"

	"errors"

	"fmt"
	"io"

	"os/exec"

	"sync"
	"syscall"
	"time"

	"github.com/compozy/agh/internal/procutil"
)

const (
	version               = "agh-daytona-launcher-sidecar-v1"
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
	stdout          *chunkQueue
	stderr          bytes.Buffer
	stderrMu        sync.Mutex
	stderrTruncated bool
	done            chan struct{}
	exitCode        int
	stopOnce        sync.Once
	streamMu        sync.Mutex
	streamClaimed   bool
}

func newManagedProcess(command string) (*managedProcess, error) {
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
		id:       randomID(),
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
	defer stdout.Close()
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
	defer stderr.Close()
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

func (p *managedProcess) wait() {
	defer close(p.done)
	if err := p.cmd.Wait(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			p.exitCode = exitErr.ExitCode()
			return
		}
		p.appendStderr(fmt.Sprintf("wait error: %v\n", err))
		p.exitCode = 1
		return
	}
	p.exitCode = 0
}

func (p *managedProcess) WriteStdin(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if p.stdin == nil {
		return errors.New("stdin is closed")
	}
	if _, err := p.stdin.Write(data); err != nil {
		return err
	}
	return nil
}

func (p *managedProcess) CloseStdin() error {
	if p.stdin == nil {
		return nil
	}
	err := p.stdin.Close()
	p.stdin = nil
	return err
}

func (p *managedProcess) Stop() error {
	var stopErr error
	p.stopOnce.Do(func() {
		if err := p.CloseStdin(); err != nil {
			stopErr = errors.Join(stopErr, err)
		}
		if p.cmd.Process == nil {
			if p.cancel != nil {
				p.cancel()
			}
			return
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
			return
		case <-time.After(stopTimeout):
		}
		if err := procutil.KillCommandProcessGroupAndWait(p.cmd, stopTimeout); err != nil {
			stopErr = errors.Join(stopErr, err)
		}
		if p.cancel != nil {
			p.cancel()
		}
		<-p.done
	})
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
