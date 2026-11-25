// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration

package api_test

import (
	"bytes"
	"encoding/json/v2"
	"net/http"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cli"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/sse"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	"github.com/compozy/compozy/internal/testutil/remotetest"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	computerFixtureAgent  = "remote-computer-fixture"
	computerFixtureText   = "Per job. The retry budget caps attempts across a job's requests."
	computerFixturePrompt = "What is the retry budget?"
)

func TestDaemonComputerJourneys(t *testing.T) {
	root := t.TempDir()
	t.Run("Should switch the CLI and resume a session stream when direct connectivity drops", func(t *testing.T) {
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		host, _ := startSessionComputer(t, f, filepath.Join(root, "route-host"), true)
		id := linkSessionComputer(t, host)
		forward := fixedFixtureForward(readRemoteDaemonStatus(t, host).Listener)
		home := filepath.Join(root, "route-controller")
		controller, registry := startComputerController(t, f, home, registryTestEmail, forward)
		addAccountComputer(t, controller, id)
		addFixtureForward(t, registry, id)
		controller.uds(t, http.MethodPatch, computerAPI(id), contract.PatchComputerRequest{Name: new(controllerTestName)}).
			expect(t, http.StatusOK, controllerTestName)
		waitComputerRoute(t, controller, id, remoteproto.RouteDirect)
		// E2E-005. This command uses the controller's actual Unix socket and route state.
		assertCLIComputerRoute(t, remoteproto.RouteDirect)
		session := makeFixtureSession(t, host, "")
		path := controllerSessions + "/" + session + "/stream?frames=raw&limit=100"
		original := controller.stream(t, bridgeAPI(id, path), "")
		last := awaitNumberedComputerEvent(t, original)

		// IT-034. Safety Invariants 1, 2, 15 and 18. The target and Last-Event-ID survive a route change.
		host.uds(t, http.MethodPatch, contract.RemotePath, contract.RemotePatchRequest{Direct: new(false)}).
			expect(t, http.StatusOK, `"enabled":false`)
		testutil.Eventually(t, "CLI reports SSH within ten seconds", 10*time.Second, func() bool {
			row := cliComputerStatus(t)
			return row.RouteInUse != nil && row.RouteInUse.Kind == remoteproto.RouteSSH &&
				hasCurrentComputerRoute(row.Routes, remoteproto.RouteSSH)
		})
		original.cancel()
		promptFixtureSession(t, host, session, "", computerFixturePrompt)
		resumed := controller.stream(t, bridgeAPI(id, path), last.ID)
		t.Cleanup(resumed.cancel)
		canonical := host.stream(t, path, last.ID)
		t.Cleanup(canonical.cancel)
		for {
			want := awaitNumberedComputerEvent(t, canonical)
			got := awaitNumberedComputerEvent(t, resumed)
			if want.ID != got.ID || want.Event != got.Event || !bytes.Equal(want.Data, got.Data) {
				t.Fatalf("replayed event %s/%s differs from target %s/%s", got.Event, got.ID, want.Event, want.ID)
			}
			if bytes.Contains(want.Data, []byte("Per job.")) {
				break
			}
		}
		assertTargetedSessionCLI(t, controller, id, session)
	})

	t.Run("Should finish a session turn while the controller is stopped", func(t *testing.T) {
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		host, _ := startSessionComputer(t, f, filepath.Join(root, "turn-host"), true)
		id := linkSessionComputer(t, host)
		home := filepath.Join(root, "turn-controller")
		controller, ring := startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{})
		progress := controller.stream(t, contract.AccountSignInEventsPath, "")
		signInAccountDevice(t, f, controller, progress, registryTestEmail)
		progress.cancel()
		addAccountComputer(t, controller, id)
		waitComputerRoute(t, controller, id, remoteproto.RouteDirect)
		base := "/c/" + string(id)
		// IT-037. Safety Invariants 1, 11 and 16. Controller shutdown ends transport while host work continues.
		session := makeFixtureSession(t, controller, base)
		startFixturePrompt(t, controller, session, base, computerFixturePrompt)
		controller.stop()
		transcript := controllerSessions + "/" + session + "/transcript"
		testutil.Eventually(t, "the host completes the admitted turn", 10*time.Second, func() bool {
			reply := host.uds(t, http.MethodGet, transcript, nil)
			return reply.StatusCode == http.StatusOK && bytes.Contains(reply.body, []byte(computerFixtureText))
		})
		restarted, _ := startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{keyring: ring})
		waitComputerRoute(t, restarted, id, remoteproto.RouteDirect)
		reply := restarted.uds(t, http.MethodGet, base+transcript, nil)
		reply.expect(t, http.StatusOK, computerFixtureText)
		if !bytes.Contains(reply.body, []byte(computerFixturePrompt)) {
			t.Fatalf("the host transcript lost the admitted prompt: %s", reply.body)
		}
	})

	// IT-090. Rule 14 blocks new sessions while preserving durable history.
	t.Run("Should keep local sessions readable when this computer becomes a controller", func(t *testing.T) {
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		host, _ := startSessionComputer(t, f, filepath.Join(root, "settings-host"), false)
		id := linkSessionComputer(t, host)
		controller, _ := startComputerController(
			t,
			f,
			filepath.Join(root, "settings-controller"),
			registryTestEmail,
			nil,
		)
		old := makeSessionIDs(t, controller, "Controller history")
		patch := contract.PatchLocalComputerRequest{RunAgents: new(false)}
		local := contract.ComputersPath + "/local"
		controller.uds(t, http.MethodPatch, local, patch).expect(t, http.StatusBadRequest, "config_invalid")
		addAccountComputer(t, controller, id)
		controller.uds(t, http.MethodPatch, local, patch).expect(t, http.StatusOK, `"run_agents":false`)
		controller.uds(t, http.MethodPost, controllerSessions, contract.CreateSessionRequest{
			Scope: "user", AgentName: config.DefaultAgentName, Name: "Rejected controller session",
		}).expect(t, http.StatusConflict, "local_execution_disabled")
		expectComputerSessions(
			t,
			controller.uds(t, http.MethodGet, controllerSessions+"?all_workspaces=true", nil),
			old,
		)
	})

	// IT-091. Rules 13 and 16, Safety Invariants 5, 11 and 16. Auto operates on
	// the winning computer in the workspace that owns the project there.
	t.Run("Should place a session using live resources and the matching repository", func(t *testing.T) {
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		host, _ := startSessionComputer(t, f, filepath.Join(root, "placement-host"), true)
		id := linkSessionComputer(t, host)
		home := filepath.Join(root, "placement-controller")
		configureComputerACP(t, home)
		forward := fixedFixtureForward(readRemoteDaemonStatus(t, host).Listener)
		controller, registry := startComputerController(t, f, home, registryTestEmail, forward)
		addAccountComputer(t, controller, id)
		waitComputerRoute(t, controller, id, remoteproto.RouteDirect)
		localWorkspace := registerPlacementProject(t, controller, "https://github.com/compozy/placement-fixture.git")
		remoteWorkspace := registerPlacementProject(t, host, "git@github.com:compozy/placement-fixture.git")
		controller.uds(t, http.MethodPatch, contract.ComputersPath+"/local", contract.PatchLocalComputerRequest{
			LoadBalancing: new(true), LoadBalancingWeight: new(25),
		}).expect(t, http.StatusOK, `"load_balancing_weight":25`)
		controller.uds(t, http.MethodPatch, computerAPI(id), contract.PatchComputerRequest{
			Weight: new(100),
		}).expect(t, http.StatusOK, `"load_balancing_weight":100`)
		localSample := readLiveResources(t, controller, contract.HostResourcesPath, f.clock.Now())
		remoteSample := readLiveResources(t, controller, bridgeAPI(id, contract.HostResourcesPath), f.clock.Now())
		wantComputer, wantWorkspace, target := id, remoteWorkspace, host
		if liveResourceScore(localSample, 25) >= liveResourceScore(remoteSample, 100) {
			wantComputer, wantWorkspace, target = computers.LocalID, localWorkspace, controller
		}
		if liveResourceScore(localSample, 25) == 0 && liveResourceScore(remoteSample, 100) == 0 {
			t.Fatalf(
				"the integration runner saturated both live samples: local=%+v remote=%+v",
				localSample,
				remoteSample,
			)
		}
		assertAutoComputerSession(t, controller, target, localWorkspace, wantWorkspace, wantComputer)

		// AST-002: Auto considers live connectivity independently of a service warning.
		if err := registry.AddSSHRoute(t.Context(), id, store.ComputerRoute{
			Kind: remoteproto.RouteSSH, Address: "fixture@testhost",
			SSH: &store.ComputerSSHTarget{User: "fixture", Port: 22, ServiceWarning: "linger_off"},
		}); err != nil {
			t.Fatal(err)
		}
		host.uds(t, http.MethodPatch, contract.RemotePath, contract.RemotePatchRequest{Direct: new(false)}).
			expect(t, http.StatusOK, `"enabled":false`)
		testutil.Eventually(t, "connected SSH computer shows a service warning", 9*time.Second, func() bool {
			view := readControllerComputer(t, controller, id)
			return view.Status == string(computers.NeedsAttention) &&
				view.RouteInUse != nil && view.RouteInUse.Kind == remoteproto.RouteSSH
		})
		controller.uds(t, http.MethodPatch, contract.ComputersPath+"/local", contract.PatchLocalComputerRequest{
			RunAgents: new(false),
		}).expect(t, http.StatusOK, `"run_agents":false`)
		reply := controller.uds(t, http.MethodPost, contract.ComputersPath+"/placement",
			contract.ComputerPlacementRequest{Workspace: localWorkspace})
		if reply.StatusCode == http.StatusConflict {
			reply.expect(t, http.StatusConflict, "no_eligible_computer")
			failure := authJSON[contract.ErrorPayload](t, reply)
			if reason := failure.Details[string(id)]; reason != "cpu_busy" && reason != "memory_busy" {
				t.Fatalf("live SSH computer was excluded before resource checks: %s", reply.body)
			}
		} else {
			reply.expect(t, http.StatusOK, string(id))
			if placement := authJSON[contract.ComputerPlacementResponse](
				t,
				reply,
			); placement.WorkspaceID != remoteWorkspace {
				t.Fatalf("Auto selected the wrong SSH workspace: %+v", placement)
			}
		}
	})
}

func assertTargetedSessionCLI(
	t *testing.T,
	controller daemonAccountFixture,
	id remoteproto.ComputerID,
	session string,
) {
	t.Helper()
	// IT-092. Safety Invariant 11. The CLI selects through the same account-scoped UDS bridge.
	var list, stderr bytes.Buffer
	code := cli.ExecuteContext(t.Context(), []string{
		"session", "list", "--computer", controllerTestName, "--json",
	}, &list, &stderr)
	var sessions []struct {
		contract.SessionPayload
		ComputerID remoteproto.ComputerID `json:"computer_id"`
	}
	path := controllerSessions + "?all_workspaces=true&state=active"
	reply := controller.uds(t, http.MethodGet, bridgeAPI(id, path), nil)
	reply.expect(t, http.StatusOK, session)
	want := authJSON[contract.SessionCatalogResponse](t, reply).Sessions
	if code != 0 || json.Unmarshal(list.Bytes(), &sessions) != nil || len(sessions) != 1 || len(want) != 1 ||
		sessions[0].ComputerID != id || !reflect.DeepEqual(sessions[0].SessionPayload, want[0]) {
		t.Fatalf(
			"targeted CLI list exit %d, stdout %s, stderr %s, bridge %s",
			code,
			list.Bytes(),
			stderr.Bytes(),
			reply.body,
		)
	}
	list.Reset()
	stderr.Reset()
	code = cli.ExecuteContext(t.Context(), []string{
		"daemon", "start", "--computer", controllerTestName, "--json",
	}, &list, &stderr)
	if code != 2 || !bytes.Contains(append(list.Bytes(), stderr.Bytes()...), []byte(`"flag_not_applicable"`)) {
		t.Fatalf("daemon start exit %d, stdout %s, stderr %s", code, list.Bytes(), stderr.Bytes())
	}
}

func registerPlacementProject(t *testing.T, computer daemonAccountFixture, origin string) string {
	t.Helper()
	project := prepareOperatorProject(t, t.TempDir())
	command := exec.CommandContext(t.Context(), "git", "remote", "add", "origin", origin)
	command.Dir = project
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("configure the project origin: %v: %s", err, out)
	}
	return registerOperatorWorkspace(t, computer, project)
}

func readLiveResources(
	t *testing.T,
	controller daemonAccountFixture,
	path string,
	now time.Time,
) contract.HostResourcesResponse {
	t.Helper()
	reply := controller.uds(t, http.MethodGet, path, nil)
	reply.expect(t, http.StatusOK, "sampled_at")
	sample := authJSON[contract.HostResourcesResponse](t, reply)
	if sample.CPUCount < 1 || sample.CPUUtil == nil || sample.MemTotal == nil || sample.MemAvailable == nil ||
		*sample.MemTotal == 0 || sample.SampledAt.IsZero() || now.Sub(sample.SampledAt) > 15*time.Second {
		t.Fatalf("%s did not return live resources: %s", path, reply.body)
	}
	return sample
}

func liveResourceScore(sample contract.HostResourcesResponse, weight int) float64 {
	free := float64(*sample.MemAvailable) / float64(*sample.MemTotal)
	if *sample.CPUUtil >= 0.95 || free <= 0.05 {
		return 0
	}
	return float64(weight*sample.CPUCount) * (1 - *sample.CPUUtil) * free
}

func assertAutoComputerSession(
	t *testing.T,
	controller, target daemonAccountFixture,
	source, workspace string,
	id remoteproto.ComputerID,
) {
	t.Helper()
	preview := controller.uds(
		t,
		http.MethodPost,
		contract.ComputersPath+"/placement",
		contract.ComputerPlacementRequest{
			Workspace: source,
		},
	)
	preview.expect(t, http.StatusOK, string(id))
	placement := authJSON[contract.ComputerPlacementResponse](t, preview)
	if placement.ComputerID != id || placement.WorkspaceID != workspace {
		t.Fatalf("Auto preview = %+v, want %s in %s", placement, id, workspace)
	}
	created := controller.uds(t, http.MethodPost, controllerSessions, contract.CreateSessionRequest{
		Scope:     "workspace",
		Workspace: source,
		AgentName: computerFixtureAgent,
		Computer:  "auto",
		Name:      "Auto placement",
	})
	created.expect(t, http.StatusCreated, "Auto placement")
	result := authJSON[contract.SessionResponse](t, created)
	if result.ComputerID != id || result.Session.WorkspaceID != workspace {
		t.Fatalf("Auto session = %+v, want %s in %s", result, id, workspace)
	}
	target.uds(t, http.MethodGet, controllerSessions+"/"+result.Session.ID, nil).
		expect(t, http.StatusOK, "Auto placement")
}

func configureComputerACP(t *testing.T, home string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the remote computer integration fixture")
	}
	fixture := filepath.Join(
		filepath.Dir(file),
		"../../testutil/acpmock/testdata/browser_session_messages_fixture.json",
	)
	registerComputerACP(t, home, acpmock.RegisterOptions{
		FixturePath: fixture, FixtureAgent: "session-message-receiver", AgentName: computerFixtureAgent,
		ProviderName: acpmock.ProviderName,
	})
}

func registerComputerACP(t *testing.T, home string, options acpmock.RegisterOptions) {
	t.Helper()
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	registration, err := acpmock.Register(paths, options)
	if err != nil {
		t.Fatal(err)
	}
	target, err := config.ResolveConfigWriteTarget(paths, "", config.WriteScopeUser, "")
	if err != nil {
		t.Fatal(err)
	}
	provider := acpmock.ProviderConfig(registration.DriverPath)
	if _, err := config.EditConfigOverlay(paths, "", target, func(editor *config.OverlayEditor) error {
		return editor.SetTable([]string{"providers", acpmock.ProviderName}, map[string]any{
			"command":       provider.Command,
			"display_name":  provider.DisplayName,
			"harness":       string(provider.Harness),
			"auth_mode":     string(provider.AuthMode),
			"none_security": string(provider.NoneSecurity),
		})
	}); err != nil {
		t.Fatalf("configure the session fixture provider: %v", err)
	}
}

func makeFixtureSession(t *testing.T, controller daemonAccountFixture, prefix string) string {
	t.Helper()
	reply := controller.uds(t, http.MethodPost, prefix+controllerSessions, contract.CreateSessionRequest{
		Scope: "user", AgentName: computerFixtureAgent, Name: "Remote session",
	})
	reply.expect(t, http.StatusCreated, "session")
	id := authJSON[contract.SessionResponse](t, reply).Session.ID
	testutil.Eventually(t, "the host session is available", 6*time.Second, func() bool {
		status := controller.uds(t, http.MethodGet, prefix+controllerSessions+"/"+id+"/status", nil)
		return status.StatusCode == http.StatusOK &&
			!bytes.Contains(status.body, []byte(`"state":"pending"`))
	})
	return id
}

func promptFixtureSession(t *testing.T, controller daemonAccountFixture, id, prefix, message string) {
	t.Helper()
	reply := controller.uds(t, http.MethodPost, fixturePromptPath(prefix, id), fixturePrompt(message))
	reply.expect(t, http.StatusOK, "finish")
}

func startFixturePrompt(t *testing.T, controller daemonAccountFixture, id, prefix, message string) {
	t.Helper()
	body, err := json.Marshal(fixturePrompt(message))
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		controller.httpBase+fixturePromptPath(prefix, id), bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := controller.httpClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("remote prompt response = HTTP %d %s", response.StatusCode, response.Header.Get("Content-Type"))
	}
}

func fixturePromptPath(prefix, id string) string {
	return prefix + "/api/workspaces/" + workspaceaccess.UserSelector + "/sessions/" + id + "/prompt"
}

func fixturePrompt(message string) contract.SendPromptRequest {
	return contract.SendPromptRequest{
		Message:        message,
		MessageID:      "m_remote_fixture",
		IdempotencyKey: "k_remote_fixture",
		Runtime:        &contract.PromptRuntimeSelectionPayload{Provider: acpmock.ProviderName},
	}
}

func awaitNumberedComputerEvent(t *testing.T, stream accountIntegrationStream) sse.Event {
	t.Helper()
	return stream.until(t, func(event sse.Event) bool {
		_, err := strconv.ParseUint(event.ID, 10, 64)
		return err == nil
	})
}

func assertCLIComputerRoute(t *testing.T, expected remoteproto.RouteKind) {
	t.Helper()
	status := cliComputerStatus(t)
	if status.RouteInUse == nil || status.RouteInUse.Kind != expected ||
		!hasCurrentComputerRoute(status.Routes, expected) {
		t.Fatalf("CLI computer status = %+v, want in_use on %s", status, expected)
	}
}

func hasCurrentComputerRoute(routes []contract.ComputerRoute, expected remoteproto.RouteKind) bool {
	for _, route := range routes {
		if route.Kind == expected && route.InUse {
			return true
		}
	}
	return false
}

func cliComputerStatus(t *testing.T) contract.Computer {
	t.Helper()
	var output, stderr bytes.Buffer
	if code := cli.ExecuteContext(t.Context(), []string{"computer", "status", controllerTestName, "--json"},
		&output, &stderr); code != 0 {
		t.Fatalf("computer status exit %d: %s", code, stderr.Bytes())
	}
	var status contract.Computer
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatalf("computer status returned invalid JSON: %v", err)
	}
	return status
}
