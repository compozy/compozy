// Suite: Slack manifest UDS route
// Invariant: UDS exposes the same typed Slack manifest response as HTTP for one bridge instance.
// Boundary IN: UDS route registration and shared BaseHandlers invocation.
// Boundary OUT: manifest construction semantics, owned by internal/bridges/slack_manifest_test.go.
package udsapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func TestSlackBridgeManifestHandlerReturnsRequestedPayload(t *testing.T) {
	t.Run("Should return the same manifest contract over UDS", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		bridges := stubBridgeService{
			GetInstanceFn: func(_ context.Context, id string) (*bridgepkg.BridgeInstance, error) {
				if id != "brg-slack" {
					t.Fatalf("GetInstance() id = %q, want brg-slack", id)
				}
				return &bridgepkg.BridgeInstance{
					ID:             id,
					Scope:          bridgepkg.ScopeGlobal,
					Platform:       "slack",
					ExtensionName:  "slack-reference",
					DisplayName:    "Support agent",
					ProviderConfig: []byte(`{"webhook":{"public_url":"https://bridge.example.com/slack/support"}}`),
				}, nil
			},
			ListProvidersFn: func(context.Context) ([]bridgepkg.BridgeProvider, error) {
				return []bridgepkg.BridgeProvider{{
					Platform:      "slack",
					ExtensionName: "slack-reference",
					DisplayName:   "Slack",
					Description:   "Connect Slack messages, commands, actions, and reactions to AGH.",
				}}, nil
			},
		}

		engine := newTestRouter(
			t,
			newTestHandlersWithBridges(
				t,
				stubSessionManager{},
				stubObserver{},
				bridges,
				stubWorkspaceService{},
				homePaths,
			),
		)
		recorder := performRequest(
			t,
			engine,
			http.MethodGet,
			"/api/bridges/providers/slack/manifest?instance=brg-slack",
			nil,
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.SlackAppManifestResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Manifest.DisplayInformation.Name != "Support agent" ||
			response.Manifest.Settings.EventSubscriptions.RequestURL !=
				"https://bridge.example.com/slack/support" {
			t.Fatalf("manifest = %#v", response.Manifest)
		}
	})
}
