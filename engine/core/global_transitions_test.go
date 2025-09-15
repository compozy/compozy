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
		assert.Equal(t, &with, st.GetWith())
		m, err := st.AsMap()
		require.NoError(t, err)
		assert.NotEmpty(t, m)
		err = st.FromMap(map[string]any{"with": map[string]any{"b": 2}})
		require.NoError(t, err)
		assert.Equal(t, 2, (*st.With)["b"])
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
		assert.Equal(t, &with, et.GetWith())
		m, err := et.AsMap()
		require.NoError(t, err)
		assert.NotEmpty(t, m)
		err = et.FromMap(map[string]any{"with": map[string]any{"z": 3}})
		require.NoError(t, err)
		assert.Equal(t, 3, (*et.With)["z"])
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
