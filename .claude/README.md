# Claude Code Custom Commands

This directory contains custom slash commands for Claude Code that implement a streamlined agent-based workflow for the Compozy development pipeline. The system uses specialized agents coordinated through centralized rules from `.cursor/rules/`.

## Architecture Overview

The command system is built on a **Single Source of Truth** principle where all development standards, patterns, and workflows are centralized in `.cursor/rules/` and injected into specialized agents as needed.

```
.claude/
├── commands/
│   ├── agent_coordinator.md         # Master workflow orchestrator
│   ├── agent_feature_enricher.md    # Feature specification enrichment
│   ├── agent_prd_creator.md         # Product Requirements Document creation
│   ├── agent_prd_analyzer.md        # PRD quality analysis and validation
│   ├── agent_techspec_creator.md    # Technical specification creation
│   ├── agent_technical_reviewer.md  # Technical specification validation
│   ├── agent_simplicity_guardian.md # Complexity analysis and simplification
│   ├── agent_task_generator.md      # Task generation using Taskmaster MCP
│   ├── task-start.md               # Start task implementation
│   ├── task-next.md                # Find and start next available task
│   ├── task-review.md              # Complete task review workflow
│   ├── session-*.md                # Development session management
│   └── prd-*.md                    # Direct PRD utilities
└── README.md                       # This file
```

## Core Principles

### 1. Centralized Rules (Single Source of Truth)

All development standards are maintained in `.cursor/rules/*.mdc` files:

- **Architecture patterns:** `.cursor/rules/architecture.mdc`
- **Go coding standards:** `.cursor/rules/go-coding-standards.mdc`
- **Testing requirements:** `.cursor/rules/testing-standards.mdc`
- **API design standards:** `.cursor/rules/api-standards.mdc`
- **Security & quality:** `.cursor/rules/quality-security.mdc`
- **PRD creation:** `.cursor/rules/prd-create.mdc`
- **Technical specifications:** `.cursor/rules/prd-tech-spec.mdc`
- **Task generation:** `.cursor/rules/task-generate-list.mdc`

### 2. Agent-Based Workflow

Specialized agents handle specific phases of development:

- **Feature Enricher:** Transforms raw feature requests into structured specifications
- **PRD Creator:** Generates comprehensive Product Requirements Documents
- **PRD Analyzer:** Validates PRD quality and completeness
- **Tech Spec Creator:** Creates detailed technical implementation specifications
- **Technical Reviewer:** Validates technical specifications for quality and compliance
- **Simplicity Guardian:** Analyzes complexity and prevents overengineering
- **Task Generator:** Converts specifications into implementable task lists using Taskmaster MCP

### 3. Rule Injection System

The `agent_coordinator.md` acts as a dependency injector, providing each agent with only the rules relevant to their specific function, ensuring:

- **Separation of concerns:** Agents focus on their specific role
- **Consistency:** All agents operate from the same rule base
- **Maintainability:** Rule changes propagate automatically to all relevant agents

## Primary Workflows

### Complete Feature Development Pipeline

The agent coordinator orchestrates the complete pipeline from feature request to implementable tasks:

```bash
# Full pipeline: Feature → PRD → Tech Spec → Tasks
/project:agent-coordinator "Add user authentication system"
```

**Process:**

1. **Feature Enrichment:** Raw request → Structured specification
2. **PRD Creation:** Specification → Comprehensive requirements document
3. **PRD Analysis:** Quality validation and completeness check
4. **Tech Spec Creation:** Requirements → Technical implementation design
5. **Technical Review:** Architecture validation and quality assurance
6. **Complexity Analysis:** Simplification and scope validation
7. **Task Generation:** Specifications → Implementable task lists using Taskmaster MCP

### Individual Agent Workflows

For targeted work on specific phases:

```bash
# Create PRD from enriched feature specification
/project:agent-prd-creator

# Analyze existing PRD for quality and completeness
/project:agent-prd-analyzer

# Create technical specification from validated PRD
/project:agent-techspec-creator

# Review technical specification for compliance
/project:agent-technical-reviewer

# Generate tasks using Taskmaster MCP workflow
/project:agent-task-generator
```

### Implementation & Completion

```bash
# Start working on next available task
/project:task-next

# Start specific task with context
/project:task-start <task-number>

# Complete task with full validation workflow
/project:task-review <task-number>
```

## Agent Details

### Core Agent Workflow

#### `agent_coordinator.md`

- **Purpose:** Master orchestrator for the complete development pipeline
- **Rules Injected:** All 14 rules from `.cursor/rules/`
- **Capabilities:**
    - Complete feature development pipeline
    - Individual workflow phase execution
    - Conflict resolution and quality oversight
    - Progress tracking and validation
- **Usage:** Primary entry point for comprehensive feature development

#### `agent_feature_enricher.md`

- **Purpose:** Transform brief feature descriptions into structured specifications
- **Rules Injected:**
    - `.cursor/rules/backwards-compatibility.mdc`
    - `.cursor/rules/architecture.mdc`
    - `.cursor/rules/prd-create.mdc`
    - `.cursor/rules/quality-security.mdc`
- **Output:** Comprehensive JSON specification ready for PRD creation

#### `agent_prd_creator.md`

- **Purpose:** Generate complete Product Requirements Documents
- **Rules Injected:**
    - `.cursor/rules/prd-create.mdc`
    - `.cursor/rules/critical-validation.mdc`
- **Output:** Stakeholder-ready PRD following official template

#### `agent_prd_analyzer.md`

- **Purpose:** Validate PRD quality, completeness, and implementation readiness
- **Rules Injected:**
    - `.cursor/rules/prd-create.mdc`
    - `.cursor/rules/quality-security.mdc`
    - `.cursor/rules/architecture.mdc`
- **Output:** Quality assessment with actionable feedback

#### `agent_techspec_creator.md`

- **Purpose:** Create detailed technical implementation specifications
- **Rules Injected:**
    - `.cursor/rules/prd-tech-spec.mdc`
    - `.cursor/rules/architecture.mdc`
    - `.cursor/rules/go-coding-standards.mdc`
    - `.cursor/rules/api-standards.mdc`
    - `.cursor/rules/go-patterns.mdc`
- **Output:** Implementation-ready technical specification

#### `agent_technical_reviewer.md`

- **Purpose:** Validate technical specifications for architectural compliance
- **Rules Injected:**
    - `.cursor/rules/architecture.mdc`
    - `.cursor/rules/prd-tech-spec.mdc`
    - `.cursor/rules/quality-security.mdc`
    - `.cursor/rules/testing-standards.mdc`
    - `.cursor/rules/api-standards.mdc`
- **Output:** Technical approval with compliance validation

#### `agent_simplicity_guardian.md`

- **Purpose:** Analyze complexity and prevent overengineering
- **Rules Injected:**
    - `.cursor/rules/backwards-compatibility.mdc`
    - `.cursor/rules/task-generate-list.mdc`
    - `.cursor/rules/architecture.mdc`
- **Output:** Complexity analysis with simplification recommendations

#### `agent_task_generator.md`

- **Purpose:** Generate comprehensive task lists using Taskmaster MCP
- **Rules Injected:**
    - `.cursor/rules/task-generate-list.mdc`
    - `.cursor/rules/task-developing.mdc`
    - `.cursor/rules/task-review.mdc`
    - `.cursor/rules/testing-standards.mdc`
- **Output:** Complete taskmaster workflow with task files and complexity analysis

### Implementation Commands

#### `/project:task-start <task-number>`

- **Purpose:** Start specific task with proper setup and context
- **Standards:** References all project standards from `.cursor/rules/`
- **Process:** Provides task context, development workflow, and standards reminders

#### `/project:task-next`

- **Purpose:** Automatically find and start next available task
- **Process:** Scans for uncompleted tasks and provides same setup as `task-start`

#### `/project:task-review <task-number>`

- **Purpose:** Complete comprehensive task review and completion workflow
- **Based on:** `.cursor/rules/task-review.mdc`
- **Process:** Zen MCP code review → Issue resolution → Pre-commit validation → Git commit

### Session Management

Session commands (`session-start`, `session-current`, `session-update`, `session-end`, `session-list`, `session-help`) provide organized development session tracking independent of the main agent workflow.

## Key Benefits

### 1. **Elimination of Duplication**

- **70% content reduction** in command files
- **Single source of truth** for all development standards
- **Consistent rule application** across all agents

### 2. **Improved Maintainability**

- **Centralized updates:** Change rules in one place, affects all relevant agents
- **Clear dependencies:** Explicit rule injection makes relationships visible
- **Reduced complexity:** Agents focus on their specific role without rule duplication

### 3. **Quality Assurance**

- **Comprehensive validation:** Multi-stage review process with specialized agents
- **Standards enforcement:** Automatic compliance with established patterns
- **Zen MCP integration:** Enhanced code review and validation capabilities

### 4. **Scalability**

- **Easy agent addition:** New agents can reference existing rules without duplication
- **Rule evolution:** Standards can evolve without touching agent files
- **Clear architecture:** Separation of concerns enables independent development

## Usage Examples

### Complete Feature Development

```bash
# Orchestrated pipeline from feature to tasks
/project:agent-coordinator "Add monitoring dashboard with real-time metrics"
```

### Targeted Development Phase

```bash
# Work on specific phase
/project:agent-prd-creator
# ... create PRD from existing feature spec ...

/project:agent-task-generator
# ... generate tasks from validated tech spec ...
```

### Implementation Workflow

```bash
# Start implementation
/project:task-next
# ... implement task following project standards ...

# Complete with validation
/project:task-review 1
```

## Architecture Notes

- **Agent Coordinator:** Central orchestrator with complexity concentrated in one manageable file
- **Rule References:** All agents use complete `.cursor/rules/*.mdc` paths for clarity
- **Dependency Injection:** Explicit rule injection ensures agents receive only relevant standards
- **Quality Gates:** Multi-stage validation prevents issues from propagating downstream
- **Session Independence:** Session management operates separately from the main agent workflow

This streamlined architecture provides a robust, maintainable foundation for the Compozy development workflow while ensuring consistent quality and standards compliance.
