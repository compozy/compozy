//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/remoteproto"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/testutil/remotetest"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const operatorTaskAgent = "s9b-task-worker"

type operatorBridge struct {
	t          *testing.T
	host       daemonAccountFixture
	controller daemonAccountFixture
	computer   remoteproto.ComputerID
}

func TestDaemonComputerOperatorTasks(t *testing.T) {
	root := t.TempDir()
	// IT-043. Safety Invariants 5, 11, 13, 16 and 21. The operator uses the
	// bridge and the agent on the target uses the native lease tools through ACP.
	t.Run("Should operate on the same project and settle both forms of native claims", func(t *testing.T) {
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		project := prepareOperatorProject(t, root)
		hostHome := filepath.Join(root, "host")
		fixturePath := filepath.Join(root, "task-agent.json")
		writeOperatorTaskFixture(t, fixturePath, "pending-inline", "pending-handoff")
		registerComputerACP(t, hostHome, acpmock.RegisterOptions{
			FixturePath: fixturePath, AgentName: operatorTaskAgent, ProviderName: acpmock.ProviderName,
		})

		var targetCapture operatorTrafficCapture
		binary := e2etest.BuildCompozyBinary(t)
		host, _ := startSessionComputer(t, f, hostHome, false,
			daemon.WithRemoteTunnelMiddleware(targetCapture.wrap),
			daemon.WithCompozyExecutableResolver(func() (string, error) { return binary, nil }))
		id := linkSessionComputer(t, host)
		controller, _ := startComputerController(t, f, filepath.Join(root, "controller"), registryTestEmail, nil)
		addAccountComputer(t, controller, id)
		waitComputerRoute(t, controller, id, remoteproto.RouteDirect)

		bridge := operatorBridge{t: t, host: host, controller: controller, computer: id}
		workspace := registerOperatorWorkspace(t, host, project)
		bridge.assertTaskCRUD(workspace)
		inlineRun := bridge.enqueueTask(workspace, taskpkg.WorktreeModeNone)
		handoffRun := bridge.enqueueTask(workspace, taskpkg.WorktreeModePerRun)
		writeOperatorTaskFixture(t, fixturePath, inlineRun, handoffRun)

		inlineSession := bridge.createSession(workspace)
		bridge.assertProjectParity(workspace, inlineSession)
		bridge.assertToolInvoke(workspace, inlineSession)
		bridge.prompt(workspace, inlineSession, "Claim and heartbeat inline")
		bridge.assertNativeCompletion(workspace, inlineRun, inlineSession, false)

		bootstrapSession := bridge.createSession(workspace)
		bridge.prompt(workspace, bootstrapSession, "Claim with handoff")
		bridge.assertNativeCompletion(workspace, handoffRun, bootstrapSession, true)

		claim := "/api/agent/tasks/claim-next"
		bridge.request(http.MethodPost, claim, struct{}{}).expect(t, http.StatusNotFound, "route_not_remote")
		arrivals := targetCapture.requests.Load()
		bridge.send(http.MethodPost, claim, struct{}{}, "fixture-rejected-claim").
			expect(t, http.StatusBadRequest, "claim_token_not_remote")
		if got := targetCapture.requests.Load(); arrivals == 0 || got != arrivals {
			t.Fatalf("tunnel request count before/after rejected claim header = %d/%d", arrivals, got)
		}
		if targetCapture.failed.Load() {
			t.Fatal("could not capture the target tunnel requests and responses")
		}
		if targetCapture.secret.Load() {
			t.Fatal("an operator request or response exposed a raw claim token at the target tunnel")
		}
	})
}

func prepareOperatorProject(t *testing.T, root string) string {
	t.Helper()
	project := filepath.Join(root, "project")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, "notes.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-b", "main"}, {"add", "notes.txt"},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "initial"},
	} {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = project
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("prepare Git project: %v: %s", err, output)
		}
	}
	if err := os.WriteFile(path, []byte("remote project bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return project
}

func (b operatorBridge) assertProjectParity(workspace, session string) {
	t := b.t
	t.Helper()
	project := "/api/workspaces/" + workspace
	for _, probe := range []struct{ path, want string }{
		{project + "/files/content?path=notes.txt", "remote project bytes"},
		{project + "/sessions/" + session + "/changes", "notes.txt"},
	} {
		b.assertReadParity(probe.path, probe.want)
	}
	worktrees := project + "/worktrees"
	for _, origin := range []struct {
		name    string
		request func(string, string, any) authReply
	}{
		{"local", func(method, path string, body any) authReply { return b.host.uds(t, method, path, body) }},
		{"remote", b.request},
	} {
		created := origin.request(http.MethodPost, worktrees, contract.CreateWorktreeRequest{
			Name: origin.name + "-fixture", BaseRef: "main",
		})
		created.expect(t, http.StatusAccepted, origin.name+"-fixture")
		id := authJSON[contract.WorktreeResponse](t, created).Worktree.ID
		path := worktrees + "/" + id
		testutil.Eventually(t, origin.name+" worktree is ready", 8*time.Second, func() bool {
			reply := b.host.uds(t, http.MethodGet, path, nil)
			return reply.StatusCode == http.StatusOK &&
				authJSON[contract.WorktreeInspectionResponse](t, reply).Worktree.State == "ready"
		})
		b.assertReadParity(path, id)
		removed := origin.request(http.MethodDelete, path+"?force=true", nil)
		removed.expect(t, http.StatusNoContent, "")
		b.host.uds(t, http.MethodGet, path, nil).expect(t, http.StatusNotFound, "")
		b.request(http.MethodGet, path, nil).expect(t, http.StatusNotFound, "")
	}
}

func (b operatorBridge) assertTaskCRUD(workspace string) {
	t := b.t
	t.Helper()
	path := b.createTask(workspace, "Bridge task fixture")
	b.assertReadParity("/api/tasks?scope=workspace&workspace="+workspace, "Bridge task fixture")
	b.assertReadParity(path, "Bridge task fixture")

	reply := b.request(http.MethodPatch, path, contract.UpdateTaskRequest{Title: new("Updated bridge task")})
	reply.expect(t, http.StatusOK, "Updated bridge task")
	b.assertReadParity(path, "Updated bridge task")

	reply = b.request(http.MethodPost, path+"/cancel", contract.CancelTaskRequest{Reason: "fixture complete"})
	reply.expect(t, http.StatusOK, "canceled")
	b.assertReadParity(path, "canceled")
	local := b.host.uds(t, http.MethodGet, path, nil)
	if task := authJSON[contract.TaskDetailResponse](t, local).Task.Task; task.Status != taskpkg.TaskStatusCanceled {
		t.Fatalf("target task state after remote cancellation = %s", task.Status)
	}
}

func (b operatorBridge) createTask(workspace, title string) string {
	t := b.t
	t.Helper()
	reply := b.request(http.MethodPost, "/api/tasks", contract.CreateTaskRequest{
		Scope: taskpkg.ScopeWorkspace, Workspace: workspace, Title: title,
	})
	reply.expect(t, http.StatusCreated, title)
	return "/api/tasks/" + authJSON[contract.TaskResponse](t, reply).Task.ID
}

func (b operatorBridge) enqueueTask(workspace string, mode taskpkg.WorktreeMode) string {
	t := b.t
	t.Helper()
	path := b.createTask(workspace, "S9b "+string(mode))
	b.request(http.MethodPut, path+"/execution-profile", contract.SetTaskExecutionProfileRequest{
		Worker:   taskpkg.WorkerProfile{Mode: taskpkg.WorkerModeSelect, AgentName: operatorTaskAgent},
		Worktree: taskpkg.WorktreePolicy{Mode: mode},
	}).expect(t, http.StatusOK, string(mode))
	reply := b.request(http.MethodPost, path+"/runs", contract.EnqueueTaskRunRequest{IdempotencyKey: "s9b-queue"})
	reply.expect(t, http.StatusCreated, "queued")
	run := authJSON[contract.TaskRunResponse](t, reply).Run
	if run.Status != taskpkg.TaskRunStatusQueued || run.SessionID != "" {
		t.Fatalf("enqueued run %q = %s in %q, want queued with no owner", run.ID, run.Status, run.SessionID)
	}
	return run.ID
}

func (b operatorBridge) createSession(workspace string) string {
	t := b.t
	t.Helper()
	reply := b.request(http.MethodPost, controllerSessions, contract.CreateSessionRequest{
		Workspace: workspace, AgentName: operatorTaskAgent, Name: "Bridge task agent",
	})
	reply.expect(t, http.StatusCreated, "session")
	return authJSON[contract.SessionResponse](t, reply).Session.ID
}

func (b operatorBridge) prompt(workspace, session, message string) {
	t := b.t
	t.Helper()
	req := fixturePrompt(message)
	req.IdempotencyKey = message
	req.MessageID = ""
	path := "/api/workspaces/" + workspace + "/sessions/" + session + "/prompt"
	b.request(http.MethodPost, path, req).expect(t, http.StatusOK, "finish")
}

func (b operatorBridge) assertNativeCompletion(workspace, runID, claimer string, handoff bool) {
	t := b.t
	t.Helper()
	path := "/api/task-runs/" + runID
	var run contract.TaskRunPayload
	testutil.Eventually(t, "the lease owner heartbeats "+runID, 15*time.Second, func() bool {
		reply := b.request(http.MethodGet, path, nil)
		reply.expect(t, http.StatusOK, runID)
		run = authJSON[contract.TaskRunResponse](t, reply).Run
		return run.Status == taskpkg.TaskRunStatusRunning && run.ClaimedAt != nil && run.LeaseUntil != nil &&
			run.LeaseUntil.Sub(*run.ClaimedAt) >= 2*time.Minute
	})
	if run.SessionID == "" || (run.SessionID != claimer) != handoff || run.ClaimTokenHash == "" {
		t.Fatalf("lease owner of run %q = %q; claimant %q, expected handoff %t", runID, run.SessionID, claimer, handoff)
	}
	owner := run.SessionID
	b.prompt(workspace, owner, "Complete "+runID)
	reply := b.request(http.MethodGet, path, nil)
	reply.expect(t, http.StatusOK, "s9b-completed")
	completed := authJSON[contract.TaskRunResponse](t, reply).Run
	if completed.ID != runID || completed.Status != taskpkg.TaskRunStatusCompleted || completed.SessionID != owner {
		t.Fatalf("run %q completed projection = %q/%s/%q; want own lease settled by %q",
			runID, completed.ID, completed.Status, completed.SessionID, owner)
	}
}

func (b operatorBridge) assertReadParity(path, want string) {
	t := b.t
	t.Helper()
	local := b.host.uds(t, http.MethodGet, path, nil)
	remote := b.request(http.MethodGet, path, nil)
	local.expect(t, http.StatusOK, want)
	remote.expect(t, http.StatusOK, want)
	if !bytes.Equal(local.body, remote.body) {
		t.Fatalf("bridged read %s differs from the same target record over UDS", path)
	}
}

func (b operatorBridge) assertToolInvoke(workspace, session string) {
	t := b.t
	t.Helper()
	path := "/api/tools/" + toolspkg.ToolIDSessionList.String() + "/invoke"
	b.request(http.MethodPost, path, contract.ToolInvokeRequest{
		WorkspaceID: workspace, Input: json.RawMessage("{}"),
	}).expect(t, http.StatusOK, session)
	for _, field := range []string{"session_id", "agent_name", "turn_id", "approval_token"} {
		b.request(http.MethodPost, path+"?"+field+"=", contract.ToolInvokeRequest{Input: json.RawMessage("{}")}).
			expect(t, http.StatusBadRequest, "remote_caller_override_forbidden")
		b.request(http.MethodPost, path, map[string]any{"input": map[string]any{}, field: nil}).
			expect(t, http.StatusBadRequest, "remote_caller_override_forbidden")
	}
	b.request(http.MethodPost, "/api/tools/"+toolspkg.ToolIDTaskRunClaimNext.String()+"/invoke", struct{}{}).
		expect(t, http.StatusBadRequest, "tool_not_forwardable")
}

func writeOperatorTaskFixture(t *testing.T, path, inlineRun, handoffRun string) {
	t.Helper()
	claim := func(id string) acpmock.Step {
		return operatorTaskStep(toolspkg.ToolIDTaskRunClaimNext, id, ",\"lease_seconds\":60")
	}
	heartbeat := func(id string) acpmock.Step {
		return operatorTaskStep(toolspkg.ToolIDTaskRunHeartbeat, id, ",\"lease_seconds\":180")
	}
	complete := func(id string) acpmock.Step {
		return operatorTaskStep(toolspkg.ToolIDTaskRunComplete, id, ",\"result\":{\"proof\":\"s9b-completed\"}")
	}
	fixture := acpmock.Fixture{Version: acpmock.FixtureVersion, Agents: []acpmock.AgentFixture{{
		Name: operatorTaskAgent, Provider: acpmock.ProviderName, Permissions: "auto_approve",
		Tools: []string{toolspkg.ToolIDTaskRunClaimNext.String(), toolspkg.ToolIDTaskRunHeartbeat.String(),
			toolspkg.ToolIDTaskRunComplete.String()},
		Turns: []acpmock.TurnFixture{
			{Match: acpmock.TurnMatch{UserText: "Claim and heartbeat inline"},
				Steps: []acpmock.Step{claim(inlineRun), heartbeat(inlineRun)}},
			{Match: acpmock.TurnMatch{UserText: "Claim with handoff"}, Steps: []acpmock.Step{claim(handoffRun)}},
			{Match: acpmock.TurnMatch{TurnSource: acp.PromptTurnSourceSynthetic},
				Steps: []acpmock.Step{heartbeat(handoffRun)}},
			{Match: acpmock.TurnMatch{UserText: "Complete " + inlineRun}, Steps: []acpmock.Step{complete(inlineRun)}},
			{Match: acpmock.TurnMatch{UserText: "Complete " + handoffRun}, Steps: []acpmock.Step{complete(handoffRun)}},
		},
	}}}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func operatorTaskStep(tool toolspkg.ToolID, runID, args string) acpmock.Step {
	return acpmock.Step{
		Kind: acpmock.StepKindNativeToolCall, ToolID: tool.String(), ToolCallID: tool.String() + ":" + runID,
		RawInput: json.RawMessage(fmt.Sprintf("{\"run_id\":%q%s}", runID, args)),
	}
}

const operatorProbeHeader = "X-Compozy-Test-Operator-Request"

type operatorTrafficCapture struct {
	requests atomic.Int64
	secret   atomic.Bool
	failed   atomic.Bool
}

func (c *operatorTrafficCapture) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get(operatorProbeHeader) == "" {
			next.ServeHTTP(w, req)
			return
		}
		c.requests.Add(1)
		in, err := httputil.DumpRequest(req, true)
		if err != nil {
			c.failed.Store(true)
		}
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, req)
		res := recorder.Result()
		out, err := httputil.DumpResponse(res, true)
		if err != nil {
			c.failed.Store(true)
		}
		if containsOperatorClaimToken(in) || containsOperatorClaimToken(out) {
			c.secret.Store(true)
		}
		for name, values := range res.Header {
			w.Header()[name] = values
		}
		w.WriteHeader(res.StatusCode)
		_, _ = io.Copy(w, res.Body)
		_ = res.Body.Close()
	})
}

func containsOperatorClaimToken(data []byte) bool {
	return bytes.Contains(data, []byte("compozy_claim_")) ||
		bytes.Contains(data, []byte("\"claim_token\"")) ||
		bytes.Contains(bytes.ToLower(data), []byte("\nx-compozy-claim-token:"))
}

func (b operatorBridge) request(method, path string, body any) authReply {
	b.t.Helper()
	return b.send(method, path, body, "")
}

func (b operatorBridge) send(method, path string, body any, claimToken string) authReply {
	t := b.t
	t.Helper()
	var input io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		input = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(
		t.Context(),
		method,
		b.controller.httpBase+bridgeAPI(b.computer, path),
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(operatorProbeHeader, "1")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if claimToken != "" {
		req.Header.Set("X-Compozy-Claim-Token", claimToken)
	}
	wire, err := httputil.DumpRequest(req, true)
	if err != nil || (claimToken == "" && containsOperatorClaimToken(wire)) {
		t.Fatalf("bridge request %s %s had an unexpected claim token or capture error: %v", method, path, err)
	}
	res, err := b.controller.httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	wire, err = httputil.DumpResponse(res, true)
	if err != nil || containsOperatorClaimToken(wire) {
		t.Fatalf("bridge response %s %s had an unexpected claim token or capture error: %v", method, path, err)
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return authReply{Response: res, body: data}
}
