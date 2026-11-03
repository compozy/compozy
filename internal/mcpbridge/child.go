package mcpbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

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
	cmd.Env = append([]string(nil), spec.Env...)
	cmd.Dir = spec.Dir
	cmd.Stdin = stdinReader
	cmd.Stdout = stdoutWriter
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
