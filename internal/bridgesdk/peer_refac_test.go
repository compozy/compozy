package bridgesdk

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestPeerResponseRefacs(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve large integer response precision", func(t *testing.T) {
		t.Parallel()

		const want int64 = 9223372036854775807
		raw := []byte(`{"jsonrpc":"2.0","id":"1","result":{"value":9223372036854775807}}`)

		var envelope rpcEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatalf("json.Unmarshal(response) error = %v", err)
		}

		peer := NewPeer(io.Reader(nil), io.Discard)
		responseCh := make(chan rpcResult, 1)
		peer.pending["1"] = responseCh
		peer.handleResponse(envelope)

		response, ok := <-responseCh
		if !ok {
			t.Fatal("response channel closed without payload")
		}
		if response.err != nil {
			t.Fatalf("response error = %v", response.err)
		}

		var decoded struct {
			Value int64 `json:"value"`
		}
		if err := json.Unmarshal(response.result, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(result) error = %v", err)
		}
		if decoded.Value != want {
			t.Fatalf("decoded.Value = %d, want %d", decoded.Value, want)
		}
	})

	t.Run("Should write one escaped JSON frame per line", func(t *testing.T) {
		t.Parallel()

		var output bytes.Buffer
		peer := NewPeer(io.Reader(nil), &output)

		if err := peer.writeFrame(rpcEnvelope{
			JSONRPC: bridgeSDKJSONRPCVersion,
			ID:      json.RawMessage(`"1"`),
			Method:  "echo<>&",
			Params:  json.RawMessage(`{"value":"<>&"}`),
		}); err != nil {
			t.Fatalf("writeFrame() error = %v", err)
		}

		got := output.String()
		if !strings.HasSuffix(got, "\n") {
			t.Fatalf("writeFrame() output = %q, want trailing newline", got)
		}
		if !strings.Contains(got, `echo\u003c\u003e\u0026`) {
			t.Fatalf("writeFrame() output = %q, want escaped method payload", got)
		}

		var decoded rpcEnvelope
		if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(frame) error = %v", err)
		}
		if got, want := decoded.Method, "echo<>&"; got != want {
			t.Fatalf("decoded.Method = %q, want %q", got, want)
		}
	})
}
