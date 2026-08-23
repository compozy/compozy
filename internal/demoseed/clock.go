package demoseed

import (
	"time"

	"github.com/compozy/compozy/internal/store"
)

// timeline anchors every fixture to one truncated instant so a reseed stays reproducible.
type timeline struct {
	now time.Time
}

func newTimeline(now time.Time) timeline {
	return timeline{now: now.UTC().Truncate(time.Second)}
}

func (t timeline) Now() time.Time { return t.now }

func (t timeline) minutesAgo(minutes int) time.Time {
	return t.now.Add(-time.Duration(minutes) * time.Minute)
}

func (t timeline) hoursAgo(hours int) time.Time {
	return t.now.Add(-time.Duration(hours) * time.Hour)
}

func (t timeline) daysAgo(days int) time.Time {
	return t.now.AddDate(0, 0, -days)
}

func (t timeline) hoursMinutesAgo(hours int, minutes int) time.Time {
	return t.now.Add(-time.Duration(hours)*time.Hour - time.Duration(minutes)*time.Minute)
}

func (t timeline) daysHoursAgo(days int, hours int) time.Time {
	return t.now.AddDate(0, 0, -days).Add(-time.Duration(hours) * time.Hour)
}

// dayStart is the local midnight the observe read model buckets "today" against.
func (t timeline) dayStart(daysBack int) time.Time {
	return store.LocalDayStart(t.now, daysBack)
}

// todayAt places an event inside the current local day without ever landing in the future.
func (t timeline) todayAt(offset time.Duration) time.Time {
	candidate := t.dayStart(0).Add(offset).UTC()
	if candidate.After(t.now) {
		return t.now.Add(-time.Minute)
	}
	if candidate.Before(t.dayStart(0).UTC()) {
		return t.dayStart(0).UTC()
	}
	return candidate
}

// dayKey formats the local-day partition key used by the token usage rollup.
func (t timeline) dayKey(daysBack int) string {
	return t.dayStart(daysBack).Format(time.DateOnly)
}
