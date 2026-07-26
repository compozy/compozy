// Suite: bridge webhook verification
// Invariant: reachability probes dial only publicly routable callback addresses and preserve actionable status semantics.
// Boundary IN: persisted public callback URL, DNS answers, HTTP status, and transport outcome.
// Boundary OUT: provider webhook registration and platform delivery transports.
package bridgesdk

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func TestWebhookCheckRecordsRemainStageAware(t *testing.T) {
	t.Run("Should validate configuration and skip reachability while disabled", func(t *testing.T) {
		t.Parallel()

		called := false
		records := WebhookCheckRecords(context.Background(), bridgepkg.BridgeInstance{
			Enabled:        false,
			ProviderConfig: []byte(`{"webhook":{"public_url":"https://bridge.example.test/slack"}}`),
		}, webhookHTTPDoerFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return nil, nil
		}))

		if called {
			t.Fatal("WebhookCheckRecords() dialed while the instance was disabled")
		}
		assertWebhookCheckStatus(t, records, "webhook.configuration", bridgepkg.BridgeCheckStatusPass)
		assertWebhookCheckStatus(t, records, "webhook.reachability", bridgepkg.BridgeCheckStatusSkipped)
	})

	t.Run("Should fail configuration without dialing an invalid public URL", func(t *testing.T) {
		t.Parallel()

		called := false
		records := WebhookCheckRecords(context.Background(), bridgepkg.BridgeInstance{
			Enabled:        true,
			ProviderConfig: []byte(`{"webhook":{"public_url":"http://bridge.example.test/slack"}}`),
		}, webhookHTTPDoerFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return nil, nil
		}))

		if called {
			t.Fatal("WebhookCheckRecords() dialed an invalid public URL")
		}
		assertWebhookCheckStatus(t, records, "webhook.configuration", bridgepkg.BridgeCheckStatusFail)
	})

	t.Run("Should reject an internal public URL before invoking the HTTP doer", func(t *testing.T) {
		t.Parallel()

		called := false
		records := WebhookCheckRecords(context.Background(), bridgepkg.BridgeInstance{
			Enabled:        true,
			ProviderConfig: []byte(`{"webhook":{"public_url":"https://169.254.169.254/latest/meta-data"}}`),
		}, webhookHTTPDoerFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return nil, nil
		}))

		if called {
			t.Fatal("WebhookCheckRecords() invoked the HTTP doer for an internal public URL")
		}
		assertWebhookCheckStatus(t, records, "webhook.configuration", bridgepkg.BridgeCheckStatusFail)
	})

	t.Run("Should classify enabled reachability without following redirects", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			statusCode int
			want       bridgepkg.BridgeCheckStatus
		}{
			{
				name:       "Should pass an authentication response",
				statusCode: http.StatusUnauthorized,
				want:       bridgepkg.BridgeCheckStatusPass,
			},
			{
				name:       "Should fail a redirect response",
				statusCode: http.StatusFound,
				want:       bridgepkg.BridgeCheckStatusFail,
			},
			{
				name:       "Should fail a missing route",
				statusCode: http.StatusNotFound,
				want:       bridgepkg.BridgeCheckStatusFail,
			},
			{
				name:       "Should fail a removed route",
				statusCode: http.StatusGone,
				want:       bridgepkg.BridgeCheckStatusFail,
			},
			{
				name:       "Should warn on a server error",
				statusCode: http.StatusServiceUnavailable,
				want:       bridgepkg.BridgeCheckStatusWarn,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				records := WebhookCheckRecords(context.Background(), bridgepkg.BridgeInstance{
					Enabled:        true,
					ProviderConfig: []byte(`{"webhook":{"public_url":"https://bridge.example.test/telegram"}}`),
				}, webhookHTTPDoerFunc(func(request *http.Request) (*http.Response, error) {
					if request.Method != http.MethodHead {
						t.Errorf("method = %q, want %q", request.Method, http.MethodHead)
					}
					return &http.Response{
						StatusCode: tt.statusCode,
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				}))

				assertWebhookCheckStatus(t, records, "webhook.configuration", bridgepkg.BridgeCheckStatusPass)
				assertWebhookCheckStatus(t, records, "webhook.reachability", tt.want)
			})
		}
	})

	t.Run("Should warn when the reachability request times out", func(t *testing.T) {
		t.Parallel()

		records := WebhookCheckRecords(context.Background(), bridgepkg.BridgeInstance{
			Enabled:        true,
			ProviderConfig: []byte(`{"webhook":{"public_url":"https://bridge.example.test/slack"}}`),
		}, webhookHTTPDoerFunc(func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}))

		assertWebhookCheckStatus(t, records, "webhook.reachability", bridgepkg.BridgeCheckStatusWarn)
	})
}

func TestSafeWebhookDialerRejectsDNSRebindingBeforeDial(t *testing.T) {
	t.Run(
		"Should re-resolve every connection and reject an internal rebound address before the second dial",
		func(t *testing.T) {
			t.Parallel()

			resolver := &sequenceWebhookResolver{answers: [][]netip.Addr{
				{netip.MustParseAddr("8.8.8.8")},
				{netip.MustParseAddr("169.254.169.254")},
			}}
			dialCalls := 0
			dialer := webhookNetworkDialerFunc(func(context.Context, string, string) (net.Conn, error) {
				dialCalls++
				client, server := net.Pipe()
				if err := server.Close(); err != nil {
					return nil, err
				}
				return client, nil
			})
			safeDialer := &safeWebhookDialer{resolver: resolver, dialer: dialer}

			connection, err := safeDialer.DialContext(context.Background(), "tcp", "bridge.example.test:443")
			if err != nil {
				t.Fatalf("first DialContext() error = %v", err)
			}
			if err := connection.Close(); err != nil {
				t.Fatalf("connection.Close() error = %v", err)
			}
			connection, err = safeDialer.DialContext(context.Background(), "tcp", "bridge.example.test:443")
			if err == nil {
				if connection != nil {
					if closeErr := connection.Close(); closeErr != nil {
						t.Fatalf("unexpected connection.Close() error = %v", closeErr)
					}
				}
				t.Fatal("second DialContext() error = nil, want DNS rebinding rejection")
			}
			if dialCalls != 1 {
				t.Fatalf("network dial calls = %d, want 1", dialCalls)
			}
		},
	)

	t.Run("Should reject a mixed public and internal DNS answer before any dial", func(t *testing.T) {
		t.Parallel()

		resolver := &sequenceWebhookResolver{answers: [][]netip.Addr{{
			netip.MustParseAddr("8.8.8.8"),
			netip.MustParseAddr("10.0.0.8"),
		}}}
		dialCalls := 0
		safeDialer := &safeWebhookDialer{
			resolver: resolver,
			dialer: webhookNetworkDialerFunc(func(context.Context, string, string) (net.Conn, error) {
				dialCalls++
				return nil, errors.New("unexpected dial")
			}),
		}

		connection, err := safeDialer.DialContext(context.Background(), "tcp", "bridge.example.test:443")
		if err == nil {
			if connection != nil {
				if closeErr := connection.Close(); closeErr != nil {
					t.Fatalf("unexpected connection.Close() error = %v", closeErr)
				}
			}
			t.Fatal("DialContext() error = nil, want mixed-answer rejection")
		}
		if dialCalls != 0 {
			t.Fatalf("network dial calls = %d, want 0", dialCalls)
		}
	})
}

type sequenceWebhookResolver struct {
	mu      sync.Mutex
	answers [][]netip.Addr
}

func (r *sequenceWebhookResolver) LookupNetIP(
	context.Context,
	string,
	string,
) ([]netip.Addr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.answers) == 0 {
		return nil, errors.New("test resolver exhausted")
	}
	answer := append([]netip.Addr(nil), r.answers[0]...)
	r.answers = r.answers[1:]
	return answer, nil
}

type webhookNetworkDialerFunc func(context.Context, string, string) (net.Conn, error)

func (f webhookNetworkDialerFunc) DialContext(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	return f(ctx, network, address)
}

func assertWebhookCheckStatus(
	t *testing.T,
	records []bridgepkg.BridgeCheckRecord,
	check string,
	want bridgepkg.BridgeCheckStatus,
) {
	t.Helper()

	for _, record := range records {
		if record.Check != check {
			continue
		}
		if record.Status != want {
			t.Fatalf("%s status = %q, want %q", check, record.Status, want)
		}
		return
	}
	t.Fatalf("records = %#v, want %q", records, check)
}
