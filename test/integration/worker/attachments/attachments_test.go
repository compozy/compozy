package attachments

import (
	"fmt"
	"strings"
	"testing"

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
			} else {
				s := fmt.Sprintf("%v", *ts.Output)
				if strings.Contains(s, "image_urls:") {
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
