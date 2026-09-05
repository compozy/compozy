package daytona

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

func TestSidecarSessionCleanupContract(t *testing.T) {
	t.Parallel()

	t.Run("Should close endpoint exactly once after Stop", func(t *testing.T) {
		t.Parallel()

		contract := newContractSidecarSession(t)
		if err := contract.session.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if err := contract.session.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("Stop(second) error = %v", err)
		}
		if got := contract.closeCount.Load(); got != 1 {
			t.Fatalf("endpoint close count = %d, want 1", got)
		}
		if got := contract.stopCount.Load(); got != 1 {
			t.Fatalf("remote stop request count = %d, want 1", got)
		}
	})

	t.Run("Should request remote stop on Close after the stream drops", func(t *testing.T) {
		t.Parallel()

		contract := newDroppedContractSidecarSession(t)
		<-contract.session.Done()
		if err := contract.session.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if got := contract.stopCount.Load(); got != 1 {
			t.Fatalf("remote stop request count = %d, want 1", got)
		}
		if err := contract.session.Close(); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
		if got := contract.stopCount.Load(); got != 1 {
			t.Fatalf("remote stop request count after second Close = %d, want 1", got)
		}
		if got := contract.closeCount.Load(); got != 1 {
			t.Fatalf("endpoint close count = %d, want 1", got)
		}
	})

	t.Run("Should close endpoint exactly once after Wait observes server exit", func(t *testing.T) {
		t.Parallel()

		contract := newContractSidecarSession(t)
		if err := contract.session.Wait(); err != nil {
			t.Fatalf("Wait() error = %v", err)
		}
		if err := contract.session.Wait(); err != nil {
			t.Fatalf("Wait(second) error = %v", err)
		}
		if got := contract.closeCount.Load(); got != 1 {
			t.Fatalf("endpoint close count = %d, want 1", got)
		}
	})

	t.Run("Should retain a nonzero server exit code after Wait", func(t *testing.T) {
		t.Parallel()

		contract := newContractSidecarSessionWithExit(t, 23)
		if err := contract.session.Wait(); err == nil {
			t.Fatal("Wait() error = nil, want nonzero exit error")
		}
		if got, ok := contract.session.ExitCode(); !ok || got != 23 {
			t.Fatalf("ExitCode() = %d, %v, want 23, true", got, ok)
		}
		if got := contract.closeCount.Load(); got != 1 {
			t.Fatalf("endpoint close count = %d, want 1", got)
		}
	})
}

type contractSidecarSession struct {
	session    *sidecarSession
	closeCount *atomic.Int32
	stopCount  *atomic.Int32
}

func newContractSidecarSession(t *testing.T) contractSidecarSession {
	t.Helper()
	return newContractSidecarSessionWithExit(t, 0)
}

func newContractSidecarSessionWithExit(t *testing.T, exitCode int) contractSidecarSession {
	t.Helper()

	var stopCount atomic.Int32
	return dialContractSidecarSession(t, newContractSidecarServer(t, exitCode, &stopCount), &stopCount)
}

func newDroppedContractSidecarSession(t *testing.T) contractSidecarSession {
	t.Helper()

	var stopCount atomic.Int32
	return dialContractSidecarSession(t, newDroppedStreamSidecarServer(t, &stopCount), &stopCount)
}

func dialContractSidecarSession(
	t *testing.T,
	server *httptest.Server,
	stopCount *atomic.Int32,
) contractSidecarSession {
	t.Helper()

	var closeCount atomic.Int32
	endpoint := newContractSidecarEndpoint(t, server, &closeCount)
	conn, response, err := websocket.DefaultDialer.Dial(
		endpoint.wsURL(sidecarSessionStreamBasePath, "session-1", "stream"),
		nil,
	)
	t.Cleanup(func() {
		if response != nil && response.Body != nil {
			if err := response.Body.Close(); err != nil {
				t.Errorf("websocket response body Close() error = %v", err)
			}
		}
	})
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	return contractSidecarSession{
		session:    newSidecarSession(conn, endpoint, "session-1", server.Client(), time.Second),
		closeCount: &closeCount,
		stopCount:  stopCount,
	}
}

func TestSidecarTransportDialCleanupContract(t *testing.T) {
	t.Parallel()
	t.Run("Should route recovery to its persisted sidecar and reuse a healthy current binary", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name, version string
			port          uint32
		}{
			{"old-process", "compozy-daytona-launcher-sidecar-v1", 40241},
			{"new-process", launcherSidecarVersion, 40242},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				server := newTestSSHServer(t, "valid-token", func(channel ssh.NewChannel) error {
					return serveSidecarTunnelContract(channel, tc.port)
				})
				source := &fakeTokenSource{access: []sshAccess{{
					Token: "valid-token", ExpiresAt: time.Now().Add(time.Hour),
				}}}
				bootstrap := newSSHTransport(newSSHTokenManager(source, time.Now), func(s *sshTransport) {
					s.host, s.port = server.host, server.port
					s.hostKeyCallback = ssh.InsecureIgnoreHostKey()
				})
				transport := &sidecarTransport{clientDialer: bootstrap}
				info := sandboxInfo{ID: "instance", LauncherProcessID: "reserved-process-identity",
					LauncherSidecarVersion: tc.version}
				if tc.version == launcherSidecarVersion {
					// No SDK or binary bootstrap is configured: healthy reuse must require neither.
					endpoint, err := transport.ensureSidecar(t.Context(), info)
					if err != nil {
						t.Fatalf("reuse healthy sidecar: %v", err)
					}
					if err := endpoint.Close(); err != nil {
						t.Fatalf("close reused endpoint: %v", err)
					}
				}
				verified, err := transport.processExitVerified(t.Context(), info)
				if err != nil || !verified {
					t.Fatalf("version-bound recovery = %t, %v", verified, err)
				}
			})
		}
	})
	t.Run("Should accept only complete exit proof for the requested remote identity", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name     string
			body     string
			status   int
			verified bool
			wantErr  bool
		}{
			{"verified", `{"id":"reserved","exited":true,"exitVerified":true,"exitCode":0}`, 200, true, false},
			{"running", `{"id":"reserved","exited":false,"exitVerified":false}`, 200, false, false},
			{"unverified-group", `{"id":"reserved","exited":true,"exitCode":1}`, 200, false, false},
			{"missing-code", `{"id":"reserved","exited":true,"exitVerified":true}`, 200, false, false},
			{"wrong-identity", `{"id":"other","exited":true,"exitVerified":true,"exitCode":0}`, 200, false, true},
			{"missing-process", `session not found`, 404, false, true},
			{"invalid-response", `{`, 200, false, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				server := newContractDialServer(t, func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodGet || r.URL.Path != "/v1/sessions/reserved" {
						t.Errorf("unexpected recovery request: %s %s", r.Method, r.URL.Path)
					}
					w.WriteHeader(tc.status)
					writeContractSidecarResponse(t, w, tc.body)
				})
				var closed atomic.Int32
				endpoint := newContractSidecarEndpoint(t, server, &closed)
				transport := &sidecarTransport{httpClient: server.Client()}
				verified, err := transport.processExitAtEndpoint(t.Context(), endpoint, "reserved")
				if verified != tc.verified || (err != nil) != tc.wantErr {
					t.Fatalf("remote proof = %t, %v", verified, err)
				}
			})
		}
	})
	t.Run("Should preserve reserved identity without falling back to anonymous launch", func(t *testing.T) {
		t.Parallel()
		for _, mode := range []string{"supported", "mismatched", "old-sidecar"} {
			t.Run(mode, func(t *testing.T) {
				t.Parallel()
				const id = "reserved-process-identity"
				var anonymous atomic.Int32
				server := newContractDialServer(t, func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/v1/launch" {
						anonymous.Add(1)
					}
					if mode == "old-sidecar" || r.URL.Path != "/v1/launch/identified" {
						http.NotFound(w, r)
						return
					}
					var request sidecarLaunchRequest
					if err := json.NewDecoder(r.Body).
						Decode(&request); err != nil || request.ID != id ||
						request.Command != "echo ok" {
						t.Errorf("identified request = %+v, %v", request, err)
						http.Error(w, "bad identity", http.StatusBadRequest)
						return
					}
					responseID := id
					if mode == "mismatched" {
						responseID = "another-process-identity"
					}
					w.WriteHeader(http.StatusCreated)
					writeContractSidecarResponse(t, w, `{"id":"`+responseID+`"}`)
				})
				var closeCount atomic.Int32
				endpoint := newContractSidecarEndpoint(t, server, &closeCount)
				transport := &sidecarTransport{httpClient: server.Client()}
				got, err := transport.launch(t.Context(), endpoint, "echo ok", id)
				switch {
				case mode == "supported":
					if err != nil || got != id {
						t.Fatalf("reserved launch = %q, %v", got, err)
					}
				case err == nil || got != "":
					t.Fatalf("unsupported identity accepted = %q, %v", got, err)
				case mode == "old-sidecar" && !strings.Contains(err.Error(), "404"):
					t.Fatalf("old sidecar response lost: %v", err)
				}
				if anonymous.Load() != 0 {
					t.Fatal("reserved launch fell back to anonymous process creation")
				}
			})
		}
	})

	t.Run("Should close endpoint when launch fails", func(t *testing.T) {
		t.Parallel()

		var closeCount atomic.Int32
		server := newContractDialServer(t, func(writer http.ResponseWriter, request *http.Request) {
			if request.Method == http.MethodPost && request.URL.Path == "/v1/launch" {
				http.Error(writer, "launch failed", http.StatusInternalServerError)
				return
			}
			http.NotFound(writer, request)
		})
		endpoint := newContractSidecarEndpoint(t, server, &closeCount)
		transport := &sidecarTransport{httpClient: server.Client(), closeTimeout: time.Second}

		_, err := transport.dialEndpoint(testutil.Context(t), endpoint, "echo ok", "")
		if err == nil {
			t.Fatal("dialEndpoint(launch failure) error = nil, want non-nil")
		}
		if got := closeCount.Load(); got != 1 {
			t.Fatalf("endpoint close count = %d, want 1", got)
		}
	})

	t.Run("Should close endpoint when websocket connect fails", func(t *testing.T) {
		t.Parallel()

		var closeCount atomic.Int32
		server := newContractDialServer(t, func(writer http.ResponseWriter, request *http.Request) {
			switch {
			case request.Method == http.MethodPost && request.URL.Path == "/v1/launch":
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusCreated)
				writeContractSidecarResponse(t, writer, "{\"id\":\"session-1\"}")
			case request.Method == http.MethodGet && request.URL.Path == "/v1/sessions/session-1/stream":
				http.Error(writer, "websocket failed", http.StatusInternalServerError)
			default:
				http.NotFound(writer, request)
			}
		})
		endpoint := newContractSidecarEndpoint(t, server, &closeCount)
		transport := &sidecarTransport{httpClient: server.Client(), closeTimeout: time.Second}

		_, err := transport.dialEndpoint(testutil.Context(t), endpoint, "echo ok", "")
		if err == nil {
			t.Fatal("dialEndpoint(connect failure) error = nil, want non-nil")
		}
		if got := closeCount.Load(); got != 1 {
			t.Fatalf("endpoint close count = %d, want 1", got)
		}
	})
}

func newContractSidecarEndpoint(
	t *testing.T,
	server *httptest.Server,
	closeCount *atomic.Int32,
) sidecarEndpoint {
	t.Helper()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(server.URL) error = %v", err)
	}
	return sidecarEndpoint{
		base:       baseURL,
		httpClient: server.Client(),
		wsDialer:   websocket.DefaultDialer,
		closeFn: func() error {
			closeCount.Add(1)
			return nil
		},
	}
}

func newContractSidecarServer(t *testing.T, exitCode int, stopCount *atomic.Int32) *httptest.Server {
	t.Helper()

	upgrader := websocket.Upgrader{}
	return newContractDialServer(t, func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodDelete && request.URL.Path == "/v1/sessions/session-1":
			stopCount.Add(1)
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodGet && request.URL.Path == "/v1/sessions/session-1/stream":
			conn, err := upgrader.Upgrade(writer, request, nil)
			if err != nil {
				t.Errorf("websocket Upgrade() error = %v", err)
				return
			}
			payload, err := json.Marshal(sidecarExitPayload{ExitCode: exitCode})
			if err != nil {
				t.Errorf("json.Marshal(exit) error = %v", err)
				return
			}
			frame := append([]byte{sidecarFrameServerExit}, payload...)
			if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
				t.Errorf("conn.WriteMessage(exit) error = %v", err)
			}
			if err := conn.Close(); err != nil {
				t.Errorf("conn.Close() error = %v", err)
			}
		default:
			http.Error(writer, fmt.Sprintf("unexpected %s %s", request.Method, request.URL.Path), http.StatusNotFound)
		}
	})
}

// newDroppedStreamSidecarServer drops the websocket without an exit frame, the
// shape a mid-session network failure produces while the remote process keeps running.
func newDroppedStreamSidecarServer(t *testing.T, stopCount *atomic.Int32) *httptest.Server {
	t.Helper()

	upgrader := websocket.Upgrader{}
	return newContractDialServer(t, func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodDelete && request.URL.Path == "/v1/sessions/session-1":
			stopCount.Add(1)
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodGet && request.URL.Path == "/v1/sessions/session-1/stream":
			conn, err := upgrader.Upgrade(writer, request, nil)
			if err != nil {
				t.Errorf("websocket Upgrade() error = %v", err)
				return
			}
			if err := conn.Close(); err != nil {
				t.Errorf("conn.Close() error = %v", err)
			}
		default:
			http.Error(writer, fmt.Sprintf("unexpected %s %s", request.Method, request.URL.Path), http.StatusNotFound)
		}
	})
}

func newContractDialServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func writeContractSidecarResponse(t *testing.T, writer http.ResponseWriter, body string) {
	t.Helper()

	if _, err := writer.Write([]byte(body)); err != nil {
		t.Errorf("writer.Write() error = %v", err)
	}
}

func serveSidecarTunnelContract(channel ssh.NewChannel, wantPort uint32) (err error) {
	var target struct {
		Host       string
		Port       uint32
		OriginHost string
		OriginPort uint32
	}
	if err := ssh.Unmarshal(channel.ExtraData(), &target); err != nil {
		return err
	}
	if target.Host != "127.0.0.1" || target.Port != wantPort {
		return channel.Reject(ssh.ConnectionFailed, "wrong sidecar endpoint")
	}
	stream, requests, err := channel.Accept()
	if err != nil {
		return err
	}
	defer func() { err = joinTestSSHCloseError(err, stream.Close()) }()
	go ssh.DiscardRequests(requests)
	request, err := http.ReadRequest(bufio.NewReader(stream))
	if err != nil {
		return err
	}
	if err := request.Body.Close(); err != nil {
		return err
	}
	var body string
	switch request.URL.Path {
	case "/healthz":
		body = `{"ok":true,"version":"` + launcherSidecarVersion + `"}`
	case "/v1/sessions/reserved-process-identity":
		body = `{"id":"reserved-process-identity","exited":true,"exitVerified":true,"exitCode":0}`
	default:
		return fmt.Errorf("unexpected recovery path %s", request.URL.Path)
	}
	response := http.Response{
		StatusCode: http.StatusOK, ProtoMajor: 1, ProtoMinor: 1,
		Header: http.Header{"Content-Type": {"application/json"}},
		Body:   io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Close: true,
	}
	return response.Write(stream)
}
