package extensionpkg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/testutil"
)

func TestConnectivityProviderSubprocessConformance(t *testing.T) {
	t.Run("Should negotiate and call every connectivity provider method [UT-081, IT-030]", func(t *testing.T) {
		challenges := gateway.NewChallengeRegistry()
		server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			nonce, ok := challenges.Resolve(gateway.TierPrivate, request.URL.Path)
			if !ok {
				http.NotFound(writer, request)
				return
			}
			if _, err := writer.Write([]byte(nonce)); err != nil {
				return
			}
		}))
		t.Cleanup(server.Close)
		manager, _, err := startConnectivityConformanceManager(
			t, "connectivity-negotiated", "connectivity_endpoint", server.URL,
		)
		if err != nil {
			t.Fatalf("Manager.Start() error = %v", err)
		}
		supervisor := newConnectivityConformanceSupervisor(
			t, manager, "connectivity-negotiated", challenges, server.Client(),
		)
		activation := connectivityConformanceActivation("connectivity-negotiated")
		reachability, err := supervisor.Establish(
			testutil.Context(t), activation, netip.MustParseAddrPort("127.0.0.1:43210"),
		)
		if err != nil {
			t.Fatalf("Establish() error = %v", err)
		}
		if err := supervisor.Verify(testutil.Context(t), reachability); err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if err := supervisor.Advertise(
			testutil.Context(t), reachability, []gateway.Surface{gateway.SurfaceOperatorUI},
		); err != nil {
			t.Fatalf("Advertise() error = %v", err)
		}
		source, err := manager.ResolveConnectivitySource(testutil.Context(t), activation.ProviderName)
		if err != nil {
			t.Fatalf("ResolveConnectivitySource() error = %v", err)
		}
		status, err := source.Status(testutil.Context(t), gateway.TierPrivate)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.Tier != reachability.Tier || status.Health != gateway.HealthHealthy ||
			len(status.Endpoints) != 1 || status.Endpoints[0] != reachability.Endpoints[0] {
			t.Fatalf("Status() = %#v, want established reachability %#v", status, reachability)
		}
		if err := supervisor.Teardown(testutil.Context(t), activation); err != nil {
			t.Fatalf("Teardown() error = %v", err)
		}
		if reachability.Tier != gateway.TierPrivate || len(reachability.Endpoints) != 1 ||
			reachability.Endpoints[0].URL != server.URL {
			t.Fatalf("Reachability = %#v, want private endpoint", reachability)
		}
		extension, err := manager.Get("connectivity-negotiated")
		if err != nil {
			t.Fatalf("Manager.Get() error = %v", err)
		}
		if !extension.Status.Active || !extension.Status.Healthy {
			t.Fatalf("provider status = %#v, want negotiated active process", extension.Status)
		}
	})

	t.Run("Should canonicalize semantically identical endpoint descriptors", func(t *testing.T) {
		first, err := connectivityReachabilityFromWire(extensioncontract.ConnectivityReachability{
			Tier: "private", Health: "healthy",
			Endpoints: []extensioncontract.ConnectivityAdvertisedEndpoint{{
				URL: "HTTPS://Gateway.Example.Test/", Scheme: "HTTPS", Stability: "stable",
			}},
		})
		if err != nil {
			t.Fatalf("connectivityReachabilityFromWire(first) error = %v", err)
		}
		second, err := connectivityReachabilityFromWire(extensioncontract.ConnectivityReachability{
			Tier: "private", Health: "healthy",
			Endpoints: []extensioncontract.ConnectivityAdvertisedEndpoint{{
				URL: "https://gateway.example.test", Scheme: "https", Stability: "stable",
			}},
		})
		if err != nil {
			t.Fatalf("connectivityReachabilityFromWire(second) error = %v", err)
		}
		if first.Endpoints[0] != second.Endpoints[0] {
			t.Fatalf("canonical endpoints differ: %#v != %#v", first.Endpoints[0], second.Endpoints[0])
		}
	})

	t.Run("Should carry an explicit private endpoint scheme policy across the provider boundary", func(t *testing.T) {
		t.Parallel()

		reachability, err := connectivityReachabilityFromWire(extensioncontract.ConnectivityReachability{
			Tier: "private", Health: "healthy",
			Endpoints: []extensioncontract.ConnectivityAdvertisedEndpoint{{
				URL: "TSNET://Gateway.Example.Test/", Scheme: "TSNET", Stability: "stable",
				SchemePolicy: "tailnet_internal",
			}},
		})
		if err != nil {
			t.Fatalf("connectivityReachabilityFromWire() error = %v", err)
		}
		if got := reachability.Endpoints[0]; got.URL != "tsnet://gateway.example.test" ||
			got.Scheme != "tsnet" || got.SchemePolicy != gateway.EndpointSchemePolicyTailnetInternal {
			t.Fatalf("endpoint = %#v, want canonical tailnet policy descriptor", got)
		}
	})

	t.Run("Should reject a real provider response for the wrong tier [IT-031]", func(t *testing.T) {
		manager, _, err := startConnectivityConformanceManager(
			t, "connectivity-wrong-tier", "connectivity_wrong_tier", "",
		)
		if err != nil {
			t.Fatalf("Manager.Start() error = %v", err)
		}
		challenges := gateway.NewChallengeRegistry()
		supervisor := newConnectivityConformanceSupervisor(
			t,
			manager,
			"connectivity-wrong-tier",
			challenges,
			&http.Client{Timeout: 50 * time.Millisecond},
		)
		activation := connectivityConformanceActivation("connectivity-wrong-tier")
		_, err = supervisor.Establish(
			testutil.Context(t), activation, netip.MustParseAddrPort("127.0.0.1:43210"),
		)
		if err != nil {
			if !errors.Is(err, gateway.ErrEndpointUnverified) || !strings.Contains(err.Error(), "wrong tier") {
				t.Fatalf("Establish() error = %v, want wrong-tier refusal", err)
			}
		} else {
			t.Fatal("Establish() error = nil, want wrong-tier refusal")
		}
		if err := supervisor.Advertise(testutil.Context(t), gateway.Reachability{
			Tier: gateway.TierPrivate, Health: gateway.HealthHealthy,
		}, nil); !errors.Is(err, gateway.ErrEndpointUnverified) {
			t.Fatalf("Advertise(unverified) error = %v, want ErrEndpointUnverified", err)
		}
	})

	t.Run("Should reject malformed provider JSON without crashing the manager [UT-082]", func(t *testing.T) {
		restartLogPath := filepath.Join(t.TempDir(), "malformed-restarts.jsonl")
		manager, _, err := startConnectivityConformanceManager(
			t,
			"connectivity-malformed",
			"connectivity_malformed",
			restartLogPath,
		)
		if err != nil {
			t.Fatalf("Manager.Start() error = %v", err)
		}
		waitForManagerCondition(t, 2*time.Second, func() bool {
			return markerLineCount(restartLogPath) >= 2
		})
		if _, err := manager.Get("connectivity-malformed"); err != nil {
			t.Fatalf("Manager.Get() after malformed payload error = %v", err)
		}
	})

	t.Run("Should reject a provider method that is unimplemented at call time [UT-083]", func(t *testing.T) {
		manager, _, err := startConnectivityConformanceManager(
			t, "connectivity-unimplemented", "connectivity_method_not_found", "",
		)
		if err != nil {
			t.Fatalf("Manager.Start() error = %v", err)
		}
		source, err := manager.ResolveConnectivitySource(testutil.Context(t), "connectivity-unimplemented")
		if err != nil {
			t.Fatalf("ResolveConnectivitySource() error = %v", err)
		}
		err = source.Teardown(testutil.Context(t), gateway.TierPrivate, time.Now().Add(time.Second))
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "method not found") {
			t.Fatalf("Teardown() error = %v, want method not found", err)
		}
	})

	t.Run("Should supervise crash restart and bounded teardown [UT-084]", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "crash-restarts.jsonl")
		manager, _, err := startConnectivityConformanceManager(
			t, "connectivity-crash", "connectivity_crash_after_establish", marker,
		)
		if err != nil {
			t.Fatalf("Manager.Start() error = %v", err)
		}
		source, err := manager.ResolveConnectivitySource(testutil.Context(t), "connectivity-crash")
		if err != nil {
			t.Fatalf("ResolveConnectivitySource() error = %v", err)
		}
		if _, err := source.Establish(testutil.Context(t), gateway.EstablishRequest{
			Tier: gateway.TierPrivate, ForwardTarget: netip.MustParseAddrPort("127.0.0.1:43210"),
			ChallengePath: gateway.ChallengePathPrefix + "crash", Deadline: time.Now().Add(time.Second),
		}); err != nil {
			t.Fatalf("Establish() error = %v", err)
		}
		waitForManagerCondition(t, 2*time.Second, func() bool {
			extension, getErr := manager.Get("connectivity-crash")
			return markerLineCount(marker) >= 3 && getErr == nil && extension.Status.Active && extension.Status.Healthy
		})
		stopCtx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()
		if err := source.Teardown(stopCtx, gateway.TierPrivate, time.Now().Add(200*time.Millisecond)); err != nil {
			t.Fatalf("Teardown(after restart) error = %v", err)
		}
	})

	t.Run("Should abort and restart a provider whose teardown hangs [UT-079, UT-084]", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "teardown-restarts.jsonl")
		manager, _, err := startConnectivityConformanceManager(
			t, "connectivity-teardown-hang", "connectivity_teardown_hang", marker,
		)
		if err != nil {
			t.Fatalf("Manager.Start() error = %v", err)
		}
		source, err := manager.ResolveConnectivitySource(testutil.Context(t), "connectivity-teardown-hang")
		if err != nil {
			t.Fatalf("ResolveConnectivitySource() error = %v", err)
		}
		teardownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		started := time.Now()
		err = source.Teardown(teardownCtx, gateway.TierPrivate, time.Now().Add(25*time.Millisecond))
		cancel()
		if err == nil {
			t.Fatal("Teardown() error = nil, want deadline")
		}
		if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
			t.Fatalf("Teardown() elapsed = %v, want bounded abort", elapsed)
		}
		waitForManagerCondition(t, 2*time.Second, func() bool {
			extension, getErr := manager.Get("connectivity-teardown-hang")
			return markerLineCount(marker) >= 2 && getErr == nil && extension.Status.Active
		})
	})

	t.Run("Should abort the exact provider process that refuses teardown", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "teardown-refused-restarts.jsonl")
		manager, _, err := startConnectivityConformanceManager(
			t, "connectivity-teardown-refused", "connectivity_teardown_refused", marker,
		)
		if err != nil {
			t.Fatalf("Manager.Start() error = %v", err)
		}
		source, err := manager.ResolveConnectivitySource(testutil.Context(t), "connectivity-teardown-refused")
		if err != nil {
			t.Fatalf("ResolveConnectivitySource() error = %v", err)
		}
		if err := source.Teardown(
			testutil.Context(t), gateway.TierPrivate, time.Now().Add(time.Second),
		); err == nil || !strings.Contains(err.Error(), "did not acknowledge") {
			t.Fatalf("Teardown() error = %v, want refusal", err)
		}
		waitForManagerCondition(t, 2*time.Second, func() bool {
			extension, getErr := manager.Get("connectivity-teardown-refused")
			return markerLineCount(marker) >= 2 && getErr == nil && extension.Status.Active
		})
	})

	t.Run("Should redact credential-shaped raw provider stdout [UT-085]", func(t *testing.T) {
		_, logs, err := startConnectivityConformanceManager(
			t, "connectivity-secret", "connectivity_secret_stdout", "",
		)
		if err == nil {
			t.Fatal("Manager.Start() error = nil, want provider initialize failure")
		}
		output := logs.String()
		if strings.Contains(output, "sk-connectivity-secret") {
			t.Fatalf("manager logs leaked provider credential: %q", output)
		}
	})
}

type connectivityConformanceTrust struct{}

func (connectivityConformanceTrust) ResolveProviderTrust(context.Context, string) (gateway.ProviderTrust, error) {
	return gateway.ProviderTrust{
		InstallSource: gateway.ProviderInstallSourceBundled,
		ChannelScopes: []string{
			gateway.ProviderChannelScope(gateway.TierPrivate),
			gateway.ProviderChannelScope(gateway.TierPublic),
		},
	}, nil
}

type connectivityConformanceIdentity struct{ name string }

func (identity connectivityConformanceIdentity) ProviderIdentity(
	context.Context,
	string,
) (gateway.ProviderIdentity, error) {
	return gateway.ProviderIdentity{Name: identity.name, InstallSource: gateway.ProviderInstallSourceBundled}, nil
}

type connectivityConformanceVerifier struct {
	client  *http.Client
	timeout time.Duration
}

func (v connectivityConformanceVerifier) Timeout() time.Duration { return v.timeout }

func (v connectivityConformanceVerifier) Verify(
	ctx context.Context,
	_ gateway.Tier,
	endpoint gateway.AdvertisedEndpoint,
	path string,
	nonce string,
) error {
	base, err := url.Parse(endpoint.URL)
	if err != nil {
		return err
	}
	base.Path = path
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), http.NoBody)
	if err != nil {
		return err
	}
	response, err := v.client.Do(request)
	if err != nil {
		return err
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 1024))
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK || string(body) != nonce {
		return gateway.ErrEndpointUnverified
	}
	return nil
}

func newConnectivityConformanceSupervisor(
	t *testing.T,
	manager *Manager,
	providerName string,
	challenges *gateway.ChallengeRegistry,
	client *http.Client,
) *gateway.ProviderSupervisor {
	t.Helper()
	supervisor, err := gateway.NewProviderSupervisor(
		manager,
		connectivityConformanceTrust{},
		connectivityConformanceIdentity{name: providerName},
		challenges,
		connectivityConformanceVerifier{client: client, timeout: time.Second},
		gateway.WithProviderSupervisorIntervals(time.Hour, 100*time.Millisecond, 100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewProviderSupervisor() error = %v", err)
	}
	return supervisor
}

func connectivityConformanceActivation(name string) gateway.ProviderActivation {
	return gateway.ProviderActivation{
		ProviderName: name,
		Tier:         gateway.TierPrivate,
		Desired:      gateway.DesiredEnabled,
		Generation:   1,
	}
}

func startConnectivityConformanceManager(
	t *testing.T,
	name string,
	scenario string,
	scenarioValue string,
) (*Manager, *bytes.Buffer, error) {
	t.Helper()
	withDaemonVersion(t, "0.6.0")
	env := newRegistryTestEnv(t)
	fixture := createManagerTestExtension(
		t,
		connectivityConformanceManifest(t, name, scenario, scenarioValue),
		nil,
	)
	installManagerFixture(t, env.registry, fixture, SourceBundled, true)
	var logs bytes.Buffer
	manager := NewManager(
		env.registry,
		WithLogger(slog.New(slog.NewTextHandler(&logs, nil))),
		WithInitializeTimeout(200*time.Millisecond),
		WithHealthCheckTimeout(50*time.Millisecond),
		WithSubprocessSignalGrace(20*time.Millisecond),
		withRestartBackoffMax(10*time.Millisecond),
		withRestartFailureThreshold(3),
		withHealthPollBounds(5*time.Millisecond, 20*time.Millisecond),
	)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := manager.Stop(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			if scenario == "connectivity_malformed" && strings.Contains(err.Error(), "decode frame") {
				return
			}
			t.Errorf("Manager.Stop(cleanup) error = %v", err)
		}
	})
	err := manager.Start(testutil.Context(t))
	if err == nil && scenario != "connectivity_malformed" && scenario != "connectivity_crash" {
		waitForManagerCondition(t, time.Second, func() bool {
			extension, getErr := manager.Get(name)
			return getErr == nil && extension.Status.Active && extension.Status.Healthy
		})
	}
	return manager, &logs, err
}

func connectivityConformanceManifest(
	t *testing.T,
	name string,
	scenario string,
	scenarioValue string,
) string {
	t.Helper()
	return fmt.Sprintf(`[extension]
name = %q
version = "0.1.0"
description = "Connectivity subprocess conformance fixture"
min_compozy_version = "0.6.0"

[network_participation]
required = true
mode = "live"
channel_scopes = ["gateway.private", "gateway.public"]

[capabilities]
provides = ["connectivity.provider"]

[permissions]
requires = []

[subprocess]
command = %q
args = ["-test.run=TestExtensionManagerHelperProcess"]
health_check_interval = "10ms"
shutdown_timeout = "40ms"

[subprocess.env]
%s = "1"
%s = %q
%s = %q
	`, name, helperCommand(t), extensionHelperEnvKey, extensionHelperScenarioKey, scenario, extensionHelperMarkerKey, scenarioValue)
}
