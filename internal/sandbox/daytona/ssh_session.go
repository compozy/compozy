package daytona

import (
	"bytes"
	"context"

	"errors"
	"fmt"
	"io"

	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type sshSession struct {
	client    *ssh.Client
	session   *ssh.Session
	stdin     io.WriteCloser
	stdout    io.Reader
	stderr    *bytes.Buffer
	closeOnce sync.Once
	done      chan struct{}
	waitErr   error
	cancel    context.CancelFunc
}

func newSSHSession(
	client *ssh.Client,
	session *ssh.Session,
	stdin io.WriteCloser,
	stdout io.Reader,
	stderr *bytes.Buffer,
	keepAlive time.Duration,
) *sshSession {
	ctx, cancel := context.WithCancel(context.Background())
	remote := &sshSession{
		client:  client,
		session: session,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		done:    make(chan struct{}),
		cancel:  cancel,
	}
	go remote.keepAlive(ctx, keepAlive)
	go func() {
		remote.waitErr = normalizeSSHWaitErr(session.Wait(), stderr)
		cancel()
		close(remote.done)
	}()
	return remote
}

func (s *sshSession) Read(p []byte) (int, error) {
	return s.stdout.Read(p)
}

func (s *sshSession) Write(p []byte) (int, error) {
	return s.stdin.Write(p)
}

func (s *sshSession) CloseWrite() error {
	if s.stdin == nil {
		return nil
	}
	return s.stdin.Close()
}

func (s *sshSession) Close() error {
	var err error
	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		err = errors.Join(s.CloseWrite(), s.session.Close(), s.client.Close())
	})
	return err
}

func (s *sshSession) Done() <-chan struct{} {
	return s.done
}

func (s *sshSession) Wait() error {
	<-s.done
	return s.waitErr
}

func (s *sshSession) Stop(ctx context.Context) error {
	if err := s.Close(); err != nil {
		return err
	}
	select {
	case <-s.done:
		return s.waitErr
	case <-ctx.Done():
		return fmt.Errorf("sandbox/daytona: stop SSH session: %w", ctx.Err())
	}
}

func (s *sshSession) Stderr() string {
	if s.stderr == nil {
		return ""
	}
	return s.stderr.String()
}

func (s *sshSession) keepAlive(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				return
			}
		}
	}
}
