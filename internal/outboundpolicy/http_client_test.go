package outboundpolicy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewHTTPClientRoutesHTTPSThroughPolicyDialer(t *testing.T) {
	t.Parallel()

	policy, err := New(false).WithTrustedOrigin("https://127.0.0.1")
	if err != nil {
		t.Fatalf("WithTrustedOrigin() error = %v", err)
	}
	wantErr := errors.New("test dial stopped")
	dialAddress := make(chan string, 1)
	var tlsDialCalled atomic.Bool
	base := &http.Client{Transport: &http.Transport{
		DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
			dialAddress <- address
			return nil, wantErr
		},
		DialTLSContext: func(context.Context, string, string) (net.Conn, error) {
			tlsDialCalled.Store(true)
			return nil, errors.New("alternate TLS dialer must not run")
		},
	}}
	client := NewHTTPClient(base, policy, time.Second, 1)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://127.0.0.1/", http.NoBody)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	response, err := client.Do(request)
	if response != nil {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Fatalf("response.Body.Close() error = %v", closeErr)
		}
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("Client.Do() error = %v, want injected dial error", err)
	}
	select {
	case address := <-dialAddress:
		if address != "127.0.0.1:443" {
			t.Fatalf("DialContext() address = %q, want pinned loopback endpoint", address)
		}
	default:
		t.Fatal("policy DialContext() was not called")
	}
	if tlsDialCalled.Load() {
		t.Fatal("inherited DialTLSContext() was called")
	}
}

func TestPolicyOriginCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("Should keep credential trust separate from private network trust", func(t *testing.T) {
		t.Parallel()

		policy, err := New(false).WithCredentialOrigin("https://api.example.com")
		if err != nil {
			t.Fatalf("WithCredentialOrigin() error = %v", err)
		}
		destination, err := url.Parse("https://api.example.com/resource")
		if err != nil {
			t.Fatalf("url.Parse() error = %v", err)
		}
		if !policy.IsTrustedOrigin(destination) {
			t.Fatal("IsTrustedOrigin() = false, want credential forwarding permission")
		}
		err = policy.ValidateResolvedAddresses(
			[]netip.Addr{netip.MustParseAddr("127.0.0.1")},
			"api.example.com",
			"443",
		)
		if !errors.Is(err, ErrBlockedDestination) {
			t.Fatalf("ValidateResolvedAddresses() error = %v, want ErrBlockedDestination", err)
		}
	})

	t.Run("Should allow private addresses only for an operator trusted origin", func(t *testing.T) {
		t.Parallel()

		policy, err := New(false).WithTrustedOrigin("https://registry.internal")
		if err != nil {
			t.Fatalf("WithTrustedOrigin() error = %v", err)
		}
		if err := policy.ValidateResolvedAddresses(
			[]netip.Addr{netip.MustParseAddr("10.0.0.8")},
			"registry.internal",
			"443",
		); err != nil {
			t.Fatalf("ValidateResolvedAddresses() error = %v, want nil", err)
		}
	})
}
