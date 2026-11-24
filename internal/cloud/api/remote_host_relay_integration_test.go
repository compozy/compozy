//go:build integration

package api_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

type remoteControlRelay struct {
	mu          sync.Mutex
	operator    http.Handler
	upstream    string
	offline     bool
	connections map[*remoteRelayConnection]struct{}
	filter      func(remoteproto.ControlMessage) []remoteproto.ControlMessage
	observe     func(remoteproto.ControlMessage)
}

type remoteRelayConnection struct{ host, cloud *websocket.Conn }

func remoteFailoverFixture(t *testing.T) (authFixture, *remotetest.Replica, *remotetest.Replica, *remoteControlRelay) {
	t.Helper()
	relay := &remoteControlRelay{connections: make(map[*remoteRelayConnection]struct{})}
	edge := httptest.NewServer(relay)
	t.Cleanup(edge.Close)
	var first, second *remotetest.Replica
	f := newAuthFixtureWithReplica(
		t,
		func(t *testing.T, config cloud.Config, register func(*cloud.App)) *remotetest.Replica {
			t.Helper()
			config.Issuer = edge.URL
			first = remotetest.StartReplica(t, config, register)
			config.Postgres.PooledDSN = appendApplicationName(t, config.Postgres.PooledDSN, "remote-replica2-pool")
			config.Postgres.DirectDSN = appendApplicationName(t, config.Postgres.DirectDSN, "remote-replica2-direct")
			second = remotetest.StartReplica(t, config, register)
			target, err := url.Parse(first.Server.URL)
			if err != nil {
				t.Fatal(err)
			}
			relay.operator = httputil.NewSingleHostReverseProxy(target)
			relay.move(first.Server.URL)
			return &remotetest.Replica{App: first.App, Server: edge}
		},
	)
	t.Cleanup(func() { relay.move("") })
	return f, first, second, relay
}

func appendApplicationName(t *testing.T, raw, name string) string {
	t.Helper()
	target, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	query := target.Query()
	query.Set("application_name", name)
	target.RawQuery = query.Encode()
	return target.String()
}

func (r *remoteControlRelay) move(target string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.upstream, r.offline = target, target == ""
	for connection := range r.connections {
		connection.close()
	}
}

func (r *remoteControlRelay) rules(
	filter func(remoteproto.ControlMessage) []remoteproto.ControlMessage,
	observe func(remoteproto.ControlMessage),
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.filter, r.observe = filter, observe
}

func (r *remoteControlRelay) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/v1/computers/control" {
		r.mu.Lock()
		handler := r.operator
		r.mu.Unlock()
		if handler == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		handler.ServeHTTP(w, request)
		return
	}
	cloud, host, ok := r.connect(w, request)
	if !ok {
		return
	}
	connection := &remoteRelayConnection{host: host, cloud: cloud}
	if !r.track(connection) {
		return
	}
	defer func() {
		connection.close()
		r.mu.Lock()
		delete(r.connections, connection)
		r.mu.Unlock()
	}()
	var reports sync.WaitGroup
	reports.Go(func() {
		defer connection.close()
		r.forwardReports(connection)
	})
	r.forwardUpdates(connection)
	connection.close()
	reports.Wait()
}

func (r *remoteControlRelay) connect(
	w http.ResponseWriter,
	request *http.Request,
) (*websocket.Conn, *websocket.Conn, bool) {
	r.mu.Lock()
	target, offline := r.upstream, r.offline
	r.mu.Unlock()
	if offline {
		w.WriteHeader(http.StatusServiceUnavailable)
		return nil, nil, false
	}
	upstream, err := url.Parse(target)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return nil, nil, false
	}
	upstream.Scheme, upstream.Path, upstream.RawQuery = "ws", request.URL.Path, request.URL.RawQuery
	dialer := &websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	cloud, response, err := dialer.DialContext(request.Context(), upstream.String(), nil)
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		w.WriteHeader(http.StatusBadGateway)
		return nil, nil, false
	}
	upgrader := websocket.Upgrader{HandshakeTimeout: 3 * time.Second}
	host, err := upgrader.Upgrade(w, request, nil)
	if err != nil {
		_ = cloud.Close()
		return nil, nil, false
	}
	return cloud, host, true
}

func (r *remoteControlRelay) track(c *remoteRelayConnection) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.offline {
		c.close()
		return false
	}
	r.connections[c] = struct{}{}
	return true
}

func (r *remoteControlRelay) forwardReports(c *remoteRelayConnection) {
	for {
		kind, frame, err := c.host.ReadMessage()
		if err != nil {
			return
		}
		r.mu.Lock()
		observe := r.observe
		r.mu.Unlock()
		if observe != nil {
			var message remoteproto.ControlMessage
			if json.Unmarshal(frame, &message) == nil {
				observe(message)
			}
		}
		if err := relayWrite(c.cloud, kind, frame); err != nil {
			return
		}
	}
}

func (r *remoteControlRelay) forwardUpdates(c *remoteRelayConnection) {
	for {
		kind, frame, err := c.cloud.ReadMessage()
		if err != nil {
			return
		}
		r.mu.Lock()
		filter := r.filter
		r.mu.Unlock()
		if filter == nil {
			if err := relayWrite(c.host, kind, frame); err != nil {
				return
			}
			continue
		}
		var message remoteproto.ControlMessage
		if json.Unmarshal(frame, &message) != nil {
			return
		}
		for _, output := range filter(message) {
			data, err := json.Marshal(output)
			if err != nil || relayWrite(c.host, kind, data) != nil {
				return
			}
		}
	}
}

func (c *remoteRelayConnection) close() {
	_ = c.host.Close()
	_ = c.cloud.Close()
}

func relayWrite(target *websocket.Conn, kind int, data []byte) error {
	if err := target.SetWriteDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}
	return target.WriteMessage(kind, data)
}

func receiveRemote[T any](t *testing.T, messages <-chan T, timeout time.Duration) T {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	select {
	case message := <-messages:
		return message
	case <-ctx.Done():
		t.Fatalf("remote fixture did not receive the expected message: %v", ctx.Err())
		var zero T
		return zero
	}
}
