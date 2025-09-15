package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mutateNestedStructures(m map[string]any) {
	if v, ok := m["nums"].([]int); ok && len(v) > 0 {
		v[0] = 9999
	}
	if nested, ok := m["nested"].(map[string]any); ok {
		nested["k2"] = "changed"
	}
	if s, ok := m["strs"].([]string); ok && len(s) > 0 {
		s[0] = "ZZZ"
	}
}

func TestDeepCopy_Input(t *testing.T) {
	t.Run("Should deep copy Input and preserve type", func(t *testing.T) {
		orig := Input{"a": 1, "nums": []int{1, 2, 3}, "nested": map[string]any{"k1": "v1"}, "strs": []string{"x", "y"}}
		cpy, err := DeepCopy[Input](orig)
		require.NoError(t, err)
		assert.Equal(t, orig, cpy)
		mutateNestedStructures(map[string]any(cpy))
		assert.NotEqual(t, orig, cpy)
		want := Input{"a": 1, "nums": []int{1, 2, 3}, "nested": map[string]any{"k1": "v1"}, "strs": []string{"x", "y"}}
		assert.Equal(t, want, orig)
		_, ok := any(cpy).(Input)
		assert.True(t, ok)
	})
	t.Run("Should return nil for nil Input", func(t *testing.T) {
		var orig Input
		cpy, err := DeepCopy[Input](orig)
		require.NoError(t, err)
		assert.Nil(t, cpy)
	})
}

func TestDeepCopy_Output(t *testing.T) {
	t.Run("Should deep copy Output and preserve type", func(t *testing.T) {
		orig := Output{"x": "y", "nums": []int{10, 20}, "nested": map[string]any{"n": 1}}
		cpy, err := DeepCopy[Output](orig)
		require.NoError(t, err)
		assert.Equal(t, orig, cpy)
		mutateNestedStructures(map[string]any(cpy))
		assert.NotEqual(t, orig, cpy)
		want := Output{"x": "y", "nums": []int{10, 20}, "nested": map[string]any{"n": 1}}
		assert.Equal(t, want, orig)
		_, ok := any(cpy).(Output)
		assert.True(t, ok)
	})
	t.Run("Should return nil for nil Output", func(t *testing.T) {
		var orig Output
		cpy, err := DeepCopy[Output](orig)
		require.NoError(t, err)
		assert.Nil(t, cpy)
	})
}

func TestDeepCopy_InputPtr(t *testing.T) {
	t.Run("Should deep copy *Input", func(t *testing.T) {
		v := Input{"k": "v", "nums": []int{1, 2}, "nested": map[string]any{"a": "b"}}
		orig := &v
		cpyPtr, err := DeepCopy[*Input](orig)
		require.NoError(t, err)
		require.NotNil(t, cpyPtr)
		assert.Equal(t, *orig, *cpyPtr)
		mutateNestedStructures(map[string]any(*cpyPtr))
		assert.NotEqual(t, *orig, *cpyPtr)
		want := Input{"k": "v", "nums": []int{1, 2}, "nested": map[string]any{"a": "b"}}
		assert.Equal(t, want, *orig)
	})
	t.Run("Should return nil when *Input is nil or points to nil map", func(t *testing.T) {
		var pnil *Input
		c1, err := DeepCopy[*Input](pnil)
		require.NoError(t, err)
		assert.Nil(t, c1)
		tmp := Input(nil)
		p := &tmp
		c2, err := DeepCopy[*Input](p)
		require.NoError(t, err)
		assert.Nil(t, c2)
	})
}

func TestDeepCopy_OutputPtr(t *testing.T) {
	t.Run("Should deep copy *Output", func(t *testing.T) {
		v := Output{"ok": true, "nested": map[string]any{"k": "v"}, "nums": []int{4, 5, 6}}
		orig := &v
		cpyPtr, err := DeepCopy[*Output](orig)
		require.NoError(t, err)
		require.NotNil(t, cpyPtr)
		assert.Equal(t, *orig, *cpyPtr)
		mutateNestedStructures(map[string]any(*cpyPtr))
		assert.NotEqual(t, *orig, *cpyPtr)
		want := Output{"ok": true, "nested": map[string]any{"k": "v"}, "nums": []int{4, 5, 6}}
		assert.Equal(t, want, *orig)
	})
	t.Run("Should return nil when *Output is nil or points to nil map", func(t *testing.T) {
		var pnil *Output
		c1, err := DeepCopy[*Output](pnil)
		require.NoError(t, err)
		assert.Nil(t, c1)
		tmp := Output(nil)
		p := &tmp
		c2, err := DeepCopy[*Output](p)
		require.NoError(t, err)
		assert.Nil(t, c2)
	})
}

func TestDeepCopy_Generic(t *testing.T) {
	t.Run("Should copy primitives", func(t *testing.T) {
		i := 42
		ic, err := DeepCopy[int](i)
		require.NoError(t, err)
		assert.Equal(t, i, ic)
		s := "hello"
		sc, err := DeepCopy[string](s)
		require.NoError(t, err)
		assert.Equal(t, s, sc)
	})
	t.Run("Should deep copy struct and map[string]any", func(t *testing.T) {
		type nestedStruct struct {
			K string
			V map[string]int
		}
		type genericStruct struct {
			N   int
			S   string
			Arr []int
			Nst *nestedStruct
		}
		orig := genericStruct{
			N:   7,
			S:   "abc",
			Arr: []int{1, 2, 3},
			Nst: &nestedStruct{K: "k", V: map[string]int{"x": 1}},
		}
		cpy, err := DeepCopy[genericStruct](orig)
		require.NoError(t, err)
		assert.Equal(t, orig, cpy)
		cpy.N, cpy.Arr[0], cpy.Nst.K, cpy.Nst.V["x"] = 8, 999, "k2", 77
		assert.NotEqual(t, orig, cpy)
		want := genericStruct{
			N:   7,
			S:   "abc",
			Arr: []int{1, 2, 3},
			Nst: &nestedStruct{K: "k", V: map[string]int{"x": 1}},
		}
		assert.Equal(t, want, orig)
		m := map[string]any{"a": 1, "b": []string{"a", "b"}, "c": map[string]any{"z": 1}}
		mc, err := DeepCopy[map[string]any](m)
		require.NoError(t, err)
		assert.Equal(t, m, mc)
		mc["a"] = 2
		mc["b"].([]string)[0] = "changed"
		mc["c"].(map[string]any)["z"] = 9
		assert.NotEqual(t, m, mc)
	})
}

func Test_deepCopyMap_SucceedsOnMapAny(t *testing.T) {
	t.Run("Should deep copy and be independent", func(t *testing.T) {
		orig := map[string]any{"k": []int{1, 2}, "m": map[string]any{"x": 1}}
		cpy, err := deepCopyMap(orig)
		require.NoError(t, err)
		assert.Equal(t, orig, cpy)
		if s, ok := cpy["k"].([]int); ok && len(s) > 0 {
			s[0] = 999
		}
		if nm, ok := cpy["m"].(map[string]any); ok {
			nm["x"] = 999
		}
		assert.NotEqual(t, orig, cpy)
	})
}
