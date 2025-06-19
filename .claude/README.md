# Claude Code Custom Commands

This directory contains custom slash commands for Claude Code that implement the project's established workflows from `.cursor/rules/`.

## Command Structure

Commands are organized by workflow phase:

```
.claude/
├── commands/
│   ├── prd-clarify.md          # PRD clarification questions
│   ├── prd-generate.md         # Generate PRD from template
│   ├── prd-techspec.md         # Generate technical specification
│   ├── tasks-parent.md         # Generate high-level tasks
│   ├── tasks-subtasks.md       # Break down into subtasks
│   ├── tasks-complexity.md     # Analyze task complexity
│   ├── task-start.md           # Start next task with proper setup
│   ├── task-next.md            # Find and start next available task automatically
│   ├── task-review.md          # Complete task review & completion workflow
│   ├── session-start.md        # Start new development session
│   ├── session-current.md      # Show current session status
│   ├── session-update.md       # Update session with progress
│   ├── session-end.md          # End current session
│   ├── session-list.md         # List all sessions
│   └── session-help.md         # Session management help
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
/project:prd-techspec <feature-name>
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

### 4. Session Management Workflow

```bash
# Start new development session
/project:session-start

# Check current session status
/project:session-current

# Update session with progress
/project:session-update

# End current session
/project:session-end

# List all sessions
/project:session-list

# Session management help
/project:session-help
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
- **Next Step:** Run `/project:prd-techspec`

#### `/project:prd-techspec <feature-name>`

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
    - Pre-commit validation (`make lint`, `make test`)
    - Git commit with proper message format
    - Update task checkboxes to mark complete

### Session Management Commands

#### `/project:session-start`

- **Purpose:** Start new development session with proper context
- **Usage:** Beginning of work session
- **Process:** Initialize session tracking and set up development context

#### `/project:session-current`

- **Purpose:** Show current session status and progress
- **Usage:** Check what's currently being worked on
- **Output:** Current session details and progress status

#### `/project:session-update`

- **Purpose:** Update session with progress and notes
- **Usage:** Log progress during development
- **Process:** Track completed work and update session state

#### `/project:session-end`

- **Purpose:** End current session with summary
- **Usage:** Conclude work session
- **Process:** Summarize work completed and clean up session state

#### `/project:session-list`

- **Purpose:** List all development sessions
- **Usage:** Review session history
- **Output:** List of all sessions with status and progress

#### `/project:session-help`

- **Purpose:** Show session management help and usage
- **Usage:** Learn about session management features
- **Output:** Detailed help for session commands

## Usage Examples

### Complete Feature Development Workflow

```bash
# 1. Start new feature
/project:prd-clarify
# Answer questions...

/project:prd-generate monitoring-system
/project:prd-techspec monitoring-system

# 2. Plan implementation
/project:tasks-parent monitoring-system
# Review parent tasks, respond "Go"

/project:tasks-subtasks monitoring-system
/project:tasks-complexity monitoring-system

# 3. Implement and complete tasks
/project:session-start
/project:task-next
# ... implement the task ...

/project:task-review 1
/project:session-end
```

### Session-Based Development

```bash
# Start work session
/project:session-start

# Check current work
/project:session-current

# Work on tasks
/project:task-next
# ... implementation work ...

# Update progress
/project:session-update

# Complete and end session
/project:task-review 1
/project:session-end
```

### Task Management Workflow

```bash
# Find next task automatically
/project:task-next

# Review specific task implementation
/project:task-review 3

# Check all sessions
/project:session-list
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
- **Session Tracking:** Organized development sessions with progress tracking
- **Documentation:** Self-documenting workflow with clear next steps

## Notes

- Commands use `$ARGUMENTS` placeholder for dynamic values
- All commands reference specific sections in `.cursor/rules/` files
- Zen MCP integration provides enhanced analysis capabilities
- Commands are designed for sequential workflow execution
- Session management helps organize development work
- Each command includes clear next-step guidance
