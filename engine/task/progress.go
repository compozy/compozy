package task

import (
	"time"

	"github.com/compozy/compozy/engine/core"
)

// -----------------------------------------------------------------------------
// Progress Aggregation Types
// -----------------------------------------------------------------------------

// ProgressInfo represents aggregated progress information for a parent task
type ProgressInfo struct {
	TotalChildren  int                     `json:"total_children"`
	SuccessCount   int                     `json:"success_count"` // Renamed from CompletedCount for clarity
	FailedCount    int                     `json:"failed_count"`
	CanceledCount  int                     `json:"canceled_count"`  // New field for canceled tasks
	TimedOutCount  int                     `json:"timed_out_count"` // New field for timed out tasks
	TerminalCount  int                     `json:"terminal_count"`  // New field: success + failed + canceled + timed_out
	RunningCount   int                     `json:"running_count"`
	PendingCount   int                     `json:"pending_count"`
	StatusCounts   map[core.StatusType]int `json:"status_counts"`
	CompletionRate float64                 `json:"completion_rate"`
	FailureRate    float64                 `json:"failure_rate"`
	OverallStatus  core.StatusType         `json:"overall_status"`
}

// CalculateOverallStatus determines the parent task status based on strategy and child statuses
func (p *ProgressInfo) CalculateOverallStatus(strategy ParallelStrategy) core.StatusType {
	if p.TotalChildren == 0 {
		p.OverallStatus = core.StatusPending
		return core.StatusPending
	}
	var status core.StatusType
	switch strategy {
	case StrategyWaitAll:
		status = p.calculateWaitAllStatus()
	case StrategyFailFast:
		status = p.calculateFailFastStatus()
	case StrategyBestEffort:
		status = p.calculateBestEffortStatus()
	case StrategyRace:
		status = p.calculateRaceStatus()
	default:
		// Default to wait_all strategy
		status = p.calculateWaitAllStatus()
	}
	p.OverallStatus = status
	return status
}

// calculateWaitAllStatus implements wait_all strategy - all tasks must complete successfully
func (p *ProgressInfo) calculateWaitAllStatus() core.StatusType {
	if p.SuccessCount == p.TotalChildren {
		return core.StatusSuccess
	}
	if p.RunningCount > 0 {
		return core.StatusRunning
	}
	if p.FailedCount > 0 || p.TimedOutCount > 0 {
		return core.StatusFailed
	}
	return core.StatusPending
}

// calculateFailFastStatus implements fail_fast strategy - fail immediately on first failure
func (p *ProgressInfo) calculateFailFastStatus() core.StatusType {
	if p.FailedCount > 0 || p.TimedOutCount > 0 {
		return core.StatusFailed
	}
	if p.SuccessCount == p.TotalChildren {
		return core.StatusSuccess
	}
	if p.RunningCount > 0 {
		return core.StatusRunning
	}
	return core.StatusPending
}

// calculateBestEffortStatus implements best_effort strategy - succeed if at least one succeeds
func (p *ProgressInfo) calculateBestEffortStatus() core.StatusType {
	if p.TerminalCount == p.TotalChildren {
		if p.SuccessCount > 0 {
			return core.StatusSuccess
		}
		return core.StatusFailed
	}
	if p.RunningCount > 0 {
		return core.StatusRunning
	}
	return core.StatusPending
}

// calculateRaceStatus implements race strategy - succeed on first completion
func (p *ProgressInfo) calculateRaceStatus() core.StatusType {
	if p.SuccessCount > 0 {
		return core.StatusSuccess
	}
	if p.TerminalCount == p.TotalChildren && p.SuccessCount == 0 {
		return core.StatusFailed
	}
	if p.RunningCount > 0 {
		return core.StatusRunning
	}
	return core.StatusPending
}

// IsComplete returns true if the parent task should be considered complete based on strategy
func (p *ProgressInfo) IsComplete(strategy ParallelStrategy) bool {
	status := p.CalculateOverallStatus(strategy)
	return status == core.StatusSuccess || status == core.StatusFailed
}

// HasFailures returns true if any child tasks have failed
func (p *ProgressInfo) HasFailures() bool {
	return p.FailedCount > 0
}

// IsAllComplete returns true if all child tasks are in a terminal state
func (p *ProgressInfo) IsAllComplete() bool {
	return p.TerminalCount == p.TotalChildren
}

// -----------------------------------------------------------------------------
// Progress Context Types
// -----------------------------------------------------------------------------

// ProgressState represents progress tracking with timing information for template context
type ProgressState struct {
	TotalChildren  int                     `json:"total_children"`
	SuccessCount   int                     `json:"success_count"` // Renamed from CompletedCount
	FailedCount    int                     `json:"failed_count"`
	CanceledCount  int                     `json:"canceled_count"`  // New field
	TimedOutCount  int                     `json:"timed_out_count"` // New field
	TerminalCount  int                     `json:"terminal_count"`  // New field
	RunningCount   int                     `json:"running_count"`
	PendingCount   int                     `json:"pending_count"`
	StatusCounts   map[core.StatusType]int `json:"status_counts"`
	StartTime      time.Time               `json:"start_time"`
	LastUpdateTime time.Time               `json:"last_update_time"`
}

// CompletionRate calculates the percentage of successfully completed tasks
func (p *ProgressState) CompletionRate() float64 {
	if p.TotalChildren == 0 {
		return 0
	}
	return float64(p.SuccessCount) / float64(p.TotalChildren)
}

// FailureRate calculates the percentage of failed tasks
func (p *ProgressState) FailureRate() float64 {
	if p.TotalChildren == 0 {
		return 0
	}
	return float64(p.FailedCount) / float64(p.TotalChildren)
}

// OverallStatus returns the overall execution status using core status types
func (p *ProgressState) OverallStatus() core.StatusType {
	if p.TotalChildren == 0 {
		return core.StatusPending
	}
	if (p.FailedCount > 0 || p.TimedOutCount > 0) && p.SuccessCount > 0 {
		// Some succeeded, some failed - still running or partially failed
		if p.RunningCount > 0 {
			return core.StatusRunning
		}
		return core.StatusFailed // Consider partial failure as failed
	}
	if p.FailedCount > 0 || p.TimedOutCount > 0 {
		return core.StatusFailed
	}
	if p.SuccessCount == p.TotalChildren {
		return core.StatusSuccess
	}
	if p.RunningCount > 0 {
		return core.StatusRunning
	}
	return core.StatusPending
}

// OverallStatusString returns the overall execution status as a human-readable string
// for backward compatibility with templates that expect string status
func (p *ProgressState) OverallStatusString() string {
	status := p.OverallStatus()
	switch status {
	case core.StatusSuccess:
		return "completed"
	case core.StatusFailed:
		if p.SuccessCount > 0 && p.RunningCount == 0 {
			return "partial_failure"
		}
		return "failed"
	case core.StatusRunning:
		return "in_progress"
	case core.StatusPending:
		return "pending"
	default:
		return "unknown"
	}
}
