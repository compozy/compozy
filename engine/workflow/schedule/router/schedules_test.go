package schrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockScheduleManager is a mock implementation of schedule.Manager
type MockScheduleManager struct {
	mock.Mock
}

func (m *MockScheduleManager) ReconcileSchedules(ctx context.Context, workflows []*workflow.Config) error {
	args := m.Called(ctx, workflows)
	return args.Error(0)
}

func (m *MockScheduleManager) ListSchedules(ctx context.Context) ([]*schedule.Info, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*schedule.Info), args.Error(1)
}

func (m *MockScheduleManager) GetSchedule(ctx context.Context, workflowID string) (*schedule.Info, error) {
	args := m.Called(ctx, workflowID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*schedule.Info), args.Error(1)
}

func (m *MockScheduleManager) UpdateSchedule(
	ctx context.Context,
	workflowID string,
	update schedule.UpdateRequest,
) error {
	args := m.Called(ctx, workflowID, update)
	return args.Error(0)
}

func (m *MockScheduleManager) DeleteSchedule(ctx context.Context, workflowID string) error {
	args := m.Called(ctx, workflowID)
	return args.Error(0)
}

func (m *MockScheduleManager) OnConfigurationReload(ctx context.Context, workflows []*workflow.Config) error {
	args := m.Called(ctx, workflows)
	return args.Error(0)
}

func (m *MockScheduleManager) StartPeriodicReconciliation(
	ctx context.Context,
	getWorkflows func() []*workflow.Config,
	interval time.Duration,
) error {
	args := m.Called(ctx, getWorkflows, interval)
	return args.Error(0)
}

func (m *MockScheduleManager) StopPeriodicReconciliation() {
	m.Called()
}

func setupTest(_ *testing.T) (*gin.Engine, *MockScheduleManager) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// Create mock schedule manager
	mockManager := new(MockScheduleManager)
	// Create app state with mock manager
	state := &appstate.State{
		BaseDeps: appstate.BaseDeps{
			ProjectConfig: &project.Config{
				Name: "test-project",
			},
		},
		Extensions: map[string]any{
			appstate.ScheduleManagerKey: mockManager,
		},
	}
	// Add middleware
	router.Use(appstate.StateMiddleware(state))
	// Register routes
	apiBase := router.Group("/api/v0")
	Register(apiBase)
	return router, mockManager
}

func TestListSchedules(t *testing.T) {
	t.Run("Should list all schedules successfully", func(t *testing.T) {
		router, mockManager := setupTest(t)
		nextRun := time.Now().Add(1 * time.Hour)
		lastRun := time.Now().Add(-1 * time.Hour)
		enabled := true
		schedules := []*schedule.Info{
			{
				WorkflowID:    "workflow-1",
				ScheduleID:    "schedule-test-project-workflow-1",
				Cron:          "0 */5 * * *",
				Timezone:      "UTC",
				Enabled:       true,
				IsOverride:    false,
				NextRunTime:   nextRun,
				LastRunTime:   &lastRun,
				LastRunStatus: "success",
			},
			{
				WorkflowID: "workflow-2",
				ScheduleID: "schedule-test-project-workflow-2",
				Cron:       "0 9 * * 1-5",
				Timezone:   "America/New_York",
				Enabled:    false,
				IsOverride: true,
				YAMLConfig: &workflow.Schedule{
					Cron:    "0 9 * * 1-5",
					Enabled: &enabled,
				},
			},
		}
		mockManager.On("ListSchedules", mock.Anything).Return(schedules, nil)
		// Make request
		req := httptest.NewRequest("GET", "/api/v0/schedules", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		// Verify response
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Status  int                  `json:"status"`
			Message string               `json:"message"`
			Data    ScheduleListResponse `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 200, response.Status)
		assert.Equal(t, "schedules retrieved", response.Message)
		assert.Len(t, response.Data.Schedules, 2)
		assert.Equal(t, 2, response.Data.Total)
		// Verify first schedule
		assert.Equal(t, "workflow-1", response.Data.Schedules[0].WorkflowID)
		assert.NotNil(t, response.Data.Schedules[0].NextRunTime)
		assert.NotNil(t, response.Data.Schedules[0].LastRunTime)
		// Verify second schedule
		assert.Equal(t, "workflow-2", response.Data.Schedules[1].WorkflowID)
		assert.True(t, response.Data.Schedules[1].IsOverride)
		assert.NotNil(t, response.Data.Schedules[1].YAMLConfig)
		mockManager.AssertExpectations(t)
	})
	t.Run("Should handle empty schedule list", func(t *testing.T) {
		router, mockManager := setupTest(t)
		mockManager.On("ListSchedules", mock.Anything).Return([]*schedule.Info{}, nil)
		req := httptest.NewRequest("GET", "/api/v0/schedules", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Data ScheduleListResponse `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response.Data.Schedules, 0)
		assert.Equal(t, 0, response.Data.Total)
		mockManager.AssertExpectations(t)
	})
	t.Run("Should handle list error", func(t *testing.T) {
		router, mockManager := setupTest(t)
		mockManager.On("ListSchedules", mock.Anything).Return(nil, errors.New("failed to connect"))
		req := httptest.NewRequest("GET", "/api/v0/schedules", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response struct {
			Status int `json:"status"`
			Error  struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 500, response.Status)
		assert.Equal(t, "failed to list schedules", response.Error.Message)
		mockManager.AssertExpectations(t)
	})
}

func TestGetSchedule(t *testing.T) {
	t.Run("Should get schedule successfully", func(t *testing.T) {
		router, mockManager := setupTest(t)
		nextRun := time.Now().Add(1 * time.Hour)
		scheduleInfo := &schedule.Info{
			WorkflowID:    "workflow-1",
			ScheduleID:    "schedule-test-project-workflow-1",
			Cron:          "0 */5 * * *",
			Timezone:      "UTC",
			Enabled:       true,
			IsOverride:    false,
			NextRunTime:   nextRun,
			LastRunStatus: "unknown",
		}
		mockManager.On("GetSchedule", mock.Anything, "workflow-1").Return(scheduleInfo, nil)
		req := httptest.NewRequest("GET", "/api/v0/schedules/workflow-1", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Status  int                  `json:"status"`
			Message string               `json:"message"`
			Data    ScheduleInfoResponse `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 200, response.Status)
		assert.Equal(t, "schedule retrieved", response.Message)
		assert.Equal(t, "workflow-1", response.Data.WorkflowID)
		assert.NotNil(t, response.Data.NextRunTime)
		mockManager.AssertExpectations(t)
	})
	t.Run("Should handle schedule not found", func(t *testing.T) {
		router, mockManager := setupTest(t)
		mockManager.On("GetSchedule", mock.Anything, "workflow-999").
			Return(nil, schedule.ErrScheduleNotFound)
		req := httptest.NewRequest("GET", "/api/v0/schedules/workflow-999", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
		var response struct {
			Status int `json:"status"`
			Error  struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 404, response.Status)
		assert.Equal(t, "schedule not found", response.Error.Message)
		mockManager.AssertExpectations(t)
	})
}

func TestUpdateSchedule(t *testing.T) {
	t.Run("Should update schedule successfully", func(t *testing.T) {
		router, mockManager := setupTest(t)
		enabled := false
		updateReq := schedule.UpdateRequest{
			Enabled: &enabled,
		}
		mockManager.On("UpdateSchedule", mock.Anything, "workflow-1", updateReq).Return(nil)
		// Return updated schedule info
		updatedInfo := &schedule.Info{
			WorkflowID:    "workflow-1",
			ScheduleID:    "schedule-test-project-workflow-1",
			Cron:          "0 */5 * * *",
			Timezone:      "UTC",
			Enabled:       false,
			IsOverride:    true,
			LastRunStatus: "unknown",
		}
		mockManager.On("GetSchedule", mock.Anything, "workflow-1").Return(updatedInfo, nil)
		// Create request body
		body := UpdateScheduleRequest{
			Enabled: &enabled,
		}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("PATCH", "/api/v0/schedules/workflow-1", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Status  int                  `json:"status"`
			Message string               `json:"message"`
			Data    ScheduleInfoResponse `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 200, response.Status)
		assert.Equal(t, "schedule updated", response.Message)
		assert.False(t, response.Data.Enabled)
		assert.True(t, response.Data.IsOverride)
		mockManager.AssertExpectations(t)
	})
	t.Run("Should reject request with no fields provided", func(t *testing.T) {
		router, _ := setupTest(t)
		body := map[string]any{}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("PATCH", "/api/v0/schedules/workflow-1", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response struct {
			Status int `json:"status"`
			Error  struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 400, response.Status)
		assert.Equal(t, "at least one of 'enabled' or 'cron' is required", response.Error.Message)
	})
	t.Run("Should update schedule with cron only", func(t *testing.T) {
		router, mockManager := setupTest(t)
		cronValue := "0 */10 * * *"
		updateReq := schedule.UpdateRequest{
			Cron: &cronValue,
		}
		mockManager.On("UpdateSchedule", mock.Anything, "workflow-1", updateReq).Return(nil)
		// Return updated schedule info
		updatedInfo := &schedule.Info{
			WorkflowID:    "workflow-1",
			ScheduleID:    "schedule-test-project-workflow-1",
			Cron:          cronValue,
			Timezone:      "UTC",
			Enabled:       true,
			IsOverride:    true,
			LastRunStatus: "unknown",
		}
		mockManager.On("GetSchedule", mock.Anything, "workflow-1").Return(updatedInfo, nil)
		// Create request body
		body := UpdateScheduleRequest{
			Cron: &cronValue,
		}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("PATCH", "/api/v0/schedules/workflow-1", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Status  int                  `json:"status"`
			Message string               `json:"message"`
			Data    ScheduleInfoResponse `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 200, response.Status)
		assert.Equal(t, "schedule updated", response.Message)
		assert.Equal(t, cronValue, response.Data.Cron)
		assert.True(t, response.Data.IsOverride)
		mockManager.AssertExpectations(t)
	})
	t.Run("Should handle schedule not found", func(t *testing.T) {
		router, mockManager := setupTest(t)
		enabled := false
		updateReq := schedule.UpdateRequest{
			Enabled: &enabled,
		}
		mockManager.On("UpdateSchedule", mock.Anything, "workflow-999", updateReq).
			Return(schedule.ErrScheduleNotFound)
		body := UpdateScheduleRequest{
			Enabled: &enabled,
		}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("PATCH", "/api/v0/schedules/workflow-999", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
		mockManager.AssertExpectations(t)
	})
}

func TestDeleteSchedule(t *testing.T) {
	t.Run("Should delete schedule successfully", func(t *testing.T) {
		router, mockManager := setupTest(t)
		mockManager.On("DeleteSchedule", mock.Anything, "workflow-1").Return(nil)
		req := httptest.NewRequest("DELETE", "/api/v0/schedules/workflow-1", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())
		mockManager.AssertExpectations(t)
	})
	t.Run("Should handle schedule not found", func(t *testing.T) {
		router, mockManager := setupTest(t)
		mockManager.On("DeleteSchedule", mock.Anything, "workflow-999").
			Return(schedule.ErrScheduleNotFound)
		req := httptest.NewRequest("DELETE", "/api/v0/schedules/workflow-999", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
		var response struct {
			Status int `json:"status"`
			Error  struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 404, response.Status)
		assert.Equal(t, "schedule not found", response.Error.Message)
		mockManager.AssertExpectations(t)
	})
	t.Run("Should handle internal error", func(t *testing.T) {
		router, mockManager := setupTest(t)
		mockManager.On("DeleteSchedule", mock.Anything, "workflow-1").
			Return(errors.New("temporal error"))
		req := httptest.NewRequest("DELETE", "/api/v0/schedules/workflow-1", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response struct {
			Status int `json:"status"`
			Error  struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 500, response.Status)
		assert.Equal(t, "failed to delete schedule", response.Error.Message)
		mockManager.AssertExpectations(t)
	})
}
