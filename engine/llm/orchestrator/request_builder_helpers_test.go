package orchestrator

import (
	"context"
	"errors"
	"testing"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type regToolWithArgs struct {
	name, desc string
	params     map[string]any
}

func (r *regToolWithArgs) Name() string                                 { return r.name }
func (r *regToolWithArgs) Description() string                          { return r.desc }
func (r *regToolWithArgs) Call(context.Context, string) (string, error) { return "{}", nil }
func (r *regToolWithArgs) ArgsType() any                                { return r.params }

type listableRegistry struct {
	tools []RegistryTool
	find  map[string]RegistryTool
	err   error
}

func (l *listableRegistry) Find(_ context.Context, name string) (RegistryTool, bool) {
	if l.find == nil {
		return nil, false
	}
	t, ok := l.find[name]
	return t, ok
}
func (l *listableRegistry) ListAll(context.Context) ([]RegistryTool, error) {
	if l.err != nil {
		return nil, l.err
	}
	return l.tools, nil
}
func (l *listableRegistry) Close() error { return nil }

func TestRequestBuilder_NormalizeCloneAndType(t *testing.T) {
	t.Run("Should normalize parameters to object with properties", func(t *testing.T) {
		in := map[string]any{"type": "string"}
		out := normalizeToolParameters(in)
		assert.Equal(t, "object", out["type"])
		_, ok := out["properties"].(map[string]any)
		assert.True(t, ok)
	})
	t.Run("Should clone maps without aliasing", func(t *testing.T) {
		src := map[string]any{"a": 1}
		c := cloneMap(src)
		src["a"] = 2
		assert.Equal(t, 1, c["a"])
	})
	t.Run("Should detect object type case-insensitively", func(t *testing.T) {
		assert.True(t, isObjectType("Object"))
		assert.False(t, isObjectType("array"))
	})
}

func TestRequestBuilder_CollectAndAppendDefs(t *testing.T) {
	rb := &requestBuilder{tools: &listableRegistry{}}
	t.Run("Should error when configured tool not found", func(t *testing.T) {
		_, _, err := rb.collectConfiguredToolDefs(context.Background(), []tool.Config{{ID: "missing"}})
		require.Error(t, err)
	})
	t.Run("Should append missing registry tools and skip included", func(t *testing.T) {
		included := map[string]struct{}{"exists": {}}
		defs := []llmadapter.ToolDefinition{
			{
				Name:        "exists",
				Description: "d",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		}
		reg := &listableRegistry{
			tools: []RegistryTool{
				&regToolWithArgs{name: "exists"},
				&regToolWithArgs{
					name: "new",
					params: map[string]any{
						"type":       "object",
						"properties": map[string]any{"x": map[string]any{"type": "string"}},
					},
				},
			},
		}
		rb.tools = reg
		out := rb.appendRegistryToolDefs(context.Background(), defs, included)
		require.Len(t, out, 2)
		assert.Equal(t, "new", out[1].Name)
		assert.Contains(t, out[1].Parameters, "properties")
	})
	t.Run("Should ignore registry list errors", func(t *testing.T) {
		rb.tools = &listableRegistry{err: errors.New("x")}
		out := rb.appendRegistryToolDefs(context.Background(), nil, map[string]struct{}{})
		assert.Nil(t, out)
	})
}

func TestNormalizeToolParameters_PreservesProvidedSchemaFields(t *testing.T) {
	in := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"foo": map[string]any{"type": "string"},
		},
	}
	out := normalizeToolParameters(in)
	props, ok := out["properties"].(map[string]any)
	require.True(t, ok)
	_, has := props["foo"]
	assert.True(t, has)
}
