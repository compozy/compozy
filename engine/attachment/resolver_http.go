package attachment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/compozy/compozy/engine/core"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// Defaults live in constants.go and mirror global config defaults.

// Sentinel errors for policy handling and tests.
var (
	ErrMaxRedirectsExceeded = errors.New("max redirects exceeded")
	ErrMaxSizeExceeded      = errors.New("download exceeds size limit")
)

// httpDownloadToTemp streams the content at urlStr into a temp file while
// enforcing size limits, timeouts, and redirect caps. It returns the temp
// file path and detected MIME (from initial bytes). Callers MUST Cleanup().
func httpDownloadToTemp(ctx context.Context, urlStr string, maxSize int64) (string, string, error) {
	s, err := validateHTTPURL(ctx, urlStr)
	if err != nil {
		return "", "", err
	}
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
		req, err := http.NewRequestWithContext(rctx, http.MethodGet, s, http.NoBody)
		if err != nil {
			done <- result{"", "", fmt.Errorf("failed to build request: %w", err)}
			return
		}
		// Set user-agent for better compatibility
		ua := HTTPUserAgent
		if cfg := appconfig.FromContext(ctx); cfg != nil && cfg.Attachments.HTTPUserAgent != "" {
			ua = cfg.Attachments.HTTPUserAgent
		}
		req.Header.Set("User-Agent", ua)
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
		// Check for empty response
		if resp.ContentLength == 0 {
			done <- result{"", "", fmt.Errorf("empty response from server")}
			return
		}
		path, written, head, err := streamToTemp(rctx, effMaxSize, resp.Body)
		if err != nil {
			done <- result{"", "", err}
			return
		}
		if written == 0 {
			_ = os.Remove(path)
			done <- result{"", "", fmt.Errorf("empty response from server")}
			return
		}
		mime := detectMIME(head)
		logger.FromContext(ctx).Debug(
			"Downloaded attachment",
			"url", sanitizeURL(s),
			"bytes", written,
			"mime", mime,
			"redirects", *redirects,
		)
		done <- result{path, mime, nil}
	}()
	// Return on completion or context timeout.
	select {
	case r := <-done:
		return r.path, r.mime, r.err
	case <-rctx.Done():
		return "", "", rctx.Err()
	}
}

func computeAttachmentLimits(ctx context.Context, maxSize int64) (int64, time.Duration, int) {
	cfg := appconfig.FromContext(ctx)
	effMaxSize := DefaultMaxDownloadSizeBytes
	if maxSize > 0 {
		effMaxSize = maxSize
	} else if cfg != nil && cfg.Attachments.MaxDownloadSizeBytes > 0 {
		effMaxSize = cfg.Attachments.MaxDownloadSizeBytes
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
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			// Keep ResponseHeaderTimeout for slow servers
			ResponseHeaderTimeout: timeout,
		},
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		redirects = len(via)
		if redirects >= maxRedirects {
			return ErrMaxRedirectsExceeded
		}
		if _, err := validateHTTPURL(req.Context(), req.URL.String()); err != nil {
			return err
		}
		return nil
	}
	return client, &redirects
}

func streamToTemp(ctx context.Context, limit int64, r io.Reader) (string, int64, []byte, error) {
	tmpf, err := os.CreateTemp("", TempFilePrefix+"*")
	if err != nil {
		return "", 0, nil, fmt.Errorf("temp file create failed: %w", err)
	}
	path := tmpf.Name()
	// Ensure cleanup on any error
	cleanup := func() {
		tmpf.Close()
		os.Remove(path)
	}
	hc := &headCapture{limit: mimeHeadLimit(ctx)}
	lr := &io.LimitedReader{R: r, N: limit + 1}
	tee := io.TeeReader(lr, hc)
	written, cErr := io.Copy(tmpf, tee)
	if cErr != nil {
		cleanup()
		return "", 0, nil, fmt.Errorf("copy failed: %w", cErr)
	}
	if written > limit {
		cleanup()
		return "", 0, nil, fmt.Errorf("%w: %d bytes", ErrMaxSizeExceeded, limit)
	}
	if cerr := tmpf.Close(); cerr != nil {
		os.Remove(path)
		return "", 0, nil, fmt.Errorf("close failed: %w", cerr)
	}
	return path, written, hc.Bytes(), nil
}

// headCapture captures up to MIMEHeadMaxBytes written through it.
type headCapture struct {
	b     []byte
	limit int
}

func (h *headCapture) Write(p []byte) (int, error) {
	if len(h.b) < h.limit {
		need := h.limit - len(h.b)
		if need > len(p) {
			need = len(p)
		}
		h.b = append(h.b, p[:need]...)
	}
	return len(p), nil
}

func (h *headCapture) Bytes() []byte { return h.b }

// sanitizeURL redacts sensitive information from URLs for logging purposes
func sanitizeURL(url string) string {
	return core.RedactString(url)
}
