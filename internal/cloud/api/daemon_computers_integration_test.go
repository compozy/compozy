//go:build integration

package api_test

import (
	"context"
	"net/http"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	controllerTestName = "testhost"
	registryTestEmail  = "controller-registry@example.test"
	controllerSessions = "/api/sessions"
)

func TestDaemonComputers(t *testing.T) {
	root := t.TempDir()
	t.Run("Should add and remove an account computer without changing its slot until unlink", func(t *testing.T) {
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		lab := newRemoteHostLab(t, f, filepath.Join(root, "slot-home"))
		lab.startHost(t, lab.db, nil, directRemoteFixture())
		id := lab.host.Status().ComputerID
		controller, _ := startComputerController(t, f, filepath.Join(root, "slot-controller"), remoteFixtureEmail, nil)
		// IT-030. Safety Invariants 1, 2, 15 and 18. The controller uses the cloud-pinned direct route.
		available := controller.uds(t, http.MethodGet, contract.ComputersPath+"/available", nil)
		available.expect(t, http.StatusOK, string(id))
		entries := authJSON[contract.AvailableComputersResponse](t, available).Computers
		if len(entries) != 1 || !entries[0].Online || entries[0].Added {
			t.Fatalf("available computers = %+v", entries)
		}
		addAccountComputer(t, controller, id)
		waitComputerRoute(t, controller, id, remoteproto.RouteDirect)
		initial := readControllerComputer(t, controller, id)

		// IT-032 and IT-016. Safety Invariants 7 and 11. Local removal deletes the row and its routes.
		remove := controller.uds(t, http.MethodDelete, computerAPI(id), nil)
		remove.expect(t, http.StatusOK, `"removed":true`)
		controller.request(t, http.MethodGet, bridgeAPI(id, controllerSessions), nil).
			expect(t, http.StatusNotFound, "computer_not_found")
		remoteCloudRequest(t, f, lab.owner, http.MethodPost, "/v1/computers/reservations", nil).
			expect(t, http.StatusPaymentRequired, "plan_limit_reached")
		addAccountComputer(t, controller, id)
		readded := readControllerComputer(t, controller, id)
		for _, route := range readded.Routes {
			if slices.ContainsFunc(
				initial.Routes,
				func(old contract.ComputerRoute) bool { return old.ID == route.ID },
			) {
				t.Fatalf("route %q survived registry removal", route.ID)
			}
		}
		controller.uds(t, http.MethodDelete, computerAPI(id)+"?unlink=true", nil).
			expect(t, http.StatusOK, `"removed":true`)
		remoteCloudRequest(t, f, lab.owner, http.MethodPost, "/v1/computers/reservations", nil).
			expect(t, http.StatusCreated, "reservation")
	})

	t.Run("Should isolate sessions by computer over HTTP and UDS", func(t *testing.T) {
		f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
		controller, _ := startComputerController(
			t,
			f,
			filepath.Join(root, "sessions-controller"),
			registryTestEmail,
			nil,
		)
		first, _ := startSessionComputer(t, f, filepath.Join(root, "sessions-a"), false)
		second, _ := startSessionComputer(t, f, filepath.Join(root, "sessions-b"), false)
		provisionRegistryPro(t, f)
		a, b := linkSessionComputer(t, first), linkSessionComputer(t, second)
		wantLocal := makeSessionIDs(t, controller, "local")
		wantA, wantB := makeSessionIDs(t, first, "A"), makeSessionIDs(t, second, "B")
		for _, id := range []remoteproto.ComputerID{a, b} {
			addAccountComputer(t, controller, id)
			waitComputerRoute(t, controller, id, remoteproto.RouteDirect)
		}
		// IT-033. Safety Invariants 11 and 18. Identity is enforced on both controller transports.
		for _, route := range []struct {
			path string
			want []string
		}{
			{bridgeAPI(a, controllerSessions), wantA}, {bridgeAPI(b, controllerSessions), wantB},
			{controllerSessions, wantLocal}, {"/c/local" + controllerSessions, wantLocal},
		} {
			listPath := route.path + "?all_workspaces=true"
			expectComputerSessions(t, controller.request(t, http.MethodGet, listPath, nil), route.want)
			expectComputerSessions(t, controller.uds(t, http.MethodGet, listPath, nil), route.want)
		}
	})
}

func startComputerController(
	t *testing.T, f authFixture, home, email string, forward computers.ForwardResolver,
	extras ...daemon.Option,
) (daemonAccountFixture, *computers.Registry) {
	t.Helper()
	ready := make(chan *computers.Registry, 1)
	options := append(extras, daemon.WithComputersOptions(
		computers.WithClock(f.clock), computers.WithSSHForward(forward),
		func(registry *computers.Registry) { ready <- registry },
	))
	controller, _ := startConfiguredDaemonAccount(t, f, home, daemonAccountBoot{options: options})
	progress := controller.stream(t, contract.AccountSignInEventsPath, "")
	signInAccountDevice(t, f, controller, progress, email)
	progress.cancel()
	return controller, <-ready
}

func startSessionComputer(
	t *testing.T, f authFixture, home string, withACP bool, options ...daemon.Option,
) (daemonAccountFixture, *daemonAccountKeyring) {
	t.Helper()
	boot := daemonAccountBoot{direct: true, options: options}
	if withACP {
		configureComputerACP(t, home)
	}
	host, ring := startConfiguredDaemonAccount(t, f, home, boot)
	progress := host.stream(t, contract.AccountSignInEventsPath, "")
	signInAccountDevice(t, f, host, progress, registryTestEmail)
	progress.cancel()
	return host, ring
}

func provisionRegistryPro(t *testing.T, f authFixture) {
	t.Helper()
	_, err := f.conn.Exec(t.Context(), `
		INSERT INTO subscriptions (id, source, account_id, status, ends_at)
		SELECT 'sub_registry_test', 'complimentary', id, 'active', $2 FROM accounts WHERE email = $1
	`, registryTestEmail, f.clock.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
}

func linkSessionComputer(t *testing.T, host daemonAccountFixture) remoteproto.ComputerID {
	t.Helper()
	reply := host.uds(t, http.MethodPost, contract.RemoteLinkPath, contract.RemoteLinkRequest{})
	reply.expect(t, http.StatusOK, "computer_id")
	status := waitForHostLease(t, host)
	if len(status.Direct.Addresses) == 0 {
		t.Fatal("remote host has no private direct address")
	}
	return authJSON[contract.RemoteLinkResponse](t, reply).ComputerID
}

func directRemoteFixture() config.RemoteConfig {
	cfg := config.DefaultRemoteConfig()
	cfg.Listener = "127.0.0.1:0"
	cfg.Direct.Enabled = true
	return cfg
}

func computerAPI(id remoteproto.ComputerID) string            { return contract.ComputersPath + "/" + string(id) }
func bridgeAPI(id remoteproto.ComputerID, path string) string { return "/c/" + string(id) + path }

func addAccountComputer(t *testing.T, controller daemonAccountFixture, id remoteproto.ComputerID) {
	t.Helper()
	reply := controller.uds(t, http.MethodPost, contract.ComputersPath, contract.AddComputerRequest{
		Kind: "account", ComputerID: id,
	})
	reply.expect(t, http.StatusCreated, string(id))
}

func waitComputerRoute(
	t *testing.T, controller daemonAccountFixture, id remoteproto.ComputerID, kind remoteproto.RouteKind,
) {
	t.Helper()
	testutil.Eventually(t, "connected over "+string(kind), 9*time.Second, func() bool {
		view := readControllerComputer(t, controller, id)
		return view.Status == string(computers.Connected) && view.RouteInUse != nil && view.RouteInUse.Kind == kind
	})
}

func readControllerComputer(
	t *testing.T,
	controller daemonAccountFixture,
	id remoteproto.ComputerID,
) contract.Computer {
	t.Helper()
	reply := controller.uds(t, http.MethodGet, computerAPI(id), nil)
	reply.expect(t, http.StatusOK, string(id))
	return authJSON[contract.Computer](t, reply)
}

func makeSessionIDs(t *testing.T, host daemonAccountFixture, name string) []string {
	t.Helper()
	result := make([]string, 0, 3)
	for range 3 {
		reply := host.uds(t, http.MethodPost, controllerSessions, contract.CreateSessionRequest{
			Scope: "user", AgentName: config.DefaultAgentName, Name: name,
		})
		reply.expect(t, http.StatusCreated, "session")
		result = append(result, authJSON[contract.SessionResponse](t, reply).Session.ID)
	}
	slices.Sort(result)
	return result
}

func expectComputerSessions(t *testing.T, reply authReply, want []string) {
	t.Helper()
	reply.expect(t, http.StatusOK, "sessions")
	result := authJSON[contract.SessionCatalogResponse](t, reply)
	ids := make([]string, 0, len(result.Sessions))
	for _, item := range result.Sessions {
		ids = append(ids, item.ID)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, want) {
		t.Fatalf("session IDs = %v, want %v", ids, want)
	}
}

func addFixtureForward(t *testing.T, registry *computers.Registry, id remoteproto.ComputerID) {
	t.Helper()
	err := registry.AddSSHRoute(t.Context(), id, store.ComputerRoute{
		Kind:    remoteproto.RouteSSH,
		Address: "fixture@testhost",
		SSH:     &store.ComputerSSHTarget{User: "fixture", Port: 22},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func fixedFixtureForward(address string) computers.ForwardResolver {
	return func(context.Context, store.ComputerRoute) (string, error) { return address, nil }
}
