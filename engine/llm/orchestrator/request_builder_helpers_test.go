package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/schema"
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

type schemaTool struct {
	name, desc string
	schema     *schema.Schema
}

func (s *schemaTool) Name() string                                 { return s.name }
func (s *schemaTool) Description() string                          { return s.desc }
func (s *schemaTool) Call(context.Context, string) (string, error) { return "{}", nil }
func (s *schemaTool) InputSchema() *schema.Schema                  { return s.schema }

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

	t.Run("Should derive parameters from schema when available", func(t *testing.T) {
		s := &schema.Schema{
			"type": "object",
			"properties": map[string]any{
				"dir": map[string]any{"type": "string"},
			},
			"required": []string{"dir"},
		}
		reg := &listableRegistry{
			tools: []RegistryTool{&schemaTool{name: "cp__list_files", schema: s}},
		}
		rb.tools = reg
		out := rb.appendRegistryToolDefs(context.Background(), nil, map[string]struct{}{})
		require.Len(t, out, 1)
		params := out[0].Parameters
		require.Contains(t, params, "properties")
		props := params["properties"].(map[string]any)
		assert.Contains(t, props, "dir")
		require.Contains(t, params, "required")
		req := params["required"].([]string)
		assert.Contains(t, req, "dir")
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

func TestRequestBuilder_ComputeJSONPreferences(t *testing.T) {
	rb := &requestBuilder{}
	schemaObj := &schema.Schema{"type": "object"}
	action := &agent.ActionConfig{ID: "structured", OutputSchema: schemaObj}
	request := Request{
		Agent:  &agent.Config{Model: agent.Model{Config: core.ProviderConfig{Provider: core.ProviderGoogle}}},
		Action: action,
	}

	ctx := context.Background()
	force := rb.computeJSONPreferences(ctx, llmadapter.DefaultOutputFormat(), request)
	assert.False(t, force)

	request.Agent.Model.Config.Provider = core.ProviderOpenAI
	force = rb.computeJSONPreferences(ctx, llmadapter.DefaultOutputFormat(), request)
	assert.True(t, force)

	format := llmadapter.NewJSONSchemaOutputFormat("structured", schemaObj, true)
	force = rb.computeJSONPreferences(ctx, format, request)
	assert.False(t, force)

	request.Action = nil
	force = rb.computeJSONPreferences(ctx, llmadapter.DefaultOutputFormat(), request)
	assert.False(t, force)
}
