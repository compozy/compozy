package procutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"github.com/compozy/agh/internal/diagnostics"
)

const maxDetachedCommandErrorBytes = 4 * 1024
const detachedCommandLogTailSlack = 4

var (
	startDetachedProcess    = os.StartProcess
	closeDetachedLaunchFile = func(file *os.File) error {
		return file.Close()
	}
)

// DetachedLaunchRequest describes one detached process launch with log capture.
type DetachedLaunchRequest struct {
	Binary  string
	Args    []string
	Sandbox []string
	LogPath string
}

// DetachedProcess wraps a detached child process whose stderr/stdout were appended to a log file.
type DetachedProcess struct {
	process   *os.Process
	logPath   string
	logOffset int64
	waitOnce  sync.Once
	done      chan struct{}
	waitErr   error
}

// PID reports the launched process id.
func (p *DetachedProcess) PID() int {
	if p == nil || p.process == nil {
		return 0
	}
	return p.process.Pid
}

// Wait blocks until the detached process exits and attaches recent log output to failures.
func (p *DetachedProcess) Wait() error {
	if p == nil || p.process == nil {
		return nil
	}
	p.startWait()
	<-p.done
	return p.waitErr
}

// Done closes after the detached child has been reaped.
func (p *DetachedProcess) Done() <-chan struct{} {
	if p == nil || p.process == nil {
		return closedDetachedProcessDone()
	}
	p.startWait()
	return p.done
}

func newDetachedProcess(process *os.Process, logPath string, logOffset int64) *DetachedProcess {
	return &DetachedProcess{
		process:   process,
		logPath:   logPath,
		logOffset: logOffset,
		done:      make(chan struct{}),
	}
}

func (p *DetachedProcess) startWait() {
	p.waitOnce.Do(func() {
		go func() {
			p.waitErr = p.waitProcess()
			close(p.done)
		}()
	})
}

func (p *DetachedProcess) waitProcess() error {
	if p == nil || p.process == nil {
		return nil
	}

	state, err := p.process.Wait()
	if err == nil {
		if state == nil || state.Success() {
			return nil
		}
		err = &exec.ExitError{ProcessState: state}
	}
	return attachCommandLog(err, p.logPath, p.logOffset)
}

func closedDetachedProcessDone() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

// SpawnDetachedLoggedProcess launches one detached child process whose stdout/stderr are appended to req.LogPath.
func SpawnDetachedLoggedProcess(
	ctx context.Context,
	req DetachedLaunchRequest,
) (*DetachedProcess, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.validate(); err != nil {
		return nil, err
	}

	return spawnDetachedLoggedProcess(ctx, req)
}

func (r DetachedLaunchRequest) validate() error {
	if strings.TrimSpace(r.Binary) == "" {
		return errors.New("procutil: launch binary is required")
	}
	if strings.TrimSpace(r.LogPath) == "" {
		return errors.New("procutil: launch log path is required")
	}
	return nil
}

func resolveLaunchBinary(binary string) (string, error) {
	resolved, err := exec.LookPath(strings.TrimSpace(binary))
	if err != nil {
		return "", fmt.Errorf("procutil: resolve launch binary %q: %w", binary, err)
	}
	return resolved, nil
}

func launchSandbox(sandbox []string) []string {
	if len(sandbox) > 0 {
		return FilteredDaemonEnv(sandbox)
	}
	return FilteredDaemonEnv(nil)
}

func closeDetachedLaunchHandles(stdinFile *os.File, logFile *os.File, logPath string) error {
	return errors.Join(
		closeDetachedLaunchHandle(stdinFile, fmt.Sprintf("%q handle", os.DevNull)),
		closeDetachedLaunchHandle(logFile, fmt.Sprintf("log handle %q", logPath)),
	)
}

func closeDetachedLaunchHandle(file *os.File, label string) error {
	if file == nil {
		return nil
	}
	if err := closeDetachedLaunchFile(file); err != nil {
		return fmt.Errorf("procutil: close %s: %w", label, err)
	}
	return nil
}

func launchArgv(binary string, args []string) []string {
	argv := make([]string, 1, len(args)+1)
	argv[0] = binary
	return append(argv, args...)
}

func attachCommandLog(err error, logPath string, logOffset int64) error {
	if err == nil {
		return nil
	}
	text, readErr := readCommandLog(logPath, logOffset)
	if readErr != nil {
		return err
	}
	text = diagnostics.RedactAndBound(recentCommandError(text), maxDetachedCommandErrorBytes)
	if text == "" {
		return err
	}
	return fmt.Errorf("%w: stderr=%s", err, text)
}

func readCommandLog(path string, offset int64) (text string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("procutil: open log %q: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("procutil: close log %q: %w", path, closeErr)
		}
	}()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("procutil: stat log %q: %w", path, err)
	}
	size := info.Size()
	if offset < 0 || offset > size {
		offset = 0
	}
	start, length := commandLogTailWindow(offset, size)
	if length == 0 {
		return "", nil
	}
	data := make([]byte, length)
	n, err := file.ReadAt(data, start)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("procutil: read log %q: %w", path, err)
	}
	if n == 0 {
		return "", fmt.Errorf("procutil: read log %q: %w", path, err)
	}
	return strings.TrimSpace(string(data[:n])), nil
}

func commandLogTailWindow(offset int64, size int64) (int64, int64) {
	if size <= offset {
		return size, 0
	}
	maxBytes := int64(maxDetachedCommandErrorBytes * detachedCommandLogTailSlack)
	available := size - offset
	if available <= maxBytes {
		return offset, available
	}
	return size - maxBytes, maxBytes
}

func recentCommandError(logText string) string {
	text := strings.TrimSpace(logText)
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	for _, line := range slices.Backward(lines) {
		line := strings.TrimSpace(line)
		if strings.HasPrefix(line, "error:") {
			return line
		}
	}

	return text
}
