package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/redact"
	"golang.org/x/sys/execabs"
)

const (
	defaultGitTimeout      = 30 * time.Second
	maxGitOutputBytes      = 1 << 20
	maxGitStreamChunkBytes = 64 << 10
)

type GitRunner interface {
	Run(context.Context, string, ...string) ([]byte, []byte, error)
}

type GitOutputStream string

const (
	GitOutputStdout GitOutputStream = "stdout"
	GitOutputStderr GitOutputStream = "stderr"
)

type GitOutput struct {
	Stream GitOutputStream
	Chunk  []byte
}

type GitOutputHandler func(GitOutput)

type StreamingGitRunner interface {
	RunStreaming(context.Context, string, GitOutputHandler, ...string) ([]byte, []byte, error)
}

type RealGitRunner struct {
	executable string
	timeout    time.Duration
	environ    func() []string
}

type cappedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

type gitOutputWriter struct {
	buffer   *cappedBuffer
	pending  []byte
	stream   GitOutputStream
	onOutput GitOutputHandler
}

func (w *gitOutputWriter) Write(value []byte) (int, error) {
	if w.buffer == nil {
		w.buffer = newCappedBuffer(maxGitOutputBytes)
	}
	written, err := w.buffer.Write(value)
	if err != nil {
		return written, err
	}
	if w.onOutput != nil && written > 0 {
		w.pending = append(w.pending, value[:written]...)
		for {
			newline := bytes.IndexByte(w.pending, '\n')
			if newline < 0 {
				break
			}
			w.emitPending(newline + 1)
		}
		for len(w.pending) >= maxGitStreamChunkBytes {
			w.emitPending(maxGitStreamChunkBytes)
		}
	}
	return written, nil
}

func (w *gitOutputWriter) Flush() {
	if len(w.pending) > 0 {
		w.emitPending(len(w.pending))
	}
}

func (w *gitOutputWriter) emitPending(length int) {
	w.onOutput(GitOutput{Stream: w.stream, Chunk: append([]byte(nil), w.pending[:length]...)})
	w.pending = append(w.pending[:0], w.pending[length:]...)
}

func (w *gitOutputWriter) Bytes() []byte {
	if w.buffer == nil {
		return nil
	}
	return w.buffer.Bytes()
}

func (w *gitOutputWriter) String() string {
	if w.buffer == nil {
		return ""
	}
	return w.buffer.String()
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (b *cappedBuffer) Write(value []byte) (int, error) {
	written := len(value)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if len(value) > remaining {
			value = value[:remaining]
		}
		if _, err := b.buffer.Write(value); err != nil {
			return 0, err
		}
	}
	return written, nil
}

func (b *cappedBuffer) String() string {
	return b.buffer.String()
}

func (b *cappedBuffer) Bytes() []byte {
	return b.buffer.Bytes()
}

func NewRealGitRunner(timeout time.Duration) (*RealGitRunner, error) {
	path, err := execabs.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitUnavailable, err)
	}
	if timeout <= 0 {
		timeout = defaultGitTimeout
	}
	return &RealGitRunner{executable: path, timeout: timeout, environ: os.Environ}, nil
}

func (r *RealGitRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, []byte, error) {
	return r.run(ctx, dir, nil, args...)
}

func (r *RealGitRunner) RunStreaming(
	ctx context.Context,
	dir string,
	onOutput GitOutputHandler,
	args ...string,
) ([]byte, []byte, error) {
	return r.run(ctx, dir, onOutput, args...)
}

func (r *RealGitRunner) run(
	ctx context.Context,
	dir string,
	onOutput GitOutputHandler,
	args ...string,
) ([]byte, []byte, error) {
	if r == nil || strings.TrimSpace(r.executable) == "" {
		return nil, nil, ErrGitUnavailable
	}
	// The executable is resolved once from the operator environment; callers supply structured git arguments.
	cmd := execabs.CommandContext(ctx, r.executable, args...) // #nosec G204
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.Env = gitEnvironment(r.environ())
	procutil.ConfigureCommandProcessGroup(cmd)
	var callbackMu sync.Mutex
	var serializedOutput GitOutputHandler
	if onOutput != nil {
		serializedOutput = func(output GitOutput) {
			callbackMu.Lock()
			defer callbackMu.Unlock()
			onOutput(output)
		}
	}
	stdout := &gitOutputWriter{stream: GitOutputStdout, onOutput: serializedOutput}
	stderr := &gitOutputWriter{stream: GitOutputStderr, onOutput: serializedOutput}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf(
			"worktree: start git %q: %w",
			diagnostics.RedactAndBound(strings.Join(args, " "), 2048), err,
		)
	}
	if err := procutil.RegisterCommandProcessGroup(cmd); err != nil {
		killErr := procutil.KillCommandProcessGroupAndWait(cmd, time.Second)
		waitErr := cmd.Wait()
		stdout.Flush()
		stderr.Flush()
		return stdout.Bytes(), stderr.Bytes(), errors.Join(
			fmt.Errorf("worktree: register git process group: %w", err), killErr, waitErr,
		)
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	timeout := r.timeout
	if timeout <= 0 {
		timeout = defaultGitTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-waitCh:
		stdout.Flush()
		stderr.Flush()
		if err != nil {
			return stdout.Bytes(), stderr.Bytes(), gitCommandError(args, stderr.String(), err)
		}
		return stdout.Bytes(), stderr.Bytes(), nil
	case <-ctx.Done():
		killErr := procutil.KillCommandProcessGroupAndWait(cmd, time.Second)
		waitErr := <-waitCh
		stdout.Flush()
		stderr.Flush()
		return stdout.Bytes(), stderr.Bytes(), errors.Join(
			gitCommandError(args, stderr.String(), ctx.Err()), killErr, waitErr,
		)
	case <-timer.C:
		killErr := procutil.KillCommandProcessGroupAndWait(cmd, time.Second)
		waitErr := <-waitCh
		stdout.Flush()
		stderr.Flush()
		return stdout.Bytes(), stderr.Bytes(), errors.Join(
			gitCommandError(args, stderr.String(), context.DeadlineExceeded), killErr, waitErr,
		)
	}
}

func gitEnvironment(parent []string) []string {
	filtered := make([]string, 0, len(parent)+3)
	for _, entry := range parent {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(name)) {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OPTIONAL_LOCKS",
			"GIT_TERMINAL_PROMPT", "GCM_INTERACTIVE":
			continue
		default:
			if redact.IsSensitiveKey(name) || redact.String(entry) != entry {
				continue
			}
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
}

func gitCommandError(args []string, stderr string, cause error) error {
	command := diagnostics.RedactAndBound(strings.Join(args, " "), 2048)
	detail := diagnostics.RedactAndBound(strings.TrimSpace(stderr), setupDiagnosticLimit)
	if detail == "" {
		return fmt.Errorf("worktree: git %q: %w", command, cause)
	}
	return fmt.Errorf("worktree: git %q: %s: %w", command, detail, cause)
}

var _ GitRunner = (*RealGitRunner)(nil)
var _ StreamingGitRunner = (*RealGitRunner)(nil)
