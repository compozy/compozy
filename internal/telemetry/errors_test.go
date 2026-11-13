package telemetry_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestClassifyMapsErrorsToFixedClasses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "Should report no class for a nil error", err: nil, want: ""},
		{
			name: "Should classify wrapped cancellation", err: fmt.Errorf("prompt: %w", context.Canceled),
			want: telemetry.ErrorCanceled,
		},
		{name: "Should classify deadlines", err: context.DeadlineExceeded, want: telemetry.ErrorDeadlineExceeded},
		{name: "Should classify I/O deadlines", err: os.ErrDeadlineExceeded, want: telemetry.ErrorDeadlineExceeded},
		{
			name: "Should classify marked provider failures", err: telemetry.ProviderError(errors.New("rate limited")),
			want: telemetry.ErrorProvider,
		},
		{
			name: "Should classify cancellation before a provider mark",
			err:  telemetry.ProviderError(context.Canceled), want: telemetry.ErrorCanceled,
		},
		{
			name: "Should classify typed permission errors",
			err:  &fs.PathError{Op: "open", Path: "/home/ana/x", Err: fs.ErrPermission},
			want: telemetry.ErrorPermissionDenied,
		},
		{name: "Should classify missing files", err: fs.ErrNotExist, want: telemetry.ErrorNotFound},
		{name: "Should classify invalid arguments", err: fs.ErrInvalid, want: telemetry.ErrorInvalidInput},
		{name: "Should classify stream failures", err: io.ErrUnexpectedEOF, want: telemetry.ErrorIO},
		{
			name: "Should classify permission text that crossed a process boundary",
			err:  errors.New("open /home/ana/secret.txt: permission denied"), want: telemetry.ErrorPermissionDenied,
		},
		{
			name: "Should classify not-found text", err: errors.New("session sess-1 not found"),
			want: telemetry.ErrorNotFound,
		},
		{
			name: "Should classify a provider mark before typed and text classes",
			err:  telemetry.ProviderError(errors.New("open /home/ana/x: permission denied")),
			want: telemetry.ErrorProvider,
		},
		{name: "Should classify permission errno values", err: syscall.EACCES, want: telemetry.ErrorPermissionDenied},
		{name: "Should classify missing-file errno values", err: syscall.ENOENT, want: telemetry.ErrorNotFound},
		{name: "Should classify other errno values as I/O failures", err: syscall.ENOSPC, want: telemetry.ErrorIO},
		{
			name: "Should classify path errors of another cause as I/O failures",
			err:  &fs.PathError{Op: "read", Path: "/home/ana/x", Err: errors.New("unclassified")},
			want: telemetry.ErrorIO,
		},
		{
			name: "Should classify network operation errors as I/O failures",
			err:  &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("unclassified")},
			want: telemetry.ErrorIO,
		},
		{
			name: "Should classify deadline text that crossed a process boundary",
			err:  errors.New("rpc: request timed out after 30s"), want: telemetry.ErrorDeadlineExceeded,
		},
		{
			name: "Should classify cancellation text that crossed a process boundary",
			err:  errors.New("rpc: context canceled"), want: telemetry.ErrorCanceled,
		},
		{
			name: "Should classify invalid-argument text that crossed a process boundary",
			err:  errors.New("invalid argument: model"), want: telemetry.ErrorInvalidInput,
		},
		{
			name: "Should classify broken-pipe text that crossed a process boundary",
			err:  errors.New("write: broken pipe"), want: telemetry.ErrorIO,
		},
		{name: "Should classify anything else as internal", err: errors.New("boom"), want: telemetry.ErrorInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := telemetry.Classify(tc.err); got != tc.want {
				t.Fatalf("Classify(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}

	t.Run("Should keep the provider failure text and chain", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("rate limited")
		marked := telemetry.ProviderError(cause)
		if marked.Error() != cause.Error() || !errors.Is(marked, cause) {
			t.Fatalf("ProviderError(%v) = %v, want the same text and chain", cause, marked)
		}
		if telemetry.ProviderError(nil) != nil {
			t.Fatal("ProviderError(nil) != nil")
		}
	})

	t.Run("Should ignore a nil error or span in MarkError", func(t *testing.T) {
		t.Parallel()

		_, span := noop.NewTracerProvider().Tracer(telemetry.ScopeSession).Start(context.Background(), "x")
		telemetry.MarkError(span, nil)
		telemetry.MarkError(nil, errors.New("boom"))
	})
}
