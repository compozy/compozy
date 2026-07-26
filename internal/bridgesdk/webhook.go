package bridgesdk

import (
	"context"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultWebhookBodyLimit int64 = 1 << 20

// SignatureVerifier validates the raw webhook request body before provider mapping runs.
type SignatureVerifier func(context.Context, *http.Request, []byte) error

// WebhookHandler receives the guarded webhook request after the shared ingress
// checks have passed.
type WebhookHandler func(http.ResponseWriter, *http.Request, WebhookRequest) error

// WebhookRequest is the provider-facing request payload after ingress guards complete.
type WebhookRequest struct {
	Body       []byte
	ReceivedAt time.Time
}

// WebhookGuardConfig configures the shared ingress hardening pipeline.
type WebhookGuardConfig struct {
	AllowedMethods      []string
	AllowedContentTypes []string
	MaxBodyBytes        int64
	RateLimiter         *FixedWindowRateLimiter
	InFlightLimiter     *InFlightLimiter
	VerifySignature     SignatureVerifier
	RequestKey          func(*http.Request) string
	Now                 func() time.Time
}

// FixedWindowRateLimiter applies a simple fixed-window request limit per key.
type FixedWindowRateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	now       func() time.Time
	lastSweep time.Time
	counts    map[string]fixedWindowCounter
}

type fixedWindowCounter struct {
	windowStart time.Time
	count       int
}

// InFlightLimiter bounds concurrent webhook requests.
type InFlightLimiter struct {
	sem chan struct{}
}

// NewFixedWindowRateLimiter constructs a new fixed-window limiter.
func NewFixedWindowRateLimiter(limit int, window time.Duration) *FixedWindowRateLimiter {
	if limit <= 0 || window <= 0 {
		return nil
	}
	return &FixedWindowRateLimiter{
		limit:  limit,
		window: window,
		now: func() time.Time {
			return time.Now().UTC()
		},
		counts: make(map[string]fixedWindowCounter),
	}
}

// Allow reports whether another request may proceed for the key.
func (l *FixedWindowRateLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}

	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		trimmedKey = "global"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if l.lastSweep.IsZero() || now.Sub(l.lastSweep) >= l.window {
		l.evictExpiredLocked(now)
		l.lastSweep = now
	}
	entry := l.counts[trimmedKey]
	if entry.windowStart.IsZero() || now.Sub(entry.windowStart) >= l.window {
		entry = fixedWindowCounter{
			windowStart: now,
			count:       1,
		}
		l.counts[trimmedKey] = entry
		return true
	}
	if entry.count >= l.limit {
		return false
	}

	entry.count++
	l.counts[trimmedKey] = entry
	return true
}

func (l *FixedWindowRateLimiter) evictExpiredLocked(now time.Time) {
	cutoff := now.Add(-l.window)
	for key, entry := range l.counts {
		if entry.windowStart.After(cutoff) || entry.windowStart.Equal(cutoff) {
			continue
		}
		delete(l.counts, key)
	}
}

// NewInFlightLimiter constructs a new in-flight semaphore.
func NewInFlightLimiter(limit int) *InFlightLimiter {
	if limit <= 0 {
		return nil
	}
	return &InFlightLimiter{
		sem: make(chan struct{}, limit),
	}
}

// TryAcquire attempts to reserve one in-flight slot.
func (l *InFlightLimiter) TryAcquire() bool {
	if l == nil {
		return true
	}
	select {
	case l.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release frees one in-flight slot.
func (l *InFlightLimiter) Release() {
	if l == nil {
		return
	}
	select {
	case <-l.sem:
	default:
	}
}

// NewWebhookHandler constructs the shared ingress-hardening handler.
func NewWebhookHandler(config WebhookGuardConfig, next WebhookHandler) (http.Handler, error) {
	if next == nil {
		return nil, errors.New("bridgesdk: webhook handler is required")
	}
	if config.Now == nil {
		config.Now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = defaultWebhookBodyLimit
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedMethod(config.AllowedMethods, r.Method) {
			if len(config.AllowedMethods) > 0 {
				w.Header().Set("Allow", strings.Join(config.AllowedMethods, ", "))
			}
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		if !allowedContentType(config.AllowedContentTypes, r.Header.Get("Content-Type")) {
			http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
			return
		}

		key := "global"
		if config.RequestKey != nil {
			key = config.RequestKey(r)
		} else if host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil && host != "" {
			key = host
		}
		if config.RateLimiter != nil && !config.RateLimiter.Allow(key) {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		if config.InFlightLimiter != nil && !config.InFlightLimiter.TryAcquire() {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		if config.InFlightLimiter != nil {
			defer config.InFlightLimiter.Release()
		}

		body, err := readBodyWithLimit(w, r, config.MaxBodyBytes)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}
		if config.VerifySignature != nil {
			if err := config.VerifySignature(r.Context(), r, body); err != nil {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
		}

		if err := next(w, r, WebhookRequest{
			Body:       body,
			ReceivedAt: config.Now(),
		}); err != nil {
			writeWebhookError(w, err)
			return
		}
	}), nil
}

func allowedMethod(allowed []string, method string) bool {
	if len(allowed) == 0 {
		return true
	}
	trimmedMethod := strings.TrimSpace(method)
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(candidate), trimmedMethod) {
			return true
		}
	}
	return false
}

func allowedContentType(allowed []string, contentType string) bool {
	if len(allowed) == 0 {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return false
	}
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(candidate), mediaType) {
			return true
		}
	}
	return false
}

func readBodyWithLimit(w http.ResponseWriter, r *http.Request, maxBytes int64) ([]byte, error) {
	bodyReader := http.MaxBytesReader(w, r.Body, maxBytes)
	body, err := io.ReadAll(bodyReader)
	if closeErr := bodyReader.Close(); closeErr != nil {
		if err != nil {
			err = errors.Join(err, closeErr)
		}
	}
	return body, err
}

func writeWebhookError(w http.ResponseWriter, err error) {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode > 0 {
		status := httpErr.StatusCode
		if httpErr.RetryAfter > 0 {
			retryAfterSeconds := int64((httpErr.RetryAfter + time.Second - 1) / time.Second)
			w.Header().Set("Retry-After", strconv.FormatInt(retryAfterSeconds, 10))
		}
		http.Error(w, httpErr.Error(), status)
		return
	}
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
