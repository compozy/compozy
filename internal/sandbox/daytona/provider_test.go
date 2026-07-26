package daytona

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/toolruntime"
)

func TestDaytonaProviderPrepareCreatesSandboxWithSnapshotLabelsAndRuntime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	sandbox := newFakeSandbox("sandbox-create")
	client := &fakeSandboxClient{created: sandbox, findErr: errSandboxNotFound}
	tokenSource := &fakeTokenSource{access: []sshAccess{{
		Token:     "ssh-token",
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}}}
	provider := newTestProvider(t, client, &fakeTransport{}, tokenSource, now)
	registry := toolruntime.NewRegistry(nil)
	provider.processRegistry = registry
	req := newDaytonaPrepareRequest(t)
	req.AgentEnv = []string{
		"AGH_SESSION_ID=sess-daytona",
		"DAYTONA_API_KEY=secret",
		"IGNORED=value",
	}
	req.Sandbox.Env = map[string]string{
		"NODE_ENV":         "test",
		"DAYTONA_API_KEY":  "blocked",
		"AGH_SESSION_ROLE": "profile",
	}

	prepared, err := provider.Prepare(context.Background(), req)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	if got, want := len(client.createRequests), 1; got != want {
		t.Fatalf("Create calls = %d, want %d", got, want)
	}
	create := client.createRequests[0]
	if got, want := create.Snapshot, "snap-base"; got != want {
		t.Fatalf("Create snapshot = %q, want %q", got, want)
	}
	if create.Image != "" {
		t.Fatalf("Create image = %q, want empty when snapshot wins", create.Image)
	}
	if got, want := create.Labels["agh_session_id"], req.SessionID; got != want {
		t.Fatalf("label agh_session_id = %q, want %q", got, want)
	}
	if got, want := create.Labels["agh_sandbox_id"], req.SandboxID; got != want {
		t.Fatalf("label agh_sandbox_id = %q, want %q", got, want)
	}
	if _, leaked := create.EnvVars["DAYTONA_API_KEY"]; leaked {
		t.Fatal("Create env propagated DAYTONA_API_KEY")
	}
	if got, want := create.EnvVars["AGH_SESSION_ID"], "sess-daytona"; got != want {
		t.Fatalf("Create env AGH_SESSION_ID = %q, want %q", got, want)
	}
	if got, want := create.EnvVars["NODE_ENV"], "test"; got != want {
		t.Fatalf("Create env NODE_ENV = %q, want %q", got, want)
	}
	if got, want := prepared.RuntimeRootDir, "/workspace/runtime"; got != want {
		t.Fatalf("RuntimeRootDir = %q, want %q", got, want)
	}
	if got, want := prepared.State.InstanceID, sandbox.id; got != want {
		t.Fatalf("State.InstanceID = %q, want %q", got, want)
	}
	if prepared.State.SSHAccessExpiresAt == nil || !prepared.State.SSHAccessExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("State.SSHAccessExpiresAt = %v, want %v", prepared.State.SSHAccessExpiresAt, now.Add(time.Hour))
	}
	if prepared.Launcher == nil {
		t.Fatal("Prepared.Launcher = nil")
	}
	if prepared.ToolHost == nil {
		t.Fatal("Prepared.ToolHost = nil")
	}
	daytonaHost, ok := prepared.ToolHost.(*daytonaToolHost)
	if !ok {
		t.Fatalf("Prepared.ToolHost type = %T, want *daytonaToolHost", prepared.ToolHost)
	}
	if daytonaHost.ProcessRegistry() != registry {
		t.Fatalf("Prepared.ToolHost.ProcessRegistry() = %p, want %p", daytonaHost.ProcessRegistry(), registry)
	}
	if got := prepared.Launch.Env; !containsString(got, "AGH_SESSION_ID=sess-daytona") ||
		!containsString(got, "NODE_ENV=test") ||
		containsKey(got, "DAYTONA_API_KEY") ||
		containsKey(got, "IGNORED") {
		t.Fatalf("Launch.Env = %#v, want allowlisted remote env only", got)
	}
}

func TestDaytonaProviderPrepareUsesImageWhenSnapshotEmpty(t *testing.T) {
	t.Parallel()

	provider, client := newProviderWithFakeClient(t)
	req := newDaytonaPrepareRequest(t)
	req.Sandbox.Daytona.Snapshot = ""
	req.Sandbox.Daytona.Image = "ubuntu:24.04"
	req.Sandbox.Daytona.StartupSource = sandbox.DaytonaStartupSourceImage
	req.Sandbox.Daytona.StartupRef = "ubuntu:24.04"

	if _, err := provider.Prepare(context.Background(), req); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if got, want := client.createRequests[0].Image, "ubuntu:24.04"; got != want {
		t.Fatalf("Create image = %q, want %q", got, want)
	}
	if client.createRequests[0].Snapshot != "" {
		t.Fatalf("Create snapshot = %q, want empty", client.createRequests[0].Snapshot)
	}
}

func TestDaytonaProviderPrepareReattachesExistingSandbox(t *testing.T) {
	t.Parallel()

	provider, client := newProviderWithFakeClient(t)
	req := newDaytonaPrepareRequest(t)
	req.InstanceID = "sandbox-existing"
	client.sandboxes["sandbox-existing"] = newFakeSandbox("sandbox-existing")

	prepared, err := provider.Prepare(context.Background(), req)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if got, want := client.getIDs, []string{"sandbox-existing"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Get IDs = %#v, want %#v", got, want)
	}
	if len(client.createRequests) != 0 {
		t.Fatalf("Create calls = %d, want 0", len(client.createRequests))
	}
	if got, want := prepared.State.InstanceID, "sandbox-existing"; got != want {
		t.Fatalf("InstanceID = %q, want %q", got, want)
	}
}

func TestDaytonaProviderFindSandboxUsesDaemonSandboxLabel(t *testing.T) {
	t.Parallel()

	provider, client := newProviderWithFakeClient(t)
	client.findErr = nil
	req := newDaytonaPrepareRequest(t)

	state, err := provider.FindSandbox(context.Background(), sandbox.FindSandboxRequest{
		SessionID:           req.SessionID,
		WorkspaceID:         req.WorkspaceID,
		SandboxID:           req.SandboxID,
		LocalRootDir:        req.LocalRootDir,
		LocalAdditionalDirs: cloneStrings(req.LocalAdditionalDirs),
		Sandbox:             req.Sandbox,
	})
	if err != nil {
		t.Fatalf("FindSandbox() error = %v", err)
	}

	if got, want := len(client.findLabels), 1; got != want {
		t.Fatalf("FindOne calls = %d, want %d", got, want)
	}
	if got, want := client.findLabels[0], map[string]string{
		"agh_sandbox_id": req.SandboxID,
	}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("FindOne labels = %#v, want %#v", got, want)
	}
	if got, want := state.InstanceID, client.created.id; got != want {
		t.Fatalf("State.InstanceID = %q, want %q", got, want)
	}
	providerState, err := decodeProviderState(state.ProviderState)
	if err != nil {
		t.Fatalf("decodeProviderState() error = %v", err)
	}
	if got, want := providerState.SandboxID, client.created.id; got != want {
		t.Fatalf("providerState.SandboxID = %q, want %q", got, want)
	}
}

func TestDaytonaProviderFindSandboxUsesExplicitLabelsAndMapsNotFound(t *testing.T) {
	t.Parallel()

	provider, client := newProviderWithFakeClient(t)
	req := newDaytonaPrepareRequest(t)
	_, err := provider.FindSandbox(context.Background(), sandbox.FindSandboxRequest{
		SessionID:   req.SessionID,
		WorkspaceID: req.WorkspaceID,
		SandboxID:   req.SandboxID,
		Sandbox:     req.Sandbox,
		Labels:      map[string]string{"agh_sandbox_id": req.SandboxID, "custom": "true"},
	})
	if !errors.Is(err, sandbox.ErrSandboxNotFound) {
		t.Fatalf("FindSandbox() error = %v, want ErrSandboxNotFound", err)
	}
	if got, want := client.findLabels[0], map[string]string{
		"agh_sandbox_id": req.SandboxID,
		"custom":         "true",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("FindOne labels = %#v, want %#v", got, want)
	}
}

func TestDaytonaProviderFindSandboxValidatesInputs(t *testing.T) {
	t.Parallel()

	provider, _ := newProviderWithFakeClient(t)
	req := newDaytonaPrepareRequest(t)
	var nilCtx context.Context
	if _, err := provider.FindSandbox(nilCtx, sandbox.FindSandboxRequest{}); err == nil {
		t.Fatal("FindSandbox(nil context) error = nil")
	}
	_, err := provider.FindSandbox(context.Background(), sandbox.FindSandboxRequest{
		SandboxID: req.SandboxID,
		Sandbox: sandbox.Resolved{
			Backend: sandbox.BackendLocal,
		},
	})
	if err == nil {
		t.Fatal("FindSandbox(local backend) error = nil")
	}
	_, err = provider.FindSandbox(context.Background(), sandbox.FindSandboxRequest{
		Sandbox: req.Sandbox,
	})
	if err == nil {
		t.Fatal("FindSandbox(empty sandbox id) error = nil")
	}
}

func TestDaytonaProviderSyncToRuntimeStreamsSeparateTarArchives(t *testing.T) {
	t.Parallel()

	localRoot := t.TempDir()
	additional := t.TempDir()
	writeTestFile(t, filepath.Join(localRoot, "root.txt"), "root")
	writeTestFile(t, filepath.Join(additional, "extra.txt"), "extra")
	transport := &fakeTransport{}
	provider := newTestProviderWithTransport(transport)
	state := newProviderSessionState(t, localRoot, []string{additional})

	result, err := provider.SyncToRuntime(context.Background(), state, sandbox.SyncOptions{
		Reason: sandbox.SyncReasonStart,
	})
	if err != nil {
		t.Fatalf("SyncToRuntime() error = %v", err)
	}
	if got, want := result.FilesSynced, 2; got != want {
		t.Fatalf("SyncToRuntime() FilesSynced = %d, want %d", got, want)
	}
	if got, want := len(transport.dials), 2; got != want {
		t.Fatalf("transport dials = %d, want %d", got, want)
	}
	assertCommandContains(t, transport.dials[0].command, "tar -xpf -")
	assertTarContains(t, transport.dials[0].session.written.Bytes(), "root.txt", "root")
	assertTarContains(t, transport.dials[1].session.written.Bytes(), "extra.txt", "extra")
}

func TestDaytonaProviderSyncFromRuntimeAppliesTarLastWriteWins(t *testing.T) {
	t.Parallel()

	localRoot := t.TempDir()
	additional := t.TempDir()
	writeTestFile(t, filepath.Join(localRoot, "root.txt"), "old")
	state := newProviderSessionState(t, localRoot, []string{additional})
	transport := &fakeTransport{
		readArchives: [][]byte{
			makeTar(t, map[string]string{"root.txt": "new"}),
			makeTar(t, map[string]string{"extra.txt": "extra"}),
		},
	}
	provider := newTestProviderWithTransport(transport)

	result, err := provider.SyncFromRuntime(context.Background(), state, sandbox.SyncOptions{
		Reason: sandbox.SyncReasonStop,
	})
	if err != nil {
		t.Fatalf("SyncFromRuntime() error = %v", err)
	}
	if got, want := result.FilesSynced, 2; got != want {
		t.Fatalf("SyncFromRuntime() FilesSynced = %d, want %d", got, want)
	}
	assertFileContent(t, filepath.Join(localRoot, "root.txt"), "new")
	assertFileContent(t, filepath.Join(additional, "extra.txt"), "extra")
}

func TestDaytonaProviderDestroyDeletesOrArchivesByPersistence(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		persistence sandbox.PersistenceMode
		wantDelete  int
		wantArchive int
	}{
		{name: "transient deletes", persistence: sandbox.PersistenceTransient, wantDelete: 1},
		{name: "archive archives", persistence: sandbox.PersistenceArchive, wantArchive: 1},
		{name: "reuse leaves sandbox", persistence: sandbox.PersistenceReuse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, client := newProviderWithFakeClient(t)
			sandbox := newFakeSandbox("sandbox-sync")
			client.sandboxes[sandbox.id] = sandbox
			state := newProviderSessionState(t, t.TempDir(), nil)
			ps, err := decodeProviderState(state.ProviderState)
			if err != nil {
				t.Fatal(err)
			}
			ps.Persistence = tc.persistence
			state.ProviderState, err = encodeProviderState(ps)
			if err != nil {
				t.Fatal(err)
			}

			if err := provider.Destroy(context.Background(), state); err != nil {
				t.Fatalf("Destroy() error = %v", err)
			}
			if sandbox.deleteCount != tc.wantDelete || sandbox.archiveCount != tc.wantArchive {
				t.Fatalf(
					"delete/archive = %d/%d, want %d/%d",
					sandbox.deleteCount,
					sandbox.archiveCount,
					tc.wantDelete,
					tc.wantArchive,
				)
			}
		})
	}
}

func TestDaytonaLauncherLaunchReturnsHandleStreams(t *testing.T) {
	t.Parallel()

	transport := &fakeTransport{
		readArchives: [][]byte{[]byte("stdout")},
	}
	launcher := &daytonaLauncher{
		transport: transport,
		sandbox:   sandboxInfo{ID: "sandbox", APIURL: defaultAPIURL},
	}
	handle, err := launcher.Launch(context.Background(), sandbox.LaunchSpec{
		Command: "cat",
		Cwd:     "/workspace",
		Env:     []string{"AGH_SESSION_ID=sess"},
	})
	if err != nil {
		t.Fatalf("Launch() error = %v", err)
	}
	if _, err := handle.Stdin().Write([]byte("stdin")); err != nil {
		t.Fatalf("Stdin().Write() error = %v", err)
	}
	output, err := io.ReadAll(handle.Stdout())
	if err != nil {
		t.Fatalf("ReadAll(Stdout()) error = %v", err)
	}
	if got, want := string(output), "stdout"; got != want {
		t.Fatalf("Stdout = %q, want %q", got, want)
	}
	if got := transport.dials[0].session.written.String(); got != "stdin" {
		t.Fatalf("captured stdin = %q, want stdin", got)
	}
	if handle.PID() != 0 {
		t.Fatalf("PID() = %d, want 0 for SSH handle", handle.PID())
	}
	if got, want := handle.Cwd(), "/workspace"; got != want {
		t.Fatalf("Cwd() = %q, want %q", got, want)
	}
	if handle.Stderr() != "" {
		t.Fatalf("Stderr() = %q, want empty", handle.Stderr())
	}
	select {
	case <-handle.Done():
	default:
		t.Fatal("Done() is not closed for completed fake session")
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if err := handle.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := handle.Stdin().Close(); err != nil {
		t.Fatalf("Stdin().Close() error = %v", err)
	}
	if err := handle.Stdout().Close(); err != nil {
		t.Fatalf("Stdout().Close() error = %v", err)
	}
}

func TestDaytonaToolHostFileOpsUseSandboxFilesystem(t *testing.T) {
	t.Parallel()

	sandbox := newFakeSandbox("sandbox-tools")
	host, err := newDaytonaToolHost(
		sandbox,
		&fakeTransport{},
		sandboxInfo{ID: sandbox.id, APIURL: defaultAPIURL},
		"/workspace",
		config.PermissionModeApproveAll,
	)
	if err != nil {
		t.Fatalf("newDaytonaToolHost() error = %v", err)
	}

	if err := host.WriteTextFile(context.Background(), "file.txt", "content"); err != nil {
		t.Fatalf("WriteTextFile() error = %v", err)
	}
	content, err := host.ReadTextFile(context.Background(), "file.txt")
	if err != nil {
		t.Fatalf("ReadTextFile() error = %v", err)
	}
	if content != "content" {
		t.Fatalf("ReadTextFile() = %q, want content", content)
	}
	if _, err := host.ResolvePath("../escape"); err == nil {
		t.Fatal("ResolvePath(escape) error = nil, want error")
	}
}

func TestDaytonaToolHostTerminalUsesSSHTransport(t *testing.T) {
	t.Parallel()

	transport := &fakeTransport{readArchives: [][]byte{[]byte("terminal output")}}
	host, err := newDaytonaToolHost(
		newFakeSandbox("sandbox-terminal"),
		transport,
		sandboxInfo{ID: "sandbox-terminal", APIURL: defaultAPIURL},
		"/workspace",
		config.PermissionModeApproveAll,
	)
	if err != nil {
		t.Fatalf("newDaytonaToolHost() error = %v", err)
	}
	response, err := host.CreateTerminal(context.Background(), acpsdk.CreateTerminalRequest{Command: "echo ok"})
	if err != nil {
		t.Fatalf("CreateTerminal() error = %v", err)
	}
	if _, err := host.WaitForTerminalExit(context.Background(), response.TerminalId); err != nil {
		t.Fatalf("WaitForTerminalExit() error = %v", err)
	}
	output, err := host.TerminalOutput(response.TerminalId)
	if err != nil {
		t.Fatalf("TerminalOutput() error = %v", err)
	}
	if output != "terminal output" {
		t.Fatalf("TerminalOutput() = %q, want terminal output", output)
	}
	if err := host.KillTerminal(response.TerminalId); err != nil {
		t.Fatalf("KillTerminal() error = %v", err)
	}
	if err := host.ReleaseTerminal(response.TerminalId); err != nil {
		t.Fatalf("ReleaseTerminal() error = %v", err)
	}
	if _, err := host.TerminalOutput(response.TerminalId); err == nil {
		t.Fatal("TerminalOutput(released) error = nil, want not found")
	}
}

func TestDaytonaToolHostCreateTerminalResolvesCwdWithinRuntimeRoot(t *testing.T) {
	t.Parallel()

	newHost := func() (*daytonaToolHost, *fakeTransport) {
		t.Helper()

		transport := &fakeTransport{readArchives: [][]byte{[]byte("terminal output")}}
		host, err := newDaytonaToolHost(
			newFakeSandbox("sandbox-terminal-cwd"),
			transport,
			sandboxInfo{ID: "sandbox-terminal-cwd", APIURL: defaultAPIURL},
			"/workspace",
			config.PermissionModeApproveAll,
		)
		if err != nil {
			t.Fatalf("newDaytonaToolHost() error = %v", err)
		}
		return host, transport
	}

	t.Run("Should rejects escaped cwd", func(t *testing.T) {
		t.Parallel()

		host, _ := newHost()
		escaped := "/outside"
		if _, err := host.CreateTerminal(context.Background(), acpsdk.CreateTerminalRequest{
			Command: "pwd",
			Cwd:     &escaped,
		}); err == nil {
			t.Fatal("CreateTerminal(escaped cwd) error = nil, want root-confined path validation error")
		}
	})

	t.Run("Should resolves relative cwd against runtime root", func(t *testing.T) {
		t.Parallel()

		host, transport := newHost()
		relative := "nested/work"
		response, err := host.CreateTerminal(context.Background(), acpsdk.CreateTerminalRequest{
			Command: "pwd",
			Cwd:     &relative,
		})
		if err != nil {
			t.Fatalf("CreateTerminal(relative cwd) error = %v", err)
		}
		if got, want := len(transport.dials), 1; got != want {
			t.Fatalf("transport dials = %d, want %d", got, want)
		}
		assertCommandContains(t, transport.dials[0].command, "/workspace/nested/work")
		if err := host.ReleaseTerminal(response.TerminalId); err != nil {
			t.Fatalf("ReleaseTerminal() error = %v", err)
		}
	})
}

func TestDaytonaToolHostPermissionDecisionModes(t *testing.T) {
	t.Parallel()

	readKind := acpsdk.ToolKindRead
	for _, tc := range []struct {
		name        string
		mode        config.PermissionMode
		kind        *acpsdk.ToolKind
		want        sandbox.PermissionDecision
		interactive bool
	}{
		{
			name: "approve all allows",
			mode: config.PermissionModeApproveAll,
			want: sandbox.PermissionDecisionAllowOnce,
		},
		{
			name: "approve reads allows read",
			mode: config.PermissionModeApproveReads,
			kind: &readKind,
			want: sandbox.PermissionDecisionAllowOnce,
		},
		{
			name:        "approve reads prompts write",
			mode:        config.PermissionModeApproveReads,
			want:        sandbox.PermissionDecisionPending,
			interactive: true,
		},
		{
			name:        "deny all prompts",
			mode:        config.PermissionModeDenyAll,
			want:        sandbox.PermissionDecisionPending,
			interactive: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			host, err := newDaytonaToolHost(
				newFakeSandbox("sandbox-perm"),
				&fakeTransport{},
				sandboxInfo{ID: "sandbox-perm", APIURL: defaultAPIURL},
				"/workspace",
				tc.mode,
			)
			if err != nil {
				t.Fatalf("newDaytonaToolHost() error = %v", err)
			}
			decision, interactive := host.PermissionDecision(acpsdk.RequestPermissionRequest{
				ToolCall: acpsdk.ToolCallUpdate{
					Kind:      tc.kind,
					Locations: []acpsdk.ToolCallLocation{{Path: "file.txt"}},
				},
			})
			if decision != tc.want || interactive != tc.interactive {
				t.Fatalf("PermissionDecision() = %q/%v, want %q/%v", decision, interactive, tc.want, tc.interactive)
			}
		})
	}
}

func TestDaytonaProviderRuntimeRootFallback(t *testing.T) {
	t.Parallel()

	provider := newTestProviderWithTransport(&fakeTransport{})
	configured := provider.runtimeRoot(context.Background(), &fakeSandbox{workingDir: "/ignored"}, "/configured")
	if configured != "/configured" {
		t.Fatalf("runtimeRoot(configured) = %q, want /configured", configured)
	}

	failing := &fakeSandbox{id: "sandbox-fail", files: map[string][]byte{}, workingDirErr: errors.New("boom")}
	fallback := provider.runtimeRoot(context.Background(), failing, "")
	if fallback != defaultRuntimeRoot {
		t.Fatalf("runtimeRoot(failing) = %q, want %q", fallback, defaultRuntimeRoot)
	}
}

func TestRemoteEnvAllowlist(t *testing.T) {
	t.Parallel()

	env := remoteEnvMap(
		[]string{
			"AGH_SESSION_ID=sess",
			"DAYTONA_API_KEY=secret",
			"PATH=/bin",
		},
		map[string]string{
			"NODE_ENV":        "test",
			"DAYTONA_API_KEY": "blocked",
		},
	)
	if _, ok := env["DAYTONA_API_KEY"]; ok {
		t.Fatal("remoteEnvMap propagated DAYTONA_API_KEY")
	}
	if got, want := env["AGH_SESSION_ID"], "sess"; got != want {
		t.Fatalf("AGH_SESSION_ID = %q, want %q", got, want)
	}
	if got, want := env["NODE_ENV"], "test"; got != want {
		t.Fatalf("NODE_ENV = %q, want %q", got, want)
	}
	if _, ok := env["PATH"]; ok {
		t.Fatal("remoteEnvMap propagated non-allowlisted PATH")
	}
}

func TestDaytonaNetworkPolicyWarnsOrErrorsForUnsupportedRequiredSettings(t *testing.T) {
	t.Parallel()

	provider := NewProvider(WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))).(*daytonaProvider)
	warnPolicy := sandbox.NetworkPolicy{AllowOutbound: true}
	if err := provider.validateNetworkPolicy(warnPolicy); err != nil {
		t.Fatalf("validateNetworkPolicy(warn) error = %v", err)
	}
	requiredPolicy := sandbox.NetworkPolicy{AllowOutbound: true, Required: true}
	if err := provider.validateNetworkPolicy(requiredPolicy); err == nil {
		t.Fatal("validateNetworkPolicy(required) error = nil, want error")
	}
}

func TestDaytonaProviderDefaultOptionsAndBackend(t *testing.T) {
	t.Parallel()

	provider := NewProvider(
		WithLogger(nil),
		withSandboxClientFactory(nil),
		withTransport(nil),
		withTokenManager(nil),
		withNow(nil),
	).(*daytonaProvider)
	if got, want := provider.Backend(), sandbox.BackendDaytona; got != want {
		t.Fatalf("Backend() = %q, want %q", got, want)
	}
	if provider.logger == nil {
		t.Fatal("logger = nil")
	}
	if provider.newClient == nil {
		t.Fatal("newClient = nil")
	}
	if provider.tokenManager == nil {
		t.Fatal("tokenManager = nil")
	}
	if provider.shellTransport == nil {
		t.Fatal("shellTransport = nil")
	}
	if provider.launcherTransport == nil {
		t.Fatal("launcherTransport = nil")
	}
	if provider.now == nil {
		t.Fatal("now = nil")
	}
	if provider.sdkTimeout != defaultSDKTimeout {
		t.Fatalf("sdkTimeout = %s, want %s", provider.sdkTimeout, defaultSDKTimeout)
	}
	if provider.createTimeout != defaultCreateTimeout {
		t.Fatalf("createTimeout = %s, want %s", provider.createTimeout, defaultCreateTimeout)
	}
	if provider.sshHost != defaultSSHHost {
		t.Fatalf("sshHost = %q, want %q", provider.sshHost, defaultSSHHost)
	}
}

func TestDaytonaDurationParsingAndShellHelpers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		raw  string
		want *int
	}{
		{name: "empty", raw: ""},
		{name: "minutes", raw: "15", want: new(15)},
		{name: "duration", raw: "90m", want: new(90)},
		{name: "invalid", raw: "not-a-duration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := parseDurationMinutes(tc.raw)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("parseDurationMinutes(%q) = %d, want nil", tc.raw, *got)
				}
				return
			}
			if got == nil || *got != *tc.want {
				t.Fatalf("parseDurationMinutes(%q) = %v, want %d", tc.raw, got, *tc.want)
			}
		})
	}

	cwd := "/workspace/custom dir"
	command := remoteTerminalCommand("/workspace/runtime", acpsdk.CreateTerminalRequest{
		Command: "printf",
		Args:    []string{"%s", "hello world"},
		Cwd:     &cwd,
		Env: []acpsdk.EnvVariable{
			{Name: "AGH_SESSION_ID", Value: "sess-daytona"},
			{Name: "DAYTONA_API_KEY", Value: "secret"},
			{Name: "", Value: "ignored"},
		},
	})
	assertCommandContains(t, command, "AGH_SESSION_ID=sess-daytona")
	assertCommandContains(t, command, "printf")
	if strings.Contains(command, "DAYTONA_API_KEY") || strings.Contains(command, "secret") {
		t.Fatalf("remoteTerminalCommand leaked blocked env var: %q", command)
	}

	dirs := remoteAdditionalDirs("/workspace/runtime", []string{"/tmp/one", "/", "/tmp/%%%/two three"})
	for _, want := range []string{
		"/workspace/runtime/.agh-additional/01-one",
		"/workspace/runtime/.agh-additional/02-dir",
		"/workspace/runtime/.agh-additional/03-two-three",
	} {
		if !containsString(dirs, want) {
			t.Fatalf("remoteAdditionalDirs() = %#v, missing %q", dirs, want)
		}
	}
	if got, want := sanitizeRemoteBase(" ... "), defaultRemoteAdditionalBase; got != want {
		t.Fatalf("sanitizeRemoteBase() = %q, want %q", got, want)
	}
}

func TestDaytonaToolHostConstructorAuthorizationAndPaths(t *testing.T) {
	t.Parallel()

	fakeSB := newFakeSandbox("sandbox-toolhost")
	transport := &fakeTransport{}
	info := sandboxInfo{ID: "sandbox-toolhost", APIURL: defaultAPIURL}
	if _, err := newDaytonaToolHost(nil, transport, info, "/workspace", ""); err == nil {
		t.Fatal("newDaytonaToolHost(nil sandbox) error = nil")
	}
	if _, err := newDaytonaToolHost(fakeSB, nil, info, "/workspace", ""); err == nil {
		t.Fatal("newDaytonaToolHost(nil transport) error = nil")
	}
	if _, err := newDaytonaToolHost(fakeSB, transport, info, "/workspace", "invalid-mode"); err == nil {
		t.Fatal("newDaytonaToolHost(invalid permission) error = nil")
	}

	host, err := newDaytonaToolHost(fakeSB, transport, info, "/workspace", "")
	if err != nil {
		t.Fatalf("newDaytonaToolHost(default permission) error = %v", err)
	}
	if err := host.Authorize(sandbox.PermissionOperationReadTextFile); err != nil {
		t.Fatalf("Authorize(read) error = %v", err)
	}
	if err := host.Authorize(sandbox.PermissionOperationWriteTextFile); err == nil {
		t.Fatal("Authorize(write) error = nil, want blocked by approve-reads")
	}
	resolved, err := host.ResolvePath("nested/file.txt")
	if err != nil {
		t.Fatalf("ResolvePath(relative) error = %v", err)
	}
	if got, want := resolved, "/workspace/nested/file.txt"; got != want {
		t.Fatalf("ResolvePath(relative) = %q, want %q", got, want)
	}
	if _, err := host.ResolvePath("/outside/file.txt"); err == nil {
		t.Fatal("ResolvePath(escape) error = nil")
	}

	allowAll, err := newDaytonaToolHost(
		fakeSB,
		transport,
		info,
		"/workspace",
		config.PermissionModeApproveAll,
	)
	if err != nil {
		t.Fatalf("newDaytonaToolHost(approve-all) error = %v", err)
	}
	if err := allowAll.Authorize(sandbox.PermissionOperationCreateTerminal); err != nil {
		t.Fatalf("Authorize(create terminal) error = %v", err)
	}
	decision, interactive := allowAll.PermissionDecision(acpsdk.RequestPermissionRequest{
		ToolCall: acpsdk.ToolCallUpdate{
			Locations: []acpsdk.ToolCallLocation{{Path: "/outside/file.txt"}},
		},
	})
	if decision != sandbox.PermissionDecisionRejectOnce || interactive {
		t.Fatalf("PermissionDecision(escape) = %q/%v, want reject_once/false", decision, interactive)
	}

	denyAll, err := newDaytonaToolHost(
		fakeSB,
		transport,
		info,
		"/workspace",
		config.PermissionModeDenyAll,
	)
	if err != nil {
		t.Fatalf("newDaytonaToolHost(deny-all) error = %v", err)
	}
	if err := denyAll.Authorize(sandbox.PermissionOperationReadTextFile); err == nil {
		t.Fatal("Authorize(read) error = nil, want blocked by deny-all")
	}
}

func TestDaytonaToolHostTerminalOutputLimitAndFailures(t *testing.T) {
	t.Parallel()

	limit := 5
	transport := &fakeTransport{
		readArchives: [][]byte{[]byte("abcdef")},
		nextStderr:   "XYZ",
		nextWaitErr:  errors.New("remote failed"),
	}
	host, err := newDaytonaToolHost(
		newFakeSandbox("sandbox-terminal-limit"),
		transport,
		sandboxInfo{ID: "sandbox-terminal-limit", APIURL: defaultAPIURL},
		"/workspace",
		config.PermissionModeApproveAll,
	)
	if err != nil {
		t.Fatalf("newDaytonaToolHost() error = %v", err)
	}
	response, err := host.CreateTerminal(context.Background(), acpsdk.CreateTerminalRequest{
		Command:         "sh",
		OutputByteLimit: &limit,
	})
	if err != nil {
		t.Fatalf("CreateTerminal() error = %v", err)
	}
	exitCode, err := host.WaitForTerminalExit(context.Background(), response.TerminalId)
	if err == nil {
		t.Fatal("WaitForTerminalExit() error = nil, want wait error")
	}
	if exitCode != 1 {
		t.Fatalf("WaitForTerminalExit() exitCode = %d, want 1", exitCode)
	}
	output, err := host.TerminalOutput(response.TerminalId)
	if err != nil {
		t.Fatalf("TerminalOutput() error = %v", err)
	}
	if got, want := output, "efXYZ"; got != want {
		t.Fatalf("TerminalOutput() = %q, want %q", got, want)
	}

	host.terminalsMu.Lock()
	host.terminals["slow"] = &remoteTerminal{done: make(chan struct{})}
	host.terminalsMu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := host.WaitForTerminalExit(ctx, "slow"); err == nil {
		t.Fatal("WaitForTerminalExit(canceled) error = nil")
	}

	var buf bytes.Buffer
	appendLimited(&buf, []byte("abcdef"), 0)
	if got, want := buf.String(), "abcdef"; got != want {
		t.Fatalf("appendLimited(no limit) = %q, want %q", got, want)
	}
	appendLimited(&buf, []byte("ghijk"), 4)
	if got, want := buf.String(), "hijk"; got != want {
		t.Fatalf("appendLimited(limit) = %q, want %q", got, want)
	}
}

func newProviderWithFakeClient(t *testing.T) (*daytonaProvider, *fakeSandboxClient) {
	t.Helper()
	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	client := &fakeSandboxClient{
		created:   newFakeSandbox("sandbox-created"),
		sandboxes: make(map[string]*fakeSandbox),
		findErr:   errSandboxNotFound,
	}
	tokenSource := &fakeTokenSource{access: []sshAccess{{
		Token:     "ssh-token",
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}}}
	return newTestProvider(t, client, &fakeTransport{}, tokenSource, now), client
}

func newTestProvider(
	t *testing.T,
	client *fakeSandboxClient,
	transport transport,
	tokenSource sshTokenSource,
	now time.Time,
) *daytonaProvider {
	t.Helper()
	if client.sandboxes == nil {
		client.sandboxes = make(map[string]*fakeSandbox)
	}
	manager := newSSHTokenManager(tokenSource, func() time.Time { return now })
	provider := NewProvider(
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		withSandboxClientFactory(func(clientConfig) (sandboxClient, error) { return client, nil }),
		withTokenManager(manager),
		withTransport(transport),
		withNow(func() time.Time { return now }),
	).(*daytonaProvider)
	provider.sdkTimeout = time.Second
	provider.createTimeout = time.Second
	return provider
}

func newTestProviderWithTransport(transport transport) *daytonaProvider {
	return &daytonaProvider{
		logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		shellTransport:    transport,
		launcherTransport: transport,
		sdkTimeout:        time.Second,
		now:               time.Now,
	}
}

func newDaytonaPrepareRequest(t *testing.T) sandbox.PrepareRequest {
	t.Helper()
	return sandbox.PrepareRequest{
		SessionID:           "sess-daytona",
		WorkspaceID:         "workspace-daytona",
		SandboxID:           "env-daytona",
		LocalRootDir:        t.TempDir(),
		LocalAdditionalDirs: []string{t.TempDir()},
		Sandbox: sandbox.Resolved{
			Profile:        "daytona-dev",
			Backend:        sandbox.BackendDaytona,
			SyncMode:       sandbox.SyncModeSessionBidirectional,
			Persistence:    sandbox.PersistenceTransient,
			RuntimeRootDir: "/workspace/runtime",
			Network:        sandbox.NetworkPolicy{AllowPublicIngress: false},
			Daytona: &sandbox.DaytonaConfig{
				APIURL:        defaultAPIURL,
				Image:         "ubuntu:24.04",
				Snapshot:      "snap-base",
				StartupSource: sandbox.DaytonaStartupSourceSnapshot,
				StartupRef:    "snap-base",
			},
		},
		AgentCommand: "cat",
		AgentEnv:     []string{"AGH_SESSION_ID=sess-daytona"},
		Permissions:  string(config.PermissionModeApproveAll),
	}
}

func newProviderSessionState(
	t *testing.T,
	localRoot string,
	localAdditional []string,
) sandbox.SessionState {
	t.Helper()
	runtimeAdditional := remoteAdditionalDirs("/runtime/root", localAdditional)
	ps := providerState{
		Version:               providerStateVersion,
		SandboxID:             "sandbox-sync",
		APIURL:                defaultAPIURL,
		LocalRootDir:          localRoot,
		LocalAdditionalDirs:   cloneStrings(localAdditional),
		RuntimeRootDir:        "/runtime/root",
		RuntimeAdditionalDirs: runtimeAdditional,
		Persistence:           sandbox.PersistenceTransient,
	}
	raw, err := encodeProviderState(ps)
	if err != nil {
		t.Fatalf("encodeProviderState() error = %v", err)
	}
	return sandbox.SessionState{
		SandboxID:             "env-sync",
		Backend:               sandbox.BackendDaytona,
		InstanceID:            "sandbox-sync",
		RuntimeRootDir:        "/runtime/root",
		RuntimeAdditionalDirs: runtimeAdditional,
		ProviderState:         raw,
	}
}

type fakeSandboxClient struct {
	created        *fakeSandbox
	sandboxes      map[string]*fakeSandbox
	createRequests []createSandboxRequest
	getIDs         []string
	findLabels     []map[string]string
	findErr        error
}

func (c *fakeSandboxClient) Create(_ context.Context, req createSandboxRequest) (daytonaSandbox, error) {
	c.createRequests = append(c.createRequests, req)
	if c.created == nil {
		c.created = newFakeSandbox("sandbox-created")
	}
	c.sandboxes[c.created.id] = c.created
	return c.created, nil
}

func (c *fakeSandboxClient) Get(_ context.Context, id string) (daytonaSandbox, error) {
	c.getIDs = append(c.getIDs, id)
	sandbox, ok := c.sandboxes[id]
	if !ok {
		return nil, errSandboxNotFound
	}
	return sandbox, nil
}

func (c *fakeSandboxClient) FindOne(_ context.Context, labels map[string]string) (daytonaSandbox, error) {
	c.findLabels = append(c.findLabels, labels)
	if c.findErr != nil {
		return nil, c.findErr
	}
	return c.created, nil
}

type fakeSandbox struct {
	id            string
	name          string
	workingDir    string
	workingDirErr error
	files         map[string][]byte
	startCount    int
	archiveCount  int
	deleteCount   int
}

func newFakeSandbox(id string) *fakeSandbox {
	return &fakeSandbox{
		id:         id,
		name:       "name-" + id,
		workingDir: "/workspace/runtime",
		files:      make(map[string][]byte),
	}
}

func (s *fakeSandbox) ID() string { return s.id }

func (s *fakeSandbox) Name() string { return s.name }

func (s *fakeSandbox) Start(context.Context) error {
	s.startCount++
	return nil
}

func (s *fakeSandbox) Archive(context.Context) error {
	s.archiveCount++
	return nil
}

func (s *fakeSandbox) Delete(context.Context) error {
	s.deleteCount++
	return nil
}

func (s *fakeSandbox) WorkingDir(context.Context) (string, error) {
	if s.workingDirErr != nil {
		return "", s.workingDirErr
	}
	return s.workingDir, nil
}

func (s *fakeSandbox) ReadFile(_ context.Context, path string) ([]byte, error) {
	content, ok := s.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), content...), nil
}

func (s *fakeSandbox) WriteFile(_ context.Context, path string, content []byte) error {
	s.files[path] = append([]byte(nil), content...)
	return nil
}

type fakeTokenSource struct {
	access []sshAccess
	calls  int
}

func (s *fakeTokenSource) FetchSSHAccess(
	context.Context,
	string,
	string,
	time.Duration,
) (sshAccess, error) {
	s.calls++
	if len(s.access) == 0 {
		return sshAccess{}, errors.New("missing fake token")
	}
	if s.calls > len(s.access) {
		return s.access[len(s.access)-1], nil
	}
	return s.access[s.calls-1], nil
}

type fakeTransport struct {
	dials        []fakeDial
	readArchives [][]byte
	nextWaitErr  error
	nextStderr   string
}

type fakeDial struct {
	sandbox sandboxInfo
	command string
	session *fakeSession
}

func (t *fakeTransport) Dial(_ context.Context, sandbox sandboxInfo, command string) (transportSession, error) {
	var read []byte
	if len(t.readArchives) > 0 {
		read = t.readArchives[0]
		t.readArchives = t.readArchives[1:]
	}
	session := newFakeSession(read)
	session.waitErr = t.nextWaitErr
	session.stderr = t.nextStderr
	t.dials = append(t.dials, fakeDial{sandbox: sandbox, command: command, session: session})
	return session, nil
}

type fakeSession struct {
	read        *bytes.Reader
	written     bytes.Buffer
	done        chan struct{}
	waitErr     error
	stderr      string
	closedWrite bool
}

func newFakeSession(read []byte) *fakeSession {
	done := make(chan struct{})
	close(done)
	return &fakeSession{
		read: bytes.NewReader(read),
		done: done,
	}
}

func (s *fakeSession) Read(p []byte) (int, error) { return s.read.Read(p) }

func (s *fakeSession) Write(p []byte) (int, error) { return s.written.Write(p) }

func (s *fakeSession) Close() error { return nil }

func (s *fakeSession) CloseWrite() error {
	s.closedWrite = true
	return nil
}

func (s *fakeSession) Done() <-chan struct{} { return s.done }

func (s *fakeSession) Wait() error { return s.waitErr }

func (s *fakeSession) Stop(context.Context) error { return nil }

func (s *fakeSession) Stderr() string { return s.stderr }

func makeTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	for name, content := range files {
		data := []byte(content)
		if err := writer.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o600,
			Size: int64(len(data)),
		}); err != nil {
			t.Fatalf("WriteHeader(%q) error = %v", name, err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("tar.Close() error = %v", err)
	}
	return buf.Bytes()
}

func assertTarContains(t *testing.T, data []byte, name string, content string) {
	t.Helper()
	dest := t.TempDir()
	if _, err := extractTar(dest, bytes.NewReader(data)); err != nil {
		t.Fatalf("extractTar(captured) error = %v", err)
	}
	assertFileContent(t, filepath.Join(dest, name), content)
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func assertFileContent(t *testing.T, path string, content string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if got := string(data); got != content {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, got, content)
	}
}

func assertCommandContains(t *testing.T, command string, want string) {
	t.Helper()
	if !strings.Contains(command, want) {
		t.Fatalf("command %q does not contain %q", command, want)
	}
}

func containsString(values []string, want string) bool {
	return slices.Contains(values, want)
}

func containsKey(values []string, key string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, key+"=") {
			return true
		}
	}
	return false
}
