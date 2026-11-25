// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
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
	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/testutil/remotetest"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/compozy/compozy/internal/worktree"
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
		bridge.prompt(workspace, inlineSession, "s9b-claim-inline", "Claim and heartbeat inline")
		bridge.assertNativeCompletion(workspace, inlineRun, inlineSession, false)

		bootstrapSession := bridge.createSession(workspace)
		bridge.prompt(workspace, bootstrapSession, "s9b-claim-handoff", "Claim with handoff")
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

func TestDaemonComputerOperatorForwarding(t *testing.T) {
	// IT-093. Safety Invariants 5, 11, 16 and 21. Every call uses the real bridge,
	// signed device scopes and the target daemon's operator tool registry.
	t.Run("Should apply source session policy, device scopes, and target tool policy", func(t *testing.T) {
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		root := t.TempDir()
		hostHome := filepath.Join(root, "target")
		var arrivals atomic.Int64
		capture := daemon.WithRemoteTunnelMiddleware(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path == computerInvokePath(toolspkg.ToolIDSessionCreate) ||
					req.URL.Path == computerInvokePath(toolspkg.ToolIDSessionList) {
					arrivals.Add(1)
					if req.Header.Get(agentidentity.HeaderSessionID) != "" ||
						req.Header.Get(agentidentity.HeaderAgent) != "" {
						t.Error("forwarded request included source agent identity")
					}
				}
				next.ServeHTTP(w, req)
			})
		})
		host, ring := startSessionComputer(t, f, hostHome, true, capture)
		computer := linkSessionComputer(t, host)
		workspace := registerOperatorWorkspace(t, host, prepareOperatorProject(t, root))
		controllerHome := filepath.Join(root, "controller")
		const deniedAgent = "forwarding-denied"
		configureForwardingAgents(t, controllerHome, deniedAgent)
		controller, _ := startComputerController(t, f, controllerHome, registryTestEmail, nil)
		device := authJSON[contract.AccountStatusResponse](
			t,
			controller.uds(t, http.MethodGet, contract.AccountPath, nil),
		).Device
		if device == nil {
			t.Fatal("controller did not enroll a device")
		}
		addAccountComputer(t, controller, computer)
		controller.uds(t, http.MethodPatch, computerAPI(computer), contract.PatchComputerRequest{Name: new("devbox")}).
			expect(t, http.StatusOK, "devbox")
		waitComputerRoute(t, controller, computer, remoteproto.RouteDirect)
		allowed := makeForwardingSource(t, controller, computerFixtureAgent)
		denied := makeForwardingSource(t, controller, deniedAgent)
		path := computerInvokePath(toolspkg.ToolIDSessionCreate)
		input, err := json.Marshal(map[string]string{
			"computer": "devbox", "agent": computerFixtureAgent, "workspace": workspace, "name": "Forwarded session",
		})
		if err != nil {
			t.Fatal(err)
		}
		call := contract.ToolInvokeRequest{Input: input, SessionID: allowed}
		created := controller.uds(t, http.MethodPost, path, call)
		created.expect(t, http.StatusOK, "Forwarded session")
		result := authJSON[contract.ToolInvokeResponse](t, created)
		var forwarded contract.SessionResponse
		if err := json.Unmarshal(result.Result.Structured, &forwarded); err != nil ||
			result.Meta == nil || result.Meta.ComputerID != computer ||
			forwarded.Session.ID == "" || forwarded.Session.WorkspaceID != workspace {
			t.Fatalf("forwarded session result = %s (%v)", created.body, err)
		}
		sessionID := forwarded.Session.ID
		host.uds(t, http.MethodGet, controllerSessions+"/"+sessionID, nil).
			expect(t, http.StatusOK, "Forwarded session")
		controller.uds(t, http.MethodGet, controllerSessions+"/"+sessionID, nil).expect(t, http.StatusNotFound, "")
		assertForwardedSessionList(t, controller, host, computer, allowed, sessionID)

		before := arrivals.Load()
		deniedCall := contract.ToolInvokeRequest{Input: input, SessionID: denied}
		sourceFailure := controller.uds(t, http.MethodPost, path, deniedCall)
		sourceFailure.expect(t, http.StatusForbidden, "tool_denied")
		if after := arrivals.Load(); before < 2 || after != before {
			t.Fatalf("target invoke arrivals before/after source refusal = %d/%d", before, after)
		}

		host.stop()
		disableForwardingTargetTools(t, hostHome)
		host, _ = startConfiguredDaemonAccount(t, f, hostHome, daemonAccountBoot{
			direct: true, keyring: ring, options: []daemon.Option{capture},
		})
		waitForHostLease(t, host)
		for _, enabled := range []bool{false, true} {
			controller.uds(t, http.MethodPatch, computerAPI(computer), contract.PatchComputerRequest{Enabled: new(enabled)}).
				expect(t, http.StatusOK, string(computer))
		}
		waitComputerRoute(t, controller, computer, remoteproto.RouteDirect)
		targetFailure := controller.uds(t, http.MethodPost, path, call)
		targetFailure.expect(t, http.StatusForbidden, string(toolspkg.ReasonSourceDisabled))
		if arrivals.Load() != before+1 {
			t.Fatal("the controller did not reach the target's denying tool registry")
		}

		host.uds(t, http.MethodPatch, contract.RemoteDevicesPath+"/"+device.ID, contract.RemoteDevicePatchRequest{
			Scopes: new([]remoteproto.Scope{remoteproto.ScopeFilesR}),
		}).expect(t, http.StatusOK, "files:read")
		testutil.Eventually(t, "target applies the narrowed device scopes", 6*time.Second, func() bool {
			for _, targetDevice := range remoteDaemonDevices(t, host) {
				if string(targetDevice.ID) == device.ID {
					return len(targetDevice.Scopes) == 1 && targetDevice.Scopes[0] == remoteproto.ScopeFilesR
				}
			}
			return false
		})
		f.clock.Advance(3 * time.Second)
		testutil.Eventually(t, "controller reconnects with files:read only", 10*time.Second, func() bool {
			scopes := readControllerComputer(t, controller, computer).Scopes
			return len(scopes) == 1 && scopes[0] == remoteproto.ScopeFilesR
		})
		controller.uds(t, http.MethodPost, path, call).expect(t, http.StatusForbidden, "remote_scope_denied")
	})
}

func configureForwardingAgents(t *testing.T, home, denied string) {
	t.Helper()
	configureComputerACP(t, home)
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	allowedPath := filepath.Join(paths.AgentsDir, computerFixtureAgent, "AGENT.md")
	if _, err := config.EditAgentDefFile(allowedPath, func(agent *config.AgentDef) error {
		agent.WorkspaceAccess = string(workspaceaccess.Unrestricted)
		agent.Tools = []string{toolspkg.ToolIDSessionList.String(), toolspkg.ToolIDSessionCreate.String()}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Register a separate identity against the same provider fixture.
	file, err := os.ReadFile(allowedPath)
	if err != nil {
		t.Fatal(err)
	}
	deniedDir := filepath.Join(paths.AgentsDir, denied)
	if err := os.MkdirAll(deniedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	deniedPath := filepath.Join(deniedDir, "AGENT.md")
	if err := os.WriteFile(deniedPath, file, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.EditAgentDefFile(deniedPath, func(agent *config.AgentDef) error {
		agent.Name = denied
		agent.DenyTools = []string{toolspkg.ToolIDSessionCreate.String()}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func makeForwardingSource(t *testing.T, controller daemonAccountFixture, agent string) string {
	t.Helper()
	created := controller.uds(t, http.MethodPost, controllerSessions, contract.CreateSessionRequest{
		Scope: "user", AgentName: agent, Name: "Forwarding caller",
	})
	created.expect(t, http.StatusCreated, "Forwarding caller")
	id := authJSON[contract.SessionResponse](t, created).Session.ID
	testutil.Eventually(t, "source agent session is active", 6*time.Second, func() bool {
		status := controller.uds(t, http.MethodGet, controllerSessions+"/"+id+"/status", nil)
		return status.StatusCode == http.StatusOK &&
			authJSON[contract.SessionStatusResponse](t, status).LifecycleState == session.StateActive
	})
	return id
}

func assertForwardedSessionList(
	t *testing.T,
	controller, host daemonAccountFixture,
	computer remoteproto.ComputerID,
	caller, projectSessionID string,
) {
	t.Helper()
	// UT-105: an unscoped operator list reads the destination profile's User sessions.
	userSession := host.uds(t, http.MethodPost, controllerSessions, contract.CreateSessionRequest{
		Scope: "user", AgentName: computerFixtureAgent, Name: "Target User session",
	})
	userSession.expect(t, http.StatusCreated, "Target User session")
	userSessionID := authJSON[contract.SessionResponse](t, userSession).Session.ID

	path := computerInvokePath(toolspkg.ToolIDSessionList)
	remote := controller.uds(t, http.MethodPost, path, contract.ToolInvokeRequest{
		SessionID: caller, Input: json.RawMessage(`{"computer":"devbox"}`),
	})
	remote.expect(t, http.StatusOK, userSessionID)
	canonical := host.uds(t, http.MethodPost, path, contract.ToolInvokeRequest{Input: json.RawMessage(`{}`)})
	canonical.expect(t, http.StatusOK, userSessionID)
	want, got := authJSON[contract.ToolInvokeResponse](t, canonical), authJSON[contract.ToolInvokeResponse](t, remote)
	if want.Meta != nil || got.Meta == nil || got.Meta.ComputerID != computer ||
		!bytes.Equal(got.Result.Structured, want.Result.Structured) {
		t.Fatalf("forwarded list differs from target list: got %s, want %s", remote.body, canonical.body)
	}
	if bytes.Contains(got.Result.Structured, []byte(projectSessionID)) {
		t.Fatalf("forwarded User list included project session %s", projectSessionID)
	}
}

func disableForwardingTargetTools(t *testing.T, home string) {
	t.Helper()
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	target, err := config.ResolveConfigWriteTarget(paths, "", config.WriteScopeUser, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.EditConfigOverlay(paths, "", target, func(editor *config.OverlayEditor) error {
		return editor.SetValue([]string{"tools", "enabled"}, false)
	}); err != nil {
		t.Fatal(err)
	}
}

func computerInvokePath(id toolspkg.ToolID) string { return "/api/tools/" + id.String() + "/invoke" }

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
				authJSON[contract.WorktreeInspectionResponse](t, reply).Worktree.State == string(worktree.StateReady)
		})
		before := authJSON[contract.WorktreeInspectionResponse](t, b.assertReadParity(path, id)).Worktree
		if _, err := os.Stat(before.Path); err != nil {
			t.Fatalf("%s worktree checkout before removal: %v", origin.name, err)
		}
		removed := origin.request(http.MethodDelete, path+"?force=true", nil)
		removed.expect(t, http.StatusNoContent, "")
		after := authJSON[contract.WorktreeInspectionResponse](t, b.assertReadParity(path, id)).Worktree
		if after.State != string(worktree.StateRemoved) {
			t.Fatalf("%s worktree state after removal = %q, want removed", origin.name, after.State)
		}
		if _, err := os.Stat(before.Path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s worktree checkout remains after removal: stat error = %v", origin.name, err)
		}
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
		Worker:   taskpkg.WorkerProfile{Mode: taskpkg.WorkerModeInherit, AgentName: operatorTaskAgent},
		Worktree: taskpkg.WorktreePolicy{Mode: mode},
	}).expect(t, http.StatusOK, string(mode))
	reply := b.request(http.MethodPost, path+"/runs", contract.EnqueueTaskRunRequest{
		IdempotencyKey: "s9b-queue-" + string(mode),
	})
	if reply.StatusCode != http.StatusCreated {
		payload := authJSON[contract.ErrorPayload](t, reply)
		t.Fatalf("enqueue %s task: HTTP %d: %s", mode, reply.StatusCode, payload.Error)
	}
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

func (b operatorBridge) prompt(workspace, session, key, message string) {
	t := b.t
	t.Helper()
	req := fixturePrompt(message)
	req.IdempotencyKey = "k_" + key
	req.MessageID = "m_" + key
	path := "/api/workspaces/" + workspace + "/sessions/" + session + "/prompt"
	reply := b.request(http.MethodPost, path, req)
	reply.expect(t, http.StatusOK, `"type":"finish"`)
	for line := range bytes.SplitSeq(reply.body, []byte("\n")) {
		data, ok := bytes.CutPrefix(line, []byte("data: "))
		if !ok || bytes.Equal(data, []byte("[DONE]")) {
			continue
		}
		var frame struct {
			Type      string `json:"type"`
			ErrorText string `json:"errorText"`
			Output    struct {
				Title string `json:"title"`
				Raw   struct {
					Status    string          `json:"status"`
					RawOutput json.RawMessage `json:"rawOutput"`
				} `json:"raw"`
			} `json:"output"`
		}
		if err := json.Unmarshal(data, &frame); err != nil {
			t.Fatalf("decode prompt frame for %q: %v", key, err)
		}
		if frame.Type == "error" {
			t.Fatalf("prompt %q failed: %s", key, diagnostics.RedactAndBound(frame.ErrorText, 512))
		}
		if frame.Type == "tool-output-available" && frame.Output.Raw.Status == "failed" {
			t.Fatalf("prompt %q tool %s failed: %s", key, frame.Output.Title,
				diagnostics.RedactAndBound(string(frame.Output.Raw.RawOutput), 512))
		}
	}
}

func (b operatorBridge) assertNativeCompletion(workspace, runID, claimer string, handoff bool) {
	t := b.t
	t.Helper()
	path := "/api/task-runs/" + runID
	var run contract.TaskRunPayload
	defer func() {
		if t.Failed() {
			t.Logf(
				"last observed lease for %s: status=%s session=%q claimed_at=%v heartbeat_at=%v lease_until=%v error=%q",
				runID,
				run.Status,
				run.SessionID,
				run.ClaimedAt,
				run.HeartbeatAt,
				run.LeaseUntil,
				diagnostics.RedactAndBound(run.Error, 512),
			)
		}
	}()
	testutil.Eventually(t, "the lease owner heartbeats "+runID, 15*time.Second, func() bool {
		reply := b.request(http.MethodGet, path, nil)
		reply.expect(t, http.StatusOK, runID)
		run = authJSON[contract.TaskRunDetailResponse](t, reply).Run.Run
		if run.ID != runID {
			t.Fatalf("task run detail returned ID %q, want %q", run.ID, runID)
		}
		return run.Status == taskpkg.TaskRunStatusRunning && run.ClaimedAt != nil && run.LeaseUntil != nil &&
			run.LeaseUntil.Sub(*run.ClaimedAt) >= 2*time.Minute
	})
	if run.SessionID == "" || (run.SessionID != claimer) != handoff || run.ClaimTokenHash == "" {
		t.Fatalf("lease owner of run %q = %q; claimant %q, expected handoff %t", runID, run.SessionID, claimer, handoff)
	}
	owner := run.SessionID
	statusPath := "/api/workspaces/" + workspace + "/sessions/" + owner + "/status"
	testutil.Eventually(t, "the lease owner's prompt ends", 15*time.Second, func() bool {
		reply := b.request(http.MethodGet, statusPath, nil)
		reply.expect(t, http.StatusOK, owner)
		return authJSON[contract.SessionStatusResponse](t, reply).State == contract.SessionHealthStateIdle
	})
	b.prompt(workspace, owner, "s9b-complete-"+runID, "Complete "+runID)
	reply := b.request(http.MethodGet, path, nil)
	reply.expect(t, http.StatusOK, "s9b-completed")
	completed := authJSON[contract.TaskRunDetailResponse](t, reply).Run.Run
	if completed.ID != runID || completed.Status != taskpkg.TaskRunStatusCompleted || completed.SessionID != owner {
		t.Fatalf("run %q completed projection = %q/%s/%q; want own lease settled by %q",
			runID, completed.ID, completed.Status, completed.SessionID, owner)
	}
}

func (b operatorBridge) assertReadParity(path, want string) authReply {
	t := b.t
	t.Helper()
	local := b.host.uds(t, http.MethodGet, path, nil)
	remote := b.request(http.MethodGet, path, nil)
	local.expect(t, http.StatusOK, want)
	remote.expect(t, http.StatusOK, want)
	if !bytes.Equal(local.body, remote.body) {
		t.Fatalf("bridged read %s differs from the same target record over UDS", path)
	}
	return remote
}

func (b operatorBridge) assertToolInvoke(workspace, session string) {
	t := b.t
	t.Helper()
	// UT-107 target control. Safety Invariant 21 retains workspace selection and refuses caller overrides.
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
		Name: operatorTaskAgent, Provider: acpmock.ProviderName, Permissions: "approve-all",
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
		maps.Copy(w.Header(), res.Header)
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
	if b.computer == "" {
		return b.controller.uds(b.t, method, path, body)
	}
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
