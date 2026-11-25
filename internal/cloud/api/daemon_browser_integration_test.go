// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration && (darwin || linux)

package api_test

import (
	"bytes"
	"encoding/json/v2"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	browserRemoteCookieName   = "__Host-compozy_rs"
	hostedBrowserSessionsPath = "/api/sessions?all_workspaces=true"
)

func TestDaemonHostedBrowser(t *testing.T) {
	root := t.TempDir()
	t.Run("Should bind the browser session and close its live streams on revocation (IT-072)", func(t *testing.T) {
		// Safety Invariants 1, 4, 5, 10, 15, 18, 22, 23 and 24.
		useHostedBundle(t)
		f, fake := newHostedAuthFixture(t)
		home := filepath.Join(root, "session")
		project := t.TempDir()
		configureComputerACP(t, home)
		host, _, _ := startHostedDaemon(t, f, home, "")
		hostname := enableHostedBrowser(t, host, false)
		browser := startHostedBrowser(t, f, fake, hostedFixtureEmail, hostname)
		var manifest struct{ Display string }
		browser.call(t, "pwa", hostname, nil, &manifest)
		if manifest.Display != "standalone" {
			t.Fatalf("hosted manifest display = %q", manifest.Display)
		}
		assertUnredeemedBrowserKeys(t, browser, host, hostname)
		code := browser.authorize(t, hostname, true)
		browser.redeem(t, hostname, code, "").expect(t, http.StatusCreated, "session_id")
		browser.cookie(t, "cloud.test", webSessionCookie, "Lax")
		browser.cookie(t, "cloud.test", browserCookie, "Lax")
		cookie := browser.cookie(t, hostname, browserRemoteCookieName, "Strict")
		device := activeBrowserDevice(t, f, hostname)
		assertBrowserKeyIdentity(t, f, host, hostname, device)
		assertTunnelRejectsBrowser(t, host, f, cookie)
		sessionID := makeFixtureSession(t, host, "")
		assertBrowserDPoP(t, browser, hostname, sessionID)
		openHostedBrowserTranscript(t, browser, hostname, host, sessionID)
		workspace := registerOperatorWorkspace(t, host, project)
		terminal := openHostedBrowserTerminal(t, browser, hostname, workspace)
		started := time.Now()
		host.uds(t, http.MethodDelete, contract.AccountPath+"/devices/"+device, nil).expect(t, http.StatusNoContent, "")
		assertBrowserStreamClosed(t, browser, hostname, "transcript", started, 0)
		assertBrowserStreamClosed(t, browser, hostname, "terminal", started, 4503)
		browser.request(t, hostname, http.MethodGet, hostedBrowserSessionsPath, nil, "", "").receipt().
			expect(t, http.StatusForbidden, "device_revoked")
		host.uds(t, http.MethodDelete, terminal, nil).expect(t, http.StatusOK, "exit")
	})

	t.Run("Should wait for the same browser key and require a fresh code after approval (IT-080)", func(t *testing.T) {
		// Safety Invariants 1, 3, 4, 18 and 24. The browser polls the real persisted approval.
		useHostedBundle(t)
		f, fake := newHostedAuthFixture(t)
		host, _, _ := startHostedDaemon(t, f, filepath.Join(root, "approval"), "")
		hostname := enableHostedBrowser(t, host, true)
		browser := startHostedBrowser(t, f, fake, hostedFixtureEmail, hostname)
		assertBrowserApprovalFlow(t, browser, host, hostname, true)
		browser.call(t, "clearBinding", "", nil, nil)
		assertBrowserApprovalFlow(t, browser, host, hostname, false)
	})

	t.Run(
		"Should isolate account peers and revoke each browser device across its own origins (IT-073, IT-077)",
		func(t *testing.T) {
			// Safety Invariants 1, 4, 10, 11, 20, 22, 23 and 24.
			useHostedBundle(t)
			assertHostedBrowserPeers(t, root)
		},
	)
}

func assertUnredeemedBrowserKeys(t *testing.T, browser hostedBrowser, host daemonAccountFixture, hostname string) {
	t.Helper()
	f := browser.fixture
	var epoch uint64
	if err := f.conn.QueryRow(t.Context(), "SELECT security_epoch FROM accounts").Scan(&epoch); err != nil {
		t.Fatal(err)
	}
	for index, mode := range []string{"wrong", "abandoned"} {
		code := browser.authorize(t, hostname, index != 0)
		if code.Status != http.StatusOK || code.Code == "" {
			t.Fatalf("hosted sign-in did not return a callback code (status %d)", code.Status)
		}
		if mode == "wrong" {
			browser.redeem(t, hostname, code, mode).expect(t, http.StatusForbidden, "browser_key_mismatch")
		}
		assertAuthRow(t, f, "SELECT count(*) FROM device_keys WHERE origin IS NOT NULL AND state = 'pending'", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM device_keys WHERE origin IS NOT NULL AND state = 'active'", 0)
		var device remoteproto.DeviceID
		var jkt string
		if err := f.conn.QueryRow(t.Context(), "SELECT device_id, jkt FROM device_keys WHERE origin = $1", hostname).
			Scan(&device, &jkt); err != nil {
			t.Fatal(err)
		}
		if snapshot := readHostedBrowserSnapshot(
			t,
			host,
		); snapshot.SecurityEpoch != epoch ||
			snapshot.Allows(device, jkt, nil) {
			t.Fatal("an unredeemed browser key changed the host's applied security snapshot")
		}
		f.clock.Advance(61 * time.Second)
		if err := f.replica.App.Auth.Sweep(t.Context()); err != nil {
			t.Fatal(err)
		}
		assertAuthRow(t, f, "SELECT count(*) FROM device_keys WHERE origin IS NOT NULL", 0)
		var current uint64
		if err := f.conn.QueryRow(t.Context(), "SELECT security_epoch FROM accounts").
			Scan(&current); err != nil ||
			current != epoch {
			t.Fatalf("unredeemed browser keys changed the account epoch: %d -> %d, %v", epoch, current, err)
		}
	}
}

func activeBrowserDevice(t *testing.T, f authFixture, hostname string) string {
	t.Helper()
	var device string
	if err := f.conn.QueryRow(t.Context(), `SELECT device_id FROM device_keys WHERE origin = $1 AND state = 'active'`, hostname).
		Scan(&device); err != nil {
		t.Fatal(err)
	}
	return device
}

func assertBrowserKeyIdentity(t *testing.T, f authFixture, host daemonAccountFixture, hostname, device string) {
	t.Helper()
	var jkt, account string
	var jwk remoteproto.JWK
	if err := f.conn.QueryRow(t.Context(), `SELECT k.jkt, k.public_jwk, d.account_id
		FROM device_keys k JOIN devices d ON d.id = k.device_id
		WHERE k.origin = $1 AND k.state = 'active' AND d.kind = 'browser' AND d.id = $2`, hostname, device).
		Scan(&jkt, &jwk, &account); err != nil {
		t.Fatal(err)
	}
	thumbprint, err := jwk.Thumbprint()
	if err != nil || jkt != thumbprint || account == "" {
		t.Fatalf("browser device key is not bound to its stored public JWK and account: %v", err)
	}
	if snapshot := readHostedBrowserSnapshot(t, host); !snapshot.Allows(remoteproto.DeviceID(device), thumbprint, nil) {
		t.Fatal("the host minted a browser session before applying the trusted key")
	}
}

func readHostedBrowserSnapshot(t *testing.T, host daemonAccountFixture) remoteproto.SecuritySnapshot {
	t.Helper()
	db, err := store.OpenSQLiteReadPool(t.Context(), host.databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	snapshot := remoteproto.SecuritySnapshot{Linked: true}
	var devices []byte
	if err := db.QueryRowContext(t.Context(), "SELECT applied_epoch, devices_json FROM remote_security_state WHERE id = 1").
		Scan(&snapshot.SecurityEpoch, &devices); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(devices, &snapshot.Devices); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func assertBrowserDPoP(t *testing.T, browser hostedBrowser, hostname, sessionID string) {
	t.Helper()
	signed := browser.request(t, hostname, http.MethodGet, hostedBrowserSessionsPath, nil, "", "")
	signed.receipt().expect(t, http.StatusOK, sessionID)
	browser.request(t, hostname, http.MethodGet, hostedBrowserSessionsPath, nil, "unsigned", "").receipt().
		expect(t, http.StatusUnauthorized, "dpop_invalid")
	browser.request(t, hostname, http.MethodGet, hostedBrowserSessionsPath, nil, "", signed.Proof).receipt().
		expect(t, http.StatusUnauthorized, "dpop_replayed")
}

func openHostedBrowserTranscript(
	t *testing.T,
	browser hostedBrowser,
	hostname string,
	host daemonAccountFixture,
	sessionID string,
) {
	t.Helper()
	path := "/api/sessions/" + sessionID + "/stream"
	transcript := browser.ticket(t, hostname, path, url.Values{"frames": {contract.SessionStreamFrameTranscript}})
	_, query, _ := strings.Cut(transcript, "?")
	browser.request(t, hostname, http.MethodGet, "/api/sessions/catalog-stream?"+query, nil, "unsigned", "").receipt().
		expect(t, http.StatusUnauthorized, "remote_ticket_invalid")
	browser.call(t, "eventSource", hostname, map[string]string{"name": "transcript", "path": transcript}, nil)
	browser.request(t, hostname, http.MethodGet, transcript, nil, "unsigned", "").receipt().
		expect(t, http.StatusUnauthorized, "remote_ticket_invalid")
	promptFixtureSession(t, host, sessionID, "", computerFixturePrompt)
	testutil.Eventually(t, "the hosted browser receives live transcript text", 5*time.Second, func() bool {
		return strings.Contains(strings.Join(browser.state(t, hostname, "transcript").Events, ""), computerFixtureText)
	})
}

func openHostedBrowserTerminal(t *testing.T, browser hostedBrowser, hostname, workspace string) string {
	t.Helper()
	terminals := "/api/workspaces/" + workspace + "/terminals"
	created := browser.request(
		t,
		hostname,
		http.MethodPost,
		terminals,
		contract.TerminalCreateRequest{
			Shell:    "sh",
			Cols:     80,
			Rows:     24,
			Title:    "Hosted shell",
			ClientID: "hosted-browser",
		},
		"",
		"",
	).receipt()
	created.expect(t, http.StatusCreated, "Hosted shell")
	terminal := terminals + "/" + string(authJSON[contract.TerminalResponse](t, created).Terminal.ID)
	attach := browser.request(t, hostname, http.MethodPost, terminal+"/attach-ticket",
		contract.TerminalAttachTicketRequest{Mode: contract.TerminalAttachModeWrite, ClientID: "hosted-browser"},
		"", "").receipt()
	attach.expect(t, http.StatusCreated, "ticket")
	query := url.Values{
		"mode":   {"write"},
		"flow":   {"ack"},
		"ticket": {authJSON[contract.TerminalAttachTicketResponse](t, attach).Ticket},
	}
	path := browser.ticket(t, hostname, terminal+"/stream", query)
	frame, err := terminalwire.EncodeClient(
		terminalwire.Frame{Op: terminalwire.ClientOpInput, Payload: []byte("echo hi\n")},
	)
	if err != nil {
		t.Fatal(err)
	}
	browser.call(
		t,
		"terminal",
		hostname,
		map[string]any{"name": "terminal", "path": path, "input": frame, "marker": "\r\nhi\r\n"},
		nil,
	)
	return terminal
}

func assertBrowserStreamClosed(
	t *testing.T,
	browser hostedBrowser,
	hostname, stream string,
	revoking time.Time,
	code int,
) {
	t.Helper()
	testutil.Eventually(t, "the revoked browser stream closes", 2*time.Second, func() bool {
		state := browser.state(t, hostname, stream)
		if state.Closed == 0 {
			return false
		}
		if !state.Open || state.Code != code || time.UnixMilli(state.Closed).Sub(revoking) > 2*time.Second {
			t.Fatalf("browser stream %s failed the revocation deadline or close code: %+v", stream, state)
		}
		return true
	})
}

func assertTunnelRejectsBrowser(t *testing.T, host daemonAccountFixture, f authFixture, cookie string) {
	t.Helper()
	listener := readRemoteDaemonStatus(t, host).Listener
	descriptor := remotetest.TunnelDescriptor(t, listener)
	tunnel := remotetest.DialTunnel(t, listener, descriptor, f.clock.Now)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://compozy-remote/api/sessions", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(&http.Cookie{Name: browserRemoteCookieName, Value: cookie})
	status, body := tunnel.Request(t, request)
	if status != http.StatusUnauthorized || !bytes.Contains(body, []byte("remote_session_invalid")) {
		t.Fatalf("SSH listener admitted browser cookie: HTTP %d", status)
	}
}

func assertBrowserApprovalFlow(
	t *testing.T,
	browser hostedBrowser,
	host daemonAccountFixture,
	hostname string,
	approve bool,
) {
	t.Helper()
	code := browser.authorize(t, hostname, true)
	pending := browser.redeem(t, hostname, code, "")
	pending.expect(t, http.StatusForbidden, "device_pending_approval")
	approvalID := authJSON[contract.ErrorPayload](t, pending).Details["approval_id"]
	if approvalID == "" {
		t.Fatal("waiting browser did not receive an approval ID")
	}
	path := "/remote/v1/browser/approvals/" + approvalID
	browser.request(t, hostname, http.MethodGet, path, nil, "wrong", "").receipt().
		expect(t, http.StatusNotFound, "approval_not_found")
	browser.request(t, hostname, http.MethodGet, path, nil, "", "").receipt().
		expect(t, http.StatusOK, `"state":"pending"`)
	device := hostedBrowserApprovalDevice(t, host, approvalID)
	browser.fixture.clock.Advance(6 * time.Minute)
	host.uds(t, http.MethodPatch, contract.RemoteDevicesPath+"/"+string(device), contract.RemoteDevicePatchRequest{
		Approved: new(approve), ApprovalID: approvalID,
	}).expect(t, http.StatusOK, approvalID)
	wantState := "denied"
	if approve {
		wantState = "approved"
	}
	status := browser.request(t, hostname, http.MethodGet, path, nil, "", "").receipt()
	status.expect(t, http.StatusOK, wantState)
	if state := authJSON[remoteproto.BrowserApprovalStatus](
		t,
		status,
	); state.State != wantState ||
		state.ApprovalID != approvalID {
		t.Fatalf("browser approval = %+v, want %s", state, wantState)
	}
	browser.redeem(t, hostname, code, "").expect(t, http.StatusForbidden, "browser_code_used")
	fresh := browser.authorize(t, hostname, false)
	if fresh.Code == code.Code || fresh.Verifier == code.Verifier {
		t.Fatal("approval retry did not get a fresh authorization code and verifier")
	}
	result := browser.redeem(t, hostname, fresh, "")
	if approve {
		result.expect(t, http.StatusCreated, "session_id")
		browser.request(t, hostname, http.MethodGet, hostedBrowserSessionsPath, nil, "", "").receipt().
			expect(t, http.StatusOK, "sessions")
		browser.request(t, hostname, http.MethodDelete, remoteproto.BrowserSessionPath, nil, "", "").receipt().
			expect(t, http.StatusNoContent, "")
		browser.request(t, hostname, http.MethodGet, hostedBrowserSessionsPath, nil, "", "").receipt().
			expect(t, http.StatusUnauthorized, "remote_session_invalid")
	} else {
		result.expect(t, http.StatusForbidden, "device_not_approved")
	}
}

func hostedBrowserApprovalDevice(t *testing.T, host daemonAccountFixture, id string) remoteproto.DeviceID {
	t.Helper()
	for _, device := range remoteDaemonDevices(t, host) {
		for _, approval := range device.Approvals {
			if approval.ID == id {
				return device.ID
			}
		}
	}
	t.Fatalf("host did not list browser approval %s", id)
	return ""
}

func assertHostedBrowserPeers(t *testing.T, root string) {
	t.Helper()
	f, fake := newHostedAuthFixture(t)
	first, _, _ := startHostedDaemon(t, f, filepath.Join(root, "a1"), "")
	second, secondID := startAdditionalHostedDaemon(t, f, filepath.Join(root, "a2"), hostedFixtureEmail, false)
	const foreignEmail = "other-browser-owner@example.test"
	foreign, _ := startAdditionalHostedDaemon(t, f, filepath.Join(root, "b1"), foreignEmail, true)
	firstHost := enableHostedBrowser(t, first, false)
	secondHost := enableHostedBrowser(t, second, false)
	foreignHost := enableHostedBrowser(t, foreign, false)
	browser := startHostedBrowser(t, f, fake, hostedFixtureEmail, firstHost, secondHost)
	other := startHostedBrowser(t, f, fake, foreignEmail, firstHost, foreignHost)
	if refused := other.authorize(
		t,
		firstHost,
		false,
	); refused.Status != http.StatusForbidden ||
		refused.Error != "computer_not_owned" {
		t.Fatalf("a browser from another account received HTTP %d with %q", refused.Status, refused.Error)
	}
	code := browser.authorize(t, firstHost, false)
	browser.redeem(t, firstHost, code, "").expect(t, http.StatusCreated, "session_id")
	browser.request(t, firstHost, http.MethodGet, bridgeAPI(secondID, "/api/sessions"), nil, "", "").receipt().
		expect(t, http.StatusForbidden, "remote_bridge_browser_denied")
	testutil.Eventually(
		t,
		"browser peers include only the other hosted computer on this account",
		2*time.Second,
		func() bool {
			peers := readBrowserPeers(t, browser, firstHost)
			return len(peers) == 1 && peers[0].ComputerID == secondID && peers[0].HostedURL == "https://"+secondHost
		},
	)
	second.uds(t, http.MethodPatch, contract.RemotePath, contract.RemotePatchRequest{CloudRoute: new(false)}).
		expect(t, http.StatusOK, `"state":"off"`)
	testutil.Eventually(t, "disabled hosted route disappears from the browser peer list", 2*time.Second, func() bool {
		return len(readBrowserPeers(t, browser, firstHost)) == 0
	})
	if resumed := enableHostedBrowser(t, second, false); resumed != secondHost {
		t.Fatalf("expected persisted hostname %s, got %s", secondHost, resumed)
	}
	secondCode := browser.authorize(t, secondHost, false)
	browser.redeem(t, secondHost, secondCode, "").expect(t, http.StatusCreated, "session_id")
	device := activeBrowserDevice(t, f, firstHost)
	if otherDevice := activeBrowserDevice(t, f, secondHost); otherDevice != device {
		t.Fatal("two hosted origins registered different devices for the same browser")
	}
	assertAuthRow(t, f, "SELECT count(*) FROM devices WHERE kind = 'browser'", 1)
	assertAuthRow(t, f, "SELECT count(*) FROM device_keys WHERE origin IS NOT NULL AND state = 'active'", 2)
	assertHostedBrowserDeviceRevocation(t, browser, first, device, firstHost, secondHost)
	control := other.authorize(t, foreignHost, false)
	other.redeem(t, foreignHost, control, "").expect(t, http.StatusCreated, "session_id")
	assertHostedBrowserRebind(t, browser, first, firstHost, device)
	other.request(t, foreignHost, http.MethodGet, hostedBrowserSessionsPath, nil, "", "").receipt().
		expect(t, http.StatusOK, "sessions")
}

func readBrowserPeers(t *testing.T, browser hostedBrowser, hostname string) []remoteproto.HostedPeer {
	t.Helper()
	reply := browser.request(t, hostname, http.MethodGet, remoteproto.BrowserPeersPath, nil, "", "").receipt()
	reply.expect(t, http.StatusOK, "peers")
	return authJSON[remoteproto.BrowserPeersResponse](t, reply).Peers
}

func assertHostedBrowserDeviceRevocation(
	t *testing.T, browser hostedBrowser, owner daemonAccountFixture, device string, hostnames ...string,
) {
	t.Helper()
	for _, hostname := range hostnames {
		path := browser.ticket(t, hostname, "/api/sessions/catalog-stream", url.Values{"all_workspaces": {"true"}})
		browser.call(t, "eventSource", hostname, map[string]string{"name": "catalog", "path": path}, nil)
	}
	started := time.Now()
	owner.uds(t, http.MethodDelete, contract.AccountPath+"/devices/"+device, nil).expect(t, http.StatusNoContent, "")
	for _, hostname := range hostnames {
		assertBrowserStreamClosed(t, browser, hostname, "catalog", started, 0)
		browser.request(t, hostname, http.MethodGet, hostedBrowserSessionsPath, nil, "", "").receipt().
			expect(t, http.StatusForbidden, "device_revoked")
	}
}

func assertHostedBrowserRebind(
	t *testing.T,
	browser hostedBrowser,
	owner daemonAccountFixture,
	hostname, oldDevice string,
) {
	t.Helper()
	browser.call(t, "clearBinding", "", nil, nil)
	if response := browser.authorize(
		t,
		hostname,
		false,
	); response.Error != "browser_key_rebind_required" ||
		response.Code != "" {
		t.Fatalf("clearing the browser binding did not require a new key: %q", response.Error)
	}
	fresh := browser.authorize(t, hostname, true)
	browser.redeem(t, hostname, fresh, "").expect(t, http.StatusCreated, "session_id")
	secondDevice := activeBrowserDevice(t, browser.fixture, hostname)
	if secondDevice == oldDevice {
		t.Fatal("the replacement key was assigned to the previous browser device")
	}
	var newKeys int
	if err := browser.fixture.conn.QueryRow(
		t.Context(),
		"SELECT count(*) FROM device_keys WHERE device_id = $1",
		secondDevice,
	).Scan(&newKeys); err != nil || newKeys != 1 {
		t.Fatalf("replacement browser device has %d keys: %v", newKeys, err)
	}
	assertAuthRow(
		t,
		browser.fixture,
		"SELECT count(*) FROM device_keys WHERE origin IS NOT NULL AND state = 'revoked'",
		2,
	)
	assertHostedBrowserDeviceRevocation(t, browser, owner, secondDevice, hostname)
}
