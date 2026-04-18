package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/update"
	"github.com/spf13/cobra"
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

func TestShouldStartUpdateCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "no args", args: nil, want: true},
		{name: "help flag", args: []string{"--help"}, want: false},
		{name: "nested help flag", args: []string{"tasks", "run", "--help"}, want: false},
		{name: "version flag", args: []string{"--version"}, want: false},
		{name: "help command", args: []string{"help"}, want: false},
		{name: "version command", args: []string{"version"}, want: false},
		{name: "completion command", args: []string{"completion", "bash"}, want: false},
		{name: "shell completion probe", args: []string{"__complete", "tasks"}, want: false},
		{name: "workflow command", args: []string{"tasks", "run", "daemon"}, want: true},
	}

	for _, tt := range tests {
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
