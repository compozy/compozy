package task

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
)

func TestProgressInfo_CalculateOverallStatus(t *testing.T) {
	tests := []struct {
		name           string
		progressInfo   *ProgressInfo
		strategy       ParallelStrategy
		expectedStatus core.StatusType
	}{
		{
			name: "WaitAll strategy - all completed successfully",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 3,
				FailedCount:    0,
				RunningCount:   0,
				PendingCount:   0,
			},
			strategy:       StrategyWaitAll,
			expectedStatus: core.StatusSuccess,
		},
		{
			name: "WaitAll strategy - one failed",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 2,
				FailedCount:    1,
				RunningCount:   0,
				PendingCount:   0,
			},
			strategy:       StrategyWaitAll,
			expectedStatus: core.StatusFailed,
		},
		{
			name: "WaitAll strategy - still running",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 1,
				FailedCount:    0,
				RunningCount:   2,
				PendingCount:   0,
			},
			strategy:       StrategyWaitAll,
			expectedStatus: core.StatusRunning,
		},
		{
			name: "FailFast strategy - immediate failure",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 1,
				FailedCount:    1,
				RunningCount:   1,
				PendingCount:   0,
			},
			strategy:       StrategyFailFast,
			expectedStatus: core.StatusFailed,
		},
		{
			name: "FailFast strategy - all completed successfully",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 3,
				FailedCount:    0,
				RunningCount:   0,
				PendingCount:   0,
			},
			strategy:       StrategyFailFast,
			expectedStatus: core.StatusSuccess,
		},
		{
			name: "BestEffort strategy - some completed, some failed",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 2,
				FailedCount:    1,
				RunningCount:   0,
				PendingCount:   0,
			},
			strategy:       StrategyBestEffort,
			expectedStatus: core.StatusSuccess,
		},
		{
			name: "BestEffort strategy - all failed",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 0,
				FailedCount:    3,
				RunningCount:   0,
				PendingCount:   0,
			},
			strategy:       StrategyBestEffort,
			expectedStatus: core.StatusFailed,
		},
		{
			name: "Race strategy - first completed",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 1,
				FailedCount:    0,
				RunningCount:   2,
				PendingCount:   0,
			},
			strategy:       StrategyRace,
			expectedStatus: core.StatusSuccess,
		},
		{
			name: "Race strategy - all failed",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 0,
				FailedCount:    3,
				RunningCount:   0,
				PendingCount:   0,
			},
			strategy:       StrategyRace,
			expectedStatus: core.StatusFailed,
		},
		{
			name: "Empty children - should be pending",
			progressInfo: &ProgressInfo{
				TotalChildren:  0,
				CompletedCount: 0,
				FailedCount:    0,
				RunningCount:   0,
				PendingCount:   0,
			},
			strategy:       StrategyWaitAll,
			expectedStatus: core.StatusPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualStatus := tt.progressInfo.CalculateOverallStatus(tt.strategy)
			assert.Equal(t, tt.expectedStatus, actualStatus,
				"Expected status %s, got %s for strategy %s",
				tt.expectedStatus, actualStatus, tt.strategy)
		})
	}
}

func TestProgressInfo_IsComplete(t *testing.T) {
	tests := []struct {
		name         string
		progressInfo *ProgressInfo
		strategy     ParallelStrategy
		expected     bool
	}{
		{
			name: "WaitAll strategy - all completed",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 3,
				FailedCount:    0,
			},
			strategy: StrategyWaitAll,
			expected: true,
		},
		{
			name: "WaitAll strategy - still running",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 1,
				RunningCount:   2,
			},
			strategy: StrategyWaitAll,
			expected: false,
		},
		{
			name: "Race strategy - first completed",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 1,
				RunningCount:   2,
			},
			strategy: StrategyRace,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.progressInfo.IsComplete(tt.strategy)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestProgressInfo_HasFailures(t *testing.T) {
	tests := []struct {
		name         string
		progressInfo *ProgressInfo
		expected     bool
	}{
		{
			name: "Has failures",
			progressInfo: &ProgressInfo{
				FailedCount: 1,
			},
			expected: true,
		},
		{
			name: "No failures",
			progressInfo: &ProgressInfo{
				FailedCount: 0,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.progressInfo.HasFailures()
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestProgressInfo_IsAllComplete(t *testing.T) {
	tests := []struct {
		name         string
		progressInfo *ProgressInfo
		expected     bool
	}{
		{
			name: "All tasks completed",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 2,
				FailedCount:    1,
			},
			expected: true,
		},
		{
			name: "Some tasks still running",
			progressInfo: &ProgressInfo{
				TotalChildren:  3,
				CompletedCount: 1,
				FailedCount:    1,
				RunningCount:   1,
			},
			expected: false,
		},
		{
			name: "Empty task list",
			progressInfo: &ProgressInfo{
				TotalChildren: 0,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.progressInfo.IsAllComplete()
			assert.Equal(t, tt.expected, actual)
		})
	}
}
