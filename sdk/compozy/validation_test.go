package compozy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/test/helpers"
)

type logRecord struct {
	level string
	msg   string
}

type capturingLogger struct {
	records []logRecord
}

func newCapturingLogger() *capturingLogger {
	return &capturingLogger{records: make([]logRecord, 0)}
}

func (l *capturingLogger) add(level, msg string) {
	l.records = append(l.records, logRecord{level: level, msg: msg})
}

func (l *capturingLogger) Debug(msg string, _ ...any) { l.add("debug", msg) }
func (l *capturingLogger) Info(msg string, _ ...any)  { l.add("info", msg) }
func (l *capturingLogger) Warn(msg string, _ ...any)  { l.add("warn", msg) }
func (l *capturingLogger) Error(msg string, _ ...any) { l.add("error", msg) }
func (l *capturingLogger) With(...any) logger.Logger  { return l }

type validationHarness struct {
	ctx     context.Context
	comp    *Compozy
	project *project.Config
	alpha   *workflow.Config
	log     *capturingLogger
	store   resources.ResourceStore
}

func newValidationHarness(t *testing.T) *validationHarness {
	t.Helper()
	ctx := helpers.NewTestContext(t)
	log := newCapturingLogger()
	ctx = logger.ContextWithLogger(ctx, log)
	projectCfg, alpha := buildTestConfigs(t, ctx)
	configureProjectForValidation(t, projectCfg)
	configureWorkflowForValidation(alpha)
	store := resources.NewMemoryResourceStore()
	comp := &Compozy{
		store:         store,
		workflowByID:  map[string]*workflow.Config{alpha.ID: alpha},
		workflowOrder: []string{alpha.ID},
		project:       projectCfg,
	}
	return &validationHarness{ctx: ctx, comp: comp, project: projectCfg, alpha: alpha, log: log, store: store}
}

func configureProjectForValidation(t *testing.T, proj *project.Config) {
	proj.Tools = []tool.Config{{ID: "tool-shared"}}
	proj.Memories = []memory.Config{{
		Resource:  string(resources.ResourceMemory),
		ID:        "memory-shared",
		Type:      memcore.TokenBasedMemory,
		MaxTokens: 4000,
		Persistence: memcore.PersistenceConfig{
			Type: memcore.RedisPersistence,
			TTL:  "1h",
		},
	}}
	proj.Embedders = []knowledge.EmbedderConfig{{
		ID:       "embed-default",
		Provider: "test",
		Model:    "embedding",
		Config:   knowledge.EmbedderRuntimeConfig{Dimension: 1536},
	}}
	proj.VectorDBs = []knowledge.VectorDBConfig{{
		ID:   "vec-default",
		Type: knowledge.VectorDBTypeFilesystem,
		Config: knowledge.VectorDBConnConfig{
			Path:      t.TempDir(),
			Dimension: 1536,
		},
	}}
	proj.KnowledgeBases = []knowledge.BaseConfig{{
		ID:       "kb-shared",
		Embedder: "embed-default",
		VectorDB: "vec-default",
		Sources: []knowledge.SourceConfig{{
			Type:  knowledge.SourceTypeMarkdownGlob,
			Paths: []string{"docs/*.md"},
		}},
	}}
}

func configureWorkflowForValidation(wf *workflow.Config) {
	if len(wf.Agents) > 0 {
		agentCfg := &wf.Agents[0]
		agentCfg.Tools = []tool.Config{{ID: "tool-shared"}}
		agentCfg.Knowledge = []core.KnowledgeBinding{{ID: "kb-shared"}}
		agentCfg.Memory = []core.MemoryReference{{ID: "memory-shared"}}
	}
	if len(wf.Tasks) > 0 && len(wf.Agents) > 0 {
		wf.Tasks[0].Agent = &agent.Config{ID: wf.Agents[0].ID}
	}
}

func (h *validationHarness) alphaAgent() *agent.Config {
	if len(h.alpha.Agents) == 0 {
		return nil
	}
	return &h.alpha.Agents[0]
}

func (h *validationHarness) addWorkflow(wf *workflow.Config) {
	h.comp.workflowByID[wf.ID] = wf
	h.comp.workflowOrder = append(h.comp.workflowOrder, wf.ID)
}

func TestValidateAndLinkSuccess(t *testing.T) {
	t.Parallel()
	h := newValidationHarness(t)
	require.NoError(t, h.comp.validateAndLink(h.ctx, h.project))
	require.NotEmpty(t, h.log.records)
	last := h.log.records[len(h.log.records)-1]
	require.Equal(t, "info", last.level)
	require.Equal(t, "validation and linking complete", last.msg)
}

func TestValidateAndLinkMissingAgent(t *testing.T) {
	t.Parallel()
	h := newValidationHarness(t)
	h.alpha.Tasks[0].Agent = &agent.Config{ID: "ghost"}
	err := h.comp.validateAndLink(h.ctx, h.project)
	require.Error(t, err)
	require.Contains(t, err.Error(), "workflow."+h.alpha.ID+".tasks."+h.alpha.Tasks[0].ID+".agent")
}

func TestValidateAndLinkMissingTool(t *testing.T) {
	t.Parallel()
	h := newValidationHarness(t)
	ag := h.alphaAgent()
	ag.Tools = []tool.Config{{ID: "missing-tool"}}
	err := h.comp.validateAndLink(h.ctx, h.project)
	require.Error(t, err)
	require.Contains(t, err.Error(), "workflow."+h.alpha.ID+".agent."+ag.ID+".tool")
}

func TestValidateAndLinkMissingKnowledge(t *testing.T) {
	t.Parallel()
	h := newValidationHarness(t)
	ag := h.alphaAgent()
	ag.Knowledge = []core.KnowledgeBinding{{ID: "missing-kb"}}
	err := h.comp.validateAndLink(h.ctx, h.project)
	require.Error(t, err)
	require.Contains(t, err.Error(), "workflow."+h.alpha.ID+".agent."+ag.ID+".knowledge")
}

func TestValidateAndLinkMissingMemory(t *testing.T) {
	t.Parallel()
	h := newValidationHarness(t)
	ag := h.alphaAgent()
	ag.Memory = []core.MemoryReference{{ID: "missing-memory"}}
	err := h.comp.validateAndLink(h.ctx, h.project)
	require.Error(t, err)
	require.Contains(t, err.Error(), "workflow."+h.alpha.ID+".agent."+ag.ID+".memory")
}

func TestValidateAndLinkMissingEmbedder(t *testing.T) {
	t.Parallel()
	h := newValidationHarness(t)
	h.project.KnowledgeBases[0].Embedder = "missing-embedder"
	err := h.comp.validateAndLink(h.ctx, h.project)
	require.Error(t, err)
	require.Contains(t, err.Error(), "project.knowledge_base.kb-shared.embedder")
}

func TestValidateAndLinkMissingVectorDB(t *testing.T) {
	t.Parallel()
	h := newValidationHarness(t)
	h.project.KnowledgeBases[0].VectorDB = "missing-vector"
	err := h.comp.validateAndLink(h.ctx, h.project)
	require.Error(t, err)
	require.Contains(t, err.Error(), "project.knowledge_base.kb-shared.vector_db")
}

func TestValidateAndLinkCircularDependency(t *testing.T) {
	t.Parallel()
	h := newValidationHarness(t)
	beta := newCallWorkflowWorkflow("workflow-beta", h.alpha.ID)
	h.addWorkflow(beta)
	h.alpha.Tasks = []task.Config{newCallWorkflowTask("alpha-to-beta", beta.ID)}
	beta.Tasks = []task.Config{newCallWorkflowTask("beta-to-alpha", h.alpha.ID)}
	err := h.comp.validateAndLink(h.ctx, h.project)
	require.Error(t, err)
	require.Contains(t, err.Error(), "workflow dependency cycle")
}

func TestValidateAndLinkWorkflowOrdering(t *testing.T) {
	t.Parallel()
	h := newValidationHarness(t)
	beta := newCallWorkflowWorkflow("workflow-beta", h.alpha.ID)
	h.comp.workflowOrder = []string{beta.ID, h.alpha.ID}
	h.addWorkflow(beta)
	require.NoError(t, h.comp.validateAndLink(h.ctx, h.project))
	require.Equal(t, h.alpha.ID, h.comp.workflowOrder[0])
	require.Equal(t, beta.ID, h.comp.workflowOrder[1])
}

func TestValidateAndLinkHybridToolReference(t *testing.T) {
	t.Parallel()
	h := newValidationHarness(t)
	ag := h.alphaAgent()
	ag.Tools = []tool.Config{{ID: "yaml-tool"}}
	_, err := h.store.Put(
		h.ctx,
		resources.ResourceKey{Project: h.project.Name, Type: resources.ResourceTool, ID: "yaml-tool"},
		&tool.Config{ID: "yaml-tool"},
	)
	require.NoError(t, err)
	require.NoError(t, h.comp.validateAndLink(h.ctx, h.project))
}

func TestValidateAndLinkHybridWorkflowReference(t *testing.T) {
	t.Parallel()
	h := newValidationHarness(t)
	h.alpha.Tasks = []task.Config{newCallWorkflowTask("call-yaml", "yaml-flow")}
	_, err := h.store.Put(
		h.ctx,
		resources.ResourceKey{Project: h.project.Name, Type: resources.ResourceWorkflow, ID: "yaml-flow"},
		&workflow.Config{ID: "yaml-flow"},
	)
	require.NoError(t, err)
	require.NoError(t, h.comp.validateAndLink(h.ctx, h.project))
}

func newCallWorkflowTask(id string, target string) task.Config {
	with := core.Input{"workflow_id": target}
	return task.Config{
		BaseConfig: task.BaseConfig{
			ID:   id,
			Type: task.TaskTypeBasic,
			Tool: &tool.Config{ID: builtinCallWorkflow},
			With: &with,
		},
	}
}

func newCallWorkflowWorkflow(id string, target string) *workflow.Config {
	wf := &workflow.Config{ID: id}
	wf.Tasks = []task.Config{newCallWorkflowTask(id+"-task", target)}
	return wf
}
