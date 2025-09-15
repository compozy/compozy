package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Input_Functions(t *testing.T) {
	t.Run("Should create new input and expose helpers", func(t *testing.T) {
		i := NewInput(nil)
		assert.NotNil(t, i)
		var in *Input
		assert.Nil(t, in.AsMap())
		assert.Nil(t, in.Prop("x"))
		in = &Input{"a": 1}
		assert.Equal(t, 1, in.Prop("a"))
		in.Set("b", 2)
		assert.Equal(t, 2, (*in)["b"])
		m := in.AsMap()
		(*in)["a"] = 100
		assert.Equal(t, 1, m["a"]) // copy
	})
	t.Run("Should merge inputs overriding values", func(t *testing.T) {
		a := Input{"a": 1, "b": []int{1}}
		b := Input{"b": []int{2}, "c": 3}
		res, err := a.Merge(&b)
		require.NoError(t, err)
		assert.Equal(t, 1, (*res)["a"])
		assert.Equal(t, []int{1, 2}, (*res)["b"]) // append slice
		assert.Equal(t, 3, (*res)["c"])
		var nilIn *Input
		r2, err := nilIn.Merge(&b)
		require.NoError(t, err)
		assert.Same(t, &b, r2)
	})
	t.Run("Should clone input deeply", func(t *testing.T) {
		in := &Input{"x": []int{1}}
		cp, err := in.Clone()
		require.NoError(t, err)
		require.NotNil(t, cp)
		(*cp)["x"].([]int)[0] = 9
		assert.Equal(t, 1, (*in)["x"].([]int)[0])
	})
}

func Test_Output_Functions(t *testing.T) {
	t.Run("Should expose helpers and merge", func(t *testing.T) {
		var o *Output
		assert.Nil(t, o.AsMap())
		o = &Output{"a": 1}
		assert.Equal(t, 1, o.Prop("a"))
		o.Set("b", 2)
		assert.Equal(t, 2, (*o)["b"])
		m := o.AsMap()
		(*o)["a"] = 100
		assert.Equal(t, 1, m["a"]) // copy
		r, err := o.Merge(Output{"a": 2, "c": 3})
		require.NoError(t, err)
		assert.Equal(t, 2, r["a"]) // override
		assert.Equal(t, 3, r["c"])
	})
	t.Run("Should clone output deeply", func(t *testing.T) {
		o := &Output{"x": []int{1}}
		cp, err := o.Clone()
		require.NoError(t, err)
		require.NotNil(t, cp)
		(*cp)["x"].([]int)[0] = 9
		assert.Equal(t, 1, (*o)["x"].([]int)[0])
	})
}
