package cli

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestParseSchedulerDrainTimeout(t *testing.T) {
	t.Parallel()

	maximum := contract.SchedulerDrainTimeoutMaxSeconds
	testCases := []struct {
		name        string
		raw         string
		wantSeconds int64
		wantErr     bool
	}{
		{name: "Should accept zero", raw: "0s"},
		{name: "Should reject negative values", raw: "-1s", wantErr: true},
		{name: "Should reject sub-second precision", raw: "1500ms", wantErr: true},
		{
			name:        "Should accept the exact duration ceiling",
			raw:         strconv.FormatInt(maximum, 10) + "s",
			wantSeconds: maximum,
		},
		{
			name:    "Should reject one second above the duration ceiling",
			raw:     strconv.FormatInt(maximum+1, 10) + "s",
			wantErr: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			seconds, err := parseSchedulerDrainTimeout(testCase.raw)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("parseSchedulerDrainTimeout(%q) = %d, want error", testCase.raw, seconds)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSchedulerDrainTimeout(%q) error = %v", testCase.raw, err)
			}
			if seconds != testCase.wantSeconds {
				t.Fatalf(
					"parseSchedulerDrainTimeout(%q) = %d, want %d",
					testCase.raw,
					seconds,
					testCase.wantSeconds,
				)
			}
		})
	}
}

func TestSchedulerCommandsMapRequests(t *testing.T) {
	t.Parallel()

	t.Run("Should render scheduler status", func(t *testing.T) {
		t.Parallel()

		stdout, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, &stubClient{
			schedulerStatusFn: func(context.Context) (SchedulerStatusRecord, error) {
				return SchedulerStatusRecord{
					Paused:           true,
					PausedBy:         "human:operator",
					PausedReason:     "deploy freeze",
					ActiveClaimCount: 2,
					QueuedRunCount:   3,
					PausedTaskCount:  1,
					AsOf:             fixedTestNow,
				}, nil
			},
		}), "scheduler", "status", "-o", "json")
		if err != nil {
			t.Fatalf("scheduler status error = %v", err)
		}
		var output SchedulerStatusRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(scheduler status) error = %v", err)
		}
		if !output.Paused || output.QueuedRunCount != 3 || output.PausedTaskCount != 1 {
			t.Fatalf("scheduler status output = %#v, want pause and queue pressure", output)
		}
	})

	t.Run("Should map scheduler pause request", func(t *testing.T) {
		t.Parallel()

		var request SchedulerPauseRequest
		deps := newWorkspaceTestDeps(t, &stubClient{
			pauseSchedulerFn: func(_ context.Context, got SchedulerPauseRequest) (SchedulerStatusRecord, error) {
				request = got
				return SchedulerStatusRecord{Paused: true, PausedReason: got.Reason, AsOf: fixedTestNow}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"scheduler",
			"pause",
			"--reason",
			"deploy freeze",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("scheduler pause error = %v", err)
		}
		if request.Reason != "deploy freeze" {
			t.Fatalf("pause request = %#v, want reason", request)
		}
		var output SchedulerStatusRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(scheduler pause) error = %v", err)
		}
		if !output.Paused || output.PausedReason != "deploy freeze" {
			t.Fatalf("scheduler pause output = %#v, want paused reason", output)
		}
	})

	t.Run("Should map scheduler drain timeout", func(t *testing.T) {
		t.Parallel()

		var request SchedulerDrainRequest
		deps := newWorkspaceTestDeps(t, &stubClient{
			drainSchedulerFn: func(_ context.Context, got SchedulerDrainRequest) (SchedulerDrainRecord, error) {
				request = got
				return SchedulerDrainRecord{
					Scheduler:   SchedulerStatusRecord{Paused: true, AsOf: fixedTestNow},
					Completed:   true,
					StartedAt:   fixedTestNow,
					CompletedAt: fixedTestNow.Add(time.Second),
				}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"scheduler",
			"drain",
			"--reason",
			"deploy freeze",
			"--timeout",
			"2s",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("scheduler drain error = %v", err)
		}
		if request.Reason != "deploy freeze" || request.TimeoutSeconds == nil || *request.TimeoutSeconds != 2 {
			t.Fatalf("drain request = %#v, want reason and 2s timeout", request)
		}
		var output SchedulerDrainRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(scheduler drain) error = %v", err)
		}
		if !output.Completed || !output.Scheduler.Paused {
			t.Fatalf("scheduler drain output = %#v, want completed paused result", output)
		}
	})

	t.Run("Should map scheduler backlog filters", func(t *testing.T) {
		t.Parallel()

		var query SchedulerBacklogQuery
		deps := newWorkspaceTestDeps(t, &stubClient{
			schedulerBacklogFn: func(_ context.Context, got SchedulerBacklogQuery) (SchedulerBacklogRecord, error) {
				query = got
				taskRecord := sampleTaskSummaryRecord()
				taskRecord.EffectivePaused = true
				taskRecord.PausedByTaskID = "task-root"
				return SchedulerBacklogRecord{
					Total: 1,
					Runs: []contract.SchedulerBacklogRunPayload{{
						Task: taskRecord,
						Run:  sampleTaskRunRecord(taskpkg.TaskRunStatusQueued),
					}},
				}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"scheduler",
			"backlog",
			"--last",
			"7",
			"--workspace",
			"ws-alpha",
			"--include-paused",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("scheduler backlog error = %v", err)
		}
		if query.Limit != 7 || query.WorkspaceID != "ws-alpha" || !query.IncludePaused {
			t.Fatalf("backlog query = %#v, want parsed filters", query)
		}
		var output SchedulerBacklogRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(scheduler backlog) error = %v", err)
		}
		if output.Total != 1 || len(output.Runs) != 1 || !output.Runs[0].Task.EffectivePaused {
			t.Fatalf("scheduler backlog output = %#v, want paused run", output)
		}
	})
}
