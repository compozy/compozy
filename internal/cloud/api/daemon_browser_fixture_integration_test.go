// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration && (darwin || linux)

package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

type hostedBrowser struct {
	input   *json.Encoder
	output  *bufio.Scanner
	fixture authFixture
}

type browserCode struct {
	Status   int    `json:"status"`
	Code     string `json:"code"`
	Verifier string `json:"verifier"`
	Error    string `json:"error"`
}

type browserReply struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
	Proof  string          `json:"proof"`
}

type browserStreamState struct {
	Open   bool     `json:"open"`
	Events []string `json:"events"`
	Closed int64    `json:"closed"`
	Code   int      `json:"code"`
}

func useHostedBundle(t *testing.T) {
	t.Helper()
	dist, err := filepath.Abs("../../../web/dist")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
		t.Fatalf("read the browser integration bundle: %v", err)
	}
	t.Setenv("COMPOZY_WEB_DIST_DIR", dist)
}

func enableHostedBrowser(t *testing.T, host daemonAccountFixture, requireApproval bool) string {
	t.Helper()
	host.request(t, http.MethodPatch, contract.RemotePath, contract.RemotePatchRequest{
		CloudRoute: new(true), RequireDeviceApproval: new(requireApproval),
	}).expect(t, http.StatusOK, "cloud_route")
	return *waitHostedRouteReady(t, host).CloudRoute.Hostname
}

func startAdditionalHostedDaemon(t *testing.T, f authFixture, home, email string, pro bool) (
	daemonAccountFixture, remoteproto.ComputerID,
) {
	t.Helper()
	installHostedConnectorFixture(t, home)
	configureHostedDaemon(t, home, "")
	host, _ := startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{
		options: []daemon.Option{daemon.WithCloudflaredInstaller(hostedFixtureInstaller)},
	})
	progress := host.stream(t, contract.AccountSignInEventsPath, "")
	signInAccountDevice(t, f, host, progress, email)
	progress.cancel()
	if pro {
		var accountID string
		if err := f.conn.QueryRow(t.Context(), "SELECT id FROM accounts WHERE email = $1", email).
			Scan(&accountID); err != nil {
			t.Fatal(err)
		}
		giveHostedPro(t, f, accountID)
	}
	linked := host.uds(t, http.MethodPost, contract.RemoteLinkPath, contract.RemoteLinkRequest{})
	linked.expect(t, http.StatusOK, "computer_id")
	waitForHostLease(t, host)
	return host, authJSON[contract.RemoteLinkResponse](t, linked).ComputerID
}

func startHostedBrowser(
	t *testing.T,
	f authFixture,
	fake *hostedFakeCloudflare,
	email string,
	hostnames ...string,
) hostedBrowser {
	t.Helper()
	port := startBrowserEdge(t, f, fake, email, hostnames)
	script, err := filepath.Abs("../../../web/e2e/scripts/hosted-browser.mjs")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Minute)
	t.Cleanup(cancel)
	// #nosec G204 -- The script is the checked-in browser fixture and the port is an ephemeral loopback listener.
	command := exec.CommandContext(ctx, "bun", script, port)
	command.Stderr = t.Output()
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close() })
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = output.Close() })
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = input.Close()
		if err := command.Wait(); err != nil {
			t.Errorf("hosted browser fixture: %v", err)
		}
	})
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 16<<10), 2<<20)
	return hostedBrowser{input: json.NewEncoder(input), output: scanner, fixture: f}
}

func startBrowserEdge(
	t *testing.T, f authFixture, fake *hostedFakeCloudflare, email string, hostnames []string,
) string {
	t.Helper()
	const providerPath = "/fixture/google/authorize"
	cloudProxy := browserProxy(t, f.replica.Server.URL)
	cloudProxy.ModifyResponse = func(response *http.Response) error {
		if query, matched := strings.CutPrefix(
			response.Header.Get("Location"),
			f.provider.Origin+"/google/authorize?",
		); matched {
			response.Header.Set("Location", providerPath+"?"+query)
		}
		return nil
	}
	proxies := make(map[string]*httputil.ReverseProxy, len(hostnames))
	for _, hostname := range hostnames {
		proxies[hostname] = browserProxy(t, fake.target(hostname).Origin)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Host == "cloud.test" && r.URL.Path == providerPath:
			serveBrowserProvider(w, r, f.provider, email)
		case r.Host == "cloud.test":
			cloudProxy.ServeHTTP(w, r)
		case proxies[r.Host] != nil:
			proxies[r.Host].ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func browserProxy(t *testing.T, endpoint string) *httputil.ReverseProxy {
	t.Helper()
	target, err := url.Parse(endpoint)
	if err != nil || target.Host == "" {
		t.Fatalf("invalid browser fixture upstream %q: %v", endpoint, err)
	}
	return &httputil.ReverseProxy{Rewrite: func(request *httputil.ProxyRequest) {
		request.SetURL(target)
		request.Out.Host = request.In.Host
	}}
}

func serveBrowserProvider(w http.ResponseWriter, r *http.Request, provider *remotetest.IdentityProvider, email string) {
	query := r.URL.Query()
	code := provider.IssueCode(query, remotetest.ProviderProfile{
		Email: email, Subject: email, Verified: true,
	})
	callback := url.Values{"state": {query.Get("state")}, "code": {code}}
	w.Header().Set("Location", "/v1/auth/callback/google?"+callback.Encode())
	w.WriteHeader(http.StatusOK)
}

func (b hostedBrowser) call(t *testing.T, action, hostname string, input, output any) {
	t.Helper()
	if err := b.input.Encode(map[string]any{"action": action, "hostname": hostname, "input": input}); err != nil {
		t.Fatal(err)
	}
	if !b.output.Scan() {
		t.Fatalf("hosted browser ended before %s: %v", action, b.output.Err())
	}
	var response struct {
		Value json.RawMessage `json:"value"`
		Error string          `json:"error"`
	}
	if err := json.Unmarshal(b.output.Bytes(), &response); err != nil {
		t.Fatalf("decode hosted browser response for %s: %v", action, err)
	}
	if response.Error != "" {
		t.Fatalf("hosted browser %s: %s", action, response.Error)
	}
	if output != nil {
		if err := json.Unmarshal(response.Value, output); err != nil {
			t.Fatalf("decode hosted browser %s result: %v", action, err)
		}
	}
}

func (b hostedBrowser) authorize(t *testing.T, hostname string, replace bool) browserCode {
	t.Helper()
	var receipt browserCode
	b.call(t, "authorize", hostname, map[string]bool{"replace": replace}, &receipt)
	return receipt
}

func (b hostedBrowser) redeem(t *testing.T, hostname string, code browserCode, mode string) authReply {
	t.Helper()
	input := remoteproto.BrowserSessionRequest{Code: code.Code, CodeVerifier: code.Verifier}
	return b.request(t, hostname, http.MethodPost, remoteproto.BrowserSessionPath, input, mode, "").receipt()
}

func (b hostedBrowser) request(t *testing.T, hostname, method, path string, body any, mode, token string) browserReply {
	t.Helper()
	input := map[string]any{
		"method": method,
		"path":   path,
		"body":   body,
		"mode":   mode,
		"now":    b.fixture.clock.Now().Unix(),
	}
	if token != "" {
		input["token"] = token
	}
	var reply browserReply
	b.call(t, "request", hostname, input, &reply)
	return reply
}

func (b browserReply) receipt() authReply {
	return authReply{Response: &http.Response{StatusCode: b.Status, Body: http.NoBody}, body: b.Body}
}

func (b hostedBrowser) state(t *testing.T, hostname, stream string) browserStreamState {
	t.Helper()
	var state browserStreamState
	b.call(t, "streamState", hostname, stream, &state)
	return state
}

func (b hostedBrowser) ticket(t *testing.T, hostname, path string, query url.Values) string {
	t.Helper()
	reply := b.request(t, hostname, http.MethodPost, remoteproto.BrowserStreamTicketsPath,
		remoteproto.BrowserStreamTicketRequest{Path: path}, "", "").receipt()
	reply.expect(t, http.StatusCreated, "expires_at")
	issued := authJSON[remoteproto.BrowserStreamTicketResponse](t, reply)
	if query == nil {
		query = url.Values{}
	}
	query.Set("remote_ticket", issued.Ticket)
	return path + "?" + query.Encode()
}

func (b hostedBrowser) cookie(t *testing.T, hostname, name, sameSite string) string {
	t.Helper()
	var cookies []struct {
		Name, Value, Domain, Path, SameSite string
		HTTPOnly, Secure                    bool
	}
	b.call(t, "cookies", "", nil, &cookies)
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.Domain == hostname && cookie.Path == "/" && cookie.HTTPOnly &&
			cookie.Secure && cookie.SameSite == sameSite {
			return cookie.Value
		}
	}
	t.Fatalf("browser did not store %s for %s with the expected cookie attributes", name, hostname)
	return ""
}
