package compozy

import (
	"context"
	"errors"
	"testing"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	enginemcp "github.com/compozy/compozy/engine/mcp"
	enginememory "github.com/compozy/compozy/engine/memory"
	engineproject "github.com/compozy/compozy/engine/project"
	projectschedule "github.com/compozy/compozy/engine/project/schedule"
	"github.com/compozy/compozy/engine/resources"
	engineschema "github.com/compozy/compozy/engine/schema"
	enginetool "github.com/compozy/compozy/engine/tool"
	enginewebhook "github.com/compozy/compozy/engine/webhook"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type resourceStoreStub struct {
	items    map[resources.ResourceKey]any
	putErr   error
	getErr   error
	metaErr  bool
	closeErr error
}

func newResourceStoreStub() *resourceStoreStub {
	return &resourceStoreStub{items: make(map[resources.ResourceKey]any)}
}

func (s *resourceStoreStub) Put(_ context.Context, key resources.ResourceKey, value any) (resources.ETag, error) {
	if s.metaErr && key.Type == resources.ResourceMeta {
		return "", errors.New("meta failure")
	}
	if s.putErr != nil && key.Type != resources.ResourceMeta {
		return "", s.putErr
	}
	s.items[key] = value
	return "", nil
}

func (s *resourceStoreStub) PutIfMatch(
	_ context.Context,
	_ resources.ResourceKey,
	_ any,
	_ resources.ETag,
) (resources.ETag, error) {
	return "", nil
}

func (s *resourceStoreStub) Get(_ context.Context, key resources.ResourceKey) (any, resources.ETag, error) {
	if s.getErr != nil && key.Type != resources.ResourceMeta {
		return nil, "", s.getErr
	}
	value, ok := s.items[key]
	if !ok {
		return nil, "", resources.ErrNotFound
	}
	return value, "", nil
}

func (s *resourceStoreStub) Delete(context.Context, resources.ResourceKey) error {
	return nil
}

func (s *resourceStoreStub) List(context.Context, string, resources.ResourceType) ([]resources.ResourceKey, error) {
	return nil, nil
}

func (s *resourceStoreStub) Watch(context.Context, string, resources.ResourceType) (<-chan resources.Event, error) {
	return nil, nil
}

func (s *resourceStoreStub) ListWithValues(
	context.Context,
	string,
	resources.ResourceType,
) ([]resources.StoredItem, error) {
	return nil, nil
}

func (s *resourceStoreStub) ListWithValuesPage(
	context.Context,
	string,
	resources.ResourceType,
	int,
	int,
) ([]resources.StoredItem, int, error) {
	return nil, 0, nil
}

func (s *resourceStoreStub) Close() error {
	return s.closeErr
}

func TestPersistResourceRequiresIdentifiers(t *testing.T) {
	store := newResourceStoreStub()
	engine := &Engine{ctx: t.Context()}
	var nilCtx context.Context
	err := engine.persistResource(
		engine.ctx,
		store,
		"proj",
		resources.ResourceWorkflow,
		"",
		map[string]any{},
		registrationSourceProgrammatic,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow id is required")
	assert.NoError(
		t,
		engine.persistResource(
			nilCtx,
			store,
			"proj",
			resources.ResourceWorkflow,
			"wf",
			map[string]any{},
			registrationSourceProgrammatic,
		),
	)
	assert.NoError(
		t,
		engine.persistResource(
			engine.ctx,
			nil,
			"proj",
			resources.ResourceWorkflow,
			"wf",
			map[string]any{},
			registrationSourceProgrammatic,
		),
	)
}

func TestPersistResourceDetectsExistingResource(t *testing.T) {
	store := newResourceStoreStub()
	ctx := t.Context()
	key := resources.ResourceKey{Project: "proj", Type: resources.ResourceWorkflow, ID: "wf"}
	store.items[key] = map[string]any{"id": "wf"}
	engine := &Engine{ctx: ctx}
	err := engine.persistResource(
		ctx,
		store,
		"proj",
		resources.ResourceWorkflow,
		"wf",
		map[string]any{},
		registrationSourceProgrammatic,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestPersistResourceHandlesStorePutErrors(t *testing.T) {
	store := newResourceStoreStub()
	store.putErr = errors.New("store failure")
	engine := &Engine{ctx: t.Context()}
	err := engine.persistResource(
		engine.ctx,
		store,
		"proj",
		resources.ResourceWorkflow,
		"wf",
		map[string]any{},
		registrationSourceProgrammatic,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "store workflow wf")
}

func TestPersistResourceReportsMetaWriteFailure(t *testing.T) {
	store := newResourceStoreStub()
	store.metaErr = true
	engine := &Engine{ctx: t.Context()}
	err := engine.persistResource(
		engine.ctx,
		store,
		"proj",
		resources.ResourceWorkflow,
		"wf",
		map[string]any{},
		registrationSourceProgrammatic,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write workflow wf metadata")
}

func TestRegisterResourceNilConfigValidation(t *testing.T) {
	engine := &Engine{ctx: t.Context(), resourceStore: newResourceStoreStub()}
	tests := []struct {
		name string
		call func() error
		want string
	}{
		{
			"Project",
			func() error {
				var cfg *engineproject.Config
				return engine.registerProject(cfg, registrationSourceProgrammatic)
			},
			"project config is required",
		},
		{
			"Workflow",
			func() error {
				var cfg *engineworkflow.Config
				return engine.registerWorkflow(cfg, registrationSourceProgrammatic)
			},
			"workflow config is required",
		},
		{
			"Agent",
			func() error {
				var cfg *engineagent.Config
				return engine.registerAgent(cfg, registrationSourceProgrammatic)
			},
			"agent config is required",
		},
		{
			"Tool",
			func() error {
				var cfg *enginetool.Config
				return engine.registerTool(cfg, registrationSourceProgrammatic)
			},
			"tool config is required",
		},
		{
			"Knowledge",
			func() error {
				var cfg *engineknowledge.BaseConfig
				return engine.registerKnowledge(cfg, registrationSourceProgrammatic)
			},
			"knowledge config is required",
		},
		{
			"Memory",
			func() error {
				var cfg *enginememory.Config
				return engine.registerMemory(cfg, registrationSourceProgrammatic)
			},
			"memory config is required",
		},
		{
			"MCP",
			func() error {
				var cfg *enginemcp.Config
				return engine.registerMCP(cfg, registrationSourceProgrammatic)
			},
			"mcp config is required",
		},
		{
			"Schema",
			func() error {
				var cfg *engineschema.Schema
				return engine.registerSchema(cfg, registrationSourceProgrammatic)
			},
			"schema config is required",
		},
		{
			"Model",
			func() error {
				var cfg *enginecore.ProviderConfig
				return engine.registerModel(cfg, registrationSourceProgrammatic)
			},
			"model config is required",
		},
		{
			"Schedule",
			func() error {
				var cfg *projectschedule.Config
				return engine.registerSchedule(cfg, registrationSourceProgrammatic)
			},
			"schedule config is required",
		},
		{
			"Webhook",
			func() error {
				var cfg *enginewebhook.Config
				return engine.registerWebhook(cfg, registrationSourceProgrammatic)
			},
			"webhook config is required",
		},
	}
	for _, tc := range tests {
		err := tc.call()
		require.Error(t, err)
		assert.Contains(t, err.Error(), tc.want)
	}
}

func TestRegisterResourceEmptyIdentifier(t *testing.T) {
	engine := &Engine{ctx: t.Context(), resourceStore: newResourceStoreStub()}
	schema := engineschema.Schema{}
	tests := []struct {
		name string
		call func() error
		want string
	}{
		{
			"Project",
			func() error { return engine.registerProject(&engineproject.Config{}, registrationSourceProgrammatic) },
			"project name is required",
		},
		{
			"Workflow",
			func() error { return engine.registerWorkflow(&engineworkflow.Config{}, registrationSourceProgrammatic) },
			"workflow id is required",
		},
		{
			"Agent",
			func() error { return engine.registerAgent(&engineagent.Config{}, registrationSourceProgrammatic) },
			"agent id is required",
		},
		{
			"Tool",
			func() error { return engine.registerTool(&enginetool.Config{}, registrationSourceProgrammatic) },
			"tool id is required",
		},
		{"Knowledge", func() error {
			return engine.registerKnowledge(&engineknowledge.BaseConfig{}, registrationSourceProgrammatic)
		}, "knowledge base id is required"},
		{
			"Memory",
			func() error { return engine.registerMemory(&enginememory.Config{}, registrationSourceProgrammatic) },
			"memory id is required",
		},
		{
			"MCP",
			func() error { return engine.registerMCP(&enginemcp.Config{}, registrationSourceProgrammatic) },
			"mcp id is required",
		},
		{
			"Schema",
			func() error { return engine.registerSchema(&schema, registrationSourceProgrammatic) },
			"schema id is required",
		},
		{"Model", func() error {
			return engine.registerModel(&enginecore.ProviderConfig{}, registrationSourceProgrammatic)
		}, "model identifier is required"},
		{"Schedule", func() error {
			return engine.registerSchedule(&projectschedule.Config{}, registrationSourceProgrammatic)
		}, "schedule id is required"},
		{
			"Webhook",
			func() error { return engine.registerWebhook(&enginewebhook.Config{}, registrationSourceProgrammatic) },
			"webhook slug is required",
		},
	}
	for _, tc := range tests {
		err := tc.call()
		require.Error(t, err)
		assert.Contains(t, err.Error(), tc.want)
	}
}

func TestRegisterResourceDuplicateDetection(t *testing.T) {
	ctx := lifecycleTestContext(t)
	engine := &Engine{ctx: ctx, resourceStore: newResourceStoreStub()}
	require.NoError(t, engine.registerProject(&engineproject.Config{Name: "dup"}, registrationSourceProgrammatic))
	require.NoError(t, engine.registerWorkflow(&engineworkflow.Config{ID: "wf"}, registrationSourceProgrammatic))
	require.NoError(t, engine.registerTool(&enginetool.Config{ID: "tool"}, registrationSourceProgrammatic))
	require.NoError(t, engine.registerKnowledge(&engineknowledge.BaseConfig{ID: "kb"}, registrationSourceProgrammatic))
	require.NoError(t, engine.registerMemory(&enginememory.Config{ID: "mem"}, registrationSourceProgrammatic))
	require.NoError(t, engine.registerMCP(&enginemcp.Config{ID: "mcp"}, registrationSourceProgrammatic))
	schema := engineschema.Schema{"id": "schema-1", "type": "object"}
	require.NoError(t, engine.registerSchema(&schema, registrationSourceProgrammatic))
	require.NoError(
		t,
		engine.registerModel(
			&enginecore.ProviderConfig{Provider: enginecore.ProviderName("openai"), Model: "gpt"},
			registrationSourceProgrammatic,
		),
	)
	require.NoError(t, engine.registerSchedule(&projectschedule.Config{ID: "schedule"}, registrationSourceProgrammatic))
	require.NoError(t, engine.registerWebhook(&enginewebhook.Config{Slug: "hook"}, registrationSourceProgrammatic))
	assert.Error(t, engine.registerProject(&engineproject.Config{Name: "dup"}, registrationSourceProgrammatic))
	assert.Error(t, engine.registerWorkflow(&engineworkflow.Config{ID: "wf"}, registrationSourceProgrammatic))
	assert.Error(t, engine.registerTool(&enginetool.Config{ID: "tool"}, registrationSourceProgrammatic))
	assert.Error(t, engine.registerKnowledge(&engineknowledge.BaseConfig{ID: "kb"}, registrationSourceProgrammatic))
	assert.Error(t, engine.registerMemory(&enginememory.Config{ID: "mem"}, registrationSourceProgrammatic))
	assert.Error(t, engine.registerMCP(&enginemcp.Config{ID: "mcp"}, registrationSourceProgrammatic))
	assert.Error(t, engine.registerSchema(&schema, registrationSourceProgrammatic))
	assert.Error(
		t,
		engine.registerModel(
			&enginecore.ProviderConfig{Provider: enginecore.ProviderName("openai"), Model: "gpt"},
			registrationSourceProgrammatic,
		),
	)
	assert.Error(t, engine.registerSchedule(&projectschedule.Config{ID: "schedule"}, registrationSourceProgrammatic))
	assert.Error(t, engine.registerWebhook(&enginewebhook.Config{Slug: "hook"}, registrationSourceProgrammatic))
}
