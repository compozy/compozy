package daytona

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"

	"strings"

	"github.com/gorilla/websocket"
)

func (s *sidecarSession) readLoop() {
	for {
		messageType, payload, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				s.finish(s.waitErr)
				return
			}
			s.finish(fmt.Errorf("sandbox/daytona: launcher sidecar websocket read: %w", err))
			return
		}
		if messageType != websocket.BinaryMessage || len(payload) == 0 {
			continue
		}
		switch payload[0] {
		case sidecarFrameServerStdout:
			if _, err := s.stdoutWriter.Write(payload[1:]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				s.finish(fmt.Errorf("sandbox/daytona: forward launcher stdout: %w", err))
				return
			}
		case sidecarFrameServerStderr:
			s.appendStderr(payload[1:])
		case sidecarFrameServerExit:
			s.finish(s.exitError(payload[1:]))
			return
		case sidecarFrameServerError:
			s.finish(
				fmt.Errorf("sandbox/daytona: launcher sidecar error: %s", strings.TrimSpace(string(payload[1:]))),
			)
			return
		}
	}
}

func (s *sidecarSession) appendStderr(payload []byte) {
	if len(payload) == 0 {
		return
	}
	s.stderrMu.Lock()
	defer s.stderrMu.Unlock()
	s.stderr.WriteString(string(payload))
}

func (s *sidecarSession) exitError(payload []byte) error {
	var exit sidecarExitPayload
	if err := json.Unmarshal(payload, &exit); err != nil {
		return fmt.Errorf("sandbox/daytona: decode launcher exit payload: %w", err)
	}
	if strings.TrimSpace(exit.Stderr) != "" {
		s.stderrMu.Lock()
		s.stderr.Reset()
		s.stderr.WriteString(exit.Stderr)
		s.stderrMu.Unlock()
	}
	if exit.ExitCode == 0 {
		return nil
	}
	stderr := strings.TrimSpace(s.Stderr())
	if stderr != "" {
		return fmt.Errorf("sandbox/daytona: launcher exited with code %d: %s", exit.ExitCode, stderr)
	}
	return fmt.Errorf("sandbox/daytona: launcher exited with code %d", exit.ExitCode)
}

func (s *sidecarSession) finish(err error) {
	s.finishOnce.Do(func() {
		s.waitErr = err
		if closeErr := s.stdoutWriter.Close(); closeErr != nil && !errors.Is(closeErr, io.ErrClosedPipe) {
			s.waitErr = errors.Join(s.waitErr, fmt.Errorf("close launcher stdout stream: %w", closeErr))
		}
		close(s.done)
	})
}

func (s *sidecarSession) requestStop(ctx context.Context) (err error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		s.endpoint.url(sidecarSessionStreamBasePath, s.sessionID),
		http.NoBody,
	)
	if err != nil {
		return fmt.Errorf("sandbox/daytona: build sidecar stop request: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sandbox/daytona: request sidecar stop: %w", err)
	}
	defer mergeHTTPResponseCloseError(&err, resp, "launcher sidecar stop")
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := readResponseSnippet(resp.Body)
		return fmt.Errorf(
			"sandbox/daytona: sidecar stop status %d: %s",
			resp.StatusCode,
			body,
		)
	}
	return nil
}

func readResponseSnippet(body io.Reader) string {
	if body == nil {
		return ""
	}
	payload, err := io.ReadAll(io.LimitReader(body, 4096))
	if err != nil {
		return fmt.Sprintf("read response body: %v", err)
	}
	return strings.TrimSpace(string(payload))
}
