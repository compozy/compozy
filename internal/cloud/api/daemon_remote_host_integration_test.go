//go:build integration

package api_test

import (
	"crypto"
	"encoding/json/v2"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestDaemonRemoteHost(t *testing.T) {
	t.Run(
		"Should link, admit a newly enrolled device on the same control link, disable, and unlink",
		func(t *testing.T) {
			f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
			daemon, ring := startDaemonAccountFixture(t, f)
			progress := daemon.stream(t, contract.AccountSignInEventsPath, "")
			const email = "remote-owner@example.test"
			owner := signInDaemonAccount(t, f, daemon, progress, email)
			accountID, _ := assertEnrolledAccount(t, f, owner, ring)

			// IT-031. Safety Invariants 1 and 15.
			initial := readRemoteDaemonStatus(t, daemon)
			assertListenerClosed(t, initial.Listener)
			reply := daemon.uds(t, http.MethodPost, contract.RemoteLinkPath, contract.RemoteLinkRequest{})
			reply.expect(t, http.StatusOK, "computer_id")
			linked := authJSON[contract.RemoteLinkResponse](t, reply)
			status := waitForHostLease(t, daemon)
			descriptor := remotetest.TunnelDescriptor(t, status.Listener)
			if descriptor.ComputerID != linked.ComputerID || !status.Enabled || descriptor.BrowserSignIn {
				t.Fatalf("link status or descriptor mismatch: %v %v", status, descriptor)
			}

			// IT-029. Safety Invariants 18, 22 and 23.
			linkID := readHostLinkID(t, f, linked.ComputerID)
			member := enrollRemoteDevice(t, f, email, "second-installation")
			grant := fetchRemoteGrant(t, f, member, linked.ComputerID)
			tunnel := remotetest.DialTunnel(t, status.Listener, descriptor, f.clock.Now)
			started := time.Now()
			minted := mintRemote(t, tunnel, member, grant, f.clock.Now())
			if minted.SessionID == "" || time.Since(started) >= 2*time.Second {
				t.Fatal("newly enrolled device could not mint within two seconds")
			}
			if got := readHostLinkID(t, f, linked.ComputerID); got != linkID {
				t.Fatalf("device enrollment restarted the control connection: %q vs %q", got, linkID)
			}

			disabled := daemon.request(
				t,
				http.MethodPatch,
				contract.RemotePath,
				contract.RemotePatchRequest{Enabled: new(false)},
			)
			disabled.expect(t, http.StatusOK, `"enabled":false`)
			assertListenerClosed(t, status.Listener)
			if got := authJSON[contract.RemoteStatusResponse](t, disabled).ComputerID; got != linked.ComputerID {
				t.Fatalf("disable lost link identity: %s", got)
			}
			daemon.uds(t, http.MethodPatch, contract.RemotePath, contract.RemotePatchRequest{Enabled: new(true)}).
				expect(t, http.StatusOK, `"enabled":true`)

			// IT-018. Safety Invariants 10 and 23.
			before := readRemoteDaemonStatus(t, daemon)
			daemon.uds(t, http.MethodDelete, contract.RemoteLinkPath, nil).expect(t, http.StatusNoContent, "")
			assertListenerClosed(t, before.Listener)
			directory := remoteCloudRequest(t, f, member, http.MethodGet, "/v1/computers", nil)
			directory.expect(t, http.StatusOK, "computers")
			computers := authJSON[struct {
				Computers []struct {
					ID remoteproto.ComputerID `json:"id"`
				} `json:"computers"`
			}](t, directory)
			if len(computers.Computers) != 0 || member.account != accountID {
				t.Fatalf("unlinked computer still appears in account directory: %v", computers)
			}
		},
	)
}

type remoteTestDevice struct {
	key     crypto.Signer
	access  string
	id      remoteproto.DeviceID
	account string
}

func enrollRemoteDevice(t *testing.T, f authFixture, email, installation string) remoteTestDevice {
	t.Helper()
	reply, grant := f.signIn(t, auth.Google, providerProfile{email: email, subject: email, verified: true}, false)
	reply.expect(t, http.StatusOK, "Return to CompozyOS.")
	token := f.redeem(t, grant)
	key, jwk := newDeviceKey(t)
	cloud, err := cloudclient.New(cloudclient.WithClock(f.clock.Now))
	if err != nil {
		t.Fatal(err)
	}
	input := enrollInput(jwk)
	input.InstallationID = installation
	result, err := cloud.Enroll(t.Context(), token.EnrollmentToken, key, input)
	if err != nil {
		t.Fatalf("enroll remote device: %v", err)
	}
	return remoteTestDevice{key, result.AccessToken, result.DeviceID, token.AccountID}
}

func fetchRemoteGrant(t *testing.T, f authFixture, device remoteTestDevice, computer remoteproto.ComputerID) string {
	t.Helper()
	path := "/v1/computers/" + string(computer) + "/grants"
	reply := remoteCloudRequest(t, f, device, http.MethodPost, path, map[string]string{"route": "ssh"})
	reply.expect(t, http.StatusOK, "grant")
	return authJSON[struct {
		Grant string `json:"grant"`
	}](t, reply).Grant
}

func remoteCloudRequest(
	t *testing.T, f authFixture, device remoteTestDevice, method, path string, input any,
) authReply {
	t.Helper()
	proof, err := remoteproto.BuildDPoP(device.key, method, f.replica.App.Issuer+path, device.access, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"Authorization": "DPoP " + device.access, "DPoP": proof}
	return callAuth(t, f.replica, method, path, input, nil, headers)
}

func mintRemote(
	t *testing.T, tunnel *remotetest.Tunnel, device remoteTestDevice, grant string, now time.Time,
) remoteproto.MintResponse {
	t.Helper()
	status, body := tunnel.Mint(t, device.key, grant, now)
	var mint remoteproto.MintResponse
	if status != http.StatusCreated || json.Unmarshal(body, &mint) != nil || mint.Credential == "" {
		t.Fatalf("mint did not return a session: HTTP %d", status)
	}
	return mint
}

func readRemoteDaemonStatus(t *testing.T, daemon daemonAccountFixture) contract.RemoteStatusResponse {
	t.Helper()
	reply := daemon.request(t, http.MethodGet, contract.RemotePath, nil)
	reply.expect(t, http.StatusOK, "control_link")
	return authJSON[contract.RemoteStatusResponse](t, reply)
}

func waitForHostLease(t *testing.T, daemon daemonAccountFixture) contract.RemoteStatusResponse {
	t.Helper()
	var status contract.RemoteStatusResponse
	testutil.Eventually(t, "remote host has a lease", 6*time.Second, func() bool {
		status = readRemoteDaemonStatus(t, daemon)
		return status.ControlLink == "connected" && status.Entitlement != nil
	})
	return status
}

func readHostLinkID(t *testing.T, f authFixture, computer remoteproto.ComputerID) string {
	t.Helper()
	var id string
	if err := f.conn.QueryRow(t.Context(), "SELECT control_link_id FROM computers WHERE id = $1", computer).
		Scan(&id); err != nil || id == "" {
		t.Fatalf("linked computer has no control connection: %v", err)
	}
	return id
}

func assertListenerClosed(t *testing.T, address string) {
	t.Helper()
	dialer := &net.Dialer{Timeout: time.Second}
	socket, err := dialer.DialContext(t.Context(), "tcp", address)
	if err == nil {
		if err := socket.Close(); err != nil {
			t.Error(err)
		}
		t.Fatalf("listener at %s accepted a connection", address)
	}
}
