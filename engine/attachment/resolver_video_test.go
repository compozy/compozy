package attachment

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Resolver_URL_MIME_Denied_Video(t *testing.T) {
	t.Run("Should reject URL when MIME not allowed and not leak temp files", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("plain"))
			require.NoError(t, err)
		}))
		defer srv.Close()
		before := SnapshotTempFiles(t)
		a := &VideoAttachment{Source: SourceURL, URL: srv.URL}
		_, err := resolveVideo(t.Context(), a, nil)
		require.Error(t, err)
		after := SnapshotTempFiles(t)
		require.Equal(t, before, after)
	})
}

// helper removed; centralized in testutil
