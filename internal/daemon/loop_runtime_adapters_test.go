package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/session"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestLoopActionSessionBinderShouldApplyPolicyGate(t *testing.T) {
	t.Parallel()

	t.Run("Should bind run-agent sessions with workspace policy and allowed tools", func(t *testing.T) {
		t.Parallel()

		allowedTools := []string{
			string(toolspkg.ToolIDTaskRead),
			string(toolspkg.ToolIDTaskUpdate),
		}
		resolved := loopActionBinderWorkspace(t, []aghconfig.AgentDef{{
			Name:        "task-worker",
			Provider:    "mock",
			Prompt:      "Handle the loop node.",
			Tools:       append([]string(nil), allowedTools...),
			Permissions: string(aghconfig.PermissionModeDenyAll),
		}})
		sessions := &loopActionBinderSessionManager{sessionID: "sess-loop-policy"}
		loopParticipation := daemonTestLiveParticipation("ws-loop", "loop-run-channel")
		binder := &loopActionSessionBinder{
			sessions: sessions,
			policyGate: &loopSessionPolicyGate{
				workspaceResolver: loopActionBinderWorkspaceResolver{
					byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
				},
			},
		}

		binding, err := binder.BindActionSession(context.Background(), looppkg.ActionSessionBindRequest{
			WorkspaceID:          looppkg.WorkspaceID("ws-loop"),
			LoopRunID:            looppkg.RunID("loop-run-policy"),
			Agent:                "task-worker",
			Handle:               "execute_task",
			AllowedTools:         []string{allowedTools[1], allowedTools[0], allowedTools[0]},
			ContractBlock:        "Follow the loop contract.",
			NetworkParticipation: new(loopParticipation),
		})
		if err != nil {
			t.Fatalf("BindActionSession() error = %v", err)
		}
		if got, want := binding.SessionID, "sess-loop-policy"; got != want {
			t.Fatalf("binding.SessionID = %q, want %q", got, want)
		}
		createCall := sessions.singleCreateCall(t)
		if got, want := createCall.SandboxRef, "evidence-lab"; got != want {
			t.Fatalf("CreateOpts.SandboxRef = %q, want %q", got, want)
		}
		if got, want := createCall.Permissions, aghconfig.PermissionModeDenyAll; got != want {
			t.Fatalf("CreateOpts.Permissions = %q, want %q", got, want)
		}
		if got := createCall.RuntimeMode; got != "" {
			t.Fatalf("CreateOpts.RuntimeMode = %q, want normal action runtime", got)
		}
		if got, want := createCall.Workspace, "ws-loop"; got != want {
			t.Fatalf("CreateOpts.Workspace = %q, want %q", got, want)
		}
		if got := participationSnapshotValue(createCall.ResolvedNetworkParticipation); got != loopParticipation {
			t.Fatalf("CreateOpts participation = %#v, want owner-bound %#v", got, loopParticipation)
		}
		if got, want := createCall.NetworkOwnerKey, "loop_run:loop-run-policy"; got != want {
			t.Fatalf("CreateOpts.NetworkOwnerKey = %q, want %q", got, want)
		}
		if !slices.Equal(createCall.AllowedToolsOverride, allowedTools) {
			t.Fatalf(
				"CreateOpts.AllowedToolsOverride = %#v, want %#v",
				createCall.AllowedToolsOverride,
				allowedTools,
			)
		}
	})

	t.Run("Should propagate allowed-tools widening failures without a session binding", func(t *testing.T) {
		t.Parallel()

		readTool := string(toolspkg.ToolIDTaskRead)
		updateTool := string(toolspkg.ToolIDTaskUpdate)
		resolved := loopActionBinderWorkspace(t, []aghconfig.AgentDef{{
			Name:     "task-worker",
			Provider: "mock",
			Prompt:   "Handle the loop node.",
			Tools:    []string{readTool},
		}})
		wideningErr := fmt.Errorf(
			"%w: allowed_tools includes %q which widens agent profile",
			session.ErrValidation,
			updateTool,
		)
		sessions := &loopActionBinderSessionManager{createErr: wideningErr}
		binder := &loopActionSessionBinder{
			sessions: sessions,
			policyGate: &loopSessionPolicyGate{
				workspaceResolver: loopActionBinderWorkspaceResolver{
					byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
				},
			},
		}

		_, err := binder.BindActionSession(context.Background(), looppkg.ActionSessionBindRequest{
			WorkspaceID:   looppkg.WorkspaceID("ws-loop"),
			Agent:         "task-worker",
			Handle:        "execute_task",
			AllowedTools:  []string{updateTool},
			ContractBlock: "Follow the loop contract.",
		})
		if !errors.Is(err, session.ErrValidation) {
			t.Fatalf("BindActionSession() error = %v, want session.ErrValidation", err)
		}
		if !strings.Contains(err.Error(), updateTool) {
			t.Fatalf("BindActionSession() error = %v, want rejected tool id", err)
		}
		if got, want := sessions.createCount(), 1; got != want {
			t.Fatalf("Create call count = %d, want %d validation attempt", got, want)
		}
	})

	t.Run("Should fail deterministically when the target agent cannot resolve", func(t *testing.T) {
		t.Parallel()

		resolved := loopActionBinderWorkspace(t, nil)
		sessions := &loopActionBinderSessionManager{sessionID: "unused"}
		binder := &loopActionSessionBinder{
			sessions: sessions,
			policyGate: &loopSessionPolicyGate{
				workspaceResolver: loopActionBinderWorkspaceResolver{
					byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
				},
			},
		}

		_, err := binder.BindActionSession(context.Background(), looppkg.ActionSessionBindRequest{
			WorkspaceID: looppkg.WorkspaceID("ws-loop"),
			Agent:       "missing-worker",
			Handle:      "execute_task",
		})
		if !errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
			t.Fatalf("BindActionSession() error = %v, want ErrAgentNotAvailable", err)
		}
		if got := sessions.createCount(); got != 0 {
			t.Fatalf("Create call count = %d, want 0", got)
		}
	})
}

func TestLoopGateJudgeRunnerShouldApplyPolicyGate(t *testing.T) {
	t.Parallel()

	t.Run("Should force an isolated judge policy and stop the temporary session", func(t *testing.T) {
		t.Parallel()

		resolved := loopActionBinderWorkspace(t, []aghconfig.AgentDef{{
			Name:        "loop-judge",
			Provider:    "mock",
			Prompt:      "Judge the loop gate.",
			Permissions: string(aghconfig.PermissionModeApproveAll),
		}})
		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge",
			events: []acp.AgentEvent{{
				Type: acp.EventTypeAgentMessage,
				Text: `{"verdict":"pass","evidence":{"checked":true}}`,
			}},
		}
		loopParticipation := daemonTestLiveParticipation("ws-loop", "loop-run-channel")
		runner := &loopGateJudgeRunner{
			sessions: sessions,
			policyGate: &loopSessionPolicyGate{
				workspaceResolver: loopActionBinderWorkspaceResolver{
					byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
				},
			},
			executions: newLoopJudgeExecutionRegistry(),
		}

		_, err := runner.Judge(context.Background(), gate.JudgeRequest{
			GateID:               "quality-gate",
			CriterionID:          "review",
			LoopRunID:            "loop-run-judge",
			Attempt:              2,
			CorrelationID:        "goal-judge:stable-attempt",
			WorkspaceID:          "ws-loop",
			Agent:                "loop-judge",
			Rubric:               "Review the evidence.",
			NetworkParticipation: new(loopParticipation),
		})
		if err != nil {
			t.Fatalf("Judge() error = %v", err)
		}
		createCall := sessions.singleCreateCall(t)
		if got, want := createCall.SandboxRef, "evidence-lab"; got != want {
			t.Fatalf("CreateOpts.SandboxRef = %q, want %q", got, want)
		}
		if got, want := createCall.Permissions, aghconfig.PermissionModeDenyAll; got != want {
			t.Fatalf("CreateOpts.Permissions = %q, want %q", got, want)
		}
		if got := participationSnapshotValue(createCall.ResolvedNetworkParticipation); got != loopParticipation {
			t.Fatalf("CreateOpts participation = %#v, want owner-bound %#v", got, loopParticipation)
		}
		if got, want := createCall.NetworkOwnerKey, "loop_run:loop-run-judge"; got != want {
			t.Fatalf("CreateOpts.NetworkOwnerKey = %q, want %q", got, want)
		}
		if got, want := sessions.stopCount(), 1; got != want {
			t.Fatalf("Stop call count = %d, want %d", got, want)
		}
		if got, want := sessions.singleStopID(t), "sess-loop-judge"; got != want {
			t.Fatalf("Stop session ID = %q, want %q", got, want)
		}
		promptCall := sessions.singlePromptCall(t)
		if promptCall.PromptMeta.Judge == nil {
			t.Fatal("PromptOpts.PromptMeta.Judge = nil")
		}
		if got, want := *promptCall.PromptMeta.Judge, (acp.PromptJudgeMeta{
			Role: acp.PromptJudgeRoleAgent, Attempt: 2, CorrelationID: "goal-judge:stable-attempt",
			GateID: "quality-gate", CriterionID: "review",
		}); got != want {
			t.Fatalf("PromptOpts.PromptMeta.Judge = %#v, want %#v", got, want)
		}
	})

	t.Run("Should stop the temporary judge session after malformed output", func(t *testing.T) {
		t.Parallel()

		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge-malformed",
			events: []acp.AgentEvent{{
				Type: acp.EventTypeAgentMessage,
				Text: "not-json",
			}},
		}
		runner := loopJudgeRunnerForTest(t, sessions)
		response, err := runner.Judge(context.Background(), loopJudgeRequestForTest())
		if err != nil {
			t.Fatalf("Judge() error = %v", err)
		}
		if response.Raw != "not-json" {
			t.Fatalf("JudgeResponse.Raw = %q, want malformed output preserved", response.Raw)
		}
		if got := sessions.stopCount(); got != 1 {
			t.Fatalf("Stop call count = %d, want 1", got)
		}
	})

	t.Run("Should reject provider tool activity before accepting a parseable verdict", func(t *testing.T) {
		t.Parallel()

		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge-tool-leak",
			events: []acp.AgentEvent{
				{
					Type:       acp.EventTypeToolCall,
					ToolCallID: "cursor-read-1",
					Title:      "Read repository files",
				},
				{
					Type: acp.EventTypeAgentMessage,
					Text: `{"verdict":"pass","blocking_issues":[],"confidence":1.0,"evidence":{"checked":true}}`,
				},
			},
		}
		_, err := loopJudgeRunnerForTest(t, sessions).Judge(context.Background(), loopJudgeRequestForTest())
		if err == nil || !strings.Contains(err.Error(), "verdict-only judge attempted tool activity") {
			t.Fatalf("Judge() error = %v, want verdict-only tool activity rejection", err)
		}
		if got, want := sessions.cancelCount(), 1; got != want {
			t.Fatalf("CancelPrompt call count = %d, want %d", got, want)
		}
		if got, want := sessions.stopCount(), 1; got != want {
			t.Fatalf("Stop call count = %d, want %d", got, want)
		}
	})

	t.Run("Should use only the final agent message as verdict authority", func(t *testing.T) {
		t.Parallel()

		const verdict = `{"verdict":"pass","blocking_issues":[],"confidence":1.0,"evidence":{"candidate_text":"GREEN"}}`
		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge-reasoning",
			events: []acp.AgentEvent{
				{Type: acp.EventTypeThought, Text: "The candidate satisfies the contract."},
				{Type: acp.EventTypeAgentMessage, Text: verdict},
			},
		}
		response, err := loopJudgeRunnerForTest(t, sessions).Judge(
			context.Background(),
			loopJudgeRequestForTest(),
		)
		if err != nil {
			t.Fatalf("Judge() error = %v", err)
		}
		if got := response.Raw; got != verdict {
			t.Fatalf("JudgeResponse.Raw = %q, want final agent verdict %q", got, verdict)
		}
		result := gate.ParseJudgeVerdict("review", dsl.CriterionAgentJudge, response.Raw)
		if !result.Passed || result.Outcome != gate.VerdictOutcomeApproved {
			t.Fatalf("ParseJudgeVerdict() = %#v, want approved", result)
		}
	})

	t.Run("Should concatenate streamed verdict chunks without inventing separators", func(t *testing.T) {
		t.Parallel()

		const verdict = `{"verdict":"pass","blocking_issues":[],"confidence":1.0,"evidence":{"candidate_text":"GREEN","exact_match":true}}`
		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge-chunked-verdict",
			events: []acp.AgentEvent{
				{Type: acp.EventTypeThought, Text: "The candidate satisfies the contract."},
				{Type: acp.EventTypeAgentMessage, Text: verdict[:96]},
				{Type: acp.EventTypeAgentMessage, Text: verdict[96:]},
			},
		}
		response, err := loopJudgeRunnerForTest(t, sessions).Judge(
			context.Background(),
			loopJudgeRequestForTest(),
		)
		if err != nil {
			t.Fatalf("Judge() error = %v", err)
		}
		if got := response.Raw; got != verdict {
			t.Fatalf("JudgeResponse.Raw = %q, want streamed verdict %q", got, verdict)
		}
		result := gate.ParseJudgeVerdict("review", dsl.CriterionAgentJudge, response.Raw)
		if !result.Passed || result.Outcome != gate.VerdictOutcomeApproved {
			t.Fatalf("ParseJudgeVerdict() = %#v, want approved", result)
		}
	})

	t.Run("Should cancel and join the exact active judge execution", func(t *testing.T) {
		t.Parallel()

		started := make(chan struct{})
		released := make(chan struct{})
		sessions := &loopActionBinderSessionManager{
			sessionID:              "sess-loop-judge-clear",
			blockPromptUntilCancel: true,
			promptStarted:          started,
			promptReleased:         released,
		}
		runner := loopJudgeRunnerForTest(t, sessions)
		runner.executions = newLoopJudgeExecutionRegistry()
		req := loopJudgeRequestForTest()
		req.CorrelationID = "judge-attempt-clear"
		judgeDone := make(chan error, 1)
		go func() {
			_, err := runner.Judge(context.Background(), req)
			judgeDone <- err
		}()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("judge prompt did not start")
		}

		revoker := loopGoalPromptLeaseRevoker{judges: runner}
		if err := revoker.RevokeGoalPromptLease(
			context.Background(),
			looppkg.GoalPromptLease{JudgeAttemptID: "judge-attempt-clear"},
			string(looppkg.TransitionCauseGoalClear),
		); err != nil {
			t.Fatalf("RevokeGoalPromptLease() error = %v", err)
		}
		select {
		case err := <-judgeDone:
			if err != nil {
				t.Fatalf("Judge() error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("judge execution did not join after revoke")
		}
		if got, want := sessions.cancelCount(), 1; got != want {
			t.Fatalf("CancelPrompt call count = %d, want %d", got, want)
		}
		if got, want := sessions.stopCount(), 1; got != want {
			t.Fatalf("Stop call count = %d, want %d", got, want)
		}
	})

	t.Run("Should defer revocation until a creating judge can bind", func(t *testing.T) {
		t.Parallel()

		createStarted := make(chan struct{})
		createReleased := make(chan struct{})
		createCanceled := make(chan struct{})
		sessions := &loopActionBinderSessionManager{
			sessionID:      "sess-loop-judge-creating",
			createStarted:  createStarted,
			createReleased: createReleased,
			createCanceled: createCanceled,
		}
		runner := loopJudgeRunnerForTest(t, sessions)
		runner.executions = newLoopJudgeExecutionRegistry()
		req := loopJudgeRequestForTest()
		req.CorrelationID = "judge-attempt-creating"

		judgeDone := make(chan error, 1)
		go func() {
			_, err := runner.Judge(context.Background(), req)
			judgeDone <- err
		}()
		select {
		case <-createStarted:
		case <-time.After(5 * time.Second):
			t.Fatal("judge creation did not start")
		}

		revokeDone := make(chan error, 1)
		go func() {
			revokeDone <- runner.revokeExecution(context.Background(), req.CorrelationID)
		}()
		waitForCondition(t, "judge revocation recorded before bind", func() bool {
			runner.executions.mu.Lock()
			defer runner.executions.mu.Unlock()
			execution := runner.executions.active[req.CorrelationID]
			return execution != nil && execution.revoked
		})
		select {
		case <-createCanceled:
			t.Fatal("judge creation context canceled before the session could bind")
		default:
		}

		close(createReleased)
		select {
		case err := <-revokeDone:
			if err != nil {
				t.Fatalf("revokeExecution() error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("judge revocation did not join after creation completed")
		}
		select {
		case err := <-judgeDone:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Judge() error = %v, want context.Canceled", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("judge execution did not finish after deferred revocation")
		}
		if got := sessions.cancelCount(); got != 0 {
			t.Fatalf("CancelPrompt call count = %d, want 0 before prompt start", got)
		}
		if got := sessions.stopCount(); got != 1 {
			t.Fatalf("Stop call count = %d, want 1 durable cleanup", got)
		}
		if sessions.stopObservedCanceledContext() {
			t.Fatal("Stop context was canceled, want detached cleanup context")
		}
	})

	t.Run("Should surface judge cleanup failure to the revoker", func(t *testing.T) {
		t.Parallel()

		stopErr := errors.New("persist stopped judge session")
		started := make(chan struct{})
		released := make(chan struct{})
		sessions := &loopActionBinderSessionManager{
			sessionID:              "sess-loop-judge-clear-stop-failure",
			stopErr:                stopErr,
			blockPromptUntilCancel: true,
			promptStarted:          started,
			promptReleased:         released,
		}
		runner := loopJudgeRunnerForTest(t, sessions)
		runner.executions = newLoopJudgeExecutionRegistry()
		req := loopJudgeRequestForTest()
		req.CorrelationID = "judge-attempt-clear-stop-failure"
		judgeDone := make(chan error, 1)
		go func() {
			_, err := runner.Judge(context.Background(), req)
			judgeDone <- err
		}()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("judge prompt did not start")
		}

		revoker := loopGoalPromptLeaseRevoker{judges: runner}
		err := revoker.RevokeGoalPromptLease(
			context.Background(),
			looppkg.GoalPromptLease{JudgeAttemptID: req.CorrelationID},
			string(looppkg.TransitionCauseGoalClear),
		)
		if !errors.Is(err, stopErr) {
			t.Fatalf("RevokeGoalPromptLease() error = %v, want judge cleanup failure", err)
		}
		select {
		case err := <-judgeDone:
			if !errors.Is(err, stopErr) {
				t.Fatalf("Judge() error = %v, want judge cleanup failure", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("judge execution did not join after cleanup failure")
		}
	})

	t.Run("Should fence a judge revoked before runtime registration", func(t *testing.T) {
		t.Parallel()

		sessions := &loopActionBinderSessionManager{sessionID: "sess-loop-judge-late"}
		runner := loopJudgeRunnerForTest(t, sessions)
		runner.executions = newLoopJudgeExecutionRegistry()
		release, err := runner.executions.reserve("judge-attempt-revoked-before-begin")
		if err != nil {
			t.Fatalf("reserve judge execution: %v", err)
		}
		defer release()
		revoker := loopGoalPromptLeaseRevoker{judges: runner}
		if err := revoker.RevokeGoalPromptLease(
			context.Background(),
			looppkg.GoalPromptLease{JudgeAttemptID: "judge-attempt-revoked-before-begin"},
			string(looppkg.TransitionCauseGoalClear),
		); err != nil {
			t.Fatalf("RevokeGoalPromptLease() error = %v", err)
		}
		req := loopJudgeRequestForTest()
		req.CorrelationID = "judge-attempt-revoked-before-begin"
		_, err = runner.Judge(context.Background(), req)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Judge() error = %v, want context.Canceled", err)
		}
		if got := sessions.createCount(); got != 0 {
			t.Fatalf("Create call count = %d, want 0", got)
		}
	})

	t.Run("Should release an abandoned revoked reservation", func(t *testing.T) {
		t.Parallel()

		registry := newLoopJudgeExecutionRegistry()
		release, err := registry.reserve("judge-attempt-abandoned")
		if err != nil {
			t.Fatalf("reserve judge execution: %v", err)
		}
		runner := loopJudgeRunnerForTest(t, &loopActionBinderSessionManager{})
		runner.executions = registry
		if err := runner.revokeExecution(context.Background(), "judge-attempt-abandoned"); err != nil {
			t.Fatalf("revokeExecution() error = %v", err)
		}
		release()

		registry.mu.Lock()
		defer registry.mu.Unlock()
		if len(registry.pending) != 0 || len(registry.revoked) != 0 {
			t.Fatalf(
				"registry pending = %d revoked = %d, want no abandoned state",
				len(registry.pending),
				len(registry.revoked),
			)
		}
	})

	t.Run("Should stop with a detached context after prompt cancellation", func(t *testing.T) {
		t.Parallel()

		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge-canceled",
			promptErr: context.Canceled,
		}
		runner := loopJudgeRunnerForTest(t, sessions)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := runner.Judge(ctx, loopJudgeRequestForTest())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Judge() error = %v, want context.Canceled", err)
		}
		if got := sessions.stopCount(); got != 1 {
			t.Fatalf("Stop call count = %d, want 1", got)
		}
		if sessions.stopObservedCanceledContext() {
			t.Fatal("Stop context was canceled, want detached cleanup context")
		}
	})

	t.Run("Should surface a temporary-session cleanup failure", func(t *testing.T) {
		t.Parallel()

		stopErr := errors.New("stop judge session")
		sessions := &loopActionBinderSessionManager{
			sessionID: "sess-loop-judge-stop-failure",
			events: []acp.AgentEvent{{
				Type: acp.EventTypeAgentMessage,
				Text: `{"verdict":"pass","evidence":{"checked":true}}`,
			}},
			stopErr: stopErr,
		}
		_, err := loopJudgeRunnerForTest(t, sessions).Judge(context.Background(), loopJudgeRequestForTest())
		if !errors.Is(err, stopErr) {
			t.Fatalf("Judge() error = %v, want cleanup failure", err)
		}
		if got := sessions.stopCount(); got != 1 {
			t.Fatalf("Stop call count = %d, want 1", got)
		}
	})

	t.Run("Should stop a partially created session when creation reports failure", func(t *testing.T) {
		t.Parallel()

		createErr := errors.New("create judge session")
		sessions := &loopActionBinderSessionManager{
			sessionID:                "sess-loop-judge-partial-create",
			createErr:                createErr,
			returnSessionOnCreateErr: true,
		}
		_, err := loopJudgeRunnerForTest(t, sessions).Judge(context.Background(), loopJudgeRequestForTest())
		if !errors.Is(err, createErr) {
			t.Fatalf("Judge() error = %v, want create failure", err)
		}
		if got := sessions.stopCount(); got != 1 {
			t.Fatalf("Stop call count = %d, want 1", got)
		}
	})
}

func TestCollectLoopPromptResultShouldNotTreatProtocolRawAsStructuredOutput(t *testing.T) {
	t.Parallel()

	t.Run("Should ignore ACP protocol raw and preserve agent text", func(t *testing.T) {
		t.Parallel()

		manager := loopPromptResultSessionManager{
			events: []acp.AgentEvent{{
				Text: "```json\n{\"summary\":\"done\",\"message\":\"loop channel result\"}\n```",
				Raw:  json.RawMessage(`{"session_update":"agent_message_chunk"}`),
			}},
		}
		result, err := collectLoopPromptResult(
			context.Background(),
			manager,
			"sess-loop",
			looppkg.ActionPromptRequest{Message: "loop event probe"},
		)
		if err != nil {
			t.Fatalf("collectLoopPromptResult() error = %v", err)
		}
		if strings.TrimSpace(result.Text) == "" || !strings.Contains(result.Text, `"summary":"done"`) {
			t.Fatalf("collectLoopPromptResult() text = %q, want agent text", result.Text)
		}
		if len(result.Structured) != 0 {
			t.Fatalf("collectLoopPromptResult() structured = %s, want empty protocol raw ignored", result.Structured)
		}
	})

	t.Run("Should preserve a reported zero token total", func(t *testing.T) {
		t.Parallel()

		zero := int64(0)
		manager := loopPromptResultSessionManager{events: []acp.AgentEvent{{
			Usage: &acp.TokenUsage{TotalTokens: &zero},
		}}}
		var reported []int64
		result, err := collectLoopPromptResult(
			context.Background(),
			manager,
			"sess-loop",
			looppkg.ActionPromptRequest{
				Message: "loop usage probe",
				UsageReporter: looppkg.ActionUsageReporterFunc(func(tokens int64) {
					reported = append(reported, tokens)
				}),
			},
		)
		if err != nil {
			t.Fatalf("collectLoopPromptResult() error = %v", err)
		}
		if result.TokensUsed != 0 || !result.TokensReported || !slices.Equal(reported, []int64{0}) {
			t.Fatalf("collected usage = %#v reports = %#v, want reported zero", result, reported)
		}
	})
}

type loopActionBinderSessionManager struct {
	mu                       sync.Mutex
	sessionID                string
	createErr                error
	returnSessionOnCreateErr bool
	createStarted            chan struct{}
	createReleased           chan struct{}
	createCanceled           chan struct{}
	promptErr                error
	stopErr                  error
	events                   []acp.AgentEvent
	blockPromptUntilCancel   bool
	promptStarted            chan struct{}
	promptReleased           chan struct{}
	promptReleaseOnce        sync.Once
	createCalls              []session.CreateOpts
	promptCalls              []session.PromptOpts
	cancelIDs                []string
	stopIDs                  []string
	stopSawCanceledContexts  []bool
}

func (m *loopActionBinderSessionManager) Status(context.Context, string) (*session.Info, error) {
	return nil, session.ErrSessionNotFound
}

func (m *loopActionBinderSessionManager) Create(
	ctx context.Context,
	opts session.CreateOpts,
) (*session.Session, error) {
	m.mu.Lock()
	m.createCalls = append(m.createCalls, opts)
	createErr := m.createErr
	returnSessionOnCreateErr := m.returnSessionOnCreateErr
	started := m.createStarted
	released := m.createReleased
	canceled := m.createCanceled
	m.mu.Unlock()
	if started != nil {
		close(started)
	}
	if released != nil {
		select {
		case <-released:
		case <-ctx.Done():
			if canceled != nil {
				close(canceled)
			}
			return nil, ctx.Err()
		}
	}
	if createErr != nil && !returnSessionOnCreateErr {
		return nil, createErr
	}
	sessionID := strings.TrimSpace(m.sessionID)
	if sessionID == "" {
		sessionID = "sess-loop-action"
	}
	created := &session.Session{
		ID:                   sessionID,
		Name:                 opts.Name,
		AgentName:            opts.AgentName,
		Model:                opts.Model,
		WorkspaceID:          opts.Workspace,
		Workspace:            firstNonEmpty(opts.Workspace, opts.WorkspacePath),
		NetworkParticipation: daemonTestParticipationFromCreateOpts(opts),
		Type:                 opts.Type,
		State:                session.StateActive,
		CreatedAt:            time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
	}
	return created, createErr
}

func (m *loopActionBinderSessionManager) Prompt(
	ctx context.Context,
	_ string,
	_ string,
) (<-chan acp.AgentEvent, error) {
	m.mu.Lock()
	promptErr := m.promptErr
	configured := append([]acp.AgentEvent(nil), m.events...)
	blockUntilCancel := m.blockPromptUntilCancel
	started := m.promptStarted
	released := m.promptReleased
	m.mu.Unlock()
	if promptErr != nil {
		return nil, promptErr
	}
	if blockUntilCancel {
		events := make(chan acp.AgentEvent)
		if started != nil {
			close(started)
		}
		go func() {
			select {
			case <-released:
			case <-ctx.Done():
			}
			close(events)
		}()
		return events, nil
	}
	events := make(chan acp.AgentEvent, len(configured))
	for _, event := range configured {
		select {
		case events <- event:
		case <-ctx.Done():
			close(events)
			return nil, ctx.Err()
		}
	}
	close(events)
	return events, nil
}

func (m *loopActionBinderSessionManager) PromptWithOpts(
	ctx context.Context,
	_ string,
	opts session.PromptOpts,
) (<-chan acp.AgentEvent, error) {
	m.mu.Lock()
	m.promptCalls = append(m.promptCalls, opts)
	m.mu.Unlock()
	return m.Prompt(ctx, "", opts.Message)
}

func (m *loopActionBinderSessionManager) CancelPrompt(_ context.Context, id string) error {
	m.mu.Lock()
	m.cancelIDs = append(m.cancelIDs, id)
	released := m.promptReleased
	m.mu.Unlock()
	if released != nil {
		m.promptReleaseOnce.Do(func() { close(released) })
	}
	return nil
}

func (m *loopActionBinderSessionManager) StopWithCause(
	ctx context.Context,
	id string,
	_ session.StopCause,
	_ string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopIDs = append(m.stopIDs, id)
	m.stopSawCanceledContexts = append(m.stopSawCanceledContexts, ctx.Err() != nil)
	return m.stopErr
}

func (m *loopActionBinderSessionManager) singleCreateCall(t *testing.T) session.CreateOpts {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.createCalls) != 1 {
		t.Fatalf("Create call count = %d, want 1", len(m.createCalls))
	}
	return m.createCalls[0]
}

func (m *loopActionBinderSessionManager) createCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.createCalls)
}

func (m *loopActionBinderSessionManager) stopCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.stopIDs)
}

func (m *loopActionBinderSessionManager) cancelCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.cancelIDs)
}

func (m *loopActionBinderSessionManager) singleStopID(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.stopIDs) != 1 {
		t.Fatalf("Stop call count = %d, want 1", len(m.stopIDs))
	}
	return m.stopIDs[0]
}

func (m *loopActionBinderSessionManager) stopObservedCanceledContext() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Contains(m.stopSawCanceledContexts, true)
}

type loopActionBinderWorkspaceResolver struct {
	byID   map[string]workspacepkg.ResolvedWorkspace
	byPath map[string]workspacepkg.ResolvedWorkspace
}

func (r loopActionBinderWorkspaceResolver) Resolve(
	_ context.Context,
	idOrPath string,
) (workspacepkg.ResolvedWorkspace, error) {
	resolved, ok := r.byID[strings.TrimSpace(idOrPath)]
	if !ok {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
	return resolved, nil
}

func (r loopActionBinderWorkspaceResolver) ResolveOrRegister(
	_ context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	resolved, ok := r.byPath[strings.TrimSpace(path)]
	if !ok {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
	return resolved, nil
}

func loopActionBinderWorkspace(t *testing.T, agents []aghconfig.AgentDef) workspacepkg.ResolvedWorkspace {
	t.Helper()
	cfg := aghconfig.DefaultWithHome(aghconfig.HomePaths{HomeDir: t.TempDir()})
	cfg.Defaults.Provider = "mock"
	cfg.Defaults.Sandbox = "evidence-lab"
	cfg.Providers["mock"] = aghconfig.ProviderConfig{Command: "mock-acp"}
	cfg.Sandboxes["evidence-lab"] = aghconfig.SandboxProfile{Backend: "local"}
	return workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:         "ws-loop",
			RootDir:    t.TempDir(),
			SandboxRef: "evidence-lab",
		},
		WorkspaceID: "ws-loop",
		Config:      cfg,
		Agents:      append([]aghconfig.AgentDef(nil), agents...),
	}
}

type loopPromptResultSessionManager struct {
	events []acp.AgentEvent
}

func (m loopPromptResultSessionManager) Create(context.Context, session.CreateOpts) (*session.Session, error) {
	return nil, errors.New("unexpected Create call")
}

func (m loopPromptResultSessionManager) Status(context.Context, string) (*session.Info, error) {
	return nil, errors.New("unexpected Status call")
}

func (m loopPromptResultSessionManager) Prompt(
	ctx context.Context,
	_ string,
	_ string,
) (<-chan acp.AgentEvent, error) {
	events := make(chan acp.AgentEvent, len(m.events))
	for _, event := range m.events {
		select {
		case events <- event:
		case <-ctx.Done():
			close(events)
			return nil, ctx.Err()
		}
	}
	close(events)
	return events, nil
}

func (m loopPromptResultSessionManager) PromptWithOpts(
	ctx context.Context,
	_ string,
	_ session.PromptOpts,
) (<-chan acp.AgentEvent, error) {
	return m.Prompt(ctx, "", "")
}

func (m loopPromptResultSessionManager) CancelPrompt(context.Context, string) error {
	return errors.New("unexpected CancelPrompt call")
}

func (m loopPromptResultSessionManager) StopWithCause(
	context.Context,
	string,
	session.StopCause,
	string,
) error {
	return errors.New("unexpected StopWithCause call")
}

func loopJudgeRunnerForTest(
	t *testing.T,
	sessions *loopActionBinderSessionManager,
) *loopGateJudgeRunner {
	t.Helper()
	resolved := loopActionBinderWorkspace(t, []aghconfig.AgentDef{{
		Name:        "loop-judge",
		Provider:    "mock",
		Prompt:      "Judge the loop gate.",
		Permissions: string(aghconfig.PermissionModeApproveAll),
	}})
	return &loopGateJudgeRunner{
		sessions: sessions,
		policyGate: &loopSessionPolicyGate{
			workspaceResolver: loopActionBinderWorkspaceResolver{
				byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": resolved},
			},
		},
		executions: newLoopJudgeExecutionRegistry(),
	}
}

func loopJudgeRequestForTest() gate.JudgeRequest {
	return gate.JudgeRequest{
		GateID:        "quality-gate",
		CriterionID:   "review",
		Attempt:       1,
		CorrelationID: "goal-judge:test",
		WorkspaceID:   "ws-loop",
		Agent:         "loop-judge",
		Rubric:        "Review the evidence.",
	}
}

func (m *loopActionBinderSessionManager) singlePromptCall(t *testing.T) session.PromptOpts {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.promptCalls) != 1 {
		t.Fatalf("PromptWithOpts call count = %d, want 1", len(m.promptCalls))
	}
	return m.promptCalls[0]
}
