//go:build integration

package control_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestCloudFanout(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should route committed notifications and recover the direct listener",
		func(t *testing.T) {
			t.Parallel()
			db := remotetest.NewDatabase(t)
			db.Migrate(t)
			first := remotetest.StartReplica(t, cloud.Config{Postgres: db.Config(t, "first")}, nil)
			second := remotetest.StartReplica(t, cloud.Config{Postgres: db.Config(t, "second")}, nil)
			one := subscribe(t, first.App.Control, "acc_a", "cmp_one")
			two := subscribe(t, second.App.Control, "acc_a", "cmp_two")
			foreign := subscribe(t, second.App.Control, "acc_b", "cmp_foreign")

			revoke := store.ControlEvent{
				Type: remoteproto.ControlRevokeDevice, AccountID: "acc_a", SecurityEpoch: 8, ResourceID: "dev_gone",
			}
			rolledBack := errors.New("rollback audit and notification")
			err := second.App.Store.Transaction(t.Context(), func(q *store.Queries) error {
				at := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
				if err := q.AppendSecurityEvent(t.Context(), store.SecurityEvent{
					AccountID: "acc_a", Actor: "system", At: at, Type: "device_revoked",
				}); err != nil {
					return err
				}
				if err := q.NotifyControlEvent(t.Context(), revoke); err != nil {
					return err
				}
				return rolledBack
			})
			if !errors.Is(err, rolledBack) {
				t.Fatalf("rollback = %v", err)
			}

			revoke.SecurityEpoch++
			commitEvent(t, second.App.Store, revoke)
			expectEvent(t, one, revoke)
			expectEvent(t, two, revoke)

			approval := store.ControlEvent{
				Type: remoteproto.ControlApprovalDecided, AccountID: "acc_a", ComputerID: "cmp_two",
				ResourceID: "apr_one", SecurityEpoch: 9,
			}
			commitEvent(t, first.App.Store, approval)
			expectEvent(t, two, approval)

			peers := store.ControlEvent{Type: remoteproto.ControlPeers, AccountID: "acc_a"}
			commitEvent(t, first.App.Store, peers)
			expectEvent(t, one, peers)
			expectEvent(t, two, peers)

			keys := store.ControlEvent{Type: remoteproto.ControlJWKS}
			commitEvent(t, first.App.Store, keys)
			expectEvent(t, one, keys)
			expectEvent(t, two, keys)
			expectEvent(t, foreign, keys)

			oldPID := directListenerPID(t, db, "first")
			var stopped bool
			err = db.Pool.QueryRow(t.Context(), "SELECT pg_terminate_backend($1)", oldPID).Scan(&stopped)
			if err != nil || !stopped {
				t.Fatalf("terminate LISTEN backend = %t, %v", stopped, err)
			}
			select {
			case reason := <-one.Closing:
				if reason != control.Resync {
					t.Fatalf("listener disconnect closed link with %q", reason)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("subscriber was not told to resync after losing LISTEN")
			}
			ready, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if err := first.App.Control.WaitReady(ready); err != nil {
				t.Fatal(err)
			}
			if newPID := directListenerPID(t, db, "first"); newPID == oldPID {
				t.Fatalf("listener reused terminated backend %d", oldPID)
			}
			after := subscribe(t, first.App.Control, "acc_a", "cmp_one")
			snapshot := store.ControlEvent{Type: remoteproto.ControlSnapshot, AccountID: "acc_a", SecurityEpoch: 10}
			commitEvent(t, second.App.Store, snapshot)
			expectEvent(t, after, snapshot)
			assertPooledConnections(t, db)
		},
	)

	t.Run(
		"Should deliver revocation, approvals, peers and keys across real control links (IT-105)",
		func(t *testing.T) {
			t.Parallel()
			db := remotetest.NewDatabase(t)
			db.Migrate(t)
			now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
			const account = "acc_links"
			db.SeedAccount(t, account)
			signer := remotetest.NewSigner(t, "active")
			admin := db.SeedControlDevice(t, account, "dev_admin", true, signer, now)
			revoked := db.SeedControlDevice(t, account, "dev_revoked", false, signer, now)
			waiting := db.SeedControlDevice(t, account, "dev_waiting", false, signer, now)
			oneKey := db.SeedControlComputer(t, account, "cmp_one", now)
			twoKey := db.SeedControlComputer(t, account, "cmp_two", now)
			db.Exec(
				t,
				"INSERT INTO %s.subscriptions (id, source, account_id, status, ends_at) VALUES ('pro','complimentary',$1,'active',$2)",
				account,
				now.Add(time.Hour),
			)
			keys, err := api.NewSigningKeys(signer, []remoteproto.JWK{
				remotetest.NewSigner(t, "overlap").PublicJWK(),
			})
			if err != nil {
				t.Fatal(err)
			}
			config := remotetest.ControlConfig(t, func() time.Time { return now }, keys)
			config.Postgres = db.Config(t, "fanout_one")
			first := remotetest.StartReplica(t, config, nil)
			config.Postgres = db.Config(t, "fanout_two")
			second := remotetest.StartReplica(t, config, nil)
			verifier, err := remoteproto.NewJWSVerifier(remoteproto.CloudIssuer, keys.JWKS(), nil)
			if err != nil {
				t.Fatal(err)
			}
			one := remotetest.AuthenticateControlLink(t, t.Context(), first.Server.URL, "cmp_one", oneKey)
			two := remotetest.AuthenticateControlLink(t, t.Context(), second.Server.URL, "cmp_two", twoKey)
			state := remoteproto.NewSecurityState("cmp_one", account)
			startOne, initialLease := firstSnapshot(t, one, verifier, now)
			startTwo, _ := firstSnapshot(t, two, verifier, now)
			if startOne.AccountID != account || startOne.SecurityEpoch != 1 || startTwo.SecurityEpoch != 1 {
				t.Fatalf("initial snapshots = %+v, %+v", startOne, startTwo)
			}
			noSessions := func(remoteproto.SecuritySnapshot) error { return nil }
			if _, err := state.ApplySnapshot(startOne, noSessions); err != nil {
				t.Fatal(err)
			}
			scope := []remoteproto.Scope{remoteproto.ScopeSessions}
			if !state.Snapshot().Allows(revoked.ID, revoked.JKT, scope) {
				t.Fatal("the first snapshot omitted an active device")
			}
			start := time.Now()
			controlHTTP(
				t,
				second,
				now,
				admin,
				http.MethodDelete,
				"/v1/devices/"+string(revoked.ID),
				nil,
				http.StatusNoContent,
			)
			delta := readControlType(t, one, remoteproto.ControlRevokeDevice, time.Until(start.Add(2*time.Second)))
			var closed bool
			if changed, repair, err := state.ApplyIncremental(delta, func(view remoteproto.SecuritySnapshot) error {
				closed = !view.Allows(revoked.ID, revoked.JKT, scope)
				return nil
			}); err != nil || repair != nil || !changed || !closed {
				t.Fatalf(
					"revocation reconciliation changed=%t repair=%+v closed=%t err=%v",
					changed,
					repair,
					closed,
					err,
				)
			}
			revokeOne := readSnapshot(t, one, verifier)
			revokeTwo := readSnapshot(t, two, verifier)
			if revokeOne.SecurityEpoch != 2 || revokeTwo.SecurityEpoch != 2 ||
				revokeOne.Allows(revoked.ID, revoked.JKT, scope) {
				t.Fatalf("revocation snapshots = %+v, %+v", revokeOne, revokeTwo)
			}
			ackSecurity(t, one, revokeOne.SecurityEpoch)
			ackSecurity(t, two, revokeTwo.SecurityEpoch)

			testApprovalRelay(t, first, two, verifier, now, admin, waiting)
			assertPeersAndKeys(t, db, first, one, two, account, keys.JWKS())
			stopListener(t, db, first, "fanout_one")

			reconnected := remotetest.AuthenticateControlLink(t, t.Context(), first.Server.URL, "cmp_one", oneKey)
			repaired, replacementLease := firstSnapshot(t, reconnected, verifier, now)
			if repaired.SecurityEpoch != 3 || repaired.Allows(revoked.ID, revoked.JKT, scope) ||
				replacementLease.Sequence <= initialLease.Sequence {
				t.Fatalf("reconnected snapshot = %+v, lease sequence = %d, initial = %d",
					repaired, replacementLease.Sequence, initialLease.Sequence)
			}
			remotetest.SendControl(t, reconnected, remoteproto.ControlSnapshotRequest, remoteproto.SnapshotRequest{
				AppliedEpoch: repaired.SecurityEpoch - 1,
			})
			if requested := readSnapshot(t, reconnected, verifier); requested.SecurityEpoch != repaired.SecurityEpoch {
				t.Fatalf("snapshot request returned epoch %d, want %d", requested.SecurityEpoch, repaired.SecurityEpoch)
			}
			commitEvent(t, second.App.Store, store.ControlEvent{Type: remoteproto.ControlJWKS})
			readControlType(t, reconnected, remoteproto.ControlJWKS, 2*time.Second)
		},
	)
}

func testApprovalRelay(
	t *testing.T, replica *remotetest.Replica, host *websocket.Conn, verifier *remoteproto.JWSVerifier,
	now time.Time, admin, waiting remotetest.ControlDevice,
) {
	t.Helper()
	const id = "apr_waiting"
	events := openAccountEvents(t, replica, now, admin)
	remotetest.SendControl(t, host, remoteproto.ControlApprovalPending, remoteproto.ApprovalPending{
		ApprovalID: id, DeviceID: waiting.ID, JKT: waiting.JKT,
	})
	readAccountEvent(t, events, "approval.pending")
	var approval control.Approval
	path := "/v1/approvals/" + id
	body := controlHTTP(t, replica, now, admin, http.MethodGet, path, nil, http.StatusOK)
	if err := json.Unmarshal(body, &approval); err != nil || approval.ID != id || approval.CanDecide ||
		approval.AppliedResult != nil {
		t.Fatalf("pending approval = %+v, %v", approval, err)
	}
	envelope, err := remoteproto.SignApprovalDecision(admin.Key, remoteproto.ApprovalDecision{
		ApprovalID: id, ComputerID: "cmp_two", DeviceID: waiting.ID, JKT: waiting.JKT,
		Decision: "approve", IssuedAt: now.Unix(), JTI: "decision_jti",
	})
	if err != nil {
		t.Fatal(err)
	}
	body = controlHTTP(t, replica, now, admin, http.MethodPost, path+"/decision",
		remoteproto.ApprovalDecided{DecisionJWS: envelope}, http.StatusConflict)
	var ineligible api.ErrorResponse
	if err := json.Unmarshal(body, &ineligible); err != nil || ineligible.Code != "approver_not_approved_on_computer" {
		t.Fatalf("ineligible decision = %+v, %v", ineligible, err)
	}
	remotetest.SendControl(
		t,
		host,
		remoteproto.ControlApprovedKeys,
		remoteproto.ApprovedKeys{JKTs: []string{admin.JKT}},
	)
	remotetest.SendControl(t, host, remoteproto.ControlSnapshotRequest, remoteproto.SnapshotRequest{AppliedEpoch: 2})
	readSnapshot(t, host, verifier)
	body = controlHTTP(t, replica, now, admin, http.MethodGet, path, nil, http.StatusOK)
	if err := json.Unmarshal(body, &approval); err != nil || !approval.CanDecide {
		t.Fatalf("eligibility after the computer approved the signer = %+v, %v", approval, err)
	}
	controlHTTP(t, replica, now, admin, http.MethodPost, path+"/decision",
		remoteproto.ApprovalDecided{DecisionJWS: envelope}, http.StatusAccepted)
	snapshot := readSnapshot(t, host, verifier)
	ackSecurity(t, host, snapshot.SecurityEpoch)
	frame := readControlType(t, host, remoteproto.ControlApprovalDecided, 2*time.Second)
	var relayed remoteproto.ApprovalDecided
	if err := json.Unmarshal(frame.Data, &relayed); err != nil || relayed.DecisionJWS != envelope ||
		frame.SecurityEpoch != snapshot.SecurityEpoch {
		t.Fatalf("approval relay did not preserve the signed decision: type=%s epoch=%d error=%v",
			frame.Type, frame.SecurityEpoch, err)
	}
	remotetest.SendControl(t, host, remoteproto.ControlApprovalApplied, remoteproto.ApprovalApplied{
		ApprovalID: id, Result: "approved",
	})
	result := readAccountEvent(t, events, "approval.applied")
	if result["approval_id"] != id || result["result"] != "approved" {
		t.Fatalf("approval event = %+v", result)
	}
	body = controlHTTP(t, replica, now, admin, http.MethodGet, "/v1/approvals", nil, http.StatusOK)
	var recent struct {
		Approvals []control.Approval `json:"approvals"`
	}
	if err := json.Unmarshal(body, &recent); err != nil || len(recent.Approvals) != 1 {
		t.Fatalf("recent decisions = %+v, %v", recent, err)
	}
	applied := recent.Approvals[0]
	if applied.ID != id || applied.State != "approved" || applied.AppliedResult == nil ||
		*applied.AppliedResult != "approved" || applied.CanDecide {
		t.Fatalf("applied approval read-back = %+v", applied)
	}
}

func assertPeersAndKeys(
	t *testing.T, db *remotetest.Database, replica *remotetest.Replica, one, two *websocket.Conn,
	account string, jwks remoteproto.JWKS,
) {
	t.Helper()
	db.Exec(
		t,
		"INSERT INTO %s.tunnels (computer_id, hostname, state) VALUES ('cmp_one','one.compozy.run','active'), ('cmp_two','two.compozy.run','active')",
	)
	commitEvent(t, replica.App.Store, store.ControlEvent{Type: remoteproto.ControlPeers, AccountID: account})
	commitEvent(t, replica.App.Store, store.ControlEvent{Type: remoteproto.ControlJWKS})
	for _, pair := range []struct {
		link *websocket.Conn
		peer remoteproto.ComputerID
	}{{one, "cmp_two"}, {two, "cmp_one"}} {
		deadline := time.Now().Add(2 * time.Second)
		for {
			message := readControlType(t, pair.link, remoteproto.ControlPeers, time.Until(deadline))
			var peers []remoteproto.HostedPeer
			if err := json.Unmarshal(message.Data, &peers); err != nil {
				t.Fatal(err)
			}
			if len(peers) == 0 {
				continue
			}
			if len(peers) != 1 || peers[0].ComputerID != pair.peer {
				t.Fatalf("unexpected hosted peers %+v", peers)
			}
			break
		}
		keys := readControlType(t, pair.link, remoteproto.ControlJWKS, 2*time.Second)
		var got remoteproto.JWKS
		if err := json.Unmarshal(keys.Data, &got); err != nil || !slices.Equal(got.Keys, jwks.Keys) {
			t.Fatalf("forwarded JWKS = %+v, %v", got, err)
		}
	}
}

func stopListener(t *testing.T, db *remotetest.Database, first *remotetest.Replica, name string) {
	t.Helper()
	witness := subscribe(t, first.App.Control, "acc_links", "cmp_one")
	pid := directListenerPID(t, db, name)
	var stopped bool
	if err := db.Pool.QueryRow(t.Context(), "SELECT pg_terminate_backend($1)", pid).
		Scan(&stopped); err != nil ||
		!stopped {
		t.Fatalf("terminate control LISTEN connection: %t, %v", stopped, err)
	}
	select {
	case reason := <-witness.Closing:
		if reason != control.Resync {
			t.Fatalf("disconnect reason = %s", reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener disconnect was not detected")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := first.App.Control.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
}

func firstSnapshot(
	t *testing.T, socket *websocket.Conn, verifier *remoteproto.JWSVerifier, now time.Time,
) (remoteproto.SecuritySnapshot, remoteproto.EntitlementLease) {
	t.Helper()
	message := remotetest.ReadControlMessage(t, socket, 2*time.Second)
	snapshot := verifySnapshot(t, verifier, message)
	ackSecurity(t, socket, snapshot.SecurityEpoch)
	message = remotetest.ReadControlMessage(t, socket, 2*time.Second)
	var token string
	if message.Type != remoteproto.ControlLease || json.Unmarshal(message.Data, &token) != nil {
		t.Fatalf("expected lease after snapshot acknowledgement, got %s", message.Type)
	}
	lease, err := verifier.VerifyLease(t.Context(), token, now)
	if err != nil || lease.SecurityEpoch != snapshot.SecurityEpoch || lease.Sequence == 0 {
		t.Fatalf("lease = %+v, %v", lease, err)
	}
	return snapshot, lease
}

func readSnapshot(
	t *testing.T,
	socket *websocket.Conn,
	verifier *remoteproto.JWSVerifier,
) remoteproto.SecuritySnapshot {
	t.Helper()
	return verifySnapshot(t, verifier, readControlType(t, socket, remoteproto.ControlSnapshot, 2*time.Second))
}

func verifySnapshot(
	t *testing.T, verifier *remoteproto.JWSVerifier, message remoteproto.ControlMessage,
) remoteproto.SecuritySnapshot {
	t.Helper()
	var token string
	if message.Type != remoteproto.ControlSnapshot || json.Unmarshal(message.Data, &token) != nil {
		t.Fatalf("expected snapshot first, got %s", message.Type)
	}
	snapshot, err := verifier.VerifySnapshot(t.Context(), token)
	if err != nil || message.SecurityEpoch != snapshot.SecurityEpoch {
		t.Fatalf("snapshot = %+v, %v", snapshot, err)
	}
	return snapshot
}

func ackSecurity(t *testing.T, socket *websocket.Conn, epoch uint64) {
	t.Helper()
	remotetest.SendControl(t, socket, remoteproto.ControlAck, remoteproto.SecurityAck{SecurityEpoch: epoch})
}

func readControlType(
	t *testing.T, socket *websocket.Conn, kind remoteproto.ControlType, timeout time.Duration,
) remoteproto.ControlMessage {
	t.Helper()
	until := time.Now().Add(timeout)
	for {
		message := remotetest.ReadControlMessage(t, socket, time.Until(until))
		if message.Type == kind {
			return message
		}
	}
}

func controlHTTP(
	t *testing.T, replica *remotetest.Replica, now time.Time, device remotetest.ControlDevice,
	method, path string, body any, status int,
) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, replica.Server.URL+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	device.AuthorizeRequest(t, req, now)
	resp, err := replica.Server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != status {
		t.Fatalf("%s %s = %d %s, %v", method, path, resp.StatusCode, data, err)
	}
	return data
}

func openAccountEvents(
	t *testing.T, replica *remotetest.Replica, now time.Time, device remotetest.ControlDevice,
) *bufio.Scanner {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, replica.Server.URL+"/v1/account/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	device.AuthorizeRequest(t, req, now)
	resp, err := replica.Server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("account event stream status=%d content type=%s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	return bufio.NewScanner(resp.Body)
}

func readAccountEvent(t *testing.T, stream *bufio.Scanner, event string) map[string]string {
	t.Helper()
	for stream.Scan() {
		if stream.Text() != "event: "+event {
			continue
		}
		if !stream.Scan() {
			t.Fatalf("event %s has no data", event)
		}
		var data map[string]string
		if err := json.Unmarshal([]byte(strings.TrimPrefix(stream.Text(), "data: ")), &data); err != nil {
			t.Fatal(err)
		}
		return data
	}
	t.Fatalf("stream closed before %s: %v", event, stream.Err())
	return nil
}

func commitEvent(t *testing.T, db *store.Store, event store.ControlEvent) {
	t.Helper()
	ctx := t.Context()
	if err := db.Transaction(
		ctx,
		func(q *store.Queries) error { return q.NotifyControlEvent(ctx, event) },
	); err != nil {
		t.Fatalf("publish %+v: %v", event, err)
	}
}

func subscribe(t *testing.T, hub *control.Hub, account string, computer remoteproto.ComputerID) *control.Subscription {
	t.Helper()
	sub, ok := hub.Subscribe(account, computer)
	if !ok {
		t.Fatalf("subscribe %s/%s", account, computer)
	}
	t.Cleanup(func() { hub.Release(sub) })
	return sub
}

func expectEvent(t *testing.T, sub *control.Subscription, want store.ControlEvent) {
	t.Helper()
	select {
	case event := <-sub.Events:
		if event != want {
			t.Fatalf("event = %+v, want %+v", event, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("event %+v did not arrive within 2s", want)
	}
}

func directListenerPID(t *testing.T, db *remotetest.Database, replica string) int32 {
	t.Helper()
	var pid int32
	err := db.Pool.QueryRow(t.Context(),
		"SELECT pid FROM pg_stat_activity WHERE application_name = $1 AND query = 'LISTEN cloud_events'",
		db.ApplicationName(replica, "direct"),
	).Scan(&pid)
	if err != nil {
		t.Fatalf("direct LISTEN session: %v", err)
	}
	return pid
}

func assertPooledConnections(t *testing.T, db *remotetest.Database) {
	t.Helper()
	var names int
	err := db.Pool.QueryRow(t.Context(),
		"SELECT count(DISTINCT application_name) FROM pg_stat_activity WHERE application_name IN ($1, $2)",
		db.ApplicationName("first", "pooled"), db.ApplicationName("second", "pooled"),
	).Scan(&names)
	if err != nil || names != 2 {
		t.Fatalf("pooled request sessions = %d, %v", names, err)
	}
}
