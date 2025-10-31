## status: completed

<task_context>
<domain>sdk/project</domain>
<type>implementation</type>
<scope>code_generation</scope>
<complexity>high</complexity>
<dependencies>ALL_PREVIOUS_TASKS</dependencies>
</task_context>

# Task 11.0: Migrate project Package to Functional Options

## Overview

Migrate `sdk/project` - the top-level orchestrator that integrates all other packages (agents, workflows, tasks, tools, memory, knowledge, schedules). This is the final and most complex migration.

**Estimated Time:** 6-8 hours

**Dependencies:** Requires ALL previous tasks (1.0-10.0) complete

<critical>
- **ORCHESTRATOR PACKAGE:** Depends on every other SDK package
- **LARGEST CONFIG:** 15+ fields integrating all subsystems
- **INTEGRATION VALIDATION:** Must validate cross-package references
- **FINAL MIGRATION:** Completes the SDK transformation
</critical>

<requirements>
- Generate options from engine/project/config.go (15+ fields)
- Validate agent references exist
- Validate workflow references exist
- Validate tool, memory, knowledge bindings
- Validate schedule configurations
- Handle resource collections (agents[], workflows[], tools[])
- Cross-reference validation across all resources
- Deep copy with all nested configs
- Extensive integration tests
</requirements>

## Subtasks

- [x] 11.1 Create sdk/project/ directory structure
- [x] 11.2 Create generate.go for project config
- [x] 11.3 Generate options (20 fields)
- [x] 11.4 Create constructor with cross-validation
- [x] 11.5 Implement resource reference validation
- [x] 11.6 Agent collection validation
- [x] 11.7 Workflow collection validation
- [x] 11.8 Tool, memory, knowledge validation
- [x] 11.9 Schedule integration
- [x] 11.10 Comprehensive unit tests (30 tests)
- [x] 11.11 Integration tests with all packages
- [x] 11.12 Complete documentation

## Implementation Details

### Engine Fields (15+ fields)
```go
type Config struct {
    ID          string
    Name        string
    Version     string
    Agents      []agent.Config      // Collection
    Workflows   []workflow.Config   // Collection
    Tools       []tool.Config       // Collection
    Memory      []memory.Config     // Collection
    Knowledge   []knowledge.Config  // Collection
    Schedules   []schedule.Config   // Collection
    Runtime     *runtime.Config     // Optional
    Env         map[string]string
    Secrets     map[string]string
    Config      map[string]any
    Metadata    map[string]any
    Hooks       *HooksConfig        // Optional
}
```

### Constructor Pattern
```go
func New(ctx context.Context, id string, name string, opts ...Option) (*project.Config, error) {
    cfg := &project.Config{
        ID:        id,
        Name:      name,
        Agents:    make([]agent.Config, 0),
        Workflows: make([]workflow.Config, 0),
        Tools:     make([]tool.Config, 0),
        // ... initialize collections
    }

    // Apply options
    for _, opt := range opts {
        opt(cfg)
    }

    // Cross-reference validation
    if err := validateAgentReferences(cfg); err != nil {
        return nil, err
    }
    if err := validateWorkflowReferences(cfg); err != nil {
        return nil, err
    }
    // ... more validation

    return core.DeepCopy(cfg)
}
```

### Cross-Reference Validation

**Agent References:**
- Agents reference tools by ID → validate tools exist
- Agents reference memory by ID → validate memory exists
- Agents reference knowledge by ID → validate knowledge exists

**Workflow References:**
- Workflows reference tasks by ID → validate task structure
- Tasks reference agents by ID → validate agents exist
- Tasks reference actions by ID → validate action exists in agent

**Schedule References:**
- Schedules reference workflows by ID → validate workflows exist

### Validation Functions
```go
func validateAgentReferences(cfg *project.Config) error
func validateWorkflowReferences(cfg *project.Config) error
func validateToolReferences(cfg *project.Config) error
func validateMemoryReferences(cfg *project.Config) error
func validateKnowledgeReferences(cfg *project.Config) error
func validateScheduleReferences(cfg *project.Config) error
func validateCircularDependencies(cfg *project.Config) error
```

### Files

**Reference (for understanding):**
- `sdk/project/builder.go` (~460 LOC) - Old builder to understand requirements
- `engine/project/config.go` - Source struct

**To Create in sdk/project/:**
- `sdk/project/generate.go`
- `sdk/project/options_generated.go` (15+ options)
- `sdk/project/constructor.go` (~200 lines with validation)
- `sdk/project/constructor_test.go` (~500+ lines)
- `sdk/project/validation.go` (cross-reference logic)
- `sdk/project/validation_test.go`
- `sdk/project/README.md`

**Note:** Do NOT modify sdk/project/ - keep as reference only

## Tests

**Unit Tests:**
- [ ] Minimal project (ID + name only)
- [ ] Project with single agent
- [ ] Project with single workflow
- [ ] Project with multiple agents
- [ ] Project with all resource types
- [ ] Empty collections allowed
- [ ] Nil context fails
- [ ] Empty ID fails
- [ ] Empty name fails

**Cross-Reference Tests:**
- [ ] Agent references valid tool
- [ ] Agent references invalid tool fails
- [ ] Workflow references valid task
- [ ] Workflow references invalid task fails
- [ ] Task references valid agent
- [ ] Task references invalid agent fails
- [ ] Schedule references valid workflow
- [ ] Schedule references invalid workflow fails
- [ ] Circular workflow dependencies fail

**Integration Tests:**
- [ ] Complete project with all subsystems
- [ ] Multi-agent multi-workflow project
- [ ] Knowledge + memory integration
- [ ] Scheduled workflow execution config
- [ ] Complex dependency graph validation

## Success Criteria

- [ ] All 15+ fields have functional options
- [ ] Cross-reference validation complete
- [ ] Circular dependency detection working
- [ ] Resource collection handling correct
- [ ] Tests pass: `gotestsum -- ./sdk/project`
- [ ] Linter clean: `golangci-lint run ./sdk/project/...`
- [ ] Integration tests with all SDK packages pass
- [ ] New code in sdk/project/ (~200 LOC)
- [ ] README documents complete project structure
- [ ] Examples reference best practices

## Notes

This task completes the SDK migration. After this:
1. All SDK packages use functional options
2. Total code reduction: ~4,000 LOC → ~1,200 LOC (70%)
3. Zero manual maintenance for new engine fields
4. Only Task 12.0 (examples) remains
