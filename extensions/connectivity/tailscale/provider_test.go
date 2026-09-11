package tailscale

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	compozysdk "github.com/compozy/compozy/sdk/go"
)

func TestBundledProviderEstablishesBothTiers(t *testing.T) {
	t.Parallel()
	privateListener := newTestListener(t)
	publicListener := newTestListener(t)
	node := &fakeTailscaleNode{
		privateListener: privateListener,
		publicListener:  publicListener,
		domains:         []string{"gateway.example.ts.net"},
	}
	provider, err := NewProvider(filepath.Join(t.TempDir(), "state"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	provider.newNode = func(string, *slog.Logger) (tailscaleNode, error) { return node, nil }
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := provider.Close(ctx); err != nil {
			t.Errorf("Provider.Close(cleanup) error = %v", err)
		}
	})

	for _, tt := range []struct {
		tier    string
		wantURL string
	}{
		{tier: tierPrivate, wantURL: "https://gateway.example.ts.net:8443"},
		{tier: tierPublic, wantURL: "https://gateway.example.ts.net"},
	} {
		t.Run("Should establish "+tt.tier, func(t *testing.T) {
			reachability, err := provider.Establish(testutil.Context(t), compozysdk.ExtensionContext{},
				compozysdk.ConnectivityEstablishRequest{
					Tier:          tt.tier,
					ForwardTarget: "127.0.0.1:43210",
					ChallengePath: challengePathPrefix + tt.tier,
					Deadline:      time.Now().Add(time.Second),
				})
			if err != nil {
				t.Fatalf("Establish() error = %v", err)
			}
			if reachability.Health != "healthy" || len(reachability.Endpoints) != 1 ||
				reachability.Endpoints[0].URL != tt.wantURL {
				t.Fatalf("Reachability = %#v, want %s", reachability, tt.wantURL)
			}
		})
	}
	if node.upCalls != 2 || node.privateCalls != 1 || node.provisionCalls != 1 || node.publicCalls != 1 {
		t.Fatalf(
			"node calls = up:%d private:%d provision:%d public:%d",
			node.upCalls,
			node.privateCalls,
			node.provisionCalls,
			node.publicCalls,
		)
	}
	if node.provisionDomain != "gateway.example.ts.net" {
		t.Fatalf("provision domain = %q, want gateway.example.ts.net", node.provisionDomain)
	}
	for _, tier := range []string{tierPrivate, tierPublic} {
		response, err := provider.Teardown(testutil.Context(t), compozysdk.ExtensionContext{},
			compozysdk.ConnectivityTeardownRequest{Tier: tier, Deadline: time.Now().Add(time.Second)})
		if err != nil || !response.Stopped {
			t.Fatalf("Teardown(%s) = (%#v, %v), want stopped", tier, response, err)
		}
	}
	if node.closeCalls != 1 {
		t.Fatalf("node close calls = %d, want 1 after final tier", node.closeCalls)
	}
}

func TestTSNetNodeOperationOwnership(t *testing.T) {
	t.Parallel()

	t.Run("Should join a late listener before returning its context error", func(t *testing.T) {
		t.Parallel()
		listener := newTestListener(t)
		started := make(chan struct{})
		release := make(chan struct{})
		ctx, cancel := context.WithCancel(testutil.Context(t))
		result := make(chan error, 1)
		go func() {
			_, err := listenWithContext(ctx, func() (net.Listener, error) {
				close(started)
				<-release
				return listener, nil
			})
			result <- err
		}()
		<-started
		cancel()
		select {
		case err := <-result:
			t.Fatalf("listenWithContext() returned before listener joined: %v", err)
		default:
		}
		close(release)
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("listenWithContext() error = %v, want context canceled", err)
		}
		if _, err := listener.Accept(); !errors.Is(err, net.ErrClosed) {
			t.Fatalf("Accept() error = %v, want closed late listener", err)
		}
	})

	t.Run("Should join node close before returning its context error", func(t *testing.T) {
		t.Parallel()
		started := make(chan struct{})
		release := make(chan struct{})
		ctx, cancel := context.WithCancel(testutil.Context(t))
		result := make(chan error, 1)
		go func() {
			result <- runContextBoundOperation(ctx, func() error {
				close(started)
				<-release
				return nil
			})
		}()
		<-started
		cancel()
		select {
		case err := <-result:
			t.Fatalf("runContextBoundOperation() returned before close joined: %v", err)
		default:
		}
		close(release)
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("runContextBoundOperation() error = %v, want context canceled", err)
		}
	})

	t.Run("Should still run cleanup when the caller context is already canceled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		called := false
		err := runContextBoundOperation(ctx, func() error {
			called = true
			return nil
		})
		if !called || !errors.Is(err, context.Canceled) {
			t.Fatalf(
				"runContextBoundOperation() = (called:%t, error:%v), want cleanup and context canceled",
				called,
				err,
			)
		}
	})
}

func TestTierForwarderBridgesToDaemonLoopback(t *testing.T) {
	t.Parallel()

	t.Run("Should forward tier requests with the browser-facing scheme and host", func(t *testing.T) {
		t.Parallel()
		const tierHost = "compozy-gateway.example.ts.net:8443"
		target := newTestListener(t)
		captured := make(chan forwardedRequestCapture, 1)
		backend := &http.Server{
			Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				captured <- forwardedRequestCapture{
					host:            request.Host,
					forwardedProto:  request.Header.Get(forwardedProtoHeader),
					forwardedHost:   request.Header.Get(forwardedHostHeader),
					forwardedHeader: request.Header.Get("Forwarded"),
				}
				writer.Header().Set("Content-Type", "text/plain")
				writer.WriteHeader(http.StatusOK)
				if _, err := writer.Write([]byte("ok")); err != nil {
					t.Errorf("write backend response error = %v", err)
				}
			}),
		}
		serveErr := make(chan error, 1)
		go func() { serveErr <- backend.Serve(target) }()
		t.Cleanup(func() {
			if err := backend.Close(); err != nil && !errors.Is(err, net.ErrClosed) &&
				!errors.Is(err, http.ErrServerClosed) {
				t.Errorf("Close(backend) error = %v", err)
			}
		})
		external := newTestListener(t)
		forwarder, err := newTierForwarder(
			external,
			target.Addr().String(),
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
		if err != nil {
			t.Fatalf("newTierForwarder() error = %v", err)
		}
		forwarder.Start()
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := forwarder.Close(ctx); err != nil {
				t.Errorf("Close(forwarder) error = %v", err)
			}
		})

		ctx, cancel := context.WithTimeout(testutil.Context(t), 5*time.Second)
		defer cancel()
		// The browser request arrives with a spoofable set of client-supplied
		// forwarding headers; the provider terminates TLS and owns the
		// truthful forwarded metadata.
		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			"http://"+external.Addr().String()+"/assets/app.js",
			http.NoBody,
		)
		if err != nil {
			t.Fatalf("NewRequestWithContext() error = %v", err)
		}
		request.Host = tierHost
		request.Header.Set("Origin", "https://"+tierHost)
		request.Header.Set(forwardedProtoHeader, "http")
		request.Header.Set(forwardedHostHeader, "evil.example.test")
		request.Header.Set("Forwarded", "for=198.51.100.9")
		response, err := (&http.Client{}).Do(request)
		if err != nil {
			t.Fatalf("forwarded request error = %v", err)
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			t.Fatalf("read forwarded response error = %v", err)
		}
		if response.StatusCode != http.StatusOK || string(body) != "ok" {
			t.Fatalf("forwarded response = %d/%q, want 200/ok", response.StatusCode, body)
		}
		var got forwardedRequestCapture
		select {
		case got = <-captured:
		case <-ctx.Done():
			t.Fatalf("backend capture error = %v", ctx.Err())
		}
		if got.host != tierHost {
			t.Fatalf("backend request Host = %q, want %q", got.host, tierHost)
		}
		if got.forwardedProto != forwardedProto {
			t.Fatalf(
				"backend %s = %q, want %q (client value must be overridden)",
				forwardedProtoHeader,
				got.forwardedProto,
				forwardedProto,
			)
		}
		if got.forwardedHost != tierHost {
			t.Fatalf(
				"backend %s = %q, want %q (client value must be overridden)",
				forwardedHostHeader,
				got.forwardedHost,
				tierHost,
			)
		}
		if got.forwardedHeader != "" {
			t.Fatalf("backend Forwarded = %q, want stripped client header", got.forwardedHeader)
		}
		if err := forwarder.Close(ctx); err != nil {
			t.Fatalf("Close(forwarder) error = %v", err)
		}
	})
}

type forwardedRequestCapture struct {
	host            string
	forwardedProto  string
	forwardedHost   string
	forwardedHeader string
}

func TestPrivateVerificationRelayLifecycle(t *testing.T) {
	t.Parallel()
	t.Run("Should forward the challenge through the private listener and close the relay", func(t *testing.T) {
		t.Parallel()
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != challengePathPrefix+"private" || r.Host != "example.com:8443" {
				t.Errorf("changed challenge request: %s %s", r.Host, r.URL.Path)
			}
			if _, err := io.WriteString(w, "exact-nonce"); err != nil {
				t.Errorf("write challenge: %v", err)
			}
		}))
		t.Cleanup(target.Close)
		certificateSource := httptest.NewTLSServer(http.NotFoundHandler())
		t.Cleanup(certificateSource.Close)
		roots := x509.NewCertPool()
		roots.AddCert(certificateSource.Certificate())
		privateListener := tls.NewListener(newTestListener(t), certificateSource.TLS.Clone())
		node := &fakeTailscaleNode{privateListener: privateListener, domains: []string{"example.com"}}
		provider, err := NewProvider(t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
		if err != nil {
			t.Fatal(err)
		}
		provider.newNode = func(string, *slog.Logger) (tailscaleNode, error) { return node, nil }
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := provider.Close(ctx); err != nil {
				t.Errorf("close provider: %v", err)
			}
		})
		req := compozysdk.ConnectivityEstablishRequest{
			Tier:          tierPrivate,
			ForwardTarget: strings.TrimPrefix(target.URL, "http://"),
			ChallengePath: challengePathPrefix + "private",
			Deadline:      time.Now().Add(time.Second),
		}
		reachable, err := provider.Establish(t.Context(), compozysdk.ExtensionContext{}, req)
		if err != nil {
			t.Fatal(err)
		}
		address := reachable.Endpoints[0].VerificationAddress
		if address == "" {
			t.Fatal("missing verification address")
		}
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots},
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, address)
			},
		}
		t.Cleanup(transport.CloseIdleConnections)
		client := &http.Client{Transport: transport, Timeout: time.Second}
		request, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"https://example.com:8443"+req.ChallengePath,
			http.NoBody,
		)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK || string(body) != "exact-nonce" {
			t.Fatalf("challenge: status=%d body=%q read=%v close=%v", response.StatusCode, body, readErr, closeErr)
		}
		status, err := provider.Status(
			t.Context(),
			compozysdk.ExtensionContext{},
			compozysdk.ConnectivityStatusRequest{Tier: tierPrivate},
		)
		if err != nil || status.Endpoints[0] != reachable.Endpoints[0] {
			t.Fatalf("status drift: %#v %v", status, err)
		}
		stopped, err := provider.Teardown(
			t.Context(),
			compozysdk.ExtensionContext{},
			compozysdk.ConnectivityTeardownRequest{Tier: tierPrivate, Deadline: time.Now().Add(time.Second)},
		)
		if err != nil || !stopped.Stopped {
			t.Fatalf("teardown: %#v %v", stopped, err)
		}
		conn, err := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", address)
		if err == nil {
			if closeErr := conn.Close(); closeErr != nil {
				t.Errorf("close connection: %v", closeErr)
			}
			t.Fatal("relay remained reachable after teardown")
		}
	})
}

func TestBundledManagedInstall(t *testing.T) {
	t.Parallel()
	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	db, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Errorf("Close(global DB) error = %v", err)
		}
	})
	registry := extensionpkg.NewRegistry(db.DB())
	if err := EnsureManagedInstall(homePaths, registry); err != nil {
		t.Fatalf("EnsureManagedInstall() error = %v", err)
	}
	installed, err := registry.Get(Name)
	if err != nil {
		t.Fatalf("Registry.Get() error = %v", err)
	}
	if installed.Source != extensionpkg.SourceBundled || !installed.Enabled ||
		installed.NetworkRequirementDigest == "" {
		t.Fatalf("installed provider = %#v, want enabled bundled live provider", installed)
	}
}

func TestBundledProviderFailureBounds(t *testing.T) {
	t.Parallel()

	t.Run("Should honor teardown deadline while establish owns serialization [UT-079]", func(t *testing.T) {
		t.Parallel()
		upStarted := make(chan struct{})
		upRelease := make(chan struct{})
		node := &fakeTailscaleNode{
			privateListener: newTestListener(t),
			domains:         []string{"gateway.example.ts.net"},
			upStarted:       upStarted,
			upRelease:       upRelease,
		}
		provider := newProviderWithNode(t, node)
		established := make(chan error, 1)
		go func() {
			_, err := provider.Establish(
				context.Background(),
				compozysdk.ExtensionContext{},
				establishRequest(tierPrivate),
			)
			established <- err
		}()
		<-upStarted
		deadline := time.Now().Add(25 * time.Millisecond)
		started := time.Now()
		_, err := provider.Teardown(
			context.Background(),
			compozysdk.ExtensionContext{},
			compozysdk.ConnectivityTeardownRequest{Tier: tierPrivate, Deadline: deadline},
		)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Teardown() error = %v, want context deadline", err)
		}
		if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
			t.Fatalf("Teardown() elapsed = %v, want bounded return", elapsed)
		}
		close(upRelease)
		if err := <-established; err != nil {
			t.Fatalf("Establish() error = %v", err)
		}
		_, err = provider.Teardown(
			testutil.Context(t),
			compozysdk.ExtensionContext{},
			compozysdk.ConnectivityTeardownRequest{
				Tier: tierPrivate, Deadline: time.Now().Add(time.Second),
			},
		)
		if err != nil {
			t.Fatalf("Teardown(retry) error = %v", err)
		}
	})

	t.Run("Should report node and domain failures as degraded", func(t *testing.T) {
		t.Parallel()
		node := &fakeTailscaleNode{
			privateListener: newTestListener(t),
			domains:         []string{"gateway.example.ts.net"},
		}
		provider := newProviderWithNode(t, node)
		_, err := provider.Establish(
			testutil.Context(t),
			compozysdk.ExtensionContext{},
			establishRequest(tierPrivate),
		)
		if err != nil {
			t.Fatalf("Establish() error = %v", err)
		}
		node.mu.Lock()
		node.healthErr = errors.New("auth revoked secret=must-not-escape")
		node.mu.Unlock()
		status, err := provider.Status(
			testutil.Context(t),
			compozysdk.ExtensionContext{},
			compozysdk.ConnectivityStatusRequest{Tier: tierPrivate},
		)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.Health != "degraded" || status.Reason != "connectivity is unavailable" {
			t.Fatalf("Status() = %#v, want redacted degradation", status)
		}
	})
}

func TestTierForwarderFailureHealth(t *testing.T) {
	t.Parallel()

	t.Run("Should degrade after an unexpected listener close", func(t *testing.T) {
		t.Parallel()
		listener := newTestListener(t)
		forwarder, err := newTierForwarder(listener, "127.0.0.1:1", slog.New(slog.NewTextHandler(io.Discard, nil)))
		if err != nil {
			t.Fatalf("newTierForwarder() error = %v", err)
		}
		forwarder.Start()
		if err := listener.Close(); err != nil {
			t.Fatalf("Close(listener) error = %v", err)
		}
		waitForForwarderHealth(t, forwarder, "degraded", "listener stopped accepting connections")
		if err := forwarder.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(forwarder) error = %v", err)
		}
	})

	t.Run("Should degrade when daemon loopback cannot be reached", func(t *testing.T) {
		t.Parallel()
		target := newTestListener(t)
		targetAddress := target.Addr().String()
		if err := target.Close(); err != nil {
			t.Fatalf("Close(target) error = %v", err)
		}
		external := newTestListener(t)
		forwarder, err := newTierForwarder(external, targetAddress, slog.New(slog.NewTextHandler(io.Discard, nil)))
		if err != nil {
			t.Fatalf("newTierForwarder() error = %v", err)
		}
		forwarder.Start()
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := forwarder.Close(ctx); err != nil {
				t.Errorf("Close(forwarder) error = %v", err)
			}
		})
		// The proxy dials the daemon loopback target per request, so drive a
		// real HTTP request to make the unavailable target observable.
		ctx, cancel := context.WithTimeout(testutil.Context(t), 5*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			"http://"+external.Addr().String()+"/api/status",
			http.NoBody,
		)
		if err != nil {
			t.Fatalf("NewRequestWithContext() error = %v", err)
		}
		response, err := (&http.Client{}).Do(request)
		if err != nil {
			t.Fatalf("forwarded request error = %v", err)
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			t.Fatalf("read forwarded response error = %v", err)
		}
		if response.StatusCode != http.StatusBadGateway {
			t.Fatalf("unreachable-target response = %d/%s, want 502", response.StatusCode, body)
		}
		waitForForwarderHealth(t, forwarder, "degraded", "forward target is unavailable")
	})
}

func TestBundledProviderDefinitionCarriesStateAndCredentialContract(t *testing.T) {
	t.Parallel()
	definition := providerDefinition()
	if len(definition.RequiresEnv) != 1 || definition.RequiresEnv[0] != "TS_AUTHKEY" {
		t.Fatalf("RequiresEnv = %#v, want TS_AUTHKEY", definition.RequiresEnv)
	}
	if definition.Subprocess.Env["COMPOZY_HOME"] != "{{env:COMPOZY_HOME}}" {
		t.Fatalf("Subprocess.Env = %#v, want COMPOZY_HOME propagation", definition.Subprocess.Env)
	}
	if definition.Name != Name || definition.Version != "0.1.0" ||
		definition.Subprocess.Command != "{{compozy_executable}}" ||
		!slices.Equal(definition.Subprocess.Args, []string{"__internal", "extension-provider", Name}) {
		t.Fatalf("provider definition identity/subprocess = %#v", definition)
	}
	if !slices.Equal(
		definition.Capabilities.Provides,
		[]string{compozysdk.CapabilityProvideConnectivityProvider},
	) {
		t.Fatalf("Capabilities.Provides = %#v, want connectivity provider", definition.Capabilities.Provides)
	}
	if definition.NetworkParticipation == nil || !definition.NetworkParticipation.Required ||
		definition.NetworkParticipation.Mode != "live" ||
		!slices.Equal(
			definition.NetworkParticipation.ChannelScopes,
			[]string{"gateway.private", "gateway.public"},
		) {
		t.Fatalf("NetworkParticipation = %#v, want live gateway scopes", definition.NetworkParticipation)
	}

	manifestData, err := fs.ReadFile(FS(), "extension.json")
	if err != nil {
		t.Fatalf("ReadFile(extension.json) error = %v", err)
	}
	manifestDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(manifestDir, "extension.json"), manifestData, 0o600); err != nil {
		t.Fatalf("WriteFile(extension.json) error = %v", err)
	}
	manifest, err := extensionpkg.LoadManifest(manifestDir)
	if err != nil {
		t.Fatalf("LoadManifest(extension.json) error = %v", err)
	}
	if definition.Name != manifest.Name || definition.Version != manifest.Version ||
		definition.Description != manifest.Description ||
		!slices.Equal(definition.RequiresEnv, manifest.RequiresEnv) ||
		!slices.Equal(definition.Capabilities.Provides, manifest.Capabilities.Provides) ||
		definition.Subprocess.Command != manifest.Subprocess.Command ||
		!slices.Equal(definition.Subprocess.Args, manifest.Subprocess.Args) ||
		!maps.Equal(definition.Subprocess.Env, manifest.Subprocess.Env) {
		t.Fatalf("providerDefinition() = %#v, want parity with extension.json %#v", definition, manifest)
	}
	if manifest.NetworkParticipation == nil || definition.NetworkParticipation == nil ||
		definition.NetworkParticipation.Required != manifest.NetworkParticipation.Required ||
		definition.NetworkParticipation.Mode != manifest.NetworkParticipation.Mode ||
		!slices.Equal(
			definition.NetworkParticipation.ChannelScopes,
			manifest.NetworkParticipation.ChannelScopes,
		) {
		t.Fatalf(
			"providerDefinition network participation = %#v, manifest = %#v",
			definition.NetworkParticipation,
			manifest.NetworkParticipation,
		)
	}
}

func TestBundledProviderRPCErrorContract(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the missing auth-key binding without exposing a credential value", func(t *testing.T) {
		t.Parallel()

		err := providerRPCError(errAuthKeyBindingRequired)
		rpcErr, ok := errors.AsType[*compozysdk.RPCError](err)
		if !ok {
			t.Fatalf("providerRPCError() error = %T, want *compozysdk.RPCError", err)
		}
		if rpcErr.Code != providerConfigurationRPCErrorCode || rpcErr.Message != "TS_AUTHKEY binding is required" {
			t.Fatalf("providerRPCError() = %#v, want actionable configuration RPC error", rpcErr)
		}
		if len(rpcErr.Data) != 0 {
			t.Fatalf("providerRPCError() data = %s, want no credential-bearing data", rpcErr.Data)
		}
	})

	t.Run("Should leave unclassified provider failures masked by the SDK", func(t *testing.T) {
		t.Parallel()

		providerErr := errors.New("provider failed with sensitive implementation detail")
		if got := providerRPCError(providerErr); !errors.Is(got, providerErr) {
			t.Fatalf("providerRPCError() error = %v, want original unclassified error", got)
		}
	})
}

type fakeTailscaleNode struct {
	mu              sync.Mutex
	privateListener net.Listener
	publicListener  net.Listener
	domains         []string
	upCalls         int
	privateCalls    int
	publicCalls     int
	provisionCalls  int
	provisionDomain string
	provisioned     bool
	closeCalls      int
	upStarted       chan struct{}
	upRelease       <-chan struct{}
	upStartedOnce   sync.Once
	healthErr       error
}

func (n *fakeTailscaleNode) Up(context.Context) error {
	n.mu.Lock()
	n.upCalls++
	started := n.upStarted
	release := n.upRelease
	n.mu.Unlock()
	if started != nil {
		n.upStartedOnce.Do(func() { close(started) })
	}
	if release != nil {
		<-release
	}
	return nil
}

func (n *fakeTailscaleNode) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" || len(n.domains) == 0 || address != net.JoinHostPort(n.domains[0], privateListenerPort) {
		return nil, errors.New("unexpected private verification destination")
	}
	return (&net.Dialer{}).DialContext(ctx, network, n.privateListener.Addr().String())
}

func (n *fakeTailscaleNode) ListenPrivate(context.Context) (net.Listener, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.privateCalls++
	return n.privateListener, nil
}

func (n *fakeTailscaleNode) ListenPublic(context.Context) (net.Listener, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.publicCalls++
	if !n.provisioned {
		return nil, errors.New("public listener opened before certificate provisioning")
	}
	return n.publicListener, nil
}

func (n *fakeTailscaleNode) CertificateDomains() []string { return n.domains }

func (n *fakeTailscaleNode) ProvisionCertificate(_ context.Context, domain string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.provisionCalls++
	n.provisionDomain = domain
	n.provisioned = true
	return nil
}

func (n *fakeTailscaleNode) Health(context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.healthErr
}

func (n *fakeTailscaleNode) Close(context.Context) error {
	n.mu.Lock()
	n.closeCalls++
	n.mu.Unlock()
	return nil
}

func newTestListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(testutil.Context(t), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("Close(listener) error = %v", err)
		}
	})
	return listener
}

func newProviderWithNode(t *testing.T, node tailscaleNode) *Provider {
	t.Helper()
	provider, err := NewProvider(filepath.Join(t.TempDir(), "state"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	provider.newNode = func(string, *slog.Logger) (tailscaleNode, error) { return node, nil }
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := provider.Close(ctx); err != nil {
			t.Errorf("Provider.Close(cleanup) error = %v", err)
		}
	})
	return provider
}

func establishRequest(tier string) compozysdk.ConnectivityEstablishRequest {
	return compozysdk.ConnectivityEstablishRequest{
		Tier: tier, ForwardTarget: "127.0.0.1:43210",
		ChallengePath: challengePathPrefix + tier, Deadline: time.Now().Add(time.Second),
	}
}

func waitForForwarderHealth(t *testing.T, forwarder *tierForwarder, wantHealth string, wantReason string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		health, reason := forwarder.Health()
		if health == wantHealth && reason == wantReason {
			return
		}
		time.Sleep(time.Millisecond)
	}
	health, reason := forwarder.Health()
	t.Fatalf("Health() = (%q, %q), want (%q, %q)", health, reason, wantHealth, wantReason)
}
