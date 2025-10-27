package project

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/memory"
	engineproject "github.com/compozy/compozy/engine/project"
	projectschedule "github.com/compozy/compozy/engine/project/schedule"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// New creates a project configuration using functional options
func New(ctx context.Context, name string, opts ...Option) (*engineproject.Config, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating project configuration", "project", name)
	cfg := initializeConfig(name)
	for _, opt := range opts {
		opt(cfg)
	}
	collected := collectValidationErrors(ctx, cfg)
	if len(collected) > 0 {
		return nil, &sdkerrors.BuildError{Errors: collected}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone project config: %w", err)
	}
	return cloned, nil
}

func initializeConfig(name string) *engineproject.Config {
	return &engineproject.Config{
		Name:           strings.TrimSpace(name),
		Models:         make([]*core.ProviderConfig, 0),
		Workflows:      make([]*engineproject.WorkflowSourceConfig, 0),
		Schedules:      make([]*projectschedule.Config, 0),
		Tools:          make([]tool.Config, 0),
		Embedders:      make([]knowledge.EmbedderConfig, 0),
		VectorDBs:      make([]knowledge.VectorDBConfig, 0),
		KnowledgeBases: make([]knowledge.BaseConfig, 0),
		Knowledge:      make([]core.KnowledgeBinding, 0),
		MCPs:           make([]mcp.Config, 0),
		Memories:       make([]*memory.Config, 0),
	}
}

func collectValidationErrors(ctx context.Context, cfg *engineproject.Config) []error {
	collected := make([]error, 0)
	appendIfError(&collected, validateName(ctx, cfg))
	appendIfError(&collected, validateVersion(cfg))
	validateDescription(cfg)
	appendIfError(&collected, validateAuthor(cfg))
	appendIfError(&collected, validateWorkflows(cfg))
	appendIfError(&collected, validateModels(cfg))
	appendIfError(&collected, validateSchedules(cfg))
	appendIfError(&collected, validateTools(cfg))
	appendIfError(&collected, validateMemories(cfg))
	appendIfError(&collected, validateKnowledge(cfg))
	return collected
}

func appendIfError(collected *[]error, err error) {
	if err != nil {
		*collected = append(*collected, err)
	}
}

func validateName(ctx context.Context, cfg *engineproject.Config) error {
	cfg.Name = strings.TrimSpace(cfg.Name)
	if err := validate.Required(ctx, "project name", cfg.Name); err != nil {
		return err
	}
	if err := validate.ID(ctx, cfg.Name); err != nil {
		return fmt.Errorf("project name must be alphanumeric or hyphenated: %w", err)
	}
	return nil
}

func validateVersion(cfg *engineproject.Config) error {
	version := strings.TrimSpace(cfg.Version)
	if version == "" {
		return nil
	}
	if _, err := semver.NewVersion(version); err != nil {
		return fmt.Errorf("version must be valid semver: %w", err)
	}
	cfg.Version = version
	return nil
}

func validateDescription(cfg *engineproject.Config) {
	cfg.Description = strings.TrimSpace(cfg.Description)
}

func validateAuthor(cfg *engineproject.Config) error {
	cfg.Author.Name = strings.TrimSpace(cfg.Author.Name)
	cfg.Author.Email = strings.TrimSpace(cfg.Author.Email)
	cfg.Author.Organization = strings.TrimSpace(cfg.Author.Organization)
	if cfg.Author.Email != "" {
		if _, err := mail.ParseAddress(cfg.Author.Email); err != nil {
			return fmt.Errorf("author email must be valid: %w", err)
		}
	}
	return nil
}

func validateWorkflows(cfg *engineproject.Config) error {
	if len(cfg.Workflows) == 0 {
		return fmt.Errorf("at least one workflow must be registered")
	}
	for i, wf := range cfg.Workflows {
		if wf == nil {
			return fmt.Errorf("workflow[%d] cannot be nil", i)
		}
		wf.Source = strings.TrimSpace(wf.Source)
		if wf.Source == "" {
			return fmt.Errorf("workflow[%d] source cannot be empty", i)
		}
	}
	return nil
}

func validateModels(cfg *engineproject.Config) error {
	if len(cfg.Models) == 0 {
		return nil
	}
	firstDefaultIdx := -1
	for i, model := range cfg.Models {
		if model != nil && model.Default {
			if firstDefaultIdx == -1 {
				firstDefaultIdx = i
			} else {
				return fmt.Errorf(
					"only one model can be marked as default, found multiple at indices %d and %d",
					firstDefaultIdx,
					i,
				)
			}
		}
	}
	return nil
}

func validateSchedules(cfg *engineproject.Config) error {
	if len(cfg.Schedules) == 0 {
		return nil
	}
	scheduleIDs := make(map[string]bool, len(cfg.Schedules))
	workflowIDSet := buildWorkflowIDSet(cfg)
	for i, sched := range cfg.Schedules {
		if sched == nil {
			continue
		}
		sched.ID = strings.TrimSpace(sched.ID)
		if sched.ID == "" {
			return fmt.Errorf("schedule[%d] id cannot be empty", i)
		}
		if scheduleIDs[sched.ID] {
			return fmt.Errorf("duplicate schedule id '%s' found", sched.ID)
		}
		scheduleIDs[sched.ID] = true
		sched.WorkflowID = strings.TrimSpace(sched.WorkflowID)
		if sched.WorkflowID == "" {
			return fmt.Errorf("schedule[%d] workflow id cannot be empty", i)
		}
		if _, exists := workflowIDSet[sched.WorkflowID]; !exists {
			return fmt.Errorf("schedule '%s' references unknown workflow '%s'", sched.ID, sched.WorkflowID)
		}
	}
	return nil
}

func buildWorkflowIDSet(cfg *engineproject.Config) map[string]struct{} {
	ids := make(map[string]struct{}, len(cfg.Workflows))
	for _, wf := range cfg.Workflows {
		if wf == nil {
			continue
		}
		source := strings.TrimSpace(wf.Source)
		if source != "" {
			ids[source] = struct{}{}
		}
	}
	return ids
}

func validateTools(cfg *engineproject.Config) error {
	if len(cfg.Tools) == 0 {
		return nil
	}
	toolIDs := make(map[string]bool, len(cfg.Tools))
	for i := range cfg.Tools {
		toolCfg := &cfg.Tools[i]
		toolCfg.ID = strings.TrimSpace(toolCfg.ID)
		if toolCfg.ID == "" {
			return fmt.Errorf("tool[%d] id cannot be empty", i)
		}
		if toolIDs[toolCfg.ID] {
			return fmt.Errorf("duplicate tool id '%s' found", toolCfg.ID)
		}
		toolIDs[toolCfg.ID] = true
	}
	return nil
}

func validateMemories(cfg *engineproject.Config) error {
	if len(cfg.Memories) == 0 {
		return nil
	}
	memoryIDs := make(map[string]bool, len(cfg.Memories))
	for i, mem := range cfg.Memories {
		if mem == nil {
			return fmt.Errorf("memory[%d] cannot be nil", i)
		}
		if strings.TrimSpace(mem.Resource) == "" {
			mem.Resource = string(core.ConfigMemory)
		}
		mem.ID = strings.TrimSpace(mem.ID)
		if mem.ID == "" {
			return fmt.Errorf("memory[%d] id cannot be empty", i)
		}
		if memoryIDs[mem.ID] {
			return fmt.Errorf("duplicate memory id '%s' found", mem.ID)
		}
		memoryIDs[mem.ID] = true
	}
	return nil
}

func validateKnowledge(cfg *engineproject.Config) error {
	if len(cfg.Knowledge) > 1 {
		return fmt.Errorf("only one knowledge binding is supported")
	}
	if len(cfg.Knowledge) == 1 {
		binding := &cfg.Knowledge[0]
		binding.ID = strings.TrimSpace(binding.ID)
		if binding.ID == "" {
			return fmt.Errorf("knowledge binding requires an id reference")
		}
	}
	embedderIDs := make(map[string]bool, len(cfg.Embedders))
	for i := range cfg.Embedders {
		embedder := &cfg.Embedders[i]
		embedder.ID = strings.TrimSpace(embedder.ID)
		if embedder.ID == "" {
			return fmt.Errorf("embedder[%d] id cannot be empty", i)
		}
		if embedderIDs[embedder.ID] {
			return fmt.Errorf("duplicate embedder id '%s' found", embedder.ID)
		}
		embedderIDs[embedder.ID] = true
	}
	vectorDBIDs := make(map[string]bool, len(cfg.VectorDBs))
	for i := range cfg.VectorDBs {
		vectorDB := &cfg.VectorDBs[i]
		vectorDB.ID = strings.TrimSpace(vectorDB.ID)
		if vectorDB.ID == "" {
			return fmt.Errorf("vector_db[%d] id cannot be empty", i)
		}
		if vectorDBIDs[vectorDB.ID] {
			return fmt.Errorf("duplicate vector_db id '%s' found", vectorDB.ID)
		}
		vectorDBIDs[vectorDB.ID] = true
	}
	knowledgeBaseIDs := make(map[string]bool, len(cfg.KnowledgeBases))
	for i := range cfg.KnowledgeBases {
		kb := &cfg.KnowledgeBases[i]
		kb.ID = strings.TrimSpace(kb.ID)
		if kb.ID == "" {
			return fmt.Errorf("knowledge_base[%d] id cannot be empty", i)
		}
		if knowledgeBaseIDs[kb.ID] {
			return fmt.Errorf("duplicate knowledge_base id '%s' found", kb.ID)
		}
		knowledgeBaseIDs[kb.ID] = true
	}
	return nil
}

// ValidateCrossReferences validates agent, workflow, tool, and other references
// This function is exported for use in project loading scenarios where
// agents and workflows are provided separately from the project config.
//
// Note: This performs basic structural validation. Full semantic validation
// (e.g., verifying agent references in tasks) happens during workflow execution
// when the actual workflow configs are loaded and parsed.
func ValidateCrossReferences(
	cfg *engineproject.Config,
	agents []agent.Config,
	_ []workflow.Config,
) error {
	collected := make([]error, 0)
	toolIDSet := buildToolIDSet(cfg)
	memoryIDSet := buildMemoryIDSet(cfg)
	knowledgeIDSet := buildKnowledgeBaseIDSet(cfg)
	for i := range agents {
		validateAgentReferences(&agents[i], toolIDSet, memoryIDSet, knowledgeIDSet, &collected)
	}
	if len(collected) > 0 {
		return &sdkerrors.BuildError{Errors: collected}
	}
	return nil
}

func validateAgentReferences(
	agentCfg *agent.Config,
	toolIDSet, memoryIDSet, knowledgeIDSet map[string]struct{},
	collected *[]error,
) {
	validateAgentTools(agentCfg, toolIDSet, collected)
	validateAgentMemory(agentCfg, memoryIDSet, collected)
	validateAgentKnowledge(agentCfg, knowledgeIDSet, collected)
}

func validateAgentTools(agentCfg *agent.Config, toolIDSet map[string]struct{}, collected *[]error) {
	for j := range agentCfg.Tools {
		toolID := strings.TrimSpace(agentCfg.Tools[j].ID)
		if toolID == "" {
			continue
		}
		if _, exists := toolIDSet[toolID]; !exists {
			*collected = append(
				*collected,
				fmt.Errorf("agent '%s' references unknown tool '%s'", agentCfg.ID, toolID),
			)
		}
	}
}

func validateAgentMemory(agentCfg *agent.Config, memoryIDSet map[string]struct{}, collected *[]error) {
	for j := range agentCfg.Memory {
		memID := strings.TrimSpace(agentCfg.Memory[j].ID)
		if memID == "" {
			continue
		}
		if _, exists := memoryIDSet[memID]; !exists {
			*collected = append(
				*collected,
				fmt.Errorf("agent '%s' references unknown memory '%s'", agentCfg.ID, memID),
			)
		}
	}
}

func validateAgentKnowledge(agentCfg *agent.Config, knowledgeIDSet map[string]struct{}, collected *[]error) {
	for j := range agentCfg.Knowledge {
		kbID := strings.TrimSpace(agentCfg.Knowledge[j].ID)
		if kbID == "" {
			continue
		}
		if _, exists := knowledgeIDSet[kbID]; !exists {
			*collected = append(
				*collected,
				fmt.Errorf("agent '%s' references unknown knowledge base '%s'", agentCfg.ID, kbID),
			)
		}
	}
}

func buildToolIDSet(cfg *engineproject.Config) map[string]struct{} {
	ids := make(map[string]struct{}, len(cfg.Tools))
	for i := range cfg.Tools {
		id := strings.TrimSpace(cfg.Tools[i].ID)
		if id != "" {
			ids[id] = struct{}{}
		}
	}
	return ids
}

func buildMemoryIDSet(cfg *engineproject.Config) map[string]struct{} {
	ids := make(map[string]struct{}, len(cfg.Memories))
	for i := range cfg.Memories {
		if cfg.Memories[i] != nil {
			id := strings.TrimSpace(cfg.Memories[i].ID)
			if id != "" {
				ids[id] = struct{}{}
			}
		}
	}
	return ids
}

func buildKnowledgeBaseIDSet(cfg *engineproject.Config) map[string]struct{} {
	ids := make(map[string]struct{}, len(cfg.KnowledgeBases))
	for i := range cfg.KnowledgeBases {
		id := strings.TrimSpace(cfg.KnowledgeBases[i].ID)
		if id != "" {
			ids[id] = struct{}{}
		}
	}
	return ids
}
