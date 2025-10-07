## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/[subdomain]</domain>
<type>implementation|integration|testing|documentation</type>
<scope>core_feature|middleware|configuration|performance</scope>
<complexity>low|medium|high</complexity>
<dependencies>external_apis|database|temporal|http_server</dependencies>
</task_context>

# Task X.0: [Parent Task Title]

## Overview

[Brief description of task]

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
[List of mandatory requirements]
</requirements>

## Subtasks

- [ ] X.1 [Subtask description]
- [ ] X.2 [Subtask description]

## Implementation Details

[Relevant sections from tech spec]

### Relevant Files

- `path/to/file.go`

### Dependent Files

- `path/to/dependency.go`

## Deliverables

- [List of artifacts that constitute "done" for this task]
- [APIs/endpoints/config updates, migrations, docs]

## Tests

- Unit tests mapped from `_tests.md` for this feature:
  - [ ] [Test case 1]
  - [ ] [Test case 2]
  - [ ] [Edge cases / error paths]

## Success Criteria

- [Measurable outcomes]
- [Quality requirements]
