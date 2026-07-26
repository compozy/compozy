package memory

import (
	"fmt"
	"time"
)

func ageDays(modTime time.Time, now time.Time) int {
	days := calendarDayNumber(now.In(modTime.Location())) - calendarDayNumber(modTime.In(modTime.Location()))
	if days < 0 {
		return 0
	}

	return days
}

func ageText(modTime time.Time, now time.Time) string {
	switch age := ageDays(modTime, now); age {
	case 0:
		return "today"
	case 1:
		return "yesterday"
	default:
		return fmt.Sprintf("%d days ago", age)
	}
}

func freshnessWarning(modTime time.Time, now time.Time) string {
	age := ageDays(modTime, now)
	if age <= 1 {
		return ""
	}

	return fmt.Sprintf("This memory is %d days old. Verify against current state before asserting as fact.", age)
}

// FreshnessWarning reports whether a memory should be treated as stale.
func FreshnessWarning(modTime time.Time, now time.Time) string {
	return freshnessWarning(modTime, now)
}

func calendarDayNumber(value time.Time) int {
	year, month, day := value.Date()
	return int(time.Date(year, month, day, 12, 0, 0, 0, time.UTC).Unix() / int64(24*time.Hour/time.Second))
}
