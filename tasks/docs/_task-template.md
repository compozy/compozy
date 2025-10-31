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

[**FOR LLM: DON'T CHANGE THIS BLOCK**]
<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the _techspec.md and the _prd.md docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

[**FOR LLM: DON'T CHANGE THIS BLOCK**]
<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
[List of mandatory requirements]
</requirements>

## Subtasks

- [ ] X.1 [Subtask description]
- [ ] X.2 [Subtask description]

## Implementation Details [**FOR LLM: KEEP THIS BRIEFLY AND HIGH-LEVEL, THE IMPLEMENTATION ALREADY EXIST IN THE TECHSPEC**]

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
