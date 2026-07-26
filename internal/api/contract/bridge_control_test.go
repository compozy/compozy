package contract

import (
	"encoding/json"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
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

	t.Run("Should normalize the real delivery target under the path bridge", func(t *testing.T) {
		t.Parallel()

		request := BridgeSendTestRequest{
			Message: "  Connection check  ",
			Target: BridgeDeliveryTargetInput{
				ThreadID: "  thread-1  ",
				Mode:     bridgepkg.DeliveryMode("DIRECT_SEND"),
			},
		}

		resolved, err := request.ToResolveDeliveryTargetRequest(" brg-owned ")
		if err != nil {
			t.Fatalf("ToResolveDeliveryTargetRequest() error = %v", err)
		}
		if got, want := resolved.BridgeInstanceID, "brg-owned"; got != want {
			t.Fatalf("BridgeInstanceID = %q, want %q", got, want)
		}
		if got, want := resolved.ThreadID, "thread-1"; got != want {
			t.Fatalf("ThreadID = %q, want %q", got, want)
		}
		if got, want := resolved.Mode, bridgepkg.DeliveryModeDirectSend; got != want {
			t.Fatalf("Mode = %q, want %q", got, want)
		}
		if got, want := request.NormalizedMessage(), "Connection check"; got != want {
			t.Fatalf("NormalizedMessage() = %q, want %q", got, want)
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
