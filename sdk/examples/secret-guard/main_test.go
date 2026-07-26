package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/subprocess"
)

func TestRunHook(t *testing.T) {
	t.Parallel()

	t.Run("Should deny when a context block contains a secret", func(t *testing.T) {
		t.Parallel()

		payload := hookspkg.InputPreSubmitPayload{
			Message: "hello",
			ContextBlocks: []hookspkg.ContextBlock{
				{Text: "sk-abc123"},
			},
		}

		var stdin bytes.Buffer
		if err := json.NewEncoder(&stdin).Encode(payload); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}

		var stdout bytes.Buffer
		if err := runHook("input_pre_submit", &stdin, &stdout); err != nil {
			t.Fatalf("runHook() error = %v", err)
		}

		var patch hookspkg.InputPreSubmitPatch
		if err := json.NewDecoder(&stdout).Decode(&patch); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if !patch.Deny {
			t.Fatalf("patch = %#v, want deny=true", patch)
		}
		if patch.DenyReason != "Context block contains a potential secret (sk-)" {
			t.Fatalf("patch deny reason = %q, want context block secret reason", patch.DenyReason)
		}
	})
}

func TestSecretGuardRuntimeHandleExecuteHook(t *testing.T) {
	t.Parallel()

	t.Run("Should deny when a context block contains a secret", func(t *testing.T) {
		t.Parallel()

		payload := hookspkg.InputPreSubmitPayload{
			Message: "hello",
			ContextBlocks: []hookspkg.ContextBlock{
				{Text: "sk-abc123"},
			},
		}

		params := executeHookParams{}
		params.Hook.Event = string(hookspkg.HookInputPreSubmit)
		params.Payload = mustRawJSON(payload)

		paramsJSON, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		response, err := (&secretGuardRuntime{}).handleExecuteHook(paramsJSON)
		if err != nil {
			t.Fatalf("handleExecuteHook() error = %v", err)
		}

		patch, ok := response.(hookspkg.InputPreSubmitPatch)
		if !ok {
			t.Fatalf("handleExecuteHook() type = %T, want hookspkg.InputPreSubmitPatch", response)
		}
		if !patch.Deny {
			t.Fatalf("patch = %#v, want deny=true", patch)
		}
		if patch.DenyReason != "Context block contains a potential secret (sk-)" {
			t.Fatalf("patch deny reason = %q, want context block secret reason", patch.DenyReason)
		}
	})
}

func TestSecretGuardShutdownLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should write shutdown response before process exits", func(t *testing.T) {
		t.Parallel()

		cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestSecretGuardShutdownHelper")
		cmd.Env = append(os.Environ(), "AGH_SECRET_GUARD_SHUTDOWN_HELPER=1")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatalf("cmd.StdinPipe() error = %v", err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatalf("cmd.StdoutPipe() error = %v", err)
		}
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			t.Fatalf("cmd.Start() error = %v", err)
		}

		request := rpcEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage("\"shutdown-1\""),
			Method:  "shutdown",
			Params: mustRawJSON(subprocess.ShutdownRequest{
				Reason:     "test",
				DeadlineMS: 1000,
			}),
		}
		if err := json.NewEncoder(stdin).Encode(request); err != nil {
			t.Fatalf("Encode(shutdown request) error = %v", err)
		}
		if err := stdin.Close(); err != nil {
			t.Fatalf("stdin.Close() error = %v", err)
		}

		line, err := readSecretGuardShutdownLine(stdout)
		if err != nil {
			if waitErr := cmd.Wait(); waitErr != nil {
				t.Fatalf("read shutdown response error = %v; wait error = %v; stderr=%s", err, waitErr, stderr.String())
			}
			t.Fatalf("read shutdown response error = %v; stderr=%s", err, stderr.String())
		}
		var response rpcEnvelope
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatalf("json.Unmarshal(response) error = %v; line=%q", err, line)
		}
		if response.Error != nil {
			t.Fatalf("shutdown response error = %#v", response.Error)
		}
		var shutdown subprocess.ShutdownResponse
		payload, err := json.Marshal(response.Result)
		if err != nil {
			t.Fatalf("json.Marshal(response.Result) error = %v", err)
		}
		if err := json.Unmarshal(payload, &shutdown); err != nil {
			t.Fatalf("json.Unmarshal(shutdown) error = %v", err)
		}
		if !shutdown.Acknowledged {
			t.Fatalf("shutdown response = %#v, want acknowledged", shutdown)
		}
		if err := cmd.Wait(); err != nil {
			t.Fatalf("cmd.Wait() error = %v; stderr=%s", err, stderr.String())
		}
	})
}

func TestSecretGuardShutdownHelper(t *testing.T) {
	if os.Getenv("AGH_SECRET_GUARD_SHUTDOWN_HELPER") != "1" {
		t.Skip("helper only")
	}

	t.Run("Should serve with delayed stdout", func(t *testing.T) {
		writer := delayedSecretGuardWriter{
			delay: 500 * time.Millisecond,
			out:   os.Stdout,
		}
		if err := runServe(os.Stdin, writer, os.Stderr); err != nil {
			t.Fatalf("runServe() error = %v", err)
		}
	})
}

func readSecretGuardShutdownLine(stdout io.Reader) (string, error) {
	type result struct {
		line string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		done <- result{line: line, err: err}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			return "", result.err
		}
		return result.line, nil
	case <-time.After(2 * time.Second):
		return "", errors.New("timed out waiting for shutdown response")
	}
}

type delayedSecretGuardWriter struct {
	delay time.Duration
	out   io.Writer
}

func (w delayedSecretGuardWriter) Write(payload []byte) (int, error) {
	time.Sleep(w.delay)
	return w.out.Write(payload)
}
