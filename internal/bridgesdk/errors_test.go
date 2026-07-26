package bridgesdk

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

// Invariant: public control diagnostics preserve provider error taxonomy without exposing provider error text.
// Owning layer: bridge SDK error classification. Canonical suite: errors_test.go.
func TestProviderIdentityCheckRecordPreservesSafeErrorTaxonomy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus bridgepkg.BridgeCheckStatus
		wantText   string
	}{
		{name: "success", wantStatus: bridgepkg.BridgeCheckStatusPass},
		{
			name:       "auth",
			err:        &AuthError{Err: errors.New("invalid sk-provider-secret")},
			wantStatus: bridgepkg.BridgeCheckStatusFail,
			wantText:   "bot_token",
		},
		{
			name:       "permanent",
			err:        &PermanentError{Err: errors.New("bad sk-provider-secret configuration")},
			wantStatus: bridgepkg.BridgeCheckStatusFail,
			wantText:   "provider configuration",
		},
		{
			name:       "rate limit",
			err:        &RateLimitError{Err: errors.New("sk-provider-secret rate limited")},
			wantStatus: bridgepkg.BridgeCheckStatusWarn,
			wantText:   "rate-limited",
		},
		{
			name:       "timeout",
			err:        context.DeadlineExceeded,
			wantStatus: bridgepkg.BridgeCheckStatusWarn,
			wantText:   "timed out",
		},
		{
			name:       "transient",
			err:        &TransientError{Err: errors.New("sk-provider-secret unavailable")},
			wantStatus: bridgepkg.BridgeCheckStatusWarn,
			wantText:   "temporarily unavailable",
		},
		{
			name: "overloaded",
			err: &HTTPError{
				StatusCode: HTTPStatusOverloaded,
				Message:    "sk-provider-secret overloaded",
			},
			wantStatus: bridgepkg.BridgeCheckStatusWarn,
			wantText:   "temporarily unavailable",
		},
		{
			name: "server error",
			err: &HTTPError{
				StatusCode: http.StatusInternalServerError,
				Message:    "sk-provider-secret server error",
			},
			wantStatus: bridgepkg.BridgeCheckStatusWarn,
			wantText:   "temporarily unavailable",
		},
		{
			name:       "canceled",
			err:        context.Canceled,
			wantStatus: bridgepkg.BridgeCheckStatusWarn,
			wantText:   "canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			record := ProviderIdentityCheckRecord(tt.err, "bot_token", "bot_token")
			if record.Check != "provider.identity" || record.Status != tt.wantStatus {
				t.Fatalf("ProviderIdentityCheckRecord() = %#v, want status %q", record, tt.wantStatus)
			}
			if tt.wantText != "" && !strings.Contains(record.Remediation, tt.wantText) {
				t.Fatalf("remediation = %q, want %q", record.Remediation, tt.wantText)
			}
			if strings.Contains(record.Remediation, "sk-provider-secret") {
				t.Fatalf("remediation leaked provider error text: %q", record.Remediation)
			}
		})
	}
}

func TestClassifyErrorMapsRepresentativeProviderFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantClass  ErrorClass
		wantRetry  bool
		wantStatus bridgepkg.BridgeStatus
		wantReason bridgepkg.BridgeDegradationReason
	}{
		{
			name:       "auth",
			err:        &AuthError{Err: errors.New("invalid token")},
			wantClass:  ErrorClassAuth,
			wantRetry:  false,
			wantStatus: bridgepkg.BridgeStatusAuthRequired,
			wantReason: bridgepkg.BridgeDegradationReasonAuthFailed,
		},
		{
			name: "rate_limit",
			err: &HTTPError{
				StatusCode: http.StatusTooManyRequests,
				Message:    "too many requests",
				RetryAfter: time.Second,
			},
			wantClass:  ErrorClassRateLimit,
			wantRetry:  true,
			wantStatus: bridgepkg.BridgeStatusDegraded,
			wantReason: bridgepkg.BridgeDegradationReasonRateLimited,
		},
		{
			name:       "timeout",
			err:        context.DeadlineExceeded,
			wantClass:  ErrorClassTimeout,
			wantRetry:  true,
			wantStatus: bridgepkg.BridgeStatusDegraded,
			wantReason: bridgepkg.BridgeDegradationReasonProviderTimeout,
		},
		{
			name:       "transient",
			err:        &TransientError{Err: io.EOF},
			wantClass:  ErrorClassTransient,
			wantRetry:  true,
			wantStatus: bridgepkg.BridgeStatusDegraded,
		},
		{
			name:       "permanent",
			err:        &PermanentError{Err: errors.New("bad request")},
			wantClass:  ErrorClassPermanent,
			wantRetry:  false,
			wantStatus: bridgepkg.BridgeStatusError,
		},
		{
			name:       "committed mutation",
			err:        &CommittedMutationError{Err: errors.New("remote result unavailable")},
			wantClass:  ErrorClassPermanent,
			wantRetry:  false,
			wantStatus: bridgepkg.BridgeStatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			classified := ClassifyError(tt.err)
			if got, want := classified.Class, tt.wantClass; got != want {
				t.Fatalf("ClassifyError().Class = %q, want %q", got, want)
			}

			recovery := classified.Recovery()
			if got, want := recovery.Retry, tt.wantRetry; got != want {
				t.Fatalf("Recovery().Retry = %v, want %v", got, want)
			}
			if got, want := recovery.Status, tt.wantStatus; got != want {
				t.Fatalf("Recovery().Status = %q, want %q", got, want)
			}
			if tt.wantReason != "" {
				if recovery.Degradation == nil {
					t.Fatal("Recovery().Degradation = nil, want non-nil")
				}
				if got, want := recovery.Degradation.Reason, tt.wantReason; got != want {
					t.Fatalf("Recovery().Degradation.Reason = %q, want %q", got, want)
				}
			}
		})
	}
}

// Invariant: provider overload and server failures remain distinct from generic transport failures.
// Owning layer: bridge SDK error classification. Canonical suite: errors_test.go.
func TestClassifyErrorSeparatesOverloadServerAndTransientFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		wantClass ErrorClass
	}{
		{
			name:      "Should classify 529 as overloaded",
			err:       &HTTPError{StatusCode: HTTPStatusOverloaded, Message: "provider overloaded"},
			wantClass: ErrorClassOverloaded,
		},
		{
			name:      "Should classify 500 as a server error",
			err:       &HTTPError{StatusCode: http.StatusInternalServerError, Message: "server error"},
			wantClass: ErrorClassServerError,
		},
		{
			name:      "Should classify 502 as a server error",
			err:       &HTTPError{StatusCode: http.StatusBadGateway, Message: "bad gateway"},
			wantClass: ErrorClassServerError,
		},
		{
			name:      "Should classify 503 as a server error",
			err:       &HTTPError{StatusCode: http.StatusServiceUnavailable, Message: "unavailable"},
			wantClass: ErrorClassServerError,
		},
		{
			name:      "Should keep connection reset transient",
			err:       errors.New("read tcp: connection reset by peer"),
			wantClass: ErrorClassTransient,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			classified := ClassifyError(test.err)
			if got, want := classified.Class, test.wantClass; got != want {
				t.Fatalf("ClassifyError().Class = %q, want %q", got, want)
			}
			if !classified.Recovery().Retry {
				t.Fatalf("ClassifyError(%q).Recovery().Retry = false, want true", test.name)
			}
		})
	}
}

func TestRetryDoRetriesRateLimitedFailuresAndSucceeds(t *testing.T) {
	t.Parallel()

	attempts := 0
	result, err := RetryDo(context.Background(), RetryConfig{
		Attempts: 3,
		StandardBackoff: RetryBackoff{
			BaseDelay: time.Millisecond,
			MaxDelay:  2 * time.Millisecond,
		},
		RandFloat: func() float64 { return 0.5 },
	}, func(context.Context) (string, error) {
		attempts++
		if attempts < 3 {
			return "", &RateLimitError{
				Err:        errors.New("slow down"),
				RetryAfter: time.Millisecond,
			}
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("RetryDo() error = %v", err)
	}
	if got, want := result, "ok"; got != want {
		t.Fatalf("RetryDo() result = %q, want %q", got, want)
	}
	if got, want := attempts, 3; got != want {
		t.Fatalf("attempts = %d, want %d", got, want)
	}

	t.Run("Should never retry a rate limit nested inside a committed mutation", func(t *testing.T) {
		t.Parallel()

		committedErr := MarkCommittedMutation(&RateLimitError{
			Err:        errors.New("remote result unavailable after commit"),
			RetryAfter: time.Millisecond,
		})
		classified := ClassifyError(committedErr)
		if got, want := classified.Class, ErrorClassPermanent; got != want {
			t.Fatalf("ClassifyError(committed rate limit).Class = %q, want %q", got, want)
		}
		if classified.Recovery().Retry {
			t.Fatal("ClassifyError(committed rate limit).Recovery().Retry = true, want false")
		}

		attempts := 0
		_, err := RetryDo(context.Background(), RetryConfig{
			Attempts: 3,
			StandardBackoff: RetryBackoff{
				BaseDelay: time.Millisecond,
				MaxDelay:  2 * time.Millisecond,
			},
			RandFloat: func() float64 { return 0.5 },
		}, func(context.Context) (string, error) {
			attempts++
			return "", committedErr
		})
		if !errors.Is(err, committedErr) {
			t.Fatalf("RetryDo() error = %v, want committed mutation", err)
		}
		if got, want := attempts, 1; got != want {
			t.Fatalf("attempts = %d, want %d", got, want)
		}
	})
}

func TestDefaultRetryConfigAndErrorUnwrapHelpers(t *testing.T) {
	t.Parallel()

	config := DefaultRetryConfig()
	if config.Attempts <= 0 ||
		config.StandardBackoff.BaseDelay <= 0 ||
		config.StandardBackoff.MaxDelay <= 0 ||
		config.OverloadedBackoff.BaseDelay <= config.StandardBackoff.BaseDelay ||
		config.OverloadedBackoff.MaxDelay <= 0 {
		t.Fatalf("DefaultRetryConfig() = %#v, want positive retry settings", config)
	}

	root := errors.New("root")
	if !errors.Is((&AuthError{Err: root}).Unwrap(), root) {
		t.Fatal("AuthError.Unwrap() does not expose root error")
	}
	if !errors.Is((&RateLimitError{Err: root}).Unwrap(), root) {
		t.Fatal("RateLimitError.Unwrap() does not expose root error")
	}
	if !errors.Is((&TransientError{Err: root}).Unwrap(), root) {
		t.Fatal("TransientError.Unwrap() does not expose root error")
	}
	if !errors.Is((&PermanentError{Err: root}).Unwrap(), root) {
		t.Fatal("PermanentError.Unwrap() does not expose root error")
	}
	if !errors.Is((&CommittedMutationError{Err: root}).Unwrap(), root) {
		t.Fatal("CommittedMutationError.Unwrap() does not expose root error")
	}
	marked := MarkCommittedMutation(root)
	var committed *CommittedMutationError
	if !errors.As(marked, &committed) || !errors.Is(marked, root) {
		t.Fatalf("MarkCommittedMutation() = %v, want committed wrapper around root", marked)
	}
	if got := MarkCommittedMutation(marked); got != marked {
		t.Fatalf("MarkCommittedMutation(marked) = %v, want original marker", got)
	}
	if MarkCommittedMutation(nil) != nil {
		t.Fatal("MarkCommittedMutation(nil) error = non-nil")
	}
	if !IsCommittedMutation(marked) || IsCommittedMutation(root) {
		t.Fatalf(
			"IsCommittedMutation() = marked:%t root:%t, want true/false",
			IsCommittedMutation(marked),
			IsCommittedMutation(root),
		)
	}
	if !ShouldContinueTextDeliveryAfterProgress(nil) ||
		!ShouldContinueTextDeliveryAfterProgress(marked) ||
		ShouldContinueTextDeliveryAfterProgress(root) {
		t.Fatal("ShouldContinueTextDeliveryAfterProgress() did not isolate committed progress failures")
	}
}

func TestCommittedMutationErrorUsesFallbackForEmptyCauses(t *testing.T) {
	t.Run("Should keep the semantic acknowledgement reason non-empty", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			err  *CommittedMutationError
		}{
			{name: "nil receiver", err: nil},
			{name: "nil cause", err: &CommittedMutationError{}},
			{name: "empty cause", err: &CommittedMutationError{Err: errors.New("")}},
			{name: "whitespace cause", err: &CommittedMutationError{Err: errors.New("  \n ")}},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if got := test.err.Error(); got != committedMutationFallbackMessage {
					t.Fatalf("CommittedMutationError.Error() = %q, want %q", got, committedMutationFallbackMessage)
				}
			})
		}
	})
}

func TestClassifyErrorCoversHTTPNetAndStringFallbacks(t *testing.T) {
	t.Parallel()

	timeoutNetErr := &net.DNSError{IsTimeout: true}
	otherNetErr := &net.DNSError{Err: "temporary failure"}

	tests := []struct {
		name string
		err  error
		want ErrorClass
	}{
		{
			name: "http auth",
			err:  &HTTPError{StatusCode: http.StatusForbidden, Message: "forbidden"},
			want: ErrorClassAuth,
		},
		{
			name: "http timeout",
			err:  &HTTPError{StatusCode: http.StatusGatewayTimeout, Message: "timed out"},
			want: ErrorClassTimeout,
		},
		{
			name: "http server error",
			err:  &HTTPError{StatusCode: http.StatusServiceUnavailable, Message: "unavailable"},
			want: ErrorClassServerError,
		},
		{
			name: "http permanent",
			err:  &HTTPError{StatusCode: http.StatusBadRequest, Message: "bad request"},
			want: ErrorClassPermanent,
		},
		{name: "net timeout", err: timeoutNetErr, want: ErrorClassTimeout},
		{name: "net transient", err: otherNetErr, want: ErrorClassTransient},
		{name: "string auth", err: errors.New("authentication failed"), want: ErrorClassAuth},
		{name: "string rate limit", err: errors.New("rate limit exceeded"), want: ErrorClassRateLimit},
		{name: "string timeout", err: errors.New("request timeout"), want: ErrorClassTimeout},
		{name: "string transient", err: errors.New("temporary unavailable"), want: ErrorClassTransient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ClassifyError(tt.err).Class; got != tt.want {
				t.Fatalf("ClassifyError().Class = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRetryDoStopsOnPermanentErrorAndHonorsContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("Should stop immediately on permanent errors", func(t *testing.T) {
		t.Parallel()

		_, err := RetryDo(context.Background(), RetryConfig{
			Attempts:        3,
			StandardBackoff: RetryBackoff{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond},
		}, func(context.Context) (string, error) {
			return "", &PermanentError{Err: errors.New("bad request")}
		})
		if err == nil {
			t.Fatal("RetryDo(permanent) error = nil, want non-nil")
		}
	})

	t.Run("Should reject pre-canceled context before the first retry attempt", func(t *testing.T) {
		t.Parallel()

		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		attempts := 0
		_, err := RetryDo(canceled, RetryConfig{
			Attempts:        3,
			StandardBackoff: RetryBackoff{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond},
		}, func(context.Context) (string, error) {
			attempts++
			return "", &RateLimitError{Err: errors.New("slow down"), RetryAfter: time.Millisecond}
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RetryDo(canceled) error = %v, want context.Canceled", err)
		}
		if attempts != 0 {
			t.Fatalf("attempts = %d, want 0", attempts)
		}
	})
}

func TestRetryDoAppliesDefaultsAndStopsWhenDelayContextCancels(t *testing.T) {
	t.Parallel()

	t.Run("Should apply default single attempt", func(t *testing.T) {
		t.Parallel()

		_, err := RetryDo(context.Background(), RetryConfig{}, func(context.Context) (string, error) {
			return "", &TransientError{Err: errors.New("temporary failure")}
		})
		if err == nil {
			t.Fatal("RetryDo(default single attempt) error = nil, want transient failure")
		}
	})

	t.Run("Should stop when delay context cancels", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		onRetry := 0
		_, err := RetryDo(ctx, RetryConfig{
			Attempts: 2,
			OnRetry: func(context.Context, RetryAttempt) error {
				onRetry++
				cancel()
				return nil
			},
		}, func(context.Context) (string, error) {
			attempts++
			return "", &TransientError{Err: errors.New("temporary failure")}
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RetryDo(canceled during delay) error = %v, want context.Canceled", err)
		}
		if attempts != 1 {
			t.Fatalf("attempts = %d, want 1", attempts)
		}
		if onRetry != 1 {
			t.Fatalf("onRetry calls = %d, want 1", onRetry)
		}
	})
}

func TestRetryDoUsesRetryAfterAndDistinctBackoffProfiles(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		err       error
		wantClass ErrorClass
		wantDelay time.Duration
	}{
		{
			name: "Should prefer Retry-After",
			err: &RateLimitError{
				Err:        errors.New("rate limited"),
				RetryAfter: 25 * time.Millisecond,
			},
			wantClass: ErrorClassRateLimit,
			wantDelay: 25 * time.Millisecond,
		},
		{
			name:      "Should use the standard profile for server errors",
			err:       &HTTPError{StatusCode: http.StatusInternalServerError, Message: "server error"},
			wantClass: ErrorClassServerError,
			wantDelay: 10 * time.Millisecond,
		},
		{
			name:      "Should use the distinct overload profile for 529",
			err:       &HTTPError{StatusCode: HTTPStatusOverloaded, Message: "overloaded"},
			wantClass: ErrorClassOverloaded,
			wantDelay: 40 * time.Millisecond,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var (
				delays   []time.Duration
				observed []RetryAttempt
				attempts int
			)
			result, err := RetryDo(t.Context(), RetryConfig{
				Attempts:          2,
				StandardBackoff:   RetryBackoff{BaseDelay: 10 * time.Millisecond, MaxDelay: time.Second},
				OverloadedBackoff: RetryBackoff{BaseDelay: 40 * time.Millisecond, MaxDelay: time.Second},
				RandFloat:         func() float64 { return 0 },
				Sleep: func(_ context.Context, delay time.Duration) error {
					delays = append(delays, delay)
					return nil
				},
				OnRetry: func(_ context.Context, attempt RetryAttempt) error {
					observed = append(observed, attempt)
					return nil
				},
			}, func(context.Context) (string, error) {
				attempts++
				if attempts == 1 {
					return "", test.err
				}
				return "ok", nil
			})
			if err != nil {
				t.Fatalf("RetryDo() error = %v", err)
			}
			if result != "ok" || attempts != 2 {
				t.Fatalf("RetryDo() = result:%q attempts:%d, want ok/2", result, attempts)
			}
			if len(delays) != 1 || delays[0] != test.wantDelay {
				t.Fatalf("retry delays = %v, want [%s]", delays, test.wantDelay)
			}
			if len(observed) != 1 || observed[0].Classified.Class != test.wantClass {
				t.Fatalf("retry attempts = %#v, want one %q classification", observed, test.wantClass)
			}
			if observed[0].Delay != test.wantDelay {
				t.Fatalf("retry attempt delay = %s, want %s", observed[0].Delay, test.wantDelay)
			}
		})
	}
}

func TestErrorHelpersHandleEmptyValues(t *testing.T) {
	t.Parallel()

	t.Run("Should handle empty Error methods", func(t *testing.T) {
		t.Parallel()

		var nilHTTP *HTTPError
		if got := nilHTTP.Error(); got != "" {
			t.Fatalf("nil HTTPError.Error() = %q, want empty string", got)
		}
		if got := (&HTTPError{StatusCode: http.StatusTooManyRequests}).Error(); got == "" {
			t.Fatal("HTTPError.Error() = empty string, want fallback text")
		}
		if got := (&AuthError{}).Error(); got != "" {
			t.Fatalf("AuthError{}.Error() = %q, want empty string", got)
		}
		if got := (&RateLimitError{}).Error(); got != "" {
			t.Fatalf("RateLimitError{}.Error() = %q, want empty string", got)
		}
		if got := (&TransientError{}).Error(); got != "" {
			t.Fatalf("TransientError{}.Error() = %q, want empty string", got)
		}
		if got := (&PermanentError{}).Error(); got != "" {
			t.Fatalf("PermanentError{}.Error() = %q, want empty string", got)
		}
		if got := errorMessage(nil); got != "" {
			t.Fatalf("errorMessage(nil) = %q, want empty string", got)
		}
	})

	t.Run("Should handle nil Unwrap receivers", func(t *testing.T) {
		t.Parallel()

		var nilAuth *AuthError
		if err := nilAuth.Unwrap(); err != nil {
			t.Fatalf("nil AuthError.Unwrap() = %v, want nil", err)
		}
		var nilRateLimit *RateLimitError
		if err := nilRateLimit.Unwrap(); err != nil {
			t.Fatalf("nil RateLimitError.Unwrap() = %v, want nil", err)
		}
		var nilTransient *TransientError
		if err := nilTransient.Unwrap(); err != nil {
			t.Fatalf("nil TransientError.Unwrap() = %v, want nil", err)
		}
		var nilPermanent *PermanentError
		if err := nilPermanent.Unwrap(); err != nil {
			t.Fatalf("nil PermanentError.Unwrap() = %v, want nil", err)
		}
	})

	t.Run("Should classify nil and default errors safely", func(t *testing.T) {
		t.Parallel()

		if got := ClassifyError(nil); got.Class != "" || got.Err != nil {
			t.Fatalf("ClassifyError(nil) = %#v, want zero classification", got)
		}
		if got := ClassifyError(errors.New("domain-specific provider failure")); got.Class != ErrorClassPermanent {
			t.Fatalf("ClassifyError(default) = %q, want permanent", got.Class)
		}
	})
}
