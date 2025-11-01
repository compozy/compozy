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
	t.Run("Should return error when workflow id missing", func(t *testing.T) {
		engine := &Engine{ctx: t.Context()}
		store := newResourceStoreStub()
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
	})
	t.Run("Should persist workflow with nil context", func(t *testing.T) {
		engine := &Engine{ctx: t.Context()}
		store := newResourceStoreStub()
		var nilCtx context.Context
		err := engine.persistResource(
			nilCtx,
			store,
			"proj",
			resources.ResourceWorkflow,
			"wf",
			map[string]any{},
			registrationSourceProgrammatic,
		)
		assert.NoError(t, err)
	})
	t.Run("Should persist workflow with nil store", func(t *testing.T) {
		engine := &Engine{ctx: t.Context()}
		err := engine.persistResource(
			engine.ctx,
			nil,
			"proj",
			resources.ResourceWorkflow,
			"wf",
			map[string]any{},
			registrationSourceProgrammatic,
		)
		assert.NoError(t, err)
	})
}

func TestPersistResourceDetectsExistingResource(t *testing.T) {
	t.Run("Should detect existing resource", func(t *testing.T) {
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
	})
}

func TestPersistResourceHandlesStorePutErrors(t *testing.T) {
	t.Run("Should surface store put failures", func(t *testing.T) {
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
	})
}

func TestPersistResourceReportsMetaWriteFailure(t *testing.T) {
	t.Run("Should report metadata write failure", func(t *testing.T) {
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
	})
}

func TestRegisterProjectResetsStateOnPersistFailure(t *testing.T) {
	store := newResourceStoreStub()
	store.putErr = errors.New("persist failure")
	engine := &Engine{ctx: t.Context(), resourceStore: store}
	cfg := &engineproject.Config{Name: "helios"}
	err := engine.registerProject(cfg, registrationSourceProgrammatic)
	require.Error(t, err)
	engine.stateMu.RLock()
	defer engine.stateMu.RUnlock()
	assert.Nil(t, engine.project)
}

func TestRegisterResourceNilConfigValidation(t *testing.T) {
	tests := []struct {
		name string
		call func(*Engine) error
		want string
	}{
		{
			name: "return error when project config missing",
			call: func(engine *Engine) error {
				var cfg *engineproject.Config
				return engine.registerProject(cfg, registrationSourceProgrammatic)
			},
			want: "project config is required",
		},
		{
			name: "return error when workflow config missing",
			call: func(engine *Engine) error {
				var cfg *engineworkflow.Config
				return engine.registerWorkflow(cfg, registrationSourceProgrammatic)
			},
			want: "workflow config is required",
		},
		{
			name: "return error when agent config missing",
			call: func(engine *Engine) error {
				var cfg *engineagent.Config
				return engine.registerAgent(cfg, registrationSourceProgrammatic)
			},
			want: "agent config is required",
		},
		{
			name: "return error when tool config missing",
			call: func(engine *Engine) error {
				var cfg *enginetool.Config
				return engine.registerTool(cfg, registrationSourceProgrammatic)
			},
			want: "tool config is required",
		},
		{
			name: "return error when knowledge config missing",
			call: func(engine *Engine) error {
				var cfg *engineknowledge.BaseConfig
				return engine.registerKnowledge(cfg, registrationSourceProgrammatic)
			},
			want: "knowledge config is required",
		},
		{
			name: "return error when memory config missing",
			call: func(engine *Engine) error {
				var cfg *enginememory.Config
				return engine.registerMemory(cfg, registrationSourceProgrammatic)
			},
			want: "memory config is required",
		},
		{
			name: "return error when mcp config missing",
			call: func(engine *Engine) error {
				var cfg *enginemcp.Config
				return engine.registerMCP(cfg, registrationSourceProgrammatic)
			},
			want: "mcp config is required",
		},
		{
			name: "return error when schema config missing",
			call: func(engine *Engine) error {
				var cfg *engineschema.Schema
				return engine.registerSchema(cfg, registrationSourceProgrammatic)
			},
			want: "schema config is required",
		},
		{
			name: "return error when model config missing",
			call: func(engine *Engine) error {
				var cfg *enginecore.ProviderConfig
				return engine.registerModel(cfg, registrationSourceProgrammatic)
			},
			want: "model config is required",
		},
		{
			name: "return error when schedule config missing",
			call: func(engine *Engine) error {
				var cfg *projectschedule.Config
				return engine.registerSchedule(cfg, registrationSourceProgrammatic)
			},
			want: "schedule config is required",
		},
		{
			name: "return error when webhook config missing",
			call: func(engine *Engine) error {
				var cfg *enginewebhook.Config
				return engine.registerWebhook(cfg, registrationSourceProgrammatic)
			},
			want: "webhook config is required",
		},
	}
	for _, tc := range tests {
		caseEntry := tc
		t.Run("Should "+caseEntry.name, func(t *testing.T) {
			engine := &Engine{ctx: t.Context(), resourceStore: newResourceStoreStub()}
			err := caseEntry.call(engine)
			require.Error(t, err)
			assert.Contains(t, err.Error(), caseEntry.want)
		})
	}
}

func TestRegisterResourceEmptyIdentifier(t *testing.T) {
	tests := []struct {
		name string
		call func(*Engine) error
		want string
	}{
		{
			name: "return error when project name missing",
			call: func(engine *Engine) error {
				return engine.registerProject(&engineproject.Config{}, registrationSourceProgrammatic)
			},
			want: "project name is required",
		},
		{
			name: "return error when workflow id missing",
			call: func(engine *Engine) error {
				return engine.registerWorkflow(&engineworkflow.Config{}, registrationSourceProgrammatic)
			},
			want: "workflow id is required",
		},
		{
			name: "return error when agent id missing",
			call: func(engine *Engine) error {
				return engine.registerAgent(&engineagent.Config{}, registrationSourceProgrammatic)
			},
			want: "agent id is required",
		},
		{
			name: "return error when tool id missing",
			call: func(engine *Engine) error {
				return engine.registerTool(&enginetool.Config{}, registrationSourceProgrammatic)
			},
			want: "tool id is required",
		},
		{
			name: "return error when knowledge id missing",
			call: func(engine *Engine) error {
				return engine.registerKnowledge(&engineknowledge.BaseConfig{}, registrationSourceProgrammatic)
			},
			want: "knowledge base id is required",
		},
		{
			name: "return error when memory id missing",
			call: func(engine *Engine) error {
				return engine.registerMemory(&enginememory.Config{}, registrationSourceProgrammatic)
			},
			want: "memory id is required",
		},
		{
			name: "return error when mcp id missing",
			call: func(engine *Engine) error {
				return engine.registerMCP(&enginemcp.Config{}, registrationSourceProgrammatic)
			},
			want: "mcp id is required",
		},
		{
			name: "return error when schema id missing",
			call: func(engine *Engine) error {
				schema := engineschema.Schema{}
				return engine.registerSchema(&schema, registrationSourceProgrammatic)
			},
			want: "schema id is required",
		},
		{
			name: "return error when model identifier missing",
			call: func(engine *Engine) error {
				return engine.registerModel(&enginecore.ProviderConfig{}, registrationSourceProgrammatic)
			},
			want: "model identifier is required",
		},
		{
			name: "return error when schedule id missing",
			call: func(engine *Engine) error {
				return engine.registerSchedule(&projectschedule.Config{}, registrationSourceProgrammatic)
			},
			want: "schedule id is required",
		},
		{
			name: "return error when webhook slug missing",
			call: func(engine *Engine) error {
				return engine.registerWebhook(&enginewebhook.Config{}, registrationSourceProgrammatic)
			},
			want: "webhook slug is required",
		},
	}
	for _, tc := range tests {
		caseEntry := tc
		t.Run("Should "+caseEntry.name, func(t *testing.T) {
			engine := &Engine{ctx: t.Context(), resourceStore: newResourceStoreStub()}
			err := caseEntry.call(engine)
			require.Error(t, err)
			assert.Contains(t, err.Error(), caseEntry.want)
		})
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
