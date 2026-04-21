package daemon

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestWorkflowWatcherHandleBackendErrorRecordsWrappedFailure(t *testing.T) {
	t.Parallel()

	watcher := &workflowWatcher{
		workflowRoot: "/tmp/demo-workflow",
		logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		})),
	}

	watcher.handleBackendError(nil)
	if err := watcher.stopError(); err != nil {
		t.Fatalf("stopError(after nil) = %v, want nil", err)
	}

	rootErr := errors.New("backend failed")
	watcher.handleBackendError(rootErr)

	err := watcher.stopError()
	if err == nil {
		t.Fatal("stopError() = nil, want wrapped backend error")
	}
	if !strings.Contains(err.Error(), "workflow watcher error") || !strings.Contains(err.Error(), "backend failed") {
		t.Fatalf("stopError() = %v, want wrapped backend error", err)
	}
}
