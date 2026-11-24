package remotehost_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"

	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const testMintURL = "https://compozy-remote" + remotehost.SessionPath
const testReadURL = "https://compozy-remote/api/sessions"

type sessionFixture struct {
	db       *globaldb.GlobalDB
	clock    *clockwork.FakeClock
	identity remotehost.Identity
	signer   *remoteproto.Signer
	verifier *remoteproto.JWSVerifier
	sessions *remotehost.Sessions
	keys     map[remoteproto.DeviceID]*ecdsa.PrivateKey
	snapshot remoteproto.SecuritySnapshot
	report   func(remoteproto.ControlType, uint64, any)
}

func newSessionFixture(t *testing.T, approval bool, report func(remoteproto.ControlType, uint64, any)) *sessionFixture {
	t.Helper()
	clock := clockwork.NewFakeClockAt(time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC))
	db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "compozy.db"))
	remoteOK(t, err)
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Errorf("close remote host database: %v", err)
		}
	})
	_, key, err := ed25519.GenerateKey(rand.Reader)
	remoteOK(t, err)
	signer, err := remoteproto.NewSigner("fixture", key)
	remoteOK(t, err)
	verifier, err := remoteproto.NewJWSVerifier(remoteproto.CloudIssuer, remoteproto.JWKS{
		Keys: []remoteproto.JWK{signer.PublicJWK()},
	}, nil)
	remoteOK(t, err)
	identity := remotehost.Identity{ComputerID: "cmp_fixture", AccountID: "acc_fixture",
		LinkedAt: clock.Now(), TLSLeafNotAfter: clock.Now().Add(time.Hour)}
	fixture := &sessionFixture{
		db: db, clock: clock, identity: identity, signer: signer, verifier: verifier,
		keys: make(map[remoteproto.DeviceID]*ecdsa.PrivateKey), report: report,
		snapshot: remoteproto.SecuritySnapshot{AccountID: identity.AccountID, SecurityEpoch: 7, Linked: true},
	}
	for _, id := range []remoteproto.DeviceID{"dev_admin", "dev_member", "dev_other_admin"} {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		remoteOK(t, err)
		jwk, err := remoteproto.NewPublicJWK(key.Public())
		remoteOK(t, err)
		jkt, err := jwk.Thumbprint()
		remoteOK(t, err)
		fixture.keys[id] = key
		admin := id != "dev_member"
		fixture.snapshot.Devices = append(fixture.snapshot.Devices, remoteproto.SnapshotDevice{
			DeviceID: id, JKTs: []string{jkt}, Admin: admin, Scopes: remoteproto.DefaultScopes(admin),
		})
	}
	fixture.restart(t, approval)
	changed, err := fixture.sessions.ApplySnapshot(t.Context(), fixture.snapshot)
	remoteOK(t, err)
	remoteEqual(t, true, changed)
	return fixture
}

func (f *sessionFixture) restart(t *testing.T, approval bool) {
	t.Helper()
	var err error
	f.sessions, err = remotehost.NewSessions(t.Context(), f.db, f.identity, f.verifier, remotehost.SessionOptions{
		Now: f.clock.Now, RequireApproval: approval, Report: f.report,
	})
	remoteOK(t, err)
}

func (f *sessionFixture) lease(t *testing.T, seq uint64) {
	t.Helper()
	remoteOK(t, f.sessions.ApplyLease(t.Context(), remoteproto.EntitlementLease{
		ComputerID: f.identity.ComputerID, AccountID: f.identity.AccountID, Plan: "free", Sequence: seq,
		AllowedRoutes: []remoteproto.RouteKind{remoteproto.RouteDirect, remoteproto.RouteSSH},
		ValidUntil:    f.clock.Now().Add(remoteproto.LeaseTTL).Unix(), SecurityEpoch: f.snapshot.SecurityEpoch,
	}))
}

func (f *sessionFixture) mintInput(
	t *testing.T, id remoteproto.DeviceID, epoch uint64, route remoteproto.RouteKind, kind remotehost.Listener,
	scopes ...remoteproto.Scope,
) remotehost.MintInput {
	t.Helper()
	jwk, err := remoteproto.NewPublicJWK(f.keys[id].Public())
	remoteOK(t, err)
	jkt, err := jwk.Thumbprint()
	remoteOK(t, err)
	now := f.clock.Now()
	grant, err := f.signer.SignGrant(remoteproto.GrantClaims{
		Issuer: remoteproto.CloudIssuer, AccountID: f.identity.AccountID, Audience: f.identity.ComputerID,
		Subject: id, Cnf: remoteproto.Confirmation{JKT: jkt}, JTI: rand.Text(), SecurityEpoch: epoch,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(remoteproto.GrantTTL).Unix(), Route: route, Scopes: scopes,
	})
	remoteOK(t, err)
	proof, err := remoteproto.BuildDPoP(f.keys[id], http.MethodPost, testMintURL, "", now)
	remoteOK(t, err)
	return remotehost.MintInput{Grant: grant, Proof: remotehost.DPoPProof{
		Token: proof, Method: http.MethodPost, URL: testMintURL, Listener: kind,
	}}
}

func (f *sessionFixture) input(
	t *testing.T,
	id remoteproto.DeviceID,
	scopes ...remoteproto.Scope,
) remotehost.MintInput {
	t.Helper()
	return f.mintInput(t, id, f.snapshot.SecurityEpoch, remoteproto.RouteSSH, remotehost.ListenerTunnel, scopes...)
}

func (f *sessionFixture) authenticate(
	t *testing.T,
	id remoteproto.DeviceID,
	result remotehost.MintResult,
) (remotehost.Principal, error) {
	t.Helper()
	proof, err := remoteproto.BuildDPoP(f.keys[id], http.MethodGet, testReadURL, result.Credential, f.clock.Now())
	remoteOK(t, err)
	return f.sessions.Authenticate(t.Context(), result.Credential, remotehost.DPoPProof{
		Token: proof, Method: http.MethodGet, URL: testReadURL, Listener: remotehost.ListenerTunnel,
	}, f.clock.Now())
}

func TestRemoteSessionAdmission(t *testing.T) {
	t.Run("Should intersect scopes and enforce the declared route scope", func(t *testing.T) {
		t.Parallel()
		// UT-021, UT-022. Safety Invariants 4 and 5.
		f := newSessionFixture(t, false, nil)
		f.snapshot.SecurityEpoch++
		f.snapshot.Devices[1].Scopes = []remoteproto.Scope{remoteproto.ScopeSessions, remoteproto.ScopeFilesR}
		_, err := f.sessions.ApplySnapshot(t.Context(), f.snapshot)
		remoteOK(t, err)
		f.lease(t, 1)
		mint, err := f.sessions.Mint(t.Context(), f.input(t, "dev_member",
			remoteproto.ScopeSessions, remoteproto.ScopeFilesR, remoteproto.ScopeTerminal))
		remoteOK(t, err)
		principal, err := f.authenticate(t, "dev_member", mint)
		remoteOK(t, err)
		remoteEqual(
			t,
			map[remoteproto.Scope]bool{remoteproto.ScopeSessions: true, remoteproto.ScopeFilesR: true},
			principal.Scopes,
		)
		ctx, release, err := f.sessions.BeginRequest(t.Context(), principal, false)
		remoteOK(t, err)
		defer release()
		router := httpapi.NewRemoteRoutes(func(routes *httpapi.RemoteRoutes) {
			routes.Group("/api/terminals", remoteproto.ScopeTerminal).GET("", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
		})
		reply := httptest.NewRecorder()
		router.ServeHTTP(reply, httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/terminals", nil))
		remoteEqual(t, http.StatusForbidden, reply.Code)
		remoteContains(t, reply.Body.String(), `"code":"remote_scope_denied"`)
		remoteContains(t, reply.Body.String(), `"scope":"terminal:operate"`)
	})
	t.Run("Should consume a signed grant exactly once", func(t *testing.T) {
		t.Parallel()
		// UT-025. Safety Invariant 3.
		f := newSessionFixture(t, false, nil)
		f.lease(t, 1)
		input := f.input(t, "dev_member", remoteproto.ScopeSessions)
		_, err := f.sessions.Mint(t.Context(), input)
		remoteOK(t, err)
		_, err = f.sessions.Mint(t.Context(), input)
		remoteErrorIs(t, err, remotehost.ErrGrantReplay)
	})
	t.Run("Should require an entitlement and match to the arrival listener", func(t *testing.T) {
		t.Parallel()
		// UT-026, UT-027. Safety Invariants 4 and 15.
		f := newSessionFixture(t, false, nil)
		_, err := f.sessions.Mint(t.Context(), f.input(t, "dev_member", remoteproto.ScopeSessions))
		remoteErrorIs(t, err, remotehost.ErrEntitlement)
		f.lease(t, 1)
		_, err = f.sessions.Mint(
			t.Context(),
			f.mintInput(t, "dev_member", 7, remoteproto.RouteCloud, remotehost.ListenerTunnel),
		)
		remoteErrorIs(t, err, remotehost.ErrRouteMismatch)
		_, err = f.sessions.Mint(
			t.Context(),
			f.mintInput(t, "dev_member", 7, remoteproto.RouteCloud, remotehost.ListenerHosted),
		)
		remoteErrorIs(t, err, remotehost.ErrPlanRequired)
	})
	t.Run("Should prove the bound key and expire at sixty minutes", func(t *testing.T) {
		t.Parallel()
		// UT-028, UT-029. Safety Invariants 4 and 18.
		f := newSessionFixture(t, false, nil)
		f.lease(t, 1)
		mint, err := f.sessions.Mint(t.Context(), f.input(t, "dev_member", remoteproto.ScopeSessions))
		remoteOK(t, err)
		_, err = f.authenticate(t, "dev_admin", mint)
		remoteErrorIs(t, err, remoteproto.ErrDPoPInvalid)
		f.clock.Advance(remotehost.SessionTTL)
		_, err = f.authenticate(t, "dev_member", mint)
		remoteErrorIs(t, err, remotehost.ErrSessionExpired)
	})
	t.Run("Should close every live stream of the revoked device", func(t *testing.T) {
		t.Parallel()
		// UT-023. Safety Invariants 4 and 10.
		f := newSessionFixture(t, false, nil)
		f.lease(t, 1)
		mint, err := f.sessions.Mint(t.Context(), f.input(t, "dev_member", remoteproto.ScopeSessions))
		remoteOK(t, err)
		var streams []context.Context
		var closes []int
		for range 2 {
			principal, err := f.authenticate(t, "dev_member", mint)
			remoteOK(t, err)
			ctx, release, err := f.sessions.BeginRequest(t.Context(), principal, false)
			remoteOK(t, err)
			defer release()
			streams = append(streams, ctx)
			remotehost.OnStreamClose(ctx, func(code int, _ string) { closes = append(closes, code) })
		}
		count, err := f.sessions.RevokeDevice(t.Context(), "dev_member")
		remoteOK(t, err)
		remoteEqual(t, 2, count)
		remoteEqual(t, []int{4503, 4503}, closes)
		for _, stream := range streams {
			remoteErrorIs(t, context.Cause(stream), remotehost.ErrDeviceRevoked)
		}
		_, err = f.authenticate(t, "dev_member", mint)
		remoteErrorIs(t, err, remotehost.ErrDeviceRevoked)
	})
}

func TestRemoteSessionEpochAdmission(t *testing.T) {
	t.Run("Should hold a future grant until its snapshot arrives", func(t *testing.T) {
		t.Parallel()
		// UT-109. Safety Invariant 22.
		requests := make(chan remoteproto.SnapshotRequest, 2)
		f := newSessionFixture(t, false, func(kind remoteproto.ControlType, _ uint64, data any) {
			if kind == remoteproto.ControlSnapshotRequest {
				requests <- data.(remoteproto.SnapshotRequest)
			}
		})
		f.lease(t, 1)
		input := f.mintInput(
			t,
			"dev_member",
			8,
			remoteproto.RouteSSH,
			remotehost.ListenerTunnel,
			remoteproto.ScopeSessions,
		)
		result := make(chan error, 1)
		var worker sync.WaitGroup
		worker.Go(func() {
			_, err := f.sessions.Mint(t.Context(), input)
			result <- err
		})
		defer worker.Wait()
		select {
		case request := <-requests:
			remoteEqual(t, uint64(7), request.AppliedEpoch)
		case <-time.After(time.Second):
			t.Fatal("snapshot request was not reported")
		}
		select {
		case err := <-result:
			t.Fatalf("mint ended before its snapshot: %v", err)
		default:
		}
		f.snapshot.SecurityEpoch++
		_, err := f.sessions.ApplySnapshot(t.Context(), f.snapshot)
		remoteOK(t, err)
		remoteOK(t, <-result)
	})
	t.Run("Should return pending without consuming the grant at the epoch deadline", func(t *testing.T) {
		t.Parallel()
		// UT-109. Safety Invariants 3 and 22.
		f := newSessionFixture(t, false, nil)
		f.lease(t, 1)
		input := f.mintInput(
			t,
			"dev_member",
			8,
			remoteproto.RouteSSH,
			remotehost.ListenerTunnel,
			remoteproto.ScopeSessions,
		)
		grant, err := f.verifier.VerifyGrant(t.Context(), input.Grant, f.clock.Now())
		remoteOK(t, err)
		_, err = f.sessions.Mint(t.Context(), input)
		remoteErrorIs(t, err, remotehost.ErrSnapshotPending)
		used, err := f.db.RemoteGrantUsed(t.Context(), grant.JTI)
		remoteOK(t, err)
		remoteEqual(t, false, used)
		f.snapshot.SecurityEpoch++
		_, err = f.sessions.ApplySnapshot(t.Context(), f.snapshot)
		remoteOK(t, err)
		_, err = f.sessions.Mint(t.Context(), f.input(t, "dev_member", remoteproto.ScopeSessions))
		remoteOK(t, err)
	})
}

func remoteOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func remoteErrorIs(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("error = %v, want %v", got, want)
	}
}

func remoteEqual(t *testing.T, want, got any) {
	t.Helper()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("value mismatch (-want +got):\n%s", diff)
	}
}

func remoteContains(t *testing.T, got, text string) {
	t.Helper()
	if !strings.Contains(got, text) {
		t.Fatalf("%q does not contain %q", got, text)
	}
}
