package contract

import (
	"encoding/json"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
)

func TestBridgeSendTestRequestContract(t *testing.T) {
	t.Run("Should reject a body target owned by a different bridge", func(t *testing.T) {
		t.Parallel()

		request := BridgeSendTestRequest{
			Message: "Connection check",
			Target: BridgeDeliveryTargetInput{
				BridgeInstanceID: "brg-other",
				PeerID:           "peer-1",
				Mode:             bridgepkg.DeliveryModeDirectSend,
			},
		}

		_, err := request.ToResolveDeliveryTargetRequest("brg-owned")
		if err == nil || !strings.Contains(err.Error(), "must match request path") {
			t.Fatalf("ToResolveDeliveryTargetRequest() error = %v, want path mismatch", err)
		}
	})

	t.Run("Should preserve the real delivery target under the exact path bridge", func(t *testing.T) {
		t.Parallel()

		request := BridgeSendTestRequest{
			Message: "  Connection check  ",
			Target: BridgeDeliveryTargetInput{
				ThreadID: "  thread-1  ",
				Mode:     bridgepkg.DeliveryModeDirectSend,
			},
		}

		resolved, err := request.ToResolveDeliveryTargetRequest(" brg-owned ")
		if err != nil {
			t.Fatalf("ToResolveDeliveryTargetRequest() error = %v", err)
		}
		if got, want := resolved.BridgeInstanceID, " brg-owned "; got != want {
			t.Fatalf("BridgeInstanceID = %q, want %q", got, want)
		}
		if got, want := resolved.ThreadID, "  thread-1  "; got != want {
			t.Fatalf("ThreadID = %q, want %q", got, want)
		}
		if got, want := resolved.Mode, bridgepkg.DeliveryModeDirectSend; got != want {
			t.Fatalf("Mode = %q, want %q", got, want)
		}
		if got, want := request.NormalizedMessage(), "Connection check"; got != want {
			t.Fatalf("NormalizedMessage() = %q, want %q", got, want)
		}
	})

	t.Run("Should reject noncanonical delivery modes at the transport boundary", func(t *testing.T) {
		t.Parallel()

		for _, mode := range []bridgepkg.DeliveryMode{"DIRECT_SEND", "direct", " reply "} {
			t.Run(string(mode), func(t *testing.T) {
				t.Parallel()

				request := BridgeSendTestRequest{
					Target: BridgeDeliveryTargetInput{PeerID: "peer-1", Mode: mode},
				}
				if _, err := request.ToResolveDeliveryTargetRequest("brg-owned"); err == nil {
					t.Fatalf("ToResolveDeliveryTargetRequest(mode=%q) error = nil", mode)
				}
			})
		}
	})
}

func TestBridgeDeliveryTargetInputJSONContract(t *testing.T) {
	t.Run("Should distinguish an omitted mode from explicit empty and null values", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			body      string
			wantError string
		}{
			{
				name: "Should allow an omitted mode for instance defaults",
				body: `{"message":"check","target":{"peer_id":"peer-1"}}`,
			},
			{
				name:      "Should reject an explicitly empty mode",
				body:      `{"message":"check","target":{"peer_id":"peer-1","mode":""}}`,
				wantError: "delivery target mode is required",
			},
			{
				name:      "Should reject an explicitly null mode",
				body:      `{"message":"check","target":{"peer_id":"peer-1","mode":null}}`,
				wantError: "delivery target mode is required",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				var request BridgeSendTestRequest
				err := json.Unmarshal([]byte(test.body), &request)
				if test.wantError == "" {
					if err != nil {
						t.Fatalf("json.Unmarshal() error = %v", err)
					}
					if _, err := request.ToResolveDeliveryTargetRequest("brg-owned"); err != nil {
						t.Fatalf("ToResolveDeliveryTargetRequest() error = %v", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("json.Unmarshal() error = %v, want %q", err, test.wantError)
				}
			})
		}
	})

	t.Run("Should reject lossy identity decoding while accepting valid Unicode", func(t *testing.T) {
		t.Parallel()

		invalidUTF8 := []byte(`{"target":{"peer_id":"`)
		invalidUTF8 = append(invalidUTF8, 0xff)
		invalidUTF8 = append(invalidUTF8, []byte(`","mode":"reply"}}`)...)
		tests := []struct {
			name      string
			body      []byte
			wantError string
			wantPeer  string
		}{
			{
				name:      "Should reject raw invalid UTF-8",
				body:      invalidUTF8,
				wantError: "valid UTF-8",
			},
			{
				name:      "Should reject an unpaired high surrogate",
				body:      []byte(`{"target":{"peer_id":"\ud800","mode":"reply"}}`),
				wantError: "valid Unicode",
			},
			{
				name:      "Should reject an unpaired low surrogate",
				body:      []byte(`{"target":{"peer_id":"\udc00","mode":"reply"}}`),
				wantError: "valid Unicode",
			},
			{
				name:      "Should reject an overwritten invalid surrogate",
				body:      []byte(`{"target":{"peer_id":"\ud800","peer_id":"peer-1","mode":"reply"}}`),
				wantError: "valid Unicode",
			},
			{
				name:      "Should reject a later invalid surrogate",
				body:      []byte(`{"target":{"peer_id":"peer-1","peer_id":"\ud800","mode":"reply"}}`),
				wantError: "valid Unicode",
			},
			{
				name:      "Should reject an overwritten null mode",
				body:      []byte(`{"target":{"peer_id":"peer-1","mode":null,"mode":"reply"}}`),
				wantError: "delivery target mode is required",
			},
			{
				name:      "Should reject a later null mode",
				body:      []byte(`{"target":{"peer_id":"peer-1","mode":"reply","mode":null}}`),
				wantError: "delivery target mode is required",
			},
			{
				name:      "Should reject an overwritten empty mode",
				body:      []byte(`{"target":{"peer_id":"peer-1","mode":"","mode":"reply"}}`),
				wantError: "delivery target mode is required",
			},
			{
				name:     "Should accept a valid surrogate pair",
				body:     []byte(`{"target":{"peer_id":"\ud83d\ude00","mode":"reply"}}`),
				wantPeer: "😀",
			},
			{
				name:     "Should accept an intentional replacement rune",
				body:     []byte(`{"target":{"peer_id":"\ufffd","mode":"reply"}}`),
				wantPeer: "�",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				var request BridgeTestDeliveryRequest
				err := json.Unmarshal(test.body, &request)
				if test.wantError != "" {
					if err == nil || !strings.Contains(err.Error(), test.wantError) {
						t.Fatalf("json.Unmarshal() error = %v, want %q", err, test.wantError)
					}
					return
				}
				if err != nil {
					t.Fatalf("json.Unmarshal() error = %v", err)
				}
				if request.Target.PeerID != test.wantPeer {
					t.Fatalf("PeerID = %q, want %q", request.Target.PeerID, test.wantPeer)
				}
			})
		}
	})
}

func TestBridgeControlResponsesKeepThePublicWireShapeClosed(t *testing.T) {
	t.Run("Should serialize verify records without provider error material", func(t *testing.T) {
		t.Parallel()

		payload := BridgeVerifyResponse{
			BridgeInstanceID: "brg-1",
			Checks: []bridgepkg.BridgeCheckRecord{{
				Check:       "provider_identity",
				Status:      bridgepkg.BridgeCheckStatusFail,
				Remediation: "Replace the bot_token binding, then run verify again.",
			}},
		}

		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		body := string(raw)
		for _, forbidden := range []string{"secret_value", "upstream_error", "stderr", "raw_error"} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("json.Marshal() = %s, want no %q field", body, forbidden)
			}
		}
	})
}
