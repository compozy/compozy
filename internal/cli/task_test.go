package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestTaskCreateAndUpdateRejectInvalidFlagCombos(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "Should require workspace for workspace scope",
			args:    []string{"task", "create", "--scope", "workspace", "--title", "Investigate"},
			wantErr: "--workspace is required when --scope is workspace",
		},
		{
			name:    "Should forbid workspace for global scope",
			args:    []string{"task", "create", "--scope", "global", "--workspace", "alpha", "--title", "Investigate"},
			wantErr: "--workspace must be empty when --scope is global",
		},
		{
			name:    "Should require change flags on update",
			args:    []string{"task", "update", "task-1"},
			wantErr: "task update requires at least one change flag",
		},
		{
			name: "Should reject clear owner with owner mutation",
			args: []string{
				"task",
				"update",
				"task-1",
				"--clear-owner",
				"--owner-kind",
				"pool",
				"--owner-ref",
				"triage",
			},
			wantErr: "--clear-owner cannot be combined with --owner-kind or --owner-ref",
		},
		{
			name: "Should reject unknown network bounds fields",
			args: []string{
				"task", "create", "--scope", "global", "--title", "Investigate",
				"--network", "live", "--network-channel-strategy", "named",
				"--network-channel", "builders", "--network-bounds", `{"max_waks":1}`,
			},
			wantErr: `json: unknown field "max_waks"`,
		},
		{
			name: "Should reject trailing network bounds values",
			args: []string{
				"task", "create", "--scope", "global", "--title", "Investigate",
				"--network", "live", "--network-channel-strategy", "named",
				"--network-channel", "builders", "--network-bounds", `{"max_wakes":1} {}`,
			},
			wantErr: "expected exactly one JSON object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := executeRootCommand(t, newTestDeps(t, &stubClient{}), tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("executeRootCommand(%v) error = %v, want %q", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestTaskInspectCommandMapsTargets(t *testing.T) {
	t.Parallel()

	t.Run("Should inspect task ids through the task inspect client", func(t *testing.T) {
		t.Parallel()

		var gotID string
		stdout, _, err := executeRootCommand(t, newTestDeps(t, &stubClient{
			inspectTaskFn: func(_ context.Context, id string) (TaskInspectRecord, error) {
				gotID = id
				return sampleTaskInspectRecord("task"), nil
			},
		}), "task", "inspect", "task-1", "-o", "json")
		if err != nil {
			t.Fatalf("task inspect task id error = %v", err)
		}
		var payload TaskInspectRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("decode task inspect output: %v", err)
		}
		if gotID != "task-1" || payload.Target != "task" || payload.NextAction != "stranded" {
			t.Fatalf("gotID/payload = %q / %#v", gotID, payload)
		}
	})

	t.Run("Should inspect run ids through the run inspect client", func(t *testing.T) {
		t.Parallel()

		var gotID string
		stdout, _, err := executeRootCommand(t, newTestDeps(t, &stubClient{
			inspectRunFn: func(_ context.Context, id string) (TaskInspectRecord, error) {
				gotID = id
				return sampleTaskInspectRecord("run"), nil
			},
		}), "task", "inspect", "run-1", "-o", "json")
		if err != nil {
			t.Fatalf("task inspect run id error = %v", err)
		}
		var payload TaskInspectRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("decode run inspect output: %v", err)
		}
		if gotID != "run-1" || payload.Target != "run" || payload.CurrentRun == nil {
			t.Fatalf("gotID/payload = %q / %#v", gotID, payload)
		}
	})

	t.Run("Should render id format diagnostic without calling the daemon for unknown ids", func(t *testing.T) {
		t.Parallel()

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, &stubClient{}),
			"task",
			"inspect",
			"unknown-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task inspect unknown id error = %v", err)
		}
		var payload TaskInspectRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("decode unknown inspect output: %v", err)
		}
		if payload.Target != "unknown" || len(payload.Diagnostics) != 1 ||
			payload.Diagnostics[0].Code != contract.CodeIDFormatUnknown {
			t.Fatalf("unknown inspect payload = %#v", payload)
		}
	})
}

func TestTaskCreateRemainsOperatorExplicitWithAgentEnv(t *testing.T) {
	t.Parallel()

	t.Run("Should keep operator task create explicit with agent env", func(t *testing.T) {
		t.Parallel()

		var gotRequest CreateTaskRequest
		deps := newTestDeps(t, &stubClient{
			createTaskFn: func(_ context.Context, request CreateTaskRequest) (TaskRecord, error) {
				gotRequest = request
				return TaskRecord{
					ID:    "task-1",
					Title: request.Title,
					Scope: taskpkg.ScopeWorkspace,
				}, nil
			},
		})
		deps.getenv = func(key string) string {
			switch key {
			case agentidentity.EnvSessionID:
				return "agent-session"
			case agentidentity.EnvAgent:
				return "coder"
			default:
				return ""
			}
		}

		if _, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"create",
			"--scope",
			"workspace",
			"--workspace",
			"alpha",
			"--title",
			"Manual task",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("executeRootCommand(task create) error = %v", err)
		}
		if gotRequest.Workspace != "alpha" {
			t.Fatalf("Workspace = %q, want explicit workspace alpha", gotRequest.Workspace)
		}
	})
}

func TestTaskCreateAndListCommandsParseTaskFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Should parse task create fields",
			run: func(t *testing.T) {
				t.Helper()

				var createRequest CreateTaskRequest
				deps := newTestDeps(t, &stubClient{
					createTaskFn: func(_ context.Context, got CreateTaskRequest) (TaskRecord, error) {
						createRequest = got
						return sampleTaskRecord(), nil
					},
				})

				createJSON, _, err := executeRootCommand(
					t,
					deps,
					"task", "create",
					"--id", "task-1",
					"--identifier", "OPS-42",
					"--scope", "workspace",
					"--workspace", "alpha",
					"--network", "live",
					"--network-channel-strategy", "named",
					"--network-channel", "builders",
					"--title", "Investigate flaky task runs",
					"--description", "Capture root cause",
					"--priority", "high",
					"--owner-kind", "pool",
					"--owner-ref", "triage",
					"--metadata", `{"source":"qa"}`,
					"-o", "json",
				)
				if err != nil {
					t.Fatalf("task create error = %v", err)
				}

				assertLiveNamedParticipationRequest(t, createRequest.NetworkParticipation, "builders")
				if createRequest.Scope != taskpkg.ScopeWorkspace ||
					createRequest.Workspace != "alpha" ||
					createRequest.Title != "Investigate flaky task runs" ||
					createRequest.Priority != taskpkg.PriorityHigh ||
					createRequest.Owner == nil ||
					createRequest.Owner.Kind != taskpkg.OwnerKindPool ||
					createRequest.Owner.Ref != "triage" ||
					string(createRequest.Metadata) != `{"source":"qa"}` {
					t.Fatalf("createRequest = %#v, want parsed workspace/channel/owner/metadata", createRequest)
				}

				var created TaskRecord
				if err := json.Unmarshal([]byte(createJSON), &created); err != nil {
					t.Fatalf("json.Unmarshal(task create) error = %v", err)
				}
				if created.ID != "task-1" || created.Title != "Investigate flaky task runs" {
					t.Fatalf("created task = %#v, want sample task output", created)
				}
			},
		},
		{
			name: "Should parse task create wake creator opt-out",
			run: func(t *testing.T) {
				t.Helper()

				var createRequest CreateTaskRequest
				deps := newTestDeps(t, &stubClient{
					createTaskFn: func(_ context.Context, got CreateTaskRequest) (TaskRecord, error) {
						createRequest = got
						return sampleTaskRecord(), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task", "create",
					"--scope", "global",
					"--title", "Quiet task",
					"--no-wake-creator",
					"-o", "json",
				); err != nil {
					t.Fatalf("task create --no-wake-creator error = %v", err)
				}
				if createRequest.WakeCreator == nil || *createRequest.WakeCreator {
					t.Fatalf("WakeCreator = %#v, want explicit false", createRequest.WakeCreator)
				}
			},
		},
		{
			name: "Should parse task list filters",
			run: func(t *testing.T) {
				t.Helper()

				var listQuery TaskListQuery
				deps := newTestDeps(t, &stubClient{
					listTasksFn: func(_ context.Context, query TaskListQuery) ([]TaskCatalogItemRecord, error) {
						listQuery = query
						return []TaskCatalogItemRecord{sampleTaskCatalogItemRecord()}, nil
					},
				})

				listJSON, _, err := executeRootCommand(
					t,
					deps,
					"task", "list",
					"--scope", "workspace",
					"--workspace", "alpha",
					"--status", "ready",
					"--owner-kind", "pool",
					"--owner-ref", "triage",
					"--parent", "task-root",
					"--participation-channel", "builders",
					"--limit", "3",
					"-o", "json",
				)
				if err != nil {
					t.Fatalf("task list error = %v", err)
				}

				if listQuery.Scope != taskpkg.CatalogScopeWorkspace ||
					listQuery.Workspace != "alpha" ||
					listQuery.Status != taskpkg.TaskStatusReady ||
					listQuery.OwnerKind != taskpkg.OwnerKindPool ||
					listQuery.OwnerRef != "triage" ||
					listQuery.ParentTaskID != "task-root" ||
					listQuery.ParticipationChannel != "builders" ||
					listQuery.Limit != 3 {
					t.Fatalf("listQuery = %#v, want parsed filters", listQuery)
				}

				var listed TaskListRecord
				if err := json.Unmarshal([]byte(listJSON), &listed); err != nil {
					t.Fatalf("json.Unmarshal(task list) error = %v", err)
				}
				if len(listed.Tasks) != 1 || listed.Tasks[0].ID != "task-1" {
					t.Fatalf("listed tasks = %#v, want one task summary", listed)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func TestTaskBlockCommandsMapRequests(t *testing.T) {
	t.Parallel()

	t.Run("Should parse block request and render stable JSON", func(t *testing.T) {
		t.Parallel()

		var (
			taskID       string
			blockRequest CreateTaskBlockRequest
		)
		deps := newTestDeps(t, &stubClient{
			blockTaskFn: func(_ context.Context, id string, request CreateTaskBlockRequest) (TaskBlockRecord, error) {
				taskID = id
				blockRequest = request
				return sampleTaskBlockRecord(), nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"block",
			"task-1",
			"--kind",
			"transient",
			"--reason",
			"waiting for quota",
			"--details",
			`{"queue":"gpu"}`,
			"--expires-in",
			"30m",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task block error = %v", err)
		}
		if taskID != "task-1" ||
			blockRequest.Kind != taskpkg.BlockKindTransient ||
			blockRequest.Reason != "waiting for quota" ||
			blockRequest.RunID != "" ||
			string(blockRequest.Details) != `{"queue":"gpu"}` ||
			blockRequest.ExpiresAt == nil ||
			!blockRequest.ExpiresAt.Equal(fixedTestNow.Add(30*time.Minute).UTC()) {
			t.Fatalf("block request task=%q request=%#v, want parsed transient block", taskID, blockRequest)
		}
		var output TaskBlockRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(task block) error = %v", err)
		}
		if output.ID != "block-1" || output.TaskID != "task-1" {
			t.Fatalf("task block output = %#v, want block-1/task-1", output)
		}
	})

	t.Run("Should reject run id without agent identity before calling the daemon", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			blockTaskFn: func(context.Context, string, CreateTaskBlockRequest) (TaskBlockRecord, error) {
				t.Fatal("BlockTask should not be called when --run-id lacks --as-agent")
				return TaskBlockRecord{}, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"block",
			"task-1",
			"--kind",
			"needs_input",
			"--reason",
			"creator input",
			"--run-id",
			"run-1",
			"-o",
			"json",
		)
		if err == nil || !strings.Contains(err.Error(), "--run-id requires --as-agent") {
			t.Fatalf("task block --run-id without --as-agent error = %v, want local validation", err)
		}
	})

	t.Run("Should block as agent with validated identity and run id", func(t *testing.T) {
		t.Parallel()

		var (
			taskID       string
			blockRequest CreateTaskBlockRequest
		)
		deps := newTestDeps(t, &stubClient{
			getSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
				if id != "sess-agent" {
					t.Fatalf("GetSession id = %q, want sess-agent", id)
				}
				return agentCommandSessionRecord(), nil
			},
			blockTaskFn: func(context.Context, string, CreateTaskBlockRequest) (TaskBlockRecord, error) {
				t.Fatal("BlockTask should not be called for --as-agent")
				return TaskBlockRecord{}, nil
			},
			blockTaskAsAgentFn: func(
				_ context.Context,
				id string,
				request CreateTaskBlockRequest,
				credentials agentidentity.Credentials,
			) (TaskBlockRecord, error) {
				assertAgentCredentials(t, credentials)
				taskID = id
				blockRequest = request
				return sampleTaskBlockRecord(), nil
			},
		})
		deps.getenv = agentCommandEnv

		if _, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"block",
			"task-1",
			"--as-agent",
			"--kind",
			"needs_input",
			"--reason",
			"creator input",
			"--run-id",
			"run-1",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("task block --as-agent error = %v", err)
		}
		if taskID != "task-1" || blockRequest.RunID != "run-1" ||
			blockRequest.Kind != taskpkg.BlockKindNeedsInput ||
			blockRequest.Reason != "creator input" {
			t.Fatalf("BlockTaskAsAgent task=%q request=%#v, want leased block request", taskID, blockRequest)
		}
	})

	t.Run("Should list blocks with cleared rows when requested", func(t *testing.T) {
		t.Parallel()

		var (
			taskID         string
			includeCleared bool
		)
		deps := newTestDeps(t, &stubClient{
			listTaskBlocksFn: func(_ context.Context, id string, include bool) ([]TaskBlockRecord, error) {
				taskID = id
				includeCleared = include
				return []TaskBlockRecord{sampleTaskBlockRecord()}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "task", "blocks", "task-1", "--all", "-o", "json")
		if err != nil {
			t.Fatalf("task blocks error = %v", err)
		}
		if taskID != "task-1" || !includeCleared {
			t.Fatalf("ListTaskBlocks task/include = %q/%v, want task-1/true", taskID, includeCleared)
		}
		var output []TaskBlockRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(task blocks) error = %v", err)
		}
		if len(output) != 1 || output[0].ID != "block-1" {
			t.Fatalf("task blocks output = %#v, want block-1", output)
		}
	})

	t.Run("Should clear a task block", func(t *testing.T) {
		t.Parallel()

		var (
			taskID  string
			blockID string
			request ClearTaskBlockRequest
		)
		deps := newTestDeps(t, &stubClient{
			clearTaskBlockFn: func(
				_ context.Context,
				id string,
				gotBlockID string,
				got ClearTaskBlockRequest,
			) (TaskBlockRecord, error) {
				taskID = id
				blockID = gotBlockID
				request = got
				block := sampleTaskBlockRecord()
				clearedAt := fixedTestNow
				block.ClearedAt = &clearedAt
				block.ClearNote = got.Note
				return block, nil
			},
		})

		if _, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"unblock",
			"task-1",
			"--block",
			"block-1",
			"--note",
			"resolved",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("task unblock error = %v", err)
		}
		if taskID != "task-1" || blockID != "block-1" || request.Note != "resolved" {
			t.Fatalf("ClearTaskBlock task/block/request = %q/%q/%#v, want resolved block", taskID, blockID, request)
		}
	})

	t.Run("Should clear a task block as agent with validated identity", func(t *testing.T) {
		t.Parallel()

		var (
			taskID  string
			blockID string
			request ClearTaskBlockRequest
		)
		deps := newTestDeps(t, &stubClient{
			getSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
				if id != "sess-agent" {
					t.Fatalf("GetSession id = %q, want sess-agent", id)
				}
				return agentCommandSessionRecord(), nil
			},
			clearTaskBlockFn: func(context.Context, string, string, ClearTaskBlockRequest) (TaskBlockRecord, error) {
				t.Fatal("ClearTaskBlock should not be called for --as-agent")
				return TaskBlockRecord{}, nil
			},
			clearTaskBlockAsAgentFn: func(
				_ context.Context,
				id string,
				gotBlockID string,
				got ClearTaskBlockRequest,
				credentials agentidentity.Credentials,
			) (TaskBlockRecord, error) {
				assertAgentCredentials(t, credentials)
				taskID = id
				blockID = gotBlockID
				request = got
				block := sampleTaskBlockRecord()
				clearedAt := fixedTestNow
				block.ClearedAt = &clearedAt
				block.ClearNote = got.Note
				return block, nil
			},
		})
		deps.getenv = agentCommandEnv

		if _, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"unblock",
			"task-1",
			"--as-agent",
			"--block",
			"block-1",
			"--note",
			"agent resolved",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("task unblock --as-agent error = %v", err)
		}
		if taskID != "task-1" || blockID != "block-1" || request.Note != "agent resolved" {
			t.Fatalf(
				"ClearTaskBlockAsAgent task/block/request = %q/%q/%#v, want agent resolved block",
				taskID,
				blockID,
				request,
			)
		}
	})

	t.Run("Should recover a needs_attention task", func(t *testing.T) {
		t.Parallel()

		var (
			taskID  string
			request RecoverTaskRequest
		)
		deps := newTestDeps(t, &stubClient{
			recoverTaskFn: func(_ context.Context, id string, got RecoverTaskRequest) (TaskRecord, error) {
				taskID = id
				request = got
				return sampleTaskRecord(), nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"recover",
			"task-1",
			"--note",
			"operator reviewed",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task recover error = %v", err)
		}
		if taskID != "task-1" || request.Note != "operator reviewed" {
			t.Fatalf("RecoverTask task/request = %q/%#v, want operator note", taskID, request)
		}
		var output TaskRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(task recover) error = %v", err)
		}
		if output.ID != "task-1" {
			t.Fatalf("task recover output = %#v, want task-1", output)
		}
	})
}

func TestTaskExecutionCommandsMapBoundaryRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		configure  func(*stubClient, *TaskExecutionRequest)
		wantAction string
	}{
		{
			name: "Should map task publish request",
			args: []string{
				"task",
				"publish",
				"task-1",
				"--idempotency-key",
				"idem-publish",
				"--network", "live",
				"--network-channel-strategy", "named",
				"--network-channel", "builders",
				"-o",
				"json",
			},
			wantAction: "publish",
			configure: func(client *stubClient, request *TaskExecutionRequest) {
				client.publishTaskFn = func(
					_ context.Context,
					_ string,
					got TaskExecutionRequest,
				) (TaskExecutionRecord, error) {
					*request = got
					return sampleTaskExecutionRecord(), nil
				}
			},
		},
		{
			name: "Should map task start request",
			args: []string{
				"task",
				"start",
				"task-1",
				"--idempotency-key",
				"idem-start",
				"--network", "live",
				"--network-channel-strategy", "named",
				"--network-channel", "builders",
				"-o",
				"json",
			},
			wantAction: "start",
			configure: func(client *stubClient, request *TaskExecutionRequest) {
				client.startTaskFn = func(
					_ context.Context,
					_ string,
					got TaskExecutionRequest,
				) (TaskExecutionRecord, error) {
					*request = got
					return sampleTaskExecutionRecord(), nil
				}
			},
		},
		{
			name: "Should map task approve request",
			args: []string{
				"task",
				"approve",
				"task-1",
				"--idempotency-key",
				"idem-approve",
				"--network", "live",
				"--network-channel-strategy", "named",
				"--network-channel", "builders",
				"-o",
				"json",
			},
			wantAction: "approve",
			configure: func(client *stubClient, request *TaskExecutionRequest) {
				client.approveTaskFn = func(
					_ context.Context,
					_ string,
					got TaskExecutionRequest,
				) (TaskExecutionRecord, error) {
					*request = got
					return sampleTaskExecutionRecord(), nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var request TaskExecutionRequest
			client := &stubClient{}
			tt.configure(client, &request)
			if _, _, err := executeRootCommand(t, newTestDeps(t, client), tt.args...); err != nil {
				t.Fatalf("task %s error = %v", tt.wantAction, err)
			}
			assertLiveNamedParticipationRequest(t, request.NetworkParticipation, "builders")
			if request.IdempotencyKey != "idem-"+tt.wantAction {
				t.Fatalf("request = %#v, want idempotency key for %s", request, tt.wantAction)
			}
		})
	}
}

func TestTaskRunCommandsMapLifecycleRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Should parse task run list filters",
			run: func(t *testing.T) {
				t.Helper()

				var runListQuery TaskRunListQuery
				deps := newTestDeps(t, &stubClient{
					listTaskRunsFn: func(_ context.Context, _ string, query TaskRunListQuery) ([]TaskRunRecord, error) {
						runListQuery = query
						return []TaskRunRecord{sampleTaskRunRecord(taskpkg.TaskRunStatusRunning)}, nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task",
					"run",
					"list",
					"task-1",
					"--status",
					"running",
					"--session",
					"sess-1",
					"--participation-channel",
					"builders",
					"--last",
					"2",
					"-o",
					"json",
				); err != nil {
					t.Fatalf("task run list error = %v", err)
				}
				if runListQuery.Status != taskpkg.TaskRunStatusRunning ||
					runListQuery.SessionID != "sess-1" ||
					runListQuery.ParticipationChannel != "builders" ||
					runListQuery.Limit != 2 {
					t.Fatalf("runListQuery = %#v, want parsed run filters", runListQuery)
				}
			},
		},
		{
			name: "Should show task run",
			run: func(t *testing.T) {
				t.Helper()

				var gotID string
				deps := newTestDeps(t, &stubClient{
					getTaskRunFn: func(_ context.Context, id string) (TaskRunDetailRecord, error) {
						gotID = id
						return sampleTaskRunDetailRecord(taskpkg.TaskRunStatusQueued), nil
					},
				})

				stdout, _, err := executeRootCommand(t, deps, "task", "run", "show", "run-1", "-o", "json")
				if err != nil {
					t.Fatalf("task run show error = %v", err)
				}
				var output TaskRunDetailRecord
				if err := json.Unmarshal([]byte(stdout), &output); err != nil {
					t.Fatalf("json.Unmarshal(task run show) error = %v", err)
				}
				if gotID != "run-1" ||
					output.Run.ID != "run-1" ||
					output.Run.Status != taskpkg.TaskRunStatusQueued ||
					output.Task == nil || output.Task.ID != "task-1" {
					t.Fatalf("gotID/output = %q / %#v, want queued run-1", gotID, output)
				}
			},
		},
		{
			name: "Should parse task run enqueue request",
			run: func(t *testing.T) {
				t.Helper()

				var enqueueRequest EnqueueTaskRunRequest
				deps := newTestDeps(t, &stubClient{
					enqueueTaskRunFn: func(_ context.Context, _ string, request EnqueueTaskRunRequest) (TaskRunRecord, error) {
						enqueueRequest = request
						return sampleTaskRunRecord(taskpkg.TaskRunStatusQueued), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task",
					"run",
					"enqueue",
					"task-1",
					"--idempotency-key",
					"idem-1",
					"--network", "live",
					"--network-channel-strategy", "named",
					"--network-channel", "builders",
					"--metadata",
					`{"schema":"agh.harness.detached.v1"}`,
					"-o",
					"json",
				); err != nil {
					t.Fatalf("task run enqueue error = %v", err)
				}
				assertLiveNamedParticipationRequest(t, enqueueRequest.NetworkParticipation, "builders")
				if enqueueRequest.IdempotencyKey != "idem-1" {
					t.Fatalf("enqueueRequest = %#v, want idempotency key", enqueueRequest)
				}
				if got, want := string(enqueueRequest.Metadata), `{"schema":"agh.harness.detached.v1"}`; got != want {
					t.Fatalf("enqueueRequest.Metadata = %q, want %q", got, want)
				}
			},
		},
		{
			name: "Should parse task run start request",
			run: func(t *testing.T) {
				t.Helper()

				var startRequest StartTaskRunRequest
				deps := newTestDeps(t, &stubClient{
					startTaskRunFn: func(_ context.Context, _ string, request StartTaskRunRequest) (TaskRunRecord, error) {
						startRequest = request
						return sampleTaskRunRecord(taskpkg.TaskRunStatusRunning), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task",
					"run",
					"start",
					"run-1",
					"--idempotency-key",
					"idem-start",
					"-o",
					"json",
				); err != nil {
					t.Fatalf("task run start error = %v", err)
				}
				if startRequest.IdempotencyKey != "idem-start" {
					t.Fatalf("startRequest = %#v, want idempotency key", startRequest)
				}
			},
		},
		{
			name: "Should parse task run attach-session request",
			run: func(t *testing.T) {
				t.Helper()

				var attachRequest AttachTaskRunSessionRequest
				deps := newTestDeps(t, &stubClient{
					attachTaskRunSessionFn: func(_ context.Context, _ string, request AttachTaskRunSessionRequest) (TaskRunRecord, error) {
						attachRequest = request
						return sampleTaskRunRecord(taskpkg.TaskRunStatusStarting), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task",
					"run",
					"attach-session",
					"run-1",
					"--session",
					"sess-attach",
					"-o",
					"json",
				); err != nil {
					t.Fatalf("task run attach-session error = %v", err)
				}
				if attachRequest.SessionID != "sess-attach" {
					t.Fatalf("attachRequest = %#v, want session id", attachRequest)
				}
			},
		},
		{
			name: "Should parse task run complete request",
			run: func(t *testing.T) {
				t.Helper()

				var completeRequest CompleteTaskRunRequest
				deps := newTestDeps(t, &stubClient{
					completeTaskRunFn: func(_ context.Context, _ string, request CompleteTaskRunRequest) (TaskRunRecord, error) {
						completeRequest = request
						return sampleTaskRunRecord(taskpkg.TaskRunStatusCompleted), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task",
					"run",
					"complete",
					"run-1",
					"--result",
					`{"ok":true}`,
					"-o",
					"json",
				); err != nil {
					t.Fatalf("task run complete error = %v", err)
				}
				if string(completeRequest.Result) != `{"ok":true}` {
					t.Fatalf("completeRequest = %#v, want JSON result", completeRequest)
				}
			},
		},
		{
			name: "Should parse task run fail request",
			run: func(t *testing.T) {
				t.Helper()

				var failRequest FailTaskRunRequest
				deps := newTestDeps(t, &stubClient{
					failTaskRunFn: func(_ context.Context, _ string, request FailTaskRunRequest) (TaskRunRecord, error) {
						failRequest = request
						return sampleTaskRunRecord(taskpkg.TaskRunStatusFailed), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task",
					"run",
					"fail",
					"run-1",
					"--error",
					"boom",
					"--metadata",
					`{"code":"E_TASK"}`,
					"-o",
					"json",
				); err != nil {
					t.Fatalf("task run fail error = %v", err)
				}
				if failRequest.Error != "boom" || string(failRequest.Metadata) != `{"code":"E_TASK"}` {
					t.Fatalf("failRequest = %#v, want error and metadata", failRequest)
				}
			},
		},
		{
			name: "Should parse task run cancel request",
			run: func(t *testing.T) {
				t.Helper()

				var cancelRequest CancelTaskRunRequest
				deps := newTestDeps(t, &stubClient{
					cancelTaskRunFn: func(_ context.Context, _ string, request CancelTaskRunRequest) (TaskRunRecord, error) {
						cancelRequest = request
						return sampleTaskRunRecord(taskpkg.TaskRunStatusCanceled), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task",
					"run",
					"cancel",
					"run-1",
					"--reason",
					"operator-request",
					"--metadata",
					`{"source":"cli"}`,
					"-o",
					"json",
				); err != nil {
					t.Fatalf("task run cancel error = %v", err)
				}
				if cancelRequest.Reason != "operator-request" || string(cancelRequest.Metadata) != `{"source":"cli"}` {
					t.Fatalf("cancelRequest = %#v, want reason and metadata", cancelRequest)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func TestAgentTaskCommandsMapLeaseRequests(t *testing.T) {
	t.Parallel()

	t.Run("Should map task next lease request", func(t *testing.T) {
		t.Parallel()

		var gotRequest AgentTaskClaimNextRequest
		deps := newAgentCommandTestDeps(t, &stubClient{
			agentTaskClaimNextFn: func(
				_ context.Context,
				request AgentTaskClaimNextRequest,
				credentials agentidentity.Credentials,
			) (AgentTaskNextRecord, error) {
				assertAgentCredentials(t, credentials)
				gotRequest = request
				return AgentTaskNextRecord{
					Claimed: true,
					Claim:   new(agentTaskClaimRecord()),
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"next",
			"--run-id",
			"run-exact",
			"--workspace-id",
			"ws-1",
			"--capability",
			"go",
			"--capability",
			"   ",
			"--capability",
			"typescript",
			"--priority-min",
			"3",
			"--lease-seconds",
			"120",
			"--wait",
			"--idempotency-key",
			"idem-next",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task next error = %v", err)
		}
		if gotRequest.RunID != "run-exact" ||
			gotRequest.WorkspaceID != "ws-1" ||
			gotRequest.PriorityMin != 3 ||
			gotRequest.LeaseSeconds != 120 ||
			!gotRequest.Wait ||
			gotRequest.IdempotencyKey != "idem-next" ||
			len(gotRequest.RequiredCapabilities) != 2 ||
			gotRequest.RequiredCapabilities[0] != "go" ||
			gotRequest.RequiredCapabilities[1] != "typescript" {
			t.Fatalf("next request = %#v, want parsed agent claim request", gotRequest)
		}
		var output AgentTaskNextRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(task next) error = %v", err)
		}
		if strings.Contains(stdout, `"claim_token"`) || strings.Contains(stdout, "agh_claim_") {
			t.Fatal("task next output leaked raw claim token")
		}
		if !output.Claimed || output.Claim == nil || output.Claim.Lease.ClaimTokenHash == "" {
			t.Fatalf("task next output = %#v, want claimed session-bound response", output)
		}
	})

	t.Run("Should reject an explicitly blank exact run selector", func(t *testing.T) {
		t.Parallel()

		deps := newAgentCommandTestDeps(t, &stubClient{
			agentTaskClaimNextFn: func(
				context.Context,
				AgentTaskClaimNextRequest,
				agentidentity.Credentials,
			) (AgentTaskNextRecord, error) {
				t.Fatal("AgentTaskClaimNext should not be called for a blank exact run selector")
				return AgentTaskNextRecord{}, nil
			},
		})
		_, _, err := executeRootCommand(t, deps, "task", "next", "--run-id", "   ")
		if err == nil || !strings.Contains(err.Error(), "--run-id cannot be blank") {
			t.Fatalf("task next blank --run-id error = %v, want explicit blank rejection", err)
		}
	})

	t.Run("Should render task next no-work response", func(t *testing.T) {
		t.Parallel()

		deps := newAgentCommandTestDeps(t, &stubClient{
			agentTaskClaimNextFn: func(
				context.Context,
				AgentTaskClaimNextRequest,
				agentidentity.Credentials,
			) (AgentTaskNextRecord, error) {
				return AgentTaskNextRecord{Claimed: false}, nil
			},
		})
		stdout, _, err := executeRootCommand(t, deps, "task", "next", "-o", "json")
		if err != nil {
			t.Fatalf("task next no-work error = %v", err)
		}
		var output AgentTaskNextRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(task next no-work) error = %v", err)
		}
		if output.Claimed || output.Claim != nil {
			t.Fatalf("task next no-work output = %#v, want claimed false with no claim", output)
		}
	})

	for _, tt := range []struct {
		name string
		args []string
		fn   func(t *testing.T) *stubClient
	}{
		{
			name: "Should map task heartbeat request",
			args: []string{"task", "heartbeat", "run-1", "--lease-seconds", "60", "-o", "json"},
			fn: func(t *testing.T) *stubClient {
				t.Helper()
				return &stubClient{
					agentTaskHeartbeatFn: func(
						_ context.Context,
						runID string,
						request AgentTaskHeartbeatRequest,
						credentials agentidentity.Credentials,
					) (AgentTaskLeaseRecord, error) {
						assertAgentCredentials(t, credentials)
						if runID != "run-1" || request.LeaseSeconds != 60 {
							t.Fatalf("heartbeat runID=%q request=%#v, want run-1 lease duration", runID, request)
						}
						return agentTaskLeaseRecord(taskpkg.TaskRunStatusClaimed), nil
					},
				}
			},
		},
		{
			name: "Should map task complete request",
			args: []string{"task", "complete", "run-1", "--result", `{"ok":true}`, "-o", "json"},
			fn: func(t *testing.T) *stubClient {
				t.Helper()
				return &stubClient{
					agentTaskCompleteFn: func(
						_ context.Context,
						runID string,
						request AgentTaskCompleteRequest,
						credentials agentidentity.Credentials,
					) (AgentTaskLeaseRecord, error) {
						assertAgentCredentials(t, credentials)
						if runID != "run-1" || string(request.Result) != `{"ok":true}` {
							t.Fatalf("complete runID=%q request=%#v, want run-1 result", runID, request)
						}
						return agentTaskLeaseRecord(taskpkg.TaskRunStatusCompleted), nil
					},
				}
			},
		},
		{
			name: "Should map task fail request",
			args: []string{
				"task",
				"fail",
				"run-1",
				"--error",
				"boom",
				"--metadata",
				`{"phase":"agent"}`,
				"-o",
				"json",
			},
			fn: func(t *testing.T) *stubClient {
				t.Helper()
				return &stubClient{
					agentTaskFailFn: func(
						ctx context.Context,
						runID string,
						request AgentTaskFailRequest,
						credentials agentidentity.Credentials,
					) (AgentTaskLeaseRecord, error) {
						if ctx == nil {
							t.Fatal("AgentTaskFail context is nil")
						}
						assertAgentCredentials(t, credentials)
						if runID != "run-1" ||
							request.Error != "boom" ||
							string(request.Metadata) != `{"phase":"agent"}` {
							t.Fatalf("fail runID=%q request=%#v, want run-1 error metadata", runID, request)
						}
						return agentTaskLeaseRecord(taskpkg.TaskRunStatusFailed), nil
					},
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout, _, err := executeRootCommand(t, newAgentCommandTestDeps(t, tt.fn(t)), tt.args...)
			if err != nil {
				t.Fatalf("%s command error = %v", tt.name, err)
			}
			if strings.Contains(stdout, "agh_claim_") {
				t.Fatalf("%s output leaked raw claim token pattern: %s", tt.name, stdout)
			}
			var output AgentTaskLeaseRecord
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatalf("json.Unmarshal(%s output) error = %v", tt.name, err)
			}
			if output.RunID != "run-1" {
				t.Fatalf("%s output = %#v, want run-1 lease", tt.name, output)
			}
		})
	}
}

func TestTaskForceCommandsMapRequests(t *testing.T) {
	t.Parallel()

	t.Run("Should map single force fail request", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			forceFailTaskRunFn: func(
				_ context.Context,
				runID string,
				request ForceFailTaskRunRequest,
			) (TaskRunRecord, error) {
				if runID != "run-1" ||
					request.Reason != "boom" ||
					string(request.Metadata) != `{"code":"E_TASK"}` {
					t.Fatalf("force fail runID=%q request=%#v, want reason metadata", runID, request)
				}
				return taskRunRecord("run-1", taskpkg.TaskRunStatusFailed), nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"fail",
			"run-1",
			"--reason",
			"boom",
			"--metadata",
			`{"code":"E_TASK"}`,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task fail error = %v", err)
		}
		var output TaskRunRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(task fail) error = %v", err)
		}
		if output.ID != "run-1" || output.Status != taskpkg.TaskRunStatusFailed {
			t.Fatalf("task fail output = %#v, want failed run-1", output)
		}
	})

	t.Run("Should map bulk release request", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			bulkForceReleaseRunsFn: func(
				_ context.Context,
				request BulkForceTaskRunRequest,
			) (BulkForceTaskRunRecord, error) {
				if strings.Join(request.RunIDs, ",") != "run-1,run-2" || request.Reason != "handoff" {
					t.Fatalf("bulk release request = %#v, want two run ids and reason", request)
				}
				run1 := taskRunRecord("run-1", taskpkg.TaskRunStatusQueued)
				run2 := taskRunRecord("run-2", taskpkg.TaskRunStatusQueued)
				return BulkForceTaskRunRecord{Results: []BulkForceTaskRunItemRecord{
					{
						RunID: "run-1",
						OK:    true,
						Run:   &run1,
					},
					{
						RunID: "run-2",
						OK:    true,
						Run:   &run2,
					},
				}}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"release",
			"run-1",
			"run-2",
			"--reason",
			"handoff",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task release error = %v", err)
		}
		var output BulkForceTaskRunRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(task release) error = %v", err)
		}
		if len(output.Results) != 2 || !output.Results[0].OK || !output.Results[1].OK {
			t.Fatalf("task release output = %#v, want two ok results", output)
		}
	})

	t.Run("Should map retry request", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			retryTaskRunFn: func(
				_ context.Context,
				runID string,
				request RetryTaskRunRequest,
			) (RetryTaskRunRecord, error) {
				if runID != "run-1" || string(request.Metadata) != `{"source":"operator"}` {
					t.Fatalf("retry runID=%q request=%#v, want metadata", runID, request)
				}
				return RetryTaskRunRecord{
					PreviousRun: taskRunRecord("run-1", taskpkg.TaskRunStatusFailed),
					Run:         taskRunRecord("run-2", taskpkg.TaskRunStatusQueued),
				}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"retry",
			"run-1",
			"--metadata",
			`{"source":"operator"}`,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task retry error = %v", err)
		}
		var output RetryTaskRunRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(task retry) error = %v", err)
		}
		if output.PreviousRun.ID != "run-1" || output.Run.ID != "run-2" {
			t.Fatalf("task retry output = %#v, want source run-1 and new run-2", output)
		}
	})

	t.Run("Should map recover request with recover-specific text output", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			recoverTaskRunFn: func(
				_ context.Context,
				runID string,
				request RecoverTaskRunRequest,
			) (RetryTaskRunRecord, error) {
				if runID != "run-1" ||
					request.Reason != "operator recovery" ||
					string(request.Metadata) != `{"source":"operator"}` {
					t.Fatalf("recover runID=%q request=%#v, want reason and metadata", runID, request)
				}
				return RetryTaskRunRecord{
					PreviousRun: taskRunRecord("run-1", taskpkg.TaskRunStatusFailed),
					Run:         taskRunRecord("run-2", taskpkg.TaskRunStatusQueued),
				}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"run",
			"recover",
			"run-1",
			"--reason",
			"operator recovery",
			"--metadata",
			`{"source":"operator"}`,
			"-o",
			"human",
		)
		if err != nil {
			t.Fatalf("task recover error = %v", err)
		}
		if !strings.Contains(stdout, "Task Run Recovery") {
			t.Fatalf("task recover output = %q, want recovery heading", stdout)
		}
		if strings.Contains(stdout, "Task Run Retry") {
			t.Fatalf("task recover output = %q, want no retry heading", stdout)
		}
	})
}

func TestAgentTaskCommandsValidateBeforeAgentCalls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "Should reject blank run id",
			args:    []string{"task", "heartbeat", " ", "-o", "json"},
			wantErr: "run id is required",
		},
		{
			name: "Should reject negative lease duration",
			args: []string{
				"task",
				"heartbeat",
				"run-1",
				"--lease-seconds",
				"-1",
				"-o",
				"json",
			},
			wantErr: "--lease-seconds must be zero or positive",
		},
		{
			name:    "Should reject negative priority",
			args:    []string{"task", "next", "--priority-min", "-1", "-o", "json"},
			wantErr: "--priority-min must be zero or positive",
		},
		{
			name: "Should reject invalid result json",
			args: []string{
				"task",
				"complete",
				"run-1",
				"--result",
				`{"ok":`,
				"-o",
				"json",
			},
			wantErr: "invalid --result JSON",
		},
		{
			name: "Should reject raw claim token in result",
			args: []string{
				"task",
				"complete",
				"run-1",
				"--result",
				`{"claim_token":"secret"}`,
				"-o",
				"json",
			},
			wantErr: "must not contain raw lease credential",
		},
		{
			name: "Should reject raw claim token in force-failure metadata",
			args: []string{
				"task",
				"fail",
				"run-1",
				"--reason",
				"boom",
				"--metadata",
				`{"claim_token":"secret"}`,
				"-o",
				"json",
			},
			wantErr: "must not contain raw lease credential",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &stubClient{
				getSessionFn: func(context.Context, string) (SessionRecord, error) {
					t.Fatal("GetSession should not be called for local validation errors")
					return SessionRecord{}, nil
				},
				agentTaskClaimNextFn: func(context.Context, AgentTaskClaimNextRequest, agentidentity.Credentials) (AgentTaskNextRecord, error) {
					t.Fatal("AgentTaskClaimNext should not be called for local validation errors")
					return AgentTaskNextRecord{}, nil
				},
				agentTaskHeartbeatFn: func(context.Context, string, AgentTaskHeartbeatRequest, agentidentity.Credentials) (AgentTaskLeaseRecord, error) {
					t.Fatal("AgentTaskHeartbeat should not be called for local validation errors")
					return AgentTaskLeaseRecord{}, nil
				},
				agentTaskCompleteFn: func(context.Context, string, AgentTaskCompleteRequest, agentidentity.Credentials) (AgentTaskLeaseRecord, error) {
					t.Fatal("AgentTaskComplete should not be called for local validation errors")
					return AgentTaskLeaseRecord{}, nil
				},
				agentTaskFailFn: func(context.Context, string, AgentTaskFailRequest, agentidentity.Credentials) (AgentTaskLeaseRecord, error) {
					t.Fatal("AgentTaskFail should not be called for local validation errors")
					return AgentTaskLeaseRecord{}, nil
				},
				agentTaskReleaseFn: func(context.Context, string, AgentTaskReleaseRequest, agentidentity.Credentials) (AgentTaskLeaseRecord, error) {
					t.Fatal("AgentTaskRelease should not be called for local validation errors")
					return AgentTaskLeaseRecord{}, nil
				},
				forceFailTaskRunFn: func(context.Context, string, ForceFailTaskRunRequest) (TaskRunRecord, error) {
					t.Fatal("ForceFailTaskRun should not be called for local validation errors")
					return TaskRunRecord{}, nil
				},
				forceReleaseTaskRunFn: func(context.Context, string, ForceReleaseTaskRunRequest) (TaskRunRecord, error) {
					t.Fatal("ForceReleaseTaskRun should not be called for local validation errors")
					return TaskRunRecord{}, nil
				},
				retryTaskRunFn: func(context.Context, string, RetryTaskRunRequest) (RetryTaskRunRecord, error) {
					t.Fatal("RetryTaskRun should not be called for local validation errors")
					return RetryTaskRunRecord{}, nil
				},
			}
			deps := newTestDeps(t, client)
			deps.getenv = agentCommandEnv
			_, _, err := executeRootCommand(t, deps, tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("executeRootCommand(%v) error = %v, want %q", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestTaskMutationCommandsMapRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Should parse task update request",
			run: func(t *testing.T) {
				t.Helper()

				var (
					updateTaskID  string
					updateRequest UpdateTaskRequest
				)
				deps := newTestDeps(t, &stubClient{
					updateTaskFn: func(_ context.Context, taskID string, request UpdateTaskRequest) (TaskRecord, error) {
						updateTaskID = taskID
						updateRequest = request
						return sampleTaskRecord(), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task", "update", "task-1",
					"--title", "Retitle triage task",
					"--description", "Refined scope",
					"--priority", "urgent",
					"--owner-kind", "pool",
					"--owner-ref", "triage",
					"--metadata", `{"priority":"low"}`,
					"--network", "live",
					"--network-channel-strategy", "named",
					"--network-channel", "builders",
					"-o", "json",
				); err != nil {
					t.Fatalf("task update error = %v", err)
				}
				if updateTaskID != "task-1" ||
					updateRequest.Title == nil || *updateRequest.Title != "Retitle triage task" ||
					updateRequest.Description == nil || *updateRequest.Description != "Refined scope" ||
					updateRequest.Priority == nil || *updateRequest.Priority != taskpkg.PriorityUrgent ||
					updateRequest.Owner == nil || updateRequest.Owner.Kind != taskpkg.OwnerKindPool || updateRequest.Owner.Ref != "triage" ||
					updateRequest.ClearOwner ||
					updateRequest.Metadata == nil || string(*updateRequest.Metadata) != `{"priority":"low"}` {
					t.Fatalf("update request = %#v, want parsed task mutation payload", updateRequest)
				}
				assertLiveNamedParticipationRequest(t, updateRequest.NetworkParticipation, "builders")
			},
		},
		{
			name: "Should reject removed --channel on task update",
			run: func(t *testing.T) {
				t.Helper()

				deps := newTestDeps(t, &stubClient{})
				_, stderr, err := executeRootCommand(
					t,
					deps,
					"task", "update", "task-1",
					"--title", "Retitle triage task",
					"--channel", "builders",
				)
				if err == nil {
					t.Fatal("task update --channel error = nil, want rejection")
				}
				if !strings.Contains(err.Error(), "unknown flag: --channel") &&
					!strings.Contains(stderr, "unknown flag: --channel") {
					t.Fatalf("task update --channel error = %v stderr=%q, want removed flag rejection", err, stderr)
				}
			},
		},
		{
			name: "Should parse task cancel request",
			run: func(t *testing.T) {
				t.Helper()

				var (
					cancelTaskID  string
					cancelRequest CancelTaskRequest
				)
				deps := newTestDeps(t, &stubClient{
					cancelTaskFn: func(_ context.Context, taskID string, request CancelTaskRequest) (TaskRecord, error) {
						cancelTaskID = taskID
						cancelRequest = request
						return sampleTaskRecord(), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task",
					"cancel",
					"task-1",
					"--reason",
					"operator-request",
					"--metadata",
					`{"source":"cli"}`,
					"-o",
					"json",
				); err != nil {
					t.Fatalf("task cancel error = %v", err)
				}
				if cancelTaskID != "task-1" || cancelRequest.Reason != "operator-request" ||
					string(cancelRequest.Metadata) != `{"source":"cli"}` {
					t.Fatalf("cancel request = %#v, want parsed cancel payload", cancelRequest)
				}
			},
		},
		{
			name: "Should parse task pause request",
			run: func(t *testing.T) {
				t.Helper()

				var (
					pauseTaskID  string
					pauseRequest PauseTaskRequest
				)
				deps := newTestDeps(t, &stubClient{
					pauseTaskFn: func(_ context.Context, taskID string, request PauseTaskRequest) (TaskRecord, error) {
						pauseTaskID = taskID
						pauseRequest = request
						record := sampleTaskRecord()
						record.Paused = true
						record.PausedReason = request.Reason
						return record, nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task",
					"pause",
					"task-1",
					"--reason",
					"provider incident",
					"--metadata",
					"{\"source\":\"cli\"}",
					"-o",
					"json",
				); err != nil {
					t.Fatalf("task pause error = %v", err)
				}
				if pauseTaskID != "task-1" || pauseRequest.Reason != "provider incident" ||
					string(pauseRequest.Metadata) != "{\"source\":\"cli\"}" {
					t.Fatalf("pause request = %#v taskID=%q, want parsed pause payload", pauseRequest, pauseTaskID)
				}
			},
		},
		{
			name: "Should parse task resume request",
			run: func(t *testing.T) {
				t.Helper()

				var (
					resumeTaskID  string
					resumeRequest ResumeTaskRequest
				)
				deps := newTestDeps(t, &stubClient{
					resumeTaskFn: func(_ context.Context, taskID string, request ResumeTaskRequest) (TaskRecord, error) {
						resumeTaskID = taskID
						resumeRequest = request
						return sampleTaskRecord(), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task",
					"resume",
					"task-1",
					"--metadata",
					"{\"source\":\"cli\"}",
					"-o",
					"json",
				); err != nil {
					t.Fatalf("task resume error = %v", err)
				}
				if resumeTaskID != "task-1" || string(resumeRequest.Metadata) != "{\"source\":\"cli\"}" {
					t.Fatalf("resume request = %#v taskID=%q, want parsed resume payload", resumeRequest, resumeTaskID)
				}
			},
		},
		{
			name: "Should parse child task create request",
			run: func(t *testing.T) {
				t.Helper()

				var (
					childParentID      string
					childCreateRequest CreateTaskChildRequest
				)
				deps := newTestDeps(t, &stubClient{
					createChildTaskFn: func(_ context.Context, parentID string, request CreateTaskChildRequest) (TaskRecord, error) {
						childParentID = parentID
						childCreateRequest = request
						return sampleTaskRecord(), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task", "child", "create", "task-root",
					"--id", "task-child",
					"--identifier", "OPS-43",
					"--scope", "workspace",
					"--workspace", "alpha",
					"--network", "live",
					"--network-channel-strategy", "named",
					"--network-channel", "builders",
					"--title", "Check runtime logs",
					"--description", "Focus on worker output",
					"--priority", "urgent",
					"--owner-kind", "human",
					"--owner-ref", "alice",
					"--metadata", `{"phase":"two"}`,
					"-o", "json",
				); err != nil {
					t.Fatalf("task child create error = %v", err)
				}
				assertLiveNamedParticipationRequest(t, childCreateRequest.NetworkParticipation, "builders")
				if childParentID != "task-root" ||
					childCreateRequest.ID != "task-child" ||
					childCreateRequest.Identifier != "OPS-43" ||
					childCreateRequest.Scope != taskpkg.ScopeWorkspace ||
					childCreateRequest.Workspace != "alpha" ||
					childCreateRequest.Title != "Check runtime logs" ||
					childCreateRequest.Description != "Focus on worker output" ||
					childCreateRequest.Priority != taskpkg.PriorityUrgent ||
					childCreateRequest.Owner == nil || childCreateRequest.Owner.Kind != taskpkg.OwnerKindHuman || childCreateRequest.Owner.Ref != "alice" ||
					string(childCreateRequest.Metadata) != `{"phase":"two"}` {
					t.Fatalf("childCreateRequest = %#v, want parsed child task payload", childCreateRequest)
				}
			},
		},
		{
			name: "Should parse add dependency request",
			run: func(t *testing.T) {
				t.Helper()

				var (
					dependencyTaskID  string
					dependencyRequest AddTaskDependencyRequest
				)
				deps := newTestDeps(t, &stubClient{
					addTaskDependencyFn: func(_ context.Context, taskID string, request AddTaskDependencyRequest) (TaskDetailRecord, error) {
						dependencyTaskID = taskID
						dependencyRequest = request
						return sampleTaskDetailRecord(), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task",
					"dependency",
					"add",
					"task-1",
					"--depends-on",
					"task-root",
					"--kind",
					"blocks",
					"-o",
					"json",
				); err != nil {
					t.Fatalf("task dependency add error = %v", err)
				}
				if dependencyTaskID != "task-1" || dependencyRequest.DependsOnTaskID != "task-root" ||
					dependencyRequest.Kind != taskpkg.DependencyKindBlocks {
					t.Fatalf("dependencyRequest = %#v, want dependency payload", dependencyRequest)
				}
			},
		},
		{
			name: "Should parse remove dependency arguments",
			run: func(t *testing.T) {
				t.Helper()

				var (
					removeTaskID      string
					removeDependsOnID string
				)
				deps := newTestDeps(t, &stubClient{
					removeTaskDependencyFn: func(_ context.Context, taskID string, dependsOnID string) (TaskDetailRecord, error) {
						removeTaskID = taskID
						removeDependsOnID = dependsOnID
						return sampleTaskDetailRecord(), nil
					},
				})

				if _, _, err := executeRootCommand(
					t,
					deps,
					"task",
					"dependency",
					"remove",
					"task-1",
					"task-root",
					"-o",
					"json",
				); err != nil {
					t.Fatalf("task dependency remove error = %v", err)
				}
				if removeTaskID != "task-1" || removeDependsOnID != "task-root" {
					t.Fatalf(
						"remove dependency args = (%q, %q), want task-1/task-root",
						removeTaskID,
						removeDependsOnID,
					)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func TestTaskProfileCommandsMapRequests(t *testing.T) {
	t.Parallel()

	t.Run("Should inspect update and delete task execution profiles", func(t *testing.T) {
		t.Parallel()

		var (
			inspectID     string
			updateID      string
			updateRequest TaskExecutionProfileRequest
			deleteID      string
		)
		deps := newTestDeps(t, &stubClient{
			getTaskExecutionProfileFn: func(_ context.Context, id string) (TaskExecutionProfileRecord, error) {
				inspectID = id
				return sampleTaskExecutionProfileRecord(), nil
			},
			setTaskExecutionProfileFn: func(
				_ context.Context,
				id string,
				request *TaskExecutionProfileRequest,
			) (TaskExecutionProfileRecord, error) {
				if request == nil {
					t.Fatal("request is nil")
					return TaskExecutionProfileRecord{}, nil
				}
				updateID = id
				updateRequest = *request
				record := *request
				record.UpdatedAt = fixedTestNow
				return record, nil
			},
			deleteTaskExecutionProfileFn: func(_ context.Context, id string) error {
				deleteID = id
				return nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "task", "profile", "inspect", "task-1", "-o", "json")
		if err != nil {
			t.Fatalf("task profile inspect error = %v", err)
		}
		var inspected TaskExecutionProfileRecord
		if err := json.Unmarshal([]byte(stdout), &inspected); err != nil {
			t.Fatalf("json.Unmarshal(profile inspect) error = %v", err)
		}
		if inspectID != "task-1" || inspected.Worker.AgentName != "worker-a" {
			t.Fatalf("inspect id/profile = %q/%#v", inspectID, inspected)
		}

		stdout, _, err = executeRootCommand(
			t,
			deps,
			"task",
			"profile",
			"update",
			"task-1",
			"--profile",
			`{"worker":{"mode":"select","agent_name":"worker-b"},"sandbox":{"mode":"none"}}`,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task profile update error = %v", err)
		}
		if updateID != "task-1" ||
			updateRequest.TaskID != "task-1" ||
			updateRequest.Worker.Mode != taskpkg.WorkerModeSelect ||
			updateRequest.Worker.AgentName != "worker-b" ||
			updateRequest.Sandbox.Mode != taskpkg.SandboxModeNone {
			t.Fatalf("update request = %#v", updateRequest)
		}
		var updated TaskExecutionProfileRecord
		if err := json.Unmarshal([]byte(stdout), &updated); err != nil {
			t.Fatalf("json.Unmarshal(profile update) error = %v", err)
		}
		if updated.TaskID != "task-1" || updated.Worker.AgentName != "worker-b" {
			t.Fatalf("updated profile = %#v", updated)
		}

		stdout, _, err = executeRootCommand(t, deps, "task", "profile", "delete", "task-1", "-o", "json")
		if err != nil {
			t.Fatalf("task profile delete error = %v", err)
		}
		if deleteID != "task-1" {
			t.Fatalf("delete id = %q, want task-1", deleteID)
		}
		if !strings.Contains(stdout, `"status": "deleted"`) {
			t.Fatalf("delete stdout = %s, want deleted status", stdout)
		}
	})

	t.Run("Should reject mismatched profile task id before calling client", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			setTaskExecutionProfileFn: func(
				context.Context,
				string,
				*TaskExecutionProfileRequest,
			) (TaskExecutionProfileRecord, error) {
				t.Fatal("SetTaskExecutionProfile should not be called for mismatched task id")
				return TaskExecutionProfileRecord{}, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"profile",
			"update",
			"task-1",
			"--profile",
			`{"task_id":"other"}`,
		)
		if err == nil || !strings.Contains(err.Error(), `profile.task_id must match task id "task-1"`) {
			t.Fatalf("task profile update mismatch error = %v", err)
		}
	})
}

func TestTaskNotificationCommandsMapRequests(t *testing.T) {
	t.Parallel()

	t.Run("Should subscribe list and delete bridge task notifications", func(t *testing.T) {
		t.Parallel()

		var (
			subscribeTaskID string
			subscribeBody   TaskBridgeNotificationSubscriptionRequest
			listTaskID      string
			listQuery       TaskBridgeNotificationSubscriptionQuery
			showTaskID      string
			showID          string
			deleteTaskID    string
			deleteID        string
		)
		deps := newTestDeps(t, &stubClient{
			createTaskBridgeNotificationSubscriptionFn: func(
				_ context.Context,
				taskID string,
				request *TaskBridgeNotificationSubscriptionRequest,
			) (TaskBridgeNotificationSubscriptionRecord, error) {
				if request == nil {
					t.Fatal("request is nil")
					return TaskBridgeNotificationSubscriptionRecord{}, nil
				}
				subscribeTaskID = taskID
				subscribeBody = *request
				return sampleTaskBridgeNotificationSubscriptionRecord(), nil
			},
			listTaskBridgeNotificationSubscriptionsFn: func(
				_ context.Context,
				taskID string,
				query TaskBridgeNotificationSubscriptionQuery,
			) ([]TaskBridgeNotificationSubscriptionRecord, error) {
				listTaskID = taskID
				listQuery = query
				return []TaskBridgeNotificationSubscriptionRecord{sampleTaskBridgeNotificationSubscriptionRecord()}, nil
			},
			getTaskBridgeNotificationSubscriptionFn: func(
				_ context.Context,
				taskID string,
				subscriptionID string,
			) (TaskBridgeNotificationSubscriptionRecord, error) {
				showTaskID = taskID
				showID = subscriptionID
				return sampleTaskBridgeNotificationSubscriptionRecord(), nil
			},
			deleteTaskBridgeNotificationSubscriptionFn: func(
				_ context.Context,
				taskID string,
				subscriptionID string,
			) error {
				deleteTaskID = taskID
				deleteID = subscriptionID
				return nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"notification",
			"subscribe",
			"task-1",
			"--subscription-id",
			"sub-1",
			"--bridge",
			"brg-1",
			"--scope",
			"workspace",
			"--workspace",
			"ws-1",
			"--peer",
			"peer-1",
			"--thread",
			"thread-1",
			"--mode",
			"reply",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task notification subscribe error = %v", err)
		}
		if subscribeTaskID != "task-1" ||
			subscribeBody.SubscriptionID != "sub-1" ||
			subscribeBody.BridgeInstanceID != "brg-1" ||
			subscribeBody.Scope != bridgepkg.ScopeWorkspace ||
			subscribeBody.WorkspaceID != "ws-1" ||
			subscribeBody.PeerID != "peer-1" ||
			subscribeBody.ThreadID != "thread-1" ||
			subscribeBody.DeliveryMode != bridgepkg.DeliveryModeReply {
			t.Fatalf("subscribe body = %#v for task %q", subscribeBody, subscribeTaskID)
		}
		var subscribed TaskBridgeNotificationSubscriptionRecord
		if err := json.Unmarshal([]byte(stdout), &subscribed); err != nil {
			t.Fatalf("json.Unmarshal(notification subscribe) error = %v", err)
		}
		if subscribed.SubscriptionID != "sub-1" ||
			subscribed.Cursor.ConsumerID == "" ||
			subscribed.Cursor.LastSequence != 7 ||
			subscribed.Cursor.LastDeliveryID != "notif:sub-1:7" {
			t.Fatalf("subscribed = %#v", subscribed)
		}

		stdout, _, err = executeRootCommand(
			t,
			deps,
			"task",
			"notification",
			"list",
			"task-1",
			"--bridge",
			"brg-1",
			"--scope",
			"workspace",
			"--workspace",
			"ws-1",
			"--last",
			"5",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task notification list error = %v", err)
		}
		if listTaskID != "task-1" ||
			listQuery.BridgeInstanceID != "brg-1" ||
			listQuery.Scope != bridgepkg.ScopeWorkspace ||
			listQuery.WorkspaceID != "ws-1" ||
			listQuery.Limit != 5 {
			t.Fatalf("list query = %#v for task %q", listQuery, listTaskID)
		}
		if !strings.Contains(stdout, `"subscription_id": "sub-1"`) ||
			!strings.Contains(stdout, `"last_sequence": 7`) ||
			!strings.Contains(stdout, `"last_error": "bridge adapter rejected send"`) {
			t.Fatalf("notification list stdout = %s", stdout)
		}

		stdout, _, err = executeRootCommand(
			t,
			deps,
			"task",
			"notification",
			"show",
			"task-1",
			"sub-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task notification show error = %v", err)
		}
		if showTaskID != "task-1" || showID != "sub-1" {
			t.Fatalf("show task/id = %q/%q", showTaskID, showID)
		}
		if !strings.Contains(stdout, `"cursor"`) ||
			!strings.Contains(stdout, `"last_delivery_id": "notif:sub-1:7"`) {
			t.Fatalf("notification show stdout = %s", stdout)
		}

		stdout, _, err = executeRootCommand(
			t,
			deps,
			"task",
			"notification",
			"show",
			"task-1",
			"sub-1",
		)
		if err != nil {
			t.Fatalf("task notification show human error = %v", err)
		}
		if !strings.Contains(stdout, "Cursor Last Sequence") ||
			!strings.Contains(stdout, "bridge adapter rejected send") {
			t.Fatalf("notification show human stdout = %s", stdout)
		}

		stdout, _, err = executeRootCommand(
			t,
			deps,
			"task",
			"notification",
			"delete",
			"task-1",
			"sub-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task notification delete error = %v", err)
		}
		if deleteTaskID != "task-1" || deleteID != "sub-1" {
			t.Fatalf("delete task/id = %q/%q", deleteTaskID, deleteID)
		}
		if !strings.Contains(stdout, `"status": "deleted"`) {
			t.Fatalf("notification delete stdout = %s", stdout)
		}
	})

	t.Run("Should reject notification subscriptions without a delivery target", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			createTaskBridgeNotificationSubscriptionFn: func(
				context.Context,
				string,
				*TaskBridgeNotificationSubscriptionRequest,
			) (TaskBridgeNotificationSubscriptionRecord, error) {
				t.Fatal("CreateTaskBridgeNotificationSubscription should not be called")
				return TaskBridgeNotificationSubscriptionRecord{}, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"notification",
			"subscribe",
			"task-1",
			"--bridge",
			"brg-1",
		)
		if err == nil || !strings.Contains(err.Error(), "requires --peer or --group") {
			t.Fatalf("task notification subscribe target error = %v", err)
		}
	})

	t.Run("Should reject negative notification list limits before calling client", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			listTaskBridgeNotificationSubscriptionsFn: func(
				context.Context,
				string,
				TaskBridgeNotificationSubscriptionQuery,
			) ([]TaskBridgeNotificationSubscriptionRecord, error) {
				t.Fatal("ListTaskBridgeNotificationSubscriptions should not be called")
				return nil, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"notification",
			"list",
			"task-1",
			"--last",
			"-1",
		)
		if err == nil || !strings.Contains(err.Error(), "--last must be zero or positive") {
			t.Fatalf("task notification list error = %v", err)
		}
	})
}

func TestTaskReviewCommandsMapRequests(t *testing.T) {
	t.Parallel()

	t.Run("Should request list show and submit task run reviews", func(t *testing.T) {
		t.Parallel()

		var (
			requestRunID string
			requestBody  TaskRunReviewRequest
			listQuery    TaskRunReviewListQuery
			showID       string
			submitID     string
			submitBody   TaskRunReviewVerdictRequest
		)
		deps := newTestDeps(t, &stubClient{
			requestTaskRunReviewFn: func(
				_ context.Context,
				runID string,
				request *TaskRunReviewRequest,
			) (TaskRunReviewRequestRecord, error) {
				if request == nil {
					t.Fatal("request is nil")
					return TaskRunReviewRequestRecord{}, nil
				}
				requestRunID = runID
				requestBody = *request
				return TaskRunReviewRequestRecord{
					Review:  sampleTaskRunReviewRecord(taskpkg.RunReviewStatusRequested),
					Created: true,
				}, nil
			},
			listTaskRunReviewsFn: func(
				_ context.Context,
				query TaskRunReviewListQuery,
			) ([]TaskRunReviewRecord, error) {
				listQuery = query
				return []TaskRunReviewRecord{sampleTaskRunReviewRecord(taskpkg.RunReviewStatusRequested)}, nil
			},
			getTaskRunReviewFn: func(_ context.Context, reviewID string) (TaskRunReviewRecord, error) {
				showID = reviewID
				return sampleTaskRunReviewRecord(taskpkg.RunReviewStatusRequested), nil
			},
			submitTaskRunReviewVerdictFn: func(
				_ context.Context,
				reviewID string,
				request *TaskRunReviewVerdictRequest,
			) (TaskRunReviewVerdictRecord, error) {
				if request == nil {
					t.Fatal("request is nil")
					return TaskRunReviewVerdictRecord{}, nil
				}
				submitID = reviewID
				submitBody = *request
				review := sampleTaskRunReviewRecord(taskpkg.RunReviewStatusRecorded)
				review.Outcome = taskpkg.RunReviewOutcomeRejected
				return TaskRunReviewVerdictRecord{Review: review}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"review",
			"request",
			"run-1",
			"--reason",
			"ready for review",
			"--policy",
			"always",
			"--round",
			"2",
			"--attempt",
			"1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task review request error = %v", err)
		}
		if requestRunID != "run-1" ||
			requestBody.RunID != "run-1" ||
			requestBody.Policy != taskpkg.ReviewPolicyAlways ||
			requestBody.ReviewRound != 2 ||
			requestBody.Attempt != 1 ||
			requestBody.Reason != "ready for review" {
			t.Fatalf("request body = %#v with run %q", requestBody, requestRunID)
		}
		var requested TaskRunReviewRequestRecord
		if err := json.Unmarshal([]byte(stdout), &requested); err != nil {
			t.Fatalf("json.Unmarshal(review request) error = %v", err)
		}
		if !requested.Created || requested.Review.ReviewID != "review-1" {
			t.Fatalf("requested review = %#v", requested)
		}

		if _, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"review",
			"list",
			"--task",
			"task-1",
			"--status",
			"requested",
			"--reviewer-session",
			"sess-review",
			"--last",
			"3",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("task review list error = %v", err)
		}
		if listQuery.TaskID != "task-1" ||
			listQuery.Status != taskpkg.RunReviewStatusRequested ||
			listQuery.ReviewerSessionID != "sess-review" ||
			listQuery.Limit != 3 {
			t.Fatalf("list query = %#v", listQuery)
		}

		stdout, _, err = executeRootCommand(t, deps, "task", "review", "show", "review-1", "-o", "json")
		if err != nil {
			t.Fatalf("task review show error = %v", err)
		}
		if showID != "review-1" {
			t.Fatalf("show id = %q, want review-1", showID)
		}
		var shown TaskRunReviewRecord
		if err := json.Unmarshal([]byte(stdout), &shown); err != nil {
			t.Fatalf("json.Unmarshal(review show) error = %v", err)
		}
		if shown.ReviewID != "review-1" {
			t.Fatalf("shown review = %#v", shown)
		}

		stdout, _, err = executeRootCommand(
			t,
			deps,
			"task",
			"review",
			"submit",
			"review-1",
			"--run",
			"run-1",
			"--outcome",
			"rejected",
			"--confidence",
			"0.75",
			"--reason",
			"tests are missing",
			"--delivery-id",
			"delivery-1",
			"--missing-work",
			"add coverage",
			"--next-round-guidance",
			"add focused tests",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("task review submit error = %v", err)
		}
		if submitID != "review-1" ||
			submitBody.RunID != "run-1" ||
			submitBody.Verdict.Outcome != taskpkg.RunReviewOutcomeRejected ||
			submitBody.Verdict.Confidence == nil ||
			*submitBody.Verdict.Confidence != 0.75 ||
			submitBody.Verdict.DeliveryID != "delivery-1" {
			t.Fatalf("submit body = %#v with id %q", submitBody, submitID)
		}
		if got := string(submitBody.Verdict.MissingWork); got != `["add coverage"]` {
			t.Fatalf("missing work = %s, want JSON array", got)
		}
		var submitted TaskRunReviewVerdictRecord
		if err := json.Unmarshal([]byte(stdout), &submitted); err != nil {
			t.Fatalf("json.Unmarshal(review submit) error = %v", err)
		}
		if submitted.Review.Outcome != taskpkg.RunReviewOutcomeRejected {
			t.Fatalf("submitted review = %#v", submitted)
		}
	})

	t.Run("Should reject ambiguous review list scope before calling client", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			listTaskRunReviewsFn: func(context.Context, TaskRunReviewListQuery) ([]TaskRunReviewRecord, error) {
				t.Fatal("ListTaskRunReviews should not be called for ambiguous scope")
				return nil, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"review",
			"list",
			"--task",
			"task-1",
			"--run",
			"run-1",
		)
		if err == nil || !strings.Contains(err.Error(), "choose either --task or --run") {
			t.Fatalf("task review list ambiguous error = %v", err)
		}
	})

	t.Run("Should allow review list filters without task or run scope", func(t *testing.T) {
		t.Parallel()

		var listQuery TaskRunReviewListQuery
		deps := newTestDeps(t, &stubClient{
			listTaskRunReviewsFn: func(_ context.Context, query TaskRunReviewListQuery) ([]TaskRunReviewRecord, error) {
				listQuery = query
				return []TaskRunReviewRecord{sampleTaskRunReviewRecord(taskpkg.RunReviewStatusRequested)}, nil
			},
		})

		if _, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"review",
			"list",
			"--status",
			"requested",
			"--reviewer-session",
			"sess-review",
			"--last",
			"2",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("task review list filter-only error = %v", err)
		}
		if listQuery.TaskID != "" || listQuery.RunID != "" {
			t.Fatalf("list query scope = %#v, want global filter-only query", listQuery)
		}
		if listQuery.Status != taskpkg.RunReviewStatusRequested ||
			listQuery.ReviewerSessionID != "sess-review" ||
			listQuery.Limit != 2 {
			t.Fatalf("list query = %#v, want status/reviewer/limit filters", listQuery)
		}
	})

	t.Run("Should reject non-array missing-work JSON before calling client", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			submitTaskRunReviewVerdictFn: func(
				context.Context,
				string,
				*TaskRunReviewVerdictRequest,
			) (TaskRunReviewVerdictRecord, error) {
				t.Fatal("SubmitTaskRunReviewVerdict should not be called for invalid --missing-work-json")
				return TaskRunReviewVerdictRecord{}, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"review",
			"submit",
			"review-1",
			"--run",
			"run-1",
			"--outcome",
			"rejected",
			"--confidence",
			"0.5",
			"--reason",
			"tests are missing",
			"--delivery-id",
			"delivery-1",
			"--missing-work-json",
			`{"todo":"write tests"}`,
		)
		if err == nil || !strings.Contains(err.Error(), "--missing-work-json must be a JSON array") {
			t.Fatalf("task review submit invalid missing-work-json error = %v", err)
		}
	})

	t.Run("Should reject negative review round and attempt before calling client", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			requestTaskRunReviewFn: func(context.Context, string, *TaskRunReviewRequest) (TaskRunReviewRequestRecord, error) {
				t.Fatal("RequestTaskRunReview should not be called")
				return TaskRunReviewRequestRecord{}, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"task",
			"review",
			"request",
			"run-1",
			"--round",
			"-1",
			"--attempt",
			"-2",
		)
		if err == nil || !strings.Contains(err.Error(), "--round must be zero or positive") {
			t.Fatalf("task review request error = %v", err)
		}
	})
}

func TestTaskCommandsSupportDetailAndToonOutput(t *testing.T) {
	t.Parallel()

	t.Run("Should render task detail human sections", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			getTaskFn: func(context.Context, string) (TaskDetailRecord, error) {
				return sampleTaskDetailRecord(), nil
			},
		})
		humanOut, _, err := executeRootCommand(t, deps, "task", "get", "task-1", "-o", "human")
		if err != nil {
			t.Fatalf("task get human error = %v", err)
		}
		if !strings.Contains(humanOut, "Task") || !strings.Contains(humanOut, "Dependencies") ||
			!strings.Contains(humanOut, "Task Runs") {
			t.Fatalf("task get human output = %q, want detail sections", humanOut)
		}
	})

	t.Run("Should render task list toon array", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			listTasksFn: func(context.Context, TaskListQuery) ([]TaskCatalogItemRecord, error) {
				return []TaskCatalogItemRecord{sampleTaskCatalogItemRecord()}, nil
			},
		})
		toonOut, _, err := executeRootCommand(t, deps, "task", "list", "-o", "toon")
		if err != nil {
			t.Fatalf("task list toon error = %v", err)
		}
		if !strings.Contains(
			toonOut,
			"tasks[1]{id,identifier,scope,workspace_id,parent_task_id,status,owner,participation_channel,title}:",
		) || !strings.Contains(toonOut, "builders") {
			t.Fatalf("task list toon output = %q, want tasks TOON array with builders", toonOut)
		}
	})
}

func TestTaskBundlesRenderTaskRunAndDetailSections(t *testing.T) {
	t.Parallel()

	t.Run("Should render task detail toon sections", func(t *testing.T) {
		t.Parallel()

		detail := sampleTaskDetailRecord()
		detailToon, err := taskDetailBundle(&detail).toon()
		if err != nil {
			t.Fatalf("taskDetailBundle().toon() error = %v", err)
		}
		if !strings.Contains(detailToon, "task_children[1]{id,identifier,scope,workspace_id,status,owner,title}:") ||
			!strings.Contains(detailToon, "task_dependencies[1]{task_id,depends_on_task_id,kind,created_at}:") ||
			!strings.Contains(
				detailToon,
				"task_runs[1]{id,status,attempt,session_id,claimed_by,participation_channel,queued_at,started_at,ended_at,error}:",
			) ||
			!strings.Contains(detailToon, "task_events[1]{id,event_type,run_id,actor,origin,timestamp}:") ||
			!strings.Contains(detailToon, "builders") {
			t.Fatalf("task detail toon output = %q, want child/dependency/run/event sections with builders", detailToon)
		}
	})

	t.Run("Should render task run human detail section", func(t *testing.T) {
		t.Parallel()

		runHuman, err := taskRunBundle(sampleTaskRunRecord(taskpkg.TaskRunStatusCompleted)).human()
		if err != nil {
			t.Fatalf("taskRunBundle().human() error = %v", err)
		}
		if !strings.Contains(runHuman, "Task Run") || !strings.Contains(runHuman, "Idempotency Key") ||
			!strings.Contains(runHuman, "Result") {
			t.Fatalf("task run human output = %q, want task run detail section", runHuman)
		}
	})

	t.Run("Should render task run detail human sections", func(t *testing.T) {
		t.Parallel()

		detail := sampleTaskRunDetailRecord(taskpkg.TaskRunStatusRunning)
		detailHuman, err := taskRunDetailBundle(&detail).human()
		if err != nil {
			t.Fatalf("taskRunDetailBundle().human() error = %v", err)
		}
		if !strings.Contains(detailHuman, "Task Run") ||
			!strings.Contains(detailHuman, "Operational Summary") ||
			!strings.Contains(detailHuman, "Tool Calls") ||
			!strings.Contains(detailHuman, "USD 0.42 (estimated)") ||
			!strings.Contains(detailHuman, "catalog_config") {
			t.Fatalf("task run detail human output = %q, want run/task/summary sections", detailHuman)
		}
	})

	t.Run("Should suppress contradictory included task cost in human and TOON output", func(t *testing.T) {
		t.Parallel()

		detail := sampleTaskRunDetailRecord(taskpkg.TaskRunStatusRunning)
		detail.Summary.CostStatus = "included"
		detail.Summary.CostSource = "none"
		detailHuman, err := taskRunDetailBundle(&detail).human()
		if err != nil {
			t.Fatalf("taskRunDetailBundle().human() error = %v", err)
		}
		if !strings.Contains(detailHuman, "Included") || strings.Contains(detailHuman, "USD 0.42") {
			t.Fatalf("task run detail included output = %q, want Included without monetary amount", detailHuman)
		}
		detailToon, err := taskRunDetailBundle(&detail).toon()
		if err != nil {
			t.Fatalf("taskRunDetailBundle().toon() error = %v", err)
		}
		if !strings.Contains(detailToon, "Included") || strings.Contains(detailToon, "0.42") {
			t.Fatalf("task run detail included TOON = %q, want Included without monetary amount", detailToon)
		}
	})

	t.Run("Should render task run toon array", func(t *testing.T) {
		t.Parallel()

		runToon, err := taskRunListBundle([]TaskRunRecord{sampleTaskRunRecord(taskpkg.TaskRunStatusCompleted)}).toon()
		if err != nil {
			t.Fatalf("taskRunListBundle().toon() error = %v", err)
		}
		if !strings.Contains(
			runToon,
			"task_runs[1]{id,status,attempt,session_id,claimed_by,participation_channel,queued_at,started_at,ended_at,error}:",
		) || !strings.Contains(runToon, "completed") || !strings.Contains(runToon, "builders") {
			t.Fatalf("task run toon output = %q, want completed task run TOON array with builders", runToon)
		}
	})

	t.Run("Should render taskless network wake detail without a fabricated task", func(t *testing.T) {
		t.Parallel()

		detail := sampleTaskRunDetailRecord(taskpkg.TaskRunStatusRunning)
		detail.Task = nil
		detail.Run.TaskID = ""
		humanOut, err := taskRunDetailBundle(&detail).human()
		if err != nil {
			t.Fatalf("taskRunDetailBundle(taskless).human() error = %v", err)
		}
		toonOut, err := taskRunDetailBundle(&detail).toon()
		if err != nil {
			t.Fatalf("taskRunDetailBundle(taskless).toon() error = %v", err)
		}
		for format, output := range map[string]string{"human": humanOut, "toon": toonOut} {
			if !strings.Contains(output, "run-1") || strings.Contains(output, "Investigate flaky task runs") {
				t.Fatalf("taskless %s output = %q, want run without fabricated Task section", format, output)
			}
		}
	})

	t.Run("Should parse dependency kind validation", func(t *testing.T) {
		t.Parallel()

		if kind, err := parseOptionalTaskDependencyKind("blocks"); err != nil || kind != taskpkg.DependencyKindBlocks {
			t.Fatalf("parseOptionalTaskDependencyKind(blocks) = (%q, %v), want blocks", kind, err)
		}
		if _, err := parseOptionalTaskDependencyKind(
			"relates",
		); err == nil ||
			!strings.Contains(err.Error(), "unsupported value") {
			t.Fatalf("parseOptionalTaskDependencyKind(relates) error = %v, want unsupported value validation", err)
		}
	})
}

func TestParseTaskListFiltersRejectsInvalidFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		ownerKindRaw string
		ownerRef     string
	}{
		{name: "Should reject owner kind without owner ref", ownerKindRaw: "pool"},
		{name: "Should reject owner ref without owner kind", ownerRef: "triage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseTaskListFilters(
				"", "", "", "", tt.ownerKindRaw, tt.ownerRef, "", "", "", "", "", 0,
			)
			if err == nil || !strings.Contains(err.Error(), "--owner-kind and --owner-ref must be provided together") {
				t.Fatalf("parseTaskListFilters() error = %v, want paired owner filter validation", err)
			}
		})
	}

	t.Run("Should name the participation channel flag in validation errors", func(t *testing.T) {
		t.Parallel()

		_, err := parseTaskListFilters(
			"", "", "", "", "", "", "", "invalid channel!", "", "", "", 0,
		)
		if err == nil || !strings.Contains(err.Error(), "invalid --participation-channel value") ||
			strings.Contains(err.Error(), "invalid --channel value") {
			t.Fatalf("parseTaskListFilters() error = %v, want current participation-channel diagnostic", err)
		}
	})
}

func sampleTaskSummaryRecord() TaskSummaryRecord {
	return TaskSummaryRecord{
		ID:                           "task-1",
		Identifier:                   "OPS-42",
		Scope:                        taskpkg.ScopeWorkspace,
		WorkspaceID:                  "ws-alpha",
		ParentTaskID:                 "task-root",
		ResolvedNetworkParticipation: testLiveResolvedParticipation("builders"),
		Title:                        "Investigate flaky task runs",
		Status:                       taskpkg.TaskStatusReady,
		Owner:                        &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "triage"},
		CreatedBy:                    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
		Origin:                       taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "tasks.create"},
		CreatedAt:                    fixedTestNow,
		UpdatedAt:                    fixedTestNow,
	}
}

func sampleTaskCatalogItemRecord() TaskCatalogItemRecord {
	return TaskCatalogItemRecord{
		ID:                           "task-1",
		Identifier:                   "OPS-42",
		Scope:                        taskpkg.ScopeWorkspace,
		WorkspaceID:                  "ws-alpha",
		ParentTaskID:                 "task-root",
		ResolvedNetworkParticipation: testLiveResolvedParticipation("builders"),
		Title:                        "Investigate flaky task runs",
		Status:                       taskpkg.TaskStatusReady,
		Owner:                        &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "triage"},
		CreatedBy:                    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
		Origin:                       taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "tasks.create"},
		CreatedAt:                    fixedTestNow,
		UpdatedAt:                    fixedTestNow,
	}
}

func sampleTaskRecord() TaskRecord {
	return TaskRecord{
		ID:                           "task-1",
		Identifier:                   "OPS-42",
		Scope:                        taskpkg.ScopeWorkspace,
		WorkspaceID:                  "ws-alpha",
		ParentTaskID:                 "task-root",
		ResolvedNetworkParticipation: testLiveResolvedParticipation("builders"),
		Title:                        "Investigate flaky task runs",
		Description:                  "Capture root cause",
		Status:                       taskpkg.TaskStatusReady,
		Owner:                        &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "triage"},
		CreatedBy:                    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
		Origin:                       taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "tasks.create"},
		CreatedAt:                    fixedTestNow,
		UpdatedAt:                    fixedTestNow,
		Metadata:                     json.RawMessage(`{"priority":"high"}`),
	}
}

func sampleTaskBlockRecord() TaskBlockRecord {
	return TaskBlockRecord{
		ID:        "block-1",
		TaskID:    "task-1",
		Kind:      taskpkg.BlockKindNeedsInput,
		Reason:    "creator input",
		CreatedAt: fixedTestNow,
		CreatedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
	}
}

func sampleTaskInspectRecord(target string) TaskInspectRecord {
	now := time.Date(2026, 4, 17, 12, 30, 0, 0, time.UTC)
	return TaskInspectRecord{
		Target: target,
		Task: TaskSummaryRecord{
			ID:     "task-1",
			Title:  "Inspect task",
			Status: taskpkg.TaskStatusReady,
			Scope:  taskpkg.ScopeWorkspace,
		},
		CurrentRun: &contract.TaskInspectRunPayload{
			RunID:                   "run-1",
			TaskID:                  "task-1",
			Status:                  taskpkg.TaskRunStatusQueued,
			ClaimTokenHashTruncated: "abcdef12",
			QueuedAt:                now.Add(-10 * time.Minute),
			Attempt:                 1,
		},
		Diagnostics: []contract.DiagnosticItem{{
			ID:            "task.inspect.task_run_stranded.run-1",
			Code:          contract.CodeTaskRunStranded,
			Severity:      contract.SeverityWarn,
			Category:      contract.CategoryTask,
			Title:         "Queued task run has no eligible session",
			Message:       "No eligible session is visible.",
			DataFreshness: contract.FreshnessLive,
		}},
		NextAction: "stranded",
		AsOf:       now,
	}
}

func sampleTaskExecutionProfileRecord() TaskExecutionProfileRecord {
	return TaskExecutionProfileRecord{
		TaskID: "task-1",
		Coordinator: taskpkg.CoordinatorProfile{
			Mode: taskpkg.CoordinatorModeGuided,
		},
		Worker: taskpkg.WorkerProfile{
			Mode:      taskpkg.WorkerModeSelect,
			AgentName: "worker-a",
			Provider:  "openai",
			Model:     "gpt-5.4",
		},
		Review: taskpkg.ReviewProfile{
			AgentName: "reviewer-a",
		},
		Sandbox: taskpkg.SandboxPolicy{
			Mode:       taskpkg.SandboxModeRef,
			SandboxRef: "macos-lab",
		},
		CreatedAt: fixedTestNow,
		UpdatedAt: fixedTestNow,
	}
}

func sampleTaskBridgeNotificationSubscriptionRecord() TaskBridgeNotificationSubscriptionRecord {
	lastDeliveredAt := fixedTestNow.Add(time.Minute)
	cursorUpdatedAt := fixedTestNow.Add(2 * time.Minute)
	return TaskBridgeNotificationSubscriptionRecord{
		SubscriptionID:   "sub-1",
		TaskID:           "task-1",
		BridgeInstanceID: "brg-1",
		Scope:            bridgepkg.ScopeWorkspace,
		WorkspaceID:      "ws-1",
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		DeliveryMode:     bridgepkg.DeliveryModeReply,
		Cursor: contract.TaskBridgeNotificationCursorPayload{
			ConsumerID:      "bridge_task_subscription:sub-1",
			StreamName:      "task_events",
			SubjectID:       "task-1",
			LastSequence:    7,
			LastDeliveryID:  "notif:sub-1:7",
			LastDeliveredAt: &lastDeliveredAt,
			LastError:       "bridge adapter rejected send",
			UpdatedAt:       &cursorUpdatedAt,
		},
		CreatedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
		CreatedAt: fixedTestNow,
		UpdatedAt: fixedTestNow,
	}
}

func sampleTaskRunReviewRecord(status taskpkg.RunReviewStatus) TaskRunReviewRecord {
	return TaskRunReviewRecord{
		ReviewID:          "review-1",
		TaskID:            "task-1",
		RunID:             "run-1",
		Policy:            taskpkg.ReviewPolicyAlways,
		ReviewRound:       1,
		Attempt:           1,
		Status:            status,
		Reason:            "ready for review",
		MissingWork:       json.RawMessage(`[]`),
		ReviewerSessionID: "sess-review",
		RequestedAt:       fixedTestNow,
		CreatedAt:         fixedTestNow,
		UpdatedAt:         fixedTestNow,
	}
}

func timePointer(value time.Time) *time.Time {
	cloned := value
	return &cloned
}

func sampleTaskRunRecord(status taskpkg.RunStatus) TaskRunRecord {
	record := TaskRunRecord{
		ID:                           "run-1",
		TaskID:                       "task-1",
		Status:                       status,
		Origin:                       taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "tasks.run.start"},
		Attempt:                      1,
		IdempotencyKey:               "idem-run",
		ResolvedNetworkParticipation: testLiveResolvedParticipation("builders"),
		QueuedAt:                     fixedTestNow,
	}

	claimedAt := fixedTestNow.Add(time.Minute)
	startedAt := fixedTestNow.Add(2 * time.Minute)
	endedAt := fixedTestNow.Add(3 * time.Minute)
	claimedBy := &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"}

	switch status {
	case taskpkg.TaskRunStatusClaimed:
		record.ClaimedBy = claimedBy
		record.ClaimedAt = timePointer(claimedAt)
	case taskpkg.TaskRunStatusStarting:
		record.ClaimedBy = claimedBy
		record.SessionID = "sess-1"
		record.ClaimedAt = timePointer(claimedAt)
	case taskpkg.TaskRunStatusRunning:
		record.ClaimedBy = claimedBy
		record.SessionID = "sess-1"
		record.ClaimedAt = timePointer(claimedAt)
		record.StartedAt = timePointer(startedAt)
	case taskpkg.TaskRunStatusCompleted:
		record.ClaimedBy = claimedBy
		record.SessionID = "sess-1"
		record.ClaimedAt = timePointer(claimedAt)
		record.StartedAt = timePointer(startedAt)
		record.EndedAt = timePointer(endedAt)
		record.Result = json.RawMessage(`{"ok":true}`)
	case taskpkg.TaskRunStatusFailed:
		record.ClaimedBy = claimedBy
		record.SessionID = "sess-1"
		record.ClaimedAt = timePointer(claimedAt)
		record.StartedAt = timePointer(startedAt)
		record.EndedAt = timePointer(endedAt)
		record.Error = "boom"
	case taskpkg.TaskRunStatusCanceled:
		record.EndedAt = timePointer(endedAt)
	}

	return record
}

func sampleTaskRunDetailRecord(status taskpkg.RunStatus) TaskRunDetailRecord {
	toolCallCount := int64(3)
	turnCount := int64(2)
	totalCost := 0.42
	currency := "USD"
	return TaskRunDetailRecord{
		Run: sampleTaskRunRecord(status),
		Task: &contract.TaskReferencePayload{
			ID:             "task-1",
			Identifier:     "OPS-42",
			Title:          "Investigate flaky task runs",
			Status:         taskpkg.TaskStatusReady,
			Priority:       taskpkg.PriorityHigh,
			Owner:          &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "triage"},
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    "ws-alpha",
			LatestEventSeq: 7,
		},
		Session: &contract.TaskRunSessionPayload{
			SessionID:   "sess-1",
			WorkspaceID: "ws-alpha",
			AgentName:   "coder",
			Name:        "QA coder",
			State:       "active",
			CreatedAt:   fixedTestNow,
			UpdatedAt:   fixedTestNow.Add(time.Minute),
		},
		Summary: contract.TaskRunOperationalSummaryPayload{
			LastActivityAt: fixedTestNow.Add(2 * time.Minute),
			LastEventType:  "agent_message",
			ToolCallCount:  &toolCallCount,
			TurnCount:      &turnCount,
			TotalCost:      &totalCost,
			CostCurrency:   &currency,
			CostStatus:     "estimated",
			CostSource:     "catalog_config",
		},
	}
}

func sampleTaskExecutionRecord() TaskExecutionRecord {
	return TaskExecutionRecord{
		Task: sampleTaskRecord(),
		Run:  sampleTaskRunRecord(taskpkg.TaskRunStatusQueued),
	}
}

func agentTaskClaimRecord() AgentTaskClaimRecord {
	lease := agentTaskLeaseRecord(taskpkg.TaskRunStatusClaimed)
	return AgentTaskClaimRecord{
		Task: contract.TaskReferencePayload{
			ID:          "task-1",
			Identifier:  "AUTO-1",
			Title:       "Run autonomous task",
			Status:      taskpkg.TaskStatusInProgress,
			Priority:    taskpkg.PriorityHigh,
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: "ws-1",
		},
		Run:   sampleTaskRunRecord(taskpkg.TaskRunStatusClaimed),
		Lease: lease,
		CoordinationChannel: &contract.CoordinationChannelPayload{
			ID:                  "builders",
			DisplayName:         "Builders",
			WorkspaceID:         "ws-1",
			TaskID:              "task-1",
			RunID:               "run-1",
			AllowedMessageKinds: contract.CoordinationMessageKinds(),
		},
	}
}

func agentTaskLeaseRecord(status taskpkg.RunStatus) AgentTaskLeaseRecord {
	leaseUntil := fixedTestNow.Add(5 * time.Minute)
	heartbeatAt := fixedTestNow.Add(time.Minute)
	return AgentTaskLeaseRecord{
		TaskID:                       "task-1",
		RunID:                        "run-1",
		Status:                       status,
		SessionID:                    "sess-agent",
		ClaimedBy:                    &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-agent"},
		ClaimTokenHash:               "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		LeaseUntil:                   &leaseUntil,
		HeartbeatAt:                  &heartbeatAt,
		ResolvedNetworkParticipation: testLiveResolvedParticipation("builders"),
	}
}

func taskRunRecord(id string, status taskpkg.RunStatus) TaskRunRecord {
	return TaskRunRecord{
		ID:       id,
		TaskID:   "task-1",
		Status:   status,
		Attempt:  1,
		Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "cli"},
		QueuedAt: fixedTestNow,
	}
}

func sampleTaskDetailRecord() TaskDetailRecord {
	return TaskDetailRecord{
		Task: sampleTaskRecord(),
		Children: []TaskSummaryRecord{
			{
				ID:          "task-child",
				Identifier:  "OPS-43",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-alpha",
				Title:       "Check runtime logs",
				Status:      taskpkg.TaskStatusInProgress,
				Owner:       &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "alice"},
			},
		},
		Dependencies: []TaskDependencyRecord{
			{
				TaskID:          "task-1",
				DependsOnTaskID: "task-blocker",
				Kind:            taskpkg.DependencyKindBlocks,
				CreatedAt:       fixedTestNow,
			},
		},
		Runs: []TaskRunRecord{
			sampleTaskRunRecord(taskpkg.TaskRunStatusRunning),
		},
		Events: []TaskEventRecord{
			{
				ID:        "evt-1",
				TaskID:    "task-1",
				RunID:     "run-1",
				EventType: "task.run_started",
				Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "tasks.run.start"},
				Timestamp: fixedTestNow,
			},
		},
	}
}
