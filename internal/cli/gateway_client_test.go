package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

// Invariant: pairing mint emits only a private handoff reference, never the raw artifact bytes.
// Owning layer: CLI gateway output serialization and artifact handoff filesystem boundary.
// Canonical suite: this pairing mint output contract suite.
func TestGatewayPairingArtifactOutput(t *testing.T) {
	t.Parallel()

	rawArtifact := "cpz_gwp_pairing-artifact-material-that-must-not-be-emitted"
	expiresAt := time.Date(2026, 8, 7, 18, 30, 0, 0, time.UTC)
	for _, format := range []string{"human", "json", "jsonl", "toon"} {
		t.Run("Should emit only a private artifact reference as "+format, func(t *testing.T) {
			t.Parallel()
			homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
				t.Fatalf("EnsureHomeLayout() error = %v", err)
			}
			gatewayAPI := &gatewayPairingAPIStub{minted: contract.GatewayPairingArtifactPayload{
				Artifact: rawArtifact, ExpiresAt: expiresAt,
			}}
			deps := commandDeps{
				resolveHome: func() (compozyconfig.HomePaths, error) { return homePaths, nil },
				loadConfig: func() (compozyconfig.Config, error) {
					return compozyconfig.LoadForHome(homePaths)
				},
				newClient: func(ClientTarget) (DaemonClient, error) {
					return &gatewayPairingDaemonClient{
						DaemonClient: &stubClient{}, gatewayClientAPI: gatewayAPI,
					}, nil
				},
			}
			stdout, stderr, err := executeRootCommand(t, deps, "pair", "mint", "-o", format)
			if err != nil {
				t.Fatalf("pair mint -o %s error = %v", format, err)
			}
			if strings.Contains(stdout, rawArtifact) || strings.Contains(stderr, rawArtifact) {
				t.Fatalf("pair mint -o %s emitted raw artifact: stdout=%q stderr=%q", format, stdout, stderr)
			}
			matches, err := filepath.Glob(filepath.Join(
				homePaths.GatewayCredentialsDir,
				gatewayPairingArtifactFilePrefix+"*"+gatewayPairingArtifactFileSuffix,
			))
			if err != nil {
				t.Fatalf("Glob(pairing artifacts) error = %v", err)
			}
			if len(matches) != 1 {
				t.Fatalf("pairing artifact files = %v, want exactly one", matches)
			}
			artifactBytes, err := readPrivateBoundedFile(matches[0], 1024, "pairing artifact")
			if err != nil {
				t.Fatalf("readPrivateBoundedFile() error = %v", err)
			}
			if strings.TrimSpace(string(artifactBytes)) != rawArtifact {
				t.Fatalf("pairing artifact file = %q, want raw minted artifact", artifactBytes)
			}
			if !strings.Contains(stdout, matches[0]) {
				t.Fatalf("pair mint -o %s output = %q, want artifact reference %q", format, stdout, matches[0])
			}
		})
	}
}

func TestGatewayClientTargetErrorsAndIsolation(t *testing.T) {
	t.Parallel()
	credential := testGatewayCredential('a')

	t.Run("Should distinguish reachability from authentication failures [UT-100]", func(t *testing.T) {
		t.Parallel()
		target, err := GatewayClientTarget("remote", "gateway.example", 443, credential)
		if err != nil {
			t.Fatalf("GatewayClientTarget() error = %v", err)
		}
		reachabilityClient := &daemonClient{
			target: target,
			httpClient: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("offline")}
			})},
		}
		_, reachabilityErr := reachabilityClient.GetGatewayStatus(t.Context())
		assertGatewayClientErrorCode(t, reachabilityErr, gatewayReachabilityCode)

		authClient := &daemonClient{
			target: target,
			httpClient: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return newHTTPResponse(
					http.StatusUnauthorized,
					`{"error":"Unauthorized","code":"gateway_device_unauthenticated"}`,
				), nil
			})},
		}
		_, authErr := authClient.GetGatewayStatus(t.Context())
		if authErr == nil || strings.Contains(authErr.Error(), "unreachable") {
			t.Fatalf("GetGatewayStatus(auth) error = %v, want distinct authentication failure", authErr)
		}
		var apiErr *daemonAPIError
		if !errors.As(authErr, &apiErr) || apiErr.payload.Code != "gateway_device_unauthenticated" {
			t.Fatalf("GetGatewayStatus(auth) error = %T %v, want gateway_device_unauthenticated", authErr, authErr)
		}
	})

	t.Run("Should distinguish pairing failures before and after request writing", func(t *testing.T) {
		t.Parallel()
		target, err := UnauthenticatedGatewayClientTarget("remote", "gateway.example", 443)
		if err != nil {
			t.Fatalf("UnauthenticatedGatewayClientTarget() error = %v", err)
		}
		request := contract.GatewayPairingRedeemRequest{
			Artifact: "pairing", Name: "remote", ActorKind: "cli_profile",
			Credential: testGatewayCredential('w'),
		}
		for _, testCase := range []struct {
			name          string
			writeStarted  bool
			requestUnsent bool
		}{
			{name: "before write", requestUnsent: true},
			{name: "after write", writeStarted: true, requestUnsent: false},
		} {
			t.Run("Should classify failure "+testCase.name, func(t *testing.T) {
				t.Parallel()
				client := &daemonClient{
					target: target,
					httpClient: &http.Client{
						Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
							if testCase.writeStarted {
								trace := httptrace.ContextClientTrace(req.Context())
								if trace == nil || trace.WroteHeaders == nil {
									t.Fatal("pairing request trace is missing")
								}
								trace.WroteHeaders()
							}
							return nil, context.Canceled
						}),
					},
				}
				_, redeemErr := client.RedeemGatewayPairing(t.Context(), request)
				if !errors.Is(redeemErr, context.Canceled) {
					t.Fatalf("RedeemGatewayPairing() error = %v, want context.Canceled", redeemErr)
				}
				if got := gatewayPairingRequestNotSent(redeemErr); got != testCase.requestUnsent {
					t.Fatalf("gatewayPairingRequestNotSent() = %t, want %t", got, testCase.requestUnsent)
				}
			})
		}
	})

	t.Run("Should isolate a revoked profile from healthy profiles [UT-105]", func(t *testing.T) {
		t.Parallel()
		revoked := testGatewayCredential('r')
		healthy := testGatewayCredential('h')
		transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			if request.Header.Get("Authorization") == "Bearer "+revoked {
				return newHTTPResponse(
					http.StatusUnauthorized,
					`{"error":"revoked","code":"gateway_device_unauthenticated"}`,
				), nil
			}
			return newHTTPResponse(http.StatusOK, `{"enabled":true}`), nil
		})
		revokedTarget, err := GatewayClientTarget("revoked", "gateway.example", 443, revoked)
		if err != nil {
			t.Fatalf("GatewayClientTarget(revoked) error = %v", err)
		}
		healthyTarget, err := GatewayClientTarget("healthy", "gateway.example", 443, healthy)
		if err != nil {
			t.Fatalf("GatewayClientTarget(healthy) error = %v", err)
		}
		revokedClient := &daemonClient{target: revokedTarget, httpClient: &http.Client{Transport: transport}}
		healthyClient := &daemonClient{target: healthyTarget, httpClient: &http.Client{Transport: transport}}
		for attempt := range 3 {
			_, err := revokedClient.GetGatewayStatus(t.Context())
			var apiErr *daemonAPIError
			if !errors.As(err, &apiErr) || apiErr.payload.Code != "gateway_device_unauthenticated" {
				t.Fatalf("revoked attempt %d error = %T %v", attempt, err, err)
			}
			status, err := healthyClient.GetGatewayStatus(t.Context())
			if err != nil || !status.Enabled {
				t.Fatalf("healthy attempt %d = %#v, %v", attempt, status, err)
			}
		}
	})
}

func TestGatewayClientTargetConstructionAndMatrix(t *testing.T) {
	t.Parallel()

	t.Run("Should construct remote and local requests at one chokepoint [UT-108]", func(t *testing.T) {
		t.Parallel()
		credential := testGatewayCredential('x')
		remoteTarget, err := GatewayClientTarget("laptop", "gateway.example", 8443, credential)
		if err != nil {
			t.Fatalf("GatewayClientTarget() error = %v", err)
		}
		var remoteRequest *http.Request
		remoteTransport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			remoteRequest = request.Clone(request.Context())
			return newHTTPResponse(http.StatusOK, `{}`), nil
		})
		remoteClient := &daemonClient{target: remoteTarget, httpClient: &http.Client{Transport: remoteTransport}}
		if err := remoteClient.doJSON(t.Context(), http.MethodGet, "/api/status", nil, nil, &struct{}{}); err != nil {
			t.Fatalf("remote doJSON() error = %v", err)
		}
		if remoteRequest.URL.String() != "https://gateway.example:8443/api/status" ||
			remoteRequest.Header.Get("Authorization") != "Bearer "+credential {
			t.Fatalf("remote request = %s headers=%v", remoteRequest.URL, remoteRequest.Header)
		}

		var localRequest *http.Request
		localTransport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			localRequest = request.Clone(request.Context())
			return newHTTPResponse(http.StatusOK, `{}`), nil
		})
		localClient := &daemonClient{
			target:     LocalClientTarget("/tmp/compozy.sock"),
			httpClient: &http.Client{Transport: localTransport},
		}
		if err := localClient.doJSON(t.Context(), http.MethodGet, "/api/status", nil, nil, &struct{}{}); err != nil {
			t.Fatalf("local doJSON() error = %v", err)
		}
		if localRequest.URL.String() != "http://unix/api/status" || localRequest.Header.Get("Authorization") != "" {
			t.Fatalf("local request = %s headers=%v", localRequest.URL, localRequest.Header)
		}
	})

	t.Run("Should refuse local-only operations before network I/O [UT-102]", func(t *testing.T) {
		t.Parallel()
		target, err := GatewayClientTarget("remote", "gateway.example", 443, testGatewayCredential('l'))
		if err != nil {
			t.Fatalf("GatewayClientTarget() error = %v", err)
		}
		var calls atomic.Int32
		transport := roundTripperFunc(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return newHTTPResponse(http.StatusOK, `{}`), nil
		})
		client := &daemonClient{
			target:       target,
			httpClient:   &http.Client{Transport: transport},
			streamClient: &http.Client{Transport: transport},
		}
		localOnly := []struct {
			method string
			path   string
		}{
			{method: http.MethodPost, path: "/api/agent/tasks/claim-next"},
			{method: http.MethodPost, path: "/api/tasks/task-1/runs"},
			{method: http.MethodPost, path: "/api/task-runs/run-1/complete"},
			{method: http.MethodPost, path: "/api/runs/run-1/retry"},
			{method: http.MethodPost, path: "/api/runs/bulk/release"},
			{method: http.MethodGet, path: "/api/scheduler"},
			{method: http.MethodPut, path: "/api/resources/skills/coder"},
			{method: http.MethodPost, path: "/api/internal/mcp/call"},
		}
		for _, operation := range localOnly {
			err = client.doJSON(t.Context(), operation.method, operation.path, nil, nil, nil)
			assertGatewayClientErrorCode(t, err, "gateway_local_only_operation")
		}
		err = client.doSSE(
			t.Context(),
			"/api/internal/hosted-mcp/projection/stream",
			nil,
			"",
			func(SSEEvent) error { return nil },
		)
		assertGatewayClientErrorCode(t, err, "gateway_local_only_operation")
		if calls.Load() != 0 {
			t.Fatalf("transport calls = %d, want 0", calls.Load())
		}
	})

	t.Run("Should allow remote task and resource reads through the chokepoint", func(t *testing.T) {
		t.Parallel()
		target, err := GatewayClientTarget("remote", "gateway.example", 443, testGatewayCredential('q'))
		if err != nil {
			t.Fatalf("GatewayClientTarget() error = %v", err)
		}
		var calls atomic.Int32
		client := &daemonClient{
			target: target,
			httpClient: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return newHTTPResponse(http.StatusOK, `{}`), nil
			})},
		}
		for _, path := range []string{"/api/tasks/task-1", "/api/resources/skills/coder"} {
			if err := client.doJSON(t.Context(), http.MethodGet, path, nil, nil, &struct{}{}); err != nil {
				t.Fatalf("GET %s error = %v", path, err)
			}
		}
		if calls.Load() != 2 {
			t.Fatalf("transport calls = %d, want 2", calls.Load())
		}
	})

	t.Run("Should resolve the web UI from the selected target", func(t *testing.T) {
		t.Parallel()
		remote, err := GatewayClientTarget("desktop", "gateway.example", 8443, testGatewayCredential('o'))
		if err != nil {
			t.Fatalf("GatewayClientTarget() error = %v", err)
		}
		remoteURL, err := webUIURL(remote, DaemonStatus{HTTPHost: "127.0.0.1", HTTPPort: 2123})
		if err != nil || remoteURL != "https://gateway.example:8443" {
			t.Fatalf("webUIURL(remote) = %q, %v", remoteURL, err)
		}
		localURL, err := webUIURL(
			LocalClientTarget("/tmp/compozy.sock"),
			DaemonStatus{HTTPHost: "127.0.0.1", HTTPPort: 2123},
		)
		if err != nil || localURL != "http://127.0.0.1:2123" {
			t.Fatalf("webUIURL(local) = %q, %v", localURL, err)
		}
	})

	t.Run("Should reject malformed colon hosts and non-IP SSH loopback aliases", func(t *testing.T) {
		t.Parallel()
		if _, err := GatewayClientTarget(
			"remote", "gateway.example:443", 443, testGatewayCredential('m'),
		); err == nil {
			t.Fatal("GatewayClientTarget() error = nil, want malformed host rejection")
		}
		if _, err := SSHForwardClientTarget("localhost", 2123); err == nil {
			t.Fatal("SSHForwardClientTarget() error = nil, want literal loopback IP requirement")
		}
		if _, err := GatewayClientTarget(
			"remote", "[]", 443, testGatewayCredential('m'),
		); err == nil {
			t.Fatal("GatewayClientTarget(bracket-only) error = nil, want empty host rejection")
		}
		if got := clientNetworkURL("http", "::1", 2123); got != "http://[::1]:2123" {
			t.Fatalf("clientNetworkURL(IPv6) = %q, want bracketed URL", got)
		}
	})

	t.Run("Should preserve remote authentication errors for open", func(t *testing.T) {
		t.Parallel()
		target, err := GatewayClientTarget("remote", "gateway.example", 443, testGatewayCredential('u'))
		if err != nil {
			t.Fatalf("GatewayClientTarget() error = %v", err)
		}
		authErr := &daemonAPIError{
			statusCode: http.StatusUnauthorized,
			payload: contract.ErrorPayload{
				Error: "device revoked", Code: gatewayDeviceUnauthenticatedCode,
			},
		}
		if got := openDaemonStatusError(target, authErr); got != authErr {
			t.Fatalf("openDaemonStatusError() = %v, want original authentication error", got)
		}
	})

	t.Run("Should disable redirects for both remote HTTP clients", func(t *testing.T) {
		t.Parallel()
		target, err := GatewayClientTarget("remote", "gateway.example", 443, testGatewayCredential('n'))
		if err != nil {
			t.Fatalf("GatewayClientTarget() error = %v", err)
		}
		baseClient, err := NewClient(target)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		client := baseClient.(*daemonClient)
		for name, httpClient := range map[string]*http.Client{
			"json":   client.httpClient,
			"stream": client.streamClient,
		} {
			t.Run("Should reject "+name+" redirects", func(t *testing.T) {
				t.Parallel()
				if err := httpClient.CheckRedirect(&http.Request{}, []*http.Request{{}}); !errors.Is(
					err,
					http.ErrUseLastResponse,
				) {
					t.Fatalf("CheckRedirect() error = %v, want http.ErrUseLastResponse", err)
				}
			})
		}
	})

	t.Run("Should bypass environment proxies for SSH forwards", func(t *testing.T) {
		t.Parallel()
		target, err := SSHForwardClientTarget("127.0.0.1", 2123)
		if err != nil {
			t.Fatalf("SSHForwardClientTarget() error = %v", err)
		}
		baseClient, err := NewClient(target)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		client := baseClient.(*daemonClient)
		for name, httpClient := range map[string]*http.Client{
			"json":   client.httpClient,
			"stream": client.streamClient,
		} {
			transport, ok := httpClient.Transport.(*http.Transport)
			if !ok || transport.Proxy != nil {
				t.Fatalf("%s transport = %#v, want proxy disabled", name, httpClient.Transport)
			}
		}
	})

	t.Run("Should keep structured stderr free of the human target banner", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := compozyconfig.DefaultWithHome(homePaths)
		cfg.Gateway.Connections = []compozyconfig.GatewayConnectionConfig{{
			Name: "remote", Scheme: "https", Host: "gateway.example", Port: 443,
			CredentialFile: "remote.cred",
		}}
		cfg.Gateway.ActiveConnection = "remote"
		requestErr := newGatewayReachabilityError(
			ClientTarget{name: "remote", kind: clientTargetGateway},
			errors.New("offline"),
		)
		deps := commandDeps{
			resolveHome: func() (compozyconfig.HomePaths, error) { return homePaths, nil },
			loadConfig:  func() (compozyconfig.Config, error) { return cfg, nil },
			readGatewayCredential: func(string, string) (string, error) {
				return testGatewayCredential('u'), nil
			},
			newClient: func(ClientTarget) (DaemonClient, error) {
				return &stubClient{statusFn: func(context.Context) (StatusRecord, error) {
					return StatusRecord{}, requestErr
				}}, nil
			},
		}
		_, stderr, err := executeRootCommand(t, deps, "status", "-o", "json")
		if !errors.Is(err, requestErr) {
			t.Fatalf("status error = %v, want reachability error", err)
		}
		if stderr != "" {
			t.Fatalf("command stderr = %q, want no human target banner", stderr)
		}
		var rendered bytes.Buffer
		if exitCode := writeExecutionError(&rendered, []string{"status", "-o", "json"}, err); exitCode == 0 {
			t.Fatal("writeExecutionError() exit code = 0, want failure")
		}
		if !json.Valid(rendered.Bytes()) {
			t.Fatalf("structured stderr = %q, want one valid JSON document", rendered.String())
		}
	})

	t.Run("Should open the active remote profile origin at command level", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := compozyconfig.DefaultWithHome(homePaths)
		cfg.Gateway.Connections = []compozyconfig.GatewayConnectionConfig{{
			Name: "remote", Scheme: "https", Host: "gateway.example", Port: 8443,
			CredentialFile: "remote.cred",
		}}
		cfg.Gateway.ActiveConnection = "remote"
		var opened string
		deps := commandDeps{
			resolveHome: func() (compozyconfig.HomePaths, error) { return homePaths, nil },
			loadConfig:  func() (compozyconfig.Config, error) { return cfg, nil },
			readGatewayCredential: func(string, string) (string, error) {
				return testGatewayCredential('v'), nil
			},
			newClient: func(ClientTarget) (DaemonClient, error) {
				return &stubClient{daemonStatusFn: func(context.Context) (DaemonStatus, error) {
					return DaemonStatus{HTTPHost: "127.0.0.1", HTTPPort: 2123}, nil
				}}, nil
			},
			openBrowser: func(_ context.Context, target string) error {
				opened = target
				return nil
			},
		}

		_, stderr, err := executeRootCommand(t, deps, "open")
		if err != nil {
			t.Fatalf("open command error = %v", err)
		}
		if opened != "https://gateway.example:8443" {
			t.Fatalf("opened URL = %q, want remote profile origin", opened)
		}
		if !strings.Contains(stderr, "target: remote") {
			t.Fatalf("open command stderr = %q, want explicit remote target", stderr)
		}
	})

	t.Run("Should publish the enforced operation matrix in gateway and connect help", func(t *testing.T) {
		t.Parallel()
		for _, command := range []string{"gateway", "connect"} {
			stdout, _, err := executeRootCommand(t, commandDeps{}, command, "--help")
			if err != nil {
				t.Fatalf("%s --help error = %v", command, err)
			}
			if !strings.Contains(stdout, gatewayOperationMatrix) {
				t.Fatalf("%s --help omitted the remote operation matrix", command)
			}
		}
	})
}

func TestGatewayClientStreamsReportStableInterruption(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve complete events then report a distinct interruption [UT-103] [UT-104]", func(t *testing.T) {
		t.Parallel()
		exerciseGatewayClientStreamInterruption(t)
	})
}

func exerciseGatewayClientStreamInterruption(t *testing.T) {
	t.Helper()
	target, err := GatewayClientTarget("remote", "gateway.example", 443, testGatewayCredential('s'))
	if err != nil {
		t.Fatalf("GatewayClientTarget() error = %v", err)
	}
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/gateway/stream-tickets" {
			return newHTTPResponse(http.StatusCreated, `{"ticket":"ticket-1"}`), nil
		}
		if request.URL.Query().Get("ticket") != "ticket-1" {
			t.Fatalf("stream ticket = %q, want ticket-1", request.URL.Query().Get("ticket"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body: io.NopCloser(&stagedResponseReader{
				chunks:      [][]byte{[]byte("event: progress\ndata: {}\n\n")},
				terminalErr: errors.New("connection reset"),
			}),
		}, nil
	})
	client := &daemonClient{
		target:       target,
		httpClient:   &http.Client{Transport: transport},
		streamClient: &http.Client{Transport: transport},
	}
	events := 0
	err = client.doSSE(t.Context(), "/api/logs/stream", nil, "", func(SSEEvent) error {
		events++
		return nil
	})
	assertGatewayClientErrorCode(t, err, gatewayStreamInterruptedCode)
	if events != 1 {
		t.Fatalf("complete events = %d, want 1 before interruption", events)
	}

	var stderr bytes.Buffer
	if exit := writeExecutionError(
		&stderr,
		[]string{"gateway", "status", "-o", "json"},
		err,
	); exit != agentidentity.ExitUnavailable {
		t.Fatalf("writeExecutionError() exit = %d, want %d", exit, agentidentity.ExitUnavailable)
	}
	if !json.Valid(stderr.Bytes()) || !bytes.Contains(stderr.Bytes(), []byte(gatewayStreamInterruptedCode)) {
		t.Fatalf("structured stream error = %q, want valid JSON with explicit interruption [UT-104]", stderr.String())
	}
}

func TestGatewayProfileTransactions(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a duplicate profile before writing its credential [UT-101]", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		profile := compozyconfig.GatewayConnectionConfig{
			Name: "laptop", Scheme: "https", Host: "gateway.example", Port: 443,
			CredentialFile: "laptop.cred",
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		if err := upsertGatewayProfileMetadata(homePaths, profile); err != nil {
			t.Fatalf("upsertGatewayProfileMetadata() error = %v", err)
		}
		var writes atomic.Int32
		err = persistPairedGatewayProfile(
			t.Context(),
			homePaths,
			profile,
			testGatewayCredential('d'),
			false,
			false,
			commandDeps{
				writeGatewayCredential: func(string, string, string) (string, error) {
					writes.Add(1)
					return "", nil
				},
			},
		)
		assertGatewayClientErrorCode(t, err, gatewayProfileConflictCode)
		if writes.Load() != 0 {
			t.Fatalf("credential writes = %d, want 0 [UT-101]", writes.Load())
		}
	})

	t.Run("Should durably write the CLI credential before redeeming the remote device [IT-060]", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		credentials := map[string]string{}
		gatewayAPI := &gatewayPairingAPIStub{beforeRedeem: func(request contract.GatewayPairingRedeemRequest) error {
			stored := credentials["laptop"]
			if stored == "" || stored != request.Credential {
				return errors.New("stored credential does not match redeem request")
			}
			if _, err := readGatewayProfileTransactionJournal(
				homePaths.GatewayCredentialsDir,
				"laptop",
			); err != nil {
				return fmt.Errorf("read pairing journal before redeem: %w", err)
			}
			return nil
		}}
		deps := commandDeps{
			resolveHome: func() (compozyconfig.HomePaths, error) { return homePaths, nil },
			loadConfig:  func() (compozyconfig.Config, error) { return compozyconfig.LoadForHome(homePaths) },
			newClient: func(ClientTarget) (DaemonClient, error) {
				return &gatewayPairingDaemonClient{DaemonClient: &stubClient{}, gatewayClientAPI: gatewayAPI}, nil
			},
			writeGatewayCredential: func(_ string, name string, credential string) (string, error) {
				credentials[name] = credential
				return name + ".cred", nil
			},
			readGatewayCredential: func(_ string, name string) (string, error) {
				credential, exists := credentials[name]
				if !exists {
					return "", os.ErrNotExist
				}
				return credential, nil
			},
			removeGatewayCredential: func(_ string, name string) error {
				delete(credentials, name)
				return nil
			},
		}
		stdout, _, err := executeRootCommand(
			t, deps,
			"pair", "redeem", "pairing", "--name", "laptop",
			"--address", "https://gateway.example", "-o", "json",
		)
		if err != nil || !json.Valid([]byte(stdout)) {
			t.Fatalf("pair redeem = stdout:%q err:%v", stdout, err)
		}
		if credentials["laptop"] == "" {
			t.Fatal("paired credential is missing after redemption")
		}
	})

	// Invariant: a pairing request that never opened a connection leaves no credential, profile, or journal.
	// Owning layer: CLI profile transaction workflow.
	// Canonical suite: TestGatewayProfileTransactions.
	t.Run("Should roll back pairing when the request never reached the gateway [US-019.EC-1]", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		credentials := map[string]string{}
		target, err := UnauthenticatedGatewayClientTarget("laptop", "gateway.example", 443)
		if err != nil {
			t.Fatalf("UnauthenticatedGatewayClientTarget() error = %v", err)
		}
		gatewayAPI := &gatewayPairingAPIStub{beforeRedeem: func(
			contract.GatewayPairingRedeemRequest,
		) error {
			return &gatewayPairingRequestError{
				cause:          newGatewayReachabilityError(target, context.Canceled),
				requestNotSent: true,
			}
		}}
		deps := commandDeps{
			resolveHome: func() (compozyconfig.HomePaths, error) { return homePaths, nil },
			loadConfig:  func() (compozyconfig.Config, error) { return compozyconfig.LoadForHome(homePaths) },
			newClient: func(ClientTarget) (DaemonClient, error) {
				return &gatewayPairingDaemonClient{DaemonClient: &stubClient{}, gatewayClientAPI: gatewayAPI}, nil
			},
			writeGatewayCredential: func(_ string, name string, credential string) (string, error) {
				credentials[name] = credential
				return name + ".cred", nil
			},
			readGatewayCredential: func(_ string, name string) (string, error) {
				credential, exists := credentials[name]
				if !exists {
					return "", os.ErrNotExist
				}
				return credential, nil
			},
			removeGatewayCredential: func(_ string, name string) error {
				delete(credentials, name)
				return nil
			},
		}
		_, _, err = executeRootCommand(
			t,
			deps,
			"connect", "add", "laptop", "--address", "https://gateway.example",
			"--pairing", "pairing-once", "--use", "-o", "json",
		)
		assertGatewayClientErrorCode(t, err, gatewayReachabilityCode)
		if _, exists := credentials["laptop"]; exists {
			t.Fatal("unreachable pairing left a credential behind")
		}
		if _, err := readGatewayProfileTransactionJournal(
			homePaths.GatewayCredentialsDir,
			"laptop",
		); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("readGatewayProfileTransactionJournal() error = %v, want os.ErrNotExist", err)
		}
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if _, exists := findGatewayProfile(loaded.Gateway.Connections, "laptop"); exists {
			t.Fatal("unreachable pairing left profile metadata behind")
		}
		if loaded.Gateway.ActiveConnection != "" {
			t.Fatalf("active profile = %q, want local", loaded.Gateway.ActiveConnection)
		}
	})

	t.Run(
		"Should recover an activated device from its durable local credential after activation fails",
		func(t *testing.T) {
			t.Parallel()
			homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
				t.Fatalf("EnsureHomeLayout() error = %v", err)
			}
			oldProfile := compozyconfig.GatewayConnectionConfig{
				Name: "laptop", Scheme: "https", Host: "old.example", Port: 443,
				CredentialFile: "laptop.cred", DefaultWorkspace: "old-workspace",
			}
			otherProfile := compozyconfig.GatewayConnectionConfig{
				Name: "other", Scheme: "https", Host: "other.example", Port: 443,
				CredentialFile: "other.cred",
			}
			if err := upsertGatewayProfileMetadata(homePaths, oldProfile); err != nil {
				t.Fatalf("upsertGatewayProfileMetadata(old) error = %v", err)
			}
			if err := upsertGatewayProfileMetadata(homePaths, otherProfile); err != nil {
				t.Fatalf("upsertGatewayProfileMetadata(other) error = %v", err)
			}
			if err := setActiveGatewayProfile(homePaths, otherProfile.Name); err != nil {
				t.Fatalf("setActiveGatewayProfile() error = %v", err)
			}

			oldCredential := testGatewayCredential('o')
			credentials := map[string]string{"laptop": oldCredential, "other": testGatewayCredential('p')}
			gatewayAPI := &gatewayPairingAPIStub{issued: contract.GatewayIssuedCredentialPayload{
				Credential: testGatewayCredential('r'),
				Device:     contract.GatewayDevicePayload{ID: "device-new"},
			}}
			activationErr := errors.New("persist active profile failed")
			activationCalls := 0
			deps := commandDeps{
				resolveHome: func() (compozyconfig.HomePaths, error) { return homePaths, nil },
				loadConfig:  func() (compozyconfig.Config, error) { return compozyconfig.LoadForHome(homePaths) },
				newClient: func(ClientTarget) (DaemonClient, error) {
					return &gatewayPairingDaemonClient{DaemonClient: &stubClient{}, gatewayClientAPI: gatewayAPI}, nil
				},
				writeGatewayCredential: func(_ string, name string, credential string) (string, error) {
					credentials[name] = credential
					return name + ".cred", nil
				},
				readGatewayCredential: func(_ string, name string) (string, error) {
					credential, exists := credentials[name]
					if !exists {
						return "", os.ErrNotExist
					}
					return credential, nil
				},
				removeGatewayCredential: func(_ string, name string) error {
					delete(credentials, name)
					return nil
				},
				applyGatewayProfileState: func(
					paths compozyconfig.HomePaths,
					name string,
					profile *compozyconfig.GatewayConnectionConfig,
					active string,
				) error {
					activationCalls++
					if activationCalls == 1 {
						return activationErr
					}
					return applyGatewayProfileConfigState(paths, name, profile, active)
				},
			}

			_, _, err = executeRootCommand(
				t, deps,
				"pair", "redeem", "pairing-once", "--name", "laptop",
				"--address", "https://new.example", "--default-workspace", "new-workspace",
				"--overwrite", "--use", "-o", "json",
			)
			if !errors.Is(err, activationErr) {
				t.Fatalf("pair redeem error = %v, want activation failure", err)
			}
			if gatewayAPI.revocations.Load() != 0 {
				t.Fatalf("device revocations = %d, want 0", gatewayAPI.revocations.Load())
			}
			newCredential := credentials[oldProfile.Name]
			if newCredential == "" || newCredential == oldCredential {
				t.Fatalf("prepared credential = %q, want a new durable credential", newCredential)
			}
			loaded, err := compozyconfig.LoadForHome(homePaths)
			if err != nil {
				t.Fatalf("LoadForHome() error = %v", err)
			}
			restored, exists := findGatewayProfile(loaded.Gateway.Connections, oldProfile.Name)
			if !exists || restored != oldProfile || loaded.Gateway.ActiveConnection != otherProfile.Name {
				t.Fatalf(
					"restored profile = %#v exists:%t active:%q, want %#v active:%q",
					restored, exists, loaded.Gateway.ActiveConnection, oldProfile, otherProfile.Name,
				)
			}
			if err := recoverGatewayProfileState(t.Context(), homePaths, deps); err != nil {
				t.Fatalf("recoverGatewayProfileState() error = %v", err)
			}
			loaded, err = compozyconfig.LoadForHome(homePaths)
			if err != nil {
				t.Fatalf("LoadForHome(recovered) error = %v", err)
			}
			recovered, exists := findGatewayProfile(loaded.Gateway.Connections, oldProfile.Name)
			if !exists || recovered.Host != "new.example" || loaded.Gateway.ActiveConnection != oldProfile.Name {
				t.Fatalf(
					"recovered profile = %#v exists:%t active:%q",
					recovered,
					exists,
					loaded.Gateway.ActiveConnection,
				)
			}
			if credentials[oldProfile.Name] != newCredential {
				t.Fatalf("recovered credential = %q, want prepared credential", credentials[oldProfile.Name])
			}
			for name := range credentials {
				if strings.HasPrefix(name, gatewayProfileTransactionBackupPrefix) {
					t.Fatalf("previous credential backup %q still exists after recovery", name)
				}
			}
		},
	)

	t.Run("Should redeem a recovery profile without resolving the broken active credential", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		broken := compozyconfig.GatewayConnectionConfig{
			Name: "broken", Scheme: "https", Host: "broken.example", Port: 443,
			CredentialFile: "broken.cred",
		}
		if err := upsertGatewayProfileMetadata(homePaths, broken); err != nil {
			t.Fatalf("upsertGatewayProfileMetadata() error = %v", err)
		}
		if err := setActiveGatewayProfile(homePaths, broken.Name); err != nil {
			t.Fatalf("setActiveGatewayProfile() error = %v", err)
		}
		gatewayAPI := &gatewayPairingAPIStub{issued: contract.GatewayIssuedCredentialPayload{
			Credential: testGatewayCredential('t'),
			Device:     contract.GatewayDevicePayload{ID: "device-recovery"},
		}}
		var credentialReads atomic.Int32
		deps := commandDeps{
			resolveHome: func() (compozyconfig.HomePaths, error) { return homePaths, nil },
			loadConfig:  func() (compozyconfig.Config, error) { return compozyconfig.LoadForHome(homePaths) },
			newClient: func(ClientTarget) (DaemonClient, error) {
				return &gatewayPairingDaemonClient{DaemonClient: &stubClient{}, gatewayClientAPI: gatewayAPI}, nil
			},
			writeGatewayCredential: func(_ string, name string, _ string) (string, error) {
				return name + ".cred", nil
			},
			readGatewayCredential: func(string, string) (string, error) {
				credentialReads.Add(1)
				return "", errors.New("broken active credential")
			},
			removeGatewayCredential: func(string, string) error { return nil },
		}

		stdout, _, err := executeRootCommand(
			t, deps,
			"pair", "redeem", "pairing-recovery", "--name", "recovery",
			"--address", "https://recovery.example", "-o", "json",
		)
		if err != nil || !json.Valid([]byte(stdout)) {
			t.Fatalf("pair redeem recovery = stdout:%q err:%v", stdout, err)
		}
		if credentialReads.Load() != 0 {
			t.Fatalf("active credential reads = %d, want 0", credentialReads.Load())
		}
	})

	t.Run("Should restore profile metadata when credential removal fails", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		profile := compozyconfig.GatewayConnectionConfig{
			Name: "laptop", Scheme: "https", Host: "gateway.example", Port: 443,
			CredentialFile: "laptop.cred",
		}
		if err := upsertGatewayProfileMetadata(homePaths, profile); err != nil {
			t.Fatalf("upsertGatewayProfileMetadata() error = %v", err)
		}
		if err := setActiveGatewayProfile(homePaths, profile.Name); err != nil {
			t.Fatalf("setActiveGatewayProfile() error = %v", err)
		}
		removeErr := errors.New("credential removal failed")
		credential := testGatewayCredential('q')
		err = removeGatewayProfile(t.Context(), homePaths, profile.Name, commandDeps{
			readGatewayCredential: func(string, string) (string, error) { return credential, nil },
			writeGatewayCredential: func(string, string, string) (string, error) {
				return profile.Name + ".cred", nil
			},
			removeGatewayCredential: func(string, string) error { return removeErr },
		})
		if !errors.Is(err, removeErr) {
			t.Fatalf("removeGatewayProfile() error = %v, want credential removal failure", err)
		}
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(after rollback) error = %v", err)
		}
		restored, exists := findGatewayProfile(loaded.Gateway.Connections, profile.Name)
		if !exists || restored != profile || loaded.Gateway.ActiveConnection != profile.Name {
			t.Fatalf("restored profile = %#v exists:%t active:%q", restored, exists, loaded.Gateway.ActiveConnection)
		}
	})

	t.Run("Should roll forward a torn profile upsert from its credential digest", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		previous := compozyconfig.GatewayConnectionConfig{
			Name: "laptop", Scheme: "https", Host: "old.example", Port: 443,
			CredentialFile: "laptop.cred",
		}
		desired := previous
		desired.Host = "new.example"
		if err := upsertGatewayProfileMetadata(homePaths, previous); err != nil {
			t.Fatalf("upsertGatewayProfileMetadata(previous) error = %v", err)
		}
		newCredential := testGatewayCredential('n')
		journal, err := newGatewayProfileTransactionJournal(gatewayProfileTransactionPlan{
			operation:          gatewayProfileTransactionUpsert,
			profile:            desired,
			desiredActive:      desired.Name,
			credential:         newCredential,
			previousProfile:    previous,
			previousCredential: testGatewayCredential('o'),
			previousExists:     true,
		})
		if err != nil {
			t.Fatalf("newGatewayProfileTransactionJournal() error = %v", err)
		}
		if err := writeGatewayProfileTransactionJournal(homePaths.GatewayCredentialsDir, journal); err != nil {
			t.Fatalf("writeGatewayProfileTransactionJournal() error = %v", err)
		}
		deps := commandDeps{
			readGatewayCredential: func(string, string) (string, error) { return newCredential, nil },
			removeGatewayCredential: func(string, string) error {
				return nil
			},
		}
		if err := recoverGatewayProfileState(t.Context(), homePaths, deps); err != nil {
			t.Fatalf("recoverGatewayProfileState() error = %v", err)
		}
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		got, exists := findGatewayProfile(loaded.Gateway.Connections, desired.Name)
		if !exists || got != desired || loaded.Gateway.ActiveConnection != desired.Name {
			t.Fatalf(
				"recovered profile = %#v exists:%t active:%q, want %#v",
				got,
				exists,
				loaded.Gateway.ActiveConnection,
				desired,
			)
		}
		if _, err := readGatewayProfileTransactionJournal(
			homePaths.GatewayCredentialsDir,
			desired.Name,
		); !errors.Is(
			err,
			os.ErrNotExist,
		) {
			t.Fatalf("recovered journal error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("Should roll back torn metadata when the new credential was never written", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		other := compozyconfig.GatewayConnectionConfig{
			Name: "other", Scheme: "ssh", Host: "other.example", Port: 22,
		}
		desired := compozyconfig.GatewayConnectionConfig{
			Name: "laptop", Scheme: "https", Host: "new.example", Port: 443,
			CredentialFile: "laptop.cred",
		}
		if err := upsertGatewayProfileMetadata(homePaths, other); err != nil {
			t.Fatalf("upsertGatewayProfileMetadata(other) error = %v", err)
		}
		if err := applyGatewayProfileConfigState(homePaths, desired.Name, &desired, desired.Name); err != nil {
			t.Fatalf("applyGatewayProfileConfigState(torn) error = %v", err)
		}
		journal, err := newGatewayProfileTransactionJournal(gatewayProfileTransactionPlan{
			operation:      gatewayProfileTransactionUpsert,
			profile:        desired,
			desiredActive:  desired.Name,
			credential:     testGatewayCredential('n'),
			previousActive: other.Name,
		})
		if err != nil {
			t.Fatalf("newGatewayProfileTransactionJournal() error = %v", err)
		}
		if err := writeGatewayProfileTransactionJournal(homePaths.GatewayCredentialsDir, journal); err != nil {
			t.Fatalf("writeGatewayProfileTransactionJournal() error = %v", err)
		}
		deps := commandDeps{
			readGatewayCredential: func(string, string) (string, error) { return "", os.ErrNotExist },
			removeGatewayCredential: func(string, string) error {
				return nil
			},
		}
		if err := recoverGatewayProfileState(t.Context(), homePaths, deps); err != nil {
			t.Fatalf("recoverGatewayProfileState() error = %v", err)
		}
		loaded, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if _, exists := findGatewayProfile(loaded.Gateway.Connections, desired.Name); exists {
			t.Fatal("torn profile still exists after recovery")
		}
		if loaded.Gateway.ActiveConnection != other.Name {
			t.Fatalf("active profile = %q, want %q", loaded.Gateway.ActiveConnection, other.Name)
		}
	})

	t.Run("Should reject a concurrent pairing overwrite before redeeming another device", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		started := make(chan struct{})
		release := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
		gatewayAPI := &gatewayPairingAPIStub{
			issued: contract.GatewayIssuedCredentialPayload{
				Credential: testGatewayCredential('c'),
				Device:     contract.GatewayDevicePayload{ID: "device-concurrent"},
			},
			redeemStarted: started,
			redeemRelease: release,
		}
		credentials := map[string]string{}
		var credentialsMu sync.Mutex
		deps := commandDeps{
			resolveHome: func() (compozyconfig.HomePaths, error) { return homePaths, nil },
			loadConfig:  func() (compozyconfig.Config, error) { return compozyconfig.LoadForHome(homePaths) },
			newClient: func(ClientTarget) (DaemonClient, error) {
				return &gatewayPairingDaemonClient{DaemonClient: &stubClient{}, gatewayClientAPI: gatewayAPI}, nil
			},
			writeGatewayCredential: func(_ string, name string, credential string) (string, error) {
				credentialsMu.Lock()
				defer credentialsMu.Unlock()
				credentials[name] = credential
				return name + ".cred", nil
			},
			readGatewayCredential: func(_ string, name string) (string, error) {
				credentialsMu.Lock()
				defer credentialsMu.Unlock()
				credential, exists := credentials[name]
				if !exists {
					return "", os.ErrNotExist
				}
				return credential, nil
			},
			removeGatewayCredential: func(_ string, name string) error {
				credentialsMu.Lock()
				defer credentialsMu.Unlock()
				delete(credentials, name)
				return nil
			},
		}
		firstResult := make(chan error, 1)
		go func() {
			_, _, commandErr := executeRootCommand(
				t,
				deps,
				"pair", "redeem", "pairing-first", "--name", "laptop",
				"--address", "https://gateway.example", "--overwrite", "-o", "json",
			)
			firstResult <- commandErr
		}()
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("first pairing did not reach redemption")
		}
		_, _, secondErr := executeRootCommand(
			t,
			deps,
			"pair", "redeem", "pairing-second", "--name", "laptop",
			"--address", "https://gateway.example", "--overwrite", "-o", "json",
		)
		assertGatewayClientErrorCode(t, secondErr, "gateway_profile_busy")
		releaseOnce.Do(func() { close(release) })
		if err := <-firstResult; err != nil {
			t.Fatalf("first pairing error = %v", err)
		}
		if gatewayAPI.redeemCalls.Load() != 1 {
			t.Fatalf("pairing redemptions = %d, want 1", gatewayAPI.redeemCalls.Load())
		}
	})
}

type gatewayPairingAPIStub struct {
	gatewayClientAPI
	minted          contract.GatewayPairingArtifactPayload
	issued          contract.GatewayIssuedCredentialPayload
	revocations     atomic.Int32
	redeemCalls     atomic.Int32
	redeemStarted   chan struct{}
	redeemRelease   <-chan struct{}
	redeemStartOnce sync.Once
	beforeRedeem    func(contract.GatewayPairingRedeemRequest) error
	listDevicesErr  error
}

func (s *gatewayPairingAPIStub) MintGatewayPairing(
	context.Context,
) (contract.GatewayPairingArtifactPayload, error) {
	return s.minted, nil
}

func (s *gatewayPairingAPIStub) ListGatewayDevices(context.Context) ([]contract.GatewayDevicePayload, error) {
	return []contract.GatewayDevicePayload{}, s.listDevicesErr
}

func (s *gatewayPairingAPIStub) RedeemGatewayPairing(
	ctx context.Context,
	request contract.GatewayPairingRedeemRequest,
) (contract.GatewayIssuedCredentialPayload, error) {
	s.redeemCalls.Add(1)
	if s.beforeRedeem != nil {
		if err := s.beforeRedeem(request); err != nil {
			return contract.GatewayIssuedCredentialPayload{}, err
		}
	}
	if s.redeemStarted != nil {
		s.redeemStartOnce.Do(func() { close(s.redeemStarted) })
	}
	if s.redeemRelease != nil {
		select {
		case <-ctx.Done():
			return contract.GatewayIssuedCredentialPayload{}, ctx.Err()
		case <-s.redeemRelease:
		}
	}
	issued := s.issued
	issued.Credential = request.Credential
	return issued, nil
}

func (s *gatewayPairingAPIStub) RevokeGatewayDevice(
	context.Context,
	string,
) (contract.GatewayRevokePayload, error) {
	s.revocations.Add(1)
	return contract.GatewayRevokePayload{}, nil
}

type gatewayPairingDaemonClient struct {
	DaemonClient
	gatewayClientAPI
}

func assertGatewayClientErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var payloadErr interface{ errorPayload() contract.ErrorPayload }
	if !errors.As(err, &payloadErr) {
		t.Fatalf("error = %T %v, want structured gateway client error", err, err)
	}
	if payloadErr.errorPayload().Code != code {
		t.Fatalf("error code = %q, want %q", payloadErr.errorPayload().Code, code)
	}
}

func testGatewayCredential(character byte) string {
	return gatewayDeviceCredentialPrefix + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{character}, 32))
}
