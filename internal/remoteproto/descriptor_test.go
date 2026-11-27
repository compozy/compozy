package remoteproto

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestHostedProbeDescriptor(t *testing.T) {
	t.Parallel()
	const hostname = "host.example.com"
	origin := &url.URL{Scheme: "https", Host: hostname}

	t.Run(
		"Should recover from a cached negative answer with verified TLS for the assigned hostname",
		func(t *testing.T) {
			t.Parallel()
			edge := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Host != hostname || req.TLS.ServerName != hostname || req.URL.Path != DescriptorPath {
					t.Errorf(
						"unexpected edge request: host %q, SNI %q, path %q",
						req.Host,
						req.TLS.ServerName,
						req.URL.Path,
					)
					http.NotFound(w, req)
					return
				}
				if _, err := io.WriteString(w, `{"computer_id":"cmp_fixture","browser_sign_in":true}`); err != nil {
					t.Errorf("write descriptor: %v", err)
				}
			}))
			t.Cleanup(edge.Close)
			edgeTransport, ok := edge.Client().Transport.(*http.Transport)
			if !ok {
				t.Fatal("TLS fixture did not supply an HTTP transport")
			}
			public := edgeTransport.Clone()
			t.Cleanup(public.CloseIdleConnections)
			dialer := &net.Dialer{}
			public.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, edge.Listener.Addr().String())
			}
			client := &http.Client{Timeout: time.Second, Transport: hostedProbeTransport{
				system: probeDNSFailure(hostname), public: public,
			}}
			descriptor, err := FetchDescriptor(t.Context(), origin, client)
			if err != nil || descriptor.ComputerID != "cmp_fixture" || !descriptor.BrowserSignIn {
				t.Fatalf("descriptor = %+v, error = %v", descriptor, err)
			}
		},
	)

	t.Run("Should report that the assigned hostname has no address when both resolvers fail", func(t *testing.T) {
		t.Parallel()
		client := &http.Client{Transport: hostedProbeTransport{
			system: probeDNSFailure(hostname), public: probeDNSFailure(hostname),
		}}
		if _, err := FetchDescriptor(t.Context(), origin, client); !errors.Is(err, ErrDescriptorDNS) {
			t.Fatalf("descriptor error = %v, want %v", err, ErrDescriptorDNS)
		}
	})

	t.Run("Should keep an unrelated proxy DNS failure out of the public resolver", func(t *testing.T) {
		t.Parallel()
		client := &http.Client{Transport: hostedProbeTransport{
			system: probeDNSFailure("proxy.example.com"),
			public: probeRoundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("unexpected public lookup for the proxy")
				return nil, ErrDescriptorUnavailable
			}),
		}}
		if _, err := FetchDescriptor(t.Context(), origin, client); !errors.Is(err, ErrDescriptorUnavailable) {
			t.Fatalf("descriptor error = %v, want %v", err, ErrDescriptorUnavailable)
		}
	})
}

type probeRoundTripFunc func(*http.Request) (*http.Response, error)

func (f probeRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func probeDNSFailure(name string) probeRoundTripFunc {
	return func(*http.Request) (*http.Response, error) {
		return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
	}
}
