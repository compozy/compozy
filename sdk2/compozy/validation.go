package compozy

import (
	"fmt"
	"sort"
	"strings"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	enginemcp "github.com/compozy/compozy/engine/mcp"
	enginememory "github.com/compozy/compozy/engine/memory"
	engineproject "github.com/compozy/compozy/engine/project"
	projectschedule "github.com/compozy/compozy/engine/project/schedule"
	engineschema "github.com/compozy/compozy/engine/schema"
	enginetask "github.com/compozy/compozy/engine/task"
	enginetool "github.com/compozy/compozy/engine/tool"
	enginewebhook "github.com/compozy/compozy/engine/webhook"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
)

type dependencyGraph map[string][]string

type validationContext struct {
	report *ValidationReport
	nodes  map[string]struct{}
	graph  dependencyGraph
}

func newValidationContext(report *ValidationReport) *validationContext {
	return &validationContext{
		report: report,
		nodes:  make(map[string]struct{}),
		graph:  make(dependencyGraph),
	}
}

func (vc *validationContext) addNode(node string) {
	if node == "" {
		return
	}
	vc.nodes[node] = struct{}{}
}

func (vc *validationContext) addEdge(from string, to string) {
	if from == "" || to == "" {
		return
	}
	deps := vc.graph[from]
	if !containsString(deps, to) {
		vc.graph[from] = append(deps, to)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (vc *validationContext) registerProject(project *engineproject.Config) {
	if project == nil {
		return
	}
	name := strings.TrimSpace(project.Name)
	if name == "" {
		vc.report.Errors = append(
			vc.report.Errors,
			ValidationError{ResourceType: "project", ResourceID: "", Message: "project name is required"},
		)
		return
	}
	vc.addNode(projectNode(name))
}

func (vc *validationContext) registerWorkflows(workflows []*engineworkflow.Config) {
	for _, wf := range workflows {
		vc.registerWorkflow(wf)
	}
}

func (vc *validationContext) registerWorkflow(wf *engineworkflow.Config) {
	if wf == nil {
		return
	}
	id := strings.TrimSpace(wf.ID)
	if id == "" {
		vc.report.Errors = append(
			vc.report.Errors,
			ValidationError{ResourceType: "workflow", ResourceID: "", Message: "workflow id is required"},
		)
		return
	}
	wfNode := workflowNode(id)
	vc.addNode(wfNode)
	vc.registerWorkflowAgents(id, wf.Agents)
	vc.registerWorkflowTools(id, wf.Tools)
	vc.registerWorkflowKnowledge(id, wf.KnowledgeBases)
	vc.registerWorkflowTasks(id, wfNode, wf.Tasks)
}

func (vc *validationContext) registerWorkflowAgents(workflowID string, agents []engineagent.Config) {
	for i := range agents {
		agentID := strings.TrimSpace(agents[i].ID)
		if agentID == "" {
			vc.report.Warnings = append(
				vc.report.Warnings,
				ValidationWarning{
					ResourceType: "workflow",
					ResourceID:   workflowID,
					Message:      "workflow agent with empty id ignored",
				},
			)
			continue
		}
		vc.addNode(agentNode(agentID))
	}
}

func (vc *validationContext) registerWorkflowTools(workflowID string, tools []enginetool.Config) {
	for i := range tools {
		toolID := strings.TrimSpace(tools[i].ID)
		if toolID == "" {
			vc.report.Warnings = append(
				vc.report.Warnings,
				ValidationWarning{
					ResourceType: "workflow",
					ResourceID:   workflowID,
					Message:      "workflow tool with empty id ignored",
				},
			)
			continue
		}
		vc.addNode(toolNode(toolID))
	}
}

func (vc *validationContext) registerWorkflowKnowledge(workflowID string, knowledge []engineknowledge.BaseConfig) {
	for i := range knowledge {
		kbID := strings.TrimSpace(knowledge[i].ID)
		if kbID == "" {
			vc.report.Warnings = append(
				vc.report.Warnings,
				ValidationWarning{
					ResourceType: "workflow",
					ResourceID:   workflowID,
					Message:      "workflow knowledge base with empty id ignored",
				},
			)
			continue
		}
		vc.addNode(knowledgeNode(kbID))
	}
}

func (vc *validationContext) registerWorkflowTasks(workflowID string, workflowNode string, tasks []enginetask.Config) {
	seen := make(map[string]struct{})
	for i := range tasks {
		taskCfg := &tasks[i]
		taskID := strings.TrimSpace(taskCfg.ID)
		if taskID == "" {
			vc.report.Errors = append(
				vc.report.Errors,
				ValidationError{ResourceType: "workflow", ResourceID: workflowID, Message: "task id is required"},
			)
			continue
		}
		if _, ok := seen[taskID]; ok {
			vc.report.Errors = append(
				vc.report.Errors,
				ValidationError{
					ResourceType: "workflow",
					ResourceID:   workflowID,
					Message:      fmt.Sprintf("duplicate task id %s", taskID),
				},
			)
		} else {
			seen[taskID] = struct{}{}
		}
		taskNodeID := taskNode(workflowID, taskID)
		vc.addNode(taskNodeID)
		vc.addEdge(workflowNode, taskNodeID)
		vc.registerTaskBindings(workflowID, taskID, taskCfg)
	}
}

func (vc *validationContext) registerTaskBindings(workflowID string, taskID string, taskCfg *enginetask.Config) {
	if taskCfg == nil {
		return
	}
	node := taskNode(workflowID, taskID)
	if taskCfg.Agent != nil {
		agentID := strings.TrimSpace(taskCfg.Agent.ID)
		if agentID != "" {
			vc.addNode(agentNode(agentID))
			vc.addEdge(node, agentNode(agentID))
		}
	}
	if taskCfg.Tool != nil {
		toolID := strings.TrimSpace(taskCfg.Tool.ID)
		if toolID != "" {
			vc.addNode(toolNode(toolID))
			vc.addEdge(node, toolNode(toolID))
		}
	}
	if taskCfg.OnSuccess != nil && taskCfg.OnSuccess.Next != nil {
		next := strings.TrimSpace(*taskCfg.OnSuccess.Next)
		if next != "" {
			vc.addEdge(node, taskNode(workflowID, next))
		}
	}
	if taskCfg.OnError != nil && taskCfg.OnError.Next != nil {
		next := strings.TrimSpace(*taskCfg.OnError.Next)
		if next != "" {
			vc.addEdge(node, taskNode(workflowID, next))
		}
	}
}

func (vc *validationContext) registerAgents(agents []*engineagent.Config) {
	registerSimpleResources(vc, "agent", agents, func(cfg *engineagent.Config) string { return cfg.ID })
}

func (vc *validationContext) registerTools(tools []*enginetool.Config) {
	registerSimpleResources(vc, "tool", tools, func(cfg *enginetool.Config) string { return cfg.ID })
}

func (vc *validationContext) registerKnowledgeBases(kb []*engineknowledge.BaseConfig) {
	registerSimpleResources(vc, "knowledge", kb, func(cfg *engineknowledge.BaseConfig) string { return cfg.ID })
}

func (vc *validationContext) registerMemories(memories []*enginememory.Config) {
	registerSimpleResources(vc, "memory", memories, func(cfg *enginememory.Config) string { return cfg.ID })
}

func (vc *validationContext) registerMCPs(mcps []*enginemcp.Config) {
	registerSimpleResources(vc, "mcp", mcps, func(cfg *enginemcp.Config) string { return cfg.ID })
}

func (vc *validationContext) registerSchemas(schemas []*engineschema.Schema) {
	registerSimpleResources(
		vc,
		"schema",
		schemas,
		func(cfg *engineschema.Schema) string { return engineschema.GetID(cfg) },
	)
}

func (vc *validationContext) registerModels(models []*enginecore.ProviderConfig) {
	registerSimpleResources(vc, "model", models, func(cfg *enginecore.ProviderConfig) string {
		provider := strings.TrimSpace(string(cfg.Provider))
		model := strings.TrimSpace(cfg.Model)
		if provider == "" && model == "" {
			return ""
		}
		return provider + ":" + model
	})
}

func (vc *validationContext) registerSchedules(schedules []*projectschedule.Config) {
	registerSimpleResources(vc, "schedule", schedules, func(cfg *projectschedule.Config) string { return cfg.ID })
}

func (vc *validationContext) registerWebhooks(webhooks []*enginewebhook.Config) {
	registerSimpleResources(vc, "webhook", webhooks, func(cfg *enginewebhook.Config) string { return cfg.Slug })
}

func registerSimpleResources[T any](vc *validationContext, typ string, values []*T, idFn func(*T) string) {
	for _, value := range values {
		if value == nil {
			continue
		}
		id := strings.TrimSpace(idFn(value))
		if id == "" {
			vc.report.Warnings = append(
				vc.report.Warnings,
				ValidationWarning{ResourceType: typ, ResourceID: "", Message: "resource with empty id ignored"},
			)
			continue
		}
		vc.addNode(fmt.Sprintf("%s:%s", typ, id))
	}
}

func workflowNode(id string) string {
	return fmt.Sprintf("workflow:%s", id)
}

func projectNode(name string) string {
	return fmt.Sprintf("project:%s", name)
}

func agentNode(id string) string {
	return fmt.Sprintf("agent:%s", id)
}

func toolNode(id string) string {
	return fmt.Sprintf("tool:%s", id)
}

func knowledgeNode(id string) string {
	return fmt.Sprintf("knowledge:%s", id)
}

func taskNode(workflowID string, taskID string) string {
	return fmt.Sprintf("task:%s/%s", workflowID, taskID)
}

func (vc *validationContext) finalize(report *ValidationReport) {
	for node, deps := range vc.graph {
		sorted := append([]string(nil), deps...)
		sort.Strings(sorted)
		report.DependencyGraph[node] = sorted
	}
	report.ResourceCount = len(vc.nodes)
}

func (vc *validationContext) detectMissingReferences() {
	for from, deps := range vc.graph {
		for _, dep := range deps {
			if _, ok := vc.nodes[dep]; ok {
				continue
			}
			typ, id := parseNode(from)
			vc.report.MissingRefs = append(vc.report.MissingRefs, MissingReference{
				ResourceType: typ,
				ResourceID:   id,
				Reference:    dep,
			})
		}
	}
}

func (vc *validationContext) detectCycles() {
	visited := make(map[string]bool)
	stack := make(map[string]bool)
	path := make([]string, 0)
	var dfs func(string)
	dfs = func(node string) {
		visited[node] = true
		stack[node] = true
		path = append(path, node)
		for _, dep := range vc.graph[node] {
			if !visited[dep] {
				dfs(dep)
			} else if stack[dep] {
				cycle := extractCycle(path, dep)
				vc.report.CircularDeps = append(vc.report.CircularDeps, CircularDependency{Chain: cycle})
			}
		}
		stack[node] = false
		path = path[:len(path)-1]
	}
	for node := range vc.graph {
		if !visited[node] {
			dfs(node)
		}
	}
}

func extractCycle(path []string, target string) []string {
	idx := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		return append([]string(nil), target)
	}
	cycle := append([]string(nil), path[idx:]...)
	cycle = append(cycle, target)
	return cycle
}

func parseNode(node string) (string, string) {
	parts := strings.SplitN(node, ":", 2)
	if len(parts) != 2 {
		return node, ""
	}
	return parts[0], parts[1]
}

func (e *Engine) ValidateReferences() (*ValidationReport, error) {
	if e == nil {
		return nil, fmt.Errorf("engine is nil")
	}
	report := &ValidationReport{
		Valid:           true,
		Errors:          make([]ValidationError, 0),
		Warnings:        make([]ValidationWarning, 0),
		CircularDeps:    make([]CircularDependency, 0),
		MissingRefs:     make([]MissingReference, 0),
		DependencyGraph: make(map[string][]string),
	}
	vc := newValidationContext(report)
	e.stateMu.RLock()
	project := e.project
	workflows := append([]*engineworkflow.Config(nil), e.workflows...)
	agents := append([]*engineagent.Config(nil), e.agents...)
	tools := append([]*enginetool.Config(nil), e.tools...)
	knowledge := append([]*engineknowledge.BaseConfig(nil), e.knowledgeBases...)
	memories := append([]*enginememory.Config(nil), e.memories...)
	mcps := append([]*enginemcp.Config(nil), e.mcps...)
	schemas := append([]*engineschema.Schema(nil), e.schemas...)
	models := append([]*enginecore.ProviderConfig(nil), e.models...)
	schedules := append([]*projectschedule.Config(nil), e.schedules...)
	webhooks := append([]*enginewebhook.Config(nil), e.webhooks...)
	e.stateMu.RUnlock()
	vc.registerProject(project)
	vc.registerWorkflows(workflows)
	vc.registerAgents(agents)
	vc.registerTools(tools)
	vc.registerKnowledgeBases(knowledge)
	vc.registerMemories(memories)
	vc.registerMCPs(mcps)
	vc.registerSchemas(schemas)
	vc.registerModels(models)
	vc.registerSchedules(schedules)
	vc.registerWebhooks(webhooks)
	vc.detectMissingReferences()
	vc.detectCycles()
	vc.finalize(report)
	report.Valid = len(report.Errors) == 0 && len(report.MissingRefs) == 0 && len(report.CircularDeps) == 0
	return report, nil
}
