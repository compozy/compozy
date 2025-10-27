package compozy

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

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
)

type resourceKind string

const (
	kindWorkflow      resourceKind = "workflow"
	kindAgent         resourceKind = "agent"
	kindTool          resourceKind = "tool"
	kindKnowledgeBase resourceKind = "knowledge_base"
	kindMemory        resourceKind = "memory"
	kindEmbedder      resourceKind = "embedder"
	kindVectorDB      resourceKind = "vector_db"
)

const (
	builtinCallWorkflow  = "cp__call_workflow"
	builtinCallWorkflows = "cp__call_workflows"
)

type resourceInfo struct {
	source     string
	referenced bool
	external   bool
}

type resourceIndex struct {
	projectName string
	buckets     map[resourceKind]map[string]*resourceInfo
}

type unusedEntry struct {
	kind   resourceKind
	id     string
	source string
}

type dependencyGraph map[string]map[string]struct{}

func newResourceIndex(projectName string) *resourceIndex {
	return &resourceIndex{
		projectName: projectName,
		buckets:     make(map[resourceKind]map[string]*resourceInfo),
	}
}

func (idx *resourceIndex) bucket(kind resourceKind) map[string]*resourceInfo {
	if _, ok := idx.buckets[kind]; !ok {
		idx.buckets[kind] = make(map[string]*resourceInfo)
	}
	return idx.buckets[kind]
}

func (idx *resourceIndex) add(kind resourceKind, id string, source string, external bool) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return
	}
	bucket := idx.bucket(kind)
	if existing, ok := bucket[trimmed]; ok {
		if existing.external && !external {
			existing.external = false
			existing.source = source
		}
		return
	}
	bucket[trimmed] = &resourceInfo{source: source, external: external}
}

func (idx *resourceIndex) mark(kind resourceKind, id string) bool {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return false
	}
	info, ok := idx.bucket(kind)[trimmed]
	if !ok {
		return false
	}
	info.referenced = true
	return true
}

func (idx *resourceIndex) info(kind resourceKind, id string) (*resourceInfo, bool) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil, false
	}
	info, ok := idx.bucket(kind)[trimmed]
	return info, ok
}

func (idx *resourceIndex) count() int {
	total := 0
	for _, bucket := range idx.buckets {
		for _, info := range bucket {
			if info.external {
				continue
			}
			total++
		}
	}
	return total
}

func (idx *resourceIndex) unused() []unusedEntry {
	unused := make([]unusedEntry, 0)
	for kind, bucket := range idx.buckets {
		for id, info := range bucket {
			if info.external || info.referenced {
				continue
			}
			unused = append(unused, unusedEntry{kind: kind, id: id, source: info.source})
		}
	}
	return unused
}

func buildResourceIndex(
	ctx context.Context,
	proj *project.Config,
	workflows map[string]*workflow.Config,
	store resources.ResourceStore,
) (*resourceIndex, error) {
	idx := newResourceIndex(proj.Name)
	addProjectResources(idx, proj)
	addWorkflowResources(idx, workflows)
	if store != nil {
		if err := addStoreResources(ctx, idx, proj.Name, store); err != nil {
			return nil, err
		}
	}
	return idx, nil
}

func addProjectResources(idx *resourceIndex, proj *project.Config) {
	for _, toolCfg := range proj.Tools {
		idx.add(kindTool, toolCfg.ID, "project.tool", false)
	}
	for _, mem := range proj.Memories {
		ensureMemoryDefaults(&mem)
		idx.add(kindMemory, mem.ID, "project.memory", false)
	}
	for _, kb := range proj.KnowledgeBases {
		idx.add(kindKnowledgeBase, kb.ID, "project.knowledge_base", false)
	}
	for _, embedder := range proj.Embedders {
		idx.add(kindEmbedder, embedder.ID, "project.embedder", false)
	}
	for _, vector := range proj.VectorDBs {
		idx.add(kindVectorDB, vector.ID, "project.vector_db", false)
	}
}

func addWorkflowResources(idx *resourceIndex, workflows map[string]*workflow.Config) {
	for id, wf := range workflows {
		if wf == nil {
			continue
		}
		idx.add(kindWorkflow, id, "sdk.workflow", false)
		for _, agentCfg := range wf.Agents {
			idx.add(kindAgent, agentCfg.ID, "workflow.agent", false)
		}
		for _, toolCfg := range wf.Tools {
			idx.add(kindTool, toolCfg.ID, "workflow.tool", false)
		}
		for _, kb := range wf.KnowledgeBases {
			idx.add(kindKnowledgeBase, kb.ID, "workflow.knowledge_base", false)
		}
	}
}

func addStoreResources(
	ctx context.Context,
	idx *resourceIndex,
	projectName string,
	store resources.ResourceStore,
) error {
	types := []struct {
		typ   resources.ResourceType
		kind  resourceKind
		label string
	}{
		{resources.ResourceWorkflow, kindWorkflow, "store.workflow"},
		{resources.ResourceAgent, kindAgent, "store.agent"},
		{resources.ResourceTool, kindTool, "store.tool"},
		{resources.ResourceKnowledgeBase, kindKnowledgeBase, "store.knowledge_base"},
		{resources.ResourceMemory, kindMemory, "store.memory"},
		{resources.ResourceEmbedder, kindEmbedder, "store.embedder"},
		{resources.ResourceVectorDB, kindVectorDB, "store.vector_db"},
	}
	for _, entry := range types {
		keys, err := store.List(ctx, projectName, entry.typ)
		if err != nil {
			return fmt.Errorf("list %s resources: %w", entry.typ, err)
		}
		for _, key := range keys {
			idx.add(entry.kind, key.ID, entry.label, true)
		}
	}
	return nil
}

func (c *Compozy) ValidateReferences(
	ctx context.Context,
	proj *project.Config,
	idx *resourceIndex,
) (dependencyGraph, error) {
	graph := createDependencyGraph(c.workflowByID)
	errs := make([]error, 0)
	errs = append(errs, validateKnowledgeBindings("project.knowledge", proj.Knowledge, idx)...)
	for _, kb := range proj.KnowledgeBases {
		errs = append(errs, validateKnowledgeBase(idx, kb, fmt.Sprintf("project.knowledge_base.%s", kb.ID))...)
	}
	for _, toolCfg := range proj.Tools {
		errs = append(errs, validateToolReference(idx, graph, proj.Name, "project.tool", &toolCfg, toolCfg.With)...)
	}
	for wfID, wfCfg := range c.workflowByID {
		if wfCfg == nil {
			continue
		}
		wfPath := fmt.Sprintf("workflow.%s", wfID)
		errs = append(errs, validateWorkflow(idx, graph, wfID, wfCfg, wfPath)...)
	}
	if len(errs) > 0 {
		return graph, errors.Join(errs...)
	}
	return graph, nil
}

func validateWorkflow(
	idx *resourceIndex,
	graph dependencyGraph,
	wfID string,
	wfCfg *workflow.Config,
	wfPath string,
) []error {
	errs := make([]error, 0)
	errs = append(errs, validateKnowledgeBindings(wfPath+".knowledge", wfCfg.Knowledge, idx)...)
	for _, kb := range wfCfg.KnowledgeBases {
		errs = append(errs, validateKnowledgeBase(idx, kb, fmt.Sprintf("%s.knowledge_base.%s", wfPath, kb.ID))...)
	}
	for _, agentCfg := range wfCfg.Agents {
		errs = append(
			errs,
			validateAgent(idx, graph, wfID, &agentCfg, fmt.Sprintf("%s.agent.%s", wfPath, agentCfg.ID))...)
	}
	for _, toolCfg := range wfCfg.Tools {
		errs = append(errs, validateToolReference(idx, graph, wfID, wfPath+".tool", &toolCfg, toolCfg.With)...)
	}
	errs = append(errs, validateTasks(idx, graph, wfID, wfCfg.Tasks, wfPath+".tasks")...)
	return errs
}

func validateTasks(
	idx *resourceIndex,
	graph dependencyGraph,
	wfID string,
	tasks []task.Config,
	basePath string,
) []error {
	errs := make([]error, 0)
	for i := range tasks {
		cfg := &tasks[i]
		path := taskPath(basePath, cfg.ID, i)
		errs = append(errs, validateTask(idx, graph, wfID, cfg, path)...)
	}
	return errs
}

func validateTask(
	idx *resourceIndex,
	graph dependencyGraph,
	wfID string,
	cfg *task.Config,
	path string,
) []error {
	errs := make([]error, 0)
	if cfg.Agent != nil && isAgentReference(cfg.Agent) {
		if !idx.mark(kindAgent, cfg.Agent.ID) {
			errs = append(errs, fmt.Errorf("%s.agent references missing agent %q", path, cfg.Agent.ID))
		}
	}
	if cfg.Tool != nil {
		errs = append(errs, validateToolReference(idx, graph, wfID, path+".tool", cfg.Tool, cfg.With)...)
	}
	errs = append(errs, validateKnowledgeBindings(path+".knowledge", cfg.Knowledge, idx)...)
	if strings.TrimSpace(cfg.MemoryRef) != "" {
		if !idx.mark(kindMemory, cfg.MemoryRef) {
			errs = append(errs, fmt.Errorf("%s.memory_ref references missing memory %q", path, cfg.MemoryRef))
		}
	}
	if len(cfg.Tasks) > 0 {
		errs = append(errs, validateTasks(idx, graph, wfID, cfg.Tasks, path+".tasks")...)
	}
	if cfg.Task != nil {
		errs = append(errs, validateTask(idx, graph, wfID, cfg.Task, path+".task")...)
	}
	return errs
}

func validateAgent(
	idx *resourceIndex,
	graph dependencyGraph,
	wfID string,
	cfg *agent.Config,
	basePath string,
) []error {
	errs := make([]error, 0)
	errs = append(errs, validateKnowledgeBindings(basePath+".knowledge", cfg.Knowledge, idx)...)
	for _, mem := range cfg.Memory {
		if !idx.mark(kindMemory, mem.ID) {
			errs = append(errs, fmt.Errorf("%s.memory references missing memory %q", basePath, mem.ID))
		}
	}
	for _, toolCfg := range cfg.Tools {
		errs = append(errs, validateToolReference(idx, graph, wfID, basePath+".tool", &toolCfg, cfg.With)...)
	}
	for _, action := range cfg.Actions {
		actionPath := fmt.Sprintf("%s.action.%s", basePath, action.ID)
		for _, toolCfg := range action.Tools {
			errs = append(errs, validateToolReference(idx, graph, wfID, actionPath+".tool", &toolCfg, action.With)...)
		}
	}
	return errs
}

func validateKnowledgeBindings(
	path string,
	bindings []core.KnowledgeBinding,
	idx *resourceIndex,
) []error {
	errs := make([]error, 0)
	for _, binding := range bindings {
		if !idx.mark(kindKnowledgeBase, binding.ID) {
			errs = append(errs, fmt.Errorf("%s references missing knowledge base %q", path, binding.ID))
		}
	}
	return errs
}

func validateKnowledgeBase(
	idx *resourceIndex,
	kb knowledge.BaseConfig,
	path string,
) []error {
	errs := make([]error, 0)
	if kb.Embedder != "" && !idx.mark(kindEmbedder, kb.Embedder) {
		errs = append(errs, fmt.Errorf("%s.embedder references missing embedder %q", path, kb.Embedder))
	}
	if kb.VectorDB != "" && !idx.mark(kindVectorDB, kb.VectorDB) {
		errs = append(errs, fmt.Errorf("%s.vector_db references missing vector_db %q", path, kb.VectorDB))
	}
	return errs
}

func validateToolReference(
	idx *resourceIndex,
	graph dependencyGraph,
	wfID string,
	path string,
	cfg *tool.Config,
	input *core.Input,
) []error {
	errs := make([]error, 0)
	if cfg == nil {
		return errs
	}
	toolID := strings.TrimSpace(cfg.ID)
	if toolID != "" && isToolReference(cfg) && !isBuiltinTool(toolID) {
		if !idx.mark(kindTool, toolID) {
			errs = append(errs, fmt.Errorf("%s references missing tool %q", path, toolID))
		}
	}
	for _, dep := range collectWorkflowDependencies(toolID, input) {
		if info, ok := idx.info(kindWorkflow, dep); !ok {
			errs = append(errs, fmt.Errorf("%s references missing workflow %q", path, dep))
		} else {
			info.referenced = true
			if !info.external {
				addWorkflowDependency(graph, wfID, dep)
			}
		}
	}
	return errs
}

func createDependencyGraph(workflows map[string]*workflow.Config) dependencyGraph {
	graph := make(dependencyGraph)
	for id := range workflows {
		graph[id] = make(map[string]struct{})
	}
	return graph
}

func addWorkflowDependency(graph dependencyGraph, from string, to string) {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return
	}
	if _, ok := graph[from]; !ok {
		graph[from] = make(map[string]struct{})
	}
	graph[from][to] = struct{}{}
}

func detectCircularDependencies(graph dependencyGraph) error {
	const (
		stateUnvisited = iota
		stateVisiting
		stateVisited
	)
	state := make(map[string]int)
	stack := make([]string, 0)
	var visit func(string) error
	visit = func(node string) error {
		state[node] = stateVisiting
		stack = append(stack, node)
		for dep := range graph[node] {
			switch state[dep] {
			case stateUnvisited:
				if err := visit(dep); err != nil {
					return err
				}
			case stateVisiting:
				cycle := extractCycle(stack, dep)
				return fmt.Errorf("workflow dependency cycle: %s", strings.Join(cycle, " -> "))
			}
		}
		stack = stack[:len(stack)-1]
		state[node] = stateVisited
		return nil
	}
	for node := range graph {
		if state[node] == stateUnvisited {
			if err := visit(node); err != nil {
				return err
			}
		}
	}
	return nil
}

func extractCycle(stack []string, target string) []string {
	idx := slices.Index(stack, target)
	if idx == -1 {
		return append([]string{}, target)
	}
	cycle := append([]string{}, stack[idx:]...)
	cycle = append(cycle, target)
	return cycle
}

func (c *Compozy) validateDependencies(
	ctx context.Context,
	graph dependencyGraph,
) error {
	order, err := buildWorkflowOrder(c.workflowOrder, graph)
	if err != nil {
		return err
	}
	if len(order) == 0 {
		return nil
	}
	c.mu.Lock()
	c.workflowOrder = order
	c.mu.Unlock()
	return nil
}

func buildWorkflowOrder(original []string, graph dependencyGraph) ([]string, error) {
	nodes := make([]string, 0, len(graph))
	orderIndex := make(map[string]int, len(original))
	for idx, id := range original {
		orderIndex[id] = idx
	}
	for node := range graph {
		nodes = append(nodes, node)
		if _, ok := orderIndex[node]; !ok {
			orderIndex[node] = len(orderIndex) + len(nodes)
		}
	}
	indegree := make(map[string]int, len(nodes))
	dependents := make(map[string][]string, len(nodes))
	for node, deps := range graph {
		for dep := range deps {
			indegree[node]++
			dependents[dep] = append(dependents[dep], node)
		}
	}
	queue := make([]string, 0)
	for _, node := range nodes {
		if indegree[node] == 0 {
			queue = append(queue, node)
		}
	}
	order := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		idx := nextByOrder(queue, orderIndex)
		node := queue[idx]
		queue = append(queue[:idx], queue[idx+1:]...)
		order = append(order, node)
		for _, dep := range dependents[node] {
			indegree[dep]--
			if indegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}
	if len(order) != len(nodes) {
		return nil, fmt.Errorf("workflow dependency ordering failed due to cycle")
	}
	return order, nil
}

func nextByOrder(queue []string, orderIndex map[string]int) int {
	best := 0
	for i := 1; i < len(queue); i++ {
		if orderIndex[queue[i]] < orderIndex[queue[best]] {
			best = i
		}
	}
	return best
}

func taskPath(base string, id string, idx int) string {
	trimmed := strings.TrimSpace(id)
	if trimmed != "" {
		return fmt.Sprintf("%s.%s", base, trimmed)
	}
	return fmt.Sprintf("%s[%d]", base, idx)
}

func isAgentReference(cfg *agent.Config) bool {
	if cfg == nil {
		return false
	}
	if strings.TrimSpace(cfg.ID) == "" {
		return false
	}
	if strings.TrimSpace(cfg.Instructions) != "" {
		return false
	}
	if cfg.Model.HasRef() || cfg.Model.HasConfig() {
		return false
	}
	if cfg.With != nil && len(*cfg.With) > 0 {
		return false
	}
	if cfg.Env != nil && len(*cfg.Env) > 0 {
		return false
	}
	if len(cfg.Actions) > 0 || len(cfg.Tools) > 0 || len(cfg.Knowledge) > 0 || len(cfg.Memory) > 0 {
		return false
	}
	return true
}

func isToolReference(cfg *tool.Config) bool {
	if cfg == nil {
		return false
	}
	if strings.TrimSpace(cfg.ID) == "" {
		return false
	}
	return cfg.Name == "" && cfg.Description == "" && cfg.Runtime == "" && cfg.Code == "" &&
		cfg.Timeout == "" && cfg.InputSchema == nil && cfg.OutputSchema == nil && cfg.With == nil &&
		cfg.Config == nil && cfg.Env == nil
}

func isBuiltinTool(id string) bool {
	switch id {
	case builtinCallWorkflow, builtinCallWorkflows:
		return true
	default:
		return false
	}
}

func collectWorkflowDependencies(toolID string, input *core.Input) []string {
	switch toolID {
	case builtinCallWorkflow:
		return collectSingleWorkflowDependency(input)
	case builtinCallWorkflows:
		return collectBatchWorkflowDependencies(input)
	default:
		return nil
	}
}

func collectSingleWorkflowDependency(input *core.Input) []string {
	if input == nil {
		return nil
	}
	if value, ok := (*input)["workflow_id"]; ok {
		if id, ok := value.(string); ok && strings.TrimSpace(id) != "" {
			return []string{strings.TrimSpace(id)}
		}
	}
	return nil
}

func collectBatchWorkflowDependencies(input *core.Input) []string {
	if input == nil {
		return nil
	}
	value, ok := (*input)["workflows"]
	if !ok {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	deps := make([]string, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			if idVal, exists := m["workflow_id"]; exists {
				if id, ok := idVal.(string); ok && strings.TrimSpace(id) != "" {
					deps = append(deps, strings.TrimSpace(id))
				}
			}
		}
	}
	return deps
}

func ensureMemoryDefaults(mem *memory.Config) {
	if mem == nil {
		return
	}
	if strings.TrimSpace(mem.Resource) == "" {
		mem.Resource = string(resources.ResourceMemory)
	}
	if mem.Type == "" {
		mem.Type = memcore.TokenBasedMemory
	}
}
