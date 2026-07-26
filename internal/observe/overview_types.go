package observe

import (
	"fmt"
	"time"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	// OverviewAttentionKindApproval marks items waiting for an approval decision.
	OverviewAttentionKindApproval = "approval"
	// OverviewAttentionKindNeedsInput marks items escalated for operator attention.
	OverviewAttentionKindNeedsInput = "needs_input"
	// OverviewAttentionKindFailure marks items whose latest execution failed.
	OverviewAttentionKindFailure = "failure"

	// OverviewActionApprove approves an approval-gated task.
	OverviewActionApprove = "approve"
	// OverviewActionReject rejects an approval-gated task.
	OverviewActionReject = "reject"
	// OverviewActionRetry retries the failed run behind an attention item.
	OverviewActionRetry = "retry"
	// OverviewActionOpen opens the attention item's owning entity.
	OverviewActionOpen = "open"
)

const (
	overviewOutcomeWindowDays = 14
	overviewPulseWindowDays   = 14
	overviewShareWindowDays   = 30
	overviewShareMaxAgents    = 4
	overviewAttentionScan     = 50
	overviewAttentionItemCap  = 12
	overviewDefaultUsageDays  = 30
)

// OverviewQuery bounds one home overview read.
type OverviewQuery struct {
	WorkspaceID     string
	TaskScope       taskpkg.CatalogScope
	UsageWindowDays int
	Actor           taskpkg.ActorIdentity
}

// Validate normalizes the usage window and rejects unsupported values.
func (q *OverviewQuery) Validate() error {
	if q.UsageWindowDays == 0 {
		q.UsageWindowDays = overviewDefaultUsageDays
	}
	return ValidateUsageWindowDays(q.UsageWindowDays)
}

// ValidateUsageWindowDays reports whether days is an accepted overview usage
// window. It is the single allowlist shared by the daemon, API, and CLI.
func ValidateUsageWindowDays(days int) error {
	switch days {
	case 7, 30, 90:
		return nil
	default:
		return fmt.Errorf("observe: unsupported usage window %d (expected 7, 30, or 90)", days)
	}
}

// OverviewView is the composed home overview read model.
type OverviewView struct {
	GeneratedAt time.Time
	Attention   OverviewAttention
	Today       OverviewToday
	Outcomes    OverviewOutcomes
	Usage       OverviewUsage
	Pulse       OverviewPulse
	Network     OverviewNetwork
	System      OverviewSystem
	Freshness   TaskDashboardFreshness
}

// OverviewAttention lists everything currently waiting on the user.
type OverviewAttention struct {
	Total  int
	ByKind map[string]int
	Items  []OverviewAttentionItem
}

// OverviewAttentionItem is one actionable attention row.
type OverviewAttentionItem struct {
	Kind       string
	Title      string
	Detail     string
	TaskID     string
	RunID      string
	SessionID  string
	OccurredAt time.Time
	Actions    []string
}

// OverviewToday summarizes today's terminal work.
type OverviewToday struct {
	RunsCompleted int
	RunsFailed    int
	TasksClosed   int
}

// OverviewOutcomes summarizes terminal run outcomes across the fixed window.
type OverviewOutcomes struct {
	WindowDays int
	Days       []store.TaskRunOutcomeDay
	Completed  int
	Failed     int
	Canceled   int
	SuccessPct float64
}

// OverviewUsage summarizes token usage and estimated cost for the selected window.
type OverviewUsage struct {
	WindowDays    int
	RetentionDays int
	Truncated     bool
	TotalTokens   int64
	EstimatedCost *float64
	CostCurrency  string
	CostStatus    string
	Days          []store.TokenUsageDay
	AgentShare    []OverviewAgentShare
}

// OverviewAgentShare is one agent's fraction of window token usage.
type OverviewAgentShare struct {
	AgentName string
	Tokens    int64
	Fraction  float64
}

// OverviewPulse summarizes hour-by-weekday activity plus derived insights.
type OverviewPulse struct {
	WindowDays     int
	Buckets        []store.EventHourWeekdayBucket
	Busiest        *store.EventHourWeekdayBucket
	LongestSession *OverviewLongestSession
}

// OverviewLongestSession is the longest user session inside the pulse window.
type OverviewLongestSession struct {
	SessionID       string
	AgentName       string
	DurationSeconds int64
	Date            string
}

// OverviewNetwork carries today-windowed network counters.
type OverviewNetwork struct {
	MessagesToday int
}

// OverviewSystem carries today-windowed system counters plus retention truth.
type OverviewSystem struct {
	HookRunsToday     int
	HookFailuresToday int
	RetentionDays     int
}
