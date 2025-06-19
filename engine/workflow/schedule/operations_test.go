package schedule

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
)

func TestListSchedulesByPrefix(t *testing.T) {
	t.Run("Should list schedules with pagination", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			overrideCache: NewOverrideCache(),
		}
		// Mock iterator
		mockIterator := &mocks.ScheduleListIterator{}
		mockIterator.On("HasNext").Return(true).Once()
		mockIterator.On("Next").Return(&client.ScheduleListEntry{ID: "schedule-test-1"}, nil).Once()
		mockIterator.On("HasNext").Return(true).Once()
		mockIterator.On("Next").Return(&client.ScheduleListEntry{ID: "schedule-test-2"}, nil).Once()
		mockIterator.On("HasNext").Return(true).Once()
		mockIterator.On("Next").Return(&client.ScheduleListEntry{ID: "schedule-test-3"}, nil).Once()
		mockIterator.On("HasNext").Return(false).Once()
		mockClient.scheduleClient.On("List", ctx, mock.Anything).Return(mockIterator, nil).Once()
		// Mock GetHandle calls
		mockHandle1 := &mocks.ScheduleHandle{}
		mockHandle2 := &mocks.ScheduleHandle{}
		mockHandle3 := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, "schedule-test-1").Return(mockHandle1)
		mockClient.scheduleClient.On("GetHandle", ctx, "schedule-test-2").Return(mockHandle2)
		mockClient.scheduleClient.On("GetHandle", ctx, "schedule-test-3").Return(mockHandle3)
		// Execute
		schedules, err := m.listSchedulesByPrefix(ctx, "schedule-test-")
		require.NoError(t, err)
		assert.Len(t, schedules, 3)
		assert.Contains(t, schedules, "schedule-test-1")
		assert.Contains(t, schedules, "schedule-test-2")
		assert.Contains(t, schedules, "schedule-test-3")
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
	})
}

func TestGetScheduleInfo(t *testing.T) {
	t.Run("Should extract schedule information correctly", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			overrideCache: NewOverrideCache(),
		}
		scheduleID := "schedule-test-project-workflow-1"
		mockHandle := &mocks.ScheduleHandle{}
		nextRun := time.Now().Add(5 * time.Minute)
		lastRun := time.Now().Add(-10 * time.Minute)
		// Mock describe
		mockHandle.On("Describe", ctx).Return(&client.ScheduleDescription{
			Schedule: client.Schedule{
				Spec: &client.ScheduleSpec{
					CronExpressions: []string{"0 0 */5 * * *"},
					TimeZoneName:    "America/New_York",
				},
				State: &client.ScheduleState{
					Paused: false,
				},
			},
			Info: client.ScheduleInfo{
				NextActionTimes: []time.Time{nextRun},
				RecentActions: []client.ScheduleActionResult{
					{
						ScheduleTime: lastRun,
					},
				},
			},
		}, nil).Once()
		// Execute
		info, err := m.getScheduleInfo(ctx, scheduleID, mockHandle)
		require.NoError(t, err)
		// Verify extracted information
		assert.Equal(t, "workflow-1", info.WorkflowID)
		assert.Equal(t, scheduleID, info.ScheduleID)
		assert.Equal(t, "0 0 */5 * * *", info.Cron)
		assert.Equal(t, "America/New_York", info.Timezone)
		assert.True(t, info.Enabled)
		assert.Equal(t, nextRun, info.NextRunTime)
		assert.NotNil(t, info.LastRunTime)
		assert.Equal(t, lastRun, *info.LastRunTime)
		assert.Equal(t, "unknown", info.LastRunStatus)
		// Verify all expectations
		mockHandle.AssertExpectations(t)
	})
	t.Run("Should detect overrides", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		enabled := false
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			overrideCache: NewOverrideCache(),
		}
		// Set up the override
		m.overrideCache.SetOverride("workflow-1", map[string]any{"enabled": enabled})
		scheduleID := "schedule-test-project-workflow-1"
		mockHandle := &mocks.ScheduleHandle{}
		// Mock describe
		mockHandle.On("Describe", ctx).Return(&client.ScheduleDescription{
			Schedule: client.Schedule{
				Spec: &client.ScheduleSpec{
					CronExpressions: []string{"0 0 */5 * * *"},
				},
				State: &client.ScheduleState{
					Paused: true,
				},
			},
			Info: client.ScheduleInfo{},
		}, nil).Once()
		// Execute
		info, err := m.getScheduleInfo(ctx, scheduleID, mockHandle)
		require.NoError(t, err)
		// Verify override detected
		assert.True(t, info.IsOverride)
		// Verify all expectations
		mockHandle.AssertExpectations(t)
	})
}

func TestCreateSchedule(t *testing.T) {
	t.Run("Should create schedule with correct configuration", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			overrideCache: NewOverrideCache(),
		}
		enabled := true
		startAt := time.Now().Add(1 * time.Hour)
		endAt := time.Now().Add(24 * time.Hour)
		wf := &workflow.Config{
			ID: "workflow-1",
			Schedule: &workflow.Schedule{
				Cron:          "0 0 */5 * * *",
				Timezone:      "America/New_York",
				Enabled:       &enabled,
				Jitter:        "30s",
				OverlapPolicy: workflow.OverlapBufferOne,
				StartAt:       &startAt,
				EndAt:         &endAt,
				Input: map[string]any{
					"key": "value",
				},
			},
		}
		scheduleID := "schedule-test-project-workflow-1"
		// Mock handle
		mockHandle := &mocks.ScheduleHandle{}
		mockHandle.On("GetID").Return(scheduleID)
		// Capture the create options
		var capturedOptions client.ScheduleOptions
		mockClient.scheduleClient.On("Create", ctx, mock.Anything).
			Run(func(args mock.Arguments) {
				capturedOptions = args.Get(1).(client.ScheduleOptions)
			}).
			Return(mockHandle, nil).Once()
		// Execute
		err := m.createSchedule(ctx, scheduleID, wf)
		require.NoError(t, err)
		// Verify captured options
		assert.Equal(t, scheduleID, capturedOptions.ID)
		assert.Equal(t, "0 0 */5 * * * *", capturedOptions.Spec.CronExpressions[0])
		assert.Equal(t, "America/New_York", capturedOptions.Spec.TimeZoneName)
		assert.NotNil(t, capturedOptions.Spec.Jitter)
		assert.Equal(t, 30*time.Second, capturedOptions.Spec.Jitter)
		assert.Equal(t, startAt, capturedOptions.Spec.StartAt)
		assert.Equal(t, endAt, capturedOptions.Spec.EndAt)
		assert.False(t, capturedOptions.Paused)
		// Note: Overlap policy is not part of ScheduleOptions in SDK v1.34.0
		// Verify workflow action
		action := capturedOptions.Action.(*client.ScheduleWorkflowAction)
		assert.Equal(t, "workflow-1", action.ID)
		assert.Equal(t, "CompozyWorkflow", action.Workflow)
		assert.Equal(t, "test-project", action.TaskQueue)
		expectedTriggerInput := map[string]any{
			"workflow_id":      "workflow-1",
			"workflow_exec_id": "",
			"input":            wf.Schedule.Input,
		}
		assert.Equal(t, []any{expectedTriggerInput}, action.Args)
		// Verify memo
		assert.Equal(t, "test-project", capturedOptions.Memo["project_id"])
		assert.Equal(t, "workflow-1", capturedOptions.Memo["workflow_id"])
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
	t.Run("Should handle disabled schedule", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			overrideCache: NewOverrideCache(),
		}
		enabled := false
		wf := &workflow.Config{
			ID: "workflow-1",
			Schedule: &workflow.Schedule{
				Cron:    "0 0 */5 * * *",
				Enabled: &enabled,
			},
		}
		scheduleID := "schedule-test-project-workflow-1"
		// Mock handle
		mockHandle := &mocks.ScheduleHandle{}
		mockHandle.On("GetID").Return(scheduleID)
		// Capture the create options
		var capturedOptions client.ScheduleOptions
		mockClient.scheduleClient.On("Create", ctx, mock.Anything).
			Run(func(args mock.Arguments) {
				capturedOptions = args.Get(1).(client.ScheduleOptions)
			}).
			Return(mockHandle, nil).Once()
		// Execute
		err := m.createSchedule(ctx, scheduleID, wf)
		require.NoError(t, err)
		// Verify schedule is paused
		assert.True(t, capturedOptions.Paused)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
}

func TestUpdateScheduleOperation(t *testing.T) {
	t.Run("Should skip update if no changes needed", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			overrideCache: NewOverrideCache(),
		}
		enabled := true
		wf := &workflow.Config{
			ID: "workflow-1",
			Schedule: &workflow.Schedule{
				Cron:          "0 0 */5 * * *",
				Timezone:      "UTC",
				Enabled:       &enabled,
				OverlapPolicy: workflow.OverlapSkip,
			},
		}
		scheduleID := "schedule-test-project-workflow-1"
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, scheduleID).Return(mockHandle)
		// Mock describe - everything matches (with 7-field format from Temporal)
		mockHandle.On("Describe", ctx).Return(&client.ScheduleDescription{
			Schedule: client.Schedule{
				Spec: &client.ScheduleSpec{
					CronExpressions: []string{"0 0 */5 * * * *"}, // 7-field format
					TimeZoneName:    "UTC",
				},
				State: &client.ScheduleState{
					Paused: false,
				},
			},
		}, nil).Once()
		// Execute - no update should be called
		err := m.updateSchedule(ctx, scheduleID, wf)
		require.NoError(t, err)
		// Verify update was NOT called
		mockHandle.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
	t.Run("Should update when changes detected", func(t *testing.T) {
		ctx := context.Background()
		mockClient := NewMockClient()
		m := &manager{
			client:        mockClient.AsWorkerClient(),
			projectID:     "test-project",
			taskQueue:     "test-project",
			overrideCache: NewOverrideCache(),
		}
		enabled := true
		wf := &workflow.Config{
			ID: "workflow-1",
			Schedule: &workflow.Schedule{
				Cron:          "0 0 */10 * * *",   // Changed
				Timezone:      "America/New_York", // Changed
				Enabled:       &enabled,
				OverlapPolicy: workflow.OverlapAllow, // Changed
			},
		}
		scheduleID := "schedule-test-project-workflow-1"
		mockHandle := &mocks.ScheduleHandle{}
		mockClient.scheduleClient.On("GetHandle", ctx, scheduleID).Return(mockHandle)
		// Mock describe - different values
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
		}, nil).Once()
		// Mock update
		mockHandle.On("Update", ctx, mock.Anything).
			Return(nil).Once()
		// Execute
		err := m.updateSchedule(ctx, scheduleID, wf)
		require.NoError(t, err)
		// Verify all expectations
		mockClient.scheduleClient.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
}
