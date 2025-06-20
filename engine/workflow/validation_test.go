package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSchedule(t *testing.T) {
	t.Run("Should validate valid cron expressions", func(t *testing.T) {
		validCrons := []string{
			"0 0 9 * * 1-5",  // 9 AM weekdays
			"0 0 * * * *",    // Every hour
			"0 0 2 * * 0",    // 2 AM every Sunday
			"0 */5 * * * *",  // Every 5 minutes
			"0 0 0 1 * *",    // First day of month
			"0 30 3 15 * *",  // 3:30 AM on 15th of month
			"*/30 * * * * *", // Every 30 seconds
		}
		for _, cron := range validCrons {
			schedule := &Schedule{Cron: cron}
			err := ValidateSchedule(schedule)
			assert.NoError(t, err, "Expected cron '%s' to be valid", cron)
		}
	})
	t.Run("Should reject invalid cron expressions", func(t *testing.T) {
		invalidCrons := []string{
			"invalid",
			"* * * * *",    // Missing seconds field
			"60 * * * * *", // Invalid second
			"* 60 * * * *", // Invalid minute
			"* * 25 * * *", // Invalid hour
			"* * * 32 * *", // Invalid day
			"* * * * 13 *", // Invalid month
			"* * * * * 8",  // Invalid weekday
		}
		for _, cron := range invalidCrons {
			schedule := &Schedule{Cron: cron}
			err := ValidateSchedule(schedule)
			assert.Error(t, err, "Expected cron '%s' to be invalid", cron)
			if err != nil {
				assert.Contains(t, err.Error(), "invalid cron expression")
			}
		}
	})
	t.Run("Should validate valid timezones", func(t *testing.T) {
		validTimezones := []string{
			"America/New_York",
			"Europe/London",
			"Asia/Tokyo",
			"UTC",
			"US/Pacific",
			"EST",
			"", // Empty should be valid (defaults to UTC)
		}
		for _, tz := range validTimezones {
			schedule := &Schedule{
				Cron:     "0 0 9 * * *",
				Timezone: tz,
			}
			err := ValidateSchedule(schedule)
			assert.NoError(t, err, "Expected timezone '%s' to be valid", tz)
		}
	})
	t.Run("Should reject invalid timezones", func(t *testing.T) {
		invalidTimezones := []string{
			"Invalid/Timezone",
			"America/InvalidCity",
			"NotATimezone",
			"GMT+25", // Out of range
		}
		for _, tz := range invalidTimezones {
			schedule := &Schedule{
				Cron:     "0 0 9 * * *",
				Timezone: tz,
			}
			err := ValidateSchedule(schedule)
			assert.Error(t, err, "Expected timezone '%s' to be invalid", tz)
			if err != nil {
				assert.Contains(t, err.Error(), "invalid timezone")
			}
		}
	})
	t.Run("Should validate overlap policies", func(t *testing.T) {
		validPolicies := []OverlapPolicy{
			"", // Empty should be valid (defaults to skip)
			OverlapSkip,
			OverlapAllow,
			OverlapBufferOne,
			OverlapCancelOther,
		}
		for _, policy := range validPolicies {
			schedule := &Schedule{
				Cron:          "0 0 9 * * *",
				OverlapPolicy: policy,
			}
			err := ValidateSchedule(schedule)
			assert.NoError(t, err, "Expected overlap policy '%s' to be valid", policy)
		}
	})
	t.Run("Should reject invalid overlap policies", func(t *testing.T) {
		invalidPolicies := []OverlapPolicy{
			"invalid_policy",
			"buffer_two",
			"terminate",
		}
		for _, policy := range invalidPolicies {
			schedule := &Schedule{
				Cron:          "0 0 9 * * *",
				OverlapPolicy: policy,
			}
			err := ValidateSchedule(schedule)
			assert.Error(t, err, "Expected overlap policy '%s' to be invalid", policy)
			if err != nil {
				assert.Contains(t, err.Error(), "unsupported overlap_policy")
			}
		}
	})
	t.Run("Should validate jitter duration format", func(t *testing.T) {
		validJitters := []string{
			"5m",
			"30s",
			"1h",
			"1h30m",
			"100ms",
			"", // Empty should be valid
		}
		for _, jitter := range validJitters {
			schedule := &Schedule{
				Cron:   "0 0 9 * * *",
				Jitter: jitter,
			}
			err := ValidateSchedule(schedule)
			assert.NoError(t, err, "Expected jitter '%s' to be valid", jitter)
		}
	})
	t.Run("Should reject invalid jitter duration format", func(t *testing.T) {
		invalidJitters := []string{
			"5 minutes",
			"invalid",
			"5x",
			"m30",
		}
		for _, jitter := range invalidJitters {
			schedule := &Schedule{
				Cron:   "0 0 9 * * *",
				Jitter: jitter,
			}
			err := ValidateSchedule(schedule)
			assert.Error(t, err, "Expected jitter '%s' to be invalid", jitter)
			if err != nil {
				assert.Contains(t, err.Error(), "invalid jitter duration")
			}
		}
	})
	t.Run("Should validate start and end times", func(t *testing.T) {
		now := time.Now()
		tomorrow := now.Add(24 * time.Hour)
		yesterday := now.Add(-24 * time.Hour)
		// Valid: start before end
		schedule := &Schedule{
			Cron:    "0 0 9 * * *",
			StartAt: &now,
			EndAt:   &tomorrow,
		}
		err := ValidateSchedule(schedule)
		assert.NoError(t, err)
		// Valid: only start time
		schedule = &Schedule{
			Cron:    "0 0 9 * * *",
			StartAt: &now,
		}
		err = ValidateSchedule(schedule)
		assert.NoError(t, err)
		// Valid: only end time
		schedule = &Schedule{
			Cron:  "0 0 9 * * *",
			EndAt: &tomorrow,
		}
		err = ValidateSchedule(schedule)
		assert.NoError(t, err)
		// Invalid: start after end
		schedule = &Schedule{
			Cron:    "0 0 9 * * *",
			StartAt: &tomorrow,
			EndAt:   &yesterday,
		}
		err = ValidateSchedule(schedule)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "start_at must be before end_at")
	})
	t.Run("Should handle DST transitions in timezones", func(t *testing.T) {
		// Test with a timezone that has DST
		schedule := &Schedule{
			Cron:     "0 0 2 * * *", // 2 AM - problematic during spring DST transition
			Timezone: "America/New_York",
		}
		err := ValidateSchedule(schedule)
		// Should still be valid - handling DST is Temporal's responsibility
		assert.NoError(t, err)
	})
	t.Run("Should validate complete schedule configuration", func(t *testing.T) {
		enabled := true
		now := time.Now()
		future := now.Add(30 * 24 * time.Hour)
		schedule := &Schedule{
			Cron:          "0 0 9 * * 1-5",
			Timezone:      "America/New_York",
			Enabled:       &enabled,
			Jitter:        "5m",
			OverlapPolicy: OverlapSkip,
			StartAt:       &now,
			EndAt:         &future,
			Input: map[string]any{
				"report_type": "daily",
				"recipients":  []string{"team@company.com"},
			},
		}
		err := ValidateSchedule(schedule)
		require.NoError(t, err)
	})
}
