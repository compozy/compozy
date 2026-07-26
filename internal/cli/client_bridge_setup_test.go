// Suite: bridge setup daemon client
// Invariant: webhook registration posts to one escaped instance route and decodes a valid typed result.
// Boundary IN: UDS-over-HTTP request construction and response validation.
// Boundary OUT: route handling and provider registration, owned by API and provider integration suites.
package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func TestUnixSocketClientRegisterBridgeWebhook(t *testing.T) {
	t.Parallel()

	t.Run("Should post to the escaped instance route and decode a warning", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost ||
					req.URL.EscapedPath() != "/api/bridges/brg%2Ftelegram/webhook/register" {
					t.Fatalf("request = %s %s, want POST escaped webhook route", req.Method, req.URL.EscapedPath())
				}
				return newHTTPResponse(
					http.StatusOK,
					`{
  "bridge_instance_id": "brg/telegram",
  "status": "warn",
  "remediation": "Confirm the webhook state with Telegram, then retry verify."
}`,
				), nil
			})},
		}

		record, err := client.RegisterBridgeWebhook(context.Background(), " brg/telegram ")
		if err != nil {
			t.Fatalf("RegisterBridgeWebhook() error = %v", err)
		}
		if record.Status != bridgepkg.BridgeCheckStatusWarn ||
			record.Remediation != "Confirm the webhook state with Telegram, then retry verify." {
			t.Fatalf("RegisterBridgeWebhook() = %#v, want typed warning", record)
		}
	})

	t.Run("Should reject a blank instance before transport", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{}
		_, err := client.RegisterBridgeWebhook(context.Background(), "   ")
		if err == nil || !strings.Contains(err.Error(), "bridge instance id is required") {
			t.Fatalf("RegisterBridgeWebhook(blank) error = %v, want instance remediation", err)
		}
	})
}
