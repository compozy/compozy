package cloudclient

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

func TestCloudOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, origin string
		required     bool
	}{
		{name: "Should use staging on development builds", origin: "https://staging.cloud.compozy.com"},
		{name: "Should use production on sign-in-required builds", origin: productionURL, required: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// UT-131. Every operation goes through the same origin and HTTP transport.
			t.Setenv("COMPOZY_CLOUD_URL", "https://staging.cloud.compozy.com")
			client, err := newClient(tc.required)
			if err != nil {
				t.Fatal(err)
			}
			client.http.Transport = accountRoundTrip(func(req *http.Request) (*http.Response, error) {
				got := req.URL.Scheme + "://" + req.URL.Host
				if got != tc.origin {
					t.Errorf("cloud origin = %s, want %s", got, tc.origin)
				}
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       http.NoBody,
					Header:     make(http.Header),
				}, nil
			})
			if err := client.CancelDeviceFlow(t.Context(), "grant"); err != nil {
				t.Fatal(err)
			}
			if err := client.RevokeDevice(t.Context(), Credentials{}, "dev_test"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCloudBackoff(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, code string
		status     int
	}{
		{name: "Should honor rate limiting", code: "rate_limited", status: http.StatusTooManyRequests},
		{name: "Should map cloud unavailability and honor its retry window", code: "account_cloud_unreachable", status: http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
			client, err := newClient(true, WithClock(func() time.Time { return now }))
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			client.http.Transport = accountRoundTrip(func(_ *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return &http.Response{
						StatusCode: tc.status,
						Header:     http.Header{"Retry-After": {"120"}},
						Body:       http.NoBody,
					}, nil
				}
				return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
			})
			_, err = client.StartDeviceFlow(t.Context(), remoteproto.DeviceFlowRequest{})
			failure, ok := errors.AsType[*Failure](err)
			if !ok || failure.Code != tc.code || RetryDelay(now, err) != 120*time.Second {
				t.Fatalf("cloud error = %#v, want %s and 120-second retry", failure, tc.code)
			}
			now = now.Add(119 * time.Second)
			if err := client.CancelDeviceFlow(t.Context(), "grant"); err != failure || calls != 1 {
				t.Fatalf("before retry window: error %v, network calls %d", err, calls)
			}
			now = now.Add(time.Second)
			if err := client.CancelDeviceFlow(t.Context(), "grant"); err != nil || calls != 2 {
				t.Fatalf("after retry window: error %v, network calls %d", err, calls)
			}
		})
	}
}

type accountRoundTrip func(*http.Request) (*http.Response, error)

func (r accountRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) { return r(req) }
