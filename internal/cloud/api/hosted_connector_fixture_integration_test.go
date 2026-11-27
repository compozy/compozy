// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration && (darwin || linux)

package api_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/testutil"
)

const (
	connectorFixtureVersion = "fixture"
	connectorFixtureAddress = "connector.json"
	connectorFixtureFailure = "port-blocked-once"
	connectorFixtureMode    = "connector-mode"
)

func runHostedFixtureProcess(runTests func() int) int {
	if os.Getenv(accountFixtureProcessEnv) == "1" {
		return runDetachedAccountFixtureProcess()
	}
	if len(os.Args) < 2 || os.Args[1] != "tunnel" || os.Getenv("COMPOZY_REMOTE_CONNECTOR") != "1" {
		return runTests()
	}
	if err := runHostedFixtureConnector(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

type hostedConnectorState struct {
	PID     int    `json:"pid"`
	Address string `json:"address"`
	hostedFakeConnector
}

func hostedConnectorDir(home string) string {
	return filepath.Join(home, "tools", "cloudflared", connectorFixtureVersion)
}

func hostedFixtureInstaller(_ context.Context, home string) (string, error) {
	return filepath.Join(hostedConnectorDir(home), "cloudflared"), nil
}

func installHostedConnectorFixture(t *testing.T, home string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, _ := hostedFixtureInstaller(t.Context(), home)
	if err := os.MkdirAll(filepath.Dir(binary), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(executable, binary); err != nil {
		t.Fatal(err)
	}
}

func runHostedFixtureConnector(args []string) error {
	metrics := slices.Index(args, "--metrics")
	if metrics < 0 || metrics+1 == len(args) {
		return errors.New("connector did not receive a metrics address")
	}
	home := os.Getenv("COMPOZY_HOME")
	dir := hostedConnectorDir(home)
	if err := os.Remove(filepath.Join(dir, connectorFixtureFailure)); err == nil {
		return errors.New("failed to dial to edge with quic: Cloudflare port 7844 blocked")
	}
	if !slices.Contains(args, "--fixture-detached") {
		mode, _ := os.ReadFile(filepath.Join(dir, connectorFixtureMode))
		if len(mode) > 0 {
			return runDetachedFixtureConnector(args, string(mode))
		}
	}
	var target hostedFakeConnector
	if err := json.Unmarshal([]byte(os.Getenv("TUNNEL_TOKEN")), &target); err != nil {
		return errors.New("connector did not receive fixture credentials in its environment")
	}
	origin, err := url.Parse(target.Origin)
	if err != nil || origin.Scheme != "http" || origin.Host == "" {
		return errors.New("connector did not receive a valid origin")
	}
	listener, err := net.Listen("tcp", args[metrics+1])
	if err != nil {
		return err
	}
	defer listener.Close()
	if crash := slices.Index(args, "--fixture-crash"); crash >= 0 {
		parent, err := strconv.Atoi(args[crash+1])
		if err != nil || parent <= 1 {
			return errors.New("connector did not receive a fixture daemon PID")
		}
		if err := syscall.Kill(parent, syscall.SIGKILL); err != nil {
			return err
		}
	}
	state := hostedConnectorState{os.Getpid(), listener.Addr().String(), target}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := fileutil.AtomicWriteFile(filepath.Join(dir, connectorFixtureAddress), data, 0o600); err != nil {
		return err
	}
	proxy := httputil.NewSingleHostReverseProxy(origin)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.Host != target.Hostname || req.Header.Get("Authorization") != "" || req.Header.Get("DPoP") != "" {
			http.Error(w, "unexpected authority or remote credential at the edge", http.StatusForbidden)
			return
		}
		proxy.ServeHTTP(w, req)
	})
	return (&http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}).Serve(listener)
}

func runDetachedFixtureConnector(args []string, mode string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	childArgs := append([]string{"tunnel", "--fixture-detached"}, args...)
	if mode == "crash-before-ready" {
		childArgs = append(childArgs, "--fixture-crash", strconv.Itoa(os.Getppid()))
	}
	cmd := exec.Command(executable, childArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func readHostedConnector(home string) (hostedConnectorState, error) {
	var state hostedConnectorState
	raw, err := os.ReadFile(filepath.Join(hostedConnectorDir(home), connectorFixtureAddress))
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(raw, &state)
	return state, err
}

func waitHostedConnector(t *testing.T, home string, previousPID int) hostedConnectorState {
	t.Helper()
	var state hostedConnectorState
	testutil.Eventually(t, "hosted connector publishes its local address", 12*time.Second, func() bool {
		var err error
		state, err = readHostedConnector(home)
		return err == nil && state.PID > 1 && state.PID != previousPID && state.Address != ""
	})
	return state
}

func hostedEdgeClient(t *testing.T, home string) *http.Client {
	t.Helper()
	edge := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		state, err := readHostedConnector(home)
		if err != nil || req.Host != state.Hostname {
			http.NotFound(w, req)
			return
		}
		httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: state.Address}).ServeHTTP(w, req)
	}))
	t.Cleanup(edge.Close)
	client := edge.Client()
	base, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("fixture TLS server did not return an HTTP transport")
	}
	transport := base.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, edge.Listener.Addr().String())
	}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}
