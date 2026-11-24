//go:build integration

package api_test

import (
	"bytes"
	"encoding/json/v2"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cli"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/sse"
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
			return row.RouteInUse != nil && row.RouteInUse.Kind == remoteproto.RouteSSH
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
		var list, stderr bytes.Buffer
		code := cli.ExecuteContext(t.Context(), []string{
			"session", "list", "--all", "--computer", controllerTestName, "--json",
		}, &list, &stderr)
		var sessions []struct {
			ID         string                 `json:"id"`
			ComputerID remoteproto.ComputerID `json:"computer_id"`
		}
		if code != 0 || json.Unmarshal(list.Bytes(), &sessions) != nil || len(sessions) != 1 ||
			sessions[0].ID != session || sessions[0].ComputerID != id {
			t.Fatalf("targeted CLI session list exit %d, stdout %s, stderr %s", code, list.Bytes(), stderr.Bytes())
		}
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
}

func configureComputerACP(t *testing.T, home string, boot daemonAccountBoot) daemonAccountBoot {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the remote computer integration fixture")
	}
	fixture := filepath.Join(
		filepath.Dir(file),
		"../../testutil/acpmock/testdata/browser_session_messages_fixture.json",
	)
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	registration, err := acpmock.Register(paths, acpmock.RegisterOptions{
		FixturePath: fixture, FixtureAgent: "session-message-receiver", AgentName: computerFixtureAgent,
		ProviderName: acpmock.ProviderName,
	})
	if err != nil {
		t.Fatal(err)
	}
	boot.configure = func(cfg *config.Config) {
		cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(registration.DriverPath)
	}
	return boot
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
