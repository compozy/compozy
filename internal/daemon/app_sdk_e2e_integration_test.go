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
	"github.com/compozy/compozy/internal/transcript"
)

const appSDKAgent = "sdk-client"
const appSDKReply = "app sdk golden reply"
const appSDKQuickstartPrompt = "List the three largest files in this repo."
const appSDKQuickstartReply = "largest.txt (4096 bytes), middle.txt (3072 bytes), smallest.txt (2048 bytes)."

func TestDaemonE2EAppSDK(t *testing.T) {
	t.Parallel()
	acpmock.RequireDriver(t)
	fixture := appSDKFixture(t)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{
			{FixturePath: fixture, FixtureAgent: appSDKAgent, AgentName: appSDKAgent},
			{FixturePath: fixture, FixtureAgent: appSDKAgent, AgentName: "claude"},
		},
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
	t.Run("Should execute the published Node quickstart and persist its reply [US-017.AC-4]", func(t *testing.T) {
		t.Parallel()
		appSDKRunQuickstart(t, root, harness, environment)
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
	turns := make([]acpmock.TurnFixture, 0, 8)
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
	turns = append(turns, acpmock.TurnFixture{
		Name: "documented quickstart", Match: acpmock.TurnMatch{TurnSource: "user", UserText: appSDKQuickstartPrompt},
		Steps: []acpmock.Step{{Kind: acpmock.StepKindAssistant, Text: appSDKQuickstartReply}},
	})
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

func appSDKRunCommand(
	t *testing.T,
	root string,
	environment map[string]string,
	executable string,
	args ...string,
) string {
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
	return string(output)
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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, harness.HTTPURL(path), http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", origin)
	request.Header.Set("Authorization", "Bearer "+issued.Token)
	listRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, harness.HTTPURL("/api/sessions"), http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	listRequest.Header = request.Header.Clone()
	listed, err := harness.HTTPClient.Do(listRequest)
	if err != nil {
		t.Fatal(err)
	}
	listBody, readErr := io.ReadAll(listed.Body)
	if err := errors.Join(readErr, listed.Body.Close()); err != nil {
		t.Fatal(err)
	}
	if listed.StatusCode != http.StatusOK || listed.Header.Get("Access-Control-Allow-Origin") != origin {
		t.Fatalf("browser sessions = %d %s headers=%v", listed.StatusCode, listBody, listed.Header)
	}
	var sessions contract.SessionsResponse
	if err := json.Unmarshal(listBody, &sessions); err != nil {
		t.Fatal(err)
	}
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
	for _, revokedRequest := range []*http.Request{listRequest, request} {
		rejected, err := harness.HTTPClient.Do(revokedRequest.Clone(ctx))
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

func appSDKRunQuickstart(t *testing.T, root string, harness *e2etest.RuntimeHarness, environment map[string]string) {
	t.Helper()
	page, err := os.ReadFile(filepath.Join(root, "packages", "site", "content", "docs", "apps", "index.mdx"))
	if err != nil {
		t.Fatal(err)
	}
	_, remaining, found := strings.Cut(string(page), "```js\n")
	script, _, closed := strings.Cut(remaining, "\n```")
	if !found || !closed {
		t.Fatal("published quickstart has no complete JavaScript program")
	}
	if err := os.WriteFile(filepath.Join(harness.WorkspaceRoot, "run.mjs"), []byte(script+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, file := range []struct {
		name string
		size int
	}{
		{"largest.txt", 4096}, {"middle.txt", 3072}, {"smallest.txt", 2048},
	} {
		if err := os.WriteFile(
			filepath.Join(harness.WorkspaceRoot, file.name),
			[]byte(strings.Repeat("x", file.size)),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
	packageScope := filepath.Join(harness.WorkspaceRoot, "node_modules", "@compozy")
	if err := os.MkdirAll(packageScope, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "sdk", "client"), filepath.Join(packageScope, "client")); err != nil {
		t.Fatal(err)
	}
	t.Log("Quickstart uses the built current-checkout package; registry installation remains release verification.")
	output := appSDKRunCommand(t, harness.WorkspaceRoot, environment, "node", "run.mjs")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var identity contract.RuntimeIdentityPayload
	if err := harness.HTTPJSON(ctx, http.MethodGet, "/api/status/identity", nil, &identity); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Connected to CompozyOS "+identity.Daemon.Version+"\n") ||
		!strings.Contains(output, appSDKQuickstartReply) {
		t.Fatalf("published quickstart did not print the daemon version and workspace reply: %q", output)
	}
	var sessions contract.SessionsResponse
	if err := harness.HTTPJSON(
		ctx,
		http.MethodGet,
		"/api/sessions?workspace_id="+harness.WorkspaceID,
		nil,
		&sessions,
	); err != nil {
		t.Fatal(err)
	}
	var matches []contract.SessionPayload
	for _, session := range sessions.Sessions {
		if session.AgentName == "claude" && session.WorkspaceID == harness.WorkspaceID {
			matches = append(matches, session)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("quickstart persisted %d claude sessions in its workspace, want one", len(matches))
	}
	pageResponse, err := harness.SessionTranscript(ctx, matches[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	text := transcript.JoinUIMessageText(transcript.MessagesFromEntries(pageResponse.Entries))
	if !strings.Contains(text, appSDKQuickstartPrompt) || !strings.Contains(text, appSDKQuickstartReply) {
		t.Fatalf("quickstart transcript omitted its persisted prompt or reply: %q", text)
	}
}
