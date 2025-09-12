package attachment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	appconfig "github.com/compozy/compozy/pkg/config"
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
	effMaxSize, effTimeout, effMaxRedirects := computeAttachmentLimits(ctx, maxSize)
	client, redirects := makeHTTPClient(effTimeout, effMaxRedirects)
	rctx, cancel := context.WithTimeout(ctx, effTimeout)
	defer cancel()
	type result struct {
		path, mime string
		err        error
	}
	done := make(chan result, 1)
	go func() {
		req, err := http.NewRequestWithContext(rctx, http.MethodGet, urlStr, http.NoBody)
		if err != nil {
			done <- result{"", "", fmt.Errorf("failed to build request: %w", err)}
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			done <- result{"", "", fmt.Errorf("request failed: %w", err)}
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			done <- result{"", "", fmt.Errorf("unexpected status: %d", resp.StatusCode)}
			return
		}
		path, written, head, err := streamToTemp(effMaxSize, resp.Body)
		if err != nil {
			done <- result{"", "", err}
			return
		}
		mime := detectMIME(head)
		logger.FromContext(ctx).Debug(
			"Downloaded attachment",
			"url", urlStr,
			"bytes", written,
			"mime", mime,
			"redirects", *redirects,
		)
		done <- result{path, mime, nil}
	}()
	// Use a guard timer that respects a lowered package default when no config is present.
	guard := effTimeout
	if cfg := appconfig.FromContext(ctx); cfg == nil || cfg.Attachments.DownloadTimeout <= 0 {
		if DefaultDownloadTimeout > 0 && DefaultDownloadTimeout < guard {
			guard = DefaultDownloadTimeout
		}
	}
	timer := time.NewTimer(guard)
	defer timer.Stop()
	select {
	case r := <-done:
		return r.path, r.mime, r.err
	case <-timer.C:
		cancel()
		return "", "", context.DeadlineExceeded
	}
}

func computeAttachmentLimits(ctx context.Context, maxSize int64) (int64, time.Duration, int) {
	cfg := appconfig.FromContext(ctx)
	effMaxSize := DefaultMaxDownloadSizeBytes
	if maxSize > 0 {
		effMaxSize = maxSize
	} else if cfg != nil && cfg.Attachments.MaxDownloadSizeBytes > 0 {
		effMaxSize = int64(cfg.Attachments.MaxDownloadSizeBytes)
	}
	effTimeout := DefaultDownloadTimeout
	if cfg != nil && cfg.Attachments.DownloadTimeout > 0 {
		effTimeout = cfg.Attachments.DownloadTimeout
	}
	effMaxRedirects := DefaultMaxRedirects
	if cfg != nil && cfg.Attachments.MaxRedirects > 0 {
		effMaxRedirects = cfg.Attachments.MaxRedirects
	}
	return effMaxSize, effTimeout, effMaxRedirects
}

func makeHTTPClient(timeout time.Duration, maxRedirects int) (*http.Client, *int) {
	redirects := 0
	client := &http.Client{Timeout: timeout, Transport: &http.Transport{ResponseHeaderTimeout: timeout}}
	client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
		redirects = len(via)
		if redirects >= maxRedirects {
			return ErrMaxRedirectsExceeded
		}
		return nil
	}
	return client, &redirects
}

func streamToTemp(limit int64, r io.Reader) (string, int64, []byte, error) {
	tmpf, err := os.CreateTemp("", "compozy-att-*")
	if err != nil {
		return "", 0, nil, fmt.Errorf("temp file create failed: %w", err)
	}
	path := tmpf.Name()
	hc := &headCapture{}
	lr := &io.LimitedReader{R: r, N: limit + 1}
	tee := io.TeeReader(lr, hc)
	written, cErr := io.Copy(tmpf, tee)
	if cErr != nil {
		tmpf.Close()
		os.Remove(path)
		return "", 0, nil, fmt.Errorf("copy failed: %w", cErr)
	}
	if written > limit {
		tmpf.Close()
		os.Remove(path)
		return "", 0, nil, fmt.Errorf("%w: %d bytes", ErrMaxSizeExceeded, limit)
	}
	if cerr := tmpf.Close(); cerr != nil {
		os.Remove(path)
		return "", 0, nil, fmt.Errorf("close failed: %w", cerr)
	}
	return path, written, hc.Bytes(), nil
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
