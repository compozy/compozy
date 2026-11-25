// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jonboulle/clockwork"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/remoteproto"
	taskpkg "github.com/compozy/compozy/internal/task"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestDaemonComputerOperatorBridgeIsolation(t *testing.T) {
	root := t.TempDir()
	// IT-048. Safety Invariant 11. A is a working controller for B, but its
	// authenticated remote device cannot use that registry as a second hop.
	t.Run("Should refuse a second hop without a request reaching the next computer", func(t *testing.T) {
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		var targetCapture operatorTrafficCapture
		first, _ := startSessionComputer(t, f, filepath.Join(root, "a"), false)
		second, _ := startSessionComputer(t, f, filepath.Join(root, "b"), false,
			daemon.WithRemoteTunnelMiddleware(targetCapture.wrap))
		provisionRegistryPro(t, f)
		a, b := linkSessionComputer(t, first), linkSessionComputer(t, second)
		controller, _ := startComputerController(t, f, filepath.Join(root, "ctl"), registryTestEmail, nil)
		addAccountComputer(t, first, b)
		addAccountComputer(t, controller, a)
		waitComputerRoute(t, first, b, remoteproto.RouteDirect)
		waitComputerRoute(t, controller, a, remoteproto.RouteDirect)

		sessions := makeSessionIDs(t, second, "Second computer")
		path := controllerSessions + "?all_workspaces=true"
		firstToSecond := operatorBridge{t: t, controller: first, computer: b}
		expectComputerSessions(t, firstToSecond.request(http.MethodGet, path, nil), sessions)
		arrivals := targetCapture.requests.Load()
		if arrivals != 1 || targetCapture.failed.Load() {
			t.Fatalf("positive control: observed %d requests at the second computer, want one", arrivals)
		}

		controllerToFirst := operatorBridge{t: t, controller: controller, computer: a}
		controllerToFirst.request(http.MethodGet, bridgeAPI(b, path), nil).
			expect(t, http.StatusForbidden, "remote_bridge_not_transitive")
		if got := targetCapture.requests.Load(); got != arrivals {
			t.Fatalf("a chained request reached the second computer: before/after %d/%d", arrivals, got)
		}
	})
}

// The daemon computer journey owns the terminal process and replay across a real bridge.
func TestDaemonComputerOperatorTerminal(t *testing.T) {
	root := t.TempDir()
	t.Run("Should open, write, resume and revoke a PTY on the target daemon", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("the terminal fixture uses a Unix shell and FIFO")
		}
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		hostHome, controllerHome := filepath.Join(root, "host"), filepath.Join(root, "controller")
		configureOperatorProvider(t, hostHome, true)
		configureOperatorProvider(t, controllerHome, false)
		host, _ := startSessionComputer(t, f, hostHome, false)
		id := linkSessionComputer(t, host)
		controller, ring := startConfiguredDaemonAccount(t, f, controllerHome, daemonAccountBoot{})
		progress := controller.stream(t, contract.AccountSignInEventsPath, "")
		device := signInAccountDevice(t, f, controller, progress, registryTestEmail).Device.ID
		progress.cancel()
		addAccountComputer(t, controller, id)
		waitComputerRoute(t, controller, id, remoteproto.RouteDirect)
		assertRemoteProviderState(t, controller, host, id)

		project := filepath.Join(root, "project")
		if err := os.Mkdir(project, 0o700); err != nil {
			t.Fatal(err)
		}
		fifo := prepareOperatorTerminalReplay(t, project)
		workspace := registerOperatorWorkspace(t, host, project)
		path := openOperatorTerminal(t, controller, id, workspace)
		socket := attachOperatorTerminal(t, controller, id, path, 0)
		cursor := startOperatorTerminalReplay(t, socket)

		// IT-041. Only the controller stops; the target produces the intervening output.
		controller.stop()
		releaseOperatorTerminalReplay(t, fifo)
		controller, _ = startConfiguredDaemonAccount(t, f, controllerHome, daemonAccountBoot{keyring: ring})
		waitComputerRoute(t, controller, id, remoteproto.RouteDirect)
		resumed := attachOperatorTerminal(t, controller, id, path, cursor)
		replay, _ := readOperatorOutput(t, resumed, "\r\ns9b-replay-c\r\n")
		if !strings.HasPrefix(replay, "s9b-replay-a\r\ns9b-replay-b\r\ns9b-replay-c\r\n") {
			t.Fatalf("terminal replay after %d missed output: %q", cursor, replay)
		}
		narrowOperatorTerminalScope(t, f.clock, host, controller, id, device, resumed)
	})
}

func registerOperatorWorkspace(t *testing.T, host daemonAccountFixture, root string) string {
	t.Helper()
	reply := host.uds(t, http.MethodPost, "/api/workspaces", contract.CreateWorkspaceRequest{
		RootDir: root, Name: "Remote operator project",
	})
	reply.expect(t, http.StatusCreated, "workspace")
	return authJSON[contract.WorkspaceResponse](t, reply).Workspace.ID
}

func openOperatorTerminal(
	t *testing.T, controller daemonAccountFixture, id remoteproto.ComputerID, workspace string,
) string {
	t.Helper()
	// IT-040. Safety Invariants 5, 11 and 18. Creation uses the local workspace route through UDS.
	path := "/api/workspaces/" + workspace + "/terminals"
	reply := controller.uds(t, http.MethodPost, bridgeAPI(id, path), contract.TerminalCreateRequest{
		Shell: "sh", Cols: 80, Rows: 24, Title: "S9b shell", ClientID: "controller-window",
	})
	reply.expect(t, http.StatusCreated, "S9b shell")
	terminal := authJSON[contract.TerminalResponse](t, reply).Terminal
	const aggregate = "/api/terminals?all_workspaces=true"
	controller.request(t, http.MethodGet, bridgeAPI(id, aggregate), nil).
		expect(t, http.StatusOK, string(terminal.ID))
	local := controller.request(t, http.MethodGet, aggregate, nil)
	local.expect(t, http.StatusOK, "terminals")
	if bytes.Contains(local.body, []byte(terminal.ID)) {
		t.Fatal("a remote terminal was registered on the controller")
	}
	return path + "/" + string(terminal.ID)
}

func attachOperatorTerminal(
	t *testing.T, controller daemonAccountFixture, id remoteproto.ComputerID, path string, after uint64,
) *websocket.Conn {
	t.Helper()
	return attachDaemonTerminal(t, controller, bridgeAPI(id, path), after)
}

func attachDaemonTerminal(t *testing.T, controller daemonAccountFixture, path string, after uint64) *websocket.Conn {
	t.Helper()
	reply := controller.request(t, http.MethodPost, path+"/attach-ticket",
		contract.TerminalAttachTicketRequest{Mode: contract.TerminalAttachModeWrite})
	reply.expect(t, http.StatusCreated, "ticket")
	ticket := authJSON[contract.TerminalAttachTicketResponse](t, reply).Ticket
	query := url.Values{
		"mode": {"write"}, "flow": {"ack"}, "ticket": {ticket}, "after_seq": {strconv.FormatUint(after, 10)},
	}
	target := "ws" + strings.TrimPrefix(controller.httpBase, "http") + path + "/stream?" + query.Encode()
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second, Subprotocols: []string{terminalwire.Subprotocol}}
	socket, response, err := dialer.DialContext(t.Context(), target, http.Header{"Origin": {controller.httpBase}})
	if err != nil {
		if response != nil && response.Body != nil {
			body, _ := io.ReadAll(response.Body)
			_ = response.Body.Close()
			t.Fatalf("attach target terminal: HTTP %d %s: %v", response.StatusCode, body, err)
		}
		t.Fatalf("attach target terminal: %v", err)
	}
	t.Cleanup(func() { _ = socket.Close() })
	if got := readOperatorTerminalFrame(t, socket); got.Op != terminalwire.ServerOpAttached {
		t.Fatalf("first terminal frame = %d, want attached", got.Op)
	}
	return socket
}

func startOperatorTerminalReplay(t *testing.T, socket *websocket.Conn) uint64 {
	t.Helper()
	// Print markers in a form distinguishable from the PTY's echo of the command.
	input := "echo hi; printf 's9b-pid-%s\\n' \"$$\"; printf 's9b-%s\\n' ready; " +
		"read line < resume; printf 's9b-replay-%s\\n' a b c\n"
	frame, err := terminalwire.EncodeClient(terminalwire.Frame{Op: terminalwire.ClientOpInput, Payload: []byte(input)})
	if err != nil {
		t.Fatal(err)
	}
	if err := socket.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		t.Fatal(err)
	}
	output, cursor := readOperatorOutput(t, socket, "\r\ns9b-ready\r\n")
	if !strings.Contains(output, "\r\nhi\r\n") {
		t.Fatalf("echo output did not come from the remote PTY: %q", output)
	}
	_, marker, ok := strings.Cut(output, "\r\ns9b-pid-")
	pid, _, _ := strings.Cut(marker, "\r\n")
	if _, err := strconv.Atoi(pid); !ok || err != nil {
		t.Fatalf("remote shell PID missing from %q", output)
	}
	if result, err := exec.CommandContext(t.Context(), "ps", "-p", pid, "-o", "comm=").CombinedOutput(); err != nil ||
		len(bytes.TrimSpace(result)) == 0 {
		t.Fatalf("remote shell PID %s absent from the target process table: %s: %v", pid, result, err)
	}
	return cursor
}

func prepareOperatorTerminalReplay(t *testing.T, project string) string {
	t.Helper()
	fifo := filepath.Join(project, "resume")
	if output, err := exec.CommandContext(t.Context(), "mkfifo", fifo).CombinedOutput(); err != nil {
		t.Fatalf("create terminal replay fixture: %s: %v", output, err)
	}
	return fifo
}

func releaseOperatorTerminalReplay(t *testing.T, fifo string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()
	if result, err := exec.CommandContext(ctx, "sh", "-c", `printf '\n' > "$1"`, "fixture", fifo).
		CombinedOutput(); err != nil {
		t.Fatalf("release remote shell while the controller is offline: %s: %v", result, err)
	}
}

func readOperatorOutput(t *testing.T, socket *websocket.Conn, marker string) (string, uint64) {
	t.Helper()
	var output strings.Builder
	for {
		frame := readOperatorTerminalFrame(t, socket)
		switch frame.Op {
		case terminalwire.ServerOpOutput:
			output.Write(frame.Payload)
			if strings.Contains(output.String(), marker) {
				return output.String(), frame.Seq + uint64(len(frame.Payload))
			}
		case terminalwire.ServerOpGap, terminalwire.ServerOpExit, terminalwire.ServerOpError:
			t.Fatalf("terminal did not reach %q: opcode %d, %s", marker, frame.Op, frame.Payload)
		}
	}
}

func readOperatorTerminalFrame(t *testing.T, socket *websocket.Conn) terminalwire.Frame {
	t.Helper()
	if err := socket.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	kind, message, err := socket.ReadMessage()
	if err != nil || kind != websocket.BinaryMessage {
		t.Fatalf("terminal frame kind %d: %v", kind, err)
	}
	frame, err := terminalwire.DecodeServer(message)
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func narrowOperatorTerminalScope(
	t *testing.T,
	clock *clockwork.FakeClock,
	host, controller daemonAccountFixture,
	id remoteproto.ComputerID,
	device string,
	socket *websocket.Conn,
) {
	t.Helper()
	// IT-042. Safety Invariants 4, 5 and 10. The real stream closes before a narrowed lease is used.
	host.uds(t, http.MethodPatch, contract.RemoteDevicesPath+"/"+device, contract.RemoteDevicePatchRequest{
		Scopes: new([]remoteproto.Scope{remoteproto.ScopeSessions}),
	}).expect(t, http.StatusOK, "sessions:operate")
	expectTerminalClose(t, socket, 4503)
	path := bridgeAPI(id, "/api/terminals")
	controller.uds(t, http.MethodGet, path, nil).expect(t, http.StatusForbidden, "device_revoked")

	// The controller refreshes its tunnel credential on a health tick.
	clock.Advance(3 * time.Second)
	testutil.Eventually(t, "controller connects with only sessions:operate", 10*time.Second, func() bool {
		scopes := readControllerComputer(t, controller, id).Scopes
		return len(scopes) == 1 && scopes[0] == remoteproto.ScopeSessions
	})
	reply := controller.uds(t, http.MethodGet, path, nil)
	reply.expect(t, http.StatusForbidden, "")
	failure := authJSON[contract.ErrorPayload](t, reply)
	if failure.Code != "remote_scope_denied" || failure.Details["scope"] != string(remoteproto.ScopeTerminal) {
		t.Fatalf("narrowed terminal request returned code=%q details=%v", failure.Code, failure.Details)
	}
}

const operatorProviderName = "remote-auth-fixture"

func configureOperatorProvider(t *testing.T, home string, signedIn bool) {
	t.Helper()
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(home, "provider-status")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '{\"loggedIn\":%t}'\n", signedIn)
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	target, err := config.ResolveConfigWriteTarget(paths, "", config.WriteScopeUser, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = config.EditConfigOverlay(paths, "", target, func(editor *config.OverlayEditor) error {
		return editor.SetTable([]string{"providers", operatorProviderName}, map[string]any{
			"command": executable, "auth_status_command": executable, "auth_mode": "native_cli", "harness": "acp",
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertRemoteProviderState(t *testing.T, controller, host daemonAccountFixture, id remoteproto.ComputerID) {
	t.Helper()
	// IT-044. Safety Invariants 5 and 11. The same provider has opposite live verdicts on the two daemons.
	const path = "/api/providers"
	probe := path + "/" + operatorProviderName + "/auth/probe"
	host.uds(t, http.MethodPost, probe, nil).expect(t, http.StatusOK, contract.ProviderAuthStateAuthenticated)
	controller.uds(t, http.MethodPost, probe, nil).expect(t, http.StatusOK, contract.ProviderAuthStateNeedsLogin)
	remote := controller.request(t, http.MethodGet, bridgeAPI(id, path), nil)
	remote.expect(t, http.StatusOK, operatorProviderName)
	local := controller.request(t, http.MethodGet, path, nil)
	local.expect(t, http.StatusOK, operatorProviderName)
	if got := operatorProviderState(t, remote); got != contract.ProviderAuthStateAuthenticated {
		t.Fatalf("bridged provider sign-in state = %q", got)
	}
	if got := operatorProviderState(t, local); got != contract.ProviderAuthStateNeedsLogin {
		t.Fatalf("controller provider sign-in state = %q", got)
	}
}

func operatorProviderState(t *testing.T, reply authReply) string {
	t.Helper()
	for _, provider := range authJSON[contract.ProviderListResponse](t, reply).Providers {
		if provider.Name == operatorProviderName {
			return provider.AuthStatus.State
		}
	}
	t.Fatalf("provider %q was not in the inventory", operatorProviderName)
	return ""
}

func TestDaemonAccountSuspension(t *testing.T) {
	root := t.TempDir()
	t.Run("Should close streams and restore link sign-in (IT-106, IT-108)", func(t *testing.T) {
		// Safety Invariants 10, 22 and 34. Release installers and the remote host observe a single epoch boundary.
		admin := newAdminFixture(t, nil)
		f := admin.authFixture
		const address = "suspension@example.test"
		owner, ring := startDaemonAccountFixture(t, f, filepath.Join(root, "owner"))
		progress := owner.stream(t, contract.AccountSignInEventsPath, "")
		first := signInDaemonAccount(t, f, owner, progress, address)
		accountID, _ := assertEnrolledAccount(t, f, first, ring)
		defer progress.cancel()

		tokenReply := owner.uds(t, http.MethodPost, contract.AccountPath+"/tokens", map[string]string{"name": "ci"})
		tokenReply.expect(t, http.StatusCreated, "token")
		token := authJSON[struct {
			Token string `json:"token"`
		}](t, tokenReply).Token
		t.Setenv("COMPOZY_ACCOUNT_TOKEN", token)
		ci, _ := startDaemonAccountFixture(t, f, filepath.Join(root, "ci"))
		testutil.Eventually(t, "token installation signs in", 5*time.Second, func() bool {
			reply := ci.uds(t, http.MethodGet, contract.AccountPath, nil)
			status := authJSON[contract.AccountStatusResponse](t, reply)
			return status.State == "signed_in" && status.Source != nil && *status.Source == "token"
		})
		t.Setenv("COMPOZY_ACCOUNT_TOKEN", "")

		device := enrollRemoteDevice(t, f, address, "ins_suspend_controller_helper")
		link := remoteCloudRequest(t, f, device, http.MethodPost, "/v1/computers/link-tokens", nil)
		link.expect(t, http.StatusCreated, "token")
		linkToken := authJSON[struct {
			Token string `json:"token"`
		}](t, link).Token
		host, _ := startConfiguredDaemonAccount(t, f, filepath.Join(root, "host"), daemonAccountBoot{direct: true})
		reply := host.uds(t, http.MethodPost, contract.RemoteLinkPath, contract.RemoteLinkRequest{LinkToken: linkToken})
		reply.expect(t, http.StatusOK, "computer_id")
		id := authJSON[contract.RemoteLinkResponse](t, reply).ComputerID
		waitForHostLease(t, host)
		requireSuspensionAccountState(t, host, "signed_in", "link", "")
		workspace := registerOperatorWorkspace(t, host, t.TempDir())
		addAccountComputer(t, owner, id)
		waitComputerRoute(t, owner, id, remoteproto.RouteDirect)
		terminal := attachOperatorTerminal(t, owner, id, openOperatorTerminal(t, owner, id, workspace), 0)

		admin.requestAdmin(t, http.MethodGet, "/admin/confirm/suspend?account_id="+accountID, nil).
			expect(t, http.StatusOK, "Signs out every installation and stops every remote session of "+address+".")
		admin.action(t, url.Values{"account_id": {accountID}}, "suspend", "account_suspended")
		expectTerminalClose(t, terminal, 4503)
		f.clock.Advance(25 * time.Hour)
		for _, daemon := range []daemonAccountFixture{owner, host, ci} {
			requireSuspensionAccountState(t, daemon, "reauth_required", "", "account_suspended")
		}
		if status := readRemoteDaemonStatus(t, host); status.ComputerID != id {
			t.Fatalf("suspension removed the host identity: %s", status.ComputerID)
		}
		callback, _ := f.signIn(
			t,
			auth.Google,
			providerProfile{subject: address, email: address, verified: true},
			false,
		)
		callback.expect(t, http.StatusForbidden, "account_suspended")
		admin.action(t, url.Values{"account_id": {accountID}}, "unsuspend", "account_unsuspended")
		f.clock.Advance(25 * time.Hour)
		requireSuspensionAccountState(t, host, "signed_in", "link", "")
		requireSuspensionAccountState(t, ci, "reauth_required", "", "")
		if signedIn := signInAccountDevice(t, f, owner, progress, address); signedIn.Device.ID != first.Device.ID {
			t.Fatal("the installation did not reuse its existing device after unsuspension")
		}
	})
}

func requireSuspensionAccountState(t *testing.T, daemon daemonAccountFixture, state, source, reason string) {
	t.Helper()
	testutil.Eventually(t, "expected account state "+state, 5*time.Second, func() bool {
		reply := daemon.uds(t, http.MethodGet, contract.AccountPath, nil)
		status := authJSON[contract.AccountStatusResponse](t, reply)
		return reply.StatusCode == http.StatusOK && string(status.State) == state &&
			(source == "" || status.Source != nil && *status.Source == source) &&
			(reason == "" || status.SignedOutReason != nil && *status.SignedOutReason == reason)
	})
}

func TestDaemonSignOutKeepsClaimedWork(t *testing.T) {
	root := t.TempDir()
	t.Run(
		"Should close operator streams while an agent settles its task and the PTY stays alive (IT-102)",
		func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("this fixture uses a Unix shell")
			}
			// Safety Invariant 14: logout closes transports, not running work or existing leases.
			f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
			home := filepath.Join(root, "agent")
			started, releaseTurn := registerAccountTurnBarrier(t, home)
			defer releaseTurn()
			d, _ := startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{signInRequired: true})
			progress := d.stream(t, contract.AccountSignInEventsPath, "")
			const address = "continuation@example.test"
			signInDaemonAccount(t, f, d, progress, address)
			project := prepareOperatorProject(t, root)
			fifo := prepareOperatorTerminalReplay(t, project)
			workspace := registerOperatorWorkspace(t, d, project)
			local := operatorBridge{t: t, host: d, controller: d}
			run := local.enqueueTask(workspace, taskpkg.WorktreeModeNone)
			session := local.createSession(workspace)
			sessionPath := "/api/workspaces/" + workspace + "/sessions/" + session
			terminal := openSignOutTerminal(t, d, workspace)

			prompt := startSignOutTurn(t, d, sessionPath)
			testutil.Receive(t, "active turn before logout", 10*time.Second, started)
			caller := map[string]string{
				agentidentity.HeaderSessionID: session,
				agentidentity.HeaderAgent:     operatorTaskAgent,
			}
			callAgent := func(path string, body any, authenticated bool) authReply {
				t.Helper()
				headers := caller
				if !authenticated {
					headers = nil
				}
				return accountDaemonRequest(
					t,
					d.udsClient,
					http.MethodPost,
					"http://localhost/api/agent"+path,
					body,
					headers,
				)
			}
			callAgent(
				"/tasks/claim-next",
				contract.AgentTaskClaimNextRequest{WorkspaceID: workspace, LeaseSeconds: 120},
				true,
			).
				expect(t, http.StatusOK, run)
			lease := "/tasks/" + run
			callAgent(lease+"/start", struct{}{}, true).expect(t, http.StatusOK, "running")
			d.uds(t, http.MethodPost, contract.AccountLogoutPath, contract.AccountLogoutRequest{}).
				expect(t, http.StatusOK, "signed_out")
			expectTerminalClose(t, terminal.connection, 4401)
			expectAccountStreamEviction(t, prompt)
			callAgent(lease+"/heartbeat", struct{}{}, false).expect(t, http.StatusUnauthorized, "sign_in_required")
			callAgent(lease+"/heartbeat", contract.AgentTaskHeartbeatRequest{LeaseSeconds: 120}, true).
				expect(t, http.StatusOK, run)
			callAgent(lease+"/complete", contract.AgentTaskCompleteRequest{}, true).
				expect(t, http.StatusOK, "completed")
			callAgent("/tasks/claim-next", struct{}{}, true).expect(t, http.StatusUnauthorized, "sign_in_required")
			callAgent("/spawn", struct{}{}, true).expect(t, http.StatusUnauthorized, "sign_in_required")
			d.uds(t, http.MethodPost, controllerSessions, contract.CreateSessionRequest{
				Workspace: workspace, AgentName: operatorTaskAgent,
			}).expect(t, http.StatusUnauthorized, "sign_in_required")
			waitDaemonAccountState(t, d, account.SignedOut, "signed_out")
			releaseTurn()
			releaseOperatorTerminalReplay(t, fifo)

			signInDaemonAccount(t, f, d, progress, address)
			now := attachDaemonTerminal(t, d, terminal.path, terminal.cursor)
			readOperatorOutput(t, now, "sign-out-pty-still-running\r\n")
			testutil.Eventually(t, "turn finishes across sign-out", 8*time.Second, func() bool {
				reply := d.uds(t, http.MethodGet, sessionPath+"/history", nil)
				return reply.StatusCode == http.StatusOK &&
					bytes.Contains(reply.body, []byte(accountTransitionFinished))
			})
		},
	)
}

type signOutTerminalFixture struct {
	connection *websocket.Conn
	path       string
	cursor     uint64
}

func openSignOutTerminal(t *testing.T, d daemonAccountFixture, workspace string) signOutTerminalFixture {
	t.Helper()
	path := "/api/workspaces/" + workspace + "/terminals"
	reply := d.uds(t, http.MethodPost, path, contract.TerminalCreateRequest{
		Shell: "sh", Cols: 80, Rows: 24, Title: "sign-out PTY",
	})
	reply.expect(t, http.StatusCreated, "sign-out PTY")
	path += "/" + string(authJSON[contract.TerminalResponse](t, reply).Terminal.ID)
	socket := attachDaemonTerminal(t, d, path, 0)
	input := "printf 'sign-out-pty-%s\\n' ready; read line < resume; printf 'sign-out-pty-%s\\n' still-running\n"
	frame, err := terminalwire.EncodeClient(terminalwire.Frame{Op: terminalwire.ClientOpInput, Payload: []byte(input)})
	if err != nil {
		t.Fatal(err)
	}
	if err := socket.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		t.Fatal(err)
	}
	_, cursor := readOperatorOutput(t, socket, "\r\nsign-out-pty-ready\r\n")
	return signOutTerminalFixture{connection: socket, path: path, cursor: cursor}
}

func startSignOutTurn(t *testing.T, d daemonAccountFixture, path string) accountIntegrationStream {
	t.Helper()
	wire, err := json.Marshal(fixturePrompt(accountTransitionPrompt))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.httpBase+path+"/prompt", bytes.NewReader(wire))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := d.httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return observeAccountStream(t, ctx, cancel, response)
}
