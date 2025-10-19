package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestInitMetrics(t *testing.T) {
	t.Run("Should initialize auth metrics successfully", func(t *testing.T) {
		ResetMetricsForTesting()

		meter := noop.NewMeterProvider().Meter("test")
		err := InitMetrics(meter)

		assert.NoError(t, err)
		assert.NotNil(t, authAttemptsTotal)
		assert.NotNil(t, authLatencySeconds)
		assert.NotNil(t, authTokenAgeSeconds)
		assert.NotNil(t, authRateLimitHits)
	})

	t.Run("Should handle nil meter gracefully", func(t *testing.T) {
		ResetMetricsForTesting()

		err := InitMetrics(nil)

		assert.NoError(t, err)
	})

	t.Run("Should initialize metrics only once", func(t *testing.T) {
		ResetMetricsForTesting()

		meter := noop.NewMeterProvider().Meter("test")

		err1 := InitMetrics(meter)
		err2 := InitMetrics(meter)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NotNil(t, authAttemptsTotal)
		assert.NotNil(t, authLatencySeconds)
		assert.NotNil(t, authTokenAgeSeconds)
		assert.NotNil(t, authRateLimitHits)
	})
}

func TestRecordAuthAttempt(t *testing.T) {
	t.Run("Should handle metrics recording with initialized counters", func(t *testing.T) {
		ResetMetricsForTesting()

		meter := noop.NewMeterProvider().Meter("test")
		err := InitMetrics(meter)
		assert.NoError(t, err)

		ctx := t.Context()
		duration := 5 * time.Millisecond

		RecordAuthAttempt(ctx, AuthOutcomeSuccess, ReasonNone, AuthMethodAPIKey, duration)
		RecordAuthAttempt(ctx, AuthOutcomeFailure, ReasonInvalidCredentials, AuthMethodAPIKey, duration)
	})

	t.Run("Should handle metrics recording when counters are nil", func(t *testing.T) {
		ResetMetricsForTesting()

		ctx := t.Context()
		duration := 1 * time.Millisecond

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("RecordAuthAttempt should not panic when metrics are nil, got panic: %v", r)
			}
		}()
		RecordAuthAttempt(ctx, AuthOutcomeFailure, ReasonUnknown, AuthMethodUnknown, duration)
	})
}

func TestRecordTokenAge(t *testing.T) {
	t.Run("Should record token age when initialized", func(t *testing.T) {
		ResetMetricsForTesting()
		meter := noop.NewMeterProvider().Meter("test")
		assert.NoError(t, InitMetrics(meter))

		ctx := t.Context()
		RecordTokenAge(ctx, time.Now().Add(-30*time.Minute), AuthMethodJWT)
	})

	t.Run("Should ignore future or zero timestamps", func(_ *testing.T) {
		ResetMetricsForTesting()
		ctx := t.Context()
		RecordTokenAge(ctx, time.Time{}, AuthMethodJWT)
		RecordTokenAge(ctx, time.Now().Add(5*time.Minute), AuthMethodJWT)
	})
}

func TestRecordRateLimitHit(t *testing.T) {
	t.Run("Should record rate limit hits with user metadata", func(t *testing.T) {
		ResetMetricsForTesting()
		meter := noop.NewMeterProvider().Meter("test")
		assert.NoError(t, InitMetrics(meter))

		ctx := t.Context()
		RecordRateLimitHit(ctx, "user-123", "192.168.1.42")
	})

	t.Run("Should mask invalid or empty IP addresses", func(t *testing.T) {
		assert.Equal(t, "unknown", maskIPAddress(""))
		assert.Equal(t, "unknown", maskIPAddress("not-an-ip"))
		assert.Equal(t, "192.168.1.0", maskIPAddress("192.168.1.42"))
		assert.Equal(t, "2001:db8:85a3::", maskIPAddress("2001:db8:85a3::8a2e:370:7334"))
		assert.Equal(t, "127.0.0.0", maskIPAddress("127.0.0.1:8080"))
		assert.Equal(t, "2001:db8::", maskIPAddress("[2001:db8::1]:443"))
	})
}
