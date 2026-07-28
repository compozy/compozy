package registry

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestInstallerInstallContentTypeCleanupContract(t *testing.T) {
	t.Parallel()

	t.Run("Should close download reader when content type is rejected", func(t *testing.T) {
		t.Parallel()

		reader := &trackingInstallReadCloser{reader: strings.NewReader("<html>login</html>")}
		downloader := &stubDownloader{
			downloadFunc: func(context.Context, string, DownloadOpts) (*DownloadResult, error) {
				return &DownloadResult{
					ContentType: "text/html; charset=utf-8",
					Reader:      reader,
				}, nil
			},
		}

		_, err := NewInstaller(downloader).Install(
			context.Background(),
			"acme/html",
			DownloadOpts{},
			filepath.Join(t.TempDir(), "html"),
		)
		if !errors.Is(err, errUnexpectedContentType) {
			t.Fatalf("Install() error = %v, want errUnexpectedContentType", err)
		}
		if !reader.closed.Load() {
			t.Fatal("download reader closed = false, want true")
		}
	})

	t.Run("Should preserve reader close errors with rejected content type", func(t *testing.T) {
		t.Parallel()

		closeErr := errors.New("close failed")
		reader := &trackingInstallReadCloser{
			reader:   strings.NewReader("<html>login</html>"),
			closeErr: closeErr,
		}
		downloader := &stubDownloader{
			downloadFunc: func(context.Context, string, DownloadOpts) (*DownloadResult, error) {
				return &DownloadResult{
					ContentType: "text/html; charset=utf-8",
					Reader:      reader,
				}, nil
			},
		}

		_, err := NewInstaller(downloader).Install(
			context.Background(),
			"acme/html",
			DownloadOpts{},
			filepath.Join(t.TempDir(), "html"),
		)
		if !errors.Is(err, errUnexpectedContentType) {
			t.Fatalf("Install() error = %v, want errUnexpectedContentType", err)
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("Install() error = %v, want joined close error", err)
		}
		if !reader.closed.Load() {
			t.Fatal("download reader closed = false, want true")
		}
	})
}

type trackingInstallReadCloser struct {
	reader   io.Reader
	closeErr error
	closed   atomic.Bool
}

func (r *trackingInstallReadCloser) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *trackingInstallReadCloser) Close() error {
	r.closed.Store(true)
	return r.closeErr
}
