package attachment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/compozy/compozy/pkg/logger"
)

// Defaults will be wired to global config in Task 4.0.
var (
	DefaultMaxDownloadSizeBytes int64 = 10 * 1024 * 1024
	DefaultDownloadTimeout            = 30 * time.Second
	DefaultMaxRedirects               = 3
)

// Sentinel errors for policy handling and tests.
var (
	ErrMaxRedirectsExceeded = errors.New("max redirects exceeded")
	ErrMaxSizeExceeded      = errors.New("download exceeds size limit")
)

// httpDownloadToTemp streams the content at urlStr into a temp file while
// enforcing size limits, timeouts, and redirect caps. It returns the temp
// file path and detected MIME (from initial bytes). Callers MUST Cleanup().
func httpDownloadToTemp(ctx context.Context, urlStr string, maxSize int64) (string, string, error) {
	if maxSize <= 0 {
		maxSize = DefaultMaxDownloadSizeBytes
	}
	client := &http.Client{Timeout: DefaultDownloadTimeout}
	redirects := 0
	client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
		redirects = len(via)
		if redirects >= DefaultMaxRedirects {
			return ErrMaxRedirectsExceeded
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, http.NoBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	tmpf, err := os.CreateTemp("", "compozy-att-*")
	if err != nil {
		return "", "", fmt.Errorf("temp file create failed: %w", err)
	}
	path := tmpf.Name()
	// Ensure cleanup if we fail after creation
	cleanupOnErr := func(e error) (string, string, error) {
		tmpf.Close()
		os.Remove(path)
		return "", "", e
	}
	// Stream with limit and accumulate up to 512 bytes for detection using LimitedReader + TeeReader
	hc := &headCapture{}
	lr := &io.LimitedReader{R: resp.Body, N: maxSize + 1}
	tee := io.TeeReader(lr, hc)
	written, cErr := io.Copy(tmpf, tee)
	if cErr != nil {
		return cleanupOnErr(fmt.Errorf("copy failed: %w", cErr))
	}
	if written > maxSize {
		return cleanupOnErr(fmt.Errorf("%w: %d bytes", ErrMaxSizeExceeded, maxSize))
	}
	if cerr := tmpf.Close(); cerr != nil {
		return cleanupOnErr(fmt.Errorf("close failed: %w", cerr))
	}
	mime := detectMIME(hc.Bytes())
	logger.FromContext(ctx).Debug("Downloaded attachment",
		"url", urlStr,
		"bytes", written,
		"mime", mime,
		"redirects", redirects,
	)
	return path, mime, nil
}

// headCapture captures up to 512 bytes written through it.
type headCapture struct{ b []byte }

func (h *headCapture) Write(p []byte) (int, error) {
	if len(h.b) < 512 {
		need := 512 - len(h.b)
		if need > len(p) {
			need = len(p)
		}
		h.b = append(h.b, p[:need]...)
	}
	return len(p), nil
}

func (h *headCapture) Bytes() []byte { return h.b }
