//go:build integration

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitestutil "github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/api/udsapi"
	automationpkg "github.com/compozy/compozy/internal/automation"
	bridgepkg "github.com/compozy/compozy/internal/bridges"
	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/network"
	"github.com/compozy/compozy/internal/observe"
	registrypkg "github.com/compozy/compozy/internal/registry"
	sandboxlocal "github.com/compozy/compozy/internal/sandbox/local"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/soul"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/version"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestCLIRoundTripIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)

	startOut, _, err := executeRootCommand(t, h.deps, "daemon", "start", "-o", "json")
	if err != nil {
		t.Fatalf("daemon start error = %v", err)
	}
	var started DaemonStatus
	if err := json.Unmarshal([]byte(startOut), &started); err != nil {
		t.Fatalf("json.Unmarshal(start) error = %v", err)
	}
	if started.Status != "running" {
		t.Fatalf("start status = %q, want %q", started.Status, "running")
	}

	newOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"demo",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new error = %v", err)
	}
	var created SessionRecord
	if err := json.Unmarshal([]byte(newOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created session id")
	}

	promptOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"prompt",
		created.ID,
		"hello __usage__",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session prompt error = %v", err)
	}
	var promptEvents []AgentEventRecord
	if err := json.Unmarshal([]byte(promptOut), &promptEvents); err != nil {
		t.Fatalf("json.Unmarshal(prompt) error = %v", err)
	}
	if len(promptEvents) < 2 {
		t.Fatalf("prompt events = %d, want at least 2", len(promptEvents))
	}

	usageOut := mustExecuteRoot(t, h.deps, "session", "usage", created.ID, "-o", "json")
	var usage SessionUsageRecord
	if err := json.Unmarshal([]byte(usageOut), &usage); err != nil {
		t.Fatalf("json.Unmarshal(session usage) error = %v", err)
	}
	if usage.TotalTokens == nil || *usage.TotalTokens != 15 ||
		usage.CostStatus != "unknown" || usage.CostSource != "none" {
		t.Fatalf("session usage = %#v, want 15 tokens with unknown/none provenance", usage)
	}

	eventsOut, _, err := executeRootCommand(t, h.deps, "session", "events", created.ID, "-o", "json")
	if err != nil {
		t.Fatalf("session events error = %v", err)
	}
	var events []SessionEventRecord
	if err := json.Unmarshal([]byte(eventsOut), &events); err != nil {
		t.Fatalf("json.Unmarshal(events) error = %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("session events = %d, want at least 2", len(events))
	}

	stopOut, _, err := executeRootCommand(t, h.deps, "session", "stop", created.ID, "-o", "json")
	if err != nil {
		t.Fatalf("session stop error = %v", err)
	}
	var stopped SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stopped); err != nil {
		t.Fatalf("json.Unmarshal(stop) error = %v", err)
	}
	if stopped.State != session.StateStopped {
		t.Fatalf("stopped.State = %q, want %q", stopped.State, session.StateStopped)
	}

	daemonStopOut, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
	if err != nil {
		t.Fatalf("daemon stop error = %v", err)
	}
	var daemonStopped DaemonStatus
	if err := json.Unmarshal([]byte(daemonStopOut), &daemonStopped); err != nil {
		t.Fatalf("json.Unmarshal(daemon stop) error = %v", err)
	}
	if daemonStopped.Status != "stopped" {
		t.Fatalf("daemon stop status = %q, want %q", daemonStopped.Status, "stopped")
	}

	if err := h.runner.waitForExit(); err != nil {
		t.Fatalf("waitForExit() error = %v", err)
	}
}

func TestRemoteCLIProfilesIntegrationIT060ThroughIT066(t *testing.T) {
	t.Run("Should keep remote profiles, streams, work, and revocation consistent [IT-060..066]", func(t *testing.T) {
		t.Parallel()
		state := newRemoteCLIIntegrationState()
		server := httptest.NewTLSServer(http.HandlerFunc(state.serveHTTP))
		t.Cleanup(server.Close)
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		credentials := make(map[string]string)
		var credentialsMu sync.Mutex
		credentialKey := func(dir string, name string) string { return filepath.Join(dir, name) }
		deps := commandDeps{
			resolveHome: func() (compozyconfig.HomePaths, error) { return homePaths, nil },
			loadConfig:  func() (compozyconfig.Config, error) { return compozyconfig.LoadForHome(homePaths) },
			writeGatewayCredential: func(dir string, name string, credential string) (string, error) {
				credentialsMu.Lock()
				defer credentialsMu.Unlock()
				if err := os.MkdirAll(dir, 0o700); err != nil {
					return "", err
				}
				path := filepath.Join(dir, name+".cred")
				if err := os.WriteFile(path, []byte("encrypted-test-payload"), 0o600); err != nil {
					return "", err
				}
				credentials[credentialKey(dir, name)] = credential
				return path, nil
			},
			readGatewayCredential: func(dir string, name string) (string, error) {
				credentialsMu.Lock()
				defer credentialsMu.Unlock()
				credential, exists := credentials[credentialKey(dir, name)]
				if !exists {
					return "", os.ErrNotExist
				}
				return credential, nil
			},
			removeGatewayCredential: func(dir string, name string) error {
				credentialsMu.Lock()
				defer credentialsMu.Unlock()
				delete(credentials, credentialKey(dir, name))
				err := os.Remove(filepath.Join(dir, name+".cred"))
				if errors.Is(err, os.ErrNotExist) {
					return nil
				}
				return err
			},
		}
		deps.newClient = redirectingGatewayClientFactory(t, server)
		deps = deps.withDefaults()

		address := "https://gateway.example:443"
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"pair", "redeem", "pairing-once", "--name", "laptop", "--address", address, "--use", "-o", "json",
		)
		if err != nil {
			t.Fatalf("pair redeem error = %v", err)
		}
		if strings.Contains(stdout, gatewayDeviceCredentialPrefix) {
			t.Fatalf("pair output leaked credential: %s", stdout)
		}
		cfg, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		profile, exists := findGatewayProfile(cfg.Gateway.Connections, "laptop")
		if !exists || cfg.Gateway.ActiveConnection != "laptop" || profile.CredentialFile != "laptop.cred" {
			t.Fatalf("paired profile = %#v active=%q [IT-060]", profile, cfg.Gateway.ActiveConnection)
		}
		credentialPath := filepath.Join(homePaths.GatewayCredentialsDir, "laptop.cred")
		credentialInfo, err := os.Stat(credentialPath)
		if err != nil || credentialInfo.Mode().Perm() != 0o600 || state.deviceCount() != 1 {
			t.Fatalf(
				"paired credential/device = info:%v err:%v devices:%d [IT-060]",
				credentialInfo,
				err,
				state.deviceCount(),
			)
		}

		passphrase := filepath.Join(t.TempDir(), "profile.passphrase")
		if err := os.WriteFile(passphrase, []byte("portable profile passphrase\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(passphrase) error = %v", err)
		}
		bundlePath := filepath.Join(t.TempDir(), "laptop.cpzprofile")
		exportOut, _, err := executeRootCommand(
			t, deps,
			"connect", "export", "laptop",
			"--passphrase-file", passphrase,
			"--output-file", bundlePath,
			"-o", "json",
		)
		if err != nil || !json.Valid([]byte(exportOut)) || strings.Contains(exportOut, state.credential) {
			t.Fatalf("connect export = stdout:%q err:%v, want redacted structured output", exportOut, err)
		}
		bundlePayload, err := os.ReadFile(bundlePath)
		if err != nil {
			t.Fatalf("os.ReadFile(bundle) error = %v", err)
		}
		if bytes.Contains(bundlePayload, []byte(state.credential)) {
			t.Fatal("portable profile bundle contains the raw device credential")
		}

		copyHomePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom(copy) error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(copyHomePaths); err != nil {
			t.Fatalf("EnsureHomeLayout(copy) error = %v", err)
		}
		copiedBundlePath := filepath.Join(copyHomePaths.HomeDir, "laptop.cpzprofile")
		if err := os.WriteFile(copiedBundlePath, bundlePayload, 0o600); err != nil {
			t.Fatalf("os.WriteFile(copied bundle) error = %v", err)
		}
		copyPassphrase := filepath.Join(copyHomePaths.HomeDir, "profile.passphrase")
		if err := os.WriteFile(copyPassphrase, []byte("portable profile passphrase\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(copy passphrase) error = %v", err)
		}
		copyDeps := deps
		copyDeps.resolveHome = func() (compozyconfig.HomePaths, error) { return copyHomePaths, nil }
		copyDeps.loadConfig = func() (compozyconfig.Config, error) { return compozyconfig.LoadForHome(copyHomePaths) }
		importOut, _, err := executeRootCommand(
			t, copyDeps,
			"connect", "import", copiedBundlePath,
			"--passphrase-file", copyPassphrase,
			"--use",
			"-o", "json",
		)
		if err != nil || !json.Valid([]byte(importOut)) || strings.Contains(importOut, state.credential) {
			t.Fatalf("connect import = stdout:%q err:%v, want redacted structured output", importOut, err)
		}
		copyConfig, err := compozyconfig.LoadForHome(copyHomePaths)
		if err != nil {
			t.Fatalf("LoadForHome(copy) error = %v", err)
		}
		copyTarget, err := resolveConfiguredClientTarget(
			copyHomePaths, &copyConfig, copyDeps.readGatewayCredential,
		)
		if err != nil {
			t.Fatalf("resolveConfiguredClientTarget(copy) error = %v", err)
		}
		if copyTarget.credential != state.credential || copyTarget.name != "laptop" {
			t.Fatalf("copied target = %#v, want same device credential [IT-061]", copyTarget)
		}

		remoteClient, err := deps.newClient(mustGatewayTarget(t, "laptop", state.credential))
		if err != nil {
			t.Fatalf("new remote client error = %v", err)
		}
		localClient := &daemonClient{
			target:       LocalClientTarget(homePaths.DaemonSocket),
			httpClient:   &http.Client{Transport: redirectRoundTripper(server)},
			streamClient: &http.Client{Transport: redirectRoundTripper(server)},
		}
		remoteGateway := remoteClient.(gatewayClientAPI)
		localGateway := any(localClient).(gatewayClientAPI)
		localStatus, err := localGateway.GetGatewayStatus(t.Context())
		if err != nil {
			t.Fatalf("local GetGatewayStatus() error = %v", err)
		}
		remoteStatus, err := remoteGateway.GetGatewayStatus(t.Context())
		if err != nil || remoteStatus.Enabled != localStatus.Enabled {
			t.Fatalf("remote/local status = %#v/%#v err=%v [IT-062]", remoteStatus, localStatus, err)
		}
		remoteConcrete := remoteClient.(*daemonClient)
		var remoteEvents atomic.Int32
		if err := remoteConcrete.doSSE(t.Context(), "/api/logs/stream", nil, "", func(SSEEvent) error {
			remoteEvents.Add(1)
			return nil
		}); err != nil {
			t.Fatalf("remote stream error = %v", err)
		}
		var localEvents atomic.Int32
		if err := localClient.doSSE(t.Context(), "/api/logs/stream", nil, "", func(SSEEvent) error {
			localEvents.Add(1)
			return nil
		}); err != nil {
			t.Fatalf("local stream error = %v", err)
		}
		if remoteEvents.Load() != localEvents.Load() || remoteEvents.Load() != 1 {
			t.Fatalf("remote/local stream events = %d/%d [IT-062]", remoteEvents.Load(), localEvents.Load())
		}
		var reconnectEvents atomic.Int32
		if err := remoteConcrete.doSSE(t.Context(), "/api/logs/stream", nil, "", func(SSEEvent) error {
			reconnectEvents.Add(1)
			return nil
		}); err != nil {
			t.Fatalf("remote reconnect stream error = %v", err)
		}
		if reconnectEvents.Load() != 1 || state.ticketCount() != 2 {
			t.Fatalf("remote reconnect events/tickets = %d/%d, want 1/2", reconnectEvents.Load(), state.ticketCount())
		}

		secondClient, err := copyDeps.newClient(copyTarget)
		if err != nil {
			t.Fatalf("new second client error = %v", err)
		}
		_, err = remoteGateway.SetGatewaySurface(t.Context(), contract.GatewaySurfaceRequest{
			Surface: "operator_ui", Tier: "private", Desired: "enabled",
		})
		if err != nil {
			t.Fatalf("SetGatewaySurface() error = %v", err)
		}
		consistent, err := secondClient.(gatewayClientAPI).GetGatewayStatus(t.Context())
		if err != nil || len(consistent.Surfaces) != 1 || consistent.Surfaces[0].Desired != "enabled" {
			t.Fatalf("second-client status = %#v, %v [IT-065]", consistent, err)
		}
		statusOut, targetStderr, err := executeRootCommand(t, deps, "gateway", "status", "-o", "json")
		if err != nil {
			t.Fatalf("gateway status command error = %v", err)
		}
		if !strings.Contains(targetStderr, "target: laptop (https://gateway.example:443)") {
			t.Fatalf("remote target indication = %q, want active profile and origin", targetStderr)
		}
		auditOut, auditStderr, err := executeRootCommand(t, deps, "gateway", "audit", "-o", "json")
		if err != nil {
			t.Fatalf("gateway audit command error = %v; stderr=%s", err, auditStderr)
		}
		for label, output := range map[string]string{
			"gateway status CLI": statusOut + targetStderr,
			"gateway audit CLI":  auditOut + auditStderr,
		} {
			if strings.Contains(output, state.credential) || strings.Contains(output, "compozy_claim_") ||
				strings.Contains(output, "cpz_gwp_") || strings.Contains(output, "cpz_gwt_") {
				t.Fatalf("[IT-080][IT-081] %s leaked gateway secret bytes: %s", label, output)
			}
		}

		if err := remoteConcrete.doJSON(
			t.Context(), http.MethodPost, "/api/sessions/session-1/prompt", nil, struct{}{}, &struct{}{},
		); err != nil {
			t.Fatalf("remote work start error = %v", err)
		}
		reconnected, err := deps.newClient(mustGatewayTarget(t, "reconnected", state.credential))
		if err != nil {
			t.Fatalf("new reconnected client error = %v", err)
		}
		var work struct {
			State string `json:"state"`
		}
		if err := reconnected.(*daemonClient).doJSON(
			t.Context(), http.MethodGet, "/api/sessions/session-1", nil, nil, &work,
		); err != nil || work.State != "running" {
			t.Fatalf("reconnected work = %#v, %v [IT-066]", work, err)
		}
		localOnlyErr := reconnected.(*daemonClient).doJSON(
			t.Context(), http.MethodPost, "/api/agent/tasks/claim-next", nil, struct{}{}, nil,
		)
		assertGatewayClientErrorCode(t, localOnlyErr, "gateway_local_only_operation")

		_, err = remoteGateway.RevokeGatewayDevice(t.Context(), state.deviceID)
		if err != nil {
			t.Fatalf("RevokeGatewayDevice() error = %v", err)
		}
		for label, client := range map[string]gatewayClientAPI{
			"original": remoteGateway,
			"copied":   secondClient.(gatewayClientAPI),
		} {
			_, err := client.GetGatewayStatus(t.Context())
			var apiErr *daemonAPIError
			if !errors.As(err, &apiErr) || apiErr.payload.Code != "gateway_device_unauthenticated" {
				t.Fatalf("%s copied-identity error = %T %v [IT-061]", label, err, err)
			}
		}
		if _, _, err := executeRootCommand(t, deps, "connect", "remove", "laptop", "-o", "json"); err != nil {
			t.Fatalf("connect remove error = %v", err)
		}
		removedConfig, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after remove) error = %v", err)
		}
		if _, exists := findGatewayProfile(removedConfig.Gateway.Connections, "laptop"); exists ||
			removedConfig.Gateway.ActiveConnection != "" {
			t.Fatalf("removed profile remains in config: %#v", removedConfig.Gateway)
		}
		if _, err := os.Stat(credentialPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("credential file after remove error = %v, want not-exist", err)
		}
	})
}

type remoteCLIIntegrationState struct {
	mu         sync.Mutex
	credential string
	deviceID   string
	revoked    bool
	paired     bool
	surfaceOn  bool
	workState  string
	tickets    map[string]struct{}
	nextTicket int
}

func newRemoteCLIIntegrationState() *remoteCLIIntegrationState {
	return &remoteCLIIntegrationState{
		credential: testGatewayCredential('i'),
		deviceID:   "dev_cli_integration",
		tickets:    make(map[string]struct{}),
	}
}

func (s *remoteCLIIntegrationState) deviceCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paired {
		return 1
	}
	return 0
}

func (s *remoteCLIIntegrationState) ticketCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nextTicket
}

func (s *remoteCLIIntegrationState) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writer.Header().Set("Content-Type", "application/json")
	if request.URL.Path == "/api/gateway/pairings/redeem" {
		if s.paired {
			writeRemoteCLIJSON(
				writer,
				http.StatusGone,
				contract.ErrorPayload{Error: "spent", Code: "gateway_pairing_spent"},
			)
			return
		}
		var redeem contract.GatewayPairingRedeemRequest
		if err := json.NewDecoder(request.Body).Decode(&redeem); err != nil ||
			!validGatewayDeviceCredential(redeem.Credential) {
			writeRemoteCLIJSON(writer, http.StatusBadRequest, contract.ErrorPayload{
				Error: "invalid", Code: "gateway_invalid_request",
			})
			return
		}
		s.credential = redeem.Credential
		s.paired = true
		writeRemoteCLIJSON(writer, http.StatusOK, contract.GatewayIssuedCredentialPayload{
			Device:     contract.GatewayDevicePayload{ID: s.deviceID, Name: "laptop", ActorKind: "cli_profile"},
			Credential: s.credential,
		})
		return
	}
	local := request.Header.Get("X-Compozy-Test-Local") == "true"
	if !local && (s.revoked || request.Header.Get("Authorization") != "Bearer "+s.credential) {
		writeRemoteCLIJSON(writer, http.StatusUnauthorized, contract.ErrorPayload{
			Error: "Unauthorized", Code: "gateway_device_unauthenticated",
		})
		return
	}
	s.serveAuthenticatedHTTP(writer, request, local)
}

func (s *remoteCLIIntegrationState) serveAuthenticatedHTTP(
	writer http.ResponseWriter,
	request *http.Request,
	local bool,
) {
	switch {
	case request.URL.Path == "/api/gateway/stream-tickets" && request.Method == http.MethodPost:
		s.nextTicket++
		ticket := fmt.Sprintf("ticket-%d", s.nextTicket)
		s.tickets[ticket] = struct{}{}
		writeRemoteCLIJSON(writer, http.StatusCreated, contract.GatewayStreamTicketPayload{Ticket: ticket})
	case request.URL.Path == "/api/logs/stream" && request.Method == http.MethodGet:
		if !local {
			ticket := request.URL.Query().Get("ticket")
			if _, exists := s.tickets[ticket]; !exists {
				writeRemoteCLIJSON(writer, http.StatusUnauthorized, contract.ErrorPayload{Error: "bad ticket"})
				return
			}
			delete(s.tickets, ticket)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(writer, "event: log\ndata: {\"message\":\"ready\"}\n\n"); err != nil {
			return
		}
	case request.URL.Path == "/api/gateway/status" && request.Method == http.MethodGet:
		status := contract.GatewayStatusPayload{Enabled: true}
		if s.surfaceOn {
			status.Surfaces = []contract.GatewaySurfacePayload{{
				Surface: "operator_ui", Tier: "private", Desired: "enabled", Observed: "on",
			}}
		}
		writeRemoteCLIJSON(writer, http.StatusOK, status)
	case request.URL.Path == "/api/gateway/audit" && request.Method == http.MethodGet:
		writeRemoteCLIJSON(writer, http.StatusOK, contract.GatewayAuditPayload{
			Ran: true, NoFindings: true, LocalOnly: true,
			Status: contract.GatewayStatusPayload{Enabled: true},
		})
	case request.URL.Path == "/api/gateway/surfaces" && request.Method == http.MethodPost:
		s.surfaceOn = true
		writeRemoteCLIJSON(writer, http.StatusOK, contract.GatewayStatusPayload{
			Enabled: true,
			Surfaces: []contract.GatewaySurfacePayload{{
				Surface: "operator_ui", Tier: "private", Desired: "enabled", Observed: "on",
			}},
		})
	case request.URL.Path == "/api/gateway/devices/"+s.deviceID && request.Method == http.MethodDelete:
		s.revoked = true
		writeRemoteCLIJSON(writer, http.StatusOK, contract.GatewayRevokePayload{
			Device:  contract.GatewayDevicePayload{ID: s.deviceID, Name: "laptop", ActorKind: "cli_profile"},
			Changed: true,
		})
	case request.URL.Path == "/api/sessions/session-1/prompt" && request.Method == http.MethodPost:
		s.workState = "running"
		writeRemoteCLIJSON(writer, http.StatusOK, struct{}{})
	case request.URL.Path == "/api/sessions/session-1" && request.Method == http.MethodGet:
		writeRemoteCLIJSON(writer, http.StatusOK, map[string]string{"state": s.workState})
	default:
		writeRemoteCLIJSON(writer, http.StatusNotFound, contract.ErrorPayload{Error: "not found"})
	}
}

func writeRemoteCLIJSON(writer http.ResponseWriter, status int, value any) {
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		return
	}
}

func redirectingGatewayClientFactory(
	t *testing.T,
	server *httptest.Server,
) func(ClientTarget) (DaemonClient, error) {
	t.Helper()
	return func(target ClientTarget) (DaemonClient, error) {
		client, err := NewClient(target)
		if err != nil {
			return nil, err
		}
		concrete := client.(*daemonClient)
		concrete.httpClient.Transport = redirectRoundTripper(server)
		concrete.streamClient.Transport = redirectRoundTripper(server)
		return concrete, nil
	}
}

func redirectRoundTripper(server *httptest.Server) http.RoundTripper {
	base := server.Client().Transport
	return roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		redirected := request.Clone(request.Context())
		if request.URL.Host == "unix" {
			redirected.Header.Set("X-Compozy-Test-Local", "true")
		}
		redirected.URL.Scheme = "https"
		redirected.URL.Host = strings.TrimPrefix(server.URL, "https://")
		return base.RoundTrip(redirected)
	})
}

func mustGatewayTarget(t *testing.T, name string, credential string) ClientTarget {
	t.Helper()
	target, err := GatewayClientTarget(name, "gateway.example", 443, credential)
	if err != nil {
		t.Fatalf("GatewayClientTarget() error = %v", err)
	}
	return target
}

func TestConnectSSHIntegrationIT070ThroughIT072(t *testing.T) {
	t.Run("Should own one loopback-only SSH forward and preserve reused daemons [IT-070..072]", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		executor := newSSHIntegrationExecutor()
		forwardReady := make(chan struct{}, 3)
		deps := commandDeps{
			resolveHome:    func() (compozyconfig.HomePaths, error) { return homePaths, nil },
			loadConfig:     func() (compozyconfig.Config, error) { return compozyconfig.LoadForHome(homePaths) },
			newSSHExecutor: func() sshExecutor { return executor },
			newClient: func(target ClientTarget) (DaemonClient, error) {
				if target.kind != clientTargetSSHForward || !strings.HasPrefix(target.baseURL, "http://127.0.0.1:") {
					return nil, fmt.Errorf("unexpected SSH forward target %#v", target)
				}
				forwardReady <- struct{}{}
				return &stubClient{daemonStatusFn: func(context.Context) (DaemonStatus, error) {
					return DaemonStatus{Status: "running", HTTPHost: "127.0.0.1", HTTPPort: 2123}, nil
				}}, nil
			},
			startTimeout: 500 * time.Millisecond,
			stopTimeout:  500 * time.Millisecond,
			pollInterval: time.Millisecond,
		}
		deps = deps.withDefaults()

		firstCtx, cancelFirst := context.WithCancel(t.Context())
		firstResult := executeSSHCommandAsync(
			deps,
			firstCtx,
			"connect",
			"ssh",
			"remote.example",
			"--name",
			"ssh-lab",
			"-o",
			"json",
		)
		firstTunnel := <-executor.tunnels
		<-forwardReady
		busyResult := executeSSHCommandAsync(
			deps,
			t.Context(),
			"connect", "ssh", "remote.example", "--name", "ssh-lab", "--overwrite", "-o", "json",
		)
		busy := <-busyResult
		assertGatewayClientErrorCode(t, busy.err, sshBusyCode)
		if executor.startCount.Load() != 1 {
			t.Fatalf("concurrent SSH starts = %d, want one [IT-071]", executor.startCount.Load())
		}
		cancelFirst()
		first := <-firstResult
		if first.err != nil || !json.Valid([]byte(first.stdout)) {
			t.Fatalf("first SSH connect = stdout:%q err:%v [IT-070]", first.stdout, first.err)
		}
		if !firstTunnel.closed() || executor.startCount.Load() != 1 || executor.stopCount.Load() != 1 {
			t.Fatalf(
				"first SSH lifecycle = tunnel_closed:%t starts:%d stops:%d [IT-070]",
				firstTunnel.closed(), executor.startCount.Load(), executor.stopCount.Load(),
			)
		}
		profileConfig, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if _, exists := findGatewayProfile(profileConfig.Gateway.Connections, "ssh-lab"); !exists {
			t.Fatal("SSH profile was not recorded [IT-070]")
		}

		executor.setRunning(true)
		secondCtx, cancelSecond := context.WithCancel(t.Context())
		defer cancelSecond()
		secondResult := executeSSHCommandAsync(
			deps,
			secondCtx,
			"connect", "ssh", "remote.example", "--name", "ssh-lab", "--overwrite", "-o", "json",
		)
		secondTunnel := <-executor.tunnels
		<-forwardReady
		secondTunnel.drop(errors.New("simulated SSH transport loss"))
		second := <-secondResult
		assertGatewayClientErrorCode(t, second.err, sshTunnelLostCode)
		if executor.startCount.Load() != 1 || executor.stopCount.Load() != 1 || !executor.isRunning() {
			t.Fatalf(
				"reused daemon lifecycle = starts:%d stops:%d running:%t [IT-071]",
				executor.startCount.Load(), executor.stopCount.Load(), executor.isRunning(),
			)
		}
		if executor.lastRemoteHost() != "127.0.0.1" || executor.gatewayMutations.Load() != 0 {
			t.Fatalf(
				"SSH exposure = remote_host:%q gateway_mutations:%d [IT-072]",
				executor.lastRemoteHost(), executor.gatewayMutations.Load(),
			)
		}

		executor.setRunning(false)
		thirdCtx, cancelThird := context.WithCancel(t.Context())
		defer cancelThird()
		thirdResult := executeSSHCommandAsync(
			deps,
			thirdCtx,
			"connect", "ssh", "remote.example", "--name", "ssh-lab", "--overwrite", "-o", "json",
		)
		thirdTunnel := <-executor.tunnels
		<-forwardReady
		thirdTunnel.drop(errors.New("simulated SSH transport loss after daemon start"))
		third := <-thirdResult
		assertGatewayClientErrorCode(t, third.err, sshTunnelLostCode)
		if executor.startCount.Load() != 2 || executor.stopCount.Load() != 1 || !executor.isRunning() {
			t.Fatalf(
				"dropped owned daemon lifecycle = starts:%d stops:%d running:%t [IT-071]",
				executor.startCount.Load(), executor.stopCount.Load(), executor.isRunning(),
			)
		}
	})
}

type sshCommandResult struct {
	stdout string
	err    error
}

func executeSSHCommandAsync(deps commandDeps, ctx context.Context, args ...string) <-chan sshCommandResult {
	result := make(chan sshCommandResult, 1)
	go func() {
		command := newRootCommand(deps)
		var stdout bytes.Buffer
		command.SetOut(&stdout)
		command.SetErr(io.Discard)
		command.SetArgs(args)
		err := command.ExecuteContext(ctx)
		result <- sshCommandResult{stdout: stdout.String(), err: err}
	}()
	return result
}

type sshIntegrationExecutor struct {
	mu                sync.Mutex
	running           bool
	remoteForwardHost string
	tunnels           chan *sshIntegrationTunnel
	startCount        atomic.Int32
	stopCount         atomic.Int32
	gatewayMutations  atomic.Int32
	startedAt         time.Time
	ownerToken        string
}

func newSSHIntegrationExecutor() *sshIntegrationExecutor {
	return &sshIntegrationExecutor{
		tunnels:   make(chan *sshIntegrationTunnel, 3),
		startedAt: time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC),
	}
}

func (e *sshIntegrationExecutor) Run(
	_ context.Context,
	_ sshTarget,
	command []string,
) ([]byte, error) {
	if len(command) >= 3 {
		switch command[2] {
		case sshOwnershipInspectScript:
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.ownerToken == "" {
				return []byte(sshOwnershipAbsent), nil
			}
			return []byte(sshOwnershipMarkerState), nil
		case sshOwnershipStopScript:
			if len(command) < 6 {
				return nil, errors.New("missing SSH ownership token")
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.ownerToken != command[5] {
				return []byte(sshOwnershipNotOwner), nil
			}
			e.running = false
			e.stopCount.Add(1)
			return json.Marshal(DaemonStatus{Status: "stopped"})
		}
	}
	joined := strings.Join(command, " ")
	switch {
	case strings.Contains(joined, "command -v compozy"):
		return []byte("/usr/local/bin/compozy\n"), nil
	case strings.Contains(joined, " version ") || strings.HasSuffix(joined, " version -o json"):
		return json.Marshal(version.Current())
	case strings.Contains(joined, "gateway"):
		e.gatewayMutations.Add(1)
		return nil, errors.New("unexpected gateway mutation")
	case strings.HasSuffix(joined, "compozy status -o json"):
		if !e.isRunning() {
			output, err := json.Marshal(contract.ErrorPayload{
				Error: "daemon unavailable",
				Diagnostic: &contract.DiagnosticItem{
					Code: contract.CodeDaemonUnavailable,
				},
			})
			if err != nil {
				return nil, fmt.Errorf("encode daemon-unavailable SSH fixture: %w", err)
			}
			return output, &sshCommandError{cause: errors.New("exit status 1"), output: string(output)}
		}
		return json.Marshal(StatusRecord{Daemon: DaemonStatus{
			Status: "running", PID: 4101, StartedAt: e.startedAt,
			HTTPHost: "127.0.0.1", HTTPPort: 2123,
		}})
	case strings.Contains(joined, "daemon start"):
		e.setRunning(true)
		e.startCount.Add(1)
		return json.Marshal(DaemonStatus{
			Status: "running", PID: 4101, StartedAt: e.startedAt,
			HTTPHost: "127.0.0.1", HTTPPort: 2123,
		})
	case strings.Contains(joined, "daemon stop"):
		e.setRunning(false)
		e.stopCount.Add(1)
		return json.Marshal(DaemonStatus{Status: "stopped"})
	default:
		return nil, fmt.Errorf("unexpected SSH command %#v", command)
	}
}

func (e *sshIntegrationExecutor) StartOwnershipLease(
	_ context.Context,
	_ sshTarget,
	_ string,
	ownership *sshDaemonOwnership,
) (sshOwnershipLease, error) {
	if ownership == nil || strings.TrimSpace(ownership.token) == "" {
		return nil, errors.New("missing SSH ownership token")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ownerToken != "" {
		return nil, newSSHBusyError()
	}
	e.ownerToken = ownership.token
	return &sshIntegrationOwnershipLease{executor: e, token: ownership.token, done: make(chan struct{})}, nil
}

func (e *sshIntegrationExecutor) StartTunnel(
	_ context.Context,
	_ sshTarget,
	remoteHost string,
	remotePort int,
) (sshTunnel, error) {
	if remotePort != 2123 {
		return nil, fmt.Errorf("remote port = %d, want 2123", remotePort)
	}
	e.mu.Lock()
	e.remoteForwardHost = remoteHost
	e.mu.Unlock()
	tunnel := &sshIntegrationTunnel{localPort: 43123, done: make(chan struct{})}
	e.tunnels <- tunnel
	return tunnel, nil
}

func (e *sshIntegrationExecutor) setRunning(running bool) {
	e.mu.Lock()
	e.running = running
	e.mu.Unlock()
}

func (e *sshIntegrationExecutor) isRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

func (e *sshIntegrationExecutor) lastRemoteHost() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.remoteForwardHost
}

type sshIntegrationTunnel struct {
	localPort int
	done      chan struct{}
	once      sync.Once
	err       error
}

type sshIntegrationOwnershipLease struct {
	executor *sshIntegrationExecutor
	token    string
	done     chan struct{}
	once     sync.Once
}

func (l *sshIntegrationOwnershipLease) Wait() error {
	<-l.done
	return nil
}

func (l *sshIntegrationOwnershipLease) Close() error {
	l.once.Do(func() {
		l.executor.mu.Lock()
		if l.executor.ownerToken == l.token {
			l.executor.ownerToken = ""
		}
		l.executor.mu.Unlock()
		close(l.done)
	})
	return nil
}

func (t *sshIntegrationTunnel) LocalPort() int { return t.localPort }

func (t *sshIntegrationTunnel) Wait() error {
	<-t.done
	return t.err
}

func (t *sshIntegrationTunnel) Close() error {
	t.once.Do(func() { close(t.done) })
	return nil
}

func (t *sshIntegrationTunnel) drop(err error) {
	t.once.Do(func() {
		t.err = err
		close(t.done)
	})
}

func (t *sshIntegrationTunnel) closed() bool {
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}

func TestIntegrationHarnessStopAndWaitIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve a daemon-stop error after forcing the daemon to exit", func(t *testing.T) {
		t.Parallel()

		h := newIntegrationHarness(t)
		mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")

		stopErr := errors.New("injected daemon stop failure")
		h.deps.stopTimeout = 250 * time.Millisecond
		h.deps.signalProcess = func(int, syscall.Signal) error {
			return stopErr
		}

		err := h.stopAndWait(t)
		if !errors.Is(err, stopErr) {
			t.Fatalf("stopAndWait() error = %v, want injected stop error", err)
		}
		if h.runner.processAlive(h.runner.pid) {
			t.Fatal("integration daemon remained running after failed daemon stop")
		}
	})

	t.Run("Should stop an unpublished daemon through harness ownership", func(t *testing.T) {
		t.Parallel()

		h := newIntegrationHarness(t)
		mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
		if err := os.Remove(h.homePaths.DaemonInfo); err != nil {
			t.Fatalf("os.Remove(daemon info) error = %v", err)
		}

		if err := h.stopAndWait(t); err != nil {
			t.Fatalf("stopAndWait() error = %v, want nil for an unpublished daemon", err)
		}
		if h.runner.processAlive(h.runner.pid) {
			t.Fatal("integration daemon remained running after unpublished cleanup")
		}
	})
}

func TestSessionListOutputFormatsIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")

	sessionOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"demo",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new error = %v", err)
	}
	var created SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}

	humanOut, _, err := executeRootCommand(t, h.deps, "session", "list", "--all", "-o", "human")
	if err != nil {
		t.Fatalf("session list human error = %v", err)
	}
	if !strings.Contains(humanOut, "Sessions") || !strings.Contains(humanOut, created.ID) ||
		!strings.Contains(humanOut, "Page") || !strings.Contains(humanOut, "Has More") {
		t.Fatalf("human output = %q, want session table and page metadata", humanOut)
	}

	jsonOut, _, err := executeRootCommand(t, h.deps, "session", "list", "--all", "-o", "json")
	if err != nil {
		t.Fatalf("session list json error = %v", err)
	}
	var listed SessionListPage
	if err := json.Unmarshal([]byte(jsonOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(session list) error = %v", err)
	}
	if len(listed.Sessions) != 1 || listed.Sessions[0].ID != created.ID {
		t.Fatalf("listed = %#v, want one created session", listed)
	}

	toonOut, _, err := executeRootCommand(t, h.deps, "session", "list", "--all", "-o", "toon")
	if err != nil {
		t.Fatalf("session list toon error = %v", err)
	}
	if !strings.Contains(
		toonOut,
		"sessions[1]{id,name,agent_name,parent_session_id,provider,sandbox_backend,state,badge,failure_kind,workspace,channel,health_state,health,updated_at}:",
	) || !strings.Contains(toonOut, "page{") || !strings.Contains(toonOut, "has_more") {
		t.Fatalf("toon output = %q, want TOON table and page metadata", toonOut)
	}
}

func TestCLISessionChannelRoundTripIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	newOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"demo",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		"builders",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new --channel error = %v", err)
	}
	var created SessionRecord
	if err := json.Unmarshal([]byte(newOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new --channel) error = %v", err)
	}
	if resolvedParticipationChannelID(created.ResolvedNetworkParticipation) != "builders" {
		t.Fatalf(
			"created.ResolvedNetworkParticipation = %#v, want channel %q",
			created.ResolvedNetworkParticipation,
			"builders",
		)
	}

	listOut, _, err := executeRootCommand(t, h.deps, "session", "list", "--all", "-o", "json")
	if err != nil {
		t.Fatalf("session list error = %v", err)
	}
	var listed SessionListPage
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(session list) error = %v", err)
	}
	if got, want := len(listed.Sessions), 1; got != want {
		t.Fatalf("len(listed) = %d, want %d", got, want)
	}
	if resolvedParticipationChannelID(listed.Sessions[0].ResolvedNetworkParticipation) != "builders" {
		t.Fatalf(
			"listed[0].ResolvedNetworkParticipation = %#v, want channel %q",
			listed.Sessions[0].ResolvedNetworkParticipation,
			"builders",
		)
	}

	stopOut, _, err := executeRootCommand(t, h.deps, "session", "stop", created.ID, "-o", "json")
	if err != nil {
		t.Fatalf("session stop error = %v", err)
	}
	var stopped SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stopped); err != nil {
		t.Fatalf("json.Unmarshal(session stop) error = %v", err)
	}
	if resolvedParticipationChannelID(stopped.ResolvedNetworkParticipation) != "builders" ||
		stopped.State != session.StateStopped {
		t.Fatalf("stopped = %#v, want stopped builders session", stopped)
	}

	if _, _, err := executeRootCommand(t, h.deps, "session", "resume", created.ID, "-o", "json"); err == nil {
		t.Fatal("session resume stopped error = nil, want attach rejection")
	} else if !strings.Contains(err.Error(), "session not attachable") {
		t.Fatalf("session resume stopped error = %v, want not attachable", err)
	}
}

func TestCLISessionRemoveAndWorkspaceRemoveIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should delete session artifacts through daemon routes", func(t *testing.T) {
		t.Parallel()

		h := newIntegrationHarness(t)
		mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")

		workspaceOut := mustExecuteRoot(t, h.deps, "workspace", "add", h.workspace, "--name", "alpha", "-o", "json")
		var registered WorkspaceRecord
		if err := json.Unmarshal([]byte(workspaceOut), &registered); err != nil {
			t.Fatalf("json.Unmarshal(workspace add) error = %v", err)
		}
		if registered.ID == "" {
			t.Fatal("workspace add returned empty id")
		}

		active := createIntegrationSession(t, h, "delete-active", "alpha")
		activeDir := filepath.Join(h.homePaths.SessionsDir, active.ID)
		assertPathExists(t, store.SessionDBFile(activeDir))
		removeOut := mustExecuteRoot(t, h.deps, "session", "remove", active.ID, "-o", "json")
		var removed SessionRecord
		if err := json.Unmarshal([]byte(removeOut), &removed); err != nil {
			t.Fatalf("json.Unmarshal(session remove) error = %v", err)
		}
		if removed.ID != active.ID {
			t.Fatalf("removed.ID = %q, want %q", removed.ID, active.ID)
		}
		assertPathMissing(t, activeDir)

		cascade := createIntegrationSession(t, h, "workspace-cascade", "alpha")
		cascadeStopOut := mustExecuteRoot(t, h.deps, "session", "stop", cascade.ID, "-o", "json")
		var stoppedCascade SessionRecord
		if err := json.Unmarshal([]byte(cascadeStopOut), &stoppedCascade); err != nil {
			t.Fatalf("json.Unmarshal(session stop) error = %v", err)
		}
		if stoppedCascade.State != session.StateStopped {
			t.Fatalf("stoppedCascade.State = %q, want %q", stoppedCascade.State, session.StateStopped)
		}
		cascadeDir := filepath.Join(h.homePaths.SessionsDir, cascade.ID)
		assertPathExists(t, store.SessionDBFile(cascadeDir))

		workspaceRemoveOut := mustExecuteRoot(t, h.deps, "workspace", "remove", "alpha", "-o", "json")
		var removedWorkspace WorkspaceRecord
		if err := json.Unmarshal([]byte(workspaceRemoveOut), &removedWorkspace); err != nil {
			t.Fatalf("json.Unmarshal(workspace remove) error = %v", err)
		}
		if removedWorkspace.ID != registered.ID {
			t.Fatalf("removedWorkspace.ID = %q, want %q", removedWorkspace.ID, registered.ID)
		}
		assertPathMissing(t, cascadeDir)

		exitCode, _, stderr := executeRootCommandWithExit(t, h.deps, "session", "status", active.ID, "-o", "json")
		if exitCode == 0 {
			t.Fatalf("session status after remove exit code = 0, want failure; stderr=%s", stderr)
		}
	})
}

func createIntegrationSession(t *testing.T, h integrationHarness, name string, workspace string) SessionRecord {
	t.Helper()

	out := mustExecuteRoot(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		name,
		"--workspace",
		workspace,
		"-o",
		"json",
	)
	var created SessionRecord
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if created.ID == "" || created.State != session.StateActive {
		t.Fatalf("created session = %#v, want active session with id", created)
	}
	return created
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat(%q) error = %v, want existing path", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		entries, readErr := os.ReadDir(path)
		entryNames := make([]string, 0, len(entries))
		for _, entry := range entries {
			entryNames = append(entryNames, entry.Name())
		}
		meta, metaErr := store.ReadSessionMeta(store.SessionMetaFile(path))
		t.Fatalf(
			"Stat(%q) error = %v, want os.ErrNotExist; entries=%v read_error=%v meta=%#v meta_error=%v",
			path,
			err,
			entryNames,
			readErr,
			meta,
			metaErr,
		)
	}
}

func TestCLISessionProviderOverrideIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	h.runner.cfg.Providers["fake-alt"] = compozyconfig.ProviderConfig{Command: "fake-alt-agent"}

	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	newOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"provider-demo",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new --provider error = %v", err)
	}

	var created SessionRecord
	if err := json.Unmarshal([]byte(newOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new --provider) error = %v", err)
	}
	if created.Runtime.Effective != nil {
		t.Fatalf("created runtime = %#v, want unbound session", created.Runtime)
	}
	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"prompt",
		created.ID,
		"use the alternate provider",
		"--provider",
		"fake-alt",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("session prompt --provider error = %v", err)
	}

	statusOut, _, err := executeRootCommand(t, h.deps, "session", "status", created.ID, "-o", "json")
	if err != nil {
		t.Fatalf("session status error = %v", err)
	}

	var status SessionStatusRecord
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("json.Unmarshal(session status) error = %v", err)
	}
	if status.SessionID != created.ID || status.AgentName != "coder" {
		t.Fatalf("status = %#v, want coder session health status", status)
	}

	listOut, _, err := executeRootCommand(t, h.deps, "session", "list", "--all", "-o", "json")
	if err != nil {
		t.Fatalf("session list error = %v", err)
	}

	var listed SessionListPage
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(session list) error = %v", err)
	}
	if got, want := len(listed.Sessions), 1; got != want {
		t.Fatalf("len(listed) = %d, want %d", got, want)
	}
	if sessionRuntimeProvider(listed.Sessions[0]) != "fake-alt" {
		t.Fatalf("listed runtime provider = %q, want fake-alt", sessionRuntimeProvider(listed.Sessions[0]))
	}

	stopOut, _, err := executeRootCommand(t, h.deps, "session", "stop", created.ID, "-o", "json")
	if err != nil {
		t.Fatalf("session stop error = %v", err)
	}

	var stopped SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stopped); err != nil {
		t.Fatalf("json.Unmarshal(session stop) error = %v", err)
	}
	if sessionRuntimeProvider(stopped) != "fake-alt" || stopped.State != session.StateStopped {
		t.Fatalf("stopped = %#v, want stopped fake-alt session", stopped)
	}

	if _, _, err := executeRootCommand(t, h.deps, "session", "resume", created.ID, "-o", "json"); err == nil {
		t.Fatal("session resume stopped provider override error = nil, want attach rejection")
	} else if !strings.Contains(err.Error(), "session not attachable") {
		t.Fatalf("session resume stopped provider override error = %v, want not attachable", err)
	}
}

func TestCLIAgentAuthoredContextIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "--json")
	writeWorkspaceAgentDef(t, h.workspace, "coder")
	mustExecuteRoot(t, h.deps, "workspace", "add", h.workspace, "--name", "alpha", "--json")

	soulBodyPath := filepath.Join(t.TempDir(), "SOUL.md")
	soulBody := strings.Join([]string{
		"---",
		`version: "1"`,
		"role: Reviewer",
		"tone:",
		"  - concise",
		"principles:",
		"  - Keep scope tight",
		"---",
		"Review implementation behavior.",
		"",
	}, "\n")
	if err := os.WriteFile(soulBodyPath, []byte(soulBody), 0o600); err != nil {
		t.Fatalf("os.WriteFile(SOUL.md) error = %v", err)
	}

	soulWriteOut, soulWriteStderr, soulWriteErr := executeRootCommand(
		t,
		h.deps,
		"agent",
		"soul",
		"write",
		"coder",
		"--file",
		soulBodyPath,
		"--expected-digest",
		"",
		"--workspace",
		"alpha",
		"--json",
	)
	if soulWriteErr != nil {
		t.Fatalf("agent soul write error = %v; stderr=%s; stdout=%s", soulWriteErr, soulWriteStderr, soulWriteOut)
	}
	var soulMutation AgentSoulMutationRecord
	if err := json.Unmarshal([]byte(soulWriteOut), &soulMutation); err != nil {
		t.Fatalf("json.Unmarshal(agent soul write) error = %v", err)
	}
	if !soulMutation.Soul.Valid || soulMutation.Soul.Digest == "" {
		t.Fatalf("soul mutation = %#v, want valid digest", soulMutation)
	}

	soulInspectOut := mustExecuteRoot(t, h.deps, "agent", "soul", "inspect", "coder", "--workspace", "alpha", "--json")
	var soulInspect AgentSoulRecord
	if err := json.Unmarshal([]byte(soulInspectOut), &soulInspect); err != nil {
		t.Fatalf("json.Unmarshal(agent soul inspect) error = %v", err)
	}
	if soulInspect.Digest != soulMutation.Soul.Digest || strings.Contains(soulInspectOut, h.homePaths.HomeDir) {
		t.Fatalf("soul inspect = %#v, want redacted matching digest", soulInspect)
	}

	heartbeatBodyPath := filepath.Join(t.TempDir(), "HEARTBEAT.md")
	heartbeatBody := strings.Join([]string{
		"---",
		`version: "1"`,
		"enabled: true",
		`summary: "Inspect state before waking work."`,
		"preferences:",
		`  min_interval: "30m"`,
		"context:",
		"  include:",
		"    - self",
		"    - session_health",
		"---",
		"Check session health before requesting work.",
		"",
	}, "\n")
	if err := os.WriteFile(heartbeatBodyPath, []byte(heartbeatBody), 0o600); err != nil {
		t.Fatalf("os.WriteFile(HEARTBEAT.md) error = %v", err)
	}

	heartbeatWriteOut := mustExecuteRoot(
		t,
		h.deps,
		"agent",
		"heartbeat",
		"write",
		"coder",
		"--file",
		heartbeatBodyPath,
		"--if-match",
		"",
		"--workspace",
		"alpha",
		"--json",
	)
	var heartbeatMutation AgentHeartbeatMutationRecord
	if err := json.Unmarshal([]byte(heartbeatWriteOut), &heartbeatMutation); err != nil {
		t.Fatalf("json.Unmarshal(agent heartbeat write) error = %v", err)
	}
	if !heartbeatMutation.Heartbeat.Valid || heartbeatMutation.Heartbeat.ConfigDigest == "" {
		t.Fatalf("heartbeat mutation = %#v, want valid policy with config digest", heartbeatMutation)
	}

	sessionOut := mustExecuteRoot(t, h.deps, "session", "new", "--agent", "coder", "--workspace", "alpha", "--json")
	var created SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}

	heartbeatStatusOut := mustExecuteRoot(
		t,
		h.deps,
		"agent",
		"heartbeat",
		"status",
		"coder",
		"--workspace",
		"alpha",
		"--session",
		created.ID,
		"--json",
	)
	var heartbeatStatus AgentHeartbeatStatusRecord
	if err := json.Unmarshal([]byte(heartbeatStatusOut), &heartbeatStatus); err != nil {
		t.Fatalf("json.Unmarshal(agent heartbeat status) error = %v", err)
	}
	if heartbeatStatus.ConfigDigest == "" || heartbeatStatus.SessionHealth == nil {
		t.Fatalf("heartbeat status = %#v, want config digest and session health", heartbeatStatus)
	}

	sessionHealthOut := mustExecuteRoot(t, h.deps, "session", "health", created.ID, "--json")
	var sessionHealth SessionHealthRecord
	if err := json.Unmarshal([]byte(sessionHealthOut), &sessionHealth); err != nil {
		t.Fatalf("json.Unmarshal(session health) error = %v", err)
	}
	if sessionHealth.SessionID != created.ID || sessionHealth.AgentName != "coder" {
		t.Fatalf("session health = %#v, want created coder session", sessionHealth)
	}

	sessionInspectOut := mustExecuteRoot(t, h.deps, "session", "inspect", created.ID, "--json")
	var sessionInspect SessionInspectRecord
	if err := json.Unmarshal([]byte(sessionInspectOut), &sessionInspect); err != nil {
		t.Fatalf("json.Unmarshal(session inspect) error = %v", err)
	}
	if sessionInspect.SessionID != created.ID || sessionInspect.ConfigDigest == "" {
		t.Fatalf("session inspect = %#v, want policy correlation", sessionInspect)
	}
}

func TestCLINetworkRoundTripIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	newOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"net-demo",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		"builders",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new --channel error = %v", err)
	}
	var created SessionRecord
	if err := json.Unmarshal([]byte(newOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new --channel) error = %v", err)
	}
	senderOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"net-sender",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		"builders",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("sender session new --channel error = %v", err)
	}
	var sender SessionRecord
	if err := json.Unmarshal([]byte(senderOut), &sender); err != nil {
		t.Fatalf("json.Unmarshal(sender session new --channel) error = %v", err)
	}

	statusOut, _, err := executeRootCommand(t, h.deps, "network", "status", "-o", "json")
	if err != nil {
		t.Fatalf("network status error = %v", err)
	}
	var status NetworkStatusRecord
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("json.Unmarshal(network status) error = %v", err)
	}
	if !status.Enabled || status.Status != network.StatusActive {
		t.Fatalf("network status = %#v, want enabled active", status)
	}

	peersOut, _, err := executeRootCommand(t, h.deps, "network", "peers", "builders", "-o", "json")
	if err != nil {
		t.Fatalf("network peers error = %v", err)
	}
	var peers []NetworkPeerRecord
	if err := json.Unmarshal([]byte(peersOut), &peers); err != nil {
		t.Fatalf("json.Unmarshal(network peers) error = %v", err)
	}
	peerSessions := make(map[string]struct{}, len(peers))
	var receiverPeerID string
	for _, peer := range peers {
		if peer.SessionID != nil {
			peerSessions[*peer.SessionID] = struct{}{}
			if *peer.SessionID == created.ID {
				receiverPeerID = peer.PeerID
			}
		}
	}
	if _, ok := peerSessions[created.ID]; !ok {
		t.Fatalf("network peers = %#v, want blocked receiver session peer", peers)
	}
	if receiverPeerID == "" {
		t.Fatalf("network peers = %#v, want receiver peer id for mention targeting", peers)
	}
	if _, ok := peerSessions[sender.ID]; !ok {
		t.Fatalf("network peers = %#v, want sender session peer", peers)
	}

	channelsOut, _, err := executeRootCommand(t, h.deps, "network", "channels", "-o", "json")
	if err != nil {
		t.Fatalf("network channels error = %v", err)
	}
	var channels []NetworkChannelRecord
	if err := json.Unmarshal([]byte(channelsOut), &channels); err != nil {
		t.Fatalf("json.Unmarshal(network channels) error = %v", err)
	}
	if len(channels) != 1 || channels[0].Channel != "builders" || channels[0].PeerCount != 2 {
		t.Fatalf("network channels = %#v, want builders peer_count=2", channels)
	}

	events, err := h.runner.blockSession(created.ID)
	if err != nil {
		t.Fatalf("blockSession() error = %v", err)
	}
	if events == nil {
		t.Fatal("blockSession() events = nil, want event stream")
	}
	if !h.runner.waitForBlocked(created.ID, 2*time.Second) {
		t.Fatal("timed out waiting for blocked session prompt")
	}

	agentDeps := h.deps
	agentDeps.getenv = func(key string) string {
		switch key {
		case agentidentity.EnvSessionID:
			return sender.ID
		case agentidentity.EnvAgent:
			return sender.AgentName
		default:
			return ""
		}
	}
	const channelRawToken = "compozy_claim_CLI_CHANNEL_INTEGRATION_123"
	stdout, stderr, err := executeRootCommand(
		t,
		agentDeps,
		"ch", "send", "builders",
		"--body", `{"claim_token":"`+channelRawToken+`"}`,
		"--task-id", "task-security",
		"--run-id", "run-security",
		"--kind", string(contract.CoordinationMessageStatus),
		"-o", "json",
	)
	if !errors.Is(err, contract.ErrRawClaimTokenMetadata) {
		t.Fatalf("ch send raw claim-token error = %v, want ErrRawClaimTokenMetadata", err)
	}
	for _, output := range []string{stdout, stderr, err.Error()} {
		if strings.Contains(output, channelRawToken) {
			t.Fatalf("ch send validation output leaked raw claim token: %s", output)
		}
	}
	if _, _, err := executeRootCommand(
		t,
		agentDeps,
		"ch", "send", "builders",
		"--body", `{"text":"spoof"}`,
		"--task-id", "task-security",
		"--run-id", "run-security",
		"--from", "alice@39f713d0a644253f04529421b9f51b9b",
	); err == nil || !strings.Contains(err.Error(), "unknown flag: --from") {
		t.Fatalf("ch send caller identity error = %v, want unsupported identity field rejection", err)
	}

	if _, _, err := executeRootCommand(t, h.deps,
		"network", "send",
		"--session", sender.ID,
		"--channel", "builders",
		"--surface", "thread",
		"--thread", "thread_claim_rejected",
		"--kind", "say",
		"--body", `{"claim_token":"compozy_claim_cli"}`,
		"-o", "json",
	); err == nil || !strings.Contains(err.Error(), "network_raw_token_rejected") {
		t.Fatalf("network send raw claim-token error = %v, want network_raw_token_rejected", err)
	}

	// The default channel fanout policy is capability_match, which digests
	// non-activating broadcasts. Mention the blocked receiver so the message is
	// fully delivered and queued in its inbox while its prompt is blocked.
	sendOut, _, err := executeRootCommand(t, h.deps,
		"network", "send",
		"--session", sender.ID,
		"--channel", "builders",
		"--surface", "thread",
		"--thread", "thread_cli_queued",
		"--kind", "say",
		"--body", `{"text":"queued hello"}`,
		"--ext", `{"compozy.workflow_id":"wf-1","compozy.handoff_version":3}`,
		"--mention", receiverPeerID,
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("network send error = %v", err)
	}
	var sent NetworkSendRecord
	if err := json.Unmarshal([]byte(sendOut), &sent); err != nil {
		t.Fatalf("json.Unmarshal(network send) error = %v", err)
	}
	if sent.ID == "" || string(sent.Ext["compozy.workflow_id"]) != `"wf-1"` {
		t.Fatalf("sent = %#v, want message id and ext metadata", sent)
	}
	if sent.Surface != "thread" || sent.ThreadID != "thread_cli_queued" {
		t.Fatalf("sent = %#v, want thread surface response", sent)
	}

	threadsOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"threads",
		"list",
		"--channel",
		"builders",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network threads list error = %v", err)
	}
	var threads contract.NetworkThreadsResponse
	if err := json.Unmarshal([]byte(threadsOut), &threads); err != nil {
		t.Fatalf("json.Unmarshal(network threads list) error = %v", err)
	}
	if len(threads.Threads) != 1 || threads.Threads[0].ThreadID != "thread_cli_queued" {
		t.Fatalf("network threads = %#v, want queued thread", threads)
	}

	threadOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"threads",
		"show",
		"--channel",
		"builders",
		"--thread",
		"thread_cli_queued",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network threads show error = %v", err)
	}
	var thread contract.NetworkThreadResponse
	if err := json.Unmarshal([]byte(threadOut), &thread); err != nil {
		t.Fatalf("json.Unmarshal(network threads show) error = %v", err)
	}
	if thread.Thread.ThreadID != "thread_cli_queued" || thread.Thread.MessageCount != 1 {
		t.Fatalf("network thread = %#v, want one queued message", thread)
	}

	threadMessagesOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"threads",
		"messages",
		"--channel",
		"builders",
		"--thread",
		"thread_cli_queued",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network threads messages error = %v", err)
	}
	var threadMessages contract.NetworkThreadMessagesResponse
	if err := json.Unmarshal([]byte(threadMessagesOut), &threadMessages); err != nil {
		t.Fatalf("json.Unmarshal(network threads messages) error = %v", err)
	}
	if len(threadMessages.Messages) != 1 || threadMessages.Messages[0].MessageID != sent.ID {
		t.Fatalf("network thread messages = %#v, want sent message", threadMessages)
	}

	var inbox []NetworkEnvelopeRecord
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		inboxOut, _, inboxErr := executeRootCommand(
			t,
			h.deps,
			"network",
			"inbox",
			"--session",
			created.ID,
			"-o",
			"json",
		)
		if inboxErr != nil {
			t.Fatalf("network inbox error = %v", inboxErr)
		}
		if err := json.Unmarshal([]byte(inboxOut), &inbox); err != nil {
			t.Fatalf("json.Unmarshal(network inbox) error = %v", err)
		}
		if len(inbox) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(inbox) == 0 {
		t.Fatal("network inbox = empty, want queued message while prompt is blocked")
	}
	if string(inbox[0].Ext["compozy.workflow_id"]) != `"wf-1"` ||
		string(inbox[0].Ext["compozy.handoff_version"]) != `3` {
		t.Fatalf("network inbox = %#v, want workflow metadata", inbox)
	}

	h.runner.releaseBlocked(created.ID)
}

func TestCLINetworkDirectRetryAndResumeIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	newSession := func(name string) SessionRecord {
		t.Helper()

		out, _, err := executeRootCommand(
			t,
			h.deps,
			"session",
			"new",
			"--agent",
			"coder",
			"--name",
			name,
			"--network",
			"live",
			"--network-channel-strategy",
			"named",
			"--network-channel",
			"builders",
			"--cwd",
			h.workspace,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("session new %s error = %v", name, err)
		}
		var created SessionRecord
		if err := json.Unmarshal([]byte(out), &created); err != nil {
			t.Fatalf("json.Unmarshal(session new %s) error = %v", name, err)
		}
		return created
	}

	sender := newSession("sender")
	receiver := newSession("receiver")
	receiverPeerID := "coder." + receiver.ID

	resolveDirect := func() contract.NetworkDirectRoomResponse {
		t.Helper()

		out, _, err := executeRootCommand(
			t,
			h.deps,
			"network",
			"directs",
			"resolve",
			"--session",
			sender.ID,
			"--channel",
			"builders",
			"--peer",
			receiverPeerID,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("network directs resolve error = %v", err)
		}
		var resolved contract.NetworkDirectRoomResponse
		if err := json.Unmarshal([]byte(out), &resolved); err != nil {
			t.Fatalf("json.Unmarshal(network directs resolve) error = %v", err)
		}
		return resolved
	}

	resolvedDirect := resolveDirect()
	directID := strings.TrimSpace(resolvedDirect.Direct.DirectID)
	if directID == "" {
		t.Fatalf("resolved direct = %#v, want non-empty direct id", resolvedDirect)
	}
	resolvedDirectAgain := resolveDirect()
	if resolvedDirectAgain.Direct.DirectID != directID {
		t.Fatalf("resolved direct again = %#v, want same direct id %q", resolvedDirectAgain, directID)
	}

	events, err := h.runner.blockSession(receiver.ID)
	if err != nil {
		t.Fatalf("blockSession() error = %v", err)
	}
	if events == nil {
		t.Fatal("blockSession() events = nil, want event stream")
	}
	if !h.runner.waitForBlocked(receiver.ID, 2*time.Second) {
		t.Fatal("timed out waiting for blocked receiver prompt")
	}

	sendDirect := func(messageID string, workID string, text string) {
		t.Helper()

		out, _, err := executeRootCommand(t, h.deps,
			"network", "send",
			"--session", sender.ID,
			"--channel", "builders",
			"--surface", "direct",
			"--direct", directID,
			"--kind", "say",
			"--to", receiverPeerID,
			"--work", workID,
			"--id", messageID,
			"--body", fmt.Sprintf(`{"text":%q}`, text),
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("network send direct error = %v", err)
		}
		var sent NetworkSendRecord
		if err := json.Unmarshal([]byte(out), &sent); err != nil {
			t.Fatalf("json.Unmarshal(network send direct) error = %v", err)
		}
		if sent.ID != messageID {
			t.Fatalf("sent.ID = %q, want %q", sent.ID, messageID)
		}
	}

	readInbox := func(sessionID string) []NetworkEnvelopeRecord {
		t.Helper()

		out, _, err := executeRootCommand(t, h.deps, "network", "inbox", "--session", sessionID, "-o", "json")
		if err != nil {
			t.Fatalf("network inbox error = %v", err)
		}
		var inbox []NetworkEnvelopeRecord
		if err := json.Unmarshal([]byte(out), &inbox); err != nil {
			t.Fatalf("json.Unmarshal(network inbox) error = %v", err)
		}
		return inbox
	}

	sendDirect("msg-direct-retry-1", "work_review_1", "please review auth.go")
	sendDirect("msg-direct-retry-1", "work_review_1", "please review auth.go")

	waitForCondition(t, 2*time.Second, func() bool {
		inbox := readInbox(receiver.ID)
		return len(inbox) == 1 && inbox[0].ID == "msg-direct-retry-1"
	})
	directsOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"directs",
		"list",
		"--channel",
		"builders",
		"--session",
		receiver.ID,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network directs list error = %v", err)
	}
	var directs contract.NetworkDirectRoomsResponse
	if err := json.Unmarshal([]byte(directsOut), &directs); err != nil {
		t.Fatalf("json.Unmarshal(network directs list) error = %v", err)
	}
	if len(directs.Directs) != 1 || directs.Directs[0].DirectID != directID {
		t.Fatalf("network directs = %#v, want direct room", directs)
	}

	directOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"directs",
		"show",
		"--channel",
		"builders",
		"--direct",
		directID,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network directs show error = %v", err)
	}
	var direct contract.NetworkDirectRoomResponse
	if err := json.Unmarshal([]byte(directOut), &direct); err != nil {
		t.Fatalf("json.Unmarshal(network directs show) error = %v", err)
	}
	if direct.Direct.DirectID != directID || direct.Direct.MessageCount != 1 {
		t.Fatalf("network direct = %#v, want one accepted message", direct)
	}

	directMessagesOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"directs",
		"messages",
		"--channel",
		"builders",
		"--direct",
		directID,
		"--work",
		"work_review_1",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network directs messages error = %v", err)
	}
	var directMessages contract.NetworkDirectRoomMessagesResponse
	if err := json.Unmarshal([]byte(directMessagesOut), &directMessages); err != nil {
		t.Fatalf("json.Unmarshal(network directs messages) error = %v", err)
	}
	if len(directMessages.Messages) != 1 || directMessages.Messages[0].MessageID != "msg-direct-retry-1" {
		t.Fatalf("network direct messages = %#v, want accepted direct message", directMessages)
	}

	workOut, _, err := executeRootCommand(
		t,
		h.deps,
		"network",
		"work",
		"lookup",
		"--work",
		"work_review_1",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("network work lookup error = %v", err)
	}
	var work contract.NetworkWorkResponse
	if err := json.Unmarshal([]byte(workOut), &work); err != nil {
		t.Fatalf("json.Unmarshal(network work lookup) error = %v", err)
	}
	if work.Work.WorkID != "work_review_1" || work.Work.DirectID != directID {
		t.Fatalf("network work = %#v, want direct-bound work", work)
	}

	h.runner.releaseBlocked(receiver.ID)
	inbox := readInbox(receiver.ID)
	if len(inbox) != 1 || inbox[0].ID != "msg-direct-retry-1" {
		t.Fatalf("network inbox after prompt completion = %#v, want immutable delivered message", inbox)
	}

	stopOut, _, err := executeRootCommand(t, h.deps, "session", "stop", receiver.ID, "-o", "json")
	if err != nil {
		t.Fatalf("session stop receiver error = %v", err)
	}
	var stopped SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stopped); err != nil {
		t.Fatalf("json.Unmarshal(session stop receiver) error = %v", err)
	}
	if stopped.State != session.StateStopped {
		t.Fatalf("stopped receiver = %#v, want stopped state", stopped)
	}

	if _, _, err := executeRootCommand(t, h.deps, "session", "resume", receiver.ID, "-o", "json"); err == nil {
		t.Fatal("session resume stopped receiver error = nil, want attach rejection")
	} else if !strings.Contains(err.Error(), "session not attachable") {
		t.Fatalf("session resume stopped receiver error = %v, want not attachable", err)
	}
	receiver = newSession("receiver-reconnect")
	receiverPeerID = "coder." + receiver.ID
	reconnectedDirect := resolveDirect()
	directID = strings.TrimSpace(reconnectedDirect.Direct.DirectID)
	if directID == "" {
		t.Fatalf("resolved reconnected direct = %#v, want non-empty direct id", reconnectedDirect)
	}

	resumedEvents, err := h.runner.blockSession(receiver.ID)
	if err != nil {
		t.Fatalf("blockSession(resumed) error = %v", err)
	}
	if resumedEvents == nil {
		t.Fatal("blockSession(resumed) events = nil, want event stream")
	}
	if !h.runner.waitForBlocked(receiver.ID, 2*time.Second) {
		t.Fatal("timed out waiting for blocked resumed receiver prompt")
	}

	sendDirect("msg-direct-resume-1", "work_review_2", "please review after resume")

	waitForCondition(t, 2*time.Second, func() bool {
		inbox := readInbox(receiver.ID)
		return len(inbox) == 1 && inbox[0].ID == "msg-direct-resume-1"
	})

	peersOut, _, err := executeRootCommand(t, h.deps, "network", "peers", "builders", "-o", "json")
	if err != nil {
		t.Fatalf("network peers error = %v", err)
	}
	var peers []NetworkPeerRecord
	if err := json.Unmarshal([]byte(peersOut), &peers); err != nil {
		t.Fatalf("json.Unmarshal(network peers) error = %v", err)
	}
	var receiverPresent bool
	for _, peer := range peers {
		if peer.SessionID != nil && *peer.SessionID == receiver.ID && peer.PeerID == receiverPeerID {
			receiverPresent = true
			break
		}
	}
	if !receiverPresent {
		t.Fatalf("network peers = %#v, want resumed receiver peer", peers)
	}

	h.runner.releaseBlocked(receiver.ID)
	inbox = readInbox(receiver.ID)
	if len(inbox) != 1 || inbox[0].ID != "msg-direct-resume-1" {
		t.Fatalf("network inbox after resumed prompt completion = %#v, want immutable delivered message", inbox)
	}
}

func TestExtensionCommandRoundTripIntegration(t *testing.T) {
	t.Parallel()
	t.Run("Should round-trip extension inventory preview and network consent", func(t *testing.T) {
		t.Parallel()

		h := newIntegrationHarness(t)
		h.runner.cfg.Extensions.Trust.AllowUnverified = true
		mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
		defer func() {
			if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
				t.Errorf("daemon stop cleanup error = %v", err)
			}
			if err := h.runner.waitForExit(); err != nil {
				t.Errorf("waitForExit() cleanup error = %v", err)
			}
		}()

		dir := writeExtensionFixture(t, "integration-ext", extensionFixtureOptions{})
		writeExtensionManifest(
			t,
			filepath.Join(dir, "extension.toml"),
			extensionFixtureManifest("integration-ext", extensionFixtureOptions{})+`

[network_participation]
required = true
mode = "live"
channel_scopes = ["team/*"]
`,
		)

		installOut, _, err := executeRootCommand(
			t,
			h.deps,
			"extension",
			"install",
			dir,
			"--allow-unverified",
			"--yes",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("extension install error = %v", err)
		}
		var installed ExtensionRecord
		if err := json.Unmarshal([]byte(installOut), &installed); err != nil {
			t.Fatalf("json.Unmarshal(extension install) error = %v", err)
		}
		if installed.Name != "integration-ext" || installed.State != "disabled" || installed.Enabled ||
			!installed.DaemonRunning ||
			installed.NetworkRequirementDigest == "" || !installed.NetworkConfirmationRequired {
			t.Fatalf(
				"installed extension = %#v, want inert daemon-backed extension awaiting network consent",
				installed,
			)
		}

		listOut, _, err := executeRootCommand(t, h.deps, "extension", "list", "-o", "json")
		if err != nil {
			t.Fatalf("extension list error = %v", err)
		}
		var listed []ExtensionRecord
		if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
			t.Fatalf("json.Unmarshal(extension list) error = %v", err)
		}
		if len(listed) != 1 || listed[0].Name != "integration-ext" || listed[0].State != "disabled" {
			t.Fatalf("listed extensions = %#v, want one inert extension", listed)
		}

		statusOut, _, err := executeRootCommand(t, h.deps, "extension", "status", "integration-ext", "-o", "json")
		if err != nil {
			t.Fatalf("extension status error = %v", err)
		}
		var status ExtensionRecord
		if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
			t.Fatalf("json.Unmarshal(extension status) error = %v", err)
		}
		if status.Name != "integration-ext" || status.State != "disabled" || !status.NetworkConfirmationRequired {
			t.Fatalf("extension status = %#v, want inert extension awaiting network consent", status)
		}

		inventoryOut := mustExecuteRoot(t, h.deps, "extension", "inventory", "integration-ext", "-o", "json")
		var inventory ExtensionInventoryRecord
		if err := json.Unmarshal([]byte(inventoryOut), &inventory); err != nil {
			t.Fatalf("json.Unmarshal(extension inventory) error = %v", err)
		}
		if inventory.Extension != "integration-ext" || inventory.Enabled || len(inventory.Items) != 0 {
			t.Fatalf("extension inventory = %#v, want disabled empty kit", inventory)
		}

		previewOut := mustExecuteRoot(t, h.deps, "extension", "preview", "integration-ext", "-o", "json")
		var preview ExtensionEnablePreviewRecord
		if err := json.Unmarshal([]byte(previewOut), &preview); err != nil {
			t.Fatalf("json.Unmarshal(extension preview) error = %v", err)
		}
		if preview.Extension != "integration-ext" || preview.NetworkRequirementDigest == "" ||
			!preview.NetworkConfirmationRequired || len(preview.Changes) != 0 {
			t.Fatalf("extension preview = %#v, want current network digest and empty kit", preview)
		}

		_, _, enableErr := executeRootCommand(t, h.deps, "extension", "enable", "integration-ext", "-o", "json")

		operationErr, operationErrMatched := errors.AsType[*extensionOperationAPIError](enableErr)
		if !operationErrMatched {
			t.Fatalf("extension enable error = %v, want structured operation error", enableErr)
		}
		operationPayload := operationErr.extensionOperationErrorPayload()
		if operationPayload.Code != "extension_network_confirmation_required" ||
			operationPayload.CurrentDigest != preview.NetworkRequirementDigest {
			t.Fatalf("extension enable error payload = %#v, want preview digest", operationPayload)
		}

		enableOut := mustExecuteRoot(
			t,
			h.deps,
			"extension",
			"enable",
			"integration-ext",
			"--"+extensionConfirmNetworkFlagName,
			preview.NetworkRequirementDigest,
			"-o",
			"json",
		)
		var enabled ExtensionEnableRecord
		if err := json.Unmarshal([]byte(enableOut), &enabled); err != nil {
			t.Fatalf("json.Unmarshal(extension enable) error = %v", err)
		}
		if !enabled.Extension.Enabled || enabled.Extension.NetworkConfirmationRequired {
			t.Fatalf("enabled extension = %#v, want enabled with recorded network consent", enabled)
		}
	})
}

func TestSessionEventsFollowIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	sessionOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"demo",
		"--cwd",
		h.workspace,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new error = %v", err)
	}
	var created SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}

	if _, _, err := executeRootCommand(t, h.deps, "session", "prompt", created.ID, "hello", "-o", "json"); err != nil {
		t.Fatalf("session prompt error = %v", err)
	}

	cmd := newRootCommand(h.deps)
	var stderr bytes.Buffer
	stdout := &lockedBuffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"session", "events", created.ID, "--follow", "-o", "json"})

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		done <- cmd.ExecuteContext(ctx)
	}()

	waitForCondition(t, 3*time.Second, func() bool {
		return strings.Contains(stdout.String(), `"type":"agent_message"`)
	})

	if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
		t.Fatalf("daemon stop error = %v", err)
	}

	if err := <-done; err != nil {
		t.Fatalf("follow command error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("follow output lines = %d, want at least 2", len(lines))
	}
	var sawAgentMessage bool
	for _, line := range lines {
		var event SessionEventRecord
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("json.Unmarshal(follow line) error = %v; line=%s", err, line)
		}
		if event.Type == "agent_message" {
			sawAgentMessage = true
		}
	}
	if !sawAgentMessage {
		t.Fatalf("follow output = %q, want streamed agent_message event", stdout.String())
	}
}

func TestWorkspaceCommandsIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	addOut, _, err := executeRootCommand(t, h.deps, "workspace", "add", h.workspace, "--name", "alpha", "-o", "json")
	if err != nil {
		t.Fatalf("workspace add error = %v", err)
	}
	var registered WorkspaceRecord
	if err := json.Unmarshal([]byte(addOut), &registered); err != nil {
		t.Fatalf("json.Unmarshal(workspace add) error = %v", err)
	}
	if registered.ID == "" {
		t.Fatal("expected registered workspace id")
	}

	infoOut, _, err := executeRootCommand(t, h.deps, "workspace", "info", "alpha", "-o", "json")
	if err != nil {
		t.Fatalf("workspace info error = %v", err)
	}
	var detail WorkspaceDetailRecord
	if err := json.Unmarshal([]byte(infoOut), &detail); err != nil {
		t.Fatalf("json.Unmarshal(workspace info) error = %v", err)
	}
	if detail.Workspace.ID != registered.ID {
		t.Fatalf("workspace info id = %q, want %q", detail.Workspace.ID, registered.ID)
	}

	sessionOut, _, err := executeRootCommand(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"demo",
		"--workspace",
		"alpha",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("session new with workspace error = %v", err)
	}
	var created SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if created.WorkspaceID != registered.ID {
		t.Fatalf("created.WorkspaceID = %q, want %q", created.WorkspaceID, registered.ID)
	}

	listOut, _, err := executeRootCommand(t, h.deps, "session", "list", "--workspace", "alpha", "--all", "-o", "json")
	if err != nil {
		t.Fatalf("session list --workspace error = %v", err)
	}
	var listed SessionListPage
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(session list) error = %v", err)
	}
	if len(listed.Sessions) != 1 || listed.Sessions[0].ID != created.ID {
		t.Fatalf("listed = %#v, want one workspace-filtered session", listed)
	}
}

func TestMemoryWriteListIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	writeOut, _, err := executeRootCommand(
		t,
		h.deps,
		"memory",
		"write",
		"--type",
		"user",
		"--name",
		"Prefs",
		"--description",
		"cli memory",
		"--content",
		"remember this",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("memory write error = %v", err)
	}
	var written MemoryMutationRecord
	if err := json.Unmarshal([]byte(writeOut), &written); err != nil {
		t.Fatalf("json.Unmarshal(memory write) error = %v; out=%s", err, writeOut)
	}
	if !written.Applied || written.Decision.TargetFilename == "" {
		t.Fatalf("written = %#v, want applied decision with target filename", written)
	}

	listOut, _, err := executeRootCommand(t, h.deps, "memory", "list", "--scope", "global", "-o", "json")
	if err != nil {
		t.Fatalf("memory list error = %v", err)
	}

	var listed struct {
		Memories []memoryListItem `json:"memories"`
	}
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(memory list) error = %v; out=%s", err, listOut)
	}
	memories := listed.Memories
	if len(memories) != 1 || memories[0].Filename != written.Decision.TargetFilename {
		t.Fatalf("memories = %#v, want %q", memories, written.Decision.TargetFilename)
	}
}

func TestAutomationJobsCreateOutputFormatsIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	humanOut, _, err := executeRootCommand(
		t,
		h.deps,
		"automation", "jobs", "create",
		"--name", "nightly-human",
		"--scope", "global",
		"--schedule", "every:30m",
		"--agent", "coder",
		"--prompt", "review repo",
		"-o", "human",
	)
	if err != nil {
		t.Fatalf("automation jobs create human error = %v", err)
	}
	if !strings.Contains(humanOut, "Automation Job") || !strings.Contains(humanOut, "nightly-human") {
		t.Fatalf("human output = %q, want created job detail", humanOut)
	}

	jsonOut, _, err := executeRootCommand(
		t,
		h.deps,
		"automation", "jobs", "create",
		"--name", "nightly-json",
		"--scope", "global",
		"--schedule", "every:45m",
		"--agent", "coder",
		"--prompt", "review repo later",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("automation jobs create json error = %v", err)
	}
	var created JobRecord
	if err := json.Unmarshal([]byte(jsonOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(automation jobs create) error = %v", err)
	}
	if created.ID == "" || created.Name != "nightly-json" || created.Scope != automationpkg.AutomationScopeGlobal {
		t.Fatalf("created job = %#v, want global created job", created)
	}
}

func TestAutomationTriggerHistoryAndRunsIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	workspaceOut := mustExecuteRoot(t, h.deps, "workspace", "add", h.workspace, "--name", "alpha", "-o", "json")
	var workspace WorkspaceRecord
	if err := json.Unmarshal([]byte(workspaceOut), &workspace); err != nil {
		t.Fatalf("json.Unmarshal(workspace add) error = %v", err)
	}
	if workspace.ID == "" {
		t.Fatal("expected workspace id after registration")
	}

	triggerOut := mustExecuteRoot(
		t,
		h.deps,
		"automation", "triggers", "create",
		"--name", "stop-review",
		"--scope", "workspace",
		"--workspace", "alpha",
		"--event", "session.stopped",
		"--agent", "coder",
		"--prompt", `review {{ index .Data "session_id" }}`,
		"-o", "json",
	)
	var createdTrigger TriggerRecord
	if err := json.Unmarshal([]byte(triggerOut), &createdTrigger); err != nil {
		t.Fatalf("json.Unmarshal(trigger create) error = %v", err)
	}
	if createdTrigger.ID == "" || createdTrigger.WorkspaceID != workspace.ID {
		t.Fatalf("created trigger = %#v, want workspace-bound trigger", createdTrigger)
	}

	sessionOut := mustExecuteRoot(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"demo",
		"--workspace",
		"alpha",
		"-o",
		"json",
	)
	var createdSession SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &createdSession); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if createdSession.ID == "" {
		t.Fatal("expected session id for trigger test")
	}

	if _, _, err := executeRootCommand(t, h.deps, "session", "stop", createdSession.ID, "-o", "json"); err != nil {
		t.Fatalf("session stop error = %v", err)
	}

	waitForCondition(t, 5*time.Second, func() bool {
		stdout, _, err := executeRootCommand(
			t,
			h.deps,
			"automation",
			"triggers",
			"history",
			createdTrigger.ID,
			"-o",
			"json",
		)
		if err != nil {
			return false
		}
		var runs contract.RunsResponse
		if err := json.Unmarshal([]byte(stdout), &runs); err != nil {
			return false
		}
		return len(runs.Runs) > 0
	})

	historyHuman, _, err := executeRootCommand(
		t,
		h.deps,
		"automation",
		"triggers",
		"history",
		createdTrigger.ID,
		"-o",
		"human",
	)
	if err != nil {
		t.Fatalf("automation triggers history human error = %v", err)
	}
	if !strings.Contains(historyHuman, "Automation Runs") ||
		!strings.Contains(historyHuman, "trigger:"+createdTrigger.ID) {
		t.Fatalf("history human output = %q, want trigger run table", historyHuman)
	}

	historyJSON, _, err := executeRootCommand(
		t,
		h.deps,
		"automation",
		"triggers",
		"history",
		createdTrigger.ID,
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("automation triggers history json error = %v", err)
	}
	var triggerRuns contract.RunsResponse
	if err := json.Unmarshal([]byte(historyJSON), &triggerRuns); err != nil {
		t.Fatalf("json.Unmarshal(trigger history) error = %v", err)
	}
	if len(triggerRuns.Runs) == 0 || triggerRuns.Runs[0].TriggerID != createdTrigger.ID {
		t.Fatalf("trigger runs = %#v, want at least one run for trigger %q", triggerRuns, createdTrigger.ID)
	}

	runsHuman, _, err := executeRootCommand(t, h.deps, "automation", "runs", "-o", "human")
	if err != nil {
		t.Fatalf("automation runs human error = %v", err)
	}
	if !strings.Contains(runsHuman, "Automation Runs") || !strings.Contains(runsHuman, createdTrigger.ID) {
		t.Fatalf("runs human output = %q, want shared run table", runsHuman)
	}

	runsJSON, _, err := executeRootCommand(t, h.deps, "automation", "runs", "-o", "json")
	if err != nil {
		t.Fatalf("automation runs json error = %v", err)
	}
	var allRuns contract.RunsResponse
	if err := json.Unmarshal([]byte(runsJSON), &allRuns); err != nil {
		t.Fatalf("json.Unmarshal(automation runs) error = %v", err)
	}
	if len(allRuns.Runs) == 0 {
		t.Fatal("expected at least one automation run in shared history")
	}
	found := false
	for _, run := range allRuns.Runs {
		if run.TriggerID == createdTrigger.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("allRuns = %#v, want one run for trigger %q", allRuns, createdTrigger.ID)
	}
}

func TestBridgeCreateAndGetIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"workspace",
		"add",
		h.workspace,
		"--name",
		"alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}

	createOut := mustExecuteRoot(
		t,
		h.deps,
		"bridge", "create",
		"--scope", "global",
		"--platform", "telegram",
		"--extension", "ext-telegram",
		"--display-name", "Support",
		"--include-peer",
		"-o", "json",
	)

	var created BridgeRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(bridge create) error = %v", err)
	}
	if created.ID == "" || created.Platform != "telegram" || created.Status != bridgepkg.BridgeStatusStarting {
		t.Fatalf("created bridge = %#v", created)
	}

	getOut := mustExecuteRoot(t, h.deps, "bridge", "get", created.ID, "-o", "json")

	var fetched BridgeRecord
	if err := json.Unmarshal([]byte(getOut), &fetched); err != nil {
		t.Fatalf("json.Unmarshal(bridge get) error = %v", err)
	}
	if fetched.ID != created.ID || fetched.DisplayName != "Support" || fetched.ExtensionName != "ext-telegram" {
		t.Fatalf("fetched bridge = %#v, want created record", fetched)
	}
}

func TestBridgeLifecycleCommandsIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	createOut := mustExecuteRoot(
		t,
		h.deps,
		"bridge", "create",
		"--scope", "global",
		"--platform", "telegram",
		"--extension", "ext-telegram",
		"--display-name", "Ops",
		"--enabled=false",
		"--include-peer",
		"-o", "json",
	)

	var created BridgeRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(bridge create) error = %v", err)
	}
	if created.Status != bridgepkg.BridgeStatusDisabled || created.Enabled {
		t.Fatalf("created lifecycle = %#v, want disabled false", created)
	}

	enableOut := mustExecuteRoot(t, h.deps, "bridge", "enable", created.ID, "-o", "json")
	var enabled BridgeRecord
	if err := json.Unmarshal([]byte(enableOut), &enabled); err != nil {
		t.Fatalf("json.Unmarshal(bridge enable) error = %v", err)
	}
	if enabled.Status != bridgepkg.BridgeStatusStarting || !enabled.Enabled {
		t.Fatalf("enabled bridge = %#v, want starting true", enabled)
	}

	disableOut := mustExecuteRoot(t, h.deps, "bridge", "disable", created.ID, "-o", "json")
	var disabled BridgeRecord
	if err := json.Unmarshal([]byte(disableOut), &disabled); err != nil {
		t.Fatalf("json.Unmarshal(bridge disable) error = %v", err)
	}
	if disabled.Status != bridgepkg.BridgeStatusDisabled || disabled.Enabled {
		t.Fatalf("disabled bridge = %#v, want disabled false", disabled)
	}

	restartOut := mustExecuteRoot(t, h.deps, "bridge", "restart", created.ID, "-o", "json")
	var restarted BridgeRecord
	if err := json.Unmarshal([]byte(restartOut), &restarted); err != nil {
		t.Fatalf("json.Unmarshal(bridge restart) error = %v", err)
	}
	if restarted.Status != bridgepkg.BridgeStatusStarting || !restarted.Enabled {
		t.Fatalf("restarted bridge = %#v, want starting true", restarted)
	}
}

func TestBridgeRoutesIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	createOut := mustExecuteRoot(
		t,
		h.deps,
		"bridge", "create",
		"--scope", "global",
		"--platform", "telegram",
		"--extension", "ext-telegram",
		"--display-name", "Support",
		"--include-peer",
		"--include-thread",
		"-o", "json",
	)

	var created BridgeRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(bridge create) error = %v", err)
	}

	bridges := h.runner.bridgeService()
	if bridges == nil {
		t.Fatal("bridge service = nil, want running integration bridge service")
	}
	if _, err := bridges.UpsertRoute(context.Background(), bridgepkg.BridgeRoute{
		BridgeInstanceID: created.ID,
		Scope:            created.Scope,
		WorkspaceID:      created.WorkspaceID,
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		SessionID:        "sess-1",
		AgentName:        "coder",
		LastActivityAt:   fixedTestNow,
	}); err != nil {
		t.Fatalf("UpsertRoute() error = %v", err)
	}

	routesOut := mustExecuteRoot(t, h.deps, "bridge", "routes", created.ID, "-o", "json")

	var routes []BridgeRouteRecord
	if err := json.Unmarshal([]byte(routesOut), &routes); err != nil {
		t.Fatalf("json.Unmarshal(bridge routes) error = %v", err)
	}
	if len(routes) != 1 || routes[0].PeerID != "peer-1" || routes[0].ThreadID != "thread-1" {
		t.Fatalf("routes = %#v, want one inserted route", routes)
	}

	_, _, err := executeRootCommand(t, h.deps, "bridge", "routes", "missing-bridge", "-o", "json")
	if err == nil || !strings.Contains(err.Error(), "bridge instance not found") {
		t.Fatalf("bridge routes missing error = %v, want bridge instance not found", err)
	}
}

func TestCLITaskCreateListGetIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"workspace",
		"add",
		h.workspace,
		"--name",
		"alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}

	createOut, _, err := executeRootCommand(
		t,
		h.deps,
		"task", "create",
		"--scope", "workspace",
		"--workspace", "alpha",
		"--network", "live",
		"--network-channel-strategy", "named",
		"--network-channel", "builders",
		"--title", "Investigate flaky task runs",
		"--description", "Capture root cause",
		"--priority", "high",
		"--owner-kind", "pool",
		"--owner-ref", "triage",
		"--metadata", `{"source":"qa"}`,
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("task create error = %v", err)
	}

	var created TaskRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(task create) error = %v", err)
	}
	if created.ID == "" ||
		created.Scope != taskpkg.ScopeWorkspace ||
		created.WorkspaceID == "" ||
		created.Priority != taskpkg.PriorityHigh {
		t.Fatalf("created task = %#v, want high-priority workspace task with id", created)
	}

	listOut, _, err := executeRootCommand(
		t,
		h.deps,
		"task",
		"list",
		"--scope",
		"workspace",
		"--workspace",
		"alpha",
		"--status",
		"ready",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("task list error = %v", err)
	}
	var listed TaskListRecord
	if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
		t.Fatalf("json.Unmarshal(task list) error = %v", err)
	}
	if len(listed.Tasks) != 1 || listed.Tasks[0].ID != created.ID {
		t.Fatalf("listed tasks = %#v, want created task", listed)
	}

	getOut, _, err := executeRootCommand(t, h.deps, "task", "get", created.ID, "-o", "json")
	if err != nil {
		t.Fatalf("task get error = %v", err)
	}
	var detail TaskDetailRecord
	if err := json.Unmarshal([]byte(getOut), &detail); err != nil {
		t.Fatalf("json.Unmarshal(task get) error = %v", err)
	}
	if detail.Task.ID != created.ID || detail.Task.Owner == nil || detail.Task.Owner.Kind != taskpkg.OwnerKindPool {
		t.Fatalf("task detail = %#v, want created task detail with owner", detail)
	}
}

func TestCLITaskRunLifecycleIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"workspace",
		"add",
		h.workspace,
		"--name",
		"alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}

	createOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"create",
		"--scope",
		"workspace",
		"--workspace",
		"alpha",
		"--title",
		"Review task lifecycle",
		"-o",
		"json",
	)
	var created TaskRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(task create) error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created task id")
	}

	enqueueOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"run",
		"enqueue",
		created.ID,
		"--idempotency-key",
		"idem-1",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		"builders",
		"--metadata",
		`{"schema":"compozy.harness.detached.v1"}`,
		"-o",
		"json",
	)
	var enqueued TaskRunRecord
	if err := json.Unmarshal([]byte(enqueueOut), &enqueued); err != nil {
		t.Fatalf("json.Unmarshal(task run enqueue) error = %v", err)
	}
	if enqueued.Status != taskpkg.TaskRunStatusQueued {
		t.Fatalf("enqueued run = %#v, want queued", enqueued)
	}
	assertDetachedHarnessMetadata(t, "enqueued metadata", enqueued.Metadata)

	agentDeps, worker := newIntegrationAgentCommandDeps(t, h, "task-run-lifecycle-worker", "alpha", "builders")
	claimOut := mustExecuteRoot(t, agentDeps, "task", "next", "--run-id", enqueued.ID, "-o", "json")
	var next AgentTaskNextRecord
	if err := json.Unmarshal([]byte(claimOut), &next); err != nil {
		t.Fatalf("json.Unmarshal(task next exact claim) error = %v", err)
	}
	if !next.Claimed || next.Claim == nil || next.Claim.Run.ID != enqueued.ID ||
		next.Claim.Run.Status != taskpkg.TaskRunStatusClaimed {
		t.Fatalf("task next = %#v, want exact claimed run %q", next, enqueued.ID)
	}

	completeOut := mustExecuteRoot(
		t,
		agentDeps,
		"task",
		"complete",
		enqueued.ID,
		"--result",
		`{"ok":true}`,
		"-o",
		"json",
	)
	var completed AgentTaskLeaseRecord
	if err := json.Unmarshal([]byte(completeOut), &completed); err != nil {
		t.Fatalf("json.Unmarshal(task complete) error = %v", err)
	}
	if completed.Status != taskpkg.TaskRunStatusCompleted || completed.RunID != enqueued.ID ||
		completed.SessionID != worker.ID {
		t.Fatalf("completed lease = %#v, want completed run %q for worker %q", completed, enqueued.ID, worker.ID)
	}

	runsOut := mustExecuteRoot(t, h.deps, "task", "run", "list", created.ID, "-o", "json")
	var runs []TaskRunRecord
	if err := json.Unmarshal([]byte(runsOut), &runs); err != nil {
		t.Fatalf("json.Unmarshal(task run list) error = %v", err)
	}
	if len(runs) != 1 || runs[0].Status != taskpkg.TaskRunStatusCompleted {
		t.Fatalf("runs = %#v, want completed run history", runs)
	}
	var resultPayload map[string]bool
	if err := json.Unmarshal(runs[0].Result, &resultPayload); err != nil {
		t.Fatalf("json.Unmarshal(completed result) error = %v", err)
	}
	if !resultPayload["ok"] {
		t.Fatalf("runs[0].Result = %s, want ok=true", runs[0].Result)
	}
	assertDetachedHarnessMetadata(t, "runs[0].Metadata", runs[0].Metadata)

	stopOut := mustExecuteRoot(t, h.deps, "session", "stop", worker.ID, "-o", "json")
	var stoppedWorker SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stoppedWorker); err != nil {
		t.Fatalf("json.Unmarshal(session stop worker) error = %v", err)
	}
	if stoppedWorker.State != session.StateStopped {
		t.Fatalf("stopped worker = %#v, want stopped", stoppedWorker)
	}

	getOut := mustExecuteRoot(t, h.deps, "task", "get", created.ID, "-o", "json")
	var detail TaskDetailRecord
	if err := json.Unmarshal([]byte(getOut), &detail); err != nil {
		t.Fatalf("json.Unmarshal(task get) error = %v", err)
	}
	if detail.Task.Status != taskpkg.TaskStatusCompleted || len(detail.Runs) != 1 || detail.Runs[0].SessionID == "" {
		t.Fatalf("task detail = %#v, want completed task with persisted run", detail)
	}
}

func assertDetachedHarnessMetadata(t *testing.T, label string, metadata json.RawMessage) {
	t.Helper()

	var decoded map[string]string
	if err := json.Unmarshal(metadata, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v; metadata=%s", label, err, string(metadata))
	}
	if got, want := decoded["schema"], "compozy.harness.detached.v1"; got != want || len(decoded) != 1 {
		t.Fatalf("%s = %#v, want schema %q only", label, decoded, want)
	}
}

func TestCLIHistoricalChannelTaskLeaseAfterDaemonRestartIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"workspace",
		"add",
		h.workspace,
		"--name",
		"alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}

	const channel = "history-run-start"
	createOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"create",
		"--scope",
		"workspace",
		"--workspace",
		"alpha",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		channel,
		"--title",
		"CLI historical lease restart",
		"-o",
		"json",
	)
	var created TaskRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(task create) error = %v", err)
	}
	if created.ID == "" {
		t.Fatalf("created = %#v, want historical task id", created)
	}

	enqueueOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"run",
		"enqueue",
		created.ID,
		"--idempotency-key",
		"idem-history-run-start",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		channel,
		"-o",
		"json",
	)
	var enqueued TaskRunRecord
	if err := json.Unmarshal([]byte(enqueueOut), &enqueued); err != nil {
		t.Fatalf("json.Unmarshal(task run enqueue) error = %v", err)
	}
	if enqueued.Status != taskpkg.TaskRunStatusQueued ||
		resolvedParticipationChannelID(enqueued.ResolvedNetworkParticipation) != channel {
		t.Fatalf("enqueued = %#v, want queued historical run", enqueued)
	}

	var worker SessionRecord
	t.Run(
		"Should claim and complete the historical run through one agent lease after daemon restart",
		func(t *testing.T) {
			if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
				t.Fatalf("daemon stop before restart error = %v", err)
			}
			if err := h.runner.waitForExit(); err != nil {
				t.Fatalf("waitForExit(before restart) error = %v", err)
			}
			mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")

			agentDeps, claimedWorker := newIntegrationAgentCommandDeps(
				t,
				h,
				"history-lease-worker",
				"alpha",
				channel,
			)
			worker = claimedWorker
			claimOut := mustExecuteRoot(t, agentDeps, "task", "next", "--run-id", enqueued.ID, "-o", "json")
			var next AgentTaskNextRecord
			if err := json.Unmarshal([]byte(claimOut), &next); err != nil {
				t.Fatalf("json.Unmarshal(task next exact claim) error = %v", err)
			}
			if !next.Claimed || next.Claim == nil || next.Claim.Run.ID != enqueued.ID ||
				next.Claim.Run.Status != taskpkg.TaskRunStatusClaimed ||
				resolvedParticipationChannelID(next.Claim.Run.ResolvedNetworkParticipation) != channel {
				t.Fatalf("task next = %#v, want exact claimed historical run", next)
			}

			getOut := mustExecuteRoot(t, h.deps, "task", "get", created.ID, "-o", "json")
			var detail TaskDetailRecord
			if err := json.Unmarshal([]byte(getOut), &detail); err != nil {
				t.Fatalf("json.Unmarshal(task get) error = %v", err)
			}
			if detail.Task.Status != taskpkg.TaskStatusReady {
				t.Fatalf(
					"detail.Task.Status = %q, want %q while the lease is claimed",
					detail.Task.Status,
					taskpkg.TaskStatusReady,
				)
			}
			if got, want := len(detail.Runs), 1; got != want {
				t.Fatalf("len(detail.Runs) = %d, want %d", got, want)
			}
			if detail.Runs[0].SessionID != worker.ID ||
				resolvedParticipationChannelID(detail.Runs[0].ResolvedNetworkParticipation) != channel {
				t.Fatalf("detail.Runs[0] = %#v, want claimed historical lease persisted", detail.Runs[0])
			}

			completeOut := mustExecuteRoot(
				t,
				agentDeps,
				"task",
				"complete",
				enqueued.ID,
				"--result",
				`{"ok":true,"path":"cli-historical-run-start-restart"}`,
				"-o",
				"json",
			)
			var completed AgentTaskLeaseRecord
			if err := json.Unmarshal([]byte(completeOut), &completed); err != nil {
				t.Fatalf("json.Unmarshal(task complete) error = %v", err)
			}
			if completed.Status != taskpkg.TaskRunStatusCompleted ||
				completed.RunID != enqueued.ID ||
				completed.SessionID != worker.ID ||
				resolvedParticipationChannelID(completed.ResolvedNetworkParticipation) != channel {
				t.Fatalf("completed = %#v, want completed historical lease", completed)
			}

			stopOut := mustExecuteRoot(t, h.deps, "session", "stop", worker.ID, "-o", "json")
			var stoppedWorker SessionRecord
			if err := json.Unmarshal([]byte(stopOut), &stoppedWorker); err != nil {
				t.Fatalf("json.Unmarshal(session stop worker) error = %v", err)
			}
			if stoppedWorker.State != session.StateStopped {
				t.Fatalf("stopped worker = %#v, want stopped", stoppedWorker)
			}

		},
	)

	t.Run("Should persist the completed manual run and leave no active sessions", func(t *testing.T) {
		getOut := mustExecuteRoot(t, h.deps, "task", "get", created.ID, "-o", "json")
		var detail TaskDetailRecord
		if err := json.Unmarshal([]byte(getOut), &detail); err != nil {
			t.Fatalf("json.Unmarshal(task get after complete) error = %v", err)
		}
		if detail.Task.Status != taskpkg.TaskStatusCompleted {
			t.Fatalf("detail.Task = %#v, want completed historical task", detail.Task)
		}
		if got, want := len(detail.Runs), 1; got != want {
			t.Fatalf("len(detail.Runs after complete) = %d, want %d", got, want)
		}
		if detail.Runs[0].Status != taskpkg.TaskRunStatusCompleted ||
			resolvedParticipationChannelID(detail.Runs[0].ResolvedNetworkParticipation) != channel {
			t.Fatalf("detail.Runs[0] = %#v, want completed historical run", detail.Runs[0])
		}

		assertNoActiveSessions(t, h.deps)
	})
}

func TestCLIAgentTaskLeaseLifecycleIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"workspace",
		"add",
		h.workspace,
		"--name",
		"alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}
	sessionOut := mustExecuteRoot(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"agent-worker",
		"--workspace",
		"alpha",
		"-o",
		"json",
	)
	var worker SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &worker); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if worker.ID == "" || worker.WorkspaceID == "" || worker.State != session.StateActive {
		t.Fatalf("worker = %#v, want active workspace session", worker)
	}
	agentSessionID := worker.ID
	agentDeps := h.deps
	agentDeps.getenv = func(key string) string {
		switch key {
		case agentidentity.EnvSessionID:
			return agentSessionID
		case agentidentity.EnvAgent:
			return worker.AgentName
		default:
			return ""
		}
	}

	createOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"create",
		"--scope",
		"workspace",
		"--workspace",
		"alpha",
		"--title",
		"Agent lease task",
		"-o",
		"json",
	)
	var created TaskRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(task create) error = %v", err)
	}
	enqueueOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"run",
		"enqueue",
		created.ID,
		"--idempotency-key",
		"idem-agent-lease",
		"-o",
		"json",
	)
	var enqueued TaskRunRecord
	if err := json.Unmarshal([]byte(enqueueOut), &enqueued); err != nil {
		t.Fatalf("json.Unmarshal(task run enqueue) error = %v", err)
	}
	if enqueued.Status != taskpkg.TaskRunStatusQueued {
		t.Fatalf("enqueued = %#v, want queued", enqueued)
	}

	var next AgentTaskNextRecord

	t.Run("Should claim the exact local task run without a fictional channel", func(t *testing.T) {
		nextOut := mustExecuteRoot(
			t,
			agentDeps,
			"task",
			"next",
			"--run-id",
			enqueued.ID,
			"-o",
			"json",
		)
		if err := json.Unmarshal([]byte(nextOut), &next); err != nil {
			t.Fatalf("json.Unmarshal(task next) error = %v", err)
		}
		if !next.Claimed ||
			next.Claim == nil ||
			next.Claim.Lease.ClaimTokenHash == "" ||
			next.Claim.Run.ID != enqueued.ID ||
			next.Claim.CoordinationChannel != nil {
			t.Fatalf("next = %#v, want claimed local run with lease hash and no channel", next)
		}
		if strings.Contains(nextOut, `"claim_token"`) || strings.Contains(nextOut, "compozy_claim_") {
			t.Fatal("task next output exposed raw claim token")
		}
	})

	t.Run("Should renew the claimed lease", func(t *testing.T) {
		heartbeatOut := mustExecuteRoot(
			t,
			agentDeps,
			"task",
			"heartbeat",
			enqueued.ID,
			"--lease-seconds",
			"60",
			"-o",
			"json",
		)
		if strings.Contains(heartbeatOut, `"claim_token"`) || strings.Contains(heartbeatOut, "compozy_claim_") {
			t.Fatal("heartbeat output exposed raw claim token")
		}
		var heartbeat AgentTaskLeaseRecord
		if err := json.Unmarshal([]byte(heartbeatOut), &heartbeat); err != nil {
			t.Fatalf("json.Unmarshal(task heartbeat) error = %v", err)
		}
		if heartbeat.RunID != enqueued.ID || heartbeat.Status != taskpkg.TaskRunStatusClaimed ||
			heartbeat.LeaseUntil == nil {
			t.Fatalf("heartbeat = %#v, want renewed claimed lease", heartbeat)
		}
	})

	t.Run("Should complete the claimed task and reject token reuse", func(t *testing.T) {
		completeOut := mustExecuteRoot(
			t,
			agentDeps,
			"task",
			"complete",
			enqueued.ID,
			"--result",
			`{"ok":true}`,
			"-o",
			"json",
		)
		if strings.Contains(completeOut, `"claim_token"`) || strings.Contains(completeOut, "compozy_claim_") {
			t.Fatal("complete output exposed raw claim token")
		}
		var completed AgentTaskLeaseRecord
		if err := json.Unmarshal([]byte(completeOut), &completed); err != nil {
			t.Fatalf("json.Unmarshal(task complete) error = %v", err)
		}
		if completed.Status != taskpkg.TaskRunStatusCompleted || completed.RunID != enqueued.ID {
			t.Fatalf("completed = %#v, want completed leased run", completed)
		}

		exitCode, _, stderr := executeRootCommandWithExit(
			t,
			agentDeps,
			"task",
			"complete",
			enqueued.ID,
			"--result",
			`{"ok":true}`,
			"-o",
			"json",
		)
		if exitCode == 0 {
			t.Fatal("second task complete exit code = 0, want stale token/lifecycle rejection")
		}
		if !strings.Contains(stderr, "not an active lease") {
			t.Fatal("second complete stderr did not include inactive lease rejection")
		}
		if strings.Contains(stderr, `"claim_token"`) || strings.Contains(stderr, "compozy_claim_") {
			t.Fatal("second complete stderr leaked raw claim token")
		}
	})

	t.Run("Should return structured no-work result", func(t *testing.T) {
		noWorkOut := mustExecuteRoot(t, agentDeps, "task", "next", "-o", "json")
		var noWork AgentTaskNextRecord
		if err := json.Unmarshal([]byte(noWorkOut), &noWork); err != nil {
			t.Fatalf("json.Unmarshal(task next no-work) error = %v", err)
		}
		if noWork.Claimed || noWork.Claim != nil {
			t.Fatalf("noWork = %#v, want structured no-work result", noWork)
		}
	})

	t.Run("Should recover stale lease and reject stale token", func(t *testing.T) {
		staleCreateOut := mustExecuteRoot(
			t,
			h.deps,
			"task",
			"create",
			"--scope",
			"workspace",
			"--workspace",
			"alpha",
			"--title",
			"Agent stale lease task",
			"-o",
			"json",
		)
		var staleTask TaskRecord
		if err := json.Unmarshal([]byte(staleCreateOut), &staleTask); err != nil {
			t.Fatalf("json.Unmarshal(stale task create) error = %v", err)
		}
		staleEnqueueOut := mustExecuteRoot(
			t,
			h.deps,
			"task",
			"run",
			"enqueue",
			staleTask.ID,
			"--idempotency-key",
			"idem-agent-stale-lease",
			"-o",
			"json",
		)
		var staleRun TaskRunRecord
		if err := json.Unmarshal([]byte(staleEnqueueOut), &staleRun); err != nil {
			t.Fatalf("json.Unmarshal(stale run enqueue) error = %v", err)
		}
		staleNextOut := mustExecuteRoot(
			t,
			agentDeps,
			"task",
			"next",
			"--run-id",
			staleRun.ID,
			"--lease-seconds",
			"1",
			"-o",
			"json",
		)
		var staleNext AgentTaskNextRecord
		if err := json.Unmarshal([]byte(staleNextOut), &staleNext); err != nil {
			t.Fatalf("json.Unmarshal(stale task next) error = %v", err)
		}
		if !staleNext.Claimed || staleNext.Claim == nil || staleNext.Claim.Run.ID != staleRun.ID {
			t.Fatalf("staleNext = %#v, want claimed stale-test run", staleNext)
		}
		if strings.Contains(staleNextOut, `"claim_token"`) || strings.Contains(staleNextOut, "compozy_claim_") {
			t.Fatal("stale task next output exposed raw claim token")
		}
		if staleNext.Claim.Lease.LeaseUntil == nil {
			t.Fatal("staleNext.Claim.Lease.LeaseUntil = nil, want bounded lease expiry")
		}
		waitUntilLeaseExpires(t, *staleNext.Claim.Lease.LeaseUntil, 3*time.Second)

		for _, tt := range []struct {
			name string
			args []string
		}{
			{
				name: "heartbeat",
				args: []string{
					"task",
					"heartbeat",
					staleRun.ID,
					"-o",
					"json",
				},
			},
			{
				name: "fail",
				args: []string{
					"task",
					"fail",
					staleRun.ID,
					"--error",
					"stale holder",
					"-o",
					"json",
				},
			},
		} {
			t.Run("Should reject expired "+tt.name+" mutation", func(t *testing.T) {
				exitCode, _, stderr := executeRootCommandWithExit(t, agentDeps, tt.args...)
				if exitCode == 0 {
					t.Fatalf("task %s after expiry exit code = 0, want lease expiry rejection", tt.name)
				}
				if !strings.Contains(stderr, "lease expired") {
					t.Fatalf("task %s after expiry stderr = %q, want lease expired rejection", tt.name, stderr)
				}
				if strings.Contains(stderr, `"claim_token"`) || strings.Contains(stderr, "compozy_claim_") {
					t.Fatalf("task %s after expiry leaked raw claim token", tt.name)
				}
			})
		}
	})
}

func TestCLIHistoricalChannelTaskNextAfterDaemonRestartIntegration(t *testing.T) {
	t.Parallel()

	h := newIntegrationHarness(t)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	if _, _, err := executeRootCommand(
		t,
		h.deps,
		"workspace",
		"add",
		h.workspace,
		"--name",
		"alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}

	const channel = "history-builders"
	sessionOut := mustExecuteRoot(
		t,
		h.deps,
		"session",
		"new",
		"--agent",
		"coder",
		"--name",
		"history-worker",
		"--workspace",
		"alpha",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		channel,
		"-o",
		"json",
	)
	var worker SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &worker); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if worker.ID == "" || worker.State != session.StateActive ||
		resolvedParticipationChannelID(worker.ResolvedNetworkParticipation) != channel {
		t.Fatalf("worker = %#v, want active worker on %q", worker, channel)
	}
	agentSessionID := worker.ID

	stopOut := mustExecuteRoot(t, h.deps, "session", "stop", worker.ID, "-o", "json")
	var stopped SessionRecord
	if err := json.Unmarshal([]byte(stopOut), &stopped); err != nil {
		t.Fatalf("json.Unmarshal(session stop) error = %v", err)
	}
	if stopped.State != session.StateStopped ||
		resolvedParticipationChannelID(stopped.ResolvedNetworkParticipation) != channel {
		t.Fatalf("stopped = %#v, want stopped worker on %q", stopped, channel)
	}

	t.Run("Should keep stopped participation historical without listing an active channel", func(t *testing.T) {
		assertCLIHistoricalChannelNotActive(t, h.deps, "", channel)
	})

	createOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"create",
		"--scope",
		"workspace",
		"--workspace",
		"alpha",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		channel,
		"--title",
		"CLI historical restart claim",
		"-o",
		"json",
	)
	var created TaskRecord
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(task create) error = %v", err)
	}
	if created.ID == "" {
		t.Fatalf("created = %#v, want historical task id", created)
	}

	enqueueOut := mustExecuteRoot(
		t,
		h.deps,
		"task",
		"run",
		"enqueue",
		created.ID,
		"--idempotency-key",
		"idem-cli-historical-restart",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		channel,
		"-o",
		"json",
	)
	var enqueued TaskRunRecord
	if err := json.Unmarshal([]byte(enqueueOut), &enqueued); err != nil {
		t.Fatalf("json.Unmarshal(task run enqueue) error = %v", err)
	}
	if enqueued.Status != taskpkg.TaskRunStatusQueued ||
		resolvedParticipationChannelID(enqueued.ResolvedNetworkParticipation) != channel {
		t.Fatalf("enqueued = %#v, want queued run bound to historical channel", enqueued)
	}

	agentDeps := h.deps
	agentDeps.getenv = func(key string) string {
		switch key {
		case agentidentity.EnvSessionID:
			return agentSessionID
		case agentidentity.EnvAgent:
			return worker.AgentName
		default:
			return ""
		}
	}

	t.Run("Should reclaim and complete the historical run after daemon restart", func(t *testing.T) {
		if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
			t.Fatalf("daemon stop before restart error = %v", err)
		}
		if err := h.runner.waitForExit(); err != nil {
			t.Fatalf("waitForExit(before restart) error = %v", err)
		}
		mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")

		resumeOut := mustExecuteRoot(
			t,
			h.deps,
			"session",
			"new",
			"--agent",
			"coder",
			"--name",
			"history-worker-restarted",
			"--workspace",
			"alpha",
			"--network",
			"live",
			"--network-channel-strategy",
			"named",
			"--network-channel",
			channel,
			"-o",
			"json",
		)
		var resumed SessionRecord
		if err := json.Unmarshal([]byte(resumeOut), &resumed); err != nil {
			t.Fatalf("json.Unmarshal(session new after restart) error = %v", err)
		}
		if resumed.State != session.StateActive ||
			resolvedParticipationChannelID(resumed.ResolvedNetworkParticipation) != channel {
			t.Fatalf("resumed = %#v, want active restarted worker on %q", resumed, channel)
		}
		agentSessionID = resumed.ID

		nextOut := mustExecuteRoot(t, agentDeps, "task", "next", "--lease-seconds", "60", "-o", "json")
		var next AgentTaskNextRecord
		if err := json.Unmarshal([]byte(nextOut), &next); err != nil {
			t.Fatalf("json.Unmarshal(task next) error = %v", err)
		}
		if !next.Claimed || next.Claim == nil {
			t.Fatalf("next = %#v, want claimed historical run", next)
		}
		if got, want := next.Claim.Run.ID, enqueued.ID; got != want {
			t.Fatalf("next.Claim.Run.ID = %q, want %q", got, want)
		}
		if resolvedParticipationChannelID(next.Claim.Run.ResolvedNetworkParticipation) != channel {
			t.Fatalf("next.Claim.Run = %#v, want historical channel preserved", next.Claim.Run)
		}
		if next.Claim.CoordinationChannel == nil {
			t.Fatal("next.Claim.CoordinationChannel = nil, want historical coordination channel")
		}
		if got, want := firstCLIValue(
			next.Claim.CoordinationChannel.ID,
			next.Claim.CoordinationChannel.ID,
		), channel; got != want {
			t.Fatalf("coordination channel = %q, want %q", got, want)
		}
		if next.Claim.Lease.ClaimTokenHash == "" {
			t.Fatal("next.Claim.Lease.ClaimTokenHash = empty, want observability hash")
		}
		if strings.Contains(nextOut, `"claim_token"`) || strings.Contains(nextOut, "compozy_claim_") {
			t.Fatal("task next output exposed raw claim token")
		}

		completeOut := mustExecuteRoot(
			t,
			agentDeps,
			"task",
			"complete",
			enqueued.ID,
			"--result",
			`{"ok":true,"path":"cli-historical-restart"}`,
			"-o",
			"json",
		)
		if strings.Contains(completeOut, `"claim_token"`) || strings.Contains(completeOut, "compozy_claim_") {
			t.Fatal("task complete output exposed raw claim token")
		}
		var completed AgentTaskLeaseRecord
		if err := json.Unmarshal([]byte(completeOut), &completed); err != nil {
			t.Fatalf("json.Unmarshal(task complete) error = %v", err)
		}
		if completed.Status != taskpkg.TaskRunStatusCompleted ||
			completed.RunID != enqueued.ID ||
			resolvedParticipationChannelID(completed.ResolvedNetworkParticipation) != channel {
			t.Fatalf("completed = %#v, want completed historical lease", completed)
		}
	})

	t.Run("Should persist the completed historical run and leave no active sessions", func(t *testing.T) {
		getOut := mustExecuteRoot(t, h.deps, "task", "get", created.ID, "-o", "json")
		var detail TaskDetailRecord
		if err := json.Unmarshal([]byte(getOut), &detail); err != nil {
			t.Fatalf("json.Unmarshal(task get) error = %v", err)
		}
		if detail.Task.Status != taskpkg.TaskStatusCompleted {
			t.Fatalf("detail.Task = %#v, want completed task", detail.Task)
		}
		if got, want := len(detail.Runs), 1; got != want {
			t.Fatalf("len(detail.Runs) = %d, want %d", got, want)
		}
		if detail.Runs[0].Status != taskpkg.TaskRunStatusCompleted ||
			detail.Runs[0].SessionID != agentSessionID ||
			resolvedParticipationChannelID(detail.Runs[0].ResolvedNetworkParticipation) != channel {
			t.Fatalf("detail.Runs[0] = %#v, want completed persisted historical run", detail.Runs[0])
		}

		stopOut := mustExecuteRoot(t, h.deps, "session", "stop", agentSessionID, "-o", "json")
		var stoppedAfterResume SessionRecord
		if err := json.Unmarshal([]byte(stopOut), &stoppedAfterResume); err != nil {
			t.Fatalf("json.Unmarshal(session stop after resume) error = %v", err)
		}
		if stoppedAfterResume.State != session.StateStopped ||
			resolvedParticipationChannelID(stoppedAfterResume.ResolvedNetworkParticipation) != channel {
			t.Fatalf("stoppedAfterResume = %#v, want stopped resumed worker on %q", stoppedAfterResume, channel)
		}

		assertCLIHistoricalChannelNotActive(t, h.deps, "", channel)

		assertNoActiveSessions(t, h.deps)
	})
}

func assertNoActiveSessions(t *testing.T, deps commandDeps) {
	t.Helper()

	statusOut := mustExecuteRoot(t, deps, "status", "-o", "json")
	var status StatusRecord
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("json.Unmarshal(status) error = %v", err)
	}
	if status.Sessions.Active != 0 {
		t.Fatalf("status.Sessions.Active = %d, want 0", status.Sessions.Active)
	}
}

type integrationHarness struct {
	deps      commandDeps
	homePaths compozyconfig.HomePaths
	workspace string
	runner    *integrationDaemon
}

func registerIntegrationHarnessCleanup(t *testing.T, h integrationHarness) {
	t.Helper()

	t.Cleanup(func() {
		if err := h.stopAndWait(t); err != nil {
			t.Errorf("integration daemon cleanup error = %v", err)
		}
	})
}

func (h integrationHarness) stopAndWait(t *testing.T) error {
	t.Helper()

	var cleanupErr error
	forceStop := func() {
		if signalErr := h.runner.signalProcess(h.runner.pid, syscall.SIGTERM); signalErr != nil {
			cleanupErr = errors.Join(
				cleanupErr,
				fmt.Errorf("force integration daemon shutdown: %w", signalErr),
			)
		}
	}

	if h.runner.processAlive(h.runner.pid) {
		_, infoErr := h.deps.readDaemonInfo(h.homePaths.DaemonInfo)
		switch {
		case errors.Is(infoErr, os.ErrNotExist):
			forceStop()
		case infoErr != nil:
			cleanupErr = fmt.Errorf("read integration daemon info: %w", infoErr)
			forceStop()
		default:
			_, stderr, stopErr := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json")
			if stopErr != nil {
				cleanupErr = fmt.Errorf("stop integration daemon: %w; stderr=%s", stopErr, stderr)
				if h.runner.processAlive(h.runner.pid) {
					forceStop()
				}
			}
		}
	}

	if waitErr := h.runner.waitForExitWithin(h.deps.stopTimeout); waitErr != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("wait for integration daemon exit: %w", waitErr))
		if h.runner.processAlive(h.runner.pid) {
			forceStop()
			if forcedWaitErr := h.runner.waitForExitWithin(h.deps.stopTimeout); forcedWaitErr != nil {
				cleanupErr = errors.Join(
					cleanupErr,
					fmt.Errorf("wait for forced integration daemon exit: %w", forcedWaitErr),
				)
			}
		}
	}
	return cleanupErr
}

func newIntegrationAgentCommandDeps(
	t *testing.T,
	h integrationHarness,
	name string,
	workspace string,
	channel string,
) (commandDeps, SessionRecord) {
	t.Helper()

	args := []string{"session", "new", "--agent", "coder", "--name", name}
	if workspace != "" {
		args = append(args, "--workspace", workspace)
	} else {
		args = append(args, "--cwd", h.workspace)
	}
	if channel != "" {
		args = append(
			args,
			"--network", "live",
			"--network-channel-strategy", "named",
			"--network-channel", channel,
		)
	}
	args = append(args, "-o", "json")

	sessionOut := mustExecuteRoot(t, h.deps, args...)
	var worker SessionRecord
	if err := json.Unmarshal([]byte(sessionOut), &worker); err != nil {
		t.Fatalf("json.Unmarshal(agent session) error = %v", err)
	}
	if worker.ID == "" || worker.AgentName == "" || worker.State != session.StateActive {
		t.Fatalf("agent session = %#v, want active agent identity", worker)
	}

	agentDeps := h.deps
	agentDeps.getenv = func(key string) string {
		switch key {
		case agentidentity.EnvSessionID:
			return worker.ID
		case agentidentity.EnvAgent:
			return worker.AgentName
		default:
			return ""
		}
	}
	return agentDeps, worker
}

type integrationDreamTrigger struct {
	enabled   bool
	triggered bool
	reason    string
	last      time.Time
}

func (t *integrationDreamTrigger) Trigger(context.Context, string) (bool, string, error) {
	return t.triggered, t.reason, nil
}

func (t *integrationDreamTrigger) LastConsolidatedAt() (time.Time, error) {
	return t.last, nil
}

func (t *integrationDreamTrigger) Enabled() bool {
	return t.enabled
}

type integrationSoulRunActivityChecker struct{}

func (integrationSoulRunActivityChecker) HasActiveRunForSession(context.Context, string, time.Time) (bool, error) {
	return false, nil
}

type integrationDaemon struct {
	t         *testing.T
	homePaths compozyconfig.HomePaths
	cfg       compozyconfig.Config
	pid       int
	startedAt time.Time

	mu       sync.Mutex
	running  bool
	cancel   context.CancelFunc
	exitDone chan struct{}
	exitErr  error

	bridges          *integrationBridgeService
	bridgeProviders  []bridgepkg.BridgeProvider
	driver           *integrationDriver
	manager          *session.Manager
	tasks            core.TaskService
	observer         core.Observer
	extensionSources []registrypkg.Source
	extensionTrust   *extensionpkg.MarketplaceTrustEvidence
}

type integrationDaemonProcess struct {
	pid       int
	done      <-chan struct{}
	waitCh    <-chan error
	terminate context.CancelFunc
}

func (d *integrationDaemon) extensionMarketplaceLoader() extensionpkg.MarketplaceSourceLoader {
	return func(context.Context) ([]registrypkg.Source, error) {
		sources := append([]registrypkg.Source(nil), d.extensionSources...)
		if len(sources) == 0 {
			return nil, errors.New("integration extension marketplace source is not configured")
		}
		return sources, nil
	}
}

type integrationExtensionService struct {
	homePaths                        compozyconfig.HomePaths
	registry                         *extensionpkg.Registry
	manager                          *extensionpkg.Manager
	marketplaceLoader                extensionpkg.MarketplaceSourceLoader
	marketplacePolicyAllowUnverified bool
	marketplaceTrust                 *extensionpkg.MarketplaceTrustEvidence
}

func (s *integrationExtensionService) Search(
	context.Context,
	contract.ExtensionSearchRequest,
) (contract.ExtensionSearchResponse, error) {
	return contract.ExtensionSearchResponse{}, nil
}

func (s *integrationExtensionService) UpdateBatch(
	context.Context,
	contract.UpdateExtensionsRequest,
	taskpkg.ActorContext,
) ([]contract.ManagedExtensionUpdatePayload, error) {
	return nil, nil
}

type integrationBridgeSecretStore interface {
	ListBridgeSecretBindings(context.Context, string) ([]bridgepkg.BridgeSecretBinding, error)
	PutBridgeSecretBinding(context.Context, bridgepkg.BridgeSecretBinding) error
	DeleteBridgeSecretBinding(context.Context, string, string) error
}

type integrationBridgeCatalogStore interface {
	CountBridgeRoutes(context.Context, []string) (map[string]int, error)
	ListBridgeSecretBindingsForInstances(
		context.Context,
		[]string,
	) (map[string][]bridgepkg.BridgeSecretBinding, error)
}

type integrationBridgeService struct {
	*bridgepkg.Service
	store             integrationBridgeSecretStore
	catalogStore      integrationBridgeCatalogStore
	taskSubscriptions bridgepkg.BridgeTaskSubscriptionStore
	providers         []bridgepkg.BridgeProvider
	checkBridgeFn     func(context.Context, string, bridgepkg.BridgeCheckRequest) (bridgepkg.BridgeCheckResponse, error)
	registerWebhookFn func(context.Context, string, bridgepkg.BridgeWebhookRegistrationRequest) (bridgepkg.BridgeWebhookRegistrationResponse, error)
	deliverBridgeFn   func(context.Context, string, bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error)
}

var _ core.BridgeService = (*integrationBridgeService)(nil)

type integrationNotifierFanout struct {
	notifiers []session.Notifier
}

type integrationDriver struct {
	mu       sync.Mutex
	nextPID  int
	nextSess int
	states   map[*session.AgentProcess]chan struct{}
	blocked  map[string]chan struct{}
}

type integrationTaskExecutor struct {
	mu   sync.Mutex
	next int
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func newIntegrationBridgeService(
	store bridgepkg.RegistryStore,
	providers []bridgepkg.BridgeProvider,
) *integrationBridgeService {
	secretStore, ok := store.(integrationBridgeSecretStore)
	if !ok {
		secretStore = nil
	}
	catalogStore, catalogStoreOK := store.(integrationBridgeCatalogStore)
	if !catalogStoreOK {
		catalogStore = nil
	}
	taskSubscriptions, taskSubscriptionsOK := store.(bridgepkg.BridgeTaskSubscriptionStore)
	if !taskSubscriptionsOK {
		taskSubscriptions = nil
	}
	return &integrationBridgeService{
		Service:           bridgepkg.NewRegistry(store),
		store:             secretStore,
		catalogStore:      catalogStore,
		taskSubscriptions: taskSubscriptions,
		providers:         append([]bridgepkg.BridgeProvider(nil), providers...),
	}
}

func (s *integrationBridgeService) DeliveryMetrics() map[string]bridgepkg.BridgeDeliveryMetrics {
	if s == nil {
		return nil
	}
	return nil
}

func (s *integrationBridgeService) DeliveryMetricsFor(
	[]string,
) (map[string]bridgepkg.BridgeDeliveryMetrics, error) {
	return nil, nil
}

func (s *integrationBridgeService) CountBridgeRoutes(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (map[string]int, error) {
	if s == nil || s.catalogStore == nil {
		return nil, errors.New("integration bridge catalog store is not configured")
	}
	return s.catalogStore.CountBridgeRoutes(ctx, bridgeInstanceIDs)
}

func (s *integrationBridgeService) ListSecretBindingsForInstances(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (map[string][]bridgepkg.BridgeSecretBinding, error) {
	if s == nil || s.catalogStore == nil {
		return nil, errors.New("integration bridge catalog store is not configured")
	}
	return s.catalogStore.ListBridgeSecretBindingsForInstances(ctx, bridgeInstanceIDs)
}

func (s *integrationBridgeService) StartInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	return s.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:      id,
		Enabled: true,
		Status:  bridgepkg.BridgeStatusStarting,
	})
}

func (s *integrationBridgeService) StopInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	return s.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:      id,
		Enabled: false,
		Status:  bridgepkg.BridgeStatusDisabled,
	})
}

func (s *integrationBridgeService) RestartInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	return s.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:      id,
		Enabled: true,
		Status:  bridgepkg.BridgeStatusStarting,
	})
}

func (s *integrationBridgeService) ListProviders(context.Context) ([]bridgepkg.BridgeProvider, error) {
	return append([]bridgepkg.BridgeProvider(nil), s.providers...), nil
}

func (s *integrationBridgeService) CheckBridge(
	ctx context.Context,
	extensionName string,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	if s.checkBridgeFn != nil {
		return s.checkBridgeFn(ctx, extensionName, request)
	}
	return bridgepkg.BridgeCheckResponse{}, bridgepkg.ErrBridgeControlTransportUnavailable
}

func (s *integrationBridgeService) RegisterBridgeWebhook(
	ctx context.Context,
	extensionName string,
	request bridgepkg.BridgeWebhookRegistrationRequest,
) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
	if s.registerWebhookFn != nil {
		return s.registerWebhookFn(ctx, extensionName, request)
	}
	return bridgepkg.BridgeWebhookRegistrationResponse{}, bridgepkg.ErrBridgeControlTransportUnavailable
}

func (s *integrationBridgeService) DeliverBridge(
	ctx context.Context,
	extensionName string,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	if s.deliverBridgeFn != nil {
		return s.deliverBridgeFn(ctx, extensionName, request)
	}
	return bridgepkg.DeliveryAck{}, bridgepkg.ErrDeliveryTransportUnavailable
}

func (s *integrationBridgeService) ListSecretBindings(
	ctx context.Context,
	bridgeInstanceID string,
) ([]bridgepkg.BridgeSecretBinding, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("integration bridge secret store is not configured")
	}
	return s.store.ListBridgeSecretBindings(ctx, bridgeInstanceID)
}

func (s *integrationBridgeService) PutSecretBinding(
	ctx context.Context,
	binding bridgepkg.BridgeSecretBinding,
	_ *string,
) error {
	if s == nil || s.store == nil {
		return errors.New("integration bridge secret store is not configured")
	}
	return s.store.PutBridgeSecretBinding(ctx, binding)
}

func (s *integrationBridgeService) DeleteSecretBinding(
	ctx context.Context,
	bridgeInstanceID string,
	bindingName string,
) error {
	if s == nil || s.store == nil {
		return errors.New("integration bridge secret store is not configured")
	}
	return s.store.DeleteBridgeSecretBinding(ctx, bridgeInstanceID, bindingName)
}

func (s *integrationBridgeService) PutBridgeTaskSubscription(
	ctx context.Context,
	subscription bridgepkg.BridgeTaskSubscription,
) error {
	if s == nil || s.taskSubscriptions == nil {
		return errors.New("integration bridge task subscription store is not configured")
	}
	return s.taskSubscriptions.PutBridgeTaskSubscription(ctx, subscription)
}

func (s *integrationBridgeService) GetBridgeTaskSubscription(
	ctx context.Context,
	subscriptionID string,
) (bridgepkg.BridgeTaskSubscription, error) {
	if s == nil || s.taskSubscriptions == nil {
		return bridgepkg.BridgeTaskSubscription{}, errors.New(
			"integration bridge task subscription store is not configured",
		)
	}
	return s.taskSubscriptions.GetBridgeTaskSubscription(ctx, subscriptionID)
}

func (s *integrationBridgeService) ListBridgeTaskSubscriptions(
	ctx context.Context,
	query bridgepkg.BridgeTaskSubscriptionQuery,
) ([]bridgepkg.BridgeTaskSubscription, error) {
	if s == nil || s.taskSubscriptions == nil {
		return nil, errors.New("integration bridge task subscription store is not configured")
	}
	return s.taskSubscriptions.ListBridgeTaskSubscriptions(ctx, query)
}

func (s *integrationBridgeService) DeleteBridgeTaskSubscription(ctx context.Context, subscriptionID string) error {
	if s == nil || s.taskSubscriptions == nil {
		return errors.New("integration bridge task subscription store is not configured")
	}
	return s.taskSubscriptions.DeleteBridgeTaskSubscription(ctx, subscriptionID)
}

func (s *integrationExtensionService) List(ctx context.Context) ([]contract.ExtensionPayload, error) {
	infos, err := s.registry.List()
	if err != nil {
		return nil, err
	}

	items := make([]contract.ExtensionPayload, 0, len(infos))
	for _, info := range infos {
		item, err := s.Status(ctx, info.Name)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *integrationExtensionService) Install(
	ctx context.Context,
	req contract.InstallExtensionRequest,
	_ taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if req.Source != contract.InstallExtensionSourceLocalPath {
		sourceFilter := string(req.Source)
		if req.Source == contract.InstallExtensionSourceCurated {
			sourceFilter = ""
		}
		info, err := extensionpkg.InstallMarketplaceManaged(
			ctx,
			s.homePaths,
			s.registry,
			s.marketplaceLoader,
			extensionpkg.MarketplaceInstallRequest{
				Slug:                   req.Ref,
				SourceFilter:           sourceFilter,
				Version:                req.Version,
				Asset:                  req.Asset,
				PolicyAllowsUnverified: s.marketplacePolicyAllowUnverified,
				AllowUnverified:        req.AllowUnverified,
				InstalledBy:            "cli-integration",
				Trust:                  s.marketplaceTrust,
			},
		)
		if err != nil {
			return contract.ExtensionPayload{}, err
		}
		if err := s.manager.Reload(ctx); err != nil {
			return contract.ExtensionPayload{}, err
		}
		return s.Status(ctx, info.Name)
	}
	manifest, err := extensionpkg.LoadManifest(req.Ref)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := extensionpkg.ValidateUnverifiedSideLoad(
		manifest.Name,
		req.Ref,
		s.marketplacePolicyAllowUnverified,
		req.AllowUnverified,
	); err != nil {
		return contract.ExtensionPayload{}, err
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(req.Ref)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := extensionpkg.InstallLocalManaged(s.homePaths, s.registry, manifest, req.Ref, checksum); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.manager.Reload(ctx); err != nil {
		return contract.ExtensionPayload{}, err
	}
	return s.Status(ctx, manifest.Name)
}

func (s *integrationExtensionService) Update(
	ctx context.Context,
	name string,
	req contract.UpdateExtensionRequest,
	_ taskpkg.ActorContext,
) (contract.ManagedExtensionUpdatePayload, error) {
	items, err := extensionpkg.UpdateMarketplaceManaged(
		ctx,
		s.homePaths,
		s.registry,
		s.marketplaceLoader,
		extensionpkg.MarketplaceUpdateRequest{
			Names:                  []string{name},
			CheckOnly:              req.CheckOnly,
			Version:                req.Version,
			PolicyAllowsUnverified: s.marketplacePolicyAllowUnverified,
			AllowUnverified:        req.AllowUnverified,
			InstalledBy:            "cli-integration",
		},
		func(context.Context) error {
			return s.manager.Reload(ctx)
		},
	)
	if err != nil {
		return contract.ManagedExtensionUpdatePayload{}, err
	}
	if len(items) == 0 {
		return contract.ManagedExtensionUpdatePayload{}, extensionpkg.ErrExtensionNotFound
	}
	item := items[0]
	return contract.ManagedExtensionUpdatePayload{
		Name:           item.Name,
		Slug:           item.Slug,
		Registry:       item.Registry,
		CurrentVersion: item.CurrentVersion,
		LatestVersion:  item.LatestVersion,
		Path:           item.Path,
		Status:         item.Status,
	}, nil
}

func (s *integrationExtensionService) Remove(
	ctx context.Context,
	name string,
	_ taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	info, err := s.registry.Get(name)
	if err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	path := extensionpkg.ManagedInstallPath(s.homePaths, info.Name)
	if err := s.registry.Uninstall(name); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	if err := os.RemoveAll(path); err != nil {
		return contract.ManagedExtensionRemovePayload{}, fmt.Errorf("remove extension install %q: %w", path, err)
	}
	if err := s.manager.Reload(ctx); err != nil {
		return contract.ManagedExtensionRemovePayload{}, err
	}
	return contract.ManagedExtensionRemovePayload{Name: info.Name, Path: path, Status: "removed"}, nil
}

func (s *integrationExtensionService) Enable(
	ctx context.Context,
	name string,
	req contract.EnableExtensionRequest,
	_ taskpkg.ActorContext,
) (contract.ExtensionEnableResult, error) {
	confirmation, err := s.registry.NetworkConfirmation(extensionpkg.GlobalInstanceKey(name))
	if err != nil {
		return contract.ExtensionEnableResult{}, err
	}
	if confirmation.Digest != "" &&
		(strings.TrimSpace(confirmation.ConfirmedBy) == "" || confirmation.ConfirmedAt.IsZero()) {
		if strings.TrimSpace(req.ConfirmNetworkDigest) != confirmation.Digest {
			return contract.ExtensionEnableResult{}, &extensionpkg.NetworkConfirmationRequiredError{
				CurrentDigest: confirmation.Digest,
			}
		}
		if err := s.registry.ConfirmNetworkRequirement(
			extensionpkg.GlobalInstanceKey(name),
			confirmation.Digest,
			"cli-integration",
			time.Now().UTC(),
		); err != nil {
			return contract.ExtensionEnableResult{}, err
		}
	}
	if err := s.registry.Enable(name); err != nil {
		return contract.ExtensionEnableResult{}, err
	}
	if err := s.manager.Reload(ctx); err != nil {
		return contract.ExtensionEnableResult{}, err
	}
	item, err := s.Status(ctx, name)
	return contract.ExtensionEnableResult{Extension: item}, err
}

func (s *integrationExtensionService) Disable(
	ctx context.Context,
	name string,
	_ taskpkg.ActorContext,
) (contract.ExtensionPayload, error) {
	if err := s.registry.Disable(name); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if err := s.manager.Reload(ctx); err != nil {
		return contract.ExtensionPayload{}, err
	}
	return s.Status(ctx, name)
}

func (s *integrationExtensionService) Status(_ context.Context, name string) (contract.ExtensionPayload, error) {
	ext, err := s.manager.Get(name)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	if ext.Manifest == nil && strings.TrimSpace(ext.Info.ManifestPath) != "" {
		manifest, loadErr := extensionpkg.LoadManifest(filepath.Dir(ext.Info.ManifestPath))
		if loadErr == nil {
			ext.Manifest = manifest
		}
	}
	return extensionpkg.DescribeExtension(ext, true, time.Now().UTC()), nil
}

func (s *integrationExtensionService) Inventory(
	ctx context.Context,
	name string,
) (contract.ExtensionInventoryPayload, error) {
	if err := s.requireEmptyIntegrationExtensionKit(ctx, name); err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	status, err := s.Status(ctx, name)
	if err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	return contract.ExtensionInventoryPayload{
		Extension: status.Name,
		Enabled:   status.Enabled,
		Items:     []contract.ExtensionKitItemPayload{},
	}, nil
}

func (s *integrationExtensionService) Preview(
	ctx context.Context,
	name string,
) (contract.ExtensionEnablePreviewPayload, error) {
	if err := s.requireEmptyIntegrationExtensionKit(ctx, name); err != nil {
		return contract.ExtensionEnablePreviewPayload{}, err
	}
	status, err := s.Status(ctx, name)
	if err != nil {
		return contract.ExtensionEnablePreviewPayload{}, err
	}
	return contract.ExtensionEnablePreviewPayload{
		Extension:                   status.Name,
		Changes:                     []contract.ExtensionKitChangePayload{},
		AgentConflicts:              []string{},
		MissingEnv:                  append([]string(nil), status.MissingEnv...),
		AutomationStarting:          []string{},
		NetworkRequirementDigest:    status.NetworkRequirementDigest,
		NetworkConfirmationRequired: status.NetworkConfirmationRequired,
	}, nil
}

func (s *integrationExtensionService) requireEmptyIntegrationExtensionKit(ctx context.Context, name string) error {
	extension, err := s.manager.InspectPackageResources(ctx, name)
	if err != nil {
		return err
	}
	if extension == nil || extension.Manifest == nil {
		return errors.New("integration extension package inspection returned no manifest")
	}
	resources := extension.Manifest.Resources
	if len(extension.StaticAgents) > 0 || len(extension.Skills) > 0 || len(extension.Loops) > 0 ||
		len(extension.AutomationJobs) > 0 || len(extension.AutomationTriggers) > 0 || len(extension.Layouts) > 0 ||
		len(extension.Hooks) > 0 || len(resources.Tools) > 0 || len(resources.MCPServers) > 0 ||
		len(resources.CommandGroups) > 0 {
		return errors.New("integration extension kit resources are unsupported")
	}
	return nil
}

func (*integrationExtensionService) ListExtensionSecrets(
	context.Context,
	string,
	taskpkg.ActorContext,
) (contract.ExtensionSecretsPayload, error) {
	return contract.ExtensionSecretsPayload{}, errors.New("integration extension secrets are unsupported")
}

func (*integrationExtensionService) SetExtensionSecrets(
	context.Context,
	string,
	contract.SetExtensionSecretsRequest,
	taskpkg.ActorContext,
) (contract.ExtensionSecretsPayload, error) {
	return contract.ExtensionSecretsPayload{}, errors.New("integration extension secrets are unsupported")
}

func (*integrationExtensionService) DeleteExtensionSecret(
	context.Context,
	string,
	string,
	taskpkg.ActorContext,
) error {
	return errors.New("integration extension secrets are unsupported")
}

func (s *integrationExtensionService) Provenance(
	ctx context.Context,
	name string,
) (contract.ExtensionProvenancePayload, error) {
	status, err := s.Status(ctx, name)
	if err != nil {
		return contract.ExtensionProvenancePayload{}, err
	}
	if status.Provenance == nil {
		return contract.ExtensionProvenancePayload{}, errors.New("integration extension provenance is missing")
	}
	return *status.Provenance, nil
}

func (s *integrationExtensionService) MarketplaceTrust(
	_ context.Context,
	evidence extensionpkg.MarketplaceTrustEvidence,
) (contract.ExtensionTrustReportPayload, error) {
	return extensionpkg.MarketplaceEntryTrustReport(evidence, s.marketplacePolicyAllowUnverified)
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func newIntegrationHarness(t *testing.T) integrationHarness {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	socketPath := shortSocketPath(t)
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	workspace := t.TempDir()
	writeAgentDef(t, homePaths, "coder")

	cfg := compozyconfig.DefaultWithHome(homePaths)
	cfg.Daemon.Socket = socketPath
	cfg.Network.Enabled = true
	cfg.Providers = map[string]compozyconfig.ProviderConfig{
		"fake": {Command: "fake-agent"},
	}

	runner := &integrationDaemon{
		t:         t,
		homePaths: homePaths,
		cfg:       cfg,
		pid:       os.Getpid(),
		startedAt: time.Now().UTC(),
	}

	deps := commandDeps{
		loadConfig: func() (compozyconfig.Config, error) {
			return cfg, nil
		},
		resolveHome: func() (compozyconfig.HomePaths, error) {
			return homePaths, nil
		},
		ensureHome: compozyconfig.EnsureHomeLayout,
		newClient:  NewClient,
		newDaemon: func() (daemonRunner, error) {
			return runner, nil
		},
		readDaemonInfo: compozydaemon.ReadInfo,
		signalProcess:  runner.signalProcess,
		processAlive:   runner.processAlive,
		processMatchesStartTime: func(pid int, startedAt time.Time) bool {
			return pid == runner.pid && startedAt.Equal(runner.startedAt)
		},
		getwd: func() (string, error) {
			return workspace, nil
		},
		getenv: func(string) string { return "" },
		now: func() time.Time {
			return time.Now().UTC()
		},
		pollInterval: 10 * time.Millisecond,
		startTimeout: 5 * time.Second,
		stopTimeout:  5 * time.Second,
		spawnDetached: func(context.Context, compozyconfig.HomePaths) (daemonProcess, error) {
			return runner.spawnDetached()
		},
	}
	h := integrationHarness{
		deps:      deps,
		homePaths: homePaths,
		workspace: workspace,
		runner:    runner,
	}
	registerIntegrationHarnessCleanup(t, h)
	return h
}

func (p *integrationDaemonProcess) PID() int {
	return p.pid
}

func (p *integrationDaemonProcess) Done() <-chan struct{} {
	return p.done
}

func (p *integrationDaemonProcess) Wait() error {
	return <-p.waitCh
}

func (p *integrationDaemonProcess) Terminate() error {
	if p.terminate != nil {
		p.terminate()
	}
	return nil
}

func (d *integrationDaemon) spawnDetached() (daemonProcess, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		return nil, errors.New("integration daemon already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	waitCh := make(chan error, 1)
	done := make(chan struct{})
	exitDone := make(chan struct{})
	d.running = true
	d.cancel = cancel
	d.exitDone = exitDone
	d.exitErr = nil

	go func() {
		err := d.Run(ctx)
		waitCh <- err
		close(waitCh)
		d.mu.Lock()
		d.running = false
		d.cancel = nil
		d.exitErr = err
		close(exitDone)
		d.mu.Unlock()
		close(done)
	}()

	return &integrationDaemonProcess{pid: d.pid, done: done, waitCh: waitCh, terminate: cancel}, nil
}

func (d *integrationDaemon) Run(ctx context.Context) (runErr error) {
	joinRunError := func(operation string, cleanupErr error) {
		if cleanupErr == nil {
			return
		}
		runErr = errors.Join(runErr, fmt.Errorf("%s: %w", operation, cleanupErr))
	}

	registry, err := globaldb.OpenGlobalDB(context.Background(), d.homePaths.DatabaseFile)
	if err != nil {
		return fmt.Errorf("open global db: %w", err)
	}
	defer func() {
		joinRunError("close global db", registry.Close(context.Background()))
	}()

	fanout := &integrationNotifierFanout{}
	resolver, err := workspacepkg.NewResolver(
		registry,
		workspacepkg.WithHomePaths(d.homePaths),
		workspacepkg.WithLogger(discardLogger()),
		workspacepkg.WithConfigLoader(func(string) (compozyconfig.Config, error) { return d.cfg, nil }),
	)
	if err != nil {
		return fmt.Errorf("new workspace resolver: %w", err)
	}
	participationResolver := apitestutil.NewIntegrationParticipationResolver(d.t, registry)
	sandboxRegistry, err := sandboxlocal.NewRegistry()
	if err != nil {
		return fmt.Errorf("new local sandbox registry: %w", err)
	}
	manager, err := session.NewManager(
		session.WithHomePaths(d.homePaths),
		session.WithWorkspaceResolver(resolver),
		session.WithLogger(discardLogger()),
		session.WithDriver(func() *integrationDriver {
			driver := newIntegrationDriver()
			d.mu.Lock()
			d.driver = driver
			d.mu.Unlock()
			return driver
		}()),
		session.WithNotifier(fanout),
		session.WithSandboxRegistry(sandboxRegistry),
		session.WithSoulSnapshotStore(registry),
		session.WithSoulRunActivityChecker(integrationSoulRunActivityChecker{}),
		session.WithSessionHealthStore(registry),
		session.WithSessionPromptAdmissionStore(registry),
		session.WithSessionHealthConfig(d.cfg.Agents.Heartbeat),
		session.WithSessionCatalog(registry),
		session.WithParticipationResolver(participationResolver),
	)
	if err != nil {
		return fmt.Errorf("new session manager: %w", err)
	}
	d.mu.Lock()
	d.manager = manager
	d.mu.Unlock()
	resolver.SetUnregisterPreparer(
		func(
			ctx context.Context,
			workspace workspacepkg.Workspace,
		) (workspacepkg.UnregisterPreparation, error) {
			return manager.PrepareWorkspaceRemoval(ctx, workspace.ID)
		},
	)

	taskManager, err := taskpkg.NewManager(
		taskpkg.WithStore(registry),
		taskpkg.WithSessionExecutor(&integrationTaskExecutor{}),
		taskpkg.WithParticipationResolver(participationResolver),
	)
	if err != nil {
		return fmt.Errorf("new task manager: %w", err)
	}

	bridgeService := newIntegrationBridgeService(registry, d.bridgeProviders)
	observer, err := observe.New(
		context.Background(),
		observe.WithHomePaths(d.homePaths),
		observe.WithRegistry(registry),
		observe.WithSessionSource(manager),
		observe.WithBridgeSource(bridgeService),
		observe.WithLogger(discardLogger()),
		observe.WithStartTime(d.startedAt),
	)
	if err != nil {
		return fmt.Errorf("new observer: %w", err)
	}
	d.mu.Lock()
	d.tasks = taskManager
	d.observer = observer
	d.mu.Unlock()
	defer func() {
		joinRunError("close observer", observer.Close(context.Background()))
	}()
	fanout.notifiers = append(fanout.notifiers, observer)

	memoryStore := memory.NewStore(
		d.homePaths.MemoryDir,
		memory.WithCatalogDatabasePath(d.homePaths.DatabaseFile),
	)
	if err := memoryStore.EnsureDirs(); err != nil {
		return fmt.Errorf("ensure memory dirs: %w", err)
	}
	if err := memoryStore.OpenCatalog(context.Background()); err != nil {
		return fmt.Errorf("open memory catalog: %w", err)
	}
	defer func() {
		joinRunError("close memory catalog", memoryStore.CloseCatalog(context.Background()))
	}()
	dreamTrigger := &integrationDreamTrigger{
		enabled:   true,
		triggered: true,
		last:      time.Date(2026, 4, 4, 3, 30, 0, 0, time.UTC),
	}
	extRegistry := extensionpkg.NewRegistry(registry.DB())
	extManager := extensionpkg.NewManager(
		extRegistry,
		extensionpkg.WithLogger(discardLogger()),
	)
	if err := extManager.Start(context.Background()); err != nil {
		return fmt.Errorf("start extension manager: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		joinRunError("stop extension manager", extManager.Stop(shutdownCtx))
	}()
	extService := &integrationExtensionService{
		homePaths:                        d.homePaths,
		registry:                         extRegistry,
		manager:                          extManager,
		marketplaceLoader:                d.extensionMarketplaceLoader(),
		marketplacePolicyAllowUnverified: d.cfg.Extensions.Trust.AllowUnverified,
		marketplaceTrust:                 d.extensionTrust,
	}

	automationManager, err := automationpkg.New(
		automationpkg.WithStore(registry),
		automationpkg.WithSessions(manager),
		automationpkg.WithWorkspaceResolver(resolver),
		automationpkg.WithConfig(d.cfg.Automation),
		automationpkg.WithLogger(discardLogger()),
		automationpkg.WithGlobalWorkspacePath(d.homePaths.HomeDir),
	)
	if err != nil {
		return fmt.Errorf("new automation manager: %w", err)
	}
	if err := automationManager.Start(ctx); err != nil {
		return fmt.Errorf("start automation manager: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		joinRunError("shutdown automation manager", automationManager.Shutdown(shutdownCtx))
	}()
	fanout.notifiers = append(fanout.notifiers, automationManager.SessionObserver())
	networkManager, err := network.NewManager(
		ctx,
		d.cfg.Network,
		d.homePaths.NetworkAuditFile,
		registry,
		network.WithManagerLogger(discardLogger()),
	)
	if err != nil {
		return fmt.Errorf("new network manager: %w", err)
	}
	manager.SetNetworkPeerLifecycle(networkManager)
	manager.SetTurnEndNotifier(networkManager.OnTurnEnd)

	soulAuthoring, err := soul.NewManagedSoulAuthoringService(registry)
	if err != nil {
		return fmt.Errorf("new soul authoring service: %w", err)
	}
	heartbeatAuthoring, err := heartbeat.NewManagedHeartbeatAuthoringService(registry)
	if err != nil {
		return fmt.Errorf("new heartbeat authoring service: %w", err)
	}
	heartbeatStatus, err := heartbeat.NewManagedHeartbeatStatusService(
		registry,
		heartbeat.WithHeartbeatStatusSessionHealthReader(manager),
	)
	if err != nil {
		return fmt.Errorf("new heartbeat status service: %w", err)
	}

	server, err := udsapi.New(
		udsapi.WithHomePaths(d.homePaths),
		udsapi.WithConfig(&d.cfg),
		udsapi.WithSocketPath(d.cfg.Daemon.Socket),
		udsapi.WithLogger(discardLogger()),
		udsapi.WithStartedAt(d.startedAt),
		udsapi.WithPollInterval(10*time.Millisecond),
		udsapi.WithSessionManager(manager),
		udsapi.WithSessionCatalog(registry),
		udsapi.WithTaskService(taskManager),
		udsapi.WithNetworkService(networkManager),
		udsapi.WithNetworkStore(registry),
		udsapi.WithObserver(observer),
		udsapi.WithAutomation(automationManager),
		udsapi.WithBridgeService(bridgeService),
		udsapi.WithWorkspaceResolver(resolver),
		udsapi.WithMemoryStore(memoryStore),
		udsapi.WithDreamTrigger(dreamTrigger),
		udsapi.WithExtensionService(extService),
		udsapi.WithSoulAuthoring(soulAuthoring),
		udsapi.WithSoulRefresher(manager),
		udsapi.WithHeartbeatAuthoring(heartbeatAuthoring),
		udsapi.WithHeartbeatStatus(heartbeatStatus),
		udsapi.WithSessionHealthReader(manager),
		udsapi.WithHeartbeatWakeEventReader(registry),
	)
	if err != nil {
		return fmt.Errorf("new uds server: %w", err)
	}

	if err := server.Start(context.Background()); err != nil {
		return fmt.Errorf("start uds server: %w", err)
	}
	d.mu.Lock()
	d.bridges = bridgeService
	d.mu.Unlock()
	defer func() {
		shutdown := func(operation string, stop func(context.Context) error) {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			joinRunError(operation, stop(shutdownCtx))
		}
		for _, info := range manager.List() {
			if info == nil || info.State == session.StateStopped {
				continue
			}
			shutdown(
				fmt.Sprintf("stop active session %q", info.ID),
				func(ctx context.Context) error { return manager.Stop(ctx, info.ID) },
			)
		}
		shutdown("shutdown network manager", networkManager.Shutdown)
		shutdown("shutdown UDS server", server.Shutdown)
		joinRunError("remove daemon info", compozydaemon.RemoveInfo(d.homePaths.DaemonInfo))
		d.mu.Lock()
		d.bridges = nil
		d.manager = nil
		d.driver = nil
		d.tasks = nil
		d.observer = nil
		d.mu.Unlock()
	}()

	if err := compozydaemon.WriteInfo(d.homePaths.DaemonInfo, compozydaemon.Info{
		PID:       d.pid,
		Port:      d.cfg.HTTP.Port,
		StartedAt: d.startedAt,
	}); err != nil {
		return fmt.Errorf("write daemon info: %w", err)
	}

	<-ctx.Done()
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return ctx.Err()
}

func (d *integrationDaemon) signalProcess(pid int, sig syscall.Signal) error {
	d.mu.Lock()
	cancel := d.cancel
	running := d.running
	d.mu.Unlock()

	if !running || pid != d.pid {
		return fmt.Errorf("integration daemon pid %d is not running", pid)
	}
	if sig != syscall.SIGTERM {
		return fmt.Errorf("unsupported signal %v", sig)
	}
	cancel()
	return nil
}

func (d *integrationDaemon) processAlive(pid int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running && pid == d.pid
}

func (d *integrationDaemon) waitForExit() error {
	return d.waitForExitWithin(0)
}

func (d *integrationDaemon) waitForExitWithin(timeout time.Duration) error {
	d.mu.Lock()
	exitDone := d.exitDone
	d.mu.Unlock()
	if exitDone == nil {
		return nil
	}

	if timeout <= 0 {
		<-exitDone
	} else {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-exitDone:
		case <-timer.C:
			return fmt.Errorf("integration daemon did not exit within %s", timeout)
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	return d.exitErr
}

func (d *integrationDaemon) bridgeService() *integrationBridgeService {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.bridges
}

func (f *integrationNotifierFanout) OnSessionCreated(ctx context.Context, sess *session.Session) {
	for _, notifier := range f.notifiers {
		notifier.OnSessionCreated(ctx, sess)
	}
}

func (f *integrationNotifierFanout) OnSessionStopped(ctx context.Context, sess *session.Session) {
	for _, notifier := range f.notifiers {
		notifier.OnSessionStopped(ctx, sess)
	}
}

func (f *integrationNotifierFanout) OnAgentEvent(ctx context.Context, sessionID string, event any) {
	for _, notifier := range f.notifiers {
		notifier.OnAgentEvent(ctx, sessionID, event)
	}
}

func newIntegrationDriver() *integrationDriver {
	return &integrationDriver{
		nextPID:  2000,
		nextSess: 1,
		states:   make(map[*session.AgentProcess]chan struct{}),
		blocked:  make(map[string]chan struct{}),
	}
}

func (e *integrationTaskExecutor) StartTaskSession(
	_ context.Context,
	_ *taskpkg.StartTaskSession,
) (*taskpkg.SessionRef, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.next++
	return &taskpkg.SessionRef{SessionID: fmt.Sprintf("task-sess-%d", e.next)}, nil
}

func (e *integrationTaskExecutor) AttachTaskSession(
	_ context.Context,
	_ string,
	sessionID string,
) (*taskpkg.SessionRef, error) {
	return &taskpkg.SessionRef{SessionID: strings.TrimSpace(sessionID)}, nil
}

func (e *integrationTaskExecutor) RequestTaskStop(context.Context, string, taskpkg.StopReason) error {
	return nil
}

func (e *integrationTaskExecutor) ForceTaskStop(context.Context, string, taskpkg.StopReason) error {
	return nil
}

func (d *integrationDriver) Start(_ context.Context, opts acp.StartOpts) (*session.AgentProcess, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.nextPID++
	d.nextSess++
	done := make(chan struct{})
	sessionID := strings.TrimSpace(opts.ResumeSessionID)
	if sessionID == "" {
		sessionID = fmt.Sprintf("acp-session-%d", d.nextSess)
	}

	proc := session.NewAgentProcess(session.AgentProcessOptions{
		PID:       d.nextPID,
		AgentName: opts.AgentName,
		Command:   opts.Command,
		Cwd:       opts.Cwd,
		SessionID: sessionID,
		Caps: acp.Caps{
			SupportsLoadSession: true,
		},
		StartedAt: time.Now().UTC(),
		Done:      done,
		Wait: func() error {
			<-done
			return nil
		},
	})
	d.states[proc] = done
	return proc, nil
}

func (d *integrationDriver) Prompt(
	_ context.Context,
	proc *session.AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	ch := make(chan acp.AgentEvent, 2)
	ch <- acp.AgentEvent{
		Type:      "agent_message",
		SessionID: proc.SessionID,
		TurnID:    req.TurnID,
		Timestamp: time.Now().UTC(),
		Text:      req.Message,
	}
	if strings.Contains(req.Message, "__block__") {
		release := make(chan struct{})
		d.mu.Lock()
		d.blocked[proc.SessionID] = release
		d.mu.Unlock()

		go func() {
			<-release
			ch <- acp.AgentEvent{
				Type:       "done",
				SessionID:  proc.SessionID,
				TurnID:     req.TurnID,
				Timestamp:  time.Now().UTC(),
				StopReason: "end_turn",
			}
			close(ch)
			d.mu.Lock()
			delete(d.blocked, proc.SessionID)
			d.mu.Unlock()
		}()
		return ch, nil
	}
	var usage *acp.TokenUsage
	if strings.Contains(req.Message, "__usage__") {
		input := int64(10)
		output := int64(5)
		total := input + output
		usage = &acp.TokenUsage{
			InputTokens:  &input,
			OutputTokens: &output,
			TotalTokens:  &total,
		}
	}
	ch <- acp.AgentEvent{
		Type:       "done",
		SessionID:  proc.SessionID,
		TurnID:     req.TurnID,
		Timestamp:  time.Now().UTC(),
		StopReason: "end_turn",
		Usage:      usage,
	}
	close(ch)
	return ch, nil
}

func (d *integrationDriver) releaseBlocked(sessionID string) {
	d.mu.Lock()
	release := d.blocked[sessionID]
	d.mu.Unlock()
	if release == nil {
		return
	}
	select {
	case <-release:
	default:
		close(release)
	}
}

func (d *integrationDriver) waitForBlocked(sessionID string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		_, ok := d.blocked[sessionID]
		d.mu.Unlock()
		if ok {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func (d *integrationDriver) Cancel(context.Context, *session.AgentProcess) error {
	return nil
}

func (d *integrationDriver) Stop(_ context.Context, proc *session.AgentProcess) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	done, ok := d.states[proc]
	if !ok {
		return nil
	}
	select {
	case <-done:
	default:
		close(done)
	}
	delete(d.states, proc)
	if release, ok := d.blocked[proc.SessionID]; ok {
		select {
		case <-release:
		default:
			close(release)
		}
		delete(d.blocked, proc.SessionID)
	}
	return nil
}

func (d *integrationDaemon) releaseBlocked(sessionID string) {
	d.mu.Lock()
	driver := d.driver
	manager := d.manager
	d.mu.Unlock()
	if driver == nil {
		return
	}
	target := sessionID
	if manager != nil {
		if info, err := manager.Status(
			context.Background(),
			sessionID,
		); err == nil &&
			strings.TrimSpace(info.ACPSessionID) != "" {
			target = info.ACPSessionID
		}
	}
	driver.releaseBlocked(target)
}

func (d *integrationDaemon) waitForBlocked(sessionID string, timeout time.Duration) bool {
	d.mu.Lock()
	driver := d.driver
	manager := d.manager
	d.mu.Unlock()
	if driver == nil {
		return false
	}
	target := sessionID
	if manager != nil {
		if info, err := manager.Status(
			context.Background(),
			sessionID,
		); err == nil &&
			strings.TrimSpace(info.ACPSessionID) != "" {
			target = info.ACPSessionID
		}
	}
	return driver.waitForBlocked(target, timeout)
}

func (d *integrationDaemon) blockSession(sessionID string) (<-chan acp.AgentEvent, error) {
	d.mu.Lock()
	manager := d.manager
	d.mu.Unlock()
	if manager == nil {
		return nil, errors.New("integration daemon session manager is not ready")
	}
	return manager.Prompt(context.Background(), sessionID, "__block__")
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func shortSocketPath(t *testing.T) string {
	t.Helper()

	root, err := os.MkdirTemp(os.TempDir(), "compozyc-")
	if err != nil {
		t.Fatalf("os.MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("os.RemoveAll(%q) error = %v", root, err)
		}
	})
	return filepath.Join(root, "daemon.sock")
}

func writeAgentDef(t *testing.T, homePaths compozyconfig.HomePaths, name string) {
	t.Helper()

	agentDir := filepath.Join(homePaths.AgentsDir, name)
	writeAgentDefInDir(t, agentDir, name)
}

func writeWorkspaceAgentDef(t *testing.T, root string, name string) {
	t.Helper()

	agentDir := filepath.Join(root, compozyconfig.DirName, compozyconfig.AgentsDirName, name)
	writeAgentDefInDir(t, agentDir, name)
}

func writeAgentDefInDir(t *testing.T, agentDir string, name string) {
	t.Helper()

	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", agentDir, err)
	}
	content := strings.Join([]string{
		"---",
		"name: " + name,
		"provider: fake",
		"model: fake-model",
		"---",
		"",
		"You are the integration test agent.",
	}, "\n")
	if err := os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(AGENT.md) error = %v", err)
	}
}

func mustExecuteRoot(t *testing.T, deps commandDeps, args ...string) string {
	t.Helper()

	stdout, stderr, err := executeRootCommand(t, deps, args...)
	if err != nil {
		t.Fatalf("executeRootCommand(%v) error = %v; stderr=%s", args, err, stderr)
	}
	return stdout
}

func waitForCondition(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func waitUntilLeaseExpires(t *testing.T, leaseUntil time.Time, timeout time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if time.Now().After(leaseUntil) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for lease expiry at %s", leaseUntil.Format(time.RFC3339Nano))
		case <-ticker.C:
		}
	}
}
