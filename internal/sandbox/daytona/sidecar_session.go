package daytona

import (
	"context"

	"errors"
	"fmt"
	"io"

	"net/http"

	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type sidecarSession struct {
	conn         *websocket.Conn
	endpoint     sidecarEndpoint
	sessionID    string
	httpClient   *http.Client
	closeTimeout time.Duration
	stdoutReader *io.PipeReader
	stdoutWriter *io.PipeWriter
	done         chan struct{}
	writeMu      sync.Mutex
	closeWriteMu sync.Once
	closeSession sync.Once
	finishOnce   sync.Once
	stderrMu     sync.Mutex
	stderr       strings.Builder
	waitErr      error
}

func newSidecarSession(
	conn *websocket.Conn,
	endpoint sidecarEndpoint,
	sessionID string,
	httpClient *http.Client,
	closeTimeout time.Duration,
) *sidecarSession {
	stdoutReader, stdoutWriter := io.Pipe()
	session := &sidecarSession{
		conn:         conn,
		endpoint:     endpoint,
		sessionID:    sessionID,
		httpClient:   httpClient,
		closeTimeout: closeTimeout,
		stdoutReader: stdoutReader,
		stdoutWriter: stdoutWriter,
		done:         make(chan struct{}),
	}
	go session.readLoop()
	return session
}

func (s *sidecarSession) Read(p []byte) (int, error) {
	return s.stdoutReader.Read(p)
}

func (s *sidecarSession) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	payload := make([]byte, len(p)+1)
	payload[0] = sidecarFrameClientStdin
	copy(payload[1:], p)
	if err := s.writeFrame(payload); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *sidecarSession) CloseWrite() error {
	var err error
	s.closeWriteMu.Do(func() {
		err = s.writeFrame([]byte{sidecarFrameClientCloseStdin})
	})
	return err
}

func (s *sidecarSession) Close() error {
	stopCtx, cancel := context.WithTimeout(context.Background(), s.closeTimeout)
	defer cancel()
	return errors.Join(s.requestStop(stopCtx), s.closeLocalResources())
}

func (s *sidecarSession) Done() <-chan struct{} {
	return s.done
}

func (s *sidecarSession) Wait() error {
	<-s.done
	return errors.Join(s.waitErr, s.closeLocalResources())
}

func (s *sidecarSession) Stop(ctx context.Context) error {
	stopErr := s.requestStop(ctx)
	select {
	case <-s.done:
		return errors.Join(stopErr, s.waitErr, s.closeLocalResources())
	case <-ctx.Done():
		return errors.Join(
			stopErr,
			fmt.Errorf("sandbox/daytona: stop launcher sidecar session: %w", ctx.Err()),
			s.closeLocalResources(),
		)
	}
}

func (s *sidecarSession) closeLocalResources() error {
	var err error
	s.closeSession.Do(func() {
		err = errors.Join(err, s.endpoint.Close())
		if s.conn != nil {
			err = errors.Join(err, s.conn.Close())
		}
		if closeErr := s.stdoutWriter.Close(); closeErr != nil && !errors.Is(closeErr, io.ErrClosedPipe) {
			err = errors.Join(err, closeErr)
		}
	})
	return err
}

func (s *sidecarSession) Stderr() string {
	s.stderrMu.Lock()
	defer s.stderrMu.Unlock()
	return s.stderr.String()
}

func (s *sidecarSession) writeFrame(payload []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.BinaryMessage, payload)
}
