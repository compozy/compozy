//go:build windows

package mcp

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsPeerIdentity(t *testing.T) {
	if socket := os.Getenv("COMPOZY_PEER_TEST_SOCKET"); socket != "" {
		t.Run("Should connect from a separate Windows process", func(t *testing.T) {
			t.Parallel()
			conn, err := (&net.Dialer{}).DialContext(t.Context(), "unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := conn.Close(); err != nil {
					t.Error(err)
				}
			})
			if _, err := io.Copy(io.Discard, conn); err != nil {
				t.Fatal(err)
			}
		})
		return
	}

	t.Run("Should bind only the socket peer user and preserve bind identity", func(t *testing.T) {
		t.Parallel()
		peer := windowsSubprocessPeer(t)
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
		service := newHostedTestService(t, executable, &hostedRegistryStub{}, func() time.Time { return now })
		launch, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: "sess-windows-peer"})
		if err != nil {
			t.Fatal(err)
		}
		if err := service.ArmLaunch(t.Context(), "sess-windows-peer"); err != nil {
			t.Fatal(err)
		}
		request := HostedBindRequest{SessionID: "sess-windows-peer", Nonce: hostedLaunchNonce(t, launch.Args)}
		for _, sid := range []string{"", "invalid-sid", "S-1-0-0"} {
			foreignPeer := peer
			foreignPeer.UserSID = sid
			if _, err := service.Bind(t.Context(), request, foreignPeer); !errors.Is(err, ErrHostedPeerInvalid) {
				t.Fatalf("Bind(peer SID %q) error = %v, want ErrHostedPeerInvalid", sid, err)
			}
		}
		wrongBinary := peer
		wrongBinary.ExecutablePath = hostedTestExecutable(t, "other.exe")
		if _, err := service.Bind(t.Context(), request, wrongBinary); !errors.Is(err, ErrHostedBinaryInvalid) {
			t.Fatalf("Bind(wrong executable) error = %v, want ErrHostedBinaryInvalid", err)
		}
		bind, err := service.Bind(t.Context(), request, peer)
		if err != nil {
			t.Fatalf("Bind(socket peer) error = %v", err)
		}
		if _, err := service.Projection(t.Context(), bind.BindID, peer); err != nil {
			t.Fatalf("Projection(socket peer) error = %v", err)
		}
		changed := peer
		changed.UserSID = "S-1-0-0"
		if _, err := service.Projection(t.Context(), bind.BindID, changed); !errors.Is(err, ErrHostedPeerInvalid) {
			t.Fatalf("Projection(changed SID) error = %v, want ErrHostedPeerInvalid", err)
		}
		changed = peer
		changed.PID++
		if err := service.ReleaseBindForPeer(t.Context(), bind.BindID, changed); !errors.Is(err, ErrHostedPeerInvalid) {
			t.Fatalf("ReleaseBindForPeer(changed PID) error = %v, want ErrHostedPeerInvalid", err)
		}
		if err := service.ReleaseBindForPeer(t.Context(), bind.BindID, peer); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Should validate successful and failed Windows peer PID responses", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name          string
			pid, returned uint32
			sysErr        error
			wantErr       bool
		}{
			{name: "Should accept a complete PID response", pid: 42, returned: 4},
			{name: "Should accept a valid PID with the Windows zero length defect", pid: 42},
			{name: "Should reject a missing PID despite zero length success", wantErr: true},
			{name: "Should reject a missing PID despite complete length", returned: 4, wantErr: true},
			{name: "Should reject a truncated PID response", pid: 42, returned: 2, wantErr: true},
			{name: "Should reject a failed ioctl despite a nonzero PID", pid: 42,
				sysErr: windows.WSAEOPNOTSUPP, wantErr: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				pid, err := validateWindowsPeerPID(tc.pid, tc.returned, tc.sysErr)
				if (err != nil) != tc.wantErr || (!tc.wantErr && pid != tc.pid) {
					t.Fatalf("validateWindowsPeerPID() = %d, %v; wantErr=%t", pid, err, tc.wantErr)
				}
				if tc.sysErr != nil && !errors.Is(err, ErrWindowsPeerCredentialsUnsupported) {
					t.Fatalf("ioctl failure = %v, want actionable unsupported error", err)
				}
				if tc.wantErr && tc.sysErr == nil &&
					(errors.Is(err, ErrWindowsPeerCredentialsUnsupported) ||
						!strings.Contains(err.Error(), "invalid Windows AF_UNIX peer pid response")) {
					t.Fatalf("validateWindowsPeerPID() error = %v, want invalid peer PID response", err)
				}
			})
		}
	})

	t.Run("Should fail closed when the peer process cannot be inspected", func(t *testing.T) {
		t.Parallel()
		if _, err := windowsProcessPeerInfo(0); !errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			t.Fatalf("windowsProcessPeerInfo(0) error = %v, want invalid process parameter", err)
		}
		if _, err := windowsSocketPeerPID(windows.InvalidHandle); !errors.Is(err, windows.WSAENOTSOCK) ||
			errors.Is(err, ErrWindowsPeerCredentialsUnsupported) {
			t.Fatalf("windowsSocketPeerPID(invalid handle) error = %v, want ordinary invalid socket failure", err)
		}
	})
}

func windowsSubprocessPeer(t *testing.T) PeerInfo {
	t.Helper()
	// Keep the AF_UNIX path below the Windows sockaddr_un limit.
	dir, err := os.MkdirTemp("", "cp-peer-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	socket := filepath.Join(dir, "p.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := listener.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	type result struct {
		peer PeerInfo
		err  error
	}
	results := make(chan result, 1)
	go func() {
		conn, err := listener.AcceptUnix()
		if err != nil {
			results <- result{err: err}
			return
		}
		peer, err := PeerInfoFromConn(conn)
		results <- result{peer: peer, err: errors.Join(err, conn.Close())}
	}()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestWindowsPeerIdentity$")
	cmd.Env = append(os.Environ(), "COMPOZY_PEER_TEST_SOCKET="+socket)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("peer subprocess: %v: %s", err, output)
	}
	got := <-results
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.peer.PID != cmd.Process.Pid || got.peer.PID == os.Getpid() || !got.peer.Supported || got.peer.UserSID == "" {
		t.Fatalf("socket peer = %#v, want child process identity", got.peer)
	}
	if err := validatePeerUser(got.peer); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(got.peer.ExecutablePath, executable) {
		t.Fatalf("peer executable = %q, want %q", got.peer.ExecutablePath, executable)
	}
	return got.peer
}
