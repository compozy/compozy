package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/update"
	"github.com/spf13/cobra"
)

type testRunContextKey string

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

func TestRunPropagatesCallerContextToUpdateCheck(t *testing.T) {
	originalArgs := os.Args
	originalNewMainCommand := newMainCommand
	originalStartUpdateCheckFn := startUpdateCheckFn
	t.Cleanup(func() {
		os.Args = originalArgs
		newMainCommand = originalNewMainCommand
		startUpdateCheckFn = originalStartUpdateCheckFn
	})

	os.Args = []string{"compozy", "tasks", "run", "demo"}

	var gotCtx context.Context
	startUpdateCheckFn = func(
		ctx context.Context,
		_ string,
	) (<-chan *update.ReleaseInfo, context.CancelFunc, <-chan struct{}) {
		gotCtx = ctx

		result := make(chan *update.ReleaseInfo)
		close(result)

		done := make(chan struct{})
		close(done)
		return result, func() {}, done
	}
	newMainCommand = func() *cobra.Command {
		return &cobra.Command{
			Use: "compozy",
			RunE: func(*cobra.Command, []string) error {
				return nil
			},
		}
	}

	ctxKey := testRunContextKey("run")
	ctx := context.WithValue(context.Background(), ctxKey, "caller-context")

	if got := run(ctx); got != 0 {
		t.Fatalf("run() exit code = %d, want 0", got)
	}
	if gotCtx == nil || gotCtx.Value(ctxKey) != "caller-context" {
		t.Fatalf("startUpdateCheck context = %#v, want propagated caller context", gotCtx)
	}
}

func TestShouldStartUpdateCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "Should skip update check when no args are provided", args: nil, want: false},
		{name: "Should skip update check for help flag", args: []string{"--help"}, want: false},
		{name: "Should skip update check for nested help flag", args: []string{"tasks", "run", "--help"}, want: false},
		{name: "Should skip update check for nested help command", args: []string{"tasks", "help"}, want: false},
		{name: "Should skip update check for version flag", args: []string{"--version"}, want: false},
		{name: "Should skip update check for help command", args: []string{"help"}, want: false},
		{name: "Should skip update check for version command", args: []string{"version"}, want: false},
		{name: "Should skip update check for completion command", args: []string{"completion", "bash"}, want: false},
		{
			name: "Should skip update check for shell completion probe",
			args: []string{"__complete", "tasks"},
			want: false,
		},
		{
			name: "Should skip update check for shell completion probe without descriptions",
			args: []string{"__completeNoDesc", "tasks"},
			want: false,
		},
		{name: "Should start update check for workflow command", args: []string{"tasks", "run", "daemon"}, want: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldStartUpdateCheck(tt.args); got != tt.want {
				t.Fatalf("shouldStartUpdateCheck(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
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

func TestShouldWriteUpdateNotification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  *cobra.Command
		want bool
	}{
		{
			name: "Should allow notification when command has no format flag",
			cmd:  &cobra.Command{Use: "ext"},
			want: true,
		},
		{
			name: "Should allow notification for text output",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "exec"}
				cmd.Flags().String("format", "text", "")
				if err := cmd.Flags().Set("format", "text"); err != nil {
					t.Fatalf("set format flag: %v", err)
				}
				return cmd
			}(),
			want: true,
		},
		{
			name: "Should suppress notification for json output",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "exec"}
				cmd.Flags().String("format", "text", "")
				if err := cmd.Flags().Set("format", "json"); err != nil {
					t.Fatalf("set format flag: %v", err)
				}
				return cmd
			}(),
			want: false,
		},
		{
			name: "Should suppress notification for raw-json output",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "start"}
				cmd.Flags().String("format", "text", "")
				if err := cmd.Flags().Set("format", "raw-json"); err != nil {
					t.Fatalf("set format flag: %v", err)
				}
				return cmd
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldWriteUpdateNotification(tt.cmd); got != tt.want {
				t.Fatalf("shouldWriteUpdateNotification() = %v, want %v", got, tt.want)
			}
		})
	}
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
