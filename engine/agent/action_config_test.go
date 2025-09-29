package agent

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ActionConfig_Utilities(t *testing.T) {
	t.Run("Should set and get CWD", func(t *testing.T) {
		var a ActionConfig
		err := a.SetCWD("/test/path")
		require.NoError(t, err)
		require.NotNil(t, a.GetCWD())
		assert.Equal(t, "/test/path", a.GetCWD().Path)
	})

	t.Run("Should return empty input when With is nil", func(t *testing.T) {
		a := &ActionConfig{}
		in := a.GetInput()
		require.NotNil(t, in)
		assert.Equal(t, 0, len(*in))
	})

	t.Run("Should report schema presence and JSON output flag correctly", func(t *testing.T) {
		a := &ActionConfig{}
		assert.False(t, a.HasSchema())
		assert.False(t, a.ShouldUseJSONOutput())

		a.OutputSchema = &schema.Schema{"type": "object"}
		assert.True(t, a.HasSchema())
		assert.True(t, a.ShouldUseJSONOutput())
	})

	t.Run("Should find action by id or return error", func(t *testing.T) {
		a1 := &ActionConfig{ID: "alpha"}
		a2 := &ActionConfig{ID: "beta"}
		got, err := FindActionConfig([]*ActionConfig{a1, a2}, "beta")
		require.NoError(t, err)
		assert.Equal(t, "beta", got.ID)

		got, err = FindActionConfig([]*ActionConfig{a1}, "missing")
		assert.Nil(t, got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "action config not found")
	})

	t.Run("Should round-trip via AsMap and FromMap", func(t *testing.T) {
		a := &ActionConfig{ID: "x", Prompt: "p"}
		m, err := a.AsMap()
		require.NoError(t, err)
		// mutate map to simulate update
		m["prompt"] = "updated"
		var dst ActionConfig
		require.NoError(t, dst.FromMap(m))
		assert.Equal(t, "x", dst.ID)
		assert.Equal(t, "updated", dst.Prompt)
	})

	t.Run("Should validate input and output against schema", func(t *testing.T) {
		a := &ActionConfig{ID: "val"}
		a.InputSchema = &schema.Schema{"type": "object", "required": []string{"name"}}
		in := &core.Input{"name": "john"}
		err := a.ValidateInput(context.Background(), in)
		assert.NoError(t, err)

		a.OutputSchema = &schema.Schema{"type": "object"}
		out := &core.Output{"ok": true}
		err = a.ValidateOutput(context.Background(), out)
		assert.NoError(t, err)
	})
}
