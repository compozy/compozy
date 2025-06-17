# Claude Code Custom Commands

This directory contains custom slash commands for Claude Code that implement the project's established workflows from `.cursor/rules/`.

## Command Structure

Commands are organized by workflow phase:

```
.claude/
├── commands/
│   ├── prd-clarify.md          # PRD clarification questions
│   ├── prd-generate.md         # Generate PRD from template
│   ├── techspec-generate.md    # Generate technical specification
│   ├── tasks-parent.md         # Generate high-level tasks
│   ├── tasks-subtasks.md       # Break down into subtasks
│   ├── tasks-complexity.md     # Analyze task complexity
│   ├── task-start.md           # Start next task with proper setup
│   ├── task-next.md            # Find and start next available task automatically
│   ├── task-review.md          # Complete task review & completion workflow
│   ├── rules-check.md          # Check code against project rules
│   ├── analyze-file.md         # Analyze current file changes
│   ├── analyze-changes.md      # Analyze work in progress changes
│   └── analyze-folder.md       # Analyze specific folder
└── README.md                   # This file
```

## Workflow Overview

### 1. PRD Development Workflow

```bash
# Start with clarifying questions
/project:prd-clarify

# Generate PRD after gathering requirements
/project:prd-generate <feature-name>

# Create technical specification
/project:techspec-generate <feature-name>
```

### 2. Task Planning Workflow

```bash
# Generate high-level parent tasks
/project:tasks-parent <feature-name>

# Break down into subtasks (after user confirms with "Go")
/project:tasks-subtasks <feature-name>

# Analyze complexity and suggest breakdowns
/project:tasks-complexity <feature-name>
```

### 3. Implementation & Completion Workflow

```bash
# Start specific task with proper setup and context
/project:task-start <task-number>

# Find and start next available task automatically
/project:task-next

# Complete full task review and completion workflow
/project:task-review <task-number>
```

### 4. Development Workflow

```bash
# Check code against project rules
/project:rules-check [file-path]

# Analyze current file with unstaged changes
/project:analyze-file

# Analyze work in progress changes
/project:analyze-changes

# Analyze specific folder
/project:analyze-folder engine/infra/monitoring
```

## Command Details

### PRD Commands

#### `/project:prd-clarify`

- **Purpose:** Ask comprehensive clarifying questions before PRD creation
- **Based on:** `prd-create.mdc` clarifying questions section
- **Usage:** Run first to gather requirements
- **Output:** List of questions covering problem, goals, users, functionality, and technical considerations

#### `/project:prd-generate <feature-name>`

- **Purpose:** Generate complete PRD using project template
- **Based on:** `prd-create.mdc` process workflow
- **Usage:** After answering clarifying questions
- **Output:** `tasks/prd-<feature-name>/_prd.md`
- **Next Step:** Run `/project:techspec-generate`

#### `/project:techspec-generate <feature-name>`

- **Purpose:** Create technical specification document
- **Based on:** `prd-tech-spec.mdc` complete workflow
- **Usage:** After PRD is complete
- **Process:** Pre-analysis with Zen MCP → Technical questions → Generate spec → Post-review
- **Output:** `tasks/prd-<feature-name>/_techspec.md`

### Task Planning Commands

#### `/project:tasks-parent <feature-name>`

- **Purpose:** Generate high-level parent tasks (Phase 1)
- **Based on:** `task-generate-list.mdc` §4
- **Usage:** After PRD and Tech Spec are complete
- **Output:** High-level task list with pause for user confirmation
- **Next Step:** User responds "Go" then run `/project:tasks-subtasks`

#### `/project:tasks-subtasks <feature-name>`

- **Purpose:** Break parent tasks into actionable subtasks (Phase 2)
- **Based on:** `task-generate-list.mdc` §6
- **Usage:** After user confirms parent tasks with "Go"
- **Output:**
    - `tasks/prd-<feature-name>/_tasks.md`
    - Individual `<num>_task.md` files

#### `/project:tasks-complexity <feature-name>`

- **Purpose:** Analyze task complexity and suggest breakdowns (Phase 3)
- **Based on:** `task-generate-list.mdc` §7
- **Usage:** After subtasks are generated
- **Process:** Uses Zen MCP to score complexity and recommend further breakdown
- **Output:** `tasks/prd-<feature-name>/task-complexity-report.json`

### Implementation Commands

#### `/project:task-start <task-number>`

- **Purpose:** Start working on specific task with proper setup and context
- **Based on:** All project standards and task workflow requirements
- **Usage:** Before beginning implementation of a specific task
- **Process:**
    - Reads task definition, PRD, and tech spec for context
    - Provides critical reminders about project standards
    - Outlines development workflow and available commands
    - Ensures proper task setup before implementation begins

#### `/project:task-next`

- **Purpose:** Find and start next available task automatically
- **Based on:** All project standards and task workflow requirements
- **Usage:** When ready to work on the next task without knowing the number
- **Process:**
    - Scans `tasks/prd-*/` directories for task files
    - Identifies next uncompleted task (first unchecked checkbox)
    - Provides same setup as `task-start` but automatically
    - Displays task number, title, and description

#### `/project:task-review <task-number>`

- **Purpose:** Complete full task review and completion workflow
- **Based on:** `task-review.mdc` steps 2-6 (complete workflow)
- **Usage:** When ready to review and complete a task
- **Process:**
    - Rules Analysis & Code Review with Zen MCP
    - Fix all identified issues
    - Pre-commit validation (`make lint`, `make test-all`)
    - Git commit with proper message format
    - Update task checkboxes to mark complete

### Development Commands

#### `/project:rules-check [file-path]`

- **Purpose:** Analyze code against all project standards
- **Based on:** All `.cursor/rules/` files
- **Usage:** Check specific file or current selection
- **Checks:**
    - Go coding standards (function length, error handling, constructor patterns)
    - Architecture compliance (dependency direction, interface usage)
    - Testing patterns (`t.Run("Should...")`, testify/mock usage)
    - API standards (Swagger docs, response formats)
    - Security requirements (no secrets in logs, input validation)

#### `/project:analyze-file`

- **Purpose:** Analyze current file with unstaged changes
- **Based on:** All `.cursor/rules/` files + Zen MCP codereview tool
- **Usage:** Analyze the currently open file with git diff
- **Process:**
    - Reads current file with unstaged changes
    - Applies go-coding-standards, architecture, testing-standards rules
    - Uses Zen MCP for deep analysis and refactoring suggestions

#### `/project:analyze-changes`

- **Purpose:** Analyze current work in progress changes
- **Based on:** All `.cursor/rules/` files + Zen MCP codereview tool
- **Usage:** Review all modified files in current working directory
- **Process:**
    - Gets modified files from `git diff --name-only` (staged + unstaged)
    - Filters out non-relevant files (generated, vendor, etc.)
    - Focuses on files related to current scope of work
    - Provides actionable improvements before committing

#### `/project:analyze-folder <folder-path>`

- **Purpose:** Analyze specific folder/directory
- **Based on:** project-structure, architecture, testing-standards rules
- **Usage:** `/project:analyze-folder engine/infra/monitoring`
- **Process:**
    - Reads all Go files in directory recursively
    - Architectural review and package cohesion analysis
    - Identifies refactoring opportunities

## Usage Examples

### Complete Feature Development Workflow

```bash
# 1. Start new feature
/project:prd-clarify
# Answer questions...

/project:prd-generate monitoring-system
/project:techspec-generate monitoring-system

# 2. Plan implementation
/project:tasks-parent monitoring-system
# Review parent tasks, respond "Go"

/project:tasks-subtasks monitoring-system
/project:tasks-complexity monitoring-system

# 3. Implement and complete tasks
/project:task-next
# ... implement the task ...

/project:task-review 1
```

### Development Workflow

```bash
# Check code against project standards
/project:rules-check src/engine/agent/service.go

# Analyze current file changes with Zen MCP
/project:analyze-file

# Analyze work in progress changes
/project:analyze-changes

# Deep analysis of specific folder
/project:analyze-folder engine/infra/monitoring
```

### Quick Task Review

```bash
# Review specific task implementation
/project:task-review 3

# Complete after fixes
/project:task-complete 3
```

## Integration with Project Standards

These commands enforce compliance with:

- **Architecture:** `.cursor/rules/architecture.mdc`
- **Go Standards:** `.cursor/rules/go-coding-standards.mdc`
- **Testing:** `.cursor/rules/testing-standards.mdc`
- **API Design:** `.cursor/rules/api-standards.mdc`
- **Security:** `.cursor/rules/quality-security.mdc`
- **Task Management:** `.cursor/rules/task-*.mdc`
- **PRD Process:** `.cursor/rules/prd-*.mdc`

## Benefits

- **Consistency:** Enforces project standards across all development phases
- **Quality:** Built-in code review and validation workflows
- **Efficiency:** One-command access to complex multi-step processes
- **Compliance:** Automatic adherence to established patterns and rules
- **Documentation:** Self-documenting workflow with clear next steps

## Notes

- Commands use `$ARGUMENTS` placeholder for dynamic values
- All commands reference specific sections in `.cursor/rules/` files
- Zen MCP integration provides enhanced analysis capabilities
- Commands are designed for sequential workflow execution
- Each command includes clear next-step guidance
