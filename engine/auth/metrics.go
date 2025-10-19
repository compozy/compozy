package auth

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	// Global metrics for auth instrumentation
	authAttemptsTotal   metric.Int64Counter
	authLatencySeconds  metric.Float64Histogram
	authTokenAgeSeconds metric.Float64Histogram
	authRateLimitHits   metric.Int64Counter
	metricsOnce         sync.Once
	metricsMutex        sync.Mutex
)

// AuthOutcome represents the result of an authentication attempt.
type AuthOutcome string //nolint:revive // exported for clarity in metrics consumers

const (
	// AuthOutcomeSuccess denotes a successful authentication.
	AuthOutcomeSuccess AuthOutcome = "success"
	// AuthOutcomeFailure denotes a failed authentication.
	AuthOutcomeFailure AuthOutcome = "failure"
)

// AuthFailureReason categorizes why authentication failed.
type AuthFailureReason string //nolint:revive // exported for clarity in metrics consumers

const (
	// ReasonNone is used when authentication succeeded.
	ReasonNone AuthFailureReason = "none"
	// ReasonInvalidCredentials indicates bad credentials or signatures.
	ReasonInvalidCredentials AuthFailureReason = "invalid_credentials" // #nosec G101 -- enum label, not a credential
	// ReasonExpiredToken indicates an expired token was provided.
	ReasonExpiredToken AuthFailureReason = "expired_token"
	// ReasonMissingAuth indicates the request lacked authentication data.
	ReasonMissingAuth AuthFailureReason = "missing_auth"
	// ReasonInvalidFormat indicates the authentication data was malformed.
	ReasonInvalidFormat AuthFailureReason = "invalid_format"
	// ReasonRateLimited indicates authentication failed due to rate limiting.
	ReasonRateLimited AuthFailureReason = "rate_limited"
	// ReasonUnknown is used when the failure reason cannot be determined.
	ReasonUnknown AuthFailureReason = "unknown"
)

// AuthMethod represents the authentication method in use.
type AuthMethod string //nolint:revive // exported for clarity in metrics consumers

const (
	// AuthMethodAPIKey represents API key authentication.
	AuthMethodAPIKey AuthMethod = "api_key"
	// AuthMethodJWT represents JWT based authentication.
	AuthMethodJWT AuthMethod = "jwt"
	// AuthMethodOAuth represents OAuth based authentication.
	AuthMethodOAuth AuthMethod = "oauth"
	// AuthMethodUnknown is used when the method cannot be determined.
	AuthMethodUnknown AuthMethod = "unknown"
)

const maskedIPUnknownValue = "unknown"

var (
	authLatencyBuckets  = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	authTokenAgeBuckets = []float64{60, 300, 900, 1800, 3600, 7200, 14400, 28800, 86400}
)

// InitMetrics initializes auth metrics.
func InitMetrics(meter metric.Meter) error {
	if meter == nil {
		return nil
	}

	var initErr error
	metricsOnce.Do(func() {
		metricsMutex.Lock()
		defer metricsMutex.Unlock()

		authAttemptsTotal, initErr = meter.Int64Counter(
			metrics.MetricNameWithSubsystem("auth", "attempts_total"),
			metric.WithDescription("Total authentication attempts categorized by outcome and reason"),
			metric.WithUnit("1"),
		)
		if initErr != nil {
			return
		}

		authLatencySeconds, initErr = meter.Float64Histogram(
			metrics.MetricNameWithSubsystem("auth", "latency_seconds"),
			metric.WithDescription("Authentication latency by method and outcome"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(authLatencyBuckets...),
		)
		if initErr != nil {
			return
		}

		authTokenAgeSeconds, initErr = meter.Float64Histogram(
			metrics.MetricNameWithSubsystem("auth", "token_age_seconds"),
			metric.WithDescription("Age of tokens used for authentication"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(authTokenAgeBuckets...),
		)
		if initErr != nil {
			return
		}

		authRateLimitHits, initErr = meter.Int64Counter(
			metrics.MetricNameWithSubsystem("auth", "rate_limit_hits_total"),
			metric.WithDescription("Number of times authentication rate limiting was triggered"),
			metric.WithUnit("1"),
		)
	})

	return initErr
}

// ResetMetricsForTesting resets the metrics initialization state for testing.
// This function should only be used in tests to ensure clean state between test runs.
func ResetMetricsForTesting() {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()
	metricsOnce = sync.Once{}
	authAttemptsTotal = nil
	authLatencySeconds = nil
	authTokenAgeSeconds = nil
	authRateLimitHits = nil
}

// RecordAuthAttempt records an authentication attempt with outcome, reason, and method.
func RecordAuthAttempt(
	ctx context.Context,
	outcome AuthOutcome,
	reason AuthFailureReason,
	method AuthMethod,
	duration time.Duration,
) {
	if authAttemptsTotal != nil {
		authAttemptsTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("outcome", string(outcome)),
				attribute.String("reason", string(reason)),
				attribute.String("method", string(normalizeMethod(method))),
			),
		)
	}

	if authLatencySeconds != nil && duration > 0 {
		authLatencySeconds.Record(ctx, duration.Seconds(),
			metric.WithAttributes(
				attribute.String("outcome", string(outcome)),
				attribute.String("method", string(normalizeMethod(method))),
			),
		)
	}
}

// RecordTokenAge records the age of a token used for authentication.
func RecordTokenAge(ctx context.Context, issuedAt time.Time, method AuthMethod) {
	if authTokenAgeSeconds == nil || issuedAt.IsZero() {
		return
	}

	age := time.Since(issuedAt)
	if age <= 0 {
		return
	}

	authTokenAgeSeconds.Record(ctx, age.Seconds(),
		metric.WithAttributes(
			attribute.String("method", string(normalizeMethod(method))),
		),
	)
}

// RecordRateLimitHit records a rate limit hit with optional user and IP metadata.
func RecordRateLimitHit(ctx context.Context, userID string, ipAddress string) {
	if authRateLimitHits == nil {
		return
	}

	maskedIP := maskIPAddress(ipAddress)
	attrs := []attribute.KeyValue{
		attribute.String("ip_address", maskedIP),
		attribute.Bool("has_user", userID != ""),
	}

	authRateLimitHits.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func normalizeMethod(method AuthMethod) AuthMethod {
	if method == "" {
		return AuthMethodUnknown
	}
	return method
}

// maskIPAddress masks the last octet for IPv4 and applies a /64 mask for IPv6 addresses.
// It handles host:port and bracketed IPv6 inputs by normalizing them before masking.
func maskIPAddress(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return maskedIPUnknownValue
	}

	// Handle host:port and bracketed IPv6 forms.
	// Try parsing with SplitHostPort first (handles both "ip:port" and "[ipv6]:port")
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	} else if strings.HasPrefix(ip, "[") && strings.HasSuffix(ip, "]") {
		// Handle plain bracketed IPv6 without port
		ip = strings.Trim(ip, "[]")
	}
	// Remove IPv6 zone-id if present (e.g., fe80::1%eth0)
	if i := strings.IndexByte(ip, '%'); i != -1 {
		ip = ip[:i]
	}

	ip = strings.TrimSpace(ip)
	if ip == "" {
		return maskedIPUnknownValue
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return maskedIPUnknownValue
	}

	if v4 := parsed.To4(); v4 != nil {
		masked := make(net.IP, len(v4))
		copy(masked, v4)
		masked[3] = 0
		return masked.String()
	}

	masked := parsed.Mask(net.CIDRMask(64, net.IPv6len*8))
	return masked.String()
}
