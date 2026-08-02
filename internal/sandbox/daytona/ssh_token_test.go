package daytona

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRESTSSHTokenSourceFetchesTokenAndExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(30 * time.Minute)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/sandbox/sandbox-token/ssh-access"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("expiresInMinutes"), "60"; got != want {
			t.Fatalf("expiresInMinutes = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer api-key"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		writeJSON(t, w, map[string]any{
			"token":     "ssh-token",
			"expiresAt": expiresAt.Format(time.RFC3339),
		})
	}))
	defer server.Close()
	source := newRESTSSHTokenSourceWithClient(
		server.Client(),
		func() string { return "api-key" },
		func() time.Time { return now },
	)
	access, err := source.FetchSSHAccess(context.Background(), server.URL, "sandbox-token", time.Hour)
	if err != nil {
		t.Fatalf("FetchSSHAccess() error = %v", err)
	}
	if access.Token != "ssh-token" || !access.IssuedAt.Equal(now) || !access.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("access = %#v, want parsed token/expiry", access)
	}
}

func TestRESTSSHTokenSourceRejectsMissingKeyAndBadStatus(t *testing.T) {
	t.Parallel()

	source := newRESTSSHTokenSourceWithClient(nil, func() string { return "" }, time.Now)
	if _, err := source.FetchSSHAccess(context.Background(), defaultAPIURL, "sandbox", time.Hour); err == nil {
		t.Fatal("FetchSSHAccess(missing key) error = nil, want error")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer server.Close()
	source = newRESTSSHTokenSourceWithClient(server.Client(), func() string { return "api-key" }, time.Now)
	if _, err := source.FetchSSHAccess(context.Background(), server.URL, "sandbox", time.Hour); err == nil {
		t.Fatal("FetchSSHAccess(bad status) error = nil, want error")
	}
}

func TestNewRESTSSHTokenSourceNormalizesHTTPClient(t *testing.T) {
	t.Parallel()

	base := &http.Client{}
	source := newRESTSSHTokenSourceWithClient(base, func() string { return "api-key" }, time.Now)
	if source.httpClient == base {
		t.Fatal("newRESTSSHTokenSourceWithClient() reused caller-owned client")
	}
	if source.httpClient.Timeout != defaultSSHRequestTimeout {
		t.Fatalf("HTTP timeout = %s, want %s", source.httpClient.Timeout, defaultSSHRequestTimeout)
	}
	if base.Timeout != 0 {
		t.Fatalf("caller HTTP timeout = %s, want unchanged zero value", base.Timeout)
	}
}

func TestRESTSSHTokenSourcePreservesResponseCleanupFailures(t *testing.T) {
	t.Parallel()

	readErr := errors.New("SSH response read failed")
	drainErr := errors.New("SSH response drain failed")
	closeErr := errors.New("SSH response close failed")
	body := &scriptedSSHResponseBody{
		readErrors: []error{readErr, drainErr},
		closeErr:   closeErr,
	}
	source := newRESTSSHTokenSourceWithClient(
		&http.Client{Transport: daytonaRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Status:     "502 Bad Gateway",
				Header:     make(http.Header),
				Body:       body,
				Request:    request,
			}, nil
		})},
		func() string { return "api-key" },
		time.Now,
	)
	_, err := source.FetchSSHAccess(t.Context(), "https://api.example.test", "sandbox", time.Hour)
	for _, want := range []error{readErr, drainErr, closeErr} {
		if !errors.Is(err, want) {
			t.Fatalf("FetchSSHAccess() error = %v, want %v", err, want)
		}
	}
	if body.closeCalls != 1 {
		t.Fatalf("response body close calls = %d, want 1", body.closeCalls)
	}
}

type daytonaRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f daytonaRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type scriptedSSHResponseBody struct {
	readErrors []error
	readCalls  int
	closeErr   error
	closeCalls int
}

func (b *scriptedSSHResponseBody) Read([]byte) (int, error) {
	if b.readCalls >= len(b.readErrors) {
		return 0, io.EOF
	}
	err := b.readErrors[b.readCalls]
	b.readCalls++
	return 0, err
}

func (b *scriptedSSHResponseBody) Close() error {
	b.closeCalls++
	return b.closeErr
}
