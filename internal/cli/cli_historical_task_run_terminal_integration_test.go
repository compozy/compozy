//go:build integration

package cli

import (
	"encoding/json"
	"testing"

	eventspkg "github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestCLIHistoricalChannelTaskRunTerminalAfterDaemonRestartIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		channel        string
		title          string
		terminalArgs   []string
		terminalAgent  bool
		wantRunStatus  taskpkg.RunStatus
		wantTaskStatus taskpkg.Status
		wantEventType  string
	}{
		{
			name:    "Should fail a historical task run after daemon restart",
			channel: "history-run-fail",
			title:   "CLI historical run fail restart",
			terminalArgs: []string{
				"task",
				"fail",
				"--error",
				"operator-detected failure",
				"--metadata",
				`{"source":"integration","mode":"historical-restart"}`,
			},
			terminalAgent:  true,
			wantRunStatus:  taskpkg.TaskRunStatusFailed,
			wantTaskStatus: taskpkg.TaskStatusReady,
			wantEventType:  string(hookspkg.HookTaskRunFailed),
		},
		{
			name:    "Should cancel a historical task run after daemon restart",
			channel: "history-run-cancel",
			title:   "CLI historical run cancel restart",
			terminalArgs: []string{
				"task",
				"run",
				"cancel",
				"--reason",
				"operator-request",
				"--metadata",
				`{"source":"integration","mode":"historical-restart"}`,
			},
			wantRunStatus:  taskpkg.TaskRunStatusCanceled,
			wantTaskStatus: taskpkg.TaskStatusCanceled,
			wantEventType:  eventspkg.TaskRunCanceled,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := newIntegrationHarness(t)
			mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
			defer func() {
				if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
					t.Fatalf("daemon stop during cleanup error = %v", err)
				}
				if err := h.runner.waitForExit(); err != nil {
					t.Fatalf("waitForExit() during cleanup error = %v", err)
				}
			}()

			if _, _, err := executeRootCommand(
				t,
				h.deps,
				"workspace",
				"add",
				h.workspace,
				"--name",
				"alpha",
				"-o",
				"json",
			); err != nil {
				t.Fatalf("workspace add error = %v", err)
			}

			createOut := mustExecuteRoot(
				t,
				h.deps,
				"task",
				"create",
				"--scope",
				"workspace",
				"--workspace",
				"alpha",
				"--network",
				"live",
				"--network-channel-strategy",
				"named",
				"--network-channel",
				tt.channel,
				"--title",
				tt.title,
				"-o",
				"json",
			)
			var created TaskRecord
			if err := json.Unmarshal([]byte(createOut), &created); err != nil {
				t.Fatalf("json.Unmarshal(task create) error = %v", err)
			}
			if created.ID == "" {
				t.Fatalf("created = %#v, want historical task id", created)
			}

			enqueueOut := mustExecuteRoot(
				t,
				h.deps,
				"task",
				"run",
				"enqueue",
				created.ID,
				"--idempotency-key",
				"idem-"+tt.channel,
				"--network",
				"live",
				"--network-channel-strategy",
				"named",
				"--network-channel",
				tt.channel,
				"-o",
				"json",
			)
			var enqueued TaskRunRecord
			if err := json.Unmarshal([]byte(enqueueOut), &enqueued); err != nil {
				t.Fatalf("json.Unmarshal(task run enqueue) error = %v", err)
			}
			if enqueued.ID == "" || enqueued.Status != taskpkg.TaskRunStatusQueued {
				t.Fatalf("enqueued = %#v, want queued run", enqueued)
			}
			if resolvedParticipationChannelID(enqueued.ResolvedNetworkParticipation) != tt.channel {
				t.Fatalf("enqueued = %#v, want preserved historical channel", enqueued)
			}

			mustExecuteRoot(t, h.deps, "daemon", "stop", "-o", "json")
			if err := h.runner.waitForExit(); err != nil {
				t.Fatalf("waitForExit() after stop error = %v", err)
			}
			mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")

			agentDeps, worker := newIntegrationAgentCommandDeps(
				t,
				h,
				tt.channel+"-worker",
				"alpha",
				tt.channel,
			)
			claimOut := mustExecuteRoot(t, agentDeps, "task", "next", "--run-id", enqueued.ID, "-o", "json")
			var next AgentTaskNextRecord
			if err := json.Unmarshal([]byte(claimOut), &next); err != nil {
				t.Fatalf("json.Unmarshal(task next exact claim) error = %v", err)
			}
			if !next.Claimed || next.Claim == nil || next.Claim.Run.ID != enqueued.ID ||
				next.Claim.Run.Status != taskpkg.TaskRunStatusClaimed {
				t.Fatalf("task next = %#v, want exact claimed run %q", next, enqueued.ID)
			}
			if resolvedParticipationChannelID(next.Claim.Run.ResolvedNetworkParticipation) != tt.channel {
				t.Fatalf("task next = %#v, want preserved historical channel", next)
			}

			args := append(append([]string{}, tt.terminalArgs...), enqueued.ID, "-o", "json")
			terminalDeps := h.deps
			if tt.terminalAgent {
				terminalDeps = agentDeps
			}
			terminalOut := mustExecuteRoot(t, terminalDeps, args...)
			if tt.terminalAgent {
				var terminalLease AgentTaskLeaseRecord
				if err := json.Unmarshal([]byte(terminalOut), &terminalLease); err != nil {
					t.Fatalf("json.Unmarshal(terminal lease) error = %v", err)
				}
				if terminalLease.Status != tt.wantRunStatus || terminalLease.RunID != enqueued.ID ||
					terminalLease.SessionID != worker.ID {
					t.Fatalf("terminal lease = %#v, want status %q for worker %q", terminalLease, tt.wantRunStatus, worker.ID)
				}
				if resolvedParticipationChannelID(terminalLease.ResolvedNetworkParticipation) != tt.channel {
					t.Fatalf("terminal lease = %#v, want preserved historical channel", terminalLease)
				}
			} else {
				var terminalRun TaskRunRecord
				if err := json.Unmarshal([]byte(terminalOut), &terminalRun); err != nil {
					t.Fatalf("json.Unmarshal(terminal run) error = %v", err)
				}
				if terminalRun.Status != tt.wantRunStatus || terminalRun.SessionID != worker.ID {
					t.Fatalf("terminalRun = %#v, want status %q for worker %q", terminalRun, tt.wantRunStatus, worker.ID)
				}
				if resolvedParticipationChannelID(terminalRun.ResolvedNetworkParticipation) != tt.channel {
					t.Fatalf("terminalRun = %#v, want preserved historical channel", terminalRun)
				}
			}

			stopWorkerOut := mustExecuteRoot(t, h.deps, "session", "stop", worker.ID, "-o", "json")
			var stoppedWorker SessionRecord
			if err := json.Unmarshal([]byte(stopWorkerOut), &stoppedWorker); err != nil {
				t.Fatalf("json.Unmarshal(session stop worker) error = %v", err)
			}
			if stoppedWorker.State != session.StateStopped {
				t.Fatalf("stopped worker = %#v, want stopped", stoppedWorker)
			}

			getOut := mustExecuteRoot(t, h.deps, "task", "get", created.ID, "-o", "json")
			var detail TaskDetailRecord
			if err := json.Unmarshal([]byte(getOut), &detail); err != nil {
				t.Fatalf("json.Unmarshal(task get) error = %v", err)
			}
			if detail.Task.Status != tt.wantTaskStatus || detail.Summary.Status != tt.wantTaskStatus {
				t.Fatalf("detail = %#v, want task status %q", detail, tt.wantTaskStatus)
			}
			if len(detail.Runs) != 1 {
				t.Fatalf("detail.Runs = %#v, want exactly one run", detail.Runs)
			}
			if detail.Runs[0].Status != tt.wantRunStatus || detail.Runs[0].SessionID != worker.ID {
				t.Fatalf("detail.Runs[0] = %#v, want terminal run with session", detail.Runs[0])
			}
			if resolvedParticipationChannelID(detail.Runs[0].ResolvedNetworkParticipation) != tt.channel {
				t.Fatalf("detail.Runs[0] = %#v, want preserved historical channel", detail.Runs[0])
			}
			if !containsCLITaskEventType(detail.Events, tt.wantEventType) {
				t.Fatalf("detail.Events = %#v, want event type %q", detail.Events, tt.wantEventType)
			}

			assertNoActiveSessions(t, h.deps)
		})
	}
}

func assertCLIHistoricalChannelNotActive(
	t *testing.T,
	deps commandDeps,
	workspace string,
	channel string,
) {
	t.Helper()

	args := []string{"network", "channels"}
	if workspace != "" {
		args = append(args, "--workspace", workspace)
	}
	args = append(args, "-o", "json")
	channelsOut := mustExecuteRoot(t, deps, args...)
	var channels []NetworkChannelRecord
	if err := json.Unmarshal([]byte(channelsOut), &channels); err != nil {
		t.Fatalf("json.Unmarshal(network channels) error = %v", err)
	}
	for _, item := range channels {
		if item.Channel == channel {
			t.Fatalf("historical-only channel %q listed as active: %#v", channel, channels)
		}
	}
}

func containsCLITaskEventType(events []TaskEventRecord, want string) bool {
	for _, event := range events {
		if event.EventType == want {
			return true
		}
	}
	return false
}
