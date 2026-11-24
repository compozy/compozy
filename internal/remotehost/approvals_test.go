package remotehost_test

import (
	"context"
	"crypto/rand"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
)

func TestRemoteDeviceApprovals(t *testing.T) {
	t.Run("Should consume pending grants and require a fresh grant after a decision", func(t *testing.T) {
		t.Parallel()
		// UT-024. Safety Invariants 3 and 4.
		f := newSessionFixture(t, true, nil)
		f.lease(t, 1)
		input := f.input(t, "dev_member", remoteproto.ScopeSessions)
		_, err := f.sessions.Mint(t.Context(), input)
		failure := remotehost.DescribeError(err)
		remoteEqual(t, "device_pending_approval", failure.Code)
		approvalID := failure.Details["approval_id"]
		remoteContains(t, approvalID, "apr_")
		approvals, err := f.db.RemoteApprovals(t.Context())
		remoteOK(t, err)
		remoteEqual(t, 1, len(approvals))
		_, err = f.sessions.Mint(t.Context(), input)
		remoteErrorIs(t, err, remotehost.ErrGrantReplay)
		f.clock.Advance(remoteproto.ApprovalTTL / 2)
		remoteOK(t, f.sessions.DecideLocal(t.Context(), "dev_member", approvalID, true))
		_, err = f.sessions.Mint(t.Context(), f.input(t, "dev_member", remoteproto.ScopeSessions))
		remoteOK(t, err)

		_, err = f.sessions.Mint(t.Context(), f.input(t, "dev_admin", remoteproto.ScopeSessions))
		remoteEqual(t, "device_pending_approval", remotehost.DescribeError(err).Code)
		remoteOK(t, f.sessions.DecideLocal(t.Context(), "dev_admin", "", false))
		_, err = f.sessions.Mint(t.Context(), f.input(t, "dev_admin", remoteproto.ScopeSessions))
		remoteErrorIs(t, err, remotehost.ErrApprovalDenied)
	})
	t.Run("Should deduplicate pending requests and replace an expired request", func(t *testing.T) {
		t.Parallel()
		// UT-013. Safety Invariant 3.
		f := newSessionFixture(t, true, nil)
		f.lease(t, 1)
		inputs := []remotehost.MintInput{
			f.input(t, "dev_member", remoteproto.ScopeSessions),
			f.input(t, "dev_member", remoteproto.ScopeSessions),
		}
		var wait sync.WaitGroup
		var failures [2]error
		for i, input := range inputs {
			wait.Go(func() { _, failures[i] = f.sessions.Mint(t.Context(), input) })
		}
		wait.Wait()
		first, second := remotehost.DescribeError(failures[0]), remotehost.DescribeError(failures[1])
		remoteEqual(t, "device_pending_approval", first.Code)
		remoteEqual(t, first, second)
		approvals, err := f.db.RemoteApprovals(t.Context())
		remoteOK(t, err)
		remoteEqual(t, 1, len(approvals))
		f.clock.Advance(remoteproto.ApprovalTTL)
		_, err = f.sessions.Mint(t.Context(), f.input(t, "dev_member", remoteproto.ScopeSessions))
		remoteErrorIs(t, err, remotehost.ErrApprovalExpired)
		_, err = f.sessions.Mint(t.Context(), f.input(t, "dev_member", remoteproto.ScopeSessions))
		renewed := remotehost.DescribeError(err)
		remoteEqual(t, "device_pending_approval", renewed.Code)
		if renewed.Details["approval_id"] == first.Details["approval_id"] {
			t.Fatal("expired approval was reused")
		}
	})
	t.Run("Should authenticate an approved admin decision from persisted state after restart", func(t *testing.T) {
		t.Parallel()
		// UT-013. Safety Invariant 25.
		var applied remoteproto.ApprovalApplied
		f := newSessionFixture(t, true, func(kind remoteproto.ControlType, _ uint64, data any) {
			if kind == remoteproto.ControlApprovalApplied {
				applied = data.(remoteproto.ApprovalApplied)
			}
		})
		f.lease(t, 1)
		_, err := f.sessions.Mint(t.Context(), f.input(t, "dev_admin"))
		remoteEqual(t, "device_pending_approval", remotehost.DescribeError(err).Code)
		remoteOK(t, f.sessions.DecideLocal(t.Context(), "dev_admin", "", true))
		_, err = f.sessions.Mint(t.Context(), f.input(t, "dev_member"))
		pending := remotehost.DescribeError(err)
		remoteEqual(t, "device_pending_approval", pending.Code)
		claims := remoteproto.ApprovalDecision{
			ApprovalID: pending.Details["approval_id"], ComputerID: f.identity.ComputerID,
			DeviceID: "dev_member", JKT: f.snapshot.Devices[1].JKTs[0], Decision: "approve",
			IssuedAt: f.clock.Now().Unix(), JTI: rand.Text(),
		}
		f.restart(t, true)
		t.Run("Should refuse an unapproved admin key", func(t *testing.T) {
			token, err := remoteproto.SignApprovalDecision(f.keys["dev_other_admin"], claims)
			remoteOK(t, err)
			assertApprovalReport(t, f, token, &applied, "approver_not_eligible")
		})
		t.Run("Should refuse a non-admin signing key", func(t *testing.T) {
			token, err := remoteproto.SignApprovalDecision(f.keys["dev_member"], claims)
			remoteOK(t, err)
			assertApprovalReport(t, f, token, &applied, "approval_signature_invalid")
		})
		t.Run("Should check the signature against the protected key", func(t *testing.T) {
			valid, err := remoteproto.SignApprovalDecision(f.keys["dev_admin"], claims)
			remoteOK(t, err)
			other, err := remoteproto.SignApprovalDecision(f.keys["dev_member"], claims)
			remoteOK(t, err)
			otherInput, _, _ := strings.CutLast(other, ".")
			_, validSignature, _ := strings.CutLast(valid, ".")
			assertApprovalReport(t, f, otherInput+"."+validSignature, &applied, "approval_signature_invalid")
		})
		token, err := remoteproto.SignApprovalDecision(f.keys["dev_admin"], claims)
		remoteOK(t, err)
		assertApprovalReport(t, f, token, &applied, "approved")
		assertApprovalReport(t, f, token, &applied, "approved")
	})
}

func assertApprovalReport(
	t *testing.T,
	f *sessionFixture,
	token string,
	report *remoteproto.ApprovalApplied,
	want string,
) {
	t.Helper()
	*report = remoteproto.ApprovalApplied{}
	remoteOK(t, f.sessions.ApplyApproval(context.WithoutCancel(t.Context()), token))
	remoteEqual(t, want, report.Result)
	remoteContains(t, report.ApprovalID, "apr_")
}
