package remotehost_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func TestRemoteHostListeners(t *testing.T) {
	t.Run("Should require a linked identity and close the listener when disabled", func(t *testing.T) {
		// UT-020. Safety Invariants 1 and 15.
		cloudHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/v1/computers/link":
				w.Header().Set("Content-Type", "application/json")
				if _, err := fmt.Fprint(w, `{"computer_id":"cmp_fixture","account_id":"acc_fixture"}`); err != nil {
					t.Error(err)
				}
			case "/v1/computers/cmp_fixture":
				w.WriteHeader(http.StatusOK)
			default:
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		}))
		t.Cleanup(cloudHTTP.Close)
		t.Setenv("COMPOZY_CLOUD_URL", cloudHTTP.URL)
		home := t.TempDir()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(home, "compozy.db"))
		remoteOK(t, err)
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Errorf("close database: %v", err)
			}
		})
		cloud, err := cloudclient.New()
		remoteOK(t, err)
		identity, err := account.New(t.Context(), db, cloud, home, account.WithKeyring(&remoteTestKeys{}))
		remoteOK(t, err)
		t.Cleanup(identity.Close)
		cfg := config.DefaultRemoteConfig()
		cfg.Listener = "127.0.0.1:0"
		host, err := remotehost.NewHost(t.Context(), db, cloud, identity, cfg, remotehost.HostOptions{})
		remoteOK(t, err)
		t.Cleanup(host.Close)
		_, err = host.Patch(t.Context(), remotehost.Patch{Enabled: new(true)})
		remoteErrorIs(t, err, remotehost.ErrNotLinked)
		link, err := host.Link(t.Context(), "test-link-token")
		remoteOK(t, err)
		remoteEqual(t, true, link.Enabled)
		status := host.Status()
		dialer := &net.Dialer{Timeout: time.Second}
		socket, err := dialer.DialContext(t.Context(), "tcp", status.Listener)
		remoteOK(t, err)
		remoteOK(t, socket.Close())
		disabled, err := host.Patch(t.Context(), remotehost.Patch{Enabled: new(false)})
		remoteOK(t, err)
		remoteEqual(t, false, disabled.Enabled)
		remoteEqual(t, link.ComputerID, disabled.ComputerID)
		if socket, err := dialer.DialContext(t.Context(), "tcp", status.Listener); err == nil {
			remoteOK(t, socket.Close())
			t.Fatal("the disabled tunnel listener accepted a connection")
		}
	})
}

type remoteTestKeys struct {
	mu    sync.Mutex
	store map[string]string
}

func (r *remoteTestKeys) Get(service, name string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.store[service+":"+name]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}

func (r *remoteTestKeys) Set(service, name, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.store == nil {
		r.store = make(map[string]string)
	}
	r.store[service+":"+name] = value
	return nil
}

func (r *remoteTestKeys) Delete(service, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, service+":"+name)
	return nil
}

func TestHostedConnectorDiagnosis(t *testing.T) {
	t.Parallel()
	t.Run("Should recognize blocked Cloudflare edge port across stderr writes (UT-070)", func(t *testing.T) {
		t.Parallel()
		var diagnostics cloudflaredDiagnostics
		for _, chunk := range []string{"unrelated error\nfailed to dial to edge with quic: ", "203.0.113.1:7844\n"} {
			n, err := diagnostics.Write([]byte(chunk))
			if n != len(chunk) || err != nil {
				t.Fatalf("stderr write = %d, %v", n, err)
			}
		}
		const want = "Hosted route can't reach Cloudflare (port 7844 blocked)"
		if got := diagnostics.diagnosis(); got != want {
			t.Fatalf("diagnosis = %q, want %q", got, want)
		}
	})
}
