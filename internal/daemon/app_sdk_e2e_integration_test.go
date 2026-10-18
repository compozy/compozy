//go:build integration && !windows

package daemon

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

const appSDKAgent = "sdk-client"
const appSDKReply = "app sdk golden reply"

func TestDaemonE2EAppSDK(t *testing.T) {
	t.Parallel()
	acpmock.RequireDriver(t)
	fixture := appSDKFixture(t)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{FixturePath: fixture, FixtureAgent: appSDKAgent, AgentName: appSDKAgent}},
	})
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	environment := map[string]string{
		"COMPOZY_HOME":           harness.HomePaths.HomeDir,
		"COMPOZY_URL":            harness.HTTPBaseURL,
		"COMPOZY_TEST_WORKSPACE": harness.WorkspaceRoot,
		"COMPOZY_TEST_AGENT":     appSDKAgent,
		"COMPOZY_TEST_REPLY":     appSDKReply,
		"COMPOZY_TEST_SOCKET":    harness.Config.Daemon.Socket,
		"COMPOZY_BIN":            harness.BinaryPath,
	}
	t.Run(
		"Should drive prompt queue replay proxy and cancellation through the public TypeScript package",
		func(t *testing.T) {
			t.Parallel()
			appSDKRunCommand(t, root, environment, "bun", "sdk/client/e2e/golden-path.ts")
		},
	)
	t.Run("Should drive the public Go client over the daemon unix socket", func(t *testing.T) {
		t.Parallel()
		appSDKRunCommand(
			t,
			root,
			environment,
			"go",
			"-C",
			"sdk/go",
			"test",
			"-tags",
			"integration",
			"./client",
			"-run",
			"^TestDaemonGoldenPath$",
			"-count=1",
		)
	})
	t.Run("Should grant browser access through CLI and revoke its live session stream", func(t *testing.T) {
		t.Parallel()
		appSDKOperatorJourney(t, harness)
	})
	t.Run("Should start reuse and stop only a daemon owned by the Node SDK", func(t *testing.T) {
		t.Parallel()
		home := e2etest.NewHomePaths(t)
		e2etest.SeedConfig(t, home, e2etest.ConfigSeedOptions{})
		operatorHome := t.TempDir()
		env := map[string]string{"COMPOZY_HOME": home.HomeDir, "COMPOZY_BIN": harness.BinaryPath, "HOME": operatorHome}
		t.Cleanup(func() {
			if _, err := os.Stat(home.DaemonInfo); errors.Is(err, os.ErrNotExist) {
				return
			}
			ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, harness.BinaryPath, "daemon", "stop")
			command.Env = appSDKEnvironment(env)
			if output, err := command.CombinedOutput(); err != nil {
				t.Errorf("stop SDK-owned daemon: %v: %s", err, output)
			}
		})
		appSDKRunCommand(t, root, env, "bun", "sdk/client/e2e/node-start.ts")
		if _, err := os.Stat(filepath.Join(operatorHome, ".compozy")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Node lifecycle touched the default operator home: %v", err)
		}
	})
}

func appSDKFixture(t *testing.T) string {
	t.Helper()
	turns := make([]acpmock.TurnFixture, 0, 7)
	for _, message := range []string{"noop", "sdk golden", "sdk queued", "sdk replay", "sdk busy", "sdk proxy", "sdk cancel"} {
		steps := []acpmock.Step{{Kind: acpmock.StepKindAssistant, Text: appSDKReply}}
		switch message {
		case "sdk busy", "sdk proxy":
			steps = []acpmock.Step{
				{Kind: acpmock.StepKindAssistant, Text: "started "},
				{
					Kind:          acpmock.StepKindDriverControl,
					DriverControl: &acpmock.DriverControlStep{Action: acpmock.DriverControlDelay, DelayMS: 1500},
				},
				{Kind: acpmock.StepKindAssistant, Text: appSDKReply},
			}
		case "sdk cancel":
			steps = []acpmock.Step{
				{Kind: acpmock.StepKindAssistant, Text: "started "},
				{
					Kind:          acpmock.StepKindDriverControl,
					DriverControl: &acpmock.DriverControlStep{Action: acpmock.DriverControlBlockUntilCancel},
				},
			}
		}
		turns = append(
			turns,
			acpmock.TurnFixture{
				Name:  message,
				Match: acpmock.TurnMatch{TurnSource: "user", UserText: message},
				Steps: steps,
			},
		)
	}
	fixture := acpmock.Fixture{Version: acpmock.FixtureVersion, Agents: []acpmock.AgentFixture{
		{
			Name:        appSDKAgent,
			Provider:    "claude",
			Permissions: "approve-all",
			Prompt:      "Exercise the public app clients.",
			Turns:       turns,
		},
	}}
	if err := fixture.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "app-sdk.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func appSDKRunCommand(t *testing.T, root string, environment map[string]string, executable string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = root
	command.Env = appSDKEnvironment(environment)
	command.WaitDelay = 5 * time.Second
	procutil.ConfigureCommandProcessGroup(command)
	command.Cancel = func() error { return procutil.SignalCommandProcessGroup(command, syscall.SIGKILL) }
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", executable, strings.Join(args, " "), err, output)
	}
	t.Logf("%s: %s", executable, output)
}

func appSDKEnvironment(overrides map[string]string) []string {
	environment := testutil.HermeticProcessEnv(os.Environ())
	for key, value := range overrides {
		prefix := key + "="
		found := false
		for index, entry := range environment {
			if strings.HasPrefix(entry, prefix) {
				environment[index] = prefix + value
				found = true
				break
			}
		}
		if !found {
			environment = append(environment, prefix+value)
		}
	}
	return environment
}

func appSDKOperatorJourney(t *testing.T, harness *e2etest.RuntimeHarness) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	const origin = "http://localhost:3000"
	var issued contract.AppTokenIssuedResponse
	if err := harness.CLI.RunJSON(
		ctx,
		&issued,
		"app-token",
		"create",
		"SDK browser",
		"--origin",
		origin,
		"-o",
		"json",
	); err != nil {
		t.Fatal(err)
	}
	if issued.AppToken.ID == "" || issued.Token == "" {
		t.Fatal("CLI omitted token identity or one-time credential")
	}
	var second contract.AppTokenIssuedResponse
	if err := harness.CLI.RunJSON(ctx, &second, "app-token", "create", "SDK backend", "-o", "json"); err != nil {
		t.Fatal(err)
	}
	var pairing contract.GatewayPairingArtifactPayload
	if err := harness.HTTPJSON(ctx, http.MethodPost, "/api/gateway/pairings", nil, &pairing); err != nil {
		t.Fatal(err)
	}
	var device contract.GatewayIssuedCredentialPayload
	if err := harness.HTTPJSON(
		ctx,
		http.MethodPost,
		"/api/gateway/pairings/redeem",
		contract.GatewayPairingRedeemRequest{
			Artifact: pairing.Artifact, Name: "SDK paired device", ActorKind: "operator_device",
		},
		&device,
	); err != nil {
		t.Fatal(err)
	}
	var devices contract.GatewayDevicesResponse
	if err := harness.CLI.RunJSON(ctx, &devices, "device", "list", "-o", "json"); err != nil {
		t.Fatal(err)
	}
	if len(devices.Devices) != 1 || devices.Devices[0].ID != device.Device.ID {
		t.Fatalf("CLI device inventory with two app tokens = %#v", devices)
	}
	if err := harness.HTTPJSON(ctx, http.MethodGet, "/api/gateway/devices", nil, &devices); err != nil {
		t.Fatal(err)
	}
	if len(devices.Devices) != 1 || devices.Devices[0].ID != device.Device.ID {
		t.Fatalf("HTTP device inventory with two app tokens = %#v", devices)
	}
	var audit contract.GatewayAuditPayload
	if err := harness.HTTPJSON(ctx, http.MethodGet, "/api/gateway/audit", nil, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.Auth.ActiveDeviceCount != 1 || audit.DeviceHighlights.Active != 1 || len(audit.Status.Devices) != 1 {
		t.Fatalf("device audit counts app credentials: %#v", audit)
	}
	session := createBoundFixtureBackedSession(t, ctx, harness, appSDKAgent, "Browser stream")
	path := "/api/workspaces/" + session.WorkspaceID + "/sessions/" + session.ID + "/stream"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, harness.HTTPURL(path), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", origin)
	request.Header.Set("Authorization", "Bearer "+issued.Token)
	response, err := harness.HTTPClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close app stream: %v", err)
		}
	})
	if response.StatusCode != http.StatusOK || response.Header.Get("Access-Control-Allow-Origin") != origin {
		t.Fatalf("browser stream = %d %v", response.StatusCode, response.Header)
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	appSDKAwaitEvent(t, scanner, "transcript_snapshot")
	if _, err := harness.PromptSessionHTTP(ctx, session.ID, "sdk golden"); err != nil {
		t.Fatal(err)
	}
	appSDKAwaitEvent(t, scanner, "transcript_delta")
	revokeCtx, revokeCancel := context.WithTimeout(ctx, 5*time.Second)
	defer revokeCancel()
	var revoked contract.AppTokenRevokeResponse
	if err := harness.CLI.RunJSON(
		revokeCtx,
		&revoked,
		"app-token",
		"revoke",
		issued.AppToken.ID,
		"-o",
		"json",
	); err != nil {
		t.Fatal(err)
	}
	if !revoked.Changed || revoked.Canceled != 1 {
		t.Fatalf("revocation = %#v", revoked)
	}
	closed := make(chan error, 1)
	go func() {
		for scanner.Scan() {
		}
		closed <- scanner.Err()
	}()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("revocation stream close: %v", err)
		}
	case <-revokeCtx.Done():
		t.Fatal("revocation did not close browser stream within five seconds")
	}
	rejected, err := harness.HTTPClient.Do(request.Clone(ctx))
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(rejected.Body)
	closeErr := rejected.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatal(err)
	}
	var payload contract.ErrorPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if rejected.StatusCode != http.StatusUnauthorized || payload.Code != "app_token_revoked" {
		t.Fatalf("revoked credential = %d %s", rejected.StatusCode, body)
	}
}

func appSDKAwaitEvent(t *testing.T, scanner *bufio.Scanner, event string) {
	t.Helper()
	for scanner.Scan() {
		if strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "event:")) == event {
			return
		}
	}
	t.Fatalf("stream ended before %s: %v", event, scanner.Err())
}
