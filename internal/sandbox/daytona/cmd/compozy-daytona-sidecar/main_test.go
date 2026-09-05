package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/compozy/compozy/internal/procutil"
	"github.com/gorilla/websocket"
)

func TestSidecarSecurityBoundaries(t *testing.T) {
	t.Run("Should return entropy failures without publishing a process ID", func(t *testing.T) {
		t.Parallel()

		entropyErr := errors.New("entropy unavailable")
		id, err := randomIDFromReader(iotest.ErrReader(entropyErr))
		if id != "" {
			t.Fatalf("randomIDFromReader() id = %q, want empty", id)
		}
		if !errors.Is(err, entropyErr) {
			t.Fatalf("randomIDFromReader() error = %v, want %v", err, entropyErr)
		}
	})

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
	t.Run("Should bound the exit observer wait after process group termination", func(t *testing.T) {
		t.Parallel()
		missing, err := os.FindProcess(1 << 30)
		if err != nil {
			t.Fatalf("create missing process handle: %v", err)
		}
		t.Cleanup(func() {
			if err := missing.Release(); err != nil {
				t.Errorf("release missing process handle: %v", err)
			}
		})
		process := newSidecarTestProcess("missing-exit-observer")
		process.cmd.Process = missing
		var attempts atomic.Int32
		process.cancel = func() { attempts.Add(1) }
		var workers sync.WaitGroup
		stopped := make(chan struct{})
		var stopErr error
		workers.Go(func() {
			stopErr = process.Stop()
			close(stopped)
		})
		t.Cleanup(func() {
			close(process.done)
			// Release a stalled observer even when the assertion exposes an unbounded wait.
			joined := make(chan struct{})
			go func() { workers.Wait(); close(joined) }()
			select {
			case <-joined:
			case <-time.After(time.Second):
				t.Error("stop worker did not join after observer release")
			}
		})
		select {
		case <-stopped:
			if !errors.Is(stopErr, context.DeadlineExceeded) {
				t.Fatalf("missing observer result = %v", stopErr)
			}
		case <-time.After(2*stopTimeout + time.Second):
			t.Fatal("stop exceeded its termination and observer budgets")
		}
		retries := make(chan error, 8)
		for range 8 {
			workers.Go(func() { retries <- process.Stop() })
		}
		deadline := time.NewTimer(2*stopTimeout + time.Second)
		defer deadline.Stop()
		for range 8 {
			select {
			case err := <-retries:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("retry observer result = %v", err)
				}
			case <-deadline.C:
				t.Fatal("concurrent retry did not share one bounded stop attempt")
			}
		}
		if attempts.Load() != 2 {
			t.Fatalf("stop attempts = %d, want original plus one shared retry", attempts.Load())
		}
	})
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
	t.Run("Should reserve one process for concurrent identified launches and reject reuse", func(t *testing.T) {
		t.Parallel()
		const id = "reserved-process-identity"
		store := newProcessStore()
		handler := newHandler(store, &websocket.Upgrader{CheckOrigin: allowWebSocketOrigin})
		t.Cleanup(func() {
			if process, found := store.Get(id); found {
				if err := process.Stop(); err != nil {
					t.Errorf("stop reserved process: %v", err)
				}
			}
		})
		responses := make(chan *httptest.ResponseRecorder, 8)
		var requests sync.WaitGroup
		for range 8 {
			requests.Go(func() {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost,
					"/v1/launch/identified", strings.NewReader(`{"command":"sleep 60","id":"`+id+`"}`)))
				responses <- response
			})
		}
		requests.Wait()
		close(responses)
		created := 0
		for response := range responses {
			if response.Code == http.StatusCreated {
				created++
				var launched launchResponse
				if err := json.Unmarshal(response.Body.Bytes(), &launched); err != nil || launched.ID != id {
					t.Fatalf("launch identity response = %q, %v", response.Body.String(), err)
				}
			} else if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "identity already used") {
				t.Fatalf("duplicate launch = %d %q", response.Code, response.Body.String())
			}
		}
		if created != 1 {
			t.Fatalf("created processes = %d, want 1", created)
		}
		process, found := store.Get(id)
		if !found || !procutil.Alive(process.cmd.Process.Pid) {
			t.Fatal("reserved identity does not resolve to its live process")
		}
		if err := process.Stop(); err != nil {
			t.Fatalf("stop reserved process: %v", err)
		}
		store.Remove(id)
		if _, err := store.LaunchIdentified("sleep 60", id); !errors.Is(err, errProcessIDUsed) {
			t.Fatalf("deleted identity reuse = %v", err)
		}
	})
	t.Run("Should report known process exit without treating missing identity as proof", func(t *testing.T) {
		t.Parallel()
		process, err := newManagedProcess("sleep 60 & echo $!; wait")
		if err != nil {
			t.Fatalf("launch process: %v", err)
		}
		t.Cleanup(func() {
			if err := process.Stop(); err != nil {
				t.Errorf("stop process: %v", err)
			}
		})
		childPID := readManagedProcessPID(t, process.stdout, 5*time.Second)
		store := newProcessStore()
		store.Put(process)
		handler := newHandler(store, &websocket.Upgrader{CheckOrigin: allowWebSocketOrigin})
		status := readSidecarProcessStatus(t, handler, process.id)
		if status["id"] != process.id || status["exited"] != false || status["exitVerified"] != false {
			t.Fatalf("live process status = %v", status)
		}
		if _, found := status["exitCode"]; found {
			t.Fatalf("live process reported an exit code: %v", status)
		}
		signalResponse := httptest.NewRecorder()
		handler.ServeHTTP(signalResponse, httptest.NewRequestWithContext(t.Context(), http.MethodPost,
			"/v1/sessions/"+process.id+"/signal", strings.NewReader(`{"signal":"terminate"}`)))
		if signalResponse.Code != http.StatusNoContent {
			t.Fatalf("signal response = %d %q", signalResponse.Code, signalResponse.Body.String())
		}
		select {
		case <-process.done:
		case <-time.After(5 * time.Second):
			t.Fatal("remote termination did not finish")
		}
		status = readSidecarProcessStatus(t, handler, process.id)
		if status["exited"] != true || status["exitVerified"] != true || status["exitCode"] == nil {
			t.Fatalf("completed process status = %v", status)
		}
		if procutil.Alive(childPID) || procutil.Alive(process.cmd.Process.Pid) {
			t.Fatal("exit proof was published before the process group exited")
		}
		unverified := newCompletedSidecarTestProcess("unverified")
		store.Put(unverified)
		status = readSidecarProcessStatus(t, handler, unverified.id)
		if status["exited"] != true || status["exitVerified"] != false {
			t.Fatalf("command exit alone claimed group exit proof: %v", status)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "/v1/sessions/unknown", http.NoBody,
		))
		if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "session not found") {
			t.Fatalf("unknown identity response = %d %q", response.Code, response.Body.String())
		}
	})
	t.Run("Should retain a failed stop and its original error across delete requests", func(t *testing.T) {
		t.Parallel()
		store := newProcessStore()
		process := newSidecarTestProcess("failed-stop")
		closeErr := errors.New("stdin close failed")
		process.stdin = &sidecarCloseErrorWriter{Writer: io.Discard, err: closeErr}
		store.Put(process)
		handler := newHandler(store, &websocket.Upgrader{CheckOrigin: allowWebSocketOrigin})
		for range 2 {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequestWithContext(
				t.Context(), http.MethodDelete, "/v1/sessions/failed-stop", http.NoBody,
			))
			if response.Code != http.StatusInternalServerError ||
				!strings.Contains(response.Body.String(), closeErr.Error()) {
				t.Fatalf("failed stop response = %d %q", response.Code, response.Body.String())
			}
			if retained, found := store.Get(process.id); !found || retained != process {
				t.Fatal("failed remote stop lost its process record")
			}
			if err := process.Stop(); !errors.Is(err, closeErr) {
				t.Fatalf("repeated stop lost original failure: %v", err)
			}
		}
	})
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

func readSidecarProcessStatus(t *testing.T, handler http.Handler, id string) map[string]any {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/v1/sessions/"+id, http.NoBody,
	))
	if response.Code != http.StatusOK {
		t.Fatalf("process status = %d %q", response.Code, response.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode process status: %v", err)
	}
	return status
}

type sidecarCloseErrorWriter struct {
	io.Writer
	err error
}

func (w *sidecarCloseErrorWriter) Close() error { return w.err }

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

	t.Run("Should report stdout and stderr pipe close failures", func(t *testing.T) {
		t.Parallel()

		stdoutCloseErr := errors.New("close stdout")
		stdoutProcess := &managedProcess{stdout: newChunkQueue()}
		stdoutProcess.captureStdout(&sidecarCloseErrorReader{
			Reader:   strings.NewReader(""),
			closeErr: stdoutCloseErr,
		})
		if stderr := stdoutProcess.stderrText(); !strings.Contains(stderr, stdoutCloseErr.Error()) {
			t.Fatalf("stdout process stderr = %q, want close failure", stderr)
		}

		stderrCloseErr := errors.New("close stderr")
		stderrProcess := &managedProcess{}
		stderrProcess.captureStderr(&sidecarCloseErrorReader{
			Reader:   strings.NewReader(""),
			closeErr: stderrCloseErr,
		})
		if stderr := stderrProcess.stderrText(); !strings.Contains(stderr, stderrCloseErr.Error()) {
			t.Fatalf("stderr process stderr = %q, want close failure", stderr)
		}
	})
}

type sidecarCloseErrorReader struct {
	io.Reader
	closeErr error
}

func (r *sidecarCloseErrorReader) Close() error {
	return r.closeErr
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
