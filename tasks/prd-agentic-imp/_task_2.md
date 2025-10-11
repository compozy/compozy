## markdown

## status: completed

<task_context>
<domain>engine/llm/prompting</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 2.0: Prompt Template & Structured Output System

## Overview

Create a template-driven prompt system that supports dynamic exemplars, provider-specific variants, and proactive structured-output priming aligned with the new provider capability layer.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Externalize system/user prompt scaffolding into reusable templates with parameter binding.
- Support provider-specific prompt variants leveraging capability flags (JSON mode, schema hints).
- Inject dynamic examples and failure guidance per iteration via template slots.
- Document template structure and ensure backward-compatible defaults during rollout.
</requirements>

## Subtasks

- [x] 2.1 Design template format (e.g., Go text/template) and migrate existing system prompt assembly.
- [x] 2.2 Add provider variant resolution using capabilities exposed in Task 1.0.
- [x] 2.3 Integrate dynamic shot/examples selection, including failure guidance injection hooks from loop state.
- [x] 2.4 Update tests to cover prompt rendering with and without provider structured-output support.
- [x] 2.5 Document template usage and rollout strategy for future prompt experimentation.

## Implementation Details

- PRD §B “Prompt System Upgrade” calls for template-driven prompts, structured-output enforcement, and dynamic examples.
- Maintain compatibility by loading current prompts as default template data until new ones are authored.
- Ensure template rendering respects `no-linebreaks` rule when embedding into messages.

### Relevant Files

- `engine/llm/prompt_builder.go`
- `engine/llm/orchestrator/prompts/*`
- `engine/llm/orchestrator/loop.go`

### Dependent Files

- `engine/llm/orchestrator/request_builder.go`
- `engine/llm/orchestrator/response_handler.go`

## Deliverables

- Template-based prompt builder supporting provider variants and dynamic examples.
- Updated orchestrator integration using the new prompt system.
- Developer documentation describing template structure and configuration knobs.

## Tests

- Unit tests mapped from PRD test strategy:
  - [x] Native JSON mode: schema present → zero finalization retries.
  - [x] Prompt fallback: structured-output disabled providers still render legacy prompts.
  - [x] Dynamic example injection verified via golden template tests.

## Success Criteria

- Prompt changes isolated to template files plus builder logic.
- Provider capability flags automatically adjust structured-output priming.
- All relevant tests pass, including regression coverage for existing prompt flows.
- `make fmt && make lint && make test` pass.
