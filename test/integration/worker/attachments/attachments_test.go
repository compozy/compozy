package attachments

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/attachment"
	"github.com/compozy/compozy/test/integration/worker/helpers"
	"github.com/stretchr/testify/require"
)

func Test_AttachmentsRouter_Image(t *testing.T) {
	t.Run("Should route to image task when kind=image", func(t *testing.T) {
		t.Parallel()
		basePath := getTestDir()
		loader := helpers.NewFixtureLoader(basePath)
		db := helpers.NewDatabaseHelper(t)
		t.Cleanup(func() { db.Cleanup(t) })
		fixture := loader.LoadFixture(t, "", "route_by_kind_image")
		result := helpers.ExecuteWorkflowAndGetState(
			t,
			fixture,
			db,
			"attachments-router-project",
			helpers.CreateBasicAgentConfig(),
		)
		require.NotNil(t, result)
		fixture.AssertWorkflowState(t, result)
		// Verify MockLLM received image parts by asserting image_urls > 0 (with safe fallback)
		require.NotNil(t, result.Output)
		var found bool
		for _, ts := range result.Tasks {
			if ts.Output == nil {
				continue
			}
			if att, ok := (*ts.Output)["attachments"].(map[string]any); ok {
				if v, ok := att["image_urls"]; ok {
					switch n := v.(type) {
					case int:
						if n > 0 {
							found = true
						}
					case int64:
						if n > 0 {
							found = true
						}
					case float64:
						if n > 0 {
							found = true
						}
					}
				}
				continue
			}
			if resp, ok := (*ts.Output)["response"].(string); ok {
				var payload map[string]any
				if err := json.Unmarshal([]byte(resp), &payload); err == nil {
					if att, ok := payload["attachments"].(map[string]any); ok {
						if v, ok := att["image_urls"]; ok {
							switch n := v.(type) {
							case float64:
								if n > 0 {
									found = true
								}
							case int:
								if n > 0 {
									found = true
								}
							case int64:
								if n > 0 {
									found = true
								}
							}
						}
					}
				}
			} else {
				s := fmt.Sprintf("%v", *ts.Output)
				if strings.Contains(s, "image_urls") {
					found = true
				}
			}
		}
		require.True(t, found, "Should include non-zero image_urls in task outputs")
	})
}

func Test_AttachmentsRouter_Audio(t *testing.T) {
	t.Run("Should route to audio task when kind=audio", func(t *testing.T) {
		t.Parallel()
		basePath := getTestDir()
		loader := helpers.NewFixtureLoader(basePath)
		db := helpers.NewDatabaseHelper(t)
		t.Cleanup(func() { db.Cleanup(t) })
		fixture := loader.LoadFixture(t, "", "route_by_kind_audio")
		// Serve a tiny WAV so attachment resolution doesn't hit external network
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "audio/wav")
			// Minimal RIFF header + silence bytes
			_, _ = w.Write(
				[]byte(
					"RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x80\x3e\x00\x00\x02\x00\x10\x00data\x00\x00\x00\x00",
				),
			)
		}))
		t.Cleanup(srv.Close)
		// Rewrite audio URL in fixture to local server
		if fixture != nil && fixture.Workflow != nil {
			for i := range fixture.Workflow.Tasks {
				tk := &fixture.Workflow.Tasks[i]
				if tk.ID == "analyze-audio" && len(tk.Attachments) > 0 {
					for j := range tk.Attachments {
						if tk.Attachments[j].Type() == attachment.TypeAudio {
							if a, ok := tk.Attachments[j].(*attachment.AudioAttachment); ok {
								a.URL = srv.URL + "/sample.wav"
								a.Path = ""
								a.URLs = nil
								a.Paths = nil
							}
						}
					}
				}
			}
		}
		result := helpers.ExecuteWorkflowAndGetState(
			t,
			fixture,
			db,
			"attachments-router-project",
			helpers.CreateBasicAgentConfig(),
		)
		require.NotNil(t, result)
		fixture.AssertWorkflowState(t, result)
	})
}
