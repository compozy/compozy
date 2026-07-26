package daytona

import (
	"bytes"
	"context"

	"errors"
	"fmt"

	"net"

	"time"

	"golang.org/x/crypto/ssh"
)

type sshDialer func(
	ctx context.Context,
	network string,
	address string,
	config *ssh.ClientConfig,
) (*ssh.Client, error)

func defaultSSHDialer(
	ctx context.Context,
	network string,
	address string,
	config *ssh.ClientConfig,
) (*ssh.Client, error) {
	dialer := net.Dialer{Timeout: defaultSSHDialTimeout}
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, err
	}
	return ssh.NewClient(clientConn, chans, reqs), nil
}

type sshTransport struct {
	tokens          *sshTokenManager
	host            string
	port            string
	dial            sshDialer
	hostKeyCallback ssh.HostKeyCallback
	keepAlive       time.Duration
	now             func() time.Time
}

type sshClientDialer interface {
	DialClient(ctx context.Context, sandbox sandboxInfo) (*ssh.Client, error)
}

func newSSHTransport(tokens *sshTokenManager, opts ...func(*sshTransport)) *sshTransport {
	transport := &sshTransport{
		tokens:          tokens,
		host:            defaultSSHHost,
		port:            defaultSSHPort,
		dial:            defaultSSHDialer,
		hostKeyCallback: defaultHostKeyCallback(),
		keepAlive:       defaultSSHKeepAlive,
		now:             time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(transport)
		}
	}
	if transport.host == "" {
		transport.host = defaultSSHHost
	}
	if transport.port == "" {
		transport.port = defaultSSHPort
	}
	if transport.dial == nil {
		transport.dial = defaultSSHDialer
	}
	if transport.hostKeyCallback == nil {
		transport.hostKeyCallback = defaultHostKeyCallback()
	}
	if transport.keepAlive <= 0 {
		transport.keepAlive = defaultSSHKeepAlive
	}
	return transport
}

func (t *sshTransport) Dial(
	ctx context.Context,
	sandbox sandboxInfo,
	command string,
) (transportSession, error) {
	session, err := t.dialWithFreshness(ctx, sandbox, command, false)
	if err == nil {
		return session, nil
	}
	retry, retryErr := t.dialWithFreshness(ctx, sandbox, command, true)
	if retryErr != nil {
		return nil, errors.Join(err, retryErr)
	}
	return retry, nil
}

func (t *sshTransport) DialClient(ctx context.Context, sandbox sandboxInfo) (*ssh.Client, error) {
	client, err := t.dialClientWithFreshness(ctx, sandbox, false)
	if err == nil {
		return client, nil
	}
	retry, retryErr := t.dialClientWithFreshness(ctx, sandbox, true)
	if retryErr != nil {
		return nil, errors.Join(err, retryErr)
	}
	return retry, nil
}

func (t *sshTransport) dialWithFreshness(
	ctx context.Context,
	sandbox sandboxInfo,
	command string,
	forceToken bool,
) (transportSession, error) {
	client, err := t.dialClientWithFreshness(ctx, sandbox, forceToken)
	if err != nil {
		return nil, err
	}
	session, err := client.NewSession()
	if err != nil {
		if closeErr := client.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, fmt.Errorf("sandbox/daytona: create SSH session for sandbox %q: %w", sandbox.ID, err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf(
			"sandbox/daytona: open SSH stdin for sandbox %q: %w",
			sandbox.ID,
			errors.Join(err, closeSSH(client, session)),
		)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf(
			"sandbox/daytona: open SSH stdout for sandbox %q: %w",
			sandbox.ID,
			errors.Join(err, closeSSH(client, session)),
		)
	}
	var stderr bytes.Buffer
	session.Stderr = &stderr
	if err := session.Start(command); err != nil {
		return nil, fmt.Errorf(
			"sandbox/daytona: start SSH command in sandbox %q: %w",
			sandbox.ID,
			errors.Join(err, closeSSH(client, session)),
		)
	}
	return newSSHSession(client, session, stdin, stdout, &stderr, t.keepAlive), nil
}

func (t *sshTransport) dialClientWithFreshness(
	ctx context.Context,
	sandbox sandboxInfo,
	forceToken bool,
) (*ssh.Client, error) {
	access, err := t.tokens.Ensure(ctx, sandbox.APIURL, sandbox.ID, forceToken)
	if err != nil {
		return nil, fmt.Errorf("sandbox/daytona: ensure SSH access for sandbox %q: %w", sandbox.ID, err)
	}
	config := &ssh.ClientConfig{
		User:            access.Token,
		Auth:            []ssh.AuthMethod{ssh.Password("")},
		HostKeyCallback: t.hostKeyCallback,
		Timeout:         defaultSSHDialTimeout,
	}
	address := net.JoinHostPort(normalizeSSHHost(firstNonEmpty(sandbox.SSHHost, t.host)), t.port)
	client, err := t.dial(ctx, "tcp", address, config)
	if err != nil {
		return nil, fmt.Errorf("sandbox/daytona: dial SSH sandbox %q: %w", sandbox.ID, err)
	}
	return client, nil
}

func closeSSH(client *ssh.Client, session *ssh.Session) error {
	var err error
	if session != nil {
		err = errors.Join(err, session.Close())
	}
	if client != nil {
		err = errors.Join(err, client.Close())
	}
	return err
}
