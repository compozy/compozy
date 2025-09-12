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
		// Verify MockLLM received image parts by scanning outputs for echo marker
		require.NotNil(t, result.Output)
		var found bool
		for _, ts := range result.Tasks {
			if ts.Output != nil {
				s := fmt.Sprintf("%v", *ts.Output)
				if strings.Contains(s, "attachments:image_urls=") {
					found = true
					break
				}
			}
		}
		require.True(t, found, "Should include attachment echo marker in task outputs")
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
