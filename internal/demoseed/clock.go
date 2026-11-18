package demoseed

import (
	"sync/atomic"
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

// writeClock is the instant the store stamps rows with. Stories that go through store
// paths owning their own timestamps move it before each write, then reset it to now.
type writeClock struct {
	now     time.Time
	current atomic.Int64
}

func newWriteClock(now time.Time) *writeClock {
	clock := &writeClock{now: now}
	clock.current.Store(now.UnixNano())
	return clock
}

func (c *writeClock) Now() time.Time { return time.Unix(0, c.current.Load()).UTC() }

// at stamps the following writes at instant.
func (c *writeClock) at(instant time.Time) { c.current.Store(instant.UnixNano()) }

// reset returns the store clock to the scenario's now.
func (c *writeClock) reset() { c.current.Store(c.now.UnixNano()) }
