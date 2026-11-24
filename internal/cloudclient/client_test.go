package cloudclient

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

func TestCloudProxyEnvironment(t *testing.T) {
	// UT-133. ProxyFromEnvironment caches the process environment, so each case runs in a fresh process.
	if mode := os.Getenv("COMPOZY_TEST_CLOUD_PROXY_MODE"); mode != "" {
		testCloudProxyTransport(t, mode)
		return
	}
	t.Parallel()
	for _, tc := range []struct{ name, mode string }{
		{"Should send all cloud traffic through the HTTPS proxy", "proxy"},
		{"Should bypass the proxy for the cloud origin in NO_PROXY", "direct"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			child := exec.CommandContext(t.Context(), path, "-test.run=^TestCloudProxyEnvironment$")
			child.Env = append(os.Environ(), "COMPOZY_TEST_CLOUD_PROXY_MODE="+tc.mode)
			if output, err := child.CombinedOutput(); err != nil {
				t.Fatalf("cloud proxy fixture: %v\n%s", err, output)
			}
		})
	}
}

func testCloudProxyTransport(t *testing.T, mode string) {
	t.Helper()
	const cloudAddress = "cloud.fixture.test:443"
	var mu sync.Mutex
	var connections []string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect || r.Host != cloudAddress {
			t.Errorf("proxy got %s %s", r.Method, r.Host)
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(proxy.Close)
	t.Setenv("COMPOZY_CLOUD_URL", "https://"+cloudAddress)
	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
	if mode == "direct" {
		t.Setenv("NO_PROXY", "cloud.fixture.test")
	}
	client, err := newClient(false)
	if err != nil {
		t.Fatal(err)
	}
	transport := client.http.Transport.(*http.Transport)
	t.Cleanup(transport.CloseIdleConnections)
	dial := transport.DialContext
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		mu.Lock()
		connections = append(connections, address)
		mu.Unlock()
		if address == cloudAddress {
			return nil, errors.New("direct cloud connection observed")
		}
		return dial(ctx, network, address)
	}
	if _, err := client.Refresh(t.Context(), "refresh", nil); err == nil {
		t.Fatal("expected stubbed refresh failure")
	}
	if _, err := client.PollDeviceFlow(t.Context(), "grant"); err == nil {
		t.Fatal("expected stubbed poll failure")
	}
	if _, err := client.GatePolicy(t.Context()); err == nil {
		t.Fatal("expected stubbed gate-policy failure")
	}
	if _, err := client.OpenComputerControl(t.Context(), "cmp_fixture"); err == nil {
		t.Fatal("expected stubbed WebSocket failure")
	}
	want := proxy.Listener.Addr().String()
	if mode == "direct" {
		want = cloudAddress
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(connections, []string{want, want, want, want}) {
		t.Fatalf("cloud connections = %v, want four requests to %s", connections, want)
	}
}

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
