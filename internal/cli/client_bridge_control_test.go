// Suite: bridge control daemon client
// Invariant: the CLI client calls the typed UDS control routes and rejects mismatched instance responses.
// Boundary IN: UDS-over-HTTP request construction and response decoding.
// Boundary OUT: provider execution, owned by daemon and adapter suites.
package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func TestUnixSocketClientBridgeControl(t *testing.T) {
	t.Run("Should call verify and validate structured checks", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost || req.URL.Path != "/api/bridges/brg-1/verify" {
					t.Fatalf("request = %s %s, want POST verify route", req.Method, req.URL.Path)
				}
				return newHTTPResponse(http.StatusOK, `{
  "bridge_instance_id": "brg-1",
  "checks": [{"check":"provider.identity","status":"pass","remediation":""}]
}`), nil
			})},
		}

		result, err := client.VerifyBridge(context.Background(), " brg-1 ")
		if err != nil {
			t.Fatalf("VerifyBridge() error = %v", err)
		}
		if len(result.Checks) != 1 || result.Checks[0].Status != bridgepkg.BridgeCheckStatusPass {
			t.Fatalf("VerifyBridge() = %#v", result)
		}
	})

	t.Run("Should post the typed real-send request and validate its target", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost || req.URL.Path != "/api/bridges/brg-1/send-test" {
					t.Fatalf("request = %s %s, want POST send-test route", req.Method, req.URL.Path)
				}
				var request BridgeSendTestRequest
				if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
					t.Fatalf("Decode(request) error = %v", err)
				}
				if request.Message != "hello" || request.Target.PeerID != "peer-1" {
					t.Fatalf("request = %#v", request)
				}
				return newHTTPResponse(http.StatusOK, `{
  "status": "delivered",
  "bridge_instance_id": "brg-1",
  "delivery_id": "del-1",
  "remote_message_id": "remote-1",
  "delivery_target": {
    "bridge_instance_id": "brg-1",
    "peer_id": "peer-1",
    "mode": "direct-send"
  }
}`), nil
			})},
		}

		result, err := client.SendBridgeTest(context.Background(), "brg-1", BridgeSendTestRequest{
			Message: "hello",
			Target: BridgeDeliveryTargetInput{
				PeerID: "peer-1",
				Mode:   bridgepkg.DeliveryModeDirectSend,
			},
		})
		if err != nil {
			t.Fatalf("SendBridgeTest() error = %v", err)
		}
		if result.DeliveryID != "del-1" || result.RemoteMessageID != "remote-1" {
			t.Fatalf("SendBridgeTest() = %#v", result)
		}
	})

	t.Run("Should reject blank identifiers before transport", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{}
		_, err := client.VerifyBridge(context.Background(), "  ")
		if err == nil || !strings.Contains(err.Error(), "bridge instance id is required") {
			t.Fatalf("VerifyBridge(blank) error = %v", err)
		}
	})

	t.Run("Should reject an unknown send-test status", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return newHTTPResponse(http.StatusOK, `{
  "status": "maybe_delivered",
  "bridge_instance_id": "brg-1",
  "delivery_id": "del-1",
  "delivery_target": {
    "bridge_instance_id": "brg-1",
    "peer_id": "peer-1",
    "mode": "direct-send"
  }
}`), nil
			})},
		}

		_, err := client.SendBridgeTest(context.Background(), "brg-1", BridgeSendTestRequest{
			Message: "hello",
			Target: BridgeDeliveryTargetInput{
				PeerID: "peer-1",
				Mode:   bridgepkg.DeliveryModeDirectSend,
			},
		})
		if err == nil || !strings.Contains(err.Error(), "invalid bridge send-test status") {
			t.Fatalf("SendBridgeTest(unknown status) error = %v", err)
		}
	})
}
