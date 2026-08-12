package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/redact"
	"golang.org/x/sys/execabs"
)

const defaultGitTimeout = 30 * time.Second

type GitRunner interface {
	Run(context.Context, string, ...string) ([]byte, []byte, error)
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
	if r == nil || strings.TrimSpace(r.executable) == "" {
		return nil, nil, ErrGitUnavailable
	}
	commandCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cmd := execabs.CommandContext(commandCtx, r.executable, args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.Env = gitEnvironment(r.environ())
	procutil.ConfigureCommandProcessGroup(cmd)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf(
			"worktree: start git %q: %w",
			diagnostics.RedactAndBound(strings.Join(args, " "), 2048), err,
		)
	}
	if err := procutil.RegisterCommandProcessGroup(cmd); err != nil {
		killErr := procutil.KillCommandProcessGroupAndWait(cmd, time.Second)
		waitErr := cmd.Wait()
		return stdout.Bytes(), stderr.Bytes(), errors.Join(
			fmt.Errorf("worktree: register git process group: %w", err), killErr, waitErr,
		)
	}

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	select {
	case err := <-waitCh:
		if err != nil {
			return stdout.Bytes(), stderr.Bytes(), gitCommandError(args, stderr.String(), err)
		}
		return stdout.Bytes(), stderr.Bytes(), nil
	case <-commandCtx.Done():
		killErr := procutil.KillCommandProcessGroupAndWait(cmd, time.Second)
		waitErr := <-waitCh
		return stdout.Bytes(), stderr.Bytes(), errors.Join(
			gitCommandError(args, stderr.String(), commandCtx.Err()), killErr, waitErr,
		)
	}
}

func gitEnvironment(parent []string) []string {
	filtered := make([]string, 0, len(parent)+2)
	for _, entry := range parent {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(name)) {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_TERMINAL_PROMPT", "GCM_INTERACTIVE":
			continue
		default:
			if redact.IsSensitiveKey(name) || redact.String(entry) != entry {
				continue
			}
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
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
