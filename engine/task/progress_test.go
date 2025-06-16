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
			name: "Should return success for WaitAll strategy when all completed successfully",
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
			name: "Should return failed for WaitAll strategy when one failed",
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
			name: "Should return running for WaitAll strategy when still running",
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
			name: "Should return failed for FailFast strategy on immediate failure",
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
			name: "Should return success for FailFast strategy when all completed successfully",
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
			name: "Should return success for BestEffort strategy when some completed and some failed",
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
			name: "Should return failed for BestEffort strategy when all failed",
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
			name: "Should return success for Race strategy when first completed",
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
			name: "Should return failed for Race strategy when all failed",
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
			name: "Should return pending for empty children",
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
	t.Run("Should return true for WaitAll strategy when all completed", func(t *testing.T) {
		progressInfo := &ProgressInfo{
			TotalChildren:  3,
			CompletedCount: 3,
			FailedCount:    0,
		}
		actual := progressInfo.IsComplete(StrategyWaitAll)
		assert.True(t, actual)
	})

	t.Run("Should return false for WaitAll strategy when still running", func(t *testing.T) {
		progressInfo := &ProgressInfo{
			TotalChildren:  3,
			CompletedCount: 1,
			RunningCount:   2,
		}
		actual := progressInfo.IsComplete(StrategyWaitAll)
		assert.False(t, actual)
	})

	t.Run("Should return true for Race strategy when first completed", func(t *testing.T) {
		progressInfo := &ProgressInfo{
			TotalChildren:  3,
			CompletedCount: 1,
			RunningCount:   2,
		}
		actual := progressInfo.IsComplete(StrategyRace)
		assert.True(t, actual)
	})
}

func TestProgressInfo_HasFailures(t *testing.T) {
	t.Run("Should return true when has failures", func(t *testing.T) {
		progressInfo := &ProgressInfo{
			FailedCount: 1,
		}
		actual := progressInfo.HasFailures()
		assert.True(t, actual)
	})

	t.Run("Should return false when no failures", func(t *testing.T) {
		progressInfo := &ProgressInfo{
			FailedCount: 0,
		}
		actual := progressInfo.HasFailures()
		assert.False(t, actual)
	})
}

func TestProgressInfo_IsAllComplete(t *testing.T) {
	t.Run("Should return true when all tasks completed", func(t *testing.T) {
		progressInfo := &ProgressInfo{
			TotalChildren:  3,
			CompletedCount: 2,
			FailedCount:    1,
		}
		actual := progressInfo.IsAllComplete()
		assert.True(t, actual)
	})

	t.Run("Should return false when some tasks still running", func(t *testing.T) {
		progressInfo := &ProgressInfo{
			TotalChildren:  3,
			CompletedCount: 1,
			FailedCount:    1,
			RunningCount:   1,
		}
		actual := progressInfo.IsAllComplete()
		assert.False(t, actual)
	})

	t.Run("Should return true for empty task list", func(t *testing.T) {
		progressInfo := &ProgressInfo{
			TotalChildren: 0,
		}
		actual := progressInfo.IsAllComplete()
		assert.True(t, actual)
	})
}
