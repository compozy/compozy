package e2e

import (
	"context"

	"errors"
	"fmt"

	"os"

	"strings"

	"syscall"
	"testing"
	"time"

	"github.com/compozy/agh/internal/procutil"

	"github.com/compozy/agh/internal/testutil"
)

// Stop shuts down the started daemon and waits for process exit.
func (h *RuntimeHarness) Stop(ctx context.Context) error {
	if h == nil {
		return nil
	}

	h.stopOnce.Do(func() {
		if err := h.stopWithContext(ctx); err != nil {
			h.stopErr = err
		}
	})
	return h.stopErr
}

func (h *RuntimeHarness) stopWithContext(ctx context.Context) error {
	defer closeIdleConnections(h.HTTPClient)
	defer closeIdleConnections(h.UDSClient)

	if h.process == nil && h.waitCh == nil {
		return nil
	}

	if exited, err := h.pollExit(); exited {
		return err
	}

	signaledProcess, err := h.requestDaemonStop(ctx)
	if err != nil {
		return err
	}

	waitErr := h.waitForExit(ctx)
	if waitErr == nil {
		return nil
	}
	if signaledProcess && !errors.Is(waitErr, context.DeadlineExceeded) {
		return nil
	}

	if err := h.forceKillDaemonProcess(); err != nil {
		return err
	}
	if err := h.waitAfterForceKill(ctx); err != nil {
		return err
	}
	return nil
}

func (h *RuntimeHarness) requestDaemonStop(ctx context.Context) (bool, error) {
	if h.process != nil && h.process.Process != nil {
		if signalErr := procutil.SignalCommandProcessGroup(
			h.process, syscall.SIGINT,
		); signalErr != nil &&
			!errors.Is(signalErr, os.ErrProcessDone) &&
			!errors.Is(signalErr, syscall.ESRCH) {
			return false, fmt.Errorf("interrupt daemon process group: %w", signalErr)
		}
		return true, nil
	}
	if h.CLI == nil {
		return false, nil
	}
	if _, _, err := h.CLI.Run(ctx, "daemon", "stop", "-o", "json"); err != nil {
		return false, fmt.Errorf("stop daemon via CLI: %w", err)
	}
	return false, nil
}

func (h *RuntimeHarness) forceKillDaemonProcess() error {
	if h.process == nil || h.process.Process == nil {
		return nil
	}
	if killErr := procutil.KillCommandProcessGroupAndWait(
		h.process, 5*time.Second,
	); killErr != nil &&
		!errors.Is(killErr, os.ErrProcessDone) &&
		!errors.Is(killErr, syscall.ESRCH) {
		return fmt.Errorf("kill daemon process group: %w", killErr)
	}
	return nil
}

func (h *RuntimeHarness) waitAfterForceKill(ctx context.Context) error {
	killCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if killWaitErr := h.waitForExit(killCtx); killWaitErr != nil && !errors.Is(killWaitErr, context.DeadlineExceeded) {
		return killWaitErr
	}
	return nil
}

func (h *RuntimeHarness) cleanupFailedStart(ctx context.Context) error {
	if h == nil {
		return nil
	}

	if h.process != nil || h.waitCh != nil {
		exited, err := h.pollExit()
		if err != nil && !exited {
			return fmt.Errorf("poll failed runtime process exit: %w", err)
		}
		if !exited {
			if err := h.stopWithContext(ctx); err != nil {
				return fmt.Errorf("stop failed runtime start: %w", err)
			}
		}
	}
	h.resetProcessState()

	for _, path := range []string{
		strings.TrimSpace(h.HomePaths.DaemonInfo),
		strings.TrimSpace(h.Config.Daemon.Socket),
	} {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove runtime start artifact %q: %w", path, err)
		}
	}

	return nil
}

func (h *RuntimeHarness) resetProcessState() {
	if h == nil {
		return
	}

	h.processWaitMu.Lock()
	defer h.processWaitMu.Unlock()

	h.process = nil
	h.waitCh = nil
	h.processExited = false
	h.processErr = nil
}

func (h *RuntimeHarness) readinessFailureShouldRetry(err error) bool {
	retryHTTPPort, retrySocketPath := h.readinessFailureRetryReasons(err)
	return retryHTTPPort || retrySocketPath
}

func (h *RuntimeHarness) readinessFailureRetryReasons(err error) (retryHTTPPort bool, retrySocketPath bool) {
	if h == nil || err == nil {
		return false, false
	}
	if !errors.Is(err, errDaemonExitedBeforeReadiness) {
		return false, false
	}

	processLog, readErr := h.readProcessLog()
	if readErr != nil {
		return false, false
	}
	return processLogHasHTTPPortConflict(processLog), processLogHasSocketPathConflict(processLog)
}

func processLogHasHTTPPortConflict(processLog string) bool {
	return strings.Contains(processLog, "listen tcp") && strings.Contains(processLog, "bind: address already in use")
}

func processLogHasSocketPathConflict(processLog string) bool {
	return strings.Contains(processLog, "listen unix") &&
		(strings.Contains(processLog, "bind: file exists") ||
			strings.Contains(processLog, "bind: address already in use"))
}

func (h *RuntimeHarness) readProcessLog() (string, error) {
	if h == nil {
		return "", errors.New("runtime harness is required")
	}
	path := strings.TrimSpace(h.processLogPath)
	if path == "" {
		return "", errors.New("runtime harness process log path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read process log %q: %w", path, err)
	}
	return string(data), nil
}

func (h *RuntimeHarness) reseedRuntimeHTTPPort(t testing.TB) error {
	t.Helper()

	if h == nil {
		return errors.New("runtime harness is required")
	}

	nextPort := testutil.FreeTCPPort(t)
	for nextPort == h.Config.HTTP.Port {
		nextPort = testutil.FreeTCPPort(t)
	}

	h.Config.HTTP.Port = nextPort
	h.HTTPBaseURL = fmt.Sprintf("http://%s:%d", h.Config.HTTP.Host, nextPort)
	return writeSeedConfigFile(h.HomePaths, &h.Config)
}

func (h *RuntimeHarness) reseedRuntimeSocketPath(t testing.TB) error {
	t.Helper()

	if h == nil {
		return errors.New("runtime harness is required")
	}

	previousSocket := h.Config.Daemon.Socket
	nextSocket := shortSocketPath(t)
	for nextSocket == previousSocket {
		nextSocket = shortSocketPath(t)
	}

	h.Config.Daemon.Socket = nextSocket
	h.UDSClient = newUDSClient(nextSocket)
	return writeSeedConfigFile(h.HomePaths, &h.Config)
}
