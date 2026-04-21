package testutil

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
)

// SSEFrame is one parsed server-sent-event frame collected from a stream body.
type SSEFrame struct {
	ID    string
	Event string
	Data  []byte
}

// ReadSSEFramesUntil reads SSE frames from body until stop reports success,
// the stream ends, or timeout expires. The helper owns body for the duration of
// the read and closes it on early termination so the scanner goroutine exits.
func ReadSSEFramesUntil(
	body io.ReadCloser,
	timeout time.Duration,
	stop func([]SSEFrame) bool,
) ([]SSEFrame, error) {
	linesCh, errCh := startSSEScan(body)
	frames := make([]SSEFrame, 0, 8)
	current := SSEFrame{}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case line, ok := <-linesCh:
			if !ok {
				return finalizeSSEFrames(frames, &current, errCh)
			}
			if line == "" {
				frames = flushSSEFrame(frames, &current)
				if stop(frames) {
					drainSSEScan(body, linesCh, errCh)
					return frames, nil
				}
				continue
			}
			appendSSELine(&current, line)
		case <-timer.C:
			drainSSEScan(body, linesCh, errCh)
			return nil, fmt.Errorf("timeout reading SSE frames; collected %#v", frames)
		}
	}
}

func startSSEScan(body io.Reader) (<-chan string, <-chan error) {
	linesCh := make(chan string, 64)
	errCh := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			linesCh <- scanner.Text()
		}
		errCh <- scanner.Err()
		close(linesCh)
	}()
	return linesCh, errCh
}

func finalizeSSEFrames(frames []SSEFrame, current *SSEFrame, errCh <-chan error) ([]SSEFrame, error) {
	frames = flushSSEFrame(frames, current)
	if err := <-errCh; err != nil {
		return nil, fmt.Errorf("scan SSE frames: %w", err)
	}
	return frames, nil
}

func flushSSEFrame(frames []SSEFrame, current *SSEFrame) []SSEFrame {
	if current == nil || (current.ID == "" && current.Event == "" && len(current.Data) == 0) {
		return frames
	}
	if len(current.Data) > 0 && current.Data[len(current.Data)-1] == '\n' {
		current.Data = current.Data[:len(current.Data)-1]
	}
	frames = append(frames, *current)
	*current = SSEFrame{}
	return frames
}

func appendSSELine(current *SSEFrame, line string) {
	if current == nil {
		return
	}
	switch {
	case strings.HasPrefix(line, "id: "):
		current.ID = strings.TrimPrefix(line, "id: ")
	case strings.HasPrefix(line, "event: "):
		current.Event = strings.TrimPrefix(line, "event: ")
	case strings.HasPrefix(line, "data: "):
		current.Data = append(current.Data, strings.TrimPrefix(line, "data: ")...)
		current.Data = append(current.Data, '\n')
	}
}

func drainSSEScan(body io.ReadCloser, linesCh <-chan string, errCh <-chan error) {
	_ = body.Close()
	for line := range linesCh {
		_ = line
	}
	<-errCh
}
