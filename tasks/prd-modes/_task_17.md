## status: completed

<task_context>
<domain>documentation</domain>
<type>documentation</type>
<scope>quick_start_guides</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 17.0: Update Quick Start

## Overview

Update quick start documentation to reflect memory mode as the new default. Simplify getting started experience by emphasizing zero-dependency setup and provide clear guidance on when to use other modes.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start (tasks/prd-modes/_techspec.md Phase 4.4)
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha
</critical>

<requirements>
- Update quick start to emphasize memory mode as default
- Simplify getting started steps (no external dependencies)
- Provide clear "next steps" for other modes
- All examples must work with memory mode
- Brief explanation of mode options without overwhelming new users
</requirements>

## Subtasks

- [x] 17.1 Update installation and first run section
- [x] 17.2 Emphasize zero-dependency default (memory mode)
- [x] 17.3 Add brief mode selection guidance
- [x] 17.4 Update example workflow to work in memory mode
- [x] 17.5 Add "next steps" section with links to other modes

## Implementation Details

See `tasks/prd-modes/_techspec.md` Section 4.4 for complete implementation details.

**Key Updates:**

**Getting Started:**
```bash
# Install
brew install compozy

# Start (default: memory mode, no external deps)
compozy start

# Your first workflow
compozy workflow run examples/hello-world.yaml
```

**Mode Guidance (Brief):**
- **Default mode:** memory (fastest, no persistence)
- **Need persistence?** Add `mode: persistent` to config
- **Production?** Use `mode: distributed` with external services

Keep quick start focused on getting users running immediately. Defer detailed mode discussions to deployment guides.

### Relevant Files

- `docs/content/docs/quick-start/index.mdx` (PRIMARY)

### Dependent Files

- `docs/content/docs/deployment/memory-mode.mdx` (Task 14.0)
- `docs/content/docs/deployment/persistent-mode.mdx` (Task 14.0)
- `docs/content/docs/deployment/distributed-mode.mdx` (Task 14.0)

## Deliverables

- [x] Updated `quick-start/index.mdx` with memory mode as default
- [x] Simplified getting started steps
- [x] Brief mode selection guidance
- [x] Working example workflow
- [x] Clear "next steps" section with mode links

## Tests

Documentation verification (no automated tests):
- [x] Installation commands are correct
- [x] `compozy start` works without configuration
- [x] Example workflow runs successfully
- [x] Links to mode documentation work
- [x] Quick start doesn't overwhelm with options
- [x] Clear path from quick start to production deployment

## Success Criteria

- Quick start emphasizes simplicity (zero dependencies)
- Memory mode is clearly the default
- Getting started steps work immediately
- Mode selection guidance is brief but helpful
- Clear progression path to other modes
- No confusion about which mode to start with
