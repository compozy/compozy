package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/procutil"
	"github.com/gorilla/websocket"
)

func TestSidecarSecurityBoundaries(t *testing.T) {
	t.Run("Should bind control plane to loopback", func(t *testing.T) {
		t.Parallel()

		if got, want := sidecarListenAddr(40241), "127.0.0.1:40241"; got != want {
			t.Fatalf("sidecarListenAddr(40241) = %q, want %q", got, want)
		}
	})

	t.Run("Should allow websocket upgrades without an origin header", func(t *testing.T) {
		t.Parallel()

		if allowed := allowWebSocketOrigin(newWebSocketRequest(t, "")); !allowed {
			t.Fatal("allowWebSocketOrigin() = false, want true for empty origin")
		}
	})

	t.Run("Should allow websocket upgrades from the same host", func(t *testing.T) {
		t.Parallel()

		if allowed := allowWebSocketOrigin(newWebSocketRequest(t, "http://127.0.0.1:40241")); !allowed {
			t.Fatal("allowWebSocketOrigin() = false, want true for same-host origin")
		}
	})

	t.Run("Should reject websocket upgrades from foreign origins", func(t *testing.T) {
		t.Parallel()

		if allowed := allowWebSocketOrigin(newWebSocketRequest(t, "https://evil.example")); allowed {
			t.Fatal("allowWebSocketOrigin() = true, want false for foreign origin")
		}
	})

	t.Run("Should reject malformed websocket origins", func(t *testing.T) {
		t.Parallel()

		if allowed := allowWebSocketOrigin(newWebSocketRequest(t, "://bad-origin")); allowed {
			t.Fatal("allowWebSocketOrigin() = true, want false for malformed origin")
		}
	})
}

func TestManagedProcessStop(t *testing.T) {
	t.Run("Should stop forked child processes before returning", func(t *testing.T) {
		t.Parallel()

		if _, err := os.Stat("/bin/sh"); err != nil {
			t.Skipf("sidecar shell is unavailable: %v", err)
		}

		process, err := newManagedProcess("sleep 60 & echo $!; wait")
		if err != nil {
			t.Fatalf("newManagedProcess() error = %v", err)
		}
		t.Cleanup(func() {
			if err := process.Stop(); err != nil {
				t.Fatalf("process.Stop() cleanup error = %v", err)
			}
		})

		shellPID := process.cmd.Process.Pid
		childPID := readManagedProcessPID(t, process.stdout, 5*time.Second)
		waitForProcessState(t, shellPID, true, 2*time.Second)
		waitForProcessState(t, childPID, true, 2*time.Second)

		if err := process.Stop(); err != nil {
			t.Fatalf("process.Stop() error = %v", err)
		}
		if procutil.Alive(shellPID) {
			t.Fatalf("procutil.Alive(%d) = true, want false after stop", shellPID)
		}
		if procutil.Alive(childPID) {
			t.Fatalf("procutil.Alive(%d) = true, want false after stop", childPID)
		}
	})
}

func TestSidecarSessionLifecycle(t *testing.T) {
	t.Run("Should delete sessions from the live store", func(t *testing.T) {
		t.Parallel()

		store := newProcessStore()
		process := newSidecarTestProcess("delete-me")
		store.Put(process)
		handler := newHandler(store, &websocket.Upgrader{CheckOrigin: allowWebSocketOrigin})

		response := httptest.NewRecorder()
		handler.ServeHTTP(
			response,
			httptest.NewRequestWithContext(
				context.Background(),
				http.MethodDelete,
				"/v1/sessions/delete-me",
				http.NoBody,
			),
		)
		if response.Code != http.StatusNoContent {
			t.Fatalf(
				"DELETE status = %d, want %d; body=%q",
				response.Code,
				http.StatusNoContent,
				response.Body.String(),
			)
		}
		if _, found := store.Get("delete-me"); found {
			t.Fatal("store.Get(delete-me) found session after DELETE, want evicted")
		}

		missing := httptest.NewRecorder()
		handler.ServeHTTP(
			missing,
			httptest.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				"/v1/sessions/delete-me/stream",
				http.NoBody,
			),
		)
		if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), "session not found") {
			t.Fatalf(
				"GET deleted stream status/body = %d/%q, want 404 session not found",
				missing.Code,
				missing.Body.String(),
			)
		}
	})

	t.Run("Should reject a second stream for one session", func(t *testing.T) {
		t.Parallel()

		store := newProcessStore()
		process := newCompletedSidecarTestProcess("stream-once")
		store.Put(process)
		server := httptest.NewServer(newHandler(store, &websocket.Upgrader{CheckOrigin: allowWebSocketOrigin}))
		t.Cleanup(server.Close)
		streamURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/sessions/stream-once/stream"

		first, response, err := websocket.DefaultDialer.Dial(streamURL, nil)
		if response != nil && response.Body != nil {
			if closeErr := response.Body.Close(); closeErr != nil {
				t.Fatalf("first response.Body.Close() error = %v", closeErr)
			}
		}
		if err != nil {
			status := 0
			if response != nil {
				status = response.StatusCode
			}
			t.Fatalf("first stream Dial() status/error = %d/%v, want successful websocket", status, err)
		}
		t.Cleanup(func() {
			if err := first.Close(); err != nil {
				t.Errorf("first.Close() error = %v", err)
			}
		})

		second, response, err := websocket.DefaultDialer.Dial(streamURL, nil)
		if response != nil && response.Body != nil {
			if closeErr := response.Body.Close(); closeErr != nil {
				t.Fatalf("second response.Body.Close() error = %v", closeErr)
			}
		}
		if err == nil {
			if closeErr := second.Close(); closeErr != nil {
				t.Errorf("second.Close() error = %v", closeErr)
			}
			t.Fatal("second stream Dial() error = nil, want HTTP 409 rejection")
		}
		if response == nil || response.StatusCode != http.StatusConflict {
			status := 0
			if response != nil {
				status = response.StatusCode
			}
			t.Fatalf("second stream status = %d, want %d", status, http.StatusConflict)
		}
	})
}

func TestSidecarOutputBoundaries(t *testing.T) {
	t.Run("Should cap queued stdout before any stream attaches", func(t *testing.T) {
		t.Parallel()

		queue := newChunkQueue()
		chunk := strings.Repeat("x", 64*1024)
		for range 80 {
			if err := queue.Push([]byte(chunk)); err != nil && !errors.Is(err, errOutputBufferExceeded) {
				t.Fatalf("queue.Push() error = %v", err)
			}
		}
		if got, limit := bufferedChunkBytes(queue), 4*1024*1024; got > limit {
			t.Fatalf("buffered stdout bytes = %d, want <= %d", got, limit)
		}
	})

	t.Run("Should cap stderr retained for exit payloads", func(t *testing.T) {
		t.Parallel()

		process := &managedProcess{}
		process.appendStderr(strings.Repeat("x", 2*1024*1024))
		if got, limit := len(process.stderrText()), 1024*1024+128; got > limit {
			t.Fatalf("stderrText length = %d, want <= %d", got, limit)
		}
	})
}

func newWebSocketRequest(t *testing.T, origin string) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://127.0.0.1:40241/v1/sessions/test-session/stream",
		http.NoBody,
	)
	req.Host = "127.0.0.1:40241"
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req
}

func newSidecarTestProcess(id string) *managedProcess {
	return &managedProcess{
		id:       id,
		cmd:      &exec.Cmd{},
		stdout:   newChunkQueue(),
		done:     make(chan struct{}),
		exitCode: -1,
	}
}

func newCompletedSidecarTestProcess(id string) *managedProcess {
	process := newSidecarTestProcess(id)
	process.stdout.Close()
	close(process.done)
	process.exitCode = 0
	return process
}

func bufferedChunkBytes(queue *chunkQueue) int {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	total := 0
	for _, chunk := range queue.chunks {
		total += len(chunk)
	}
	return total
}

func readManagedProcessPID(t *testing.T, stdout *chunkQueue, timeout time.Duration) int {
	t.Helper()

	chunk := readChunkWithin(t, stdout, timeout)
	pid, err := strconv.Atoi(strings.TrimSpace(string(chunk)))
	if err != nil {
		t.Fatalf("parse child pid %q error = %v", string(chunk), err)
	}
	return pid
}

func readChunkWithin(t *testing.T, stdout *chunkQueue, timeout time.Duration) []byte {
	t.Helper()

	type popResult struct {
		chunk []byte
		ok    bool
	}
	resultCh := make(chan popResult, 1)
	go func() {
		chunk, ok := stdout.Pop()
		resultCh <- popResult{chunk: chunk, ok: ok}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-resultCh:
		if !result.ok {
			t.Fatal("stdout.Pop() = closed, want child pid output")
		}
		return result.chunk
	case <-timer.C:
		t.Fatalf("stdout.Pop() timed out after %s", timeout)
		return nil
	}
}

func waitForProcessState(t *testing.T, pid int, wantAlive bool, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if procutil.Alive(pid) == wantAlive {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("procutil.Alive(%d) did not become %t within %s", pid, wantAlive, timeout)
		}
		<-ticker.C
	}
}
