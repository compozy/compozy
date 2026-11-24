//go:build integration

package api_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestDaemonComputerBridge(t *testing.T) {
	root := t.TempDir()
	t.Run("Should close a revoked device's bridge stream with its policy close code", func(t *testing.T) {
		f, lab := newControllerHost(t, filepath.Join(root, "revoke"))
		var closed atomic.Bool
		lab.startHost(t, lab.db, remoteScopeFixture(&closed), directRemoteFixture())
		id := lab.host.Status().ComputerID
		ready := make(chan *account.Service, 1)
		controller, _ := startComputerController(
			t, f, filepath.Join(root, "revoke-controller"), remoteFixtureEmail, nil,
			daemon.WithAccountOptions(func(service *account.Service) { ready <- service }),
		)
		device := controllerCloudDevice(t, <-ready)
		addAccountComputer(t, controller, id)
		waitComputerRoute(t, controller, id, remoteproto.RouteDirect)
		socket := openBridgeTerminal(t, controller, id)

		// IT-017. Safety Invariants 10, 11 and 18. The WS policy frame passes through the bridge.
		remoteCloudRequest(t, f, lab.owner, http.MethodDelete, "/v1/devices/"+string(device.id), nil).
			expect(t, http.StatusNoContent, "")
		expectTerminalClose(t, socket, 4503)
		if !closed.Load() {
			t.Fatal("the target did not close the revoked bridge stream")
		}
		remoteCloudRequest(t, f, device, http.MethodPost, "/v1/computers/"+string(id)+"/grants",
			map[string]string{"route": "direct"}).expect(t, http.StatusForbidden, "device_revoked")
	})

	t.Run("Should fail closed on a route to a different computer before asking the cloud", func(t *testing.T) {
		f, lab := newControllerHost(t, filepath.Join(root, "different"))
		cfg := directRemoteFixture()
		cfg.Direct.Enabled = false
		lab.startHost(t, lab.db, nil, cfg)
		id := lab.host.Status().ComputerID
		other, _ := startSessionComputer(t, f, filepath.Join(root, "different-target"), false)
		_ = linkSessionComputer(t, other)
		forward := fixedFixtureForward(readRemoteDaemonStatus(t, other).Listener)
		controller, registry := startComputerController(
			t, f, filepath.Join(root, "different-controller"), remoteFixtureEmail, forward,
		)
		addAccountComputer(t, controller, id)
		addFixtureForward(t, registry, id)

		// IT-035. Safety Invariant 2. A real daemon's descriptor has a different identity.
		testutil.Eventually(t, "route identity refusal", 5*time.Second, func() bool {
			view := readControllerComputer(t, controller, id)
			return view.Diagnosis != nil && view.Diagnosis.Code == computers.ErrIdentityMismatch.Code
		})
		var grants int
		if err := f.conn.QueryRow(t.Context(), "SELECT count(*) FROM grants_issued WHERE computer_id = $1", id).
			Scan(&grants); err != nil || grants != 0 {
			t.Fatalf("wrong-identity route issued %d grants, query error %v", grants, err)
		}
	})

	t.Run("Should forward the lease close and surface the cloud diagnosis after the outage", func(t *testing.T) {
		f, lab := newControllerHost(t, filepath.Join(root, "lease"))
		var closed atomic.Bool
		lab.startHost(t, lab.db, remoteScopeFixture(&closed), directRemoteFixture())
		id := lab.host.Status().ComputerID
		controller, _ := startComputerController(
			t, f, filepath.Join(root, "lease-controller"), remoteFixtureEmail, nil,
		)
		addAccountComputer(t, controller, id)
		waitComputerRoute(t, controller, id, remoteproto.RouteDirect)
		socket := openBridgeTerminal(t, controller, id)
		// UT-038. Safety Invariants 11 and 20. A tunneled device principal cannot chain a bridge.
		controller.uds(t, http.MethodGet, bridgeAPI(id, "/c/local"+controllerSessions), nil).
			expect(t, http.StatusForbidden, "remote_bridge_not_transitive")
		// IT-036. Safety Invariants 4 and 14. Existing access expires with the last lease.
		f.replica.App.StartDrain()
		f.replica.Server.Close()
		f.clock.Advance(16 * time.Minute)
		expectTerminalClose(t, socket, 4402)
		f.clock.Advance(healthWait)
		testutil.Eventually(t, "lease expiry cloud diagnosis", 6*time.Second, func() bool {
			view := readControllerComputer(t, controller, id)
			return view.Status == string(computers.NeedsAttention) && view.Diagnosis != nil &&
				view.Diagnosis.Step == "cloud"
		})
	})
}

const healthWait = 3 * time.Second

func newControllerHost(t *testing.T, home string) (authFixture, *remoteHostLab) {
	t.Helper()
	f := newAuthFixtureWithReplica(t, remotetest.StartLocalReplica)
	return f, newRemoteHostLab(t, f, home)
}

func controllerCloudDevice(t *testing.T, service *account.Service) remoteTestDevice {
	t.Helper()
	status := service.Status()
	device := remoteTestDevice{id: status.Profile.Device.ID, account: status.AccountID}
	err := service.UseDevice(t.Context(), func(_ context.Context, credentials cloudclient.Credentials) error {
		device.key, device.access = credentials.Key, credentials.AccessToken
		return nil
	})
	if err != nil || device.access == "" {
		t.Fatalf("read test device credentials: %v", err)
	}
	return device
}

func openBridgeTerminal(t *testing.T, controller daemonAccountFixture, id remoteproto.ComputerID) *websocket.Conn {
	t.Helper()
	target := "ws" + strings.TrimPrefix(controller.httpBase, "http") + bridgeAPI(id, remoteFixtureStream)
	dialer := &websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	socket, response, err := dialer.DialContext(t.Context(), target, nil)
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		t.Fatalf("upgrade the controller bridge: %v", err)
	}
	t.Cleanup(func() { _ = socket.Close() })
	if err := socket.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, message, err := socket.ReadMessage(); err != nil || string(message) != "ready" {
		t.Fatalf("bridge stream did not open: %v", err)
	}
	return socket
}

const (
	transferPath  = "/api/controller-transfer-fixture"
	transferCount = 50
)

func TestDaemonComputerBridgeTransfers(t *testing.T) {
	root := t.TempDir()
	t.Run("Should stream fifty simultaneous transfers within the yamux window", func(t *testing.T) {
		f, lab := newControllerHost(t, filepath.Join(root, "transfer"))
		payload := bytes.Repeat([]byte("streaming-fixture"), (1<<20)/len("streaming-fixture"))
		payload = append(payload, make([]byte, (1<<20)-len(payload))...)
		entered, consume := make(chan struct{}, transferCount), make(chan struct{})
		release := sync.OnceFunc(func() { close(consume) })
		defer release()
		lab.startHost(t, lab.db, transferFixture(entered, consume, sha256.Sum256(payload)), directRemoteFixture())
		id := lab.host.Status().ComputerID
		controller, _ := startComputerController(
			t, f, filepath.Join(root, "transfer-controller"), remoteFixtureEmail, nil,
		)
		addAccountComputer(t, controller, id)
		waitComputerRoute(t, controller, id, remoteproto.RouteDirect)

		// IT-039. Safety Invariant 1. The production tunnel uses binary WS frames with an inner TLS session.
		before := liveHeap()
		results := make(chan error, transferCount)
		var workers sync.WaitGroup
		for range transferCount {
			workers.Go(func() {
				results <- transferThroughBridge(t.Context(), controller, id, payload)
			})
		}
		t.Cleanup(workers.Wait)
		ctx, cancel := context.WithTimeout(t.Context(), 7*time.Second)
		defer cancel()
		for range transferCount {
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("the bridge did not open every independent stream")
			}
		}
		// Allow one receive window at either endpoint and runtime overhead, below a body per stream.
		during := liveHeap()
		const limit = 2*transferCount*(256<<10) + 20<<20
		release()
		if during > before+limit {
			t.Errorf("held streams allocated %d bytes above baseline, budget %d", during-before, limit)
		}
		for range transferCount {
			if err := <-results; err != nil {
				t.Errorf("transfer integrity: %v", err)
			}
		}
	})
}

func transferFixture(entered chan<- struct{}, consume <-chan struct{}, expected [32]byte) http.Handler {
	return httpapi.NewRemoteRoutes(func(routes *httpapi.RemoteRoutes) {
		routes.Group(transferPath, remoteproto.ScopeFilesW).POST("", func(c *gin.Context) {
			entered <- struct{}{}
			select {
			case <-consume:
			case <-c.Request.Context().Done():
				return
			}
			hash := sha256.New()
			n, err := io.Copy(hash, c.Request.Body)
			if err != nil || n != 1<<20 || !bytes.Equal(expected[:], hash.Sum(nil)) {
				c.JSON(http.StatusBadRequest, gin.H{"code": "bad_transfer"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"sha256": fmt.Sprintf("%x", expected)})
		})
	})
}

func liveHeap() uint64 {
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.HeapAlloc
}

func transferThroughBridge(
	ctx context.Context, controller daemonAccountFixture, id remoteproto.ComputerID, payload []byte,
) error {
	target := controller.httpBase + bridgeAPI(id, transferPath)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	response, err := controller.httpClient.Do(request)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024))
	if err = errors.Join(err, response.Body.Close()); err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, fmt.Appendf(nil, "%x", sha256.Sum256(payload))) {
		return fmt.Errorf("unexpected transfer response HTTP %d: %s", response.StatusCode, body)
	}
	return nil
}
