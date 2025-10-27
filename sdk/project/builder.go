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
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	engineschedule "github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// Builder constructs engine project configurations using a fluent API while
// accumulating validation errors until Build is invoked.
type Builder struct {
	config         *engineproject.Config
	errors         []error
	workflows      []*workflow.Config
	agents         []*agent.Config
	schedules      []*engineschedule.Config
	memories       []*memory.Config
	embedders      []knowledge.EmbedderConfig
	vectorDBs      []knowledge.VectorDBConfig
	knowledgeBases []knowledge.BaseConfig
	mcps           []mcp.Config
	tools          []tool.Config
}

// New creates a project builder with the provided project name.
func New(name string) *Builder {
	return &Builder{
		config: &engineproject.Config{
			Name:   strings.TrimSpace(name),
			Models: make([]*core.ProviderConfig, 0),
		},
		errors:         make([]error, 0),
		workflows:      make([]*workflow.Config, 0),
		agents:         make([]*agent.Config, 0),
		schedules:      make([]*engineschedule.Config, 0),
		memories:       make([]*memory.Config, 0),
		embedders:      make([]knowledge.EmbedderConfig, 0),
		vectorDBs:      make([]knowledge.VectorDBConfig, 0),
		knowledgeBases: make([]knowledge.BaseConfig, 0),
		mcps:           make([]mcp.Config, 0),
		tools:          make([]tool.Config, 0),
	}
}

// WithVersion configures the semantic version associated with the project.
func (b *Builder) WithVersion(version string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(version)
	b.config.Version = trimmed
	return b
}

// WithDescription sets a human-readable description for the project.
func (b *Builder) WithDescription(desc string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(desc)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("description cannot be empty"))
		return b
	}
	b.config.Description = trimmed
	return b
}

// WithAuthor configures the author metadata for the project.
func (b *Builder) WithAuthor(name, email, org string) *Builder {
	if b == nil {
		return nil
	}
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		b.errors = append(b.errors, fmt.Errorf("author name cannot be empty"))
	}
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		b.errors = append(b.errors, fmt.Errorf("author email cannot be empty"))
	} else if _, err := mail.ParseAddress(trimmedEmail); err != nil {
		b.errors = append(b.errors, fmt.Errorf("author email must be valid: %w", err))
	}
	trimmedOrg := strings.TrimSpace(org)
	b.config.Author = core.Author{
		Name:         trimmedName,
		Email:        trimmedEmail,
		Organization: trimmedOrg,
	}
	return b
}

// AddModel registers a model provider configuration with the project.
func (b *Builder) AddModel(model *core.ProviderConfig) *Builder {
	if b == nil {
		return nil
	}
	if model == nil {
		b.errors = append(b.errors, fmt.Errorf("model cannot be nil"))
		return b
	}
	b.config.Models = append(b.config.Models, model)
	return b
}

// AddWorkflow registers a workflow configuration with the project builder.
func (b *Builder) AddWorkflow(wf *workflow.Config) *Builder {
	if b == nil {
		return nil
	}
	if wf == nil {
		b.errors = append(b.errors, fmt.Errorf("workflow cannot be nil"))
		return b
	}
	b.workflows = append(b.workflows, wf)
	return b
}

// AddAgent registers an agent configuration with the project builder.
func (b *Builder) AddAgent(agentCfg *agent.Config) *Builder {
	if b == nil {
		return nil
	}
	if agentCfg == nil {
		b.errors = append(b.errors, fmt.Errorf("agent cannot be nil"))
		return b
	}
	b.agents = append(b.agents, agentCfg)
	return b
}

// AddSchedule registers a workflow schedule with the project builder.
func (b *Builder) AddSchedule(scheduleCfg *engineschedule.Config) *Builder {
	if b == nil {
		return nil
	}
	if scheduleCfg == nil {
		b.errors = append(b.errors, fmt.Errorf("schedule cannot be nil"))
		return b
	}
	id := strings.TrimSpace(scheduleCfg.ID)
	if id == "" {
		b.errors = append(b.errors, fmt.Errorf("schedule id cannot be empty"))
	}
	workflowID := strings.TrimSpace(scheduleCfg.WorkflowID)
	if workflowID == "" {
		b.errors = append(b.errors, fmt.Errorf("schedule workflow id cannot be empty"))
	}
	cloned, err := core.DeepCopy(scheduleCfg)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("failed to copy schedule config: %w", err))
		return b
	}
	cloned.ID = id
	cloned.WorkflowID = workflowID
	b.schedules = append(b.schedules, cloned)
	return b
}

// AddMemory registers a memory configuration with the project builder.
func (b *Builder) AddMemory(memCfg *memory.Config) *Builder {
	if b == nil {
		return nil
	}
	if memCfg == nil {
		b.errors = append(b.errors, fmt.Errorf("memory cannot be nil"))
		return b
	}
	b.memories = append(b.memories, memCfg)
	return b
}

// AddEmbedder registers an embedder configuration with the project builder.
func (b *Builder) AddEmbedder(embedCfg *knowledge.EmbedderConfig) *Builder {
	if b == nil {
		return nil
	}
	if embedCfg == nil {
		b.errors = append(b.errors, fmt.Errorf("embedder cannot be nil"))
		return b
	}
	b.embedders = append(b.embedders, *embedCfg)
	return b
}

// AddVectorDB registers a vector database configuration with the project builder.
func (b *Builder) AddVectorDB(vectorCfg *knowledge.VectorDBConfig) *Builder {
	if b == nil {
		return nil
	}
	if vectorCfg == nil {
		b.errors = append(b.errors, fmt.Errorf("vector db cannot be nil"))
		return b
	}
	b.vectorDBs = append(b.vectorDBs, *vectorCfg)
	return b
}

// AddKnowledgeBase registers a knowledge base configuration with the project builder.
func (b *Builder) AddKnowledgeBase(baseCfg *knowledge.BaseConfig) *Builder {
	if b == nil {
		return nil
	}
	if baseCfg == nil {
		b.errors = append(b.errors, fmt.Errorf("knowledge base cannot be nil"))
		return b
	}
	b.knowledgeBases = append(b.knowledgeBases, *baseCfg)
	return b
}

// AddMCP registers an MCP server configuration with the project builder.
func (b *Builder) AddMCP(mcpCfg *mcp.Config) *Builder {
	if b == nil {
		return nil
	}
	if mcpCfg == nil {
		b.errors = append(b.errors, fmt.Errorf("mcp cannot be nil"))
		return b
	}
	b.mcps = append(b.mcps, *mcpCfg)
	return b
}

// AddTool registers a tool configuration with the project builder.
func (b *Builder) AddTool(toolCfg *tool.Config) *Builder {
	if b == nil {
		return nil
	}
	if toolCfg == nil {
		b.errors = append(b.errors, fmt.Errorf("tool cannot be nil"))
		return b
	}
	b.tools = append(b.tools, *toolCfg)
	return b
}

// Build validates the accumulated configuration and returns a project config.
func (b *Builder) Build(ctx context.Context) (*engineproject.Config, error) {
	if err := b.ensureBuilderState(ctx); err != nil {
		return nil, err
	}
	errs := b.collectBuildErrors(ctx)
	if len(errs) > 0 {
		return nil, &sdkerrors.BuildError{Errors: errs}
	}
	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone project config: %w", err)
	}
	return cloned, nil
}

func (b *Builder) ensureBuilderState(ctx context.Context) error {
	if b == nil {
		return fmt.Errorf("project builder is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("building project configuration", "project", b.config.Name, "workflows", len(b.workflows))
	return nil
}

func (b *Builder) collectBuildErrors(ctx context.Context) []error {
	collected := append(make([]error, 0, len(b.errors)+4), b.errors...)
	if err := b.validateProjectName(ctx); err != nil {
		collected = append(collected, err)
	}
	if err := b.validateVersion(); err != nil {
		collected = append(collected, err)
	}
	if err := b.ensureWorkflowsPresent(); err != nil {
		collected = append(collected, err)
	}
	schedules, scheduleErrs := b.prepareSchedules()
	collected = append(collected, scheduleErrs...)
	b.config.Schedules = schedules
	b.config.Memories = b.memories
	b.config.Embedders = b.embedders
	b.config.VectorDBs = b.vectorDBs
	b.config.KnowledgeBases = b.knowledgeBases
	b.config.MCPs = b.mcps
	b.config.Tools = b.tools
	return filterErrors(collected)
}

func (b *Builder) validateProjectName(ctx context.Context) error {
	if err := validate.Required(ctx, "project name", b.config.Name); err != nil {
		return err
	}
	if err := validate.ID(ctx, b.config.Name); err != nil {
		return fmt.Errorf("project name must be alphanumeric or hyphenated: %w", err)
	}
	return nil
}

func (b *Builder) validateVersion() error {
	version := strings.TrimSpace(b.config.Version)
	if version == "" {
		return nil
	}
	if _, err := semver.NewVersion(version); err != nil {
		return fmt.Errorf("version must be valid semver: %w", err)
	}
	b.config.Version = version
	return nil
}

func (b *Builder) ensureWorkflowsPresent() error {
	if len(b.workflows) == 0 {
		return fmt.Errorf("at least one workflow must be registered")
	}
	return nil
}

func (b *Builder) prepareSchedules() ([]*engineschedule.Config, []error) {
	if len(b.schedules) == 0 {
		return nil, nil
	}
	errs := make([]error, 0, len(b.schedules))
	dupeSchedules := findDuplicateScheduleIDs(b.schedules)
	if len(dupeSchedules) > 0 {
		errs = append(errs, fmt.Errorf("duplicate schedule ids found: %s", strings.Join(dupeSchedules, ", ")))
	}
	workflowIDs := b.workflowIDSet()
	clonedSchedules := make([]*engineschedule.Config, 0, len(b.schedules))
	for _, sched := range b.schedules {
		if sched == nil {
			continue
		}
		if _, exists := workflowIDs[sched.WorkflowID]; !exists {
			errs = append(errs, fmt.Errorf("schedule %s references unknown workflow %s", sched.ID, sched.WorkflowID))
			continue
		}
		clone, err := core.DeepCopy(sched)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to clone schedule config: %w", err))
			continue
		}
		clonedSchedules = append(clonedSchedules, clone)
	}
	return clonedSchedules, errs
}

func (b *Builder) workflowIDSet() map[string]struct{} {
	ids := make(map[string]struct{}, len(b.workflows))
	for _, wf := range b.workflows {
		if wf == nil {
			continue
		}
		trimmed := strings.TrimSpace(wf.ID)
		if trimmed != "" {
			ids[trimmed] = struct{}{}
		}
	}
	return ids
}

func findDuplicateScheduleIDs(schedules []*engineschedule.Config) []string {
	if len(schedules) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(schedules))
	dupes := make([]string, 0)
	for _, sched := range schedules {
		if sched == nil {
			continue
		}
		id := strings.TrimSpace(sched.ID)
		if id == "" {
			continue
		}
		if seen[id] {
			if !containsString(dupes, id) {
				dupes = append(dupes, id)
			}
			continue
		}
		seen[id] = true
	}
	return dupes
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func filterErrors(errs []error) []error {
	if len(errs) == 0 {
		return nil
	}
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return filtered
}
