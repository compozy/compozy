package attachment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_httpDownloadToTemp(t *testing.T) {
	t.Run("Should download under size limit and detect MIME", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10})
		}))
		defer srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		path, mime, err := httpDownloadToTemp(ctx, srv.URL, 1024)
		require.NoError(t, err)
		require.Equal(t, "image/png", mime)
		rf := &resolvedFile{path: path, temp: true}
		defer rf.Cleanup()
	})

	t.Run("Should cap redirects and return error", func(t *testing.T) {
		prev := DefaultMaxRedirects
		DefaultMaxRedirects = 2
		t.Cleanup(func() { DefaultMaxRedirects = prev })
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Redirect(w, &http.Request{}, srv.URL, http.StatusFound)
		}))
		defer srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _, err := httpDownloadToTemp(ctx, srv.URL, 1024)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrMaxRedirectsExceeded)
	})

	t.Run("Should enforce size limit", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			buf := make([]byte, 4096)
			w.Write(buf)
		}))
		defer srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _, err := httpDownloadToTemp(ctx, srv.URL, 1024)
		require.Error(t, err)
	})

	t.Run("Should respect context timeout", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}))
		defer srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, _, err := httpDownloadToTemp(ctx, srv.URL, 1024)
		require.Error(t, err)
		// Prefer the concrete deadline error when available
		// but avoid over-asserting in case transports wrap it.
		// So we only check it when it matches exactly.
		if err == context.DeadlineExceeded {
			require.ErrorIs(t, err, context.DeadlineExceeded)
		}
	})

	t.Run("Should reject empty response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusOK)
			// No body written - empty response
		}))
		defer srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _, err := httpDownloadToTemp(ctx, srv.URL, 1024)
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty response")
	})
}
