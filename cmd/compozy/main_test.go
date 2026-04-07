package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/update"
)

func TestWaitForUpdateResult(t *testing.T) {
	t.Parallel()

	wantReady := &update.ReleaseInfo{Version: "v1.2.3"}
	tests := []struct {
		name  string
		setup func() <-chan *update.ReleaseInfo
		want  *update.ReleaseInfo
	}{
		{
			name: "Should return a ready release",
			setup: func() <-chan *update.ReleaseInfo {
				result := make(chan *update.ReleaseInfo, 1)
				result <- wantReady
				close(result)
				return result
			},
			want: wantReady,
		},
		{
			name: "Should return nil for a closed channel",
			setup: func() <-chan *update.ReleaseInfo {
				result := make(chan *update.ReleaseInfo)
				close(result)
				return result
			},
		},
		{
			name: "Should return nil when the update check does not finish quickly",
			setup: func() <-chan *update.ReleaseInfo {
				return make(chan *update.ReleaseInfo)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := waitForUpdateResult(tt.setup()); got != tt.want {
				t.Fatalf("waitForUpdateResult() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestStartUpdateCheckClosesCompletionSignal(t *testing.T) {
	t.Parallel()

	result, cancel, done := startUpdateCheck(context.Background(), "dev")
	cancel()

	select {
	case <-done:
	case <-time.After(2 * updateResultWaitTimeout):
		t.Fatalf("startUpdateCheck did not signal completion within %s", 2*updateResultWaitTimeout)
	}

	if got := waitForUpdateResult(result); got != nil {
		t.Fatalf("waitForUpdateResult() = %#v, want nil", got)
	}
}

func TestWriteUpdateNotification(t *testing.T) {
	t.Parallel()

	t.Run("Should write the rendered notification", func(t *testing.T) {
		t.Parallel()

		var sink capturingWriter
		release := &update.ReleaseInfo{Version: "v1.2.3"}

		if err := writeUpdateNotification(&sink, "v1.2.2", release); err != nil {
			t.Fatalf("writeUpdateNotification() error = %v", err)
		}
		if sink.writes == 0 {
			t.Fatal("expected notification to be written")
		}
	})

	t.Run("Should return writer failures", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("stderr write failed")
		release := &update.ReleaseInfo{Version: "v1.2.3"}

		err := writeUpdateNotification(errWriter{err: wantErr}, "v1.2.2", release)
		if !errors.Is(err, wantErr) {
			t.Fatalf("writeUpdateNotification() error = %v, want %v", err, wantErr)
		}
	})
}

type capturingWriter struct {
	writes int
}

func (w *capturingWriter) Write(p []byte) (int, error) {
	w.writes++
	return len(p), nil
}

type errWriter struct {
	err error
}

func (w errWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}
