package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SuccessTransition_Mapping(t *testing.T) {
	t.Run("Should expose With via GetWith and map helpers", func(t *testing.T) {
		with := Input{"a": 1}
		st := &SuccessTransition{With: &with}
		assert.Same(t, &with, st.GetWith(), "GetWith should return the same pointer instance")
		m, err := st.AsMap()
		require.NoError(t, err)
		assert.NotEmpty(t, m)
		assert.Contains(t, m, "with")
		wm, ok := m["with"].(map[string]any)
		require.True(t, ok)
		assert.EqualValues(t, 1, wm["a"])
		err = st.FromMap(map[string]any{"with": map[string]any{"b": 2}})
		require.NoError(t, err)
		assert.EqualValues(t, 2, (*st.With)["b"])
		assert.EqualValues(t, 1, (*st.With)["a"])
	})
	t.Run("Should error on invalid with type", func(t *testing.T) {
		st := &SuccessTransition{}
		err := st.FromMap(map[string]any{"with": 123})
		require.Error(t, err)
		assert.ErrorContains(t, err, "with")
	})
	t.Run("Should handle nil With safely", func(t *testing.T) {
		st := &SuccessTransition{}
		m, err := st.AsMap()
		require.NoError(t, err)
		assert.NotNil(t, m)
		require.NoError(t, st.FromMap(map[string]any{}))
		assert.Nil(t, st.With)
	})
}

func Test_ErrorTransition_Mapping(t *testing.T) {
	t.Run("Should expose With via GetWith and map helpers", func(t *testing.T) {
		with := Input{"x": "y"}
		et := &ErrorTransition{With: &with}
		assert.Same(t, &with, et.GetWith(), "GetWith should return the same pointer instance")
		m, err := et.AsMap()
		require.NoError(t, err)
		assert.NotEmpty(t, m)
		assert.Contains(t, m, "with")
		wm, ok := m["with"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "y", wm["x"])
		err = et.FromMap(map[string]any{"with": map[string]any{"z": 3}})
		require.NoError(t, err)
		assert.EqualValues(t, 3, (*et.With)["z"])
		assert.Equal(t, "y", (*et.With)["x"])
	})
	t.Run("Should error on invalid with type", func(t *testing.T) {
		et := &ErrorTransition{}
		err := et.FromMap(map[string]any{"with": 123})
		require.Error(t, err)
		assert.ErrorContains(t, err, "with")
	})
	t.Run("Should handle nil With safely", func(t *testing.T) {
		et := &ErrorTransition{}
		m, err := et.AsMap()
		require.NoError(t, err)
		assert.NotNil(t, m)
		require.NoError(t, et.FromMap(map[string]any{}))
		assert.Nil(t, et.With)
	})
}
