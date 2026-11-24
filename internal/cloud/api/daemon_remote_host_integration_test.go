//go:build integration

package api_test

import (
	"crypto"
	"crypto/rand"
	"encoding/json/v2"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloud/auth"
	cloudstore "github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/sse"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestDaemonRemoteHost(t *testing.T) {
	f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
	daemon, ring := startDaemonAccountFixture(t, f, t.TempDir())
	t.Run(
		"Should link, admit a newly enrolled device on the same control link, disable, and unlink",
		func(t *testing.T) {
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
			if devices := remoteDaemonDevices(t, daemon); len(devices) != 0 {
				t.Fatalf("account devices without a host connection appeared in the host's list: %v", devices)
			}

			// IT-029. Safety Invariants 18, 22 and 23.
			linkID := readHostLinkID(t, f, linked.ComputerID)
			member := enrollRemoteDevice(t, f, email, "second-installation")
			grant := fetchRemoteGrant(t, f, member, linked.ComputerID)
			f.clock.Advance(time.Second)
			tunnel := remotetest.DialTunnel(t, status.Listener, descriptor, f.clock.Now)
			started := time.Now()
			minted := mintRemote(t, tunnel, member, grant, f.clock.Now())
			if minted.SessionID == "" || time.Since(started) >= 2*time.Second {
				t.Fatal("newly enrolled device could not mint within two seconds")
			}
			if got := readHostLinkID(t, f, linked.ComputerID); got != linkID {
				t.Fatalf("device enrollment restarted the control connection: %q vs %q", got, linkID)
			}
			devices := remoteDaemonDevices(t, daemon)
			if len(devices) != 1 || devices[0].ID != member.id || devices[0].LastSeenAt == nil ||
				!devices[0].LastSeenAt.Equal(f.clock.Now()) {
				t.Fatalf("host devices after mint: %v", devices)
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

func TestDaemonRemoteApprovalDecisions(t *testing.T) {
	root := t.TempDir()
	t.Run("Should deliver signed decisions and recover persisted results (IT-071)", func(t *testing.T) {
		// Safety Invariants 3, 4 and 25. The target and deciding controller both use real daemon APIs.
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		const email = "approval-owner@example.test"
		home := filepath.Join(root, "host")
		host, ring := startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{direct: true})
		progress := host.stream(t, contract.AccountSignInEventsPath, "")
		signInDaemonAccount(t, f, host, progress, email)
		progress.cancel()
		link := host.uds(t, http.MethodPost, contract.RemoteLinkPath, contract.RemoteLinkRequest{})
		link.expect(t, http.StatusOK, "computer_id")
		computer := authJSON[contract.RemoteLinkResponse](t, link).ComputerID
		host.uds(t, http.MethodPatch, contract.RemotePath, contract.RemotePatchRequest{RequireDeviceApproval: new(true)}).
			expect(t, http.StatusOK, `"require_device_approval":true`)

		admin, signer := startApprovalAdmin(t, f, host, filepath.Join(root, "admin"), email)
		ineligible, _ := startApprovalAdmin(t, f, host, filepath.Join(root, "other"), email)
		tunnel := approvalTunnel(t, f, host)
		trusted := pendingDeviceApproval(t, f, tunnel, signer, computer)
		host.uds(t, http.MethodPatch, contract.RemoteDevicesPath+"/"+string(signer.id), contract.RemoteDevicePatchRequest{
			Approved: new(true), ApprovalID: trusted,
		}).
			expect(t, http.StatusOK, trusted)
		adminSession := mintRemote(t, tunnel, signer, fetchRemoteGrant(t, f, signer, computer), f.clock.Now())

		waiting := enrollRemoteDevice(t, f, email, "waiting-for-approval")
		id := pendingDeviceApproval(t, f, tunnel, waiting, computer)
		f.clock.Advance(6 * time.Minute)
		approved := waitAccountApproval(t, admin, id, func(a remoteproto.AccountApproval) bool { return a.CanDecide })
		path := accountApprovalPath(id)
		if result := readAccountApproval(t, ineligible, path); result.CanDecide {
			t.Fatal("an admin without an approved key on the target can decide")
		}
		ineligible.uds(t, http.MethodPost, path+"/decision", contract.AccountApprovalDecisionRequest{Approve: new(true)}).
			expect(t, http.StatusConflict, "approver_not_approved_on_computer")
		assertRemoteLocalDecisionDenied(t, f, tunnel, signer, adminSession, waiting.id, id)
		assertInvalidApprovalEnvelopes(t, f, admin, signer, waiting.id, approved)

		events := admin.stream(t, accountStreamPath, "")
		defer events.cancel()
		decideAccountApproval(t, admin, id, true)
		awaitAccountApprovalResult(t, events, id, "approved")
		mintRemote(t, tunnel, waiting, fetchRemoteGrant(t, f, waiting, computer), f.clock.Now())

		denied := enrollRemoteDevice(t, f, email, "waiting-for-denial")
		denialID := pendingDeviceApproval(t, f, tunnel, denied, computer)
		waitAccountApproval(t, admin, denialID, func(a remoteproto.AccountApproval) bool { return a.CanDecide })
		host.stop()
		boot := daemonAccountBoot{keyring: ring, direct: true}
		host, _ = startConfiguredDaemonAccount(t, f, home, boot)
		tunnel = approvalTunnel(t, f, host)
		decideAccountApproval(t, admin, denialID, false)
		awaitAccountApprovalResult(t, events, denialID, "denied")
		status, body := tunnel.Mint(t, denied.key, fetchRemoteGrant(t, f, denied, computer), f.clock.Now())
		(authReply{Response: &http.Response{StatusCode: status}, body: body}).expect(
			t, http.StatusForbidden, "device_not_approved",
		)

		assertApprovalAfterControllerReconnect(t, f, host, admin, boot, home, email, computer, tunnel, events)
	})
}

func startApprovalAdmin(
	t *testing.T, f authFixture, owner daemonAccountFixture, home, email string,
) (daemonAccountFixture, remoteTestDevice) {
	t.Helper()
	accountService := make(chan *account.Service, 1)
	controller, _ := startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{
		options: []daemon.Option{daemon.WithAccountOptions(func(s *account.Service) { accountService <- s })},
	})
	progress := controller.stream(t, contract.AccountSignInEventsPath, "")
	signInAccountDevice(t, f, controller, progress, email)
	progress.cancel()
	device := controllerCloudDevice(t, <-accountService)
	owner.uds(t, http.MethodPatch, contract.AccountPath+"/devices/"+string(device.id), remoteproto.AccountDevicePatch{
		Admin: new(true),
	}).expect(t, http.StatusOK, `"admin":true`)
	return controller, device
}

func approvalTunnel(t *testing.T, f authFixture, host daemonAccountFixture) *remotetest.Tunnel {
	t.Helper()
	status := waitForHostLease(t, host)
	return remotetest.DialTunnel(t, status.Listener, remotetest.TunnelDescriptor(t, status.Listener), f.clock.Now)
}

func pendingDeviceApproval(
	t *testing.T, f authFixture, tunnel *remotetest.Tunnel, device remoteTestDevice, computer remoteproto.ComputerID,
) string {
	t.Helper()
	status, body := tunnel.Mint(t, device.key, fetchRemoteGrant(t, f, device, computer), f.clock.Now())
	reply := authReply{Response: &http.Response{StatusCode: status}, body: body}
	reply.expect(t, http.StatusForbidden, "device_pending_approval")
	id := authJSON[contract.ErrorPayload](t, reply).Details["approval_id"]
	if id == "" {
		t.Fatal("pending mint did not return an approval ID")
	}
	return id
}

func accountApprovalPath(id string) string { return contract.AccountPath + "/approvals/" + id }

func readAccountApproval(t *testing.T, d daemonAccountFixture, path string) remoteproto.AccountApproval {
	t.Helper()
	reply := d.uds(t, http.MethodGet, path, nil)
	reply.expect(t, http.StatusOK, "can_decide")
	return authJSON[remoteproto.AccountApproval](t, reply)
}

func waitAccountApproval(
	t *testing.T, controller daemonAccountFixture, id string, ready func(remoteproto.AccountApproval) bool,
) remoteproto.AccountApproval {
	t.Helper()
	var approval remoteproto.AccountApproval
	testutil.Eventually(t, "account approval "+id+" is ready", 5*time.Second, func() bool {
		reply := controller.uds(t, http.MethodGet, accountApprovalPath(id), nil)
		if reply.StatusCode != http.StatusOK {
			return false
		}
		approval = authJSON[remoteproto.AccountApproval](t, reply)
		return ready(approval)
	})
	return approval
}

func decideAccountApproval(t *testing.T, admin daemonAccountFixture, id string, approve bool) {
	t.Helper()
	admin.uds(t, http.MethodPost, accountApprovalPath(id)+"/decision", contract.AccountApprovalDecisionRequest{
		Approve: new(approve),
	}).expect(t, http.StatusAccepted, id)
}

func awaitAccountApprovalResult(t *testing.T, events accountIntegrationStream, id, result string) {
	t.Helper()
	events.until(t, func(frame sse.Event) bool {
		if frame.Event != "account.changed" {
			return false
		}
		change := decodeDaemonAccountEvent[account.Event](t, frame).Data
		return change.Approval != nil && change.Approval.ID == id && change.Approval.Result == result
	})
}

func assertRemoteLocalDecisionDenied(
	t *testing.T, f authFixture, tunnel *remotetest.Tunnel, signer remoteTestDevice, session remoteproto.MintResponse,
	device remoteproto.DeviceID, id string,
) {
	t.Helper()
	request := remotetest.TunnelAuthorizedRequest(t, http.MethodPatch, contract.RemoteDevicesPath+"/"+string(device),
		contract.RemoteDevicePatchRequest{Approved: new(true), ApprovalID: id},
		signer.key, session.Credential, f.clock.Now())
	status, body := tunnel.Request(t, request)
	(authReply{Response: &http.Response{StatusCode: status}, body: body}).expect(
		t,
		http.StatusForbidden,
		"approval_local_only",
	)
}

func assertInvalidApprovalEnvelopes(
	t *testing.T, f authFixture, admin daemonAccountFixture, signer remoteTestDevice,
	device remoteproto.DeviceID, approval remoteproto.AccountApproval,
) {
	t.Helper()
	for _, computer := range []remoteproto.ComputerID{approval.ComputerID, "cmp_other"} {
		now := f.clock.Now()
		envelope, err := remoteproto.SignApprovalDecision(signer.key, remoteproto.ApprovalDecision{
			ApprovalID: approval.ID, ComputerID: computer, DeviceID: device, JKT: approval.KeyFingerprint,
			Decision: "approve", IssuedAt: now.Unix(), JTI: rand.Text(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if computer == approval.ComputerID {
			index := strings.LastIndexByte(envelope, '.') + 1
			replacement := "A"
			if envelope[index] == 'A' {
				replacement = "B"
			}
			envelope = envelope[:index] + replacement + envelope[index+1:]
		}
		// Inject a corrupt relay message after the cloud validation boundary to exercise the host's verifier.
		err = f.replica.App.Store.Transaction(t.Context(), func(q *cloudstore.Queries) error {
			row := cloudstore.CloudApproval{
				ID: approval.ID, DecisionJWS: envelope, DecidedByDevice: signer.id, DecidedAt: now,
			}
			if err := q.SaveCloudApprovalDecision(t.Context(), signer.account, row); err != nil {
				return err
			}
			_, err := q.AdvanceSecurity(t.Context(), cloudstore.ControlEvent{
				Type: remoteproto.ControlSnapshot, AccountID: signer.account,
			})
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		waitAccountApproval(t, admin, approval.ID, func(a remoteproto.AccountApproval) bool {
			return a.State == "pending" && a.AppliedResult != nil && *a.AppliedResult == "approval_signature_invalid"
		})
	}
}

func assertApprovalAfterControllerReconnect(
	t *testing.T, f authFixture, host, admin daemonAccountFixture, boot daemonAccountBoot, home, email string,
	computer remoteproto.ComputerID, tunnel *remotetest.Tunnel, events accountIntegrationStream,
) {
	t.Helper()
	waiting := enrollRemoteDevice(t, f, email, "waiting-for-reconnected-admin")
	id := pendingDeviceApproval(t, f, tunnel, waiting, computer)
	// Observe the open account feed before disconnecting it. Hold the decision on the cloud while the host is down.
	events.until(t, func(frame sse.Event) bool {
		if frame.Event != "account.changed" {
			return false
		}
		change := decodeDaemonAccountEvent[account.Event](t, frame).Data
		return change.Kind == "approval.pending" && change.ApprovalID == id
	})
	host.stop()
	decideAccountApproval(t, admin, id, true)
	f.replica.Server.CloseClientConnections()
	restarted, _ := startConfiguredDaemonAccount(t, f, home, boot)
	waitForHostLease(t, restarted)
	waitAccountApproval(t, admin, id, func(a remoteproto.AccountApproval) bool { return a.State == "approved" })
	f.clock.Advance(time.Second)
	awaitAccountApprovalResult(t, events, id, "approved")
	applied := readAccountApproval(t, admin, accountApprovalPath(id))
	if applied.AppliedResult == nil || *applied.AppliedResult != "approved" {
		t.Fatal("account approval read-back did not include the applied result")
	}
}

type remoteTestDevice struct {
	key     crypto.Signer
	access  string
	id      remoteproto.DeviceID
	account string
}

func enrollRemoteDevice(t *testing.T, f authFixture, email, installation string) remoteTestDevice {
	t.Helper()
	cloud, err := cloudclient.New(cloudclient.WithClock(f.clock.Now))
	if err != nil {
		t.Fatal(err)
	}
	grant, err := cloud.StartDeviceFlow(t.Context(), remoteproto.DeviceFlowRequest{
		Provider: auth.Google, InstallationID: installation, ClientName: "MacBook Pro", ClientPlatform: "macOS",
	})
	if err != nil {
		t.Fatalf("start device flow: %v", err)
	}
	cookie := f.openActivation(t, grant, auth.Google)
	redirect := f.continueGrant(t, grant, auth.Google, cookie, false)
	profile := providerProfile{email: email, subject: email, verified: true}
	callback := f.callbackPath(t, auth.Google, redirect, profile)
	f.request(t, http.MethodGet, callback, nil, cookie).expect(t, http.StatusOK, "Return to CompozyOS.")
	token := f.redeem(t, grant)
	key, jwk := newDeviceKey(t)
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

func remoteDaemonDevices(t *testing.T, daemon daemonAccountFixture) []contract.RemoteDeviceResponse {
	t.Helper()
	reply := daemon.uds(t, http.MethodGet, contract.RemoteDevicesPath, nil)
	reply.expect(t, http.StatusOK, "devices")
	return authJSON[contract.RemoteDevicesResponse](t, reply).Devices
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
