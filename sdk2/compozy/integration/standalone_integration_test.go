package compozy

import (
	"fmt"
	"net/http"
	"testing"

	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStandaloneIntegrationLifecycle(t *testing.T) {
	ctx := lifecycleTestContext(t)
	engine, err := New(
		ctx,
		WithWorkflow(&engineworkflow.Config{ID: "integration-standalone"}),
	)
	require.NoError(t, err)
	require.NoError(t, engine.Start(ctx))
	t.Cleanup(func() {
		require.NoError(t, engine.Stop(ctx))
		engine.Wait()
	})
	assert.True(t, engine.IsStarted())
	server := engine.Server()
	require.NotNil(t, server)
	resp, err := http.Get(fmt.Sprintf("http://%s", server.Addr))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	store := engine.ResourceStore()
	require.NotNil(t, store)
}
