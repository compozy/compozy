//go:build integration

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestCLIHistoricalChannelMixedOwnershipAfterDaemonRestartIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	defer func() {
		if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
			if strings.Contains(err.Error(), "daemon is not running") {
				return
			}
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

	const channel = "history-mixed-ownership"

	sessionOut := mustExecuteRoot(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"history-mixed-ownership-seed",
		"--workspace",
		"alpha",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		channel,
		"-o",
		"json",
	)
	var worker SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &worker); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if worker.ID == "" || worker.State != session.StateActive ||
		resolvedParticipationChannelID(worker.ResolvedNetworkParticipation) != channel {
		t.Fatalf("worker = %#v, want active worker on %q", worker, channel)
	}

	stopOut := mustExecuteRoot(t, h.deps, "session", "stop", worker.ID, "-o", "json")
	var stopped SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stopped); err != nil {
		t.Fatalf("json.Unmarshal(session stop) error = %v", err)
	}
	if stopped.State != session.StateStopped ||
		resolvedParticipationChannelID(stopped.ResolvedNetworkParticipation) != channel {
		t.Fatalf("stopped = %#v, want stopped worker on %q", stopped, channel)
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
		channel,
		"--title",
		"CLI historical mixed ownership",
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
		"idem-cli-history-mixed-ownership",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		channel,
		"-o",
		"json",
	)
	var enqueued TaskRunRecord
	if err := json.Unmarshal([]byte(enqueueOut), &enqueued); err != nil {
		t.Fatalf("json.Unmarshal(task run enqueue) error = %v", err)
	}
	if enqueued.Status != taskpkg.TaskRunStatusQueued {
		t.Fatalf("enqueued = %#v, want queued run", enqueued)
	}
	if resolvedParticipationChannelID(enqueued.ResolvedNetworkParticipation) != channel {
		t.Fatalf("enqueued = %#v, want preserved historical channel", enqueued)
	}

	agentSessionID := worker.ID
	agentDeps := h.deps
	agentDeps.getenv = func(key string) string {
		switch key {
		case agentidentity.EnvSessionID:
			return agentSessionID
		case agentidentity.EnvAgent:
			return worker.AgentName
		default:
			return ""
		}
	}

	var claim AgentTaskNextRecord

	// Intentionally serial: each subtest advances the same historical run lifecycle.
	t.Run("Should keep stopped participation historical without listing an active channel", func(t *testing.T) {
		assertCLIHistoricalChannelNotActive(t, h.deps, "alpha", channel)
	})

	t.Run("Should reclaim the historical run after daemon restart", func(t *testing.T) {
		if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
			t.Fatalf("daemon stop before restart error = %v", err)
		}
		if err := h.runner.waitForExit(); err != nil {
			t.Fatalf("waitForExit(before restart) error = %v", err)
		}
		mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")

		resumeOut := mustExecuteRoot(
			t,
			h.deps,
			"session",
			"new",
			"--agent",
			"coder",
			"--name",
			"history-mixed-ownership-restarted",
			"--workspace",
			"alpha",
			"--network",
			"live",
			"--network-channel-strategy",
			"named",
			"--network-channel",
			channel,
			"-o",
			"json",
		)
		var resumed SessionRecord
		if err := json.Unmarshal([]byte(resumeOut), &resumed); err != nil {
			t.Fatalf("json.Unmarshal(session new after restart) error = %v", err)
		}
		if resumed.State != session.StateActive ||
			resolvedParticipationChannelID(resumed.ResolvedNetworkParticipation) != channel {
			t.Fatalf("resumed = %#v, want active restarted worker on %q", resumed, channel)
		}
		agentSessionID = resumed.ID

		nextOut := mustExecuteRoot(t, agentDeps, "task", "next", "--lease-seconds", "60", "-o", "json")
		if err := json.Unmarshal([]byte(nextOut), &claim); err != nil {
			t.Fatalf("json.Unmarshal(task next) error = %v", err)
		}
		if !claim.Claimed || claim.Claim == nil {
			t.Fatalf("claim = %#v, want claimed historical run", claim)
		}
		if claim.Claim.Run.ID != enqueued.ID {
			t.Fatalf("claim.Claim.Run.ID = %q, want %q", claim.Claim.Run.ID, enqueued.ID)
		}
		if resolvedParticipationChannelID(claim.Claim.Run.ResolvedNetworkParticipation) != channel {
			t.Fatalf("claim.Claim.Run = %#v, want preserved historical channel", claim.Claim.Run)
		}
		if claim.Claim.CoordinationChannel == nil {
			t.Fatal("claim.Claim.CoordinationChannel = nil, want coordination channel")
		}
		if got, want := firstCLIValue(
			claim.Claim.CoordinationChannel.ID,
			claim.Claim.CoordinationChannel.ID,
		), channel; got != want {
			t.Fatalf("coordination channel = %q, want %q", got, want)
		}
		if claim.Claim.Lease.ClaimTokenHash == "" {
			t.Fatal("claim.Claim.Lease.ClaimTokenHash = empty, want observability hash")
		}
		if strings.Contains(nextOut, `"claim_token"`) || strings.Contains(nextOut, "agh_claim_") {
			t.Fatalf("task next output exposed raw claim token: %s", nextOut)
		}
	})

	t.Run("Should reject human terminal completion and failure without claim token", func(t *testing.T) {
		for _, tt := range []struct {
			name        string
			args        []string
			wantMessage string
		}{
			{
				name:        "Should reject human task run complete",
				args:        []string{"task", "run", "complete", enqueued.ID, "--result", `{"ok":true,"path":"human-cli-complete"}`, "-o", "json"},
				wantMessage: "requires token-fenced completion",
			},
			{
				name:        "Should reject human task run fail",
				args:        []string{"task", "run", "fail", enqueued.ID, "--error", "human-cli-fail", "-o", "json"},
				wantMessage: "requires token-fenced failure",
			},
		} {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				exitCode, _, stderr := executeRootCommandWithExit(t, h.deps, tt.args...)
				if exitCode == 0 {
					t.Fatalf("executeRootCommandWithExit(%v) exitCode = %d, want non-zero", tt.args, exitCode)
				}
				if !strings.Contains(stderr, "invalid claim token") || !strings.Contains(stderr, tt.wantMessage) {
					t.Fatalf("stderr = %q, want invalid token + %q", stderr, tt.wantMessage)
				}
				if strings.Contains(stderr, "agh_claim_") {
					t.Fatalf("stderr leaked raw claim token: %s", stderr)
				}
			})
		}
	})

	t.Run("Should allow human cancel override and preserve historical bindings", func(t *testing.T) {
		cancelOut := mustExecuteRoot(
			t,
			h.deps,
			"task",
			"run",
			"cancel",
			enqueued.ID,
			"--reason",
			"human-cli-override",
			"-o",
			"json",
		)
		var canceled TaskRunRecord
		if err := json.Unmarshal([]byte(cancelOut), &canceled); err != nil {
			t.Fatalf("json.Unmarshal(task run cancel) error = %v", err)
		}
		if canceled.Status != taskpkg.TaskRunStatusCanceled {
			t.Fatalf("canceled = %#v, want canceled run", canceled)
		}
		if canceled.ClaimedBy == nil || canceled.ClaimedBy.Ref != agentSessionID {
			t.Fatalf("canceled.ClaimedBy = %#v, want agent-session %q", canceled.ClaimedBy, agentSessionID)
		}
		if canceled.SessionID != agentSessionID {
			t.Fatalf("canceled.SessionID = %q, want %q", canceled.SessionID, agentSessionID)
		}
		if resolvedParticipationChannelID(canceled.ResolvedNetworkParticipation) != channel {
			t.Fatalf("canceled = %#v, want preserved historical channel", canceled)
		}
	})

	t.Run("Should reject stale agent completion after human cancel", func(t *testing.T) {
		exitCode, _, stderr := executeRootCommandWithExit(
			t,
			agentDeps,
			"task",
			"complete",
			enqueued.ID,
			"--result",
			`{"ok":true,"path":"stale-agent-after-cancel"}`,
			"-o",
			"json",
		)
		if exitCode == 0 {
			t.Fatalf("executeRootCommandWithExit(task complete stale) exitCode = %d, want non-zero", exitCode)
		}
		if !strings.Contains(stderr, "not an active lease") {
			t.Fatalf("stderr = %q, want inactive lease rejection", stderr)
		}
		if strings.Contains(stderr, "agh_claim_") {
			t.Fatalf("stderr leaked raw claim token: %s", stderr)
		}
	})

	t.Run("Should persist the canceled historical run and leave no active sessions", func(t *testing.T) {
		getOut := mustExecuteRoot(t, h.deps, "task", "get", created.ID, "-o", "json")
		var detail TaskDetailRecord
		if err := json.Unmarshal([]byte(getOut), &detail); err != nil {
			t.Fatalf("json.Unmarshal(task get) error = %v", err)
		}
		if detail.Task.Status != taskpkg.TaskStatusCanceled || detail.Summary.Status != taskpkg.TaskStatusCanceled {
			t.Fatalf("detail = %#v, want canceled task detail", detail)
		}
		if got, want := len(detail.Runs), 1; got != want {
			t.Fatalf("len(detail.Runs) = %d, want %d", got, want)
		}
		if detail.Runs[0].Status != taskpkg.TaskRunStatusCanceled {
			t.Fatalf("detail.Runs[0] = %#v, want canceled run", detail.Runs[0])
		}
		if detail.Runs[0].SessionID != agentSessionID {
			t.Fatalf("detail.Runs[0].SessionID = %q, want %q", detail.Runs[0].SessionID, agentSessionID)
		}
		if detail.Runs[0].ClaimedBy == nil || detail.Runs[0].ClaimedBy.Ref != agentSessionID {
			t.Fatalf("detail.Runs[0].ClaimedBy = %#v, want agent-session %q", detail.Runs[0].ClaimedBy, agentSessionID)
		}
		if resolvedParticipationChannelID(detail.Runs[0].ResolvedNetworkParticipation) != channel {
			t.Fatalf("detail.Runs[0] = %#v, want preserved historical channel", detail.Runs[0])
		}
		if !containsCLITaskEventType(detail.Events, "task.run_canceled") {
			t.Fatalf("detail.Events = %#v, want task.run_canceled event", detail.Events)
		}

		stopOut := mustExecuteRoot(t, h.deps, "session", "stop", agentSessionID, "-o", "json")
		var stoppedAfterResume SessionRecord
		if err := json.Unmarshal([]byte(stopOut), &stoppedAfterResume); err != nil {
			t.Fatalf("json.Unmarshal(session stop after resume) error = %v", err)
		}
		if stoppedAfterResume.State != session.StateStopped ||
			resolvedParticipationChannelID(stoppedAfterResume.ResolvedNetworkParticipation) != channel {
			t.Fatalf("stoppedAfterResume = %#v, want stopped worker on %q", stoppedAfterResume, channel)
		}

		assertNoActiveSessions(t, h.deps)
	})
}
