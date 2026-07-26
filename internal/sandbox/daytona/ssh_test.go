package daytona

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"net"
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
	host string
	port string
}

func newTestSSHServer(t *testing.T, validUser string) testSSHServer {
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
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Logf("listener.Close() error = %v", err)
		}
	})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleTestSSHConn(conn, config)
		}
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	return testSSHServer{host: host, port: port}
}

func handleTestSSHConn(conn net.Conn, config *ssh.ServerConfig) {
	server, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer server.Close()
	go ssh.DiscardRequests(reqs)
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go handleTestSSHSession(channel, requests)
	}
}

func handleTestSSHSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()
	for req := range requests {
		switch req.Type {
		case "exec":
			_ = req.Reply(true, nil)
			if bytes.Contains(req.Payload, []byte("cat")) {
				_, _ = io.Copy(channel, channel)
			}
			_, _ = channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
			return
		default:
			_ = req.Reply(false, nil)
		}
	}
}
