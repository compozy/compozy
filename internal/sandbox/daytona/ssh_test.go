package daytona

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestSSHTransportDialConnectsAndStreams(t *testing.T) {
	t.Parallel()

	server := newTestSSHServer(t, "valid-token")
	tokenSource := &fakeTokenSource{access: []sshAccess{{
		Token:     "valid-token",
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}}}
	transport := newSSHTransport(
		newSSHTokenManager(tokenSource, time.Now),
		func(t *sshTransport) {
			t.host = server.host
			t.port = server.port
			t.hostKeyCallback = ssh.InsecureIgnoreHostKey()
			t.keepAlive = time.Hour
		},
	)

	session, err := transport.Dial(context.Background(), sandboxInfo{ID: "sandbox", APIURL: defaultAPIURL}, "cat")
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	if _, err := session.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := session.CloseWrite(); err != nil {
		t.Fatalf("CloseWrite() error = %v", err)
	}
	output, err := io.ReadAll(session)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if got, want := string(output), "hello"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	if err := session.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	select {
	case <-session.Done():
	default:
		t.Fatal("Done() was not closed after Wait()")
	}
	if stderr := session.Stderr(); stderr != "" {
		t.Fatalf("Stderr() = %q, want empty", stderr)
	}
	if err := session.Stop(context.Background()); err != nil {
		t.Logf("Stop() after wait returned expected closed-session error: %v", err)
	}
}

func TestSSHTransportDialRetainsRemoteExitCode(t *testing.T) {
	t.Parallel()
	t.Run("Should retain the remote exit status after Wait", func(t *testing.T) {
		t.Parallel()
		testSSHTransportDialRetainsRemoteExitCode(t)
	})
}

func testSSHTransportDialRetainsRemoteExitCode(t *testing.T) {
	t.Helper()

	server := newTestSSHServer(t, "valid-token")
	tokenSource := &fakeTokenSource{access: []sshAccess{{
		Token:     "valid-token",
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}}}
	transport := newSSHTransport(
		newSSHTokenManager(tokenSource, time.Now),
		func(transport *sshTransport) {
			transport.host = server.host
			transport.port = server.port
			transport.hostKeyCallback = ssh.InsecureIgnoreHostKey()
			transport.keepAlive = time.Hour
		},
	)

	session, err := transport.Dial(
		t.Context(),
		sandboxInfo{ID: "sandbox", APIURL: defaultAPIURL},
		"exit 23",
	)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	if _, err := io.ReadAll(session); err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if err := session.Wait(); err == nil {
		t.Fatal("Wait() error = nil, want nonzero exit error")
	}
	if got, ok := session.ExitCode(); !ok || got != 23 {
		t.Fatalf("ExitCode() = %d, %v, want 23, true", got, ok)
	}
}

func TestSSHTransportDialRetriesWithFreshTokenAfterAuthFailure(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	server := newTestSSHServer(t, "fresh-token")
	tokenSource := &fakeTokenSource{access: []sshAccess{
		{Token: "expired-token", IssuedAt: now, ExpiresAt: now.Add(time.Hour)},
		{Token: "fresh-token", IssuedAt: now, ExpiresAt: now.Add(time.Hour)},
	}}
	transport := newSSHTransport(
		newSSHTokenManager(tokenSource, func() time.Time { return now }),
		func(t *sshTransport) {
			t.host = server.host
			t.port = server.port
			t.hostKeyCallback = ssh.InsecureIgnoreHostKey()
			t.keepAlive = time.Hour
		},
	)

	session, err := transport.Dial(context.Background(), sandboxInfo{ID: "sandbox", APIURL: defaultAPIURL}, "cat")
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got, want := tokenSource.calls, 2; got != want {
		t.Fatalf("FetchSSHAccess calls = %d, want %d", got, want)
	}
}

func TestSSHTransportDialFailsWithInvalidToken(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	server := newTestSSHServer(t, "valid-token")
	tokenSource := &fakeTokenSource{access: []sshAccess{{
		Token:     "invalid-token",
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}}}
	transport := newSSHTransport(
		newSSHTokenManager(tokenSource, func() time.Time { return now }),
		func(t *sshTransport) {
			t.host = server.host
			t.port = server.port
			t.hostKeyCallback = ssh.InsecureIgnoreHostKey()
			t.keepAlive = time.Hour
		},
	)

	if _, err := transport.Dial(
		context.Background(),
		sandboxInfo{ID: "sandbox", APIURL: defaultAPIURL},
		"cat",
	); err == nil {
		t.Fatal("Dial() error = nil, want invalid token error")
	}
}

func TestSSHTokenManagerRefreshesAtHalfExpiry(t *testing.T) {
	t.Parallel()

	issued := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	now := issued
	source := &fakeTokenSource{access: []sshAccess{
		{Token: "first", IssuedAt: issued, ExpiresAt: issued.Add(time.Hour)},
		{Token: "second", IssuedAt: issued.Add(31 * time.Minute), ExpiresAt: issued.Add(91 * time.Minute)},
	}}
	manager := newSSHTokenManager(source, func() time.Time { return now })

	first, err := manager.Ensure(context.Background(), defaultAPIURL, "sandbox", false)
	if err != nil {
		t.Fatalf("Ensure(first) error = %v", err)
	}
	now = issued.Add(29 * time.Minute)
	cached, err := manager.Ensure(context.Background(), defaultAPIURL, "sandbox", false)
	if err != nil {
		t.Fatalf("Ensure(cached) error = %v", err)
	}
	now = issued.Add(31 * time.Minute)
	refreshed, err := manager.Ensure(context.Background(), defaultAPIURL, "sandbox", false)
	if err != nil {
		t.Fatalf("Ensure(refreshed) error = %v", err)
	}
	if first.Token != "first" || cached.Token != "first" || refreshed.Token != "second" {
		t.Fatalf("tokens = %q/%q/%q, want first/first/second", first.Token, cached.Token, refreshed.Token)
	}
	if got, want := source.calls, 2; got != want {
		t.Fatalf("FetchSSHAccess calls = %d, want %d", got, want)
	}
}

func TestDefaultHostKeyCallbackRequiresKnownHosts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	callback := defaultHostKeyCallback()
	if err := callback("example.com", nil, nil); err == nil {
		t.Fatal("defaultHostKeyCallback() error = nil, want missing known_hosts error")
	}
}

type testSSHServer struct {
	host        string
	port        string
	listener    net.Listener
	workers     sync.WaitGroup
	mu          sync.Mutex
	connections map[net.Conn]struct{}
	errs        []error
}

func newTestSSHServer(t *testing.T, validUser string, forwarding ...func(ssh.NewChannel) error) *testSSHServer {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("NewSignerFromKey() error = %v", err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(meta ssh.ConnMetadata, _ []byte) (*ssh.Permissions, error) {
			if meta.User() == validUser {
				return nil, nil
			}
			return nil, errors.New("invalid user")
		},
	}
	config.AddHostKey(signer)

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	server := &testSSHServer{
		host:        host,
		port:        port,
		listener:    listener,
		connections: make(map[net.Conn]struct{}),
	}
	server.workers.Go(func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					server.recordError(fmt.Errorf("accept test SSH connection: %w", err))
				}
				return
			}
			server.trackConnection(conn)
			server.workers.Go(func() {
				defer server.untrackConnection(conn)
				if serveErr := handleTestSSHConn(
					conn,
					config,
					forwarding,
				); serveErr != nil &&
					!errors.Is(serveErr, net.ErrClosed) {
					server.recordError(serveErr)
				}
			})
		}
	})
	t.Cleanup(func() {
		if closeErr := listener.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			server.recordError(fmt.Errorf("close test SSH listener: %w", closeErr))
		}
		server.closeConnections()
		server.workers.Wait()
		for _, serveErr := range server.errors() {
			t.Errorf("test SSH server error = %v", serveErr)
		}
	})
	return server
}

func (s *testSSHServer) trackConnection(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[conn] = struct{}{}
}

func (s *testSSHServer) untrackConnection(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connections, conn)
}

func (s *testSSHServer) closeConnections() {
	s.mu.Lock()
	connections := make([]net.Conn, 0, len(s.connections))
	for conn := range s.connections {
		connections = append(connections, conn)
	}
	s.mu.Unlock()
	for _, conn := range connections {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			s.recordError(fmt.Errorf("close test SSH connection: %w", err))
		}
	}
}

func (s *testSSHServer) recordError(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs = append(s.errs, err)
}

func (s *testSSHServer) errors() []error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]error(nil), s.errs...)
}

func handleTestSSHConn(conn net.Conn, config *ssh.ServerConfig, forwarding []func(ssh.NewChannel) error) (err error) {
	server, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return nil
	}
	defer func() {
		err = joinTestSSHCloseError(err, server.Close())
	}()
	go ssh.DiscardRequests(reqs)
	for newChannel := range chans {
		if newChannel.ChannelType() == "direct-tcpip" && len(forwarding) != 0 {
			if err := forwarding[0](newChannel); err != nil {
				return err
			}
			continue
		}
		if newChannel.ChannelType() != "session" {
			if rejectErr := newChannel.Reject(ssh.UnknownChannelType, "unsupported"); rejectErr != nil {
				return fmt.Errorf("reject unsupported SSH channel: %w", rejectErr)
			}
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			return fmt.Errorf("accept SSH session channel: %w", err)
		}
		if err := handleTestSSHSession(channel, requests); err != nil {
			return err
		}
	}
	return nil
}

func handleTestSSHSession(channel ssh.Channel, requests <-chan *ssh.Request) (err error) {
	defer func() {
		err = joinTestSSHCloseError(err, channel.Close())
	}()
	for req := range requests {
		switch req.Type {
		case "exec":
			if err := req.Reply(true, nil); err != nil {
				return fmt.Errorf("reply to SSH exec request: %w", err)
			}
			if bytes.Contains(req.Payload, []byte("cat")) {
				if _, err := io.Copy(channel, channel); err != nil {
					return fmt.Errorf("echo SSH session stream: %w", err)
				}
			}
			exitCode := byte(0)
			if bytes.Contains(req.Payload, []byte("exit 23")) {
				exitCode = 23
			}
			if _, err := channel.SendRequest("exit-status", false, []byte{0, 0, 0, exitCode}); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return fmt.Errorf("send SSH exit status: %w", err)
			}
			return nil
		default:
			if err := req.Reply(false, nil); err != nil {
				return fmt.Errorf("reject unsupported SSH request: %w", err)
			}
		}
	}
	return nil
}

func joinTestSSHCloseError(err error, closeErr error) error {
	if closeErr == nil || errors.Is(closeErr, io.EOF) || errors.Is(closeErr, net.ErrClosed) {
		return err
	}
	return errors.Join(err, closeErr)
}
