package store

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// LocalDay formats one instant as the daemon-local calendar day bucket used by
// the daily usage rollup and the SQLite 'localtime' overview aggregates.
func LocalDay(value time.Time) string {
	return value.In(time.Local).Format(time.DateOnly)
}

// LocalDayStart returns daemon-local midnight daysBack days before value's day.
func LocalDayStart(value time.Time, daysBack int) time.Time {
	local := value.In(time.Local)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
	return start.AddDate(0, 0, -daysBack)
}

// TokenUsageDailyUpdate merges one turn of token usage into the daily rollup.
type TokenUsageDailyUpdate struct {
	Day          string
	WorkspaceID  string
	AgentName    string
	InputTokens  *int64
	OutputTokens *int64
	TotalTokens  *int64
	CostAmount   *float64
	CostCurrency *string
	CostStatus   string
	CostSource   string
	Turns        int64
	UpdatedAt    time.Time
}

// Validate ensures the rollup update carries a canonical day bucket, non-negative
// token deltas, and a valid cost shape.
func (u TokenUsageDailyUpdate) Validate() error {
	if _, err := time.Parse(time.DateOnly, strings.TrimSpace(u.Day)); err != nil {
		return fmt.Errorf("store: token usage day must be a YYYY-MM-DD bucket: %w", err)
	}
	for _, delta := range []*int64{u.InputTokens, u.OutputTokens, u.TotalTokens} {
		if delta != nil && *delta < 0 {
			return errors.New("store: token usage deltas must be non-negative")
		}
	}
	return validateTokenCostShape(u.CostStatus, u.CostSource, u.CostAmount, u.CostCurrency)
}

// OverviewSinceQuery bounds one overview aggregate by timestamp and optional workspace.
type OverviewSinceQuery struct {
	WorkspaceID string
	Since       time.Time
}

// Validate ensures the aggregate window carries an explicit lower bound.
func (q OverviewSinceQuery) Validate() error {
	if q.Since.IsZero() {
		return errors.New("store: overview since bound is required")
	}
	return nil
}

// OverviewDayQuery bounds one daily-rollup aggregate by day bucket and optional workspace.
type OverviewDayQuery struct {
	WorkspaceID string
	SinceDay    string
}

// Validate ensures the rollup window starts on a canonical day bucket.
func (q OverviewDayQuery) Validate() error {
	if _, err := time.Parse(time.DateOnly, strings.TrimSpace(q.SinceDay)); err != nil {
		return fmt.Errorf("store: overview since day must be a YYYY-MM-DD bucket: %w", err)
	}
	return nil
}

// TokenUsageDay is one daemon-local day of summed token usage.
type TokenUsageDay struct {
	Day          string
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}

// TokenUsageAgentTotal is one agent's summed token usage inside a rollup window.
type TokenUsageAgentTotal struct {
	AgentName   string
	TotalTokens int64
}

// TokenUsageCostGroup is one provenance-uniform cost bucket inside a rollup window.
type TokenUsageCostGroup struct {
	CostStatus      string
	CostSource      string
	CostCurrency    string
	TotalCost       float64
	RowsWithoutCost int64
	RowsTotal       int64
}

// TaskRunOutcomeDay is one daemon-local day of terminal worker-run outcomes.
type TaskRunOutcomeDay struct {
	Day       string
	Completed int
	Failed    int
	Canceled  int
}

// TaskClosedDay is one daemon-local day of completed-task closures.
type TaskClosedDay struct {
	Day    string
	Closed int
}

// EventHourWeekdayBucket is one hour-by-weekday event count; weekday 0 is Sunday.
type EventHourWeekdayBucket struct {
	Weekday int
	Hour    int
	Events  int
}

// HookDispatchCounts summarizes hook dispatch completions inside a window.
type HookDispatchCounts struct {
	Runs     int
	Failures int
}

// LongestSessionSample is the longest-running user session inside a window.
type LongestSessionSample struct {
	SessionID      string
	AgentName      string
	StartedAt      time.Time
	RuntimeSeconds int64
}
