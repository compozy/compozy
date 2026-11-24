//go:build integration

package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestCloudRemoteHostTransactions(t *testing.T) {
	t.Run("Should roll back a used grant when storing the session encounters SQLite busy", func(t *testing.T) {
		// IT-038. Safety Invariant 3.
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		lab := newRemoteHostLab(t, f)
		target := url.URL{Scheme: "file", Path: lab.db.Path(), RawQuery: "_pragma=busy_timeout(0)"}
		contender, err := sql.Open("sqlite", target.String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := contender.Close(); err != nil {
				t.Errorf("close competing write connection: %v", err)
			}
		})
		fault := &busyRemoteStore{Store: lab.db, contender: contender}
		tunnel := lab.start(t, fault, nil)
		grant := fetchRemoteGrant(t, f, lab.member, lab.host.Status().ComputerID)
		verifier, err := remoteproto.NewJWSVerifier(f.replica.App.Issuer, f.cfg.Keys.JWKS(), nil)
		if err != nil {
			t.Fatal(err)
		}
		claims, err := verifier.VerifyGrant(t.Context(), grant, f.clock.Now())
		if err != nil {
			t.Fatal(err)
		}
		fault.armed.Store(true)
		status, body := tunnel.Mint(t, lab.member.key, grant, f.clock.Now())
		fault.armed.Store(false)
		if status != http.StatusInternalServerError || !bytes.Contains(body, []byte("remote_operation_failed")) ||
			!fault.sawBusy.Load() {
			t.Fatalf("mint did not fail at the injected SQLite write: HTTP %d %s", status, body)
		}
		var rows int
		if err := lab.db.ReadDB().
			QueryRowContext(t.Context(), "SELECT COUNT(*) FROM remote_sessions").
			Scan(&rows); err != nil ||
			rows != 0 {
			t.Fatalf("failed mint left %d session rows: %v", rows, err)
		}
		if used, err := lab.db.RemoteGrantUsed(t.Context(), claims.JTI); err != nil || used {
			t.Fatalf("failed mint consumed grant: used=%t error=%v", used, err)
		}
		lab.mint(t, tunnel, lab.member)
	})

	t.Run("Should reject a rename already at the commit boundary when its device is revoked", func(t *testing.T) {
		// IT-047. Safety Invariants 10 and 16.
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		lab := newRemoteHostLab(t, f)
		paused := make(chan struct{})
		record := store.SessionInfo{ID: "sess_remote_rename", Name: "Original", ProfileID: store.DefaultProfileID,
			AgentName: "general", Provider: "codex", RuntimeStatus: store.SessionRuntimeUnbound,
			State: "stopped", CreatedAt: f.clock.Now(), UpdatedAt: f.clock.Now()}
		if err := lab.db.RegisterSession(t.Context(), &record); err != nil {
			t.Fatal(err)
		}
		const path = "/api/workspaces/user/sessions/sess_remote_rename"
		operator := httpapi.NewRemoteRoutes(func(routes *httpapi.RemoteRoutes) {
			routes.Group(path, remoteproto.ScopeSessions).PATCH("", func(c *gin.Context) {
				ctx := store.ContextWithMutationCommitFence(c.Request.Context(), func(ctx context.Context) error {
					close(paused)
					<-ctx.Done()
					return nil
				})
				renamed := record
				renamed.Name = "Renamed"
				if err := lab.db.RegisterSession(ctx, &renamed); err != nil {
					core.RespondError(c, http.StatusInternalServerError, err, true)
					return
				}
				c.JSON(http.StatusOK, renamed.Name)
			})
		})
		tunnel := lab.start(t, lab.db, operator)
		mint := lab.mint(t, tunnel, lab.member)
		request := lab.input(t, http.MethodPatch, path, map[string]string{"name": "Renamed"}, lab.member, mint)
		type result struct {
			status int
			body   []byte
			err    error
		}
		completed := make(chan result, 1)
		go func() {
			status, body, err := tunnel.RoundTrip(request)
			completed <- result{status, body, err}
		}()
		receiveRemote(t, paused, 2*time.Second)
		remoteCloudRequest(t, f, lab.owner, http.MethodDelete, "/v1/devices/"+string(lab.member.id), nil).
			expect(t, http.StatusNoContent, "")
		rename := receiveRemote(t, completed, 3*time.Second)
		if rename.err != nil || rename.status != http.StatusForbidden ||
			!bytes.Contains(rename.body, []byte("device_revoked")) {
			t.Fatalf("in-flight rename was not fenced: %d %s %v", rename.status, rename.body, rename.err)
		}
		var name string
		if err := lab.db.ReadDB().QueryRowContext(t.Context(), "SELECT name FROM sessions WHERE id = ?", record.ID).
			Scan(&name); err != nil || name != record.Name {
			t.Fatalf("rename escaped the fence: %q %v", name, err)
		}
	})
}

type busyRemoteStore struct {
	remotehost.Store
	contender      *sql.DB
	armed, sawBusy atomic.Bool
}

func (s *busyRemoteStore) WriteRemote(ctx context.Context, run func(context.Context, remotehost.Tx) error) error {
	return s.Store.WriteRemote(ctx, func(ctx context.Context, tx remotehost.Tx) error {
		return run(ctx, busyRemoteTx{Tx: tx, source: s})
	})
}

type busyRemoteTx struct {
	remotehost.Tx
	source *busyRemoteStore
}

func (tx busyRemoteTx) InsertSession(ctx context.Context, session remotehost.Session) error {
	if !tx.source.armed.Load() {
		return tx.Tx.InsertSession(ctx, session)
	}
	_, err := tx.source.contender.ExecContext(ctx, "BEGIN IMMEDIATE")
	if store.IsSQLiteBusy(err) {
		tx.source.sawBusy.Store(true)
	}
	return err
}
