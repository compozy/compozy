package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/taskgroups"
)

// UT-038: Task Group failures retain their typed public contract and do not leak
// plan or issue filesystem paths.
func TestTaskGroupErrorsUseTypedSafeTransportProblems(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name: "missing task group",
			err: &taskgroups.Error{
				Cause:             taskgroups.ErrTaskGroupNotFound,
				Initiative:        "customer-management",
				TaskGroupID:       "TG-999",
				ValidTaskGroupIDs: []string{"TG-001", "TG-002"},
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "task_group_not_found",
		},
		{
			name: "dependencies unmet",
			err: &taskgroups.Error{
				Cause:       taskgroups.ErrDependenciesUnmet,
				Initiative:  "customer-management",
				TaskGroupID: "TG-002",
				PlanPath:    "/private/workspace/.compozy/tasks/customer-management/_task_groups.md",
				Issues: []taskgroups.Issue{{
					Path:    "/private/workspace/.compozy/tasks/customer-management/_task_groups.md",
					Field:   "depends_on",
					Message: "TG-001 is incomplete",
				}},
			},
			wantStatus: http.StatusConflict,
			wantCode:   "task_group_dependencies_unmet",
		},
		{
			name: "completion conflict",
			err: &taskgroups.Error{
				Cause:       taskgroups.ErrCompletionConflict,
				Initiative:  "customer-management",
				TaskGroupID: "TG-002",
			},
			wantStatus: http.StatusConflict,
			wantCode:   "task_group_completion_conflict",
		},
		{
			name: "invalid plan",
			err: &taskgroups.Error{
				Cause:       taskgroups.ErrInvalidPlan,
				Initiative:  "customer-management",
				TaskGroupID: "TG-002",
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "task_group_plan_invalid",
		},
		{
			name: "selection required",
			err: &taskgroups.Error{
				Cause:             taskgroups.ErrSelectionRequired,
				Initiative:        "customer-management",
				ValidTaskGroupIDs: []string{"TG-001"},
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "task_group_selection_required",
		},
		{
			name: "plan read only",
			err: &taskgroups.Error{
				Cause:       taskgroups.ErrPlanReadOnly,
				Initiative:  "customer-management",
				TaskGroupID: "TG-002",
			},
			wantStatus: http.StatusForbidden,
			wantCode:   "task_group_plan_read_only",
		},
	}

	for _, tc := range testCases {
		t.Run("Should return "+tc.name, func(t *testing.T) {
			t.Parallel()
			engine := newCanonicalHandlersEngine(core.NewHandlers(&core.HandlerConfig{
				TransportName: "test",
				Tasks: &taskGroupErrorTaskService{
					smokeTaskService: &smokeTaskService{},
					err:              tc.err,
				},
			}))
			request := httptest.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				"/api/tasks/customer-management/runs",
				strings.NewReader(`{"workspace":"ws-1","task_group_id":"TG-002"}`),
			)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)

			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, tc.wantStatus, response.Body.String())
			}
			var payload core.TransportError
			decodeJSON(t, response.Body.Bytes(), &payload)
			if payload.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", payload.Code, tc.wantCode)
			}
			if payload.Details["initiative_slug"] != "customer-management" {
				t.Fatalf("initiative_slug = %#v", payload.Details["initiative_slug"])
			}
			if strings.Contains(response.Body.String(), "/private/workspace") {
				t.Fatalf("response leaked a filesystem path: %s", response.Body.String())
			}
		})
	}
}

// IT-060, IT-061, IT-062 and IT-070: child identity is structured transport
// data, while the public task route retains a one-segment initiative slug.
func TestTaskRunRoutesUseStructuredTaskGroupIdentity(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	tasks := &recordingTaskService{
		smokeTaskService: &smokeTaskService{
			run:      core.Run{RunID: "task-run", Mode: "task"},
			multiRun: core.Run{RunID: "multi-run", Mode: "task_multi"},
		},
	}
	engine := newCanonicalHandlersEngine(core.NewHandlers(&core.HandlerConfig{
		TransportName: "test",
		Tasks:         tasks,
	}))

	t.Run("Should accept a single task group body", func(t *testing.T) {
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/api/tasks/customer-management/runs",
			strings.NewReader(`{"workspace":"ws-1","task_group_id":"TG-002","allow_out_of_order":true}`),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusCreated, response.Body.String())
		}
		if tasks.runWorkflow != "customer-management/TG-002" {
			t.Fatalf("workflow = %q, want task group reference", tasks.runWorkflow)
		}
		if tasks.runRequest.TaskGroupID != "TG-002" || !tasks.runRequest.AllowOutOfOrder {
			t.Fatalf("run request = %#v, want task group id and override", tasks.runRequest)
		}
	})

	t.Run("Should accept multiple task group targets", func(t *testing.T) {
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/api/task-runs/multiple",
			strings.NewReader(
				`{"workspace":"ws-1","targets":[{"initiative_slug":"customer-management","task_group_id":"TG-001"},{"initiative_slug":"customer-management","task_group_id":"TG-002"}],"allow_out_of_order":true}`,
			),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusCreated, response.Body.String())
		}
		if len(tasks.multiRequest.Slugs) != 0 || len(tasks.multiRequest.Targets) != 2 {
			t.Fatalf("multiple request = %#v, want two structured targets", tasks.multiRequest)
		}
		if !tasks.multiRequest.AllowOutOfOrder {
			t.Fatalf("multiple request = %#v, want authorization preserved", tasks.multiRequest)
		}
	})

	t.Run("Should reject a slash child route", func(t *testing.T) {
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/api/tasks/customer-management/TG-002/runs",
			http.NoBody,
		)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
		}
	})
}

// UT-038 companion: the published TaskRunTarget schema marks task_group_id required
// with a TG-NNN pattern, so runtime normalization must reject any structured
// target that would satisfy an "optional task_group_id" contract. This keeps the
// generated schema's required fields honest against actual handler acceptance.
func TestStructuredTaskTargetsRejectInvalidTaskGroupID(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	engine := newCanonicalHandlersEngine(core.NewHandlers(&core.HandlerConfig{
		TransportName: "test",
		Tasks:         &smokeTaskService{run: core.Run{RunID: "multi-run", Mode: "task_multi"}},
	}))

	testCases := []struct {
		name     string
		body     string
		wantCode string
	}{
		{
			name:     "missing task group id",
			body:     `{"workspace":"ws-1","targets":[{"initiative_slug":"customer-management"}]}`,
			wantCode: "task_group_selection_required",
		},
		{
			name:     "blank task group id",
			body:     `{"workspace":"ws-1","targets":[{"initiative_slug":"customer-management","task_group_id":"   "}]}`,
			wantCode: "task_group_selection_required",
		},
		{
			name:     "malformed task group id",
			body:     `{"workspace":"ws-1","targets":[{"initiative_slug":"customer-management","task_group_id":"TG-1"}]}`,
			wantCode: "task_group_invalid_reference",
		},
	}

	for _, tc := range testCases {
		t.Run("Should reject "+tc.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/api/task-runs/multiple",
				strings.NewReader(tc.body),
			)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)

			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf(
					"status = %d, want %d; body=%s",
					response.Code,
					http.StatusUnprocessableEntity,
					response.Body.String(),
				)
			}
			var payload core.TransportError
			decodeJSON(t, response.Body.Bytes(), &payload)
			if payload.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q; body=%s", payload.Code, tc.wantCode, response.Body.String())
			}
		})
	}
}

type taskGroupErrorTaskService struct {
	*smokeTaskService
	err error
}

func (s *taskGroupErrorTaskService) StartRun(context.Context, string, string, core.TaskRunRequest) (core.Run, error) {
	return core.Run{}, s.err
}

type recordingTaskService struct {
	*smokeTaskService
	runWorkflow  string
	runRequest   core.TaskRunRequest
	multiRequest core.TaskRunMultipleRequest
}

func (s *recordingTaskService) StartRun(
	_ context.Context,
	_ string,
	workflow string,
	req core.TaskRunRequest,
) (core.Run, error) {
	s.runWorkflow = workflow
	s.runRequest = req
	return s.run, nil
}

func (s *recordingTaskService) StartRunMultiple(
	_ context.Context,
	_ string,
	req core.TaskRunMultipleRequest,
) (core.Run, error) {
	s.multiRequest = req
	return s.multiRun, nil
}
