package subprocess

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"testing"
	"time"

	bridges "github.com/compozy/agh/internal/bridges/contract"
)

type discardWriteCloser struct {
	io.Writer
}

func (discardWriteCloser) Close() error {
	return nil
}

func BenchmarkTransportWriteJSONRequest(b *testing.B) {
	process := &Process{stdin: discardWriteCloser{Writer: io.Discard}}
	transport := newTransport(process, defaultMaxMessageBytes)
	request := rpcRequest{
		JSONRPC: jsonRPCVersion,
		ID:      json.RawMessage(`42`),
		Method:  "echo",
		Params: struct {
			Message string `json:"message"`
			DelayMS int64  `json:"delay_ms,omitempty"`
		}{
			Message: "hello",
			DelayMS: 25,
		},
	}

	b.ReportAllocs()

	for b.Loop() {
		if err := transport.writeJSON(request); err != nil {
			b.Fatalf("writeJSON() error = %v", err)
		}
	}
}

func BenchmarkParseRPCIDNumeric(b *testing.B) {
	raw := json.RawMessage(`123456789`)

	b.ReportAllocs()

	for b.Loop() {
		id, err := parseRPCID(raw)
		if err != nil {
			b.Fatalf("parseRPCID() error = %v", err)
		}
		if id.key != "n:123456789" {
			b.Fatalf("parseRPCID() key = %q", id.key)
		}
	}
}

func BenchmarkCloneInitializeBridgeRuntime(b *testing.B) {
	runtime := benchmarkBridgeRuntime(8, 3)

	b.ReportAllocs()

	for b.Loop() {
		cloned := CloneInitializeBridgeRuntime(runtime)
		if cloned == nil {
			b.Fatal("CloneInitializeBridgeRuntime() = nil")
			return
		}
		if len(cloned.ManagedInstances) != len(runtime.ManagedInstances) {
			b.Fatalf(
				"len(cloned.ManagedInstances) = %d, want %d",
				len(cloned.ManagedInstances),
				len(runtime.ManagedInstances),
			)
		}
	}
}

func BenchmarkBoundedBufferWriteOverflow(b *testing.B) {
	prefill := bytes.Repeat([]byte("a"), 6*1024)
	payload := bytes.Repeat([]byte("x"), 4*1024)

	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		buffer := &boundedBuffer{
			buf:   append([]byte(nil), prefill...),
			limit: 8 * 1024,
		}
		b.StartTimer()

		if _, err := buffer.Write(payload); err != nil {
			b.Fatalf("boundedBuffer.Write() error = %v", err)
		}
		if got, want := len(buffer.buf), buffer.limit; got != want {
			b.Fatalf("len(buffer.buf) = %d, want %d", got, want)
		}
	}
}

func benchmarkBridgeRuntime(instances int, secretsPerInstance int) *InitializeBridgeRuntime {
	now := time.Unix(1, 0).UTC()
	managed := make([]InitializeBridgeManagedInstance, 0, instances)
	for i := range instances {
		managed = append(managed, InitializeBridgeManagedInstance{
			Instance: bridges.BridgeInstance{
				ID:               "brg-" + strconv.Itoa(i),
				Scope:            bridges.ScopeWorkspace,
				WorkspaceID:      "ws-" + strconv.Itoa(i),
				Platform:         "telegram",
				ExtensionName:    "telegram-reference",
				DisplayName:      "Telegram " + strconv.Itoa(i),
				Enabled:          true,
				Status:           bridges.BridgeStatusReady,
				RoutingPolicy:    bridges.RoutingPolicy{IncludePeer: true},
				ProviderConfig:   json.RawMessage(`{"mode":"bot","token_kind":"secret"}`),
				DeliveryDefaults: json.RawMessage(`{"peer_id":"peer-1"}`),
				CreatedAt:        now,
				UpdatedAt:        now,
			},
			BoundSecrets: benchmarkSecrets(secretsPerInstance),
		})
	}

	return &InitializeBridgeRuntime{
		RuntimeVersion:   InitializeBridgeRuntimeVersion2,
		Purpose:          BridgeRuntimePurposeService,
		Provider:         "telegram-reference",
		Platform:         "telegram",
		ManagedInstances: managed,
	}
}

func benchmarkSecrets(count int) []InitializeBridgeBoundSecret {
	if count <= 0 {
		return nil
	}

	secrets := make([]InitializeBridgeBoundSecret, 0, count)
	for i := range count {
		secrets = append(secrets, InitializeBridgeBoundSecret{
			BindingName: "secret_" + strconv.Itoa(i),
			Kind:        "token",
			Value:       "value-" + strconv.Itoa(i),
		})
	}
	return secrets
}
