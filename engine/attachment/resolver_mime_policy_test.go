package attachment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// snapshotTempFiles returns files in os.TempDir with our prefix
func snapshotTempFiles(t *testing.T) map[string]struct{} {
	t.Helper()
	out := map[string]struct{}{}
	entries, _ := os.ReadDir(os.TempDir())
	for _, e := range entries {
		name := e.Name()
		if len(name) >= 13 && name[:13] == "compozy-att-" {
			out[filepath.Join(os.TempDir(), name)] = struct{}{}
		}
	}
	return out
}

func Test_Resolver_URL_MIME_Denied_PDF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("plain"))
	}))
	defer srv.Close()
	before := snapshotTempFiles(t)
	a := &PDFAttachment{Source: SourceURL, URL: srv.URL}
	_, err := resolvePDF(context.Background(), a, nil)
	require.Error(t, err)
	after := snapshotTempFiles(t)
	require.Equal(t, before, after, "no leftover temp files expected")
}

func Test_Resolver_URL_MIME_Denied_Audio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("plain"))
	}))
	defer srv.Close()
	before := snapshotTempFiles(t)
	a := &AudioAttachment{Source: SourceURL, URL: srv.URL}
	_, err := resolveAudio(context.Background(), a, nil)
	require.Error(t, err)
	after := snapshotTempFiles(t)
	require.Equal(t, before, after, "no leftover temp files expected")
}
