//go:build integration

package api_test

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/udsapi"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/observe"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
	"github.com/compozy/compozy/internal/workspace"
)

const remoteFixtureEmail = "remote-lab@example.test"
const remoteFixtureStream = "/api/terminals/test/stream"

type remoteHostLab struct {
	test    authFixture
	paths   config.HomePaths
	db      *globaldb.GlobalDB
	service *account.Service
	cloud   *cloudclient.Client
	host    *remotehost.Host
	owner   remoteTestDevice
	member  remoteTestDevice
}

func newRemoteHostLab(t *testing.T, f authFixture, home string) *remoteHostLab {
	t.Helper()
	t.Setenv("COMPOZY_HOME", home)
	t.Setenv("COMPOZY_CLOUD_URL", f.replica.Server.URL)
	paths, err := config.ResolveHomePathsFrom(home)
	if err != nil {
		t.Fatal(err)
	}
	db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(home, "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Errorf("close remote host database: %v", err)
		}
	})
	client, err := cloudclient.New(cloudclient.WithClock(f.clock.Now))
	if err != nil {
		t.Fatal(err)
	}
	service, err := account.New(t.Context(), db, client, home,
		account.WithClock(f.clock), account.WithKeyring(&daemonAccountKeyring{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)
	linkLabAccount(t, f, service)
	status := service.Status()
	owner := remoteTestDevice{id: status.Profile.Device.ID, account: status.AccountID}
	err = service.UseDevice(t.Context(), func(_ context.Context, creds cloudclient.Credentials) error {
		owner.key, owner.access = creds.Key, creds.AccessToken
		return nil
	})
	if err != nil || owner.access == "" {
		t.Fatalf("read owner credential: %v", err)
	}
	member := enrollRemoteDevice(t, f, remoteFixtureEmail, "remote-lab-member")
	return &remoteHostLab{test: f, paths: paths, db: db, service: service, cloud: client, owner: owner, member: member}
}

func linkLabAccount(t *testing.T, f authFixture, service *account.Service) {
	t.Helper()
	login, err := service.StartLogin(t.Context(), auth.Google)
	if err != nil {
		t.Fatal(err)
	}
	grant := remoteproto.DeviceFlowResponse{UserCode: login.UserCode}
	cookie := f.openActivation(t, grant, auth.Google)
	redirect := f.continueGrant(t, grant, auth.Google, cookie, false)
	path := f.callbackPath(t, auth.Google, redirect, providerProfile{
		email: remoteFixtureEmail, subject: remoteFixtureEmail, verified: true,
	})
	f.request(t, http.MethodGet, path, nil, cookie).expect(t, http.StatusOK, "Return to CompozyOS.")
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if err := f.clock.BlockUntilContext(ctx, 2); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(5 * time.Second)
	testutil.Eventually(t, "remote host account sign-in", 5*time.Second, func() bool {
		return service.Status().State == account.SignedIn
	})
}

func (l *remoteHostLab) start(t *testing.T, storage remotehost.Store, operator http.Handler) *remotetest.Tunnel {
	t.Helper()
	cfg := config.DefaultRemoteConfig()
	cfg.Listener = "127.0.0.1:0"
	l.startHost(t, storage, operator, cfg)
	address := l.host.Status().Listener
	descriptor := remotetest.TunnelDescriptor(t, address)
	return remotetest.DialTunnel(t, address, descriptor, l.test.clock.Now)
}

func (l *remoteHostLab) startHost(
	t *testing.T, storage remotehost.Store, operator http.Handler, cfg config.RemoteConfig,
) {
	t.Helper()
	host, err := remotehost.NewHost(t.Context(), storage, l.cloud, l.service, cfg, remotehost.HostOptions{
		Now: l.test.clock.Now, Operator: operator,
	})
	if err != nil {
		t.Fatal(err)
	}
	l.host = host
	t.Cleanup(host.Close)
	link, err := host.Link(t.Context(), "")
	if err != nil || !link.Enabled || link.ComputerID == "" {
		t.Fatalf("link remote host: %v", err)
	}
	testutil.Eventually(t, "control lease on linked computer", 5*time.Second, func() bool {
		status := host.Status()
		return status.ControlLink == "connected" && status.Entitlement != nil
	})
}

func (l *remoteHostLab) sessionServices(t *testing.T) (*session.Manager, *workspace.Resolver) {
	t.Helper()
	workspaces, err := workspace.NewResolver(l.db, workspace.WithHomePaths(l.paths))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := session.NewManager(session.WithHomePaths(l.paths), session.WithSessionCatalog(l.db),
		session.WithWorkspaceResolver(workspaces))
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
	return manager, workspaces
}

func (l *remoteHostLab) startUDS(t *testing.T) {
	t.Helper()
	manager, workspaces := l.sessionServices(t)
	tasks, err := task.NewManager(task.WithStore(l.db))
	if err != nil {
		t.Fatal(err)
	}
	observer, err := observe.New(t.Context(), observe.WithHomePaths(l.paths), observe.WithRegistry(l.db))
	if err != nil {
		t.Fatal(err)
	}
	server, err := udsapi.New(
		udsapi.WithHomePaths(l.paths),
		udsapi.WithSessionManager(manager),
		udsapi.WithTaskService(tasks),
		udsapi.WithObserver(observer),
		udsapi.WithWorkspaceResolver(workspaces),
		udsapi.WithRemoteHost(l.host),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Errorf("shutdown remote host UDS: %v", err)
		}
	})
}

func (l *remoteHostLab) mint(
	t *testing.T,
	tunnel *remotetest.Tunnel,
	device remoteTestDevice,
) remoteproto.MintResponse {
	t.Helper()
	grant := fetchRemoteGrant(t, l.test, device, l.host.Status().ComputerID)
	return mintRemote(t, tunnel, device, grant, l.test.clock.Now())
}

func (l *remoteHostLab) readSnapshot(t *testing.T) remoteproto.SecuritySnapshot {
	t.Helper()
	identity, err := l.db.RemoteIdentity(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := l.db.RemoteSnapshot(t.Context(), identity)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func (l *remoteHostLab) input(
	t *testing.T, method, path string, payload any, device remoteTestDevice, mint remoteproto.MintResponse,
) *http.Request {
	t.Helper()
	return remotetest.TunnelAuthorizedRequest(t, method, path, payload, device.key, mint.Credential, l.test.clock.Now())
}

func remoteScopeFixture(closed *atomic.Bool) http.Handler {
	return httpapi.NewRemoteRoutes(func(routes *httpapi.RemoteRoutes) {
		routes.Group("/api/sessions", remoteproto.ScopeSessions).GET("", func(c *gin.Context) {
			p, _ := remotehost.PrincipalFromContext(c.Request.Context())
			c.JSON(http.StatusOK, p.Scopes)
		})
		routes.Group(remoteFixtureStream, remoteproto.ScopeTerminal).GET("", func(c *gin.Context) {
			serveTerminalFixture(c, closed)
		})
	})
}

func serveTerminalFixture(c *gin.Context, closed *atomic.Bool) {
	upgrader := websocket.Upgrader{HandshakeTimeout: 2 * time.Second}
	socket, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() { _ = socket.Close() }()
	remotehost.OnStreamClose(c.Request.Context(), func(code int, reason string) {
		if err := socket.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason),
			time.Now().Add(time.Second)); err == nil {
			closed.Store(true)
		}
		_ = socket.Close()
	})
	if err := socket.WriteMessage(websocket.TextMessage, []byte("ready")); err == nil {
		_, _, _ = socket.ReadMessage()
	}
}

func openRemoteTerminal(
	t *testing.T, l *remoteHostLab, tunnel *remotetest.Tunnel, device remoteTestDevice, mint remoteproto.MintResponse,
) *websocket.Conn {
	t.Helper()
	socket := tunnel.OpenStream(t, l.input(t, http.MethodGet, remoteFixtureStream, nil, device, mint))
	if err := socket.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, frame, err := socket.ReadMessage(); err != nil || string(frame) != "ready" {
		t.Fatalf("remote terminal did not start: %v", err)
	}
	return socket
}

func expectTerminalClose(t *testing.T, socket *websocket.Conn, code int) {
	t.Helper()
	if err := socket.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		_, _, err := socket.ReadMessage()
		if err == nil {
			continue
		}
		closed, ok := errors.AsType[*websocket.CloseError](err)
		if !ok || closed.Code != code {
			t.Fatalf("remote terminal closed with %v, want %d", err, code)
		}
		return
	}
}

func snapshotScopes(snapshot remoteproto.SecuritySnapshot, id remoteproto.DeviceID) []remoteproto.Scope {
	index := slices.IndexFunc(
		snapshot.Devices,
		func(device remoteproto.SnapshotDevice) bool { return device.DeviceID == id },
	)
	if index < 0 {
		return nil
	}
	return snapshot.Devices[index].Scopes
}
