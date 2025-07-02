---
status: pending
---

<task_context>
<domain>cli/init</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>templates,validation</dependencies>
</task_context>

# Task 2.0: Project Initialization Command

## Overview

Create the `compozy init` command that guides users through project setup with interactive prompts, creates the project structure, and provides example workflows and agents to help users get started quickly.

## Subtasks

- [ ] 2.1 Create init command with interactive prompts using Bubble Tea
- [ ] 2.2 Build project scaffolding templates with embedded examples
- [ ] 2.3 Implement validation framework for project names and structure
- [ ] 2.4 Add git integration for repository initialization
- [ ] 2.5 Create example workflows, agents, and tools for common use cases
- [ ] 2.6 Build project structure validator to ensure consistency

## Implementation Details

### Interactive Flow

1. **Project name**: Validate against naming rules
2. **Project type**: API automation, data pipeline, AI workflow
3. **Features**: Select components (workflows, agents, tools)
4. **Examples**: Choose from templates or start blank
5. **Git init**: Optionally initialize git repository

### Project Structure

```
my-project/
├── compozy.yaml          # Project configuration
├── .compozy/             # Local settings (gitignored)
├── workflows/            # Workflow definitions
│   └── example.yaml
├── agents/               # Agent configurations
│   └── assistant.yaml
├── tools/                # Custom tools
│   └── hello.ts
├── .gitignore            # Pre-configured
└── README.md             # Generated documentation
```

### Template System

- Embedded templates using Go embed
- Variable substitution for project-specific values
- Multiple template sets for different use cases
- Ability to extend with custom templates

### Validation Rules

- Project name: lowercase, alphanumeric, hyphens
- No conflicts with existing directories
- Valid YAML structure for all configs
- Required fields populated

## Success Criteria

- [ ] Interactive prompts are intuitive and guide users effectively
- [ ] Generated project structure follows best practices
- [ ] Example workflows run successfully out of the box
- [ ] Validation catches common errors before file creation
- [ ] Git integration works across platforms
- [ ] Non-interactive mode available for CI/CD (--yes flag)
- [ ] Generated projects pass `compozy validate` checks

<critical>
**MANDATORY REQUIREMENTS:**

- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing parent tasks
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
