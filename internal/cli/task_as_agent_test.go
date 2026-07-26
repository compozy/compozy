package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestUnixSocketClientCreateTaskAsAgentSendsIdentityHeaders(t *testing.T) {
	t.Parallel()

	t.Run("Should send validated identity headers to task create endpoint", func(t *testing.T) {
		t.Parallel()

		credentials := agentidentity.Credentials{
			SessionID:   "sess-1",
			AgentName:   "coder",
			WorkspaceID: "ws-1",
		}
		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					assertAgentRequestHeaders(t, req, credentials)
					if req.Method != http.MethodPost || req.URL.Path != "/api/tasks" {
						t.Fatalf("request = %s %s, want POST /api/tasks", req.Method, req.URL.Path)
					}
					var payload contract.CreateTaskRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(create task body) error = %v", err)
					}
					if payload.Title != "Agent task" ||
						payload.Scope != taskpkg.ScopeWorkspace ||
						payload.Workspace != "alpha" {
						t.Fatalf("create task payload = %#v", payload)
					}
					body := mustJSON(t, contract.TaskResponse{Task: sampleTaskRecord()})
					return newHTTPResponse(http.StatusCreated, string(body)), nil
				}),
			},
		}

		created, err := client.CreateTaskAsAgent(context.Background(), CreateTaskRequest{
			Scope:     taskpkg.ScopeWorkspace,
			Workspace: "alpha",
			Title:     "Agent task",
		}, credentials)
		if err != nil {
			t.Fatalf("CreateTaskAsAgent() error = %v", err)
		}
		if created.ID == "" {
			t.Fatalf("CreateTaskAsAgent() = %#v, want created task", created)
		}
	})
}

func TestUnixSocketClientTaskReviewAsAgentSendsIdentityHeaders(t *testing.T) {
	t.Parallel()

	t.Run("Should send validated identity headers to review request endpoint", func(t *testing.T) {
		t.Parallel()

		credentials := agentidentity.Credentials{
			SessionID:   "sess-1",
			AgentName:   "coder",
			WorkspaceID: "ws-1",
		}
		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					assertAgentRequestHeaders(t, req, credentials)
					if req.Method != http.MethodPost || req.URL.Path != "/api/task-runs/run-1/reviews" {
						t.Fatalf("request = %s %s, want POST /api/task-runs/run-1/reviews", req.Method, req.URL.Path)
					}
					var payload contract.CreateTaskRunReviewRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(review request body) error = %v", err)
					}
					if payload.RunID != "run-1" || payload.Reason != "ready" {
						t.Fatalf("review request payload = %#v", payload)
					}
					body := mustJSON(t, contract.TaskRunReviewRequestResponse{
						Review:  sampleTaskRunReviewRecord(taskpkg.RunReviewStatusRequested),
						Created: true,
					})
					return newHTTPResponse(http.StatusCreated, string(body)), nil
				}),
			},
		}

		review, err := client.RequestTaskRunReviewAsAgent(context.Background(), "run-1", &TaskRunReviewRequest{
			RunID:  "run-1",
			Reason: "ready",
		}, credentials)
		if err != nil {
			t.Fatalf("RequestTaskRunReviewAsAgent() error = %v", err)
		}
		if !review.Created || review.Review.ReviewID == "" {
			t.Fatalf("RequestTaskRunReviewAsAgent() = %#v, want created review", review)
		}
	})

	t.Run("Should send validated identity headers to review verdict endpoint", func(t *testing.T) {
		t.Parallel()

		credentials := agentidentity.Credentials{
			SessionID:   "sess-1",
			AgentName:   "coder",
			WorkspaceID: "ws-1",
		}
		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					assertAgentRequestHeaders(t, req, credentials)
					if req.Method != http.MethodPost || req.URL.Path != "/api/task-reviews/review-1/verdict" {
						t.Fatalf(
							"request = %s %s, want POST /api/task-reviews/review-1/verdict",
							req.Method,
							req.URL.Path,
						)
					}
					var payload contract.SubmitTaskRunReviewVerdictRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("json.Decode(review verdict body) error = %v", err)
					}
					if payload.RunID != "run-1" || payload.Verdict.Outcome != taskpkg.RunReviewOutcomeApproved {
						t.Fatalf("review verdict payload = %#v", payload)
					}
					review := sampleTaskRunReviewRecord(taskpkg.RunReviewStatusRecorded)
					review.Outcome = taskpkg.RunReviewOutcomeApproved
					body := mustJSON(t, contract.TaskRunReviewVerdictResponse{Review: review})
					return newHTTPResponse(http.StatusOK, string(body)), nil
				}),
			},
		}

		confidence := 0.8
		verdict, err := client.SubmitTaskRunReviewVerdictAsAgent(
			context.Background(),
			"review-1",
			&TaskRunReviewVerdictRequest{
				RunID: "run-1",
				Verdict: taskpkg.RunReviewVerdict{
					Outcome:    taskpkg.RunReviewOutcomeApproved,
					Confidence: &confidence,
					Reason:     "ship it",
					DeliveryID: "delivery-1",
				},
			},
			credentials,
		)
		if err != nil {
			t.Fatalf("SubmitTaskRunReviewVerdictAsAgent() error = %v", err)
		}
		if verdict.Review.Outcome != taskpkg.RunReviewOutcomeApproved {
			t.Fatalf("SubmitTaskRunReviewVerdictAsAgent() = %#v, want approved verdict", verdict)
		}
	})
}

func TestTaskCreateAsAgentUsesValidatedIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should send agent identity for explicit task create", func(t *testing.T) {
		t.Parallel()

		var gotRequest CreateTaskRequest
		deps := newTestDeps(t, &stubClient{
			getSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
				if id != "sess-agent" {
					t.Fatalf("GetSession id = %q, want sess-agent", id)
				}
				return agentCommandSessionRecord(), nil
			},
			createTaskFn: func(context.Context, CreateTaskRequest) (TaskRecord, error) {
				t.Fatal("CreateTask should not be called for --as-agent")
				return TaskRecord{}, nil
			},
			createTaskAsAgentFn: func(
				_ context.Context,
				request CreateTaskRequest,
				credentials agentidentity.Credentials,
			) (TaskRecord, error) {
				assertAgentCredentials(t, credentials)
				gotRequest = request
				return TaskRecord{
					ID:        "task-agent",
					Title:     request.Title,
					Scope:     taskpkg.ScopeWorkspace,
					CreatedAt: fixedTestNow,
					UpdatedAt: fixedTestNow,
				}, nil
			},
		})
		deps.getenv = agentCommandEnv

		if _, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"create",
			"--as-agent",
			"--scope",
			"workspace",
			"--workspace",
			"alpha",
			"--title",
			"Agent task",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("executeRootCommand(task create --as-agent) error = %v", err)
		}
		if gotRequest.Workspace != "alpha" || gotRequest.Title != "Agent task" {
			t.Fatalf("CreateTaskAsAgent request = %#v", gotRequest)
		}
	})
}

func TestTaskReviewAsAgentUsesValidatedIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should request and submit reviews with agent identity", func(t *testing.T) {
		t.Parallel()

		var (
			requestedRunID string
			submittedID    string
		)
		deps := newAgentCommandTestDeps(t, &stubClient{
			requestTaskRunReviewFn: func(context.Context, string, *TaskRunReviewRequest) (TaskRunReviewRequestRecord, error) {
				t.Fatal("RequestTaskRunReview should not be called for --as-agent")
				return TaskRunReviewRequestRecord{}, nil
			},
			requestTaskRunReviewAsAgentFn: func(
				_ context.Context,
				runID string,
				request *TaskRunReviewRequest,
				credentials agentidentity.Credentials,
			) (TaskRunReviewRequestRecord, error) {
				assertAgentCredentials(t, credentials)
				if request == nil {
					t.Fatal("review request is nil")
					return TaskRunReviewRequestRecord{}, nil
				}
				requestedRunID = runID
				if request.RunID != "run-1" || request.Reason != "ready for review" {
					t.Fatalf("review request = %#v", request)
				}
				return TaskRunReviewRequestRecord{
					Review:  sampleTaskRunReviewRecord(taskpkg.RunReviewStatusRequested),
					Created: true,
				}, nil
			},
			submitTaskRunReviewVerdictFn: func(
				context.Context,
				string,
				*TaskRunReviewVerdictRequest,
			) (TaskRunReviewVerdictRecord, error) {
				t.Fatal("SubmitTaskRunReviewVerdict should not be called for --as-agent")
				return TaskRunReviewVerdictRecord{}, nil
			},
			submitTaskRunReviewVerdictAsAgentFn: func(
				_ context.Context,
				reviewID string,
				request *TaskRunReviewVerdictRequest,
				credentials agentidentity.Credentials,
			) (TaskRunReviewVerdictRecord, error) {
				assertAgentCredentials(t, credentials)
				if request == nil {
					t.Fatal("review verdict request is nil")
					return TaskRunReviewVerdictRecord{}, nil
				}
				submittedID = reviewID
				if request.RunID != "run-1" ||
					request.Verdict.Outcome != taskpkg.RunReviewOutcomeApproved ||
					request.Verdict.DeliveryID != "delivery-1" {
					t.Fatalf("review verdict request = %#v", request)
				}
				review := sampleTaskRunReviewRecord(taskpkg.RunReviewStatusRecorded)
				review.Outcome = taskpkg.RunReviewOutcomeApproved
				return TaskRunReviewVerdictRecord{Review: review}, nil
			},
		})

		if _, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"review",
			"request",
			"run-1",
			"--as-agent",
			"--reason",
			"ready for review",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("task review request --as-agent error = %v", err)
		}
		if requestedRunID != "run-1" {
			t.Fatalf("requested run id = %q, want run-1", requestedRunID)
		}

		if _, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"review",
			"submit",
			"review-1",
			"--as-agent",
			"--run",
			"run-1",
			"--outcome",
			"approved",
			"--confidence",
			"0.8",
			"--reason",
			"looks good",
			"--delivery-id",
			"delivery-1",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("task review submit --as-agent error = %v", err)
		}
		if submittedID != "review-1" {
			t.Fatalf("submitted review id = %q, want review-1", submittedID)
		}
	})
}
