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

	bridgepkg "github.com/compozy/compozy/internal/bridges"
)

func TestUnixSocketClientBridgeControl(t *testing.T) {
	t.Run("Should call verify and validate structured checks", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/compozy.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost || req.URL.EscapedPath() != "/api/bridges/%20brg-1%20/verify" {
					t.Fatalf("request = %s %s, want POST verify route", req.Method, req.URL.Path)
				}
				return newHTTPResponse(http.StatusOK, `{
  "bridge_instance_id": " brg-1 ",
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
			socketPath: "/tmp/compozy.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost || req.URL.EscapedPath() != "/api/bridges/%20brg-1%20/send-test" {
					t.Fatalf("request = %s %s, want POST send-test route", req.Method, req.URL.Path)
				}
				var request BridgeSendTestRequest
				if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
					t.Fatalf("Decode(request) error = %v", err)
				}
				if request.Message != "hello" || request.Target.PeerID != " peer-1 " ||
					request.Target.ThreadID != " thread-1 " || request.Target.GroupID != " group-1 " {
					t.Fatalf("request = %#v", request)
				}
				return newHTTPResponse(http.StatusOK, `{
  "status": "delivered",
  "bridge_instance_id": " brg-1 ",
  "delivery_id": "del-1",
  "remote_message_id": "remote-1",
  "delivery_target": {
    "bridge_instance_id": " brg-1 ",
    "peer_id": " peer-1 ",
    "thread_id": " thread-1 ",
    "group_id": " group-1 ",
    "mode": "direct-send"
  }
}`), nil
			})},
		}

		result, err := client.SendBridgeTest(context.Background(), " brg-1 ", BridgeSendTestRequest{
			Message: "hello",
			Target: BridgeDeliveryTargetInput{
				PeerID:   " peer-1 ",
				ThreadID: " thread-1 ",
				GroupID:  " group-1 ",
				Mode:     bridgepkg.DeliveryModeDirectSend,
			},
		})
		if err != nil {
			t.Fatalf("SendBridgeTest() error = %v", err)
		}
		if result.DeliveryID != "del-1" || result.RemoteMessageID != "remote-1" {
			t.Fatalf("SendBridgeTest() = %#v", result)
		}
	})

	t.Run("Should preserve opaque identifiers on the dry-run client route", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/compozy.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost ||
					req.URL.EscapedPath() != "/api/bridges/%20brg-dry%20/test-delivery" {
					t.Fatalf("request = %s %s, want POST test-delivery route", req.Method, req.URL.EscapedPath())
				}
				var request BridgeTestDeliveryRequest
				if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
					t.Fatalf("Decode(request) error = %v", err)
				}
				if request.Target.PeerID != " peer-dry " || request.Target.ThreadID != " thread-dry " ||
					request.Target.GroupID != " group-dry " {
					t.Fatalf("request = %#v", request)
				}
				return newHTTPResponse(http.StatusOK, `{
  "status": "resolved",
  "delivery_target": {
    "bridge_instance_id": " brg-dry ",
    "peer_id": " peer-dry ",
    "thread_id": " thread-dry ",
    "group_id": " group-dry ",
    "mode": "reply"
  }
}`), nil
			})},
		}

		result, err := client.TestBridgeDelivery(context.Background(), " brg-dry ", BridgeTestDeliveryRequest{
			Target: BridgeDeliveryTargetInput{
				PeerID:   " peer-dry ",
				ThreadID: " thread-dry ",
				GroupID:  " group-dry ",
				Mode:     bridgepkg.DeliveryModeReply,
			},
		})
		if err != nil {
			t.Fatalf("TestBridgeDelivery() error = %v", err)
		}
		if result.DeliveryTarget.BridgeInstanceID != " brg-dry " ||
			result.DeliveryTarget.PeerID != " peer-dry " ||
			result.DeliveryTarget.ThreadID != " thread-dry " ||
			result.DeliveryTarget.GroupID != " group-dry " {
			t.Fatalf("TestBridgeDelivery() = %#v", result)
		}
	})

	t.Run("Should reject empty and invalid identifiers before transport", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{}
		_, err := client.VerifyBridge(context.Background(), "")
		if err == nil || !strings.Contains(err.Error(), "bridge instance id is required") {
			t.Fatalf("VerifyBridge(blank) error = %v", err)
		}
		_, err = client.VerifyBridge(context.Background(), string([]byte{0xff}))
		if err == nil || !strings.Contains(err.Error(), "must be valid UTF-8") {
			t.Fatalf("VerifyBridge(invalid UTF-8) error = %v", err)
		}

		client.httpClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("transport should not be called for an invalid target identity")
			return nil, nil
		})}
		invalidID := string([]byte{0xff})
		_, err = client.SendBridgeTest(context.Background(), "brg-1", BridgeSendTestRequest{
			Message: "hello",
			Target: BridgeDeliveryTargetInput{
				PeerID: invalidID,
				Mode:   bridgepkg.DeliveryModeDirectSend,
			},
		})
		if err == nil || !strings.Contains(err.Error(), "peer id must be valid UTF-8") {
			t.Fatalf("SendBridgeTest(invalid peer ID) error = %v", err)
		}
		_, err = client.TestBridgeDelivery(context.Background(), "brg-1", BridgeTestDeliveryRequest{
			Target: BridgeDeliveryTargetInput{
				GroupID: invalidID,
				Mode:    bridgepkg.DeliveryModeDirectSend,
			},
		})
		if err == nil || !strings.Contains(err.Error(), "group id must be valid UTF-8") {
			t.Fatalf("TestBridgeDelivery(invalid group ID) error = %v", err)
		}
	})

	t.Run("Should reject an unknown send-test status", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/compozy.sock",
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
