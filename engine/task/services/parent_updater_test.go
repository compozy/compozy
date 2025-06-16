package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParentStatusUpdater_UpdateParentStatus(t *testing.T) {
	t.Run("Should return running status for WaitAll strategy with running children", func(t *testing.T) {
		ctx := context.Background()
		repo := testutil.NewInMemoryRepo()
		svc := NewParentStatusUpdater(repo)
		parent := testutil.BuildParent(task.StrategyWaitAll)
		repo.AddState(parent)
		childStatuses := []core.StatusType{core.StatusRunning, core.StatusSuccess, core.StatusSuccess}
		var originalChildren []*task.State
		for i, status := range childStatuses {
			child := testutil.BuildChildWithTaskID(parent.TaskExecID, status, fmt.Sprintf("child-%d", i))
			repo.AddState(child)
			originalChildren = append(originalChildren, child)
		}
		input := &UpdateParentStatusInput{
			ParentStateID: parent.TaskExecID,
			Strategy:      task.StrategyWaitAll,
			Recursive:     false,
		}
		updatedParent, err := svc.UpdateParentStatus(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, updatedParent)
		assert.Equal(t, core.StatusRunning, updatedParent.Status)
		persistedParent, err := repo.GetState(ctx, parent.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusRunning, persistedParent.Status)
		assert.True(t, persistedParent.UpdatedAt.After(persistedParent.CreatedAt), "UpdatedAt should be refreshed")
		for i, originalChild := range originalChildren {
			currentChild, err := repo.GetState(ctx, originalChild.TaskExecID)
			require.NoError(t, err)
			assert.Equal(t, originalChild.Status, currentChild.Status, "child %d status should remain unchanged", i)
		}
		require.NotNil(t, persistedParent.Output)
		progressInfo, exists := (*persistedParent.Output)["progress_info"]
		assert.True(t, exists, "progress_info should be set in output")
		assert.NotNil(t, progressInfo, "progress_info should not be nil")
	})

	t.Run("Should return success status for WaitAll strategy with all successful children", func(t *testing.T) {
		ctx := context.Background()
		repo := testutil.NewInMemoryRepo()
		svc := NewParentStatusUpdater(repo)
		parent := testutil.BuildParent(task.StrategyWaitAll)
		repo.AddState(parent)
		childStatuses := []core.StatusType{core.StatusSuccess, core.StatusSuccess, core.StatusSuccess}
		var originalChildren []*task.State
		for i, status := range childStatuses {
			child := testutil.BuildChildWithTaskID(parent.TaskExecID, status, fmt.Sprintf("child-%d", i))
			repo.AddState(child)
			originalChildren = append(originalChildren, child)
		}
		input := &UpdateParentStatusInput{
			ParentStateID: parent.TaskExecID,
			Strategy:      task.StrategyWaitAll,
			Recursive:     false,
		}
		updatedParent, err := svc.UpdateParentStatus(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, updatedParent)
		assert.Equal(t, core.StatusSuccess, updatedParent.Status)
		persistedParent, err := repo.GetState(ctx, parent.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, persistedParent.Status)
		assert.True(t, true, "Terminal status correctly set")
		for i, originalChild := range originalChildren {
			currentChild, err := repo.GetState(ctx, originalChild.TaskExecID)
			require.NoError(t, err)
			assert.Equal(t, originalChild.Status, currentChild.Status, "child %d status should remain unchanged", i)
		}
		require.NotNil(t, persistedParent.Output)
		progressInfo, exists := (*persistedParent.Output)["progress_info"]
		assert.True(t, exists, "progress_info should be set in output")
		assert.NotNil(t, progressInfo, "progress_info should not be nil")
	})

	t.Run("Should return failed status for FailFast strategy with failed child", func(t *testing.T) {
		ctx := context.Background()
		repo := testutil.NewInMemoryRepo()
		svc := NewParentStatusUpdater(repo)
		parent := testutil.BuildParent(task.StrategyFailFast)
		repo.AddState(parent)
		childStatuses := []core.StatusType{core.StatusFailed, core.StatusRunning, core.StatusSuccess}
		var originalChildren []*task.State
		for i, status := range childStatuses {
			child := testutil.BuildChildWithTaskID(parent.TaskExecID, status, fmt.Sprintf("child-%d", i))
			repo.AddState(child)
			originalChildren = append(originalChildren, child)
		}
		input := &UpdateParentStatusInput{
			ParentStateID: parent.TaskExecID,
			Strategy:      task.StrategyFailFast,
			Recursive:     false,
		}
		updatedParent, err := svc.UpdateParentStatus(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, updatedParent)
		assert.Equal(t, core.StatusFailed, updatedParent.Status)
		persistedParent, err := repo.GetState(ctx, parent.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusFailed, persistedParent.Status)
		assert.True(t, true, "Terminal status correctly set")
		for i, originalChild := range originalChildren {
			currentChild, err := repo.GetState(ctx, originalChild.TaskExecID)
			require.NoError(t, err)
			assert.Equal(t, originalChild.Status, currentChild.Status, "child %d status should remain unchanged", i)
		}
		require.NotNil(t, persistedParent.Output)
		progressInfo, exists := (*persistedParent.Output)["progress_info"]
		assert.True(t, exists, "progress_info should be set in output")
		assert.NotNil(t, progressInfo, "progress_info should not be nil")
	})

	t.Run("Should return success status for Race strategy with early completion", func(t *testing.T) {
		ctx := context.Background()
		repo := testutil.NewInMemoryRepo()
		svc := NewParentStatusUpdater(repo)
		parent := testutil.BuildParent(task.StrategyRace)
		repo.AddState(parent)
		childStatuses := []core.StatusType{core.StatusSuccess, core.StatusRunning, core.StatusRunning}
		var originalChildren []*task.State
		for i, status := range childStatuses {
			child := testutil.BuildChildWithTaskID(parent.TaskExecID, status, fmt.Sprintf("child-%d", i))
			repo.AddState(child)
			originalChildren = append(originalChildren, child)
		}
		input := &UpdateParentStatusInput{
			ParentStateID: parent.TaskExecID,
			Strategy:      task.StrategyRace,
			Recursive:     false,
		}
		updatedParent, err := svc.UpdateParentStatus(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, updatedParent)
		assert.Equal(t, core.StatusSuccess, updatedParent.Status)
		persistedParent, err := repo.GetState(ctx, parent.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, persistedParent.Status)
		assert.True(t, true, "Terminal status correctly set")
		for i, originalChild := range originalChildren {
			currentChild, err := repo.GetState(ctx, originalChild.TaskExecID)
			require.NoError(t, err)
			assert.Equal(t, originalChild.Status, currentChild.Status, "child %d status should remain unchanged", i)
		}
		require.NotNil(t, persistedParent.Output)
		progressInfo, exists := (*persistedParent.Output)["progress_info"]
		assert.True(t, exists, "progress_info should be set in output")
		assert.NotNil(t, progressInfo, "progress_info should not be nil")
	})

	t.Run("Should return pending status for WaitAll strategy with no children", func(t *testing.T) {
		ctx := context.Background()
		repo := testutil.NewInMemoryRepo()
		svc := NewParentStatusUpdater(repo)
		parent := testutil.BuildParent(task.StrategyWaitAll)
		repo.AddState(parent)
		input := &UpdateParentStatusInput{
			ParentStateID: parent.TaskExecID,
			Strategy:      task.StrategyWaitAll,
			Recursive:     false,
		}
		updatedParent, err := svc.UpdateParentStatus(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, updatedParent)
		assert.Equal(t, core.StatusPending, updatedParent.Status)
		persistedParent, err := repo.GetState(ctx, parent.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusPending, persistedParent.Status)
		assert.True(t, persistedParent.UpdatedAt.After(persistedParent.CreatedAt), "UpdatedAt should be refreshed")
		require.NotNil(t, persistedParent.Output)
		progressInfo, exists := (*persistedParent.Output)["progress_info"]
		assert.True(t, exists, "progress_info should be set in output")
		assert.NotNil(t, progressInfo, "progress_info should not be nil")
	})
}

func TestParentStatusUpdater_RecursiveUpdate(t *testing.T) {
	t.Run("Should update parent and grandparent recursively when enabled", func(t *testing.T) {
		ctx := context.Background()
		repo := testutil.NewInMemoryRepo()
		svc := NewParentStatusUpdater(repo)
		grandparent := testutil.BuildParent(task.StrategyWaitAll)
		parent := testutil.BuildChild(grandparent.TaskExecID, core.StatusRunning)
		child := testutil.BuildChild(parent.TaskExecID, core.StatusRunning)
		repo.AddState(grandparent)
		repo.AddState(parent)
		repo.AddState(child)
		child.Status = core.StatusSuccess
		repo.UpsertState(ctx, child)
		input := &UpdateParentStatusInput{
			ParentStateID: parent.TaskExecID,
			Strategy:      task.StrategyWaitAll,
			Recursive:     true,
		}
		updatedParent, err := svc.UpdateParentStatus(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, updatedParent.Status)
		persistedGrandparent, err := repo.GetState(ctx, grandparent.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, persistedGrandparent.Status)
	})
}

func TestParentStatusUpdater_CycleDetection(t *testing.T) {
	t.Run("Should return error when cycle is detected to prevent infinite loop", func(t *testing.T) {
		ctx := context.Background()
		repo := testutil.NewInMemoryRepo()
		svc := NewParentStatusUpdater(repo)
		parent := testutil.BuildParent(task.StrategyWaitAll)
		repo.AddState(parent)
		input := &UpdateParentStatusInput{
			ParentStateID: parent.TaskExecID,
			Strategy:      task.StrategyWaitAll,
			Recursive:     false,
			visited:       map[core.ID]bool{parent.TaskExecID: true}, // Pre-visited
		}
		_, err := svc.UpdateParentStatus(ctx, input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cycle detected")
	})
}

func TestParentStatusUpdater_MaxDepthExceeded(t *testing.T) {
	t.Run("Should return error when maximum recursion depth is exceeded", func(t *testing.T) {
		ctx := context.Background()
		repo := testutil.NewInMemoryRepo()
		svc := NewParentStatusUpdater(repo)
		parent := testutil.BuildParent(task.StrategyWaitAll)
		repo.AddState(parent)
		input := &UpdateParentStatusInput{
			ParentStateID: parent.TaskExecID,
			Strategy:      task.StrategyWaitAll,
			Recursive:     false,
			depth:         MaxRecursionDepth + 1,
		}
		_, err := svc.UpdateParentStatus(ctx, input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "maximum recursion depth")
	})
}

func TestParentStatusUpdater_ShouldUpdateParentStatus(t *testing.T) {
	tests := []struct {
		name          string
		currentStatus core.StatusType
		newStatus     core.StatusType
		shouldUpdate  bool
	}{
		{
			name:          "Should update when status changes from pending to running",
			currentStatus: core.StatusPending,
			newStatus:     core.StatusRunning,
			shouldUpdate:  true,
		},
		{
			name:          "Should not update when status changes from running to pending",
			currentStatus: core.StatusRunning,
			newStatus:     core.StatusPending,
			shouldUpdate:  false,
		},
		{
			name:          "Should update when status changes from running to success",
			currentStatus: core.StatusRunning,
			newStatus:     core.StatusSuccess,
			shouldUpdate:  true,
		},
		{
			name:          "Should update when status changes from success to failed",
			currentStatus: core.StatusSuccess,
			newStatus:     core.StatusFailed,
			shouldUpdate:  true,
		},
		{
			name:          "Should not update when status remains the same",
			currentStatus: core.StatusRunning,
			newStatus:     core.StatusRunning,
			shouldUpdate:  false,
		},
	}

	svc := NewParentStatusUpdater(nil) // No repo needed for this test

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.ShouldUpdateParentStatus(tt.currentStatus, tt.newStatus)
			assert.Equal(t, tt.shouldUpdate, result)
		})
	}
}

func TestParentStatusUpdater_ErrorHandling(t *testing.T) {
	t.Run("Should return error when parent state is not found", func(t *testing.T) {
		ctx := context.Background()
		repo := testutil.NewInMemoryRepo()
		svc := NewParentStatusUpdater(repo)
		nonExistentID, _ := core.NewID()
		input := &UpdateParentStatusInput{
			ParentStateID: nonExistentID,
			Strategy:      task.StrategyWaitAll,
			Recursive:     false,
		}
		_, err := svc.UpdateParentStatus(ctx, input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get parent state")
	})
}
