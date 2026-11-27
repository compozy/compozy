package remoteproto

import (
	"encoding/json/v2"
	"errors"
	"testing"
	"time"
)

const (
	testSecurityAccount  = "acc_fixture"
	testSecurityComputer = "cmp_abcdefgh"
)

func TestSecurityState(t *testing.T) {
	t.Parallel()

	t.Run("Should hold future leases and repair epoch gaps with snapshots (UT-009)", func(t *testing.T) {
		t.Parallel()
		now := time.Unix(fixtureIssuedAt, 0)
		signer := newTestSigner(t, "lease")
		verifier := grantVerifier(t, JWKS{Keys: []JWK{signer.PublicJWK()}}, nil)
		state := NewSecurityState(testSecurityComputer, testSecurityAccount)
		reconcile := func(SecuritySnapshot) error { return nil }
		applyTestSnapshot(t, state, 6, reconcile, true)
		lease := EntitlementLease{
			ComputerID: testSecurityComputer, AccountID: testSecurityAccount, Plan: "free",
			AllowedRoutes: []RouteKind{RouteDirect, RouteSSH}, ValidUntil: now.Add(LeaseTTL).Unix(),
			Sequence: 2, SecurityEpoch: 7,
		}
		applyTestLease(t, state, signer, verifier, lease, now, LeaseHeld)
		if _, ok := state.CurrentLease(now); ok || state.ReadyForMint(7) {
			t.Fatal("future lease admitted a mint at epoch 6")
		}
		applyTestSnapshot(t, state, 7, reconcile, true)
		if current, ok := state.CurrentLease(now); !ok || current.Sequence != 2 || !state.ReadyForMint(7) {
			t.Fatalf("lease was not admitted by snapshot 7: %+v, %t", current, ok)
		}
		applyTestLease(t, state, signer, verifier, lease, now, LeaseIgnored)
		lease.Sequence--
		applyTestLease(t, state, signer, verifier, lease, now, LeaseIgnored)
		if current, _ := state.CurrentLease(now); current.Sequence != 2 {
			t.Fatalf("lease fell back to sequence %d", current.Sequence)
		}

		applyTestSnapshot(t, state, 12, reconcile, true)
		applyTestSnapshot(t, state, 11, reconcile, false)
		applyTestSnapshot(t, state, 12, reconcile, false)
		changed, request, err := state.ApplyIncremental(
			ControlMessage{Type: ControlScopesChanged, SecurityEpoch: 14}, reconcile,
		)
		if err != nil || changed || request == nil || request.AppliedEpoch != 12 || state.ReadyForMint(12) {
			t.Fatalf("gap = changed %t, request %+v, err %v, mint %t",
				changed, request, err, state.ReadyForMint(12))
		}
		applyTestSnapshot(t, state, 13, reconcile, true)
		if state.ReadyForMint(13) {
			t.Fatal("an older repair snapshot released the hold")
		}
		applyTestSnapshot(t, state, 14, reconcile, true)
		if !state.ReadyForMint(14) {
			t.Fatal("epoch 14 did not release the hold")
		}
	})

	t.Run("Should reconcile revoked devices, narrowed scopes and unlink before ack (UT-011)", func(t *testing.T) {
		t.Parallel()
		state := NewSecurityState(testSecurityComputer, testSecurityAccount)
		a := SnapshotDevice{
			DeviceID: "dev_alpha", JKTs: []string{hashBase64([]byte("a"))},
			Scopes: []Scope{ScopeSessions, ScopeTerminal},
		}
		b := SnapshotDevice{
			DeviceID: "dev_bravo", JKTs: []string{hashBase64([]byte("b"))}, Scopes: []Scope{ScopeSessions},
		}
		sessions := map[DeviceID]SnapshotDevice{a.DeviceID: a, b.DeviceID: b}
		listenersOpen := true
		reconcile := func(snapshot SecuritySnapshot) error {
			if state.Snapshot().SecurityEpoch >= snapshot.SecurityEpoch {
				t.Fatal("new epoch became visible before reconciliation")
			}
			for id, session := range sessions {
				if !snapshot.Allows(id, session.JKTs[0], session.Scopes) {
					delete(sessions, id)
				}
			}
			if !snapshot.Linked {
				listenersOpen = false
			}
			return nil
		}
		snapshot := SecuritySnapshot{
			AccountID: testSecurityAccount, SecurityEpoch: 9, Linked: true, Devices: []SnapshotDevice{a},
		}
		if changed, err := state.ApplySnapshot(snapshot, reconcile); !changed || err != nil {
			t.Fatalf("snapshot 9 = %t, %v", changed, err)
		}
		if _, ok := sessions[a.DeviceID]; !ok || len(sessions) != 1 || state.Snapshot().SecurityEpoch != 9 {
			t.Fatalf("sessions after revocation = %+v", sessions)
		}
		snapshot.SecurityEpoch++
		snapshot.Devices[0].Scopes = []Scope{ScopeSessions}
		if changed, err := state.ApplySnapshot(snapshot, reconcile); !changed || err != nil || len(sessions) != 0 {
			t.Fatalf("scope narrowing = changed %t, err %v, sessions %+v", changed, err, sessions)
		}
		snapshot.SecurityEpoch++
		snapshot.Linked = false
		if changed, err := state.ApplySnapshot(snapshot, reconcile); !changed || err != nil || listenersOpen {
			t.Fatalf("unlink = changed %t, err %v, listeners open %t", changed, err, listenersOpen)
		}
	})

	t.Run("Should apply an incremental revocation only after reconciliation", func(t *testing.T) {
		t.Parallel()
		state := NewSecurityState(testSecurityComputer, testSecurityAccount)
		device := SnapshotDevice{DeviceID: "dev_alpha", JKTs: []string{hashBase64([]byte("a"))}}
		snapshot := SecuritySnapshot{
			AccountID: testSecurityAccount, SecurityEpoch: 1, Linked: true, Devices: []SnapshotDevice{device},
		}
		reconcile := func(SecuritySnapshot) error { return nil }
		if changed, err := state.ApplySnapshot(snapshot, reconcile); !changed || err != nil {
			t.Fatalf("initial snapshot = %t, %v", changed, err)
		}
		data, err := json.Marshal(RevokeDevice{DeviceID: device.DeviceID})
		if err != nil {
			t.Fatal(err)
		}
		msg := ControlMessage{Type: ControlRevokeDevice, SecurityEpoch: 2, Data: data}
		refused := errors.New("reconciliation could not finish")
		if changed, _, err := state.ApplyIncremental(msg, func(SecuritySnapshot) error { return refused }); changed ||
			!errors.Is(err, refused) || state.Snapshot().SecurityEpoch != 1 {
			t.Fatalf("failed reconciliation advanced epoch: changed %t, err %v", changed, err)
		}
		if changed, repair, err := state.ApplyIncremental(msg, reconcile); !changed || repair != nil || err != nil {
			t.Fatalf("contiguous revoke = %t, %+v, %v", changed, repair, err)
		}
		if snapshot := state.Snapshot(); snapshot.SecurityEpoch != 2 ||
			snapshot.Allows(device.DeviceID, device.JKTs[0], nil) {
			t.Fatalf("revocation did not remove the device at epoch 2: %+v", snapshot)
		}
	})
}

func applyTestSnapshot(
	t *testing.T, state *SecurityState, epoch uint64, reconcile func(SecuritySnapshot) error, want bool,
) {
	t.Helper()
	changed, err := state.ApplySnapshot(
		SecuritySnapshot{AccountID: testSecurityAccount, SecurityEpoch: epoch, Linked: true}, reconcile,
	)
	if err != nil || changed != want {
		t.Fatalf("ApplySnapshot(%d) = %t, %v, want %t", epoch, changed, err, want)
	}
}

func applyTestLease(
	t *testing.T, state *SecurityState, signer *Signer, verifier Verifier, lease EntitlementLease,
	now time.Time, want LeaseDisposition,
) {
	t.Helper()
	token, err := signer.SignLease(lease)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := verifier.VerifyLease(t.Context(), token, now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := state.ApplyLease(signed, now)
	if err != nil || got != want {
		t.Fatalf("ApplyLease(%+v) = %v, %v, want %v", lease, got, err, want)
	}
}
