//go:build integration

package api_test

import (
	"bytes"
	"encoding/json/v2"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cli"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestCloudRemoteHostSecurity(t *testing.T) {
	t.Run(
		"Should close revoked streams, reconcile narrowed scopes across replicas, and expire offline",
		func(t *testing.T) {
			// IT-019. Safety Invariants 4, 6, 10, 16 and 23.
			f, _, second, relay := remoteFailoverFixture(t)
			lab := newRemoteHostLab(t, f)
			var closed atomic.Bool
			tunnel := lab.start(t, lab.db, remoteScopeFixture(&closed))
			member := lab.member
			mint := lab.mint(t, tunnel, member)
			socket := openRemoteTerminal(t, lab, tunnel, member, mint)
			epoch := lab.readSnapshot(t).SecurityEpoch
			relay.move("")
			replica := f
			replica.replica = second
			remoteCloudRequest(t, replica, lab.owner, http.MethodDelete, "/v1/devices/"+string(member.id), nil).
				expect(t, http.StatusNoContent, "")
			checkCloseBeforeAck(t, relay, epoch+1, &closed)
			relay.move(second.Server.URL)
			expectTerminalClose(t, socket, 4503)
			testutil.Eventually(t, "device revocation applied on reconnect", 3*time.Second, func() bool {
				return lab.readSnapshot(t).SecurityEpoch > epoch
			})
			status, body := tunnel.Request(t, lab.input(t, http.MethodGet, "/api/sessions", nil, member, mint))
			if status != http.StatusForbidden || !bytes.Contains(body, []byte("device_revoked")) {
				t.Fatalf("revoked session response: %d %s", status, body)
			}

			member = enrollRemoteDevice(t, f, remoteFixtureEmail, "remote-lab-narrowing")
			mint = lab.mint(t, tunnel, member)
			closed.Store(false)
			socket = openRemoteTerminal(t, lab, tunnel, member, mint)
			epoch = lab.readSnapshot(t).SecurityEpoch
			relay.move("")
			changeRemoteScopes(t, replica, lab, member, remoteproto.ScopeSessions)
			checkCloseBeforeAck(t, relay, epoch+1, &closed)
			relay.move(second.Server.URL)
			expectTerminalClose(t, socket, 4503)
			testutil.Eventually(t, "narrowed scopes applied on reconnect", 3*time.Second, func() bool {
				return slices.Equal(
					snapshotScopes(lab.readSnapshot(t), member.id),
					[]remoteproto.Scope{remoteproto.ScopeSessions},
				)
			})
			assertNarrowedRemoteSession(t, lab, tunnel, member)

			ownerMint := lab.mint(t, tunnel, lab.owner)
			socket = openRemoteTerminal(t, lab, tunnel, lab.owner, ownerMint)
			relay.move("")
			f.clock.Advance(remoteproto.LeaseTTL)
			expectTerminalClose(t, socket, 4402)
		},
	)

	t.Run("Should invalidate a stream and narrow the next session through the CLI", func(t *testing.T) {
		// IT-045. Safety Invariants 4, 10 and 23.
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		lab := newRemoteHostLab(t, f)
		var closed atomic.Bool
		tunnel := lab.start(t, lab.db, remoteScopeFixture(&closed))
		lab.startUDS(t)
		mint := lab.mint(t, tunnel, lab.member)
		socket := openRemoteTerminal(t, lab, tunnel, lab.member, mint)
		var stdout, stderr bytes.Buffer
		code := cli.ExecuteContext(t.Context(), []string{
			"remote", "devices", "scopes", string(lab.member.id), "--set", "sessions:operate", "--json",
		}, &stdout, &stderr)
		if code != 0 || !bytes.Contains(stdout.Bytes(), []byte("sessions:operate")) {
			t.Fatalf("remote devices scopes exited %d: %s %s", code, stdout.String(), stderr.String())
		}
		expectTerminalClose(t, socket, 4503)
		assertNarrowedRemoteSession(t, lab, tunnel, lab.member)
	})
}

func checkCloseBeforeAck(t *testing.T, relay *remoteControlRelay, epoch uint64, closed *atomic.Bool) {
	t.Helper()
	relay.rules(nil, func(message remoteproto.ControlMessage) {
		if message.Type == remoteproto.ControlAck && message.SecurityEpoch == epoch && !closed.Load() {
			t.Errorf("host acknowledged epoch %d before closing the terminal stream", epoch)
		}
	})
}

func changeRemoteScopes(
	t *testing.T, replica authFixture, lab *remoteHostLab, device remoteTestDevice, scopes ...remoteproto.Scope,
) {
	t.Helper()
	path := "/v1/devices/" + string(device.id)
	remoteCloudRequest(t, replica, lab.owner, http.MethodPatch, path, remoteproto.AccountDevicePatch{
		Scopes: map[remoteproto.ComputerID][]remoteproto.Scope{lab.host.Status().ComputerID: scopes},
	}).expect(t, http.StatusOK, string(device.id))
}

func assertNarrowedRemoteSession(
	t *testing.T, lab *remoteHostLab, tunnel *remotetest.Tunnel, device remoteTestDevice,
) {
	t.Helper()
	mint := lab.mint(t, tunnel, device)
	status, body := tunnel.Request(t, lab.input(t, http.MethodGet, "/api/sessions", nil, device, mint))
	var scopes map[remoteproto.Scope]bool
	if status != http.StatusOK || json.Unmarshal(body, &scopes) != nil || len(scopes) != 1 ||
		!scopes[remoteproto.ScopeSessions] {
		t.Fatalf("narrowed session response: %d %s", status, body)
	}
	status, body = tunnel.Request(t, lab.input(t, http.MethodGet, remoteFixtureStream, nil, device, mint))
	if status != http.StatusForbidden || !bytes.Contains(body, []byte("remote_scope_denied")) {
		t.Fatalf("terminal scope response: %d %s", status, body)
	}
}
