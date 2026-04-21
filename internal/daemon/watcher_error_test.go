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

	testCases := []struct {
		name       string
		backendErr error
		wantNil    bool
	}{
		{
			name:    "Should return nil when the backend error is nil",
			wantNil: true,
		},
		{
			name:       "Should return a wrapped backend failure",
			backendErr: errors.New("backend failed"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			watcher := &workflowWatcher{
				workflowRoot: t.TempDir(),
				logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
					Level: slog.LevelWarn,
				})),
			}

			watcher.handleBackendError(tc.backendErr)
			err := watcher.stopError()
			if tc.wantNil {
				if err != nil {
					t.Fatalf("stopError() = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatal("stopError() = nil, want wrapped backend error")
			}
			if !errors.Is(err, tc.backendErr) {
				t.Fatalf("stopError() = %v, want wrapped cause %v", err, tc.backendErr)
			}
			if !strings.Contains(err.Error(), "workflow watcher error") {
				t.Fatalf("stopError() = %v, want workflow watcher context", err)
			}
			if !strings.Contains(err.Error(), tc.backendErr.Error()) {
				t.Fatalf("stopError() = %v, want backend error text", err)
			}
		})
	}
}
