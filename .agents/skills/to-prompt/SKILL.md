---
name: to-prompt
description: Transform code, issues, or context into a detailed prompt/context for another LLM to fix or implement. Use when preparing comprehensive context for external LLM assistance, bug fixes, improvements, or feature implementations. Provides detailed context without implementation suggestions, letting the receiving LLM decide how to implement solutions. Focuses on "what" (problem, requirements, current state) not "how" (implementation approach).
---

# To Prompt

Transform code, issues, or context into detailed prompts for another LLM to fix or implement.

## Overview

This skill helps create comprehensive, context-rich prompts for external LLM assistance. The goal is to provide all necessary context about the problem, current state, and requirements without prescribing implementation approaches. The receiving LLM decides how to implement the solution based on the context provided.

## Core Principles

<critical>
- **MUST** explain all context comprehensively and in detail
- **MUST** show code snippets only to illustrate current implementation, structure, or problem areas
- **MUST NOT** include implementation suggestions, solutions, or "how to fix" instructions
- **MUST NOT** show example solutions, code patterns, or step-by-step guides
- **MUST** let the receiving LLM decide how to implement the solution based on the context provided
- **MUST** focus on "what" (problem, requirements, current state) not "how" (implementation approach)
</critical>

## Task Type Guidance

### Bug Fix

When transforming a bug fix task, ensure the prompt includes:

- **Reproduction steps**: Exact steps to reproduce the bug consistently
- **Error messages and logs**: Complete error messages, stack traces, console logs, and any diagnostic output
- **Current behavior**: What actually happens when the bug occurs
- **Expected behavior**: What should happen instead
- **Environment context**: OS, browser, Node version, dependencies versions, configuration
- **Recent changes**: What changed recently that might have introduced the bug (git history, recent commits)
- **Affected code**: Show the current implementation of code paths involved in the bug
- **Related components**: Files, modules, or systems that interact with the buggy code
- **Regression tests**: Current tests (if any) and what regression tests should be written to prevent the bug from recurring
- **Impact**: Who/what is affected by this bug and severity

### Improvement

When transforming an improvement task, ensure the prompt includes:

- **Current state**: Detailed description of how things work now
- **Current implementation**: Code showing the existing approach
- **What needs improvement**: Specific aspects that need enhancement (performance, maintainability, usability, etc.)
- **Constraints**: Technical constraints, backward compatibility requirements, or limitations
- **Success criteria**: How to measure if the improvement is successful
- **Related code**: Files and modules that will be affected
- **Dependencies**: External libraries, APIs, or systems involved
- **User impact**: How users will benefit from the improvement
- **Non-goals**: What should NOT be changed or improved

### Feature

When transforming a feature task, ensure the prompt includes:

- **Requirements**: Complete functional requirements and user stories
- **Current system context**: How the system works now and where the feature fits
- **Integration points**: Where the feature connects with existing code
- **Data models**: Current data structures and what needs to be added/modified
- **API contracts**: Existing APIs and what new endpoints or methods are needed
- **User flows**: How users will interact with the feature
- **Edge cases**: Boundary conditions and special scenarios to consider
- **Constraints**: Technical, business, or design constraints
- **Dependencies**: External services, libraries, or systems required
- **Testing requirements**: What needs to be tested (unit, integration, E2E)

## What NOT to Include

- Implementation suggestions or "how to fix" instructions
- Example solutions or code patterns to follow
- Step-by-step implementation guides
- Prescribed approaches or methodologies
- "Before/after" code examples showing solutions

## Usage

When asked to transform code, issues, or context into a prompt:

1. **Gather comprehensive context**: Collect all relevant information about the problem, current state, and requirements
2. **Show current code**: Include code snippets to illustrate the current implementation, structure, or problem areas
3. **Describe the problem**: Clearly explain what needs to be fixed, improved, or implemented
4. **Provide context**: Include environment details, related components, dependencies, and constraints
5. **Avoid solutions**: Do not include implementation suggestions, examples, or step-by-step guides
6. **Focus on "what"**: Describe the problem, requirements, and current state, not how to solve it

The resulting prompt should be comprehensive enough for another LLM to understand the full context and decide on the best implementation approach independently.
