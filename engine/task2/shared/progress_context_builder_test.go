package shared

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/stretchr/testify/assert"
)

func TestBuildProgressContext(t *testing.T) {
	t.Run("Should build progress context with all required fields", func(t *testing.T) {
		startTime := time.Now().Add(-30 * time.Second)
		progressState := &task.ProgressState{
			TotalChildren:  10,
			CompletedCount: 6,
			FailedCount:    2,
			RunningCount:   1,
			PendingCount:   1,
			StatusCounts: map[core.StatusType]int{
				core.StatusSuccess: 6,
				core.StatusFailed:  2,
				core.StatusRunning: 1,
				core.StatusPending: 1,
			},
			StartTime:      startTime,
			LastUpdateTime: time.Now(),
		}

		result := BuildProgressContext(progressState)

		assert.Equal(t, 10, result["total"])
		assert.Equal(t, 6, result["completed"])
		assert.Equal(t, 2, result["failed"])
		assert.Equal(t, 1, result["running"])
		assert.Equal(t, 1, result["pending"])
		assert.Equal(t, 0.6, result["completionRate"])
		assert.Equal(t, 0.2, result["failureRate"])
		assert.Equal(t, "in_progress", result["overallStatus"]) // Has running tasks = in_progress

		// Check elapsed time is reasonable (around 30 seconds)
		elapsedSeconds, ok := result["elapsedSeconds"].(float64)
		assert.True(t, ok)
		assert.Greater(t, elapsedSeconds, 29.0)
		assert.Less(t, elapsedSeconds, 31.0)
	})

	t.Run("Should handle empty progress state", func(t *testing.T) {
		startTime := time.Now()
		progressState := &task.ProgressState{
			TotalChildren:  0,
			CompletedCount: 0,
			FailedCount:    0,
			RunningCount:   0,
			PendingCount:   0,
			StatusCounts:   make(map[core.StatusType]int),
			StartTime:      startTime,
			LastUpdateTime: startTime,
		}

		result := BuildProgressContext(progressState)

		assert.Equal(t, 0, result["total"])
		assert.Equal(t, 0, result["completed"])
		assert.Equal(t, 0, result["failed"])
		assert.Equal(t, 0, result["running"])
		assert.Equal(t, 0, result["pending"])
		assert.Equal(t, 0.0, result["completionRate"])
		assert.Equal(t, 0.0, result["failureRate"])
		assert.Equal(t, "pending", result["overallStatus"]) // Empty state = pending

		// Check elapsed time is very small
		elapsedSeconds, ok := result["elapsedSeconds"].(float64)
		assert.True(t, ok)
		assert.Less(t, elapsedSeconds, 1.0)
	})

	t.Run("Should handle completed state", func(t *testing.T) {
		startTime := time.Now().Add(-60 * time.Second)
		progressState := &task.ProgressState{
			TotalChildren:  5,
			CompletedCount: 5,
			FailedCount:    0,
			RunningCount:   0,
			PendingCount:   0,
			StartTime:      startTime,
			LastUpdateTime: time.Now(),
		}

		result := BuildProgressContext(progressState)

		assert.Equal(t, 5, result["total"])
		assert.Equal(t, 5, result["completed"])
		assert.Equal(t, 0, result["failed"])
		assert.Equal(t, 1.0, result["completionRate"])
		assert.Equal(t, 0.0, result["failureRate"])
		assert.Equal(t, "completed", result["overallStatus"])
	})

	t.Run("Should handle all failed state", func(t *testing.T) {
		progressState := &task.ProgressState{
			TotalChildren:  3,
			CompletedCount: 0,
			FailedCount:    3,
			RunningCount:   0,
			PendingCount:   0,
			StartTime:      time.Now(),
			LastUpdateTime: time.Now(),
		}

		result := BuildProgressContext(progressState)

		assert.Equal(t, 3, result["total"])
		assert.Equal(t, 0, result["completed"])
		assert.Equal(t, 3, result["failed"])
		assert.Equal(t, 0.0, result["completionRate"])
		assert.Equal(t, 1.0, result["failureRate"])
		assert.Equal(t, "failed", result["overallStatus"]) // All failed = failed status
	})
}
