//go:build integration

package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil/remotetest"
	"github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func TestCloudRemoteHostTransactions(t *testing.T) {
	t.Run("Should roll back a used grant when storing the session encounters SQLite busy", func(t *testing.T) {
		// IT-038. Safety Invariant 3.
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		lab := newRemoteHostLab(t, f, t.TempDir())
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
		lab := newRemoteHostLab(t, f, t.TempDir())
		paused := make(chan struct{})
		record := store.SessionInfo{ID: "sess_remote_rename", Name: "Original", ProfileID: store.DefaultProfileID,
			AgentName: "general", Provider: "codex", RuntimeStatus: store.SessionRuntimeUnbound,
			State: "stopped", CreatedAt: f.clock.Now(), UpdatedAt: f.clock.Now()}
		if err := lab.db.RegisterSession(t.Context(), &record); err != nil {
			t.Fatal(err)
		}
		metaPath := store.SessionMetaFile(filepath.Join(lab.paths.SessionsDir, record.ID))
		if err := store.WriteSessionMeta(metaPath, &store.SessionMeta{
			ID: record.ID, Name: record.Name, ProfileID: record.ProfileID, AgentName: record.AgentName,
			Provider: record.Provider, State: record.State, RuntimeStatus: record.RuntimeStatus,
			SessionType: string(session.SessionTypeUser), CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
		}); err != nil {
			t.Fatal(err)
		}
		path := "/api/workspaces/" + workspaceaccess.UserSelector + "/sessions/" + record.ID
		operator := remoteRenameRoutes(t, lab, paused)
		tunnel := lab.start(t, lab.db, operator)
		mint := lab.mint(t, tunnel, lab.member)
		request := lab.input(
			t,
			http.MethodPatch,
			path,
			contract.RenameSessionRequest{Name: "Renamed"},
			lab.member,
			mint,
		)
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
		if meta, err := store.ReadSessionMeta(metaPath); err != nil || meta.Name != record.Name {
			t.Fatalf("rename escaped the metadata rollback: %q %v", meta.Name, err)
		}
	})
}

func remoteRenameRoutes(t *testing.T, lab *remoteHostLab, paused chan<- struct{}) http.Handler {
	t.Helper()
	manager, err := session.NewManager(session.WithHomePaths(lab.paths), session.WithSessionCatalog(lab.db))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := manager.Shutdown(ctx); err != nil {
			t.Errorf("shut down session manager: %v", err)
		}
	})
	workspaces, err := workspace.NewResolver(lab.db, workspace.WithHomePaths(lab.paths))
	if err != nil {
		t.Fatal(err)
	}
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: "remote", MaskInternalErrors: true, Sessions: manager, Workspaces: workspaces,
	})
	pauseBeforeCommit := func(c *gin.Context) {
		ctx := store.ContextWithMutationCommitFence(c.Request.Context(), func(ctx context.Context) error {
			close(paused)
			<-ctx.Done()
			return nil
		})
		c.Request = c.Request.WithContext(ctx)
	}
	return httpapi.NewRemoteRoutes(func(routes *httpapi.RemoteRoutes) {
		routes.Group("/api/workspaces/:workspace_id/sessions", remoteproto.ScopeSessions,
			handlers.WorkspaceReferenceGuard, pauseBeforeCommit).PATCH("/:session_id", handlers.RenameSession)
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
