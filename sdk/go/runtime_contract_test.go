package aghsdk_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"testing"
	"time"

	aghsdk "github.com/compozy/agh/sdk/go"
)

func TestSDKRuntimeContracts(t *testing.T) {
	t.Parallel()

	t.Run("Should reject response frames missing both result and error", func(t *testing.T) {
		t.Parallel()

		inputReader, inputWriter := io.Pipe()
		outputReader, outputWriter := io.Pipe()
		transport := aghsdk.NewStdioTransport(aghsdk.StdioTransportOptions{
			Input:  inputReader,
			Output: outputWriter,
		})
		t.Cleanup(func() {
			closePipes(t, inputReader, inputWriter, outputReader, outputWriter)
		})

		callErr := make(chan error, 1)
		go func() {
			var result json.RawMessage
			callErr <- transport.Call(context.Background(), "echo", nil, &result)
		}()
		message := readMessage(t, bufio.NewReader(outputReader))
		if got, want := message["method"], "echo"; got != want {
			t.Fatalf("request method = %#v, want %q", got, want)
		}
		id, ok := message["id"].(float64)
		if !ok {
			t.Fatalf("request id = %#v, want numeric id", message["id"])
		}
		malformed := fmt.Appendf(nil, `{"jsonrpc":"2.0","id":%.0f}`+"\n", id)
		if _, err := inputWriter.Write(malformed); err != nil {
			t.Fatalf("inputWriter.Write(malformed) error = %v", err)
		}

		select {
		case err := <-callErr:
			var rpcErr *aghsdk.RPCError
			if !errors.As(err, &rpcErr) || rpcErr.Code != -32600 {
				t.Fatalf("Call() error = %v, want invalid request RPC error", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Call() did not return after malformed response")
		}
	})

	t.Run("Should freeze handlers and tools after initialize", func(t *testing.T) {
		t.Parallel()

		runtime := newRuntimeHarness(t)
		extension := aghsdk.NewExtension(
			aghsdk.ExtensionDefinition{Name: "Frozen Extension", Version: "0.1.0"},
			aghsdk.WithStdio(runtime.extensionInput, runtime.extensionOutput),
			aghsdk.WithStderr(io.Discard),
		)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			done <- extension.Run(ctx)
		}()
		t.Cleanup(func() {
			cancel()
			if err := runtime.closeInput(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("close input error = %v", err)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("extension runtime did not stop")
			}
		})

		params := initializeParams("Frozen Extension")
		params["capabilities"].(map[string]any)["provides"] = []string{}
		initialize := runtime.call(t, 1, "initialize", params)
		if initialize.Error != nil {
			t.Fatalf("initialize error = %#v", initialize.Error)
		}
		var initialized aghsdk.InitializeResponse
		decodeResult(t, initialize.Result, &initialized)
		snapshot := slices.Clone(initialized.ImplementedMethods)

		if err := extension.Handle("late/method", func(
			context.Context,
			aghsdk.ExtensionContext,
			json.RawMessage,
		) (any, error) {
			return map[string]bool{"ok": true}, nil
		}); err == nil {
			t.Fatal("Handle(after initialize) error = nil, want registration error")
		}
		if err := extension.Tool("late", validToolOptions(), rawOKHandler); err == nil {
			t.Fatal("Tool(after initialize) error = nil, want registration error")
		}
		if got := extension.GetImplementedMethods(); !slices.Equal(got, snapshot) {
			t.Fatalf("GetImplementedMethods() = %#v, want initialized snapshot %#v", got, snapshot)
		}
		provideTools := runtime.call(t, 2, "provide_tools", map[string]any{})
		if provideTools.Error == nil || provideTools.Error.Code != -32601 {
			t.Fatalf("provide_tools response error = %#v, want method not found", provideTools.Error)
		}
	})
}
