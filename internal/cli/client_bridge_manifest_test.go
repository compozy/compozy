// Suite: Slack manifest daemon client
// Invariant: the client sends one trimmed instance query and decodes the typed manifest response.
// Boundary IN: UDS-over-HTTP request construction and response decoding.
// Boundary OUT: route handling and manifest semantics, owned by API and bridge suites.
package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestUnixSocketClientSlackBridgeManifest(t *testing.T) {
	t.Run("Should request and decode the typed Slack manifest", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/api/bridges/providers/slack/manifest" {
					t.Fatalf("request = %s %s, want GET manifest route", req.Method, req.URL.Path)
				}
				if got := req.URL.Query().Get("instance"); got != "brg-slack" {
					t.Fatalf("manifest instance query = %q, want brg-slack", got)
				}
				return newHTTPResponse(
					http.StatusOK,
					`{
  "manifest": {
    "_metadata": {"major_version": 1, "minor_version": 1},
    "display_information": {
      "name": "Support agent",
      "description": "Connect Slack messages to AGH.",
      "background_color": "#E8572A"
    },
    "features": {
      "bot_user": {"display_name": "Support agent", "always_online": false},
      "slash_commands": [{
        "command": "/agh",
        "url": "https://bridge.example.com/slack/support",
        "description": "Send a command to AGH",
        "usage_hint": "<command>",
        "should_escape": false
      }]
    },
    "oauth_config": {"scopes": {"bot": ["chat:write"]}},
    "settings": {
      "event_subscriptions": {
        "request_url": "https://bridge.example.com/slack/support",
        "bot_events": ["message.channels"]
      },
      "interactivity": {
        "is_enabled": true,
        "request_url": "https://bridge.example.com/slack/support"
      },
      "org_deploy_enabled": false,
      "socket_mode_enabled": false,
      "token_rotation_enabled": false
    }
  }
}`,
				), nil
			})},
		}

		manifest, err := client.SlackBridgeManifest(context.Background(), " brg-slack ")
		if err != nil {
			t.Fatalf("SlackBridgeManifest() error = %v", err)
		}
		if manifest.Settings.EventSubscriptions.RequestURL !=
			"https://bridge.example.com/slack/support" {
			t.Fatalf("SlackBridgeManifest() = %#v", manifest)
		}
	})

	t.Run("Should reject a blank instance before transport", func(t *testing.T) {
		t.Parallel()

		client := &unixSocketClient{}
		_, err := client.SlackBridgeManifest(context.Background(), "   ")
		if err == nil || !strings.Contains(err.Error(), "bridge instance id is required") {
			t.Fatalf("SlackBridgeManifest(blank) error = %v, want instance remediation", err)
		}
	})
}
