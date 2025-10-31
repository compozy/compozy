package compozy

import (
	"testing"
	"time"

	client "github.com/compozy/compozy/sdk/v2/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stringerStub string

func (s stringerStub) String() string {
	return string(s)
}

func TestStringFromOptionsHandlesVariants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", stringFromOptions(nil, "key"))
	assert.Equal(t, "", stringFromOptions(map[string]any{}, "key"))
	assert.Equal(t, "value", stringFromOptions(map[string]any{"key": " value "}, "key"))
	assert.Equal(t, "stringer", stringFromOptions(map[string]any{"key": stringerStub(" stringer ")}, "key"))
	assert.Equal(t, "", stringFromOptions(map[string]any{"key": 10}, "key"))
}

func TestBuildTaskSyncRequestPrefersDuration(t *testing.T) {
	t.Parallel()
	timeout := 5 * time.Second
	req := &ExecuteSyncRequest{
		Input:   map[string]any{"key": "value"},
		Options: map[string]any{"timeout": 30},
		Timeout: &timeout,
	}
	payload := buildTaskSyncRequest(req)
	require.NotNil(t, payload.Timeout)
	assert.Equal(t, 5, *payload.Timeout)
	require.NotNil(t, payload.With)
	payload.With["key"] = "changed"
	assert.Equal(t, "value", req.Input["key"])
}

func TestBuildTaskSyncRequestUsesTimeoutOption(t *testing.T) {
	t.Parallel()
	req := &ExecuteSyncRequest{
		Input:   map[string]any{"key": "value"},
		Options: map[string]any{"timeout": "12"},
	}
	payload := buildTaskSyncRequest(req)
	require.NotNil(t, payload.Timeout)
	assert.Equal(t, 12, *payload.Timeout)
}

func TestBuildWorkflowExecuteRequestHandlesNilInput(t *testing.T) {
	t.Parallel()
	payload := buildWorkflowExecuteRequest(nil)
	assert.Empty(t, payload.Input)
	assert.Empty(t, payload.TaskID)
}

func TestEnsureClientValidatesEngine(t *testing.T) {
	t.Parallel()
	_, err := ensureClient(nil)
	assert.Error(t, err)
	engine := &Engine{}
	_, err = ensureClient(engine)
	assert.Error(t, err)
	engine.client = &client.Client{}
	cli, err := ensureClient(engine)
	require.NoError(t, err)
	assert.NotNil(t, cli)
}
