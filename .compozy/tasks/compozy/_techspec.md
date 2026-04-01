# Technical Specification: Compozy Skills for Compozy

## Executive Summary

Convert Compozy's core prompt workflows (PRD creation, TechSpec generation, Task breakdown) into three self-contained Claude Code skills that integrate natively with the Compozy package. Each skill follows the agentskills.io spec, bundles its own templates via progressive disclosure, and writes plain markdown files to `.compozy/tasks/<name>/`. A small Go code change updates the task file naming convention from `_task_*.md` to `task_*.md`.

## System Architecture

### Component Overview

Three new skills plus two surgical Go changes:

```
skills/
  create-prd/                    # NEW — PRD creation with brainstorming
    SKILL.md                     # Core logic (<500 lines)
    references/
      prd-template.md            # Self-contained PRD template
      question-protocol.md       # Brainstorming phases & question rules

  create-techspec/               # NEW — TechSpec generation
    SKILL.md                     # Core logic (<500 lines)
    references/
      techspec-template.md       # Self-contained TechSpec template

  create-tasks/                  # NEW — Task breakdown + enrichment
    SKILL.md                     # Core logic (<500 lines)
    references/
      task-template.md           # Task file template with YAML frontmatter metadata
      task-context-schema.md     # Frontmatter schema for Compozy's ParseTaskFile()

  execute-prd-task/              # EXISTING — unchanged
  fix-reviews/                   # EXISTING — generic review remediation skill
  verification-before-completion/ # EXISTING — unchanged
```

Go code changes:

```
internal/core/plan/input.go    # Update task file pattern: _task_*.md → task_*.md
internal/core/prompt/common.go # Update ExtractTaskNumber: _task_ prefix → task_ prefix
```

### Output Convention

All skills write to `.compozy/tasks/<name>/`:

```
.compozy/tasks/<name>/
  _prd.md           # Written by create-prd
  _techspec.md      # Written by create-techspec
  _tasks.md         # Master task list, written by create-tasks
  task_01.md        # Individual task files (NO underscore prefix)
  task_02.md
  task_N.md
```

The `_` prefix on `_prd.md`, `_techspec.md`, and `_tasks.md` marks them as meta documents. Task files (`task_*.md`) have no prefix — they are the executable units that Compozy processes.

## Implementation Design

### Skill 1: `create-prd`

**Identity**: PRD creation specialist producing business-focused Product Requirements Documents.

**Workflow (5 phases)**:

1. **Discovery** — Spawn two parallel Agent tool calls:
   - Agent 1: Explore codebase for relevant patterns, existing features, architecture
   - Agent 2: Web search for market trends, competitive analysis, user needs (3-5 searches)

2. **Understanding** — Ask clarifying questions one at a time (max 3 per round):
   - WHAT features users need
   - WHY it provides business value
   - WHO the target users are
   - Success criteria and constraints
   - NEVER ask technical questions (databases, APIs, frameworks, architecture)

3. **Options** — Present 2-3 product approaches with trade-offs. Lead with recommendation. Wait for user selection.

4. **Refinement** — Follow-up questions based on chosen approach. Confirm key decisions before proceeding.

5. **Creation** — Read `references/prd-template.md`, generate `_prd.md`, write to `.compozy/tasks/<name>/`.

**Critical Constraints**:
- Questions focus on WHAT/WHY, never HOW/WHERE/WHICH
- Must complete at least one clarification round before presenting options
- PRD describes user capabilities and business outcomes ONLY
- No databases, APIs, code structure, frameworks, testing, architecture
- If `_prd.md` already exists, read it first and operate in update mode
- If `.compozy/tasks/<name>/` doesn't exist, create it

**Template** (`references/prd-template.md`):
Derived from Compozy's `PRD_TEMPLATE` merged with Compozy's `_prd-template.md`:

```markdown
## Overview
[High-level overview: what problem it solves, who it's for, why it's valuable]

## Goals
[Specific, measurable objectives: success metrics, key metrics, business objectives]

## User Stories
[As a [type], I want [action] so that [benefit] — primary/secondary personas, main flows, edge cases]

## Core Features
[Main features: what it does, why important, high-level how, functional requirements]

## User Experience
[User journey: personas, key flows, UI/UX considerations, accessibility]

## High-Level Technical Constraints
[Required integrations, compliance mandates, performance targets, data privacy — NO implementation details]

## Non-Goals (Out of Scope)
[Explicitly excluded features, future considerations, boundaries]

## Phased Rollout Plan
[MVP → Phase 2 → Phase 3 with success criteria per phase]

## Success Metrics
[User engagement, performance benchmarks, business impact, quality attributes]

## Risks and Mitigations
[Non-technical risks: adoption, competition, timeline, resource constraints]

## Open Questions
[Remaining unclear requirements, edge cases, dependencies on external factors]
```

**Question Protocol** (`references/question-protocol.md`):
- Phase-based brainstorming: discovery → understanding → options → refinement → creation
- Max 3 questions per round
- Prefer multiple-choice when options can be predetermined
- Wait for answers before next round
- Must have clarity on purpose, constraints, and success criteria before presenting approaches
- YAGNI ruthlessly — remove non-essential features

---

### Skill 2: `create-techspec`

**Identity**: Technical specification specialist translating PRD business requirements into implementation designs.

**Workflow (4 phases)**:

1. **Context Gathering**:
   - Check for `_prd.md` in `.compozy/tasks/<name>/`. If exists, read it as primary input.
   - If no PRD, ask user for a description of what needs technical specification.
   - Spawn Agent to explore codebase: architecture patterns, existing components, dependencies, tech stack.

2. **Technical Clarification** — Ask technical questions one at a time:
   - Architecture approach and component boundaries
   - Data models and storage choices
   - API design and integration points
   - Testing strategy and performance requirements
   - This is the inverse of PRD — here we focus on HOW/WHERE/WHICH

3. **Design Proposal** — Present technical approach for approval:
   - System architecture overview
   - Key interfaces and data models
   - Implementation sequencing
   - Trade-offs and risks
   - Wait for user approval before writing

4. **Creation** — Read `references/techspec-template.md`, generate `_techspec.md`, write to `.compozy/tasks/<name>/`.

**Critical Constraints**:
- Prefers PRD as input but works without one
- Every PRD goal/user story should map to a technical component
- References PRD sections but doesn't duplicate business context
- If `_techspec.md` already exists, read it and operate in update mode

**Template** (`references/techspec-template.md`):
Derived from Compozy's `_techspec-template.md`:

```markdown
## Executive Summary
[Brief technical overview: key architectural decisions, implementation strategy]

## System Architecture
### Component Overview
[Main components, responsibilities, relationships, data flow]

## Implementation Design
### Core Interfaces
[Key service interfaces with code examples, <=20 lines per example]

### Data Models
[Core domain entities, request/response types, database schemas]

### API Endpoints
[Method, path, description, request/response format]

## Integration Points
[External services, authentication, error handling — only if applicable]

## Impact Analysis
[Table: affected component, type of impact, description & risk, required action]

## Testing Approach
### Unit Tests
[Strategy, key components, mock requirements, critical scenarios]

### Integration Tests
[Components to test together, test data requirements]

## Development Sequencing
### Build Order
[Ordered implementation sequence with dependencies]

### Technical Dependencies
[Blocking dependencies: infrastructure, external services, team deliverables]

## Monitoring & Observability
[Metrics, logs, alerting thresholds]

## Technical Considerations
### Key Decisions
[Choice rationale, trade-offs, rejected alternatives]

### Known Risks
[Challenges, mitigations, research areas]
```

---

### Skill 3: `create-tasks`

**Identity**: Task breakdown and enrichment specialist decomposing PRDs and TechSpecs into detailed, actionable task lists.

**Workflow (5 phases)**:

1. **Context Loading**:
   - Read `_prd.md` and `_techspec.md` from `.compozy/tasks/<name>/`. Warn if TechSpec is missing.
   - Spawn Agent to explore codebase: files to create/modify, test patterns, conventions.

2. **Task Breakdown**:
   - Decompose TechSpec implementation sections into granular, independently implementable tasks.
   - Each task gets: title, domain, type, scope, complexity, dependencies.
   - Tests embedded in each task — NEVER separate "test tasks".
   - Follow `references/task-template.md` structure.

3. **Interactive Approval**:
   - Present full task breakdown: titles + descriptions + complexity + dependencies.
   - Wait for user feedback. If changes requested, revise and present again.
   - Iterate until user approves.

4. **File Generation**:
   - Write `_tasks.md` (master list with all task titles, statuses, dependencies).
   - Write individual `task_01.md`, `task_02.md`, ... `task_N.md`.
   - Each file includes YAML frontmatter with `status`, `domain`, `type`, `scope`, `complexity`, and `dependencies`.

5. **Task Enrichment** (per-task, after approval):
   - For each task file, perform in-place enrichment:
     - **Already-enriched check**: If file has `## Overview`, `## Deliverables`, `## Tests` — skip.
     - **Context analysis**: Map task to PRD requirements and TechSpec guidance.
     - **Codebase exploration**: Spawn Agent to discover relevant files, dependent files, integration points, project rules/standards.
     - **Content generation**: Fill all template sections following `references/task-template.md`:
       - Overview (what + why, 2-3 sentences)
       - Requirements (specific, numbered)
       - Subtasks (3-7 checklist items, WHAT not HOW)
       - Implementation Details (file paths, integration points — reference TechSpec for patterns)
       - Relevant Files + Dependent Files (discovered paths)
       - Deliverables (concrete outputs + mandatory test items with >=80% coverage)
       - Tests (specific test cases as checklists)
       - Success Criteria (measurable outcomes)
     - **Complexity reassessment**: Re-evaluate based on exploration (low/medium/high/critical).
     - Update task file in-place with enriched content.

**Critical Constraints**:
- Every task must be independently implementable when its dependencies are met
- Every task MUST include Tests section and test items in Deliverables
- NEVER create separate tasks dedicated solely to testing
- Task numbering sequential, matching between `_tasks.md` and individual files
- Enrichment focuses on WHAT not HOW — implementation details stay in TechSpec
- Minimize code in tasks — show code only to illustrate current structure or problem areas
- If enrichment fails for one task, continue to next, report failures at end

**Task File Template** (`references/task-template.md`):

```markdown
---
status: pending
domain: [e.g., Authentication, API, Frontend]
type: [e.g., Feature Implementation, Bug Fix, Refactor]
scope: [e.g., Full, Partial]
complexity: [low, medium, high, critical]
dependencies:
  - task_01
  - task_02
---

# Task N: [Title]

## Overview
[2-3 sentences: what the task accomplishes, why it matters]

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- [Requirement 1 — specific technical requirement]
- [Requirement 2 — e.g., "MUST authenticate users via JWT tokens"]
- [Requirement 3]
</requirements>

## Subtasks
- [ ] N.1 [Subtask description]
- [ ] N.2 [Subtask description]
- [ ] N.3 [Subtask description]

## Implementation Details
[File paths to create/modify, integration points, dependencies.
Reference TechSpec implementation section for code patterns.]

### Relevant Files
- `path/to/file`

### Dependent Files
- `path/to/dependency`

## Deliverables
- [Concrete output 1]
- [Concrete output 2]
- Unit tests with 80%+ coverage **(REQUIRED)**
- Integration tests for [feature] **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] [Test case 1 — e.g., "Happy path authentication"]
  - [ ] [Test case 2 — e.g., "Invalid credentials handling"]
  - [ ] [Edge cases / error paths]
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- [Measurable outcome 1]
- [Measurable outcome 2]
```

**Task Context Schema** (`references/task-context-schema.md`):

```markdown
# Task Frontmatter Schema

Task metadata is parsed from YAML frontmatter by Compozy's `ParseTaskFile()` function
in `internal/core/prompt/common.go`.

## Required Fields

- `status`: One of `pending`, `in_progress`, `completed`, `done`, `finished`
- `domain`: Feature area (e.g., "Authentication", "API", "Frontend", "Database")
- `type`: Work type (e.g., "Feature Implementation", "Bug Fix", "Refactor", "Configuration")
- `scope`: Coverage (e.g., "Full", "Partial")
- `complexity`: One of: `low`, `medium`, `high`, `critical`
- `dependencies`: YAML list of task numbers (e.g., `["task_01", "task_02"]`) or `[]`

## Parser Compatibility

Compozy reads task files matching `task_\d+\.md` (no underscore prefix).
The file MUST start with YAML frontmatter for proper parsing.
```

---

### Go Code Changes

#### Change 1: `internal/core/plan/input.go`

Update the task file regex pattern in `readTaskEntries()`:

```go
// Before:
reTaskFile := regexp.MustCompile(`^_task_\d+\.md$`)

// After:
reTaskFile := regexp.MustCompile(`^task_\d+\.md$`)
```

#### Change 2: `internal/core/prompt/common.go`

Update `ExtractTaskNumber()` to match the new naming:

```go
// Before:
reTaskFile := regexp.MustCompile(`^_task_\d+\.md$`)
// ...
numStr := strings.TrimPrefix(filename, "_task_")

// After:
reTaskFile := regexp.MustCompile(`^task_\d+\.md$`)
// ...
numStr := strings.TrimPrefix(filename, "task_")
```

#### Impact

- Both changes are surgical — regex pattern + string prefix only.
- No other Go files reference `_task_` pattern.
- Existing `_prd.md`, `_techspec.md`, `_tasks.md` naming is unaffected.
- Tests in `prompt/prompt_test.go` will need updating to match new pattern.

---

## Integration Flow (End-to-End)

```
1. User has an idea/issue
         |
         v
2. create-prd skill
   |-- Parallel: codebase exploration + web research (Agent tool)
   |-- Brainstorming phases (questions -> options -> refinement)
   '-- Writes: .compozy/tasks/<name>/_prd.md
         |
         v
3. create-techspec skill
   |-- Reads _prd.md (if exists, works without)
   |-- Codebase exploration for architecture context
   |-- Technical clarification questions
   '-- Writes: .compozy/tasks/<name>/_techspec.md
         |
         v
4. create-tasks skill
   |-- Reads _prd.md + _techspec.md
   |-- Codebase exploration for files/patterns
   |-- Task breakdown -> interactive approval loop
   |-- Writes: _tasks.md + task_01.md ... task_N.md
   '-- Enrichment phase: fills each task with full detail
         |
         v
5. compozy start --tasks-dir .compozy/tasks/<name>/
   |-- Reads task_*.md files (updated Go pattern)
   |-- execute-prd-task skill runs each task
   '-- Tasks already enriched — straight to implementation
```

Each step is independent. A user can run just `create-prd`, or pick up from any point. All artifacts are plain markdown files in `.compozy/tasks/<name>/`.

## Testing Approach

### Skill Validation
- Run `python3 scripts/validate-metadata.py` for each skill's name + description
- Cross-reference each SKILL.md against `references/checklist.md`
- Verify each SKILL.md is under 500 lines
- Verify all templates are in `references/` (progressive disclosure)
- Verify no human docs (README.md, CHANGELOG.md, etc.)

### Go Code Validation
- Update existing tests in `prompt/prompt_test.go` for new `task_*.md` pattern
- Test `ExtractTaskNumber("task_01.md")` returns 1
- Test `ExtractTaskNumber("task_99.md")` returns 99
- Test `ExtractTaskNumber("_task_01.md")` returns 0 (old pattern no longer matches)
- Run `make verify` (fmt + lint + test)

### Integration Validation
- Create a sample `.compozy/tasks/test/` with task files using new naming
- Run `compozy start --tasks-dir .compozy/tasks/test/ --dry-run` to verify discovery
- Verify generated prompts reference correct file paths

## Development Sequencing

### Build Order

1. **Go code changes** (input.go + common.go + tests) — unblocks everything else
2. **create-prd skill** — first in the workflow chain, no dependencies on other skills
3. **create-techspec skill** — reads PRD output, can be developed in parallel with create-prd
4. **create-tasks skill** — depends on understanding both PRD and TechSpec output formats
5. **Integration testing** — end-to-end flow validation
