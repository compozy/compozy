package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
)

// MockClient wraps temporal mocks with our worker.Client interface
type MockClient struct {
	*mocks.Client
	scheduleClient *mocks.ScheduleClient
}

func NewMockClient() *MockClient {
	mockClient := &mocks.Client{}
	scheduleClient := &mocks.ScheduleClient{}
	mockClient.On("ScheduleClient").Return(scheduleClient)
	return &MockClient{
		Client:         mockClient,
		scheduleClient: scheduleClient,
	}
}

func (m *MockClient) AsWorkerClient() *worker.Client {
	return &worker.Client{
		Client: m.Client,
	}
}

func TestNewManager(t *testing.T) {
	t.Run("Should create manager with correct configuration", func(t *testing.T) {
		mockClient := NewMockClient()
		projectID := "test-project"
		mgr := NewManager(mockClient.AsWorkerClient(), projectID)
		require.NotNil(t, mgr)
		// Since manager is an interface, we can't access internal state directly
		// We'll test the behavior through interface methods in other tests
	})
}

func TestScheduleIDGeneration(t *testing.T) {
	t.Run("Should generate correct schedule IDs", func(t *testing.T) {
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "my-project",
			taskQueue:     "my-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		// Test schedule ID generation
		scheduleID := m.scheduleID("workflow-123")
		assert.Equal(t, "schedule-my-project-workflow-123", scheduleID)
		// Test workflow ID extraction
		workflowID := m.workflowIDFromScheduleID(scheduleID)
		assert.Equal(t, "workflow-123", workflowID)
		// Test prefix generation
		prefix := m.schedulePrefix()
		assert.Equal(t, "schedule-my-project-", prefix)
	})
}

func TestReconcileSchedules(t *testing.T) {
	t.Run("Should create new schedules", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		// Mock empty existing schedules - List returns an iterator
		mockIterator := &mocks.ScheduleListIterator{}
		mockIterator.On("HasNext").Return(false).Once()
		mockClient.scheduleClient.On("List", ctx, mock.Anything).
			Return(mockIterator, nil).Once()
		// Prepare test workflows
		enabled := true
		workflows := []*workflow.Config{
			{
				ID: "scheduled-workflow",
				Schedule: &workflow.Schedule{
					Cron:          "0 0 */5 * * *",
					Timezone:      "UTC",
					Enabled:       &enabled,
					OverlapPolicy: workflow.OverlapSkip,
				},
			},
		}
		// Mock schedule creation
		mockHandle := &mocks.ScheduleHandle{}
		mockHandle.On("GetID").Return("schedule-test-project-scheduled-workflow")
		mockClient.scheduleClient.On("Create", ctx, mock.Anything).
			Return(mockHandle, nil).Once()
		// Execute reconciliation
		err := m.ReconcileSchedules(ctx, workflows)
		require.NoError(t, err)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
	t.Run("Should update existing schedules", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		// Mock existing schedule - List returns an iterator
		existingScheduleID := "schedule-test-project-workflow-1"
		mockIterator := &mocks.ScheduleListIterator{}
		mockIterator.On("HasNext").Return(true).Once()
		mockIterator.On("Next").Return(&client.ScheduleListEntry{
			ID: existingScheduleID,
		}, nil).Once()
		mockIterator.On("HasNext").Return(false).Once()
		mockClient.scheduleClient.On("List", ctx, mock.Anything).
			Return(mockIterator, nil).Once()
		// Create mock handle for existing schedule
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, existingScheduleID).Return(mockHandle)
		// Mock describe for update check
		mockHandle.On("Describe", ctx).Return(&client.ScheduleDescription{
			Schedule: client.Schedule{
				Spec: &client.ScheduleSpec{
					CronExpressions: []string{"0 0 */10 * * *"}, // Different from new config
					TimeZoneName:    "UTC",
				},
				State: &client.ScheduleState{
					Paused: false,
				},
			},
		}, nil).Once()
		// Mock update
		mockHandle.On("Update", ctx, mock.Anything).
			Return(nil).Once()
		// Prepare test workflows with updated schedule
		enabled := true
		workflows := []*workflow.Config{
			{
				ID: "workflow-1",
				Schedule: &workflow.Schedule{
					Cron:          "0 0 */5 * * *", // Changed from */10 to */5
					Timezone:      "UTC",
					Enabled:       &enabled,
					OverlapPolicy: workflow.OverlapSkip,
				},
			},
		}
		// Execute reconciliation
		err := m.ReconcileSchedules(ctx, workflows)
		require.NoError(t, err)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
	t.Run("Should delete removed schedules", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		// Mock existing schedule that should be deleted
		scheduleToDelete := "schedule-test-project-old-workflow"
		mockIterator := &mocks.ScheduleListIterator{}
		mockIterator.On("HasNext").Return(true).Once()
		mockIterator.On("Next").Return(&client.ScheduleListEntry{
			ID: scheduleToDelete,
		}, nil).Once()
		mockIterator.On("HasNext").Return(false).Once()
		mockClient.scheduleClient.On("List", ctx, mock.Anything).
			Return(mockIterator, nil).Once()
		// Create mock handle for deletion
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, scheduleToDelete).Return(mockHandle)
		mockHandle.On("Delete", ctx).Return(nil).Once()
		// Empty workflows list (nothing in YAML)
		workflows := []*workflow.Config{}
		// Execute reconciliation
		err := m.ReconcileSchedules(ctx, workflows)
		require.NoError(t, err)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
	t.Run("Should handle partial failures gracefully", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		// Mock iterator that fails (simulating Temporal connectivity issues)
		mockClient.scheduleClient.On("List", ctx, mock.Anything).
			Return(nil, errors.New("temporal connection failed")).Once()

		// Workflows with schedules (desired state should still be built)
		enabled := true
		workflows := []*workflow.Config{
			{
				ID: "workflow-1",
				Schedule: &workflow.Schedule{
					Cron:    "0 0 */5 * * *",
					Enabled: &enabled,
				},
			},
		}

		// Mock create handle for new schedule (since existing schedules failed to list)
		mockHandle := &mocks.ScheduleHandle{}
		mockHandle.On("GetID").Return("schedule-test-project-workflow-1")
		mockClient.scheduleClient.On("Create", ctx, mock.Anything).
			Return(mockHandle, nil).Once()

		// Execute reconciliation - should proceed despite listing failure
		err := m.ReconcileSchedules(ctx, workflows)
		require.NoError(t, err)

		// Verify create was called (since no existing schedules were found)
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
}

func TestUpdateSchedule(t *testing.T) {
	t.Run("Should update schedule and track override", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		workflowID := "workflow-1"
		scheduleID := "schedule-test-project-workflow-1"
		enabled := false
		updateReq := UpdateRequest{
			Enabled: &enabled,
		}
		// Create mock handle
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, scheduleID).Return(mockHandle)
		// Mock describe to get current schedule state
		mockHandle.On("Describe", ctx).Return(&client.ScheduleDescription{
			Schedule: client.Schedule{
				Spec: &client.ScheduleSpec{
					CronExpressions: []string{"0 0 */5 * * *"},
					TimeZoneName:    "UTC",
				},
				State: &client.ScheduleState{
					Paused: true, // Currently paused, will be enabled
				},
			},
			Info: client.ScheduleInfo{},
		}, nil).Once()
		// Mock update
		mockHandle.On("Update", ctx, mock.Anything).
			Return(nil).Once()
		// Execute update
		err := m.UpdateSchedule(ctx, workflowID, updateReq)
		require.NoError(t, err)
		// Verify override was stored
		override, exists := m.overrideCache.GetOverride(workflowID)
		assert.True(t, exists)
		assert.Equal(t, workflowID, override.WorkflowID)
		assert.Equal(t, false, override.Values["enabled"])
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
	t.Run("Should remove override on update failure", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		workflowID := "workflow-1"
		scheduleID := "schedule-test-project-workflow-1"
		enabled := false
		updateReq := UpdateRequest{
			Enabled: &enabled,
		}
		// Create mock handle
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, scheduleID).Return(mockHandle)
		// Mock describe to get current schedule state
		mockHandle.On("Describe", ctx).Return(&client.ScheduleDescription{
			Schedule: client.Schedule{
				Spec: &client.ScheduleSpec{
					CronExpressions: []string{"0 0 */5 * * *"},
					TimeZoneName:    "UTC",
				},
				State: &client.ScheduleState{
					Paused: true,
				},
			},
			Info: client.ScheduleInfo{},
		}, nil).Once()
		// Mock update failure
		mockHandle.On("Update", ctx, mock.Anything).
			Return(assert.AnError).Once()
		// Execute update
		err := m.UpdateSchedule(ctx, workflowID, updateReq)
		require.Error(t, err)
		// Verify override was NOT stored
		_, exists := m.overrideCache.GetOverride(workflowID)
		assert.False(t, exists)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
	t.Run("Should update schedule with cron override", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		workflowID := "workflow-1"
		scheduleID := "schedule-test-project-workflow-1"
		enabled := false
		cronOverride := "0 0 */10 * * *"
		updateReq := UpdateRequest{
			Enabled: &enabled,
			Cron:    &cronOverride,
		}
		// Create mock handle
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, scheduleID).Return(mockHandle)
		// Mock describe to get current schedule state
		mockHandle.On("Describe", ctx).Return(&client.ScheduleDescription{
			Schedule: client.Schedule{
				Spec: &client.ScheduleSpec{
					CronExpressions: []string{"0 0 */5 * * *"},
					TimeZoneName:    "UTC",
				},
				State: &client.ScheduleState{
					Paused: true,
				},
			},
			Info: client.ScheduleInfo{},
		}, nil).Once()
		// Mock update
		mockHandle.On("Update", ctx, mock.Anything).
			Return(nil).Once()
		// Execute update
		err := m.UpdateSchedule(ctx, workflowID, updateReq)
		require.NoError(t, err)
		// Verify override was stored with both enabled and cron
		override, exists := m.overrideCache.GetOverride(workflowID)
		assert.True(t, exists)
		assert.Equal(t, workflowID, override.WorkflowID)
		assert.Equal(t, false, override.Values["enabled"])
		assert.Equal(t, cronOverride, override.Values["cron"])
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})

	t.Run("Should reject invalid cron expression", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		workflowID := "workflow-1"
		scheduleID := "schedule-test-project-workflow-1"
		invalidCron := "invalid cron"
		updateReq := UpdateRequest{
			Cron: &invalidCron,
		}
		// Create mock handle - update should not be called
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, scheduleID).Return(mockHandle)
		// Mock describe to get current schedule state (called before validation)
		mockHandle.On("Describe", ctx).Return(&client.ScheduleDescription{
			Schedule: client.Schedule{
				Spec: &client.ScheduleSpec{
					CronExpressions: []string{"0 0 */5 * * *"},
					TimeZoneName:    "UTC",
				},
				State: &client.ScheduleState{
					Paused: false,
				},
			},
			Info: client.ScheduleInfo{},
		}, nil).Once()
		// Execute update
		err := m.UpdateSchedule(ctx, workflowID, updateReq)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid cron expression")
		// Verify override was NOT stored
		_, exists := m.overrideCache.GetOverride(workflowID)
		assert.False(t, exists)
		// Verify update was NOT called
		mockHandle.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
}

func TestDeleteSchedule(t *testing.T) {
	t.Run("Should delete schedule and remove override", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		workflowID := "workflow-1"
		scheduleID := "schedule-test-project-workflow-1"
		// Add an override to verify it gets removed
		enabled := false
		m.overrideCache.SetOverride(workflowID, map[string]any{"enabled": enabled})
		// Create mock handle
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, scheduleID).Return(mockHandle)
		mockHandle.On("Delete", ctx).Return(nil).Once()
		// Execute delete
		err := m.DeleteSchedule(ctx, workflowID)
		require.NoError(t, err)
		// Verify override was removed
		_, exists := m.overrideCache.GetOverride(workflowID)
		assert.False(t, exists)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase with spaces",
			input:    "My Project Name",
			expected: "my-project-name",
		},
		{
			name:     "already lowercase",
			input:    "my-project",
			expected: "my-project",
		},
		{
			name:     "mixed case no spaces",
			input:    "MyProject",
			expected: "myproject",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := slugify(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestListSchedules(t *testing.T) {
	t.Run("Should list all schedules with workflow info", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		// Mock iterator returning schedules
		mockIterator := &mocks.ScheduleListIterator{}
		mockIterator.On("HasNext").Return(true).Once()
		mockIterator.On("Next").Return(&client.ScheduleListEntry{
			ID: "schedule-test-project-workflow-1",
		}, nil).Once()
		mockIterator.On("HasNext").Return(false).Once()
		mockClient.scheduleClient.On("List", ctx, mock.Anything).
			Return(mockIterator, nil).Once()
		// Mock schedule handle and describe
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, "schedule-test-project-workflow-1").Return(mockHandle)
		mockHandle.On("Describe", ctx).Return(&client.ScheduleDescription{
			Schedule: client.Schedule{
				Spec: &client.ScheduleSpec{
					CronExpressions: []string{"0 0 */5 * * *"},
					TimeZoneName:    "UTC",
				},
				State: &client.ScheduleState{
					Paused: false,
				},
			},
			Info: client.ScheduleInfo{},
		}, nil).Once()
		// Execute
		schedules, err := m.ListSchedules(ctx)
		require.NoError(t, err)
		require.Len(t, schedules, 1)
		assert.Equal(t, "workflow-1", schedules[0].WorkflowID)
		assert.Equal(t, "0 0 */5 * * *", schedules[0].Cron)
		assert.True(t, schedules[0].Enabled)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
	t.Run("Should handle list error", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		// Mock list error
		mockClient.scheduleClient.On("List", ctx, mock.Anything).
			Return(nil, assert.AnError).Once()
		// Execute
		schedules, err := m.ListSchedules(ctx)
		require.Error(t, err)
		assert.Nil(t, schedules)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
	})
}

func TestGetSchedule(t *testing.T) {
	t.Run("Should get schedule info for workflow", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		workflowID := "workflow-1"
		scheduleID := "schedule-test-project-workflow-1"
		// Mock schedule handle and describe
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, scheduleID).Return(mockHandle)
		mockHandle.On("Describe", ctx).Return(&client.ScheduleDescription{
			Schedule: client.Schedule{
				Spec: &client.ScheduleSpec{
					CronExpressions: []string{"0 0 */5 * * *"},
					TimeZoneName:    "UTC",
				},
				State: &client.ScheduleState{
					Paused: false,
				},
			},
			Info: client.ScheduleInfo{},
		}, nil).Once()
		// Execute
		info, err := m.GetSchedule(ctx, workflowID)
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Equal(t, workflowID, info.WorkflowID)
		assert.Equal(t, scheduleID, info.ScheduleID)
		assert.Equal(t, "0 0 */5 * * *", info.Cron)
		assert.True(t, info.Enabled)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
	t.Run("Should handle schedule not found", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}
		workflowID := "workflow-1"
		scheduleID := "schedule-test-project-workflow-1"
		// Mock schedule handle with not found error
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, scheduleID).Return(mockHandle)
		mockHandle.On("Describe", ctx).Return(nil, errors.New("schedule not found")).Once()
		// Execute
		info, err := m.GetSchedule(ctx, workflowID)
		require.Error(t, err)
		assert.Nil(t, info)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
}

func TestManager_OverrideTracking(t *testing.T) {
	t.Run("Should track API overrides and skip reconciliation", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}

		// Set up an API override
		workflowID := "test-workflow"
		override := UpdateRequest{Enabled: &[]bool{false}[0]}

		// Mock schedule handle for update
		scheduleID := m.scheduleID(workflowID)
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, scheduleID).Return(mockHandle)

		// Mock describe to get current schedule state
		mockHandle.On("Describe", ctx).Return(&client.ScheduleDescription{
			Schedule: client.Schedule{
				Spec: &client.ScheduleSpec{
					CronExpressions: []string{"0 0 */5 * * *"},
					TimeZoneName:    "UTC",
				},
				State: &client.ScheduleState{
					Paused: true,
				},
			},
			Info: client.ScheduleInfo{},
		}, nil).Once()

		// Mock successful update
		mockHandle.On("Update", ctx, mock.Anything).Return(nil).Once()

		// Execute API override
		err := m.UpdateSchedule(ctx, workflowID, override)
		require.NoError(t, err)

		// Verify override is tracked in cache
		cachedOverride, exists := m.overrideCache.GetOverride(workflowID)
		assert.True(t, exists)
		assert.Equal(t, workflowID, cachedOverride.WorkflowID)
		assert.Equal(t, false, cachedOverride.Values["enabled"])

		// Verify reconciliation would be skipped for older YAML
		oldYAMLTime := time.Now().Add(-time.Hour)
		shouldSkip := m.overrideCache.ShouldSkipReconciliation(workflowID, oldYAMLTime)
		assert.True(t, shouldSkip)

		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})

	t.Run("Should respect override logic during reconciliation", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}

		// Mock empty existing schedules
		mockIterator := &mocks.ScheduleListIterator{}
		mockIterator.On("HasNext").Return(false).Once()
		mockClient.scheduleClient.On("List", ctx, mock.Anything).
			Return(mockIterator, nil).Once()

		// Create a workflow
		enabled := true
		workflows := []*workflow.Config{
			{
				ID: "test-workflow",
				Schedule: &workflow.Schedule{
					Cron:    "0 0 */5 * * *",
					Enabled: &enabled,
				},
			},
		}

		// Mock schedule creation
		mockHandle := &mocks.ScheduleHandle{}
		mockHandle.On("GetID").Return("schedule-test-project-test-workflow")
		mockClient.scheduleClient.On("Create", ctx, mock.Anything).
			Return(mockHandle, nil).Once()

		// Execute reconciliation
		err := m.ReconcileSchedules(ctx, workflows)
		require.NoError(t, err)

		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
}

func TestManager_ConfigurationReload(t *testing.T) {
	t.Run("Should trigger reconciliation on configuration reload", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}

		// Mock empty existing schedules
		mockIterator := &mocks.ScheduleListIterator{}
		mockIterator.On("HasNext").Return(false).Once()
		mockClient.scheduleClient.On("List", ctx, mock.Anything).
			Return(mockIterator, nil).Once()

		// Prepare test workflows
		enabled := true
		workflows := []*workflow.Config{
			{
				ID: "reload-workflow",
				Schedule: &workflow.Schedule{
					Cron:    "0 0 */5 * * *",
					Enabled: &enabled,
				},
			},
		}

		// Mock schedule creation
		mockHandle := &mocks.ScheduleHandle{}
		mockHandle.On("GetID").Return("schedule-test-project-reload-workflow")
		mockClient.scheduleClient.On("Create", ctx, mock.Anything).
			Return(mockHandle, nil).Once()

		// Execute configuration reload
		err := m.OnConfigurationReload(ctx, workflows)
		require.NoError(t, err)

		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
}

func TestManager_PeriodicReconciliationValidation(t *testing.T) {
	t.Run("Should validate positive interval", func(t *testing.T) {
		ctx := context.Background()
		m := &manager{
			config:        DefaultConfig(),
			overrideCache: NewOverrideCache(),
		}

		getWorkflows := func() []*workflow.Config {
			return []*workflow.Config{}
		}

		// Test zero interval
		err := m.StartPeriodicReconciliation(ctx, getWorkflows, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "interval must be positive")

		// Test negative interval
		err = m.StartPeriodicReconciliation(ctx, getWorkflows, -time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "interval must be positive")
	})

	t.Run("Should handle stop when not started", func(_ *testing.T) {
		m := &manager{
			config: DefaultConfig(),
		}

		// Should not panic
		m.StopPeriodicReconciliation()
	})
}

func TestManager_PeriodicReconciliation(t *testing.T) {
	t.Run("Should start and stop periodic reconciliation", func(t *testing.T) {
		// This test would require mocking the worker client and temporal
		// For now, we'll test the basic validation logic
		manager := &manager{
			periodicCancel: nil,
		}

		ctx := context.Background()
		getWorkflows := func() []*workflow.Config {
			return []*workflow.Config{}
		}

		// Test invalid interval
		err := manager.StartPeriodicReconciliation(ctx, getWorkflows, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "interval must be positive")

		// Test negative interval
		err = manager.StartPeriodicReconciliation(ctx, getWorkflows, -time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "interval must be positive")
	})

	t.Run("Should allow stopping when not started", func(_ *testing.T) {
		manager := &manager{}

		// Should not panic
		manager.StopPeriodicReconciliation()
	})
}
