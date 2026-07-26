package acp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/store"
)

func TestFailureFromErrorClassifiesTypedAndContextFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve typed startup failure and redact secrets", func(t *testing.T) {
		t.Parallel()

		err := WrapFailure(store.FailureStartup, "startup failed", errors.New("token=super-secret"))
		failure, ok := FailureFromError(err, store.FailureUnknown)
		if !ok {
			t.Fatal("FailureFromError() ok = false, want true")
		}
		if got, want := failure.Kind, store.FailureStartup; got != want {
			t.Fatalf("failure.Kind = %q, want %q", got, want)
		}
		if strings.Contains(failure.Summary, "super-secret") || !strings.Contains(failure.Summary, "[REDACTED]") {
			t.Fatalf("failure.Summary = %q, want redacted secret", failure.Summary)
		}
	})

	t.Run("Should classify request errors as protocol failures", func(t *testing.T) {
		t.Parallel()

		failure, ok := FailureFromError(&acpsdk.RequestError{Code: -32603, Message: "bad frame"}, store.FailureUnknown)
		if !ok {
			t.Fatal("FailureFromError(request error) ok = false, want true")
		}
		if got, want := failure.Kind, store.FailureProtocol; got != want {
			t.Fatalf("failure.Kind = %q, want %q", got, want)
		}
	})

	t.Run("Should classify context cancellation", func(t *testing.T) {
		t.Parallel()

		failure, ok := FailureFromError(context.Canceled, store.FailureUnknown)
		if !ok {
			t.Fatal("FailureFromError(context canceled) ok = false, want true")
		}
		if got, want := failure.Kind, store.FailureCanceled; got != want {
			t.Fatalf("failure.Kind = %q, want %q", got, want)
		}
	})
}

func TestProbeTargetCommandReportsStructuredTimeoutAndCancellation(t *testing.T) {
	t.Parallel()

	t.Run("Should return timeout when lookup exceeds probe timeout", func(t *testing.T) {
		t.Parallel()

		result := ProbeTargetCommand(context.Background(), ProbeTarget{
			AgentName: "coder",
			Provider:  "fake",
			Command:   "fake-agent --acp",
		}, ProbeOptions{
			Timeout: 10 * time.Millisecond,
			Lookup: func(ctx context.Context, _ string) (string, error) {
				<-ctx.Done()
				return "", ctx.Err()
			},
		})
		if got, want := result.Status, ProbeStatusTimeout; got != want {
			t.Fatalf("result.Status = %q, want %q", got, want)
		}
		if result.Error == "" {
			t.Fatal("result.Error = empty, want timeout detail")
		}
	})

	t.Run("Should return canceled when parent context is canceled", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result := ProbeTargetCommand(ctx, ProbeTarget{Command: "fake-agent"}, ProbeOptions{})
		if got, want := result.Status, ProbeStatusCanceled; got != want {
			t.Fatalf("result.Status = %q, want %q", got, want)
		}
	})

	t.Run("Should return ok with executable path", func(t *testing.T) {
		t.Parallel()

		result := ProbeTargetCommand(context.Background(), ProbeTarget{Command: "fake-agent --acp"}, ProbeOptions{
			Lookup: func(context.Context, string) (string, error) {
				return "/usr/local/bin/fake-agent", nil
			},
		})
		if got, want := result.Status, ProbeStatusOK; got != want {
			t.Fatalf("result.Status = %q, want %q", got, want)
		}
		if got, want := result.Executable, "/usr/local/bin/fake-agent"; got != want {
			t.Fatalf("result.Executable = %q, want %q", got, want)
		}
	})

	t.Run("Should redact exposed command and parse errors", func(t *testing.T) {
		t.Parallel()

		result := ProbeTargetCommand(context.Background(), ProbeTarget{
			Command: `fake-agent --api-key=super-secret "unterminated`,
		}, ProbeOptions{})
		if got, want := result.Status, ProbeStatusInvalid; got != want {
			t.Fatalf("result.Status = %q, want %q", got, want)
		}
		if strings.Contains(result.Command, "super-secret") || strings.Contains(result.Error, "super-secret") {
			t.Fatalf("result = %#v, want redacted command and error", result)
		}
		if !strings.Contains(result.Command, "[REDACTED]") || !strings.Contains(result.Error, "[REDACTED]") {
			t.Fatalf("result = %#v, want redacted marker", result)
		}
	})
}
