package attachment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/engine/attachment/testutil"
	"github.com/stretchr/testify/require"
)

func Test_Resolver_URL_MIME_Denied_PDF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("plain"))
	}))
	defer srv.Close()
	before := testutil.SnapshotTempFiles(t)
	a := &PDFAttachment{Source: SourceURL, URL: srv.URL}
	_, err := resolvePDF(context.Background(), a, nil)
	require.Error(t, err)
	after := testutil.SnapshotTempFiles(t)
	require.Equal(t, before, after, "no leftover temp files expected")
}

func Test_Resolver_URL_MIME_Denied_Audio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("plain"))
	}))
	defer srv.Close()
	before := testutil.SnapshotTempFiles(t)
	a := &AudioAttachment{Source: SourceURL, URL: srv.URL}
	_, err := resolveAudio(context.Background(), a, nil)
	require.Error(t, err)
	after := testutil.SnapshotTempFiles(t)
	require.Equal(t, before, after, "no leftover temp files expected")
}
