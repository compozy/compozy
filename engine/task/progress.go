package task

import (
	"github.com/compozy/compozy/engine/core"
)

// -----------------------------------------------------------------------------
// Progress Aggregation Types
// -----------------------------------------------------------------------------

// ProgressInfo represents aggregated progress information for a parent task
type ProgressInfo struct {
	TotalChildren  int                     `json:"total_children"`
	CompletedCount int                     `json:"completed_count"`
	FailedCount    int                     `json:"failed_count"`
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
	if p.CompletedCount == p.TotalChildren {
		return core.StatusSuccess
	}
	if p.RunningCount > 0 {
		return core.StatusRunning
	}
	if p.FailedCount > 0 {
		return core.StatusFailed
	}
	return core.StatusPending
}

// calculateFailFastStatus implements fail_fast strategy - fail immediately on first failure
func (p *ProgressInfo) calculateFailFastStatus() core.StatusType {
	if p.FailedCount > 0 {
		return core.StatusFailed
	}
	if p.CompletedCount == p.TotalChildren {
		return core.StatusSuccess
	}
	if p.RunningCount > 0 {
		return core.StatusRunning
	}
	return core.StatusPending
}

// calculateBestEffortStatus implements best_effort strategy - succeed if at least one succeeds
func (p *ProgressInfo) calculateBestEffortStatus() core.StatusType {
	allDone := (p.CompletedCount + p.FailedCount) == p.TotalChildren
	if allDone {
		if p.CompletedCount > 0 {
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
	if p.CompletedCount > 0 {
		return core.StatusSuccess
	}
	if p.FailedCount == p.TotalChildren {
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
	return (p.CompletedCount + p.FailedCount) == p.TotalChildren
}
