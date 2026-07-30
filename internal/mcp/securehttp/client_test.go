// Suite: secure outbound HTTP policy
// Invariant: MCP and OAuth requests never reach an unapproved network destination or forward credentials across origins.
// Boundary IN: URL policy, DNS validation, dialing, redirects, and response limits.
// Boundary OUT: MCP and OAuth protocol semantics, owned by their callers.
package securehttp

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
)

func TestClientValidateURL(t *testing.T) {
	t.Parallel()

	publicClient := NewClient()
	loopbackClient := NewClient(WithAllowLoopback(true))
	tests := []struct {
		name   string
		client *Client
		url    string
		want   error
	}{
		{
			name:   "Should require HTTPS for public destinations",
			client: publicClient,
			url:    "http://mcp.example.test/connect",
			want:   ErrInsecureTransport,
		},
		{
			name:   "Should reject private IP destinations",
			client: publicClient,
			url:    "https://10.0.0.8/connect",
			want:   ErrBlockedDestination,
		},
		{
			name:   "Should reject metadata IP destinations",
			client: publicClient,
			url:    "https://169.254.169.254/latest/meta-data",
			want:   ErrBlockedDestination,
		},
		{
			name:   "Should reject loopback without explicit permission",
			client: publicClient,
			url:    "https://127.0.0.1/mcp",
			want:   ErrBlockedDestination,
		},
		{
			name:   "Should allow explicit loopback HTTP when configured",
			client: loopbackClient,
			url:    "http://127.0.0.1:8080/mcp",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.client.ValidateURL(test.url)
			if !errors.Is(err, test.want) {
				t.Fatalf("ValidateURL() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestSecureDialerDNSPolicy(t *testing.T) {
	t.Run("Should reject a mixed public and private DNS response before dialing", func(t *testing.T) {
		t.Parallel()

		calls := 0
		dialer := secureDialer{
			policy: policy{},
			resolver: &sequenceResolver{answers: [][]netip.Addr{{
				netip.MustParseAddr("8.8.8.8"),
				netip.MustParseAddr("10.0.0.8"),
			}}},
			dialer: networkDialerFunc(func(context.Context, string, string) (net.Conn, error) {
				calls++
				return nil, errors.New("unexpected dial")
			}),
		}

		connection, err := dialer.DialContext(t.Context(), "tcp", "mcp.example.test:443")
		if connection != nil {
			if closeErr := connection.Close(); closeErr != nil {
				t.Fatalf("connection.Close() error = %v", closeErr)
			}
			t.Fatal("DialContext() returned a connection, want none")
		}
		if !errors.Is(err, ErrBlockedDestination) {
			t.Fatalf("DialContext() error = %v, want %v", err, ErrBlockedDestination)
		}
		if calls != 0 {
			t.Fatalf("dial calls = %d, want 0", calls)
		}
	})

	t.Run("Should revalidate DNS before every connection", func(t *testing.T) {
		t.Parallel()

		calls := 0
		dialer := secureDialer{
			policy: policy{},
			resolver: &sequenceResolver{answers: [][]netip.Addr{
				{netip.MustParseAddr("8.8.8.8")},
				{netip.MustParseAddr("169.254.169.254")},
			}},
			dialer: networkDialerFunc(func(context.Context, string, string) (net.Conn, error) {
				calls++
				client, server := net.Pipe()
				if err := server.Close(); err != nil {
					return nil, err
				}
				return client, nil
			}),
		}

		connection, err := dialer.DialContext(t.Context(), "tcp", "mcp.example.test:443")
		if err != nil {
			t.Fatalf("first DialContext() error = %v", err)
		}
		if err := connection.Close(); err != nil {
			t.Fatalf("first connection.Close() error = %v", err)
		}
		connection, err = dialer.DialContext(t.Context(), "tcp", "mcp.example.test:443")
		if connection != nil {
			if closeErr := connection.Close(); closeErr != nil {
				t.Fatalf("second connection.Close() error = %v", closeErr)
			}
			t.Fatal("second DialContext() returned a connection, want none")
		}
		if !errors.Is(err, ErrBlockedDestination) {
			t.Fatalf("second DialContext() error = %v, want %v", err, ErrBlockedDestination)
		}
		if calls != 1 {
			t.Fatalf("dial calls = %d, want 1", calls)
		}
	})

	t.Run("Should require loopback DNS answers for an explicit loopback host", func(t *testing.T) {
		t.Parallel()

		calls := 0
		dialer := secureDialer{
			policy: policy{allowLoopback: true},
			resolver: &sequenceResolver{answers: [][]netip.Addr{{
				netip.MustParseAddr("8.8.8.8"),
			}}},
			dialer: networkDialerFunc(func(context.Context, string, string) (net.Conn, error) {
				calls++
				return nil, errors.New("unexpected dial")
			}),
		}

		connection, err := dialer.DialContext(t.Context(), "tcp", "localhost:8080")
		if connection != nil {
			if closeErr := connection.Close(); closeErr != nil {
				t.Fatalf("connection.Close() error = %v", closeErr)
			}
			t.Fatal("DialContext() returned a connection, want none")
		}
		if !errors.Is(err, ErrBlockedDestination) {
			t.Fatalf("DialContext() error = %v, want %v", err, ErrBlockedDestination)
		}
		if calls != 0 {
			t.Fatalf("dial calls = %d, want 0", calls)
		}
	})
}

func TestClientRedirectPolicy(t *testing.T) {
	t.Run("Should strip credentials when following a cross-origin redirect", func(t *testing.T) {
		t.Parallel()

		targetHeaders := make(chan http.Header, 1)
		target := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			targetHeaders <- request.Header.Clone()
			response.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(target.Close)
		source := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			http.Redirect(response, request, target.URL, http.StatusFound)
		}))
		t.Cleanup(source.Close)

		request := newRequest(t, source.URL)
		request.Header.Set("Authorization", "Bearer secret-token")
		request.Header.Set("Cookie", "session=secret-cookie")
		response, err := NewClient(WithAllowLoopback(true)).Do(request)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer func() {
			if closeErr := response.Body.Close(); closeErr != nil {
				t.Errorf("response body Close() error = %v", closeErr)
			}
		}()
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("response status = %d, want %d", response.StatusCode, http.StatusNoContent)
		}
		headers := <-targetHeaders
		if authorization := headers.Get("Authorization"); authorization != "" {
			t.Fatalf("redirect Authorization = %q, want empty", authorization)
		}
		if cookie := headers.Get("Cookie"); cookie != "" {
			t.Fatalf("redirect Cookie = %q, want empty", cookie)
		}
	})

	t.Run("Should reject an insecure redirect without leaking query secrets", func(t *testing.T) {
		t.Parallel()

		source := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			http.Redirect(response, request, "http://mcp.example.test/callback?code=secret-code", http.StatusFound)
		}))
		t.Cleanup(source.Close)

		response, err := NewClient(WithAllowLoopback(true)).Do(newRequest(t, source.URL))
		if response != nil {
			defer func() {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Errorf("response body Close() error = %v", closeErr)
				}
			}()
			t.Fatal("Do() response was not nil, want nil")
		}
		if !errors.Is(err, ErrInsecureTransport) {
			t.Fatalf("Do() error = %v, want %v", err, ErrInsecureTransport)
		}
		if strings.Contains(err.Error(), "secret-code") {
			t.Fatalf("Do() error leaked redirect secret: %q", err)
		}
	})
}

func TestClientResponseBodyLimit(t *testing.T) {
	t.Run("Should stop reading a response that exceeds its configured limit", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			_, err := response.Write([]byte("four"))
			if err != nil {
				t.Errorf("response.Write() error = %v", err)
			}
		}))
		t.Cleanup(server.Close)

		response, err := NewClient(WithAllowLoopback(true), WithMaxResponseBodyBytes(3)).Do(newRequest(t, server.URL))
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		defer func() {
			if closeErr := response.Body.Close(); closeErr != nil {
				t.Errorf("response body Close() error = %v", closeErr)
			}
		}()
		body, readErr := io.ReadAll(response.Body)
		if !errors.Is(readErr, ErrResponseBodyTooLarge) {
			t.Fatalf("io.ReadAll() error = %v, want %v", readErr, ErrResponseBodyTooLarge)
		}
		if got, want := string(body), "fou"; got != want {
			t.Fatalf("response body = %q, want %q", got, want)
		}
	})
}

func TestClientTLSFloor(t *testing.T) {
	t.Run("Should require TLS 1.2 or newer", func(t *testing.T) {
		t.Parallel()

		transport, ok := NewClient().HTTPClient().Transport.(secureTransport)
		if !ok {
			t.Fatal("HTTP client transport does not enforce the secure transport policy")
		}
		baseTransport, ok := transport.base.(*http.Transport)
		if !ok {
			t.Fatal("secure transport base is not an HTTP transport")
		}
		if baseTransport.TLSClientConfig == nil {
			t.Fatal("TLS client config is nil")
		}
		if got, want := baseTransport.TLSClientConfig.MinVersion, uint16(tls.VersionTLS12); got != want {
			t.Fatalf("TLS minimum version = %d, want %d", got, want)
		}
	})
}

type sequenceResolver struct {
	mu      sync.Mutex
	answers [][]netip.Addr
}

func (r *sequenceResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.answers) == 0 {
		return nil, errors.New("test resolver exhausted")
	}
	answer := append([]netip.Addr(nil), r.answers[0]...)
	r.answers = r.answers[1:]
	return answer, nil
}

type networkDialerFunc func(context.Context, string, string) (net.Conn, error)

func (f networkDialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

func newRequest(t *testing.T, rawURL string) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	return request
}
