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

// downloadResult carries the outcome of an HTTP download attempt.
type downloadResult struct {
	path string
	mime string
	err  error
}

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
	done := make(chan downloadResult, 1)
	go func() {
		done <- executeHTTPDownload(ctx, rctx, client, redirects, s, effMaxSize)
	}()
	return awaitDownload(rctx, done)
}

// executeHTTPDownload performs the HTTP download and returns its outcome.
func executeHTTPDownload(
	ctx context.Context,
	requestCtx context.Context,
	client *http.Client,
	redirects *int,
	url string,
	maxSize int64,
) downloadResult {
	req, err := buildDownloadRequest(ctx, requestCtx, url)
	if err != nil {
		return downloadResult{"", "", err}
	}
	resp, err := client.Do(req)
	if err != nil {
		return downloadResult{"", "", fmt.Errorf("request failed: %w", err)}
	}
	defer resp.Body.Close()
	if err := validateHTTPResponse(resp); err != nil {
		return downloadResult{"", "", err}
	}
	path, written, head, err := streamToTemp(requestCtx, maxSize, resp.Body)
	if err != nil {
		return downloadResult{"", "", err}
	}
	if err := ensureContentWritten(path, written); err != nil {
		return downloadResult{"", "", err}
	}
	mime := detectMIME(head)
	logDownloadSuccess(ctx, url, written, mime, redirects)
	return downloadResult{path: path, mime: mime, err: nil}
}

// awaitDownload waits for the download result or context cancellation.
func awaitDownload(ctx context.Context, done <-chan downloadResult) (string, string, error) {
	select {
	case r := <-done:
		return r.path, r.mime, r.err
	case <-ctx.Done():
		return "", "", ctx.Err()
	}
}

// buildDownloadRequest constructs the HTTP request with the effective user agent.
func buildDownloadRequest(ctx context.Context, requestCtx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("User-Agent", effectiveUserAgent(ctx))
	return req, nil
}

// effectiveUserAgent resolves the user-agent header for outbound requests.
func effectiveUserAgent(ctx context.Context) string {
	ua := HTTPUserAgent
	if cfg := appconfig.FromContext(ctx); cfg != nil && cfg.Attachments.HTTPUserAgent != "" {
		return cfg.Attachments.HTTPUserAgent
	}
	return ua
}

// validateHTTPResponse ensures the response status and length are acceptable.
func validateHTTPResponse(resp *http.Response) error {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	if resp.ContentLength == 0 {
		return fmt.Errorf("empty response from server")
	}
	return nil
}

// ensureContentWritten confirms bytes were written and cleans up zero-length files.
func ensureContentWritten(path string, written int64) error {
	if written == 0 {
		_ = os.Remove(path)
		return fmt.Errorf("empty response from server")
	}
	return nil
}

// logDownloadSuccess records download metadata for diagnostics.
func logDownloadSuccess(ctx context.Context, url string, written int64, mime string, redirects *int) {
	logger.FromContext(ctx).Debug(
		"Downloaded attachment",
		"url", sanitizeURL(url),
		"bytes", written,
		"mime", mime,
		"redirects", *redirects,
	)
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
		need := min(h.limit-len(h.b), len(p))
		h.b = append(h.b, p[:need]...)
	}
	return len(p), nil
}

func (h *headCapture) Bytes() []byte { return h.b }

// sanitizeURL redacts sensitive information from URLs for logging purposes
func sanitizeURL(url string) string {
	return core.RedactString(url)
}
