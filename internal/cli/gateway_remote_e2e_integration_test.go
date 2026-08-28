//go:build integration

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/httpapi"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store/globaldb"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const remoteGatewayE2ETimeout = 10 * time.Second

func TestRemoteGatewayE2EProfileWorkSurvivesDisconnectAndReconnect(t *testing.T) {
	t.Run(
		"Should pair, preserve admitted work, reconnect, and refuse local-only operations [E2E-002]",
		func(t *testing.T) {
			harness := newIntegrationHarness(t)
			mustExecuteRoot(t, harness.deps, "daemon", "start", "-o", "json")

			sessionOut := mustExecuteRoot(
				t,
				harness.deps,
				"session", "new", "--agent", "coder", "--cwd", harness.workspace, "-o", "json",
			)
			var created SessionRecord
			if err := json.Unmarshal([]byte(sessionOut), &created); err != nil {
				t.Fatalf("json.Unmarshal(session new) error = %v", err)
			}

			gatewayService, closeGatewayStore := newRemoteGatewayE2EService(t, harness)
			defer closeGatewayStore()
			privateServer := startRemoteGatewayE2EPrivateServer(t, harness, gatewayService)
			tlsGateway := startRemoteGatewayE2ETLSProxy(t, privateServer)
			remoteDeps := newRemoteGatewayE2ECommandDeps(t, harness, tlsGateway)

			artifact, err := gatewayService.MintPairing(t.Context(), gateway.PairingRequest{
				Source: gateway.PairingSourceLocal,
			})
			if err != nil {
				t.Fatalf("MintPairing() error = %v", err)
			}
			pairOut, _, err := executeRootCommand(
				t,
				remoteDeps,
				"pair", "redeem", artifact.Artifact,
				"--name", "e2e-remote", "--address", tlsGateway.URL, "--use", "-o", "json",
			)
			if err != nil {
				t.Fatalf("pair redeem error = %v", err)
			}
			if strings.Contains(pairOut, gatewayDeviceCredentialPrefix) {
				t.Fatalf("pair redeem output leaked the device credential: %s", pairOut)
			}
			assertRemoteGatewayE2EProfileAtRest(t, harness.homePaths)

			promptCtx, cancelPrompt := context.WithCancel(t.Context())
			promptResult, promptOutput := executeRemoteGatewayE2EPrompt(
				remoteDeps,
				promptCtx,
				created.ID,
				"long work __block__",
			)
			blockedPromptID, blocked := waitForRemoteGatewayE2EBlocked(harness.runner, remoteGatewayE2ETimeout)
			if !blocked {
				cancelPrompt()
				select {
				case promptErr := <-promptResult:
					t.Fatalf(
						"remote prompt ended before entering blocked work: %v; output=%q",
						promptErr,
						promptOutput.String(),
					)
				case <-time.After(time.Second):
				}
				t.Fatal("remote prompt did not reach the admitted blocked-work state")
			}
			waitForRemoteGatewayE2EOutput(t, promptOutput, `"type":"start"`)
			cancelPrompt()
			if promptErr := <-promptResult; !errors.Is(promptErr, context.Canceled) &&
				!errors.Is(promptErr, errSessionPromptStreamIncomplete) {
				t.Fatalf("disconnected remote prompt error = %v, want canceled or incomplete stream", promptErr)
			}

			if status, err := harness.runner.manager.Status(t.Context(), created.ID); err != nil || status == nil {
				t.Fatalf("Status(admitted remote work) = %#v, %v", status, err)
			}
			releaseRemoteGatewayE2EBlocked(harness.runner, blockedPromptID)
			waitForRemoteGatewayE2ESessionEvent(t, remoteDeps, created.ID, "done")

			client, err := clientFromDeps(remoteDeps)
			if err != nil {
				t.Fatalf("clientFromDeps(reconnected) error = %v", err)
			}
			remoteClient, ok := client.(*daemonClient)
			if !ok {
				t.Fatalf("reconnected client = %T, want *daemonClient", client)
			}
			localOnlyErr := remoteClient.doJSON(
				t.Context(),
				http.MethodPost,
				"/api/agent/tasks/claim-next",
				nil,
				struct{}{},
				nil,
			)
			assertGatewayClientErrorCode(t, localOnlyErr, "gateway_local_only_operation")
		},
	)
}

type remoteGatewayE2EPolicy struct{}

func (remoteGatewayE2EPolicy) Plan(context.Context) (gateway.ExposurePlan, error) {
	return gateway.ExposurePlan{}, nil
}

func (remoteGatewayE2EPolicy) Transition(
	context.Context,
	gateway.TransitionRequest,
) (gateway.Status, error) {
	return remoteGatewayE2EStatus(), nil
}

func (remoteGatewayE2EPolicy) Reconcile(context.Context) (gateway.Status, error) {
	return remoteGatewayE2EStatus(), nil
}

func (remoteGatewayE2EPolicy) SetEnabled(context.Context, bool) error { return nil }

func (remoteGatewayE2EPolicy) Status(context.Context) (gateway.Status, error) {
	return remoteGatewayE2EStatus(), nil
}

func (remoteGatewayE2EPolicy) Acquire(tier gateway.Tier, surface gateway.Surface) (func(), error) {
	if tier != gateway.TierPrivate || surface != gateway.SurfaceOperatorUI {
		return nil, gateway.ErrExposureRefused
	}
	return func() {}, nil
}

func (remoteGatewayE2EPolicy) Close(context.Context) error { return nil }

func remoteGatewayE2EStatus() gateway.Status {
	return gateway.Status{
		Enabled: true,
		Tiers: []gateway.TierStatus{{
			Tier: gateway.TierPrivate, Desired: gateway.DesiredEnabled,
			Observed: string(gateway.SurfaceOn), ListenerAddress: "127.0.0.1:0", Advertised: true,
		}},
	}
}

func newRemoteGatewayE2EService(
	t *testing.T,
	harness integrationHarness,
) (*gateway.Service, func()) {
	t.Helper()
	store, err := globaldb.OpenGlobalDB(t.Context(), harness.homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB(gateway E2E) error = %v", err)
	}
	devices, err := gateway.NewDeviceService(store)
	if err != nil {
		if closeErr := store.Close(t.Context()); closeErr != nil {
			t.Errorf("Close(gateway E2E store after device-service failure) error = %v", closeErr)
		}
		t.Fatalf("NewDeviceService() error = %v", err)
	}
	service, err := gateway.NewService(remoteGatewayE2EPolicy{}, devices)
	if err != nil {
		if closeErr := store.Close(t.Context()); closeErr != nil {
			t.Errorf("Close(gateway E2E store after service failure) error = %v", closeErr)
		}
		t.Fatalf("NewService() error = %v", err)
	}
	return service, func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), remoteGatewayE2ETimeout)
		defer cancel()
		if err := store.Close(closeCtx); err != nil {
			t.Errorf("Close(gateway E2E store) error = %v", err)
		}
	}
}

func startRemoteGatewayE2EPrivateServer(
	t *testing.T,
	harness integrationHarness,
	gatewayService *gateway.Service,
) *httpapi.Server {
	t.Helper()
	harness.runner.mu.Lock()
	manager := harness.runner.manager
	tasks := harness.runner.tasks
	observer := harness.runner.observer
	harness.runner.mu.Unlock()
	if manager == nil || tasks == nil || observer == nil {
		t.Fatal("integration daemon services are not ready")
	}
	workspaceStore, err := globaldb.OpenGlobalDB(t.Context(), harness.homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB(workspace E2E) error = %v", err)
	}
	resolver, err := workspacepkg.NewResolver(
		workspaceStore,
		workspacepkg.WithHomePaths(harness.homePaths),
		workspacepkg.WithLogger(discardLogger()),
		workspacepkg.WithConfigLoader(func(string) (compozyconfig.Config, error) {
			return harness.runner.cfg, nil
		}),
		workspacepkg.WithProfileConfigLoader(func(string, string) (compozyconfig.Config, error) {
			return harness.runner.cfg, nil
		}),
	)
	if err != nil {
		if closeErr := workspaceStore.Close(t.Context()); closeErr != nil {
			t.Errorf("Close(workspace E2E store after resolver failure) error = %v", closeErr)
		}
		t.Fatalf("NewResolver() error = %v", err)
	}
	server, err := httpapi.New(
		httpapi.WithHomePaths(harness.homePaths),
		httpapi.WithConfig(&harness.runner.cfg),
		httpapi.WithLogger(discardLogger()),
		httpapi.WithHost("127.0.0.1"),
		httpapi.WithPort(0),
		httpapi.WithSurfaceSet(httpapi.SurfaceSetPrivate),
		httpapi.WithSessionManager(manager),
		httpapi.WithSessionCatalog(workspaceStore),
		httpapi.WithTaskService(tasks),
		httpapi.WithObserver(observer),
		httpapi.WithWorkspaceResolver(resolver),
		httpapi.WithGatewayService(gatewayService),
		httpapi.WithDeviceAuth(gatewayService),
		httpapi.WithGatewayChallenge(gateway.TierPrivate, gateway.NewChallengeRegistry()),
	)
	if err != nil {
		if closeErr := workspaceStore.Close(t.Context()); closeErr != nil {
			t.Errorf("Close(workspace E2E store after server failure) error = %v", closeErr)
		}
		t.Fatalf("httpapi.New(private E2E) error = %v", err)
	}
	if err := server.Start(t.Context()); err != nil {
		if closeErr := workspaceStore.Close(t.Context()); closeErr != nil {
			t.Errorf("Close(workspace E2E store after start failure) error = %v", closeErr)
		}
		t.Fatalf("Start(private E2E server) error = %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), remoteGatewayE2ETimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			t.Errorf("Shutdown(private E2E server) error = %v", err)
		}
		if err := workspaceStore.Close(shutdownCtx); err != nil {
			t.Errorf("Close(workspace E2E store) error = %v", err)
		}
	})
	return server
}

func startRemoteGatewayE2ETLSProxy(t *testing.T, privateServer *httpapi.Server) *httptest.Server {
	t.Helper()
	upstream, err := url.Parse("http://" + netip.AddrPortFrom(
		netip.MustParseAddr("127.0.0.1"),
		uint16(privateServer.Port()),
	).String())
	if err != nil {
		t.Fatalf("url.Parse(private E2E server) error = %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Transport = &http.Transport{Proxy: nil}
	tlsGateway := httptest.NewTLSServer(proxy)
	t.Cleanup(func() {
		tlsGateway.CloseClientConnections()
		tlsGateway.Close()
		proxy.Transport.(*http.Transport).CloseIdleConnections()
	})
	return tlsGateway
}

func newRemoteGatewayE2ECommandDeps(
	t *testing.T,
	harness integrationHarness,
	tlsGateway *httptest.Server,
) commandDeps {
	t.Helper()
	keys := &memoryCredentialKeyring{values: make(map[string]string)}
	credentialStore := gatewayCredentialStore{
		dir:     harness.homePaths.GatewayCredentialsDir,
		keys:    keys,
		entropy: bytes.NewReader(bytes.Repeat([]byte{0x6d}, 4096)),
	}
	trustedTransport, ok := tlsGateway.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("TLS gateway transport = %T, want *http.Transport", tlsGateway.Client().Transport)
	}
	deps := commandDeps{
		resolveHome: func() (compozyconfig.HomePaths, error) { return harness.homePaths, nil },
		loadConfig: func() (compozyconfig.Config, error) {
			return compozyconfig.LoadForHome(harness.homePaths)
		},
		getwd: func() (string, error) { return harness.workspace, nil },
		writeGatewayCredential: func(_ string, name string, credential string) (string, error) {
			return credentialStore.write(name, credential)
		},
		readGatewayCredential: func(_ string, name string) (string, error) {
			return credentialStore.read(name)
		},
		removeGatewayCredential: func(_ string, name string) error {
			return credentialStore.remove(name)
		},
	}
	deps.newClient = func(target ClientTarget) (DaemonClient, error) {
		client, err := NewClient(target)
		if err != nil {
			return nil, err
		}
		if !target.isRemoteGateway() {
			return client, nil
		}
		concrete, ok := client.(*daemonClient)
		if !ok {
			return nil, fmt.Errorf("remote gateway E2E client = %T, want *daemonClient", client)
		}
		checkRedirect := func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		concrete.httpClient = &http.Client{
			Transport: trustedTransport.Clone(), Timeout: defaultUnixSocketClientTimeout,
			CheckRedirect: checkRedirect,
		}
		concrete.streamClient = &http.Client{
			Transport: trustedTransport.Clone(), CheckRedirect: checkRedirect,
		}
		return concrete, nil
	}
	return deps.withDefaults()
}

func assertRemoteGatewayE2EProfileAtRest(t *testing.T, homePaths compozyconfig.HomePaths) {
	t.Helper()
	cfg, err := compozyconfig.LoadForHome(homePaths)
	if err != nil {
		t.Fatalf("LoadForHome(paired E2E profile) error = %v", err)
	}
	profile, exists := findGatewayProfile(cfg.Gateway.Connections, "e2e-remote")
	if !exists || cfg.Gateway.ActiveConnection != "e2e-remote" || profile.Scheme != "https" {
		t.Fatalf("paired E2E profile = %#v active=%q", profile, cfg.Gateway.ActiveConnection)
	}
	credentialPath := filepath.Join(homePaths.GatewayCredentialsDir, "e2e-remote.cred")
	info, err := os.Stat(credentialPath)
	if err != nil {
		t.Fatalf("Stat(E2E credential) error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("E2E credential permissions = %o, want 600", info.Mode().Perm())
	}
	contents, err := os.ReadFile(credentialPath)
	if err != nil {
		t.Fatalf("ReadFile(E2E credential) error = %v", err)
	}
	if bytes.Contains(contents, []byte(gatewayDeviceCredentialPrefix)) {
		t.Fatal("E2E credential file contains a plaintext credential")
	}
}

func executeRemoteGatewayE2EPrompt(
	deps commandDeps,
	ctx context.Context,
	sessionID string,
	message string,
) (<-chan error, *lockedBuffer) {
	result := make(chan error, 1)
	output := &lockedBuffer{}
	go func() {
		command := newRootCommand(deps)
		command.SetOut(output)
		command.SetErr(io.Discard)
		command.SetArgs([]string{"session", "prompt", sessionID, message, "-o", "jsonl"})
		result <- command.ExecuteContext(ctx)
	}()
	return result, output
}

func waitForRemoteGatewayE2EOutput(t *testing.T, output *lockedBuffer, fragment string) {
	t.Helper()
	deadline := time.Now().Add(remoteGatewayE2ETimeout)
	for time.Now().Before(deadline) {
		if strings.Contains(output.String(), fragment) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("remote prompt output = %q, want %q", output.String(), fragment)
}

func waitForRemoteGatewayE2EBlocked(runner *integrationDaemon, timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		runner.mu.Lock()
		driver := runner.driver
		runner.mu.Unlock()
		if driver != nil {
			driver.mu.Lock()
			for promptID := range driver.blocked {
				driver.mu.Unlock()
				return promptID, true
			}
			driver.mu.Unlock()
		}
		time.Sleep(20 * time.Millisecond)
	}
	return "", false
}

func releaseRemoteGatewayE2EBlocked(runner *integrationDaemon, promptID string) {
	runner.mu.Lock()
	driver := runner.driver
	runner.mu.Unlock()
	if driver != nil {
		driver.releaseBlocked(promptID)
	}
}

func waitForRemoteGatewayE2ESessionEvent(
	t *testing.T,
	deps commandDeps,
	sessionID string,
	eventType string,
) {
	t.Helper()
	deadline := time.Now().Add(remoteGatewayE2ETimeout)
	var lastOutput string
	var lastErr error
	for time.Now().Before(deadline) {
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session", "events", sessionID, "--last", strconv.Itoa(20), "-o", "json",
		)
		lastOutput = stdout
		lastErr = err
		if err == nil {
			var events []SessionEventRecord
			if decodeErr := json.Unmarshal([]byte(stdout), &events); decodeErr == nil {
				for _, event := range events {
					if event.Type == eventType {
						return
					}
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf(
		"reconnected session %q did not expose %q progress: error=%v output=%s",
		sessionID,
		eventType,
		lastErr,
		lastOutput,
	)
}
