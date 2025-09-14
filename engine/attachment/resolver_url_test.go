package attachment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Resolver_URL_Attachment_Scheme_Validation(t *testing.T) {
	t.Run("Should accept absolute https URL", func(t *testing.T) {
		a := &URLAttachment{URL: "https://example.com/path"}
		res, err := resolveURL(context.Background(), a)
		require.NoError(t, err)
		u, ok := res.AsURL()
		require.True(t, ok)
		require.Equal(t, "https://example.com/path", u)
	})

	t.Run("Should reject unsupported scheme file", func(t *testing.T) {
		a := &URLAttachment{URL: "file:///etc/passwd"}
		_, err := resolveURL(context.Background(), a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported URL scheme")
	})

	t.Run("Should reject relative URL", func(t *testing.T) {
		a := &URLAttachment{URL: "/local/path"}
		_, err := resolveURL(context.Background(), a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "must be absolute")
	})
}

func Test_Resolver_Image_URL_Scheme_Validation(t *testing.T) {
	t.Run("Should reject image URL with non-HTTP scheme", func(t *testing.T) {
		a := &ImageAttachment{Source: SourceURL, URL: "javascript:alert(1)"}
		_, err := resolveImage(context.Background(), a, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported URL scheme")
	})
}

func Test_httpDownloadToTemp_Rejects_Unsupported_Scheme(t *testing.T) {
	t.Run("Should reject file scheme early", func(t *testing.T) {
		_, _, err := httpDownloadToTemp(context.Background(), "file:///etc/hosts", 1024)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported URL scheme")
	})
}
