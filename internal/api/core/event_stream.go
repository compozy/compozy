package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	eventStreamWriteTimeout = 10 * time.Second
	eventStreamPingInterval = 30 * time.Second
	eventStreamPongTimeout  = 60 * time.Second
)

// prepareEventStream keeps the public SSE representation on both transports.
// WebSockets let document-wide subscriptions remain live without occupying the
// browser's small HTTP/1.1 request pool across tabs.
func (h *BaseHandlers) prepareEventStream(c *gin.Context) (FlushWriter, func(), bool) {
	if !websocket.IsWebSocketUpgrade(c.Request) {
		writer, err := PrepareSSE(c)
		if err != nil {
			h.respondError(c, http.StatusInternalServerError, err)
			return nil, nil, false
		}
		return writer, func() {}, true
	}
	stop, done, started := h.eventStreams.begin()
	if !started {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: event streams are shutting down"))
		return nil, nil, false
	}
	upgrader := websocket.Upgrader{HandshakeTimeout: eventStreamWriteTimeout}
	connection, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		done()
		if h.Logger != nil {
			h.Logger.Debug("api: event stream upgrade failed", "error", err)
		}
		return nil, nil, false
	}
	ctx, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(ctx)
	socket := &eventStreamSocket{connection: connection, cancel: cancel}
	var workers sync.WaitGroup
	cleanup := func() {
		cancel()
		if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) && h.Logger != nil {
			h.Logger.Debug("api: close event stream", "error", err)
		}
		workers.Wait()
		done()
	}
	connection.SetReadLimit(1024)
	if err := connection.SetReadDeadline(time.Now().Add(eventStreamPongTimeout)); err != nil {
		cleanup()
		return nil, nil, false
	}
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(eventStreamPongTimeout))
	})
	workers.Go(func() {
		defer cancel()
		// ReadMessage processes ping/pong/close frames. These subscriptions accept
		// no application messages; any such message also ends the connection.
		if _, _, err := connection.ReadMessage(); err != nil &&
			!websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) &&
			!errors.Is(err, net.ErrClosed) && ctx.Err() == nil && h.Logger != nil {
			h.Logger.Debug("api: event stream disconnected", "error", err)
		}
	})
	workers.Go(func() {
		defer cancel()
		ticker := time.NewTicker(eventStreamPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				if err := connection.WriteControl(websocket.PingMessage, nil,
					time.Now().Add(eventStreamWriteTimeout)); err != nil {
					return
				}
			}
		}
	})
	return socket, cleanup, true
}

// ShutdownEventStreams joins upgraded streams, which net/http cannot drain.
func (h *BaseHandlers) ShutdownEventStreams(ctx context.Context) error {
	if h == nil {
		return nil
	}
	return h.eventStreams.shutdown(ctx)
}

type eventStreamSocket struct {
	connection *websocket.Conn
	cancel     context.CancelFunc
	buffer     bytes.Buffer
	writeErr   error
}

func (s *eventStreamSocket) Write(data []byte) (int, error) {
	if s.writeErr != nil {
		return 0, s.writeErr
	}
	return s.buffer.Write(data)
}

func (s *eventStreamSocket) Flush() {
	if s.writeErr != nil || s.buffer.Len() == 0 {
		return
	}
	s.writeErr = s.connection.SetWriteDeadline(time.Now().Add(eventStreamWriteTimeout))
	if s.writeErr == nil {
		s.writeErr = s.connection.WriteMessage(websocket.TextMessage, s.buffer.Bytes())
	}
	s.buffer.Reset()
	if s.writeErr != nil {
		s.cancel()
	}
}

func (s *eventStreamSocket) FlushError() error {
	s.Flush()
	if s.writeErr != nil {
		return fmt.Errorf("flush event stream: %w", s.writeErr)
	}
	return nil
}

func eventStreamCursor(c *gin.Context) string {
	if cursor := strings.TrimSpace(c.GetHeader("Last-Event-ID")); cursor != "" {
		return cursor
	}
	return strings.TrimSpace(c.Query("last_event_id"))
}
