package mcpbridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/subprocess"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// stdioChild owns one bridged stdio MCP server process from start to reaping.
// The bridge starts it directly, so the process joins its kill group (the
// Windows job object) before the MCP handshake and before it can spawn workers.
type stdioChild struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr *tailBuffer
	stop   context.CancelFunc
	waited chan struct{}
	// exitErr is the child's exit status; read only after waited is closed.
	exitErr error
}

func startStdioChild(ctx context.Context, name string, spec Spec) (_ *stdioChild, err error) {
	// Resolve against the child's own PATH and directory, as the agent would.
	executable, err := subprocess.ResolveExecutable(strings.TrimSpace(spec.Command), spec.Env, spec.Dir)
	if err != nil {
		return nil, fmt.Errorf("mcpbridge: resolve stdio server %q command: %w", name, err)
	}
	// Separate pipes keep cmd.Wait from closing the ends the MCP client reads and writes.
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("mcpbridge: create stdio server %q stdin: %w", name, err)
	}
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("mcpbridge: create stdio server %q stdout: %w", name, err),
			stdinReader.Close(), stdinWriter.Close(),
		)
	}
	// The child outlives the start request and lives until close, not the caller's context.
	childCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	cmd := exec.CommandContext(childCtx, executable, spec.Args...)
	cmd.Env = slices.Clone(spec.Env)
	cmd.Dir = spec.Dir
	stderr := &tailBuffer{limit: stderrTailBytes}
	cmd.Stdin = stdinReader
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderr
	// A detached helper that inherits stderr would otherwise keep Wait copying
	// it forever after the group is killed; bound that wait so close returns.
	cmd.WaitDelay = childExitTimeout
	procutil.ConfigureCommandProcessGroup(cmd)
	startErr := cmd.Start()
	// The child holds its own copies; the parent keeps only its ends.
	closeErr := errors.Join(stdinReader.Close(), stdoutWriter.Close())
	if startErr != nil {
		stop()
		return nil, errors.Join(
			fmt.Errorf("mcpbridge: start stdio server %q: %w", name, startErr),
			closeErr, stdinWriter.Close(), stdoutReader.Close(),
		)
	}
	child := &stdioChild{
		cmd:    cmd,
		stdin:  stdinWriter,
		stdout: stdoutReader,
		stderr: stderr,
		stop:   stop,
		waited: make(chan struct{}),
	}
	go func() {
		defer close(child.waited)
		// Failures surface through the MCP session; the status is kept for diagnosis.
		child.exitErr = cmd.Wait()
	}()
	if err := errors.Join(closeErr, procutil.RegisterCommandProcessGroup(cmd)); err != nil {
		return nil, errors.Join(
			fmt.Errorf("mcpbridge: register stdio server %q process group: %w", name, err),
			child.close(),
		)
	}
	return child, nil
}

// transport is the MCP client transport over the child's stdio pipes.
func (c *stdioChild) transport() sdkmcp.Transport {
	return &sdkmcp.IOTransport{Reader: c.stdout, Writer: c.stdin}
}

// close ends stdin, gives the server a grace period, then kills its whole
// process group so anything it spawned goes with it, and reaps the child.
func (c *stdioChild) close() error {
	if c == nil {
		return nil
	}
	errs := []error{ignoreClosed(c.stdin.Close())}
	select {
	case <-c.waited:
	case <-time.After(childExitTimeout):
	}
	if err := procutil.KillCommandProcessGroupAndWait(c.cmd, childExitTimeout); err != nil {
		errs = append(errs, fmt.Errorf("stop process group: %w", err))
	}
	c.stop()
	<-c.waited
	errs = append(errs, ignoreClosed(c.stdout.Close()))
	return errors.Join(errs...)
}

func ignoreClosed(err error) error {
	if errors.Is(err, os.ErrClosed) {
		return nil
	}
	return err
}

// stderrSummary is the redacted, bounded tail of the child's stderr, for
// startup errors; empty when the child wrote nothing.
func (c *stdioChild) stderrSummary() string {
	if c == nil || c.stderr == nil {
		return ""
	}
	return strings.TrimSpace(diagnostics.RedactAndBound(c.stderr.String(), stderrTailBytes))
}

// tailBuffer keeps only the last limit bytes written to it.
type tailBuffer struct {
	mu      sync.Mutex
	buf     []byte
	limit   int
	partial bool
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if overflow := len(b.buf) - b.limit; overflow > 0 {
		b.partial = b.buf[overflow-1] != '\n'
		b.buf = append(b.buf[:0], b.buf[overflow:]...)
	}
	return len(p), nil
}

// String returns the kept tail. When the cut landed mid-line, the first line
// may start mid-credential, too short for redaction to recognize, so only
// whole lines are returned.
func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.partial {
		return string(b.buf)
	}
	_, rest, found := bytes.Cut(b.buf, []byte{'\n'})
	if !found {
		return ""
	}
	return string(rest)
}
