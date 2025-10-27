package compozy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
)

const sdkMetaSource = "sdk"

type registrationSummary struct {
	workflows      int
	agents         int
	tools          int
	knowledgeBases int
	memories       int
	mcps           int
	schemas        int
}

// loadProjectIntoEngine registers the project and associated workflows within
// the engine resource store so they are available for execution.
func (c *Compozy) loadProjectIntoEngine(ctx context.Context, proj *project.Config) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if proj == nil {
		return fmt.Errorf("project config is required")
	}
	log := logger.FromContext(ctx)
	log.Info("loading project into engine", "project", proj.Name)
	if err := c.RegisterProject(ctx, proj); err != nil {
		return err
	}
	order := c.snapshotWorkflowOrder()
	workflows := c.collectWorkflows(order)
	summary := registrationSummary{}
	if err := c.registerWorkflows(ctx, workflows, &summary); err != nil {
		return err
	}
	if err := c.registerWorkflowAgents(ctx, workflows, &summary); err != nil {
		return err
	}
	if err := c.registerProjectResources(ctx, proj, &summary); err != nil {
		return err
	}
	if err := c.registerWorkflowResources(ctx, workflows, &summary); err != nil {
		return err
	}
	log.Info(
		"project registered in engine",
		"project",
		proj.Name,
		"workflows",
		summary.workflows,
		"agents",
		summary.agents,
		"tools",
		summary.tools,
		"knowledge_bases",
		summary.knowledgeBases,
		"memories",
		summary.memories,
		"mcps",
		summary.mcps,
		"schemas",
		summary.schemas,
	)
	return nil
}

func (c *Compozy) snapshotWorkflowOrder() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]string(nil), c.workflowOrder...)
}

func (c *Compozy) registeredProjectName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.project == nil {
		return ""
	}
	return strings.TrimSpace(c.project.Name)
}

func (c *Compozy) requireStore() (resources.ResourceStore, error) {
	store := c.ResourceStore()
	if store == nil {
		return nil, fmt.Errorf("resource store is not configured")
	}
	return store, nil
}

func (c *Compozy) collectWorkflows(order []string) []*workflow.Config {
	result := make([]*workflow.Config, 0, len(order))
	c.mu.RLock()
	for _, id := range order {
		wf := c.workflowByID[id]
		if wf != nil {
			result = append(result, wf)
		}
	}
	c.mu.RUnlock()
	return result
}

func (c *Compozy) registerWorkflows(
	ctx context.Context,
	workflows []*workflow.Config,
	summary *registrationSummary,
) error {
	for _, wf := range workflows {
		if err := c.RegisterWorkflow(ctx, wf); err != nil {
			return err
		}
		summary.workflows++
	}
	return nil
}

func (c *Compozy) registerWorkflowAgents(
	ctx context.Context,
	workflows []*workflow.Config,
	summary *registrationSummary,
) error {
	for _, wf := range workflows {
		for i := range wf.Agents {
			if err := c.RegisterAgent(ctx, &wf.Agents[i]); err != nil {
				return err
			}
			summary.agents++
		}
	}
	return nil
}

func (c *Compozy) registerProjectResources(
	ctx context.Context,
	proj *project.Config,
	summary *registrationSummary,
) error {
	for i := range proj.Tools {
		if err := c.RegisterTool(ctx, &proj.Tools[i]); err != nil {
			return err
		}
		summary.tools++
	}
	for i := range proj.KnowledgeBases {
		if err := c.RegisterKnowledgeBase(ctx, &proj.KnowledgeBases[i]); err != nil {
			return err
		}
		summary.knowledgeBases++
	}
	for i := range proj.Memories {
		if err := c.RegisterMemory(ctx, proj.Memories[i]); err != nil {
			return err
		}
		summary.memories++
	}
	for i := range proj.MCPs {
		if err := c.RegisterMCP(ctx, &proj.MCPs[i]); err != nil {
			return err
		}
		summary.mcps++
	}
	for i := range proj.Schemas {
		if err := c.RegisterSchema(ctx, &proj.Schemas[i]); err != nil {
			return err
		}
		summary.schemas++
	}
	return nil
}

func (c *Compozy) registerWorkflowResources(
	ctx context.Context,
	workflows []*workflow.Config,
	summary *registrationSummary,
) error {
	for _, wf := range workflows {
		for i := range wf.Tools {
			if err := c.RegisterTool(ctx, &wf.Tools[i]); err != nil {
				return err
			}
			summary.tools++
		}
		for i := range wf.Schemas {
			if err := c.RegisterSchema(ctx, &wf.Schemas[i]); err != nil {
				return err
			}
			summary.schemas++
		}
	}
	return nil
}

func (c *Compozy) validateAndLink(ctx context.Context, proj *project.Config) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if proj == nil {
		return fmt.Errorf("project config is required")
	}
	log := logger.FromContext(ctx)
	store := c.ResourceStore()
	idx, err := buildResourceIndex(ctx, proj, c.workflowByID, store)
	if err != nil {
		log.Error("failed to build resource index", "error", err)
		return fmt.Errorf("build resource index: %w", err)
	}
	graph, err := c.ValidateReferences(ctx, proj, idx)
	if err != nil {
		log.Error("reference validation failed", "error", err)
		return fmt.Errorf("reference validation failed: %w", err)
	}
	if err := detectCircularDependencies(graph); err != nil {
		log.Error("circular dependency detected", "error", err)
		return fmt.Errorf("circular dependency detected: %w", err)
	}
	if err := c.validateDependencies(ctx, graph); err != nil {
		log.Error("dependency validation failed", "error", err)
		return fmt.Errorf("dependency validation failed: %w", err)
	}
	for _, unused := range idx.unused() {
		log.Warn(
			"unused resource definition",
			"resource",
			string(unused.kind),
			"id",
			unused.id,
			"source",
			unused.source,
		)
	}
	log.Info("validation and linking complete", "resources", idx.count())
	return nil
}

// RegisterProject validates and registers the provided project configuration in the resource store.
func (c *Compozy) RegisterProject(ctx context.Context, proj *project.Config) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if proj == nil {
		return fmt.Errorf("project config is required")
	}
	name := strings.TrimSpace(proj.Name)
	if name == "" {
		return fmt.Errorf("project name is required for registration")
	}
	store := c.ResourceStore()
	if store == nil {
		return fmt.Errorf("resource store is not configured")
	}
	if err := proj.Validate(ctx); err != nil {
		return fmt.Errorf("project %s validation failed: %w", name, err)
	}
	key := resources.ResourceKey{Project: name, Type: resources.ResourceProject, ID: name}
	if _, _, err := store.Get(ctx, key); err == nil {
		return fmt.Errorf("project %s already registered", name)
	} else if err != nil && !errors.Is(err, resources.ErrNotFound) {
		return fmt.Errorf("inspect project %s registration state: %w", name, err)
	}
	if _, err := store.Put(ctx, key, proj); err != nil {
		return fmt.Errorf("store project %s: %w", name, err)
	}
	if err := resources.WriteMeta(
		ctx,
		store,
		name,
		resources.ResourceProject,
		name,
		sdkMetaSource,
		"sdk",
	); err != nil {
		return fmt.Errorf("write project %s metadata: %w", name, err)
	}
	c.mu.Lock()
	c.project = proj
	c.mu.Unlock()
	logger.FromContext(ctx).Info("project registered", "project", name)
	return nil
}

// RegisterWorkflow validates and registers a workflow configuration in the resource store.
func (c *Compozy) RegisterWorkflow(ctx context.Context, wf *workflow.Config) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if wf == nil {
		return fmt.Errorf("workflow config is required")
	}
	store, err := c.requireStore()
	if err != nil {
		return err
	}
	projectName := c.registeredProjectName()
	if projectName == "" {
		return fmt.Errorf("project name is required for workflow registration")
	}
	id := strings.TrimSpace(wf.ID)
	if id == "" {
		return fmt.Errorf("workflow id is required for registration")
	}
	if err := wf.Validate(ctx); err != nil {
		return fmt.Errorf("workflow %s validation failed: %w", id, err)
	}
	if err := persistResource(ctx, store, projectName, resources.ResourceWorkflow, id, wf, "workflow"); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("workflow registered", "project", projectName, "workflow", id)
	return nil
}

// RegisterAgent validates and registers an agent configuration in the resource store.
func (c *Compozy) RegisterAgent(ctx context.Context, agentCfg *agent.Config) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if agentCfg == nil {
		return fmt.Errorf("agent config is required")
	}
	store, err := c.requireStore()
	if err != nil {
		return err
	}
	projectName := c.registeredProjectName()
	if projectName == "" {
		return fmt.Errorf("project name is required for agent registration")
	}
	id := strings.TrimSpace(agentCfg.ID)
	if id == "" {
		return fmt.Errorf("agent id is required for registration")
	}
	if err := agentCfg.Validate(ctx); err != nil {
		return fmt.Errorf("agent %s validation failed: %w", id, err)
	}
	if err := persistResource(ctx, store, projectName, resources.ResourceAgent, id, agentCfg, "agent"); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("agent registered", "project", projectName, "agent", id)
	return nil
}

// RegisterTool validates and registers a tool configuration in the resource store.
func (c *Compozy) RegisterTool(ctx context.Context, toolCfg *tool.Config) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if toolCfg == nil {
		return fmt.Errorf("tool config is required")
	}
	store, err := c.requireStore()
	if err != nil {
		return err
	}
	projectName := c.registeredProjectName()
	if projectName == "" {
		return fmt.Errorf("project name is required for tool registration")
	}
	id := strings.TrimSpace(toolCfg.ID)
	if id == "" {
		return fmt.Errorf("tool id is required for registration")
	}
	if err := toolCfg.Validate(ctx); err != nil {
		return fmt.Errorf("tool %s validation failed: %w", id, err)
	}
	if err := persistResource(ctx, store, projectName, resources.ResourceTool, id, toolCfg, "tool"); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("tool registered", "project", projectName, "tool", id)
	return nil
}

// RegisterSchema validates and registers a schema in the resource store.
func (c *Compozy) RegisterSchema(ctx context.Context, schemaCfg *schema.Schema) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if schemaCfg == nil {
		return fmt.Errorf("schema config is required")
	}
	store, err := c.requireStore()
	if err != nil {
		return err
	}
	projectName := c.registeredProjectName()
	if projectName == "" {
		return fmt.Errorf("project name is required for schema registration")
	}
	id := strings.TrimSpace(schema.GetID(schemaCfg))
	if id == "" {
		return fmt.Errorf("schema id is required for registration")
	}
	if _, err := schemaCfg.Compile(ctx); err != nil {
		return fmt.Errorf("schema %s validation failed: %w", id, err)
	}
	if err := persistResource(ctx, store, projectName, resources.ResourceSchema, id, schemaCfg, "schema"); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("schema registered", "project", projectName, "schema", id)
	return nil
}

// RegisterKnowledgeBase validates and registers a knowledge base configuration in the resource store.
func (c *Compozy) RegisterKnowledgeBase(ctx context.Context, kb *knowledge.BaseConfig) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if kb == nil {
		return fmt.Errorf("knowledge base config is required")
	}
	store := c.ResourceStore()
	if store == nil {
		return fmt.Errorf("resource store is not configured")
	}
	projectName := c.registeredProjectName()
	if projectName == "" {
		return fmt.Errorf("project name is required for knowledge base registration")
	}
	id, err := normalizeKnowledgeBase(kb)
	if err != nil {
		return err
	}
	defs, err := buildKnowledgeDefinitions(ctx, store, projectName, kb, id)
	if err != nil {
		return err
	}
	if err := defs.Validate(ctx); err != nil {
		return fmt.Errorf("knowledge base %s validation failed: %w", id, err)
	}
	if kb.Ingest == "" {
		kb.Ingest = knowledge.IngestManual
	}
	if err := persistKnowledgeBase(ctx, store, projectName, id, kb); err != nil {
		return err
	}
	logger.FromContext(ctx).Info(
		"knowledge base registered",
		"project",
		projectName,
		"knowledge_base",
		id,
		"embedder",
		kb.Embedder,
		"vector_db",
		kb.VectorDB,
	)
	return nil
}

func normalizeKnowledgeBase(kb *knowledge.BaseConfig) (string, error) {
	id := strings.TrimSpace(kb.ID)
	if id == "" {
		return "", fmt.Errorf("knowledge base id is required for registration")
	}
	kb.ID = id
	kb.Embedder = strings.TrimSpace(kb.Embedder)
	kb.VectorDB = strings.TrimSpace(kb.VectorDB)
	if kb.Embedder == "" {
		return "", fmt.Errorf("knowledge base %s requires embedder reference", id)
	}
	if kb.VectorDB == "" {
		return "", fmt.Errorf("knowledge base %s requires vector_db reference", id)
	}
	return id, nil
}

func buildKnowledgeDefinitions(
	ctx context.Context,
	store resources.ResourceStore,
	projectName string,
	kb *knowledge.BaseConfig,
	id string,
) (knowledge.Definitions, error) {
	embedderCfg, err := loadKnowledgeEmbedder(ctx, store, projectName, kb.Embedder)
	if err != nil {
		return knowledge.Definitions{}, fmt.Errorf("knowledge base %s: %w", id, err)
	}
	if embedderCfg == nil {
		return knowledge.Definitions{}, fmt.Errorf("knowledge base %s references missing embedder %s", id, kb.Embedder)
	}
	vectorCfg, err := loadKnowledgeVector(ctx, store, projectName, kb.VectorDB)
	if err != nil {
		return knowledge.Definitions{}, fmt.Errorf("knowledge base %s: %w", id, err)
	}
	if vectorCfg == nil {
		return knowledge.Definitions{}, fmt.Errorf("knowledge base %s references missing vector_db %s", id, kb.VectorDB)
	}
	kbCopy, err := core.DeepCopy(kb)
	if err != nil {
		return knowledge.Definitions{}, fmt.Errorf("knowledge base %s copy failed: %w", id, err)
	}
	embedderCopy, err := core.DeepCopy(embedderCfg)
	if err != nil {
		return knowledge.Definitions{}, fmt.Errorf("knowledge base %s embedder copy failed: %w", id, err)
	}
	vectorCopy, err := core.DeepCopy(vectorCfg)
	if err != nil {
		return knowledge.Definitions{}, fmt.Errorf("knowledge base %s vector copy failed: %w", id, err)
	}
	defs := knowledge.Definitions{
		Embedders:      []knowledge.EmbedderConfig{*embedderCopy},
		VectorDBs:      []knowledge.VectorDBConfig{*vectorCopy},
		KnowledgeBases: []knowledge.BaseConfig{*kbCopy},
	}
	defs.NormalizeWithDefaults(knowledge.DefaultDefaults())
	return defs, nil
}

func persistKnowledgeBase(
	ctx context.Context,
	store resources.ResourceStore,
	projectName string,
	id string,
	kb *knowledge.BaseConfig,
) error {
	return persistResource(ctx, store, projectName, resources.ResourceKnowledgeBase, id, kb, "knowledge base")
}

// RegisterMemory validates and registers a memory configuration in the resource store.
func (c *Compozy) RegisterMemory(ctx context.Context, memCfg *memory.Config) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if memCfg == nil {
		return fmt.Errorf("memory config is required")
	}
	store := c.ResourceStore()
	if store == nil {
		return fmt.Errorf("resource store is not configured")
	}
	projectName := c.registeredProjectName()
	if projectName == "" {
		return fmt.Errorf("project name is required for memory registration")
	}
	id, err := normalizeMemoryConfig(memCfg)
	if err != nil {
		return err
	}
	if err := memCfg.Validate(ctx); err != nil {
		return fmt.Errorf("memory %s validation failed: %w", id, err)
	}
	if err := persistMemory(ctx, store, projectName, id, memCfg); err != nil {
		return err
	}
	logger.FromContext(ctx).Info(
		"memory registered",
		"project",
		projectName,
		"memory",
		id,
		"persistence",
		string(memCfg.Persistence.Type),
	)
	return nil
}

func normalizeMemoryConfig(memCfg *memory.Config) (string, error) {
	id := strings.TrimSpace(memCfg.ID)
	if id == "" {
		return "", fmt.Errorf("memory id is required for registration")
	}
	memCfg.ID = id
	if strings.TrimSpace(memCfg.Resource) == "" {
		memCfg.Resource = string(resources.ResourceMemory)
	}
	return id, nil
}

func persistMemory(
	ctx context.Context,
	store resources.ResourceStore,
	projectName string,
	id string,
	memCfg *memory.Config,
) error {
	return persistResource(ctx, store, projectName, resources.ResourceMemory, id, memCfg, "memory")
}

// RegisterMCP validates and registers an MCP server configuration in the resource store.
func (c *Compozy) RegisterMCP(ctx context.Context, mcpCfg *mcp.Config) error {
	if c == nil {
		return fmt.Errorf("compozy instance is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if mcpCfg == nil {
		return fmt.Errorf("mcp config is required")
	}
	store := c.ResourceStore()
	if store == nil {
		return fmt.Errorf("resource store is not configured")
	}
	projectName := c.registeredProjectName()
	if projectName == "" {
		return fmt.Errorf("project name is required for mcp registration")
	}
	id, err := normalizeMCPConfig(mcpCfg)
	if err != nil {
		return err
	}
	if err := mcpCfg.Validate(ctx); err != nil {
		return fmt.Errorf("mcp %s validation failed: %w", id, err)
	}
	if err := persistMCP(ctx, store, projectName, id, mcpCfg); err != nil {
		return err
	}
	logger.FromContext(ctx).Info(
		"mcp registered",
		"project",
		projectName,
		"mcp",
		id,
		"transport",
		string(mcpCfg.Transport),
	)
	return nil
}

func normalizeMCPConfig(mcpCfg *mcp.Config) (string, error) {
	id := strings.TrimSpace(mcpCfg.ID)
	if id == "" {
		return "", fmt.Errorf("mcp id is required for registration")
	}
	mcpCfg.ID = id
	return id, nil
}

func persistMCP(
	ctx context.Context,
	store resources.ResourceStore,
	projectName string,
	id string,
	mcpCfg *mcp.Config,
) error {
	return persistResource(ctx, store, projectName, resources.ResourceMCP, id, mcpCfg, "mcp")
}

func loadKnowledgeEmbedder(
	ctx context.Context,
	store resources.ResourceStore,
	projectName string,
	embedderID string,
) (*knowledge.EmbedderConfig, error) {
	key := resources.ResourceKey{Project: projectName, Type: resources.ResourceEmbedder, ID: embedderID}
	value, _, err := store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load embedder %s: %w", embedderID, err)
	}
	switch typed := value.(type) {
	case *knowledge.EmbedderConfig:
		return typed, nil
	case knowledge.EmbedderConfig:
		return &typed, nil
	default:
		return nil, fmt.Errorf("embedder %s has unexpected type %T", embedderID, value)
	}
}

func persistResource(
	ctx context.Context,
	store resources.ResourceStore,
	projectName string,
	resourceType resources.ResourceType,
	id string,
	value any,
	resourceName string,
) error {
	key := resources.ResourceKey{Project: projectName, Type: resourceType, ID: id}
	if _, _, err := store.Get(ctx, key); err == nil {
		return fmt.Errorf("%s %s already registered", resourceName, id)
	} else if err != nil && !errors.Is(err, resources.ErrNotFound) {
		return fmt.Errorf("inspect %s %s registration state: %w", resourceName, id, err)
	}
	if _, err := store.Put(ctx, key, value); err != nil {
		return fmt.Errorf("store %s %s: %w", resourceName, id, err)
	}
	if err := resources.WriteMeta(ctx, store, projectName, resourceType, id, sdkMetaSource, "sdk"); err != nil {
		return fmt.Errorf("write %s %s metadata: %w", resourceName, id, err)
	}
	return nil
}

func loadKnowledgeVector(
	ctx context.Context,
	store resources.ResourceStore,
	projectName string,
	vectorID string,
) (*knowledge.VectorDBConfig, error) {
	key := resources.ResourceKey{Project: projectName, Type: resources.ResourceVectorDB, ID: vectorID}
	value, _, err := store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load vector_db %s: %w", vectorID, err)
	}
	switch typed := value.(type) {
	case *knowledge.VectorDBConfig:
		return typed, nil
	case knowledge.VectorDBConfig:
		return &typed, nil
	default:
		return nil, fmt.Errorf("vector_db %s has unexpected type %T", vectorID, value)
	}
}
