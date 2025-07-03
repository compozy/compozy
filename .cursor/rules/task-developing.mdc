---
description:
globs:
alwaysApply: true
---
# Task List Management

Guidelines for managing task lists in markdown files to track progress on completing a PRD and its technical specification

## Task Implementation

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** check dependent files APIs before write tests to avoid write wrong code
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing ANY subtask
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

<completion_protocol>
1. When you finish a **sub‑task**, immediately mark it as completed by changing `[ ]` to `[x]`.
2. If **all** subtasks underneath a parent task are now `[x]`, also mark the **parent task** as completed.
- Stop after each sub‑task and wait for the user's go‑ahead.
</completion_protocol>

## Task List Maintenance

<maintenance_rules>
1. **Update the task list as you work:**
   - Mark tasks and subtasks as completed (`[x]`) per the protocol above.
   - Add new tasks as they emerge.

2. **Maintain the "Relevant Files" section:**
   - List every file created or modified.
   - Give each file a one‑line description of its purpose.
</maintenance_rules>

## AI Instructions

<ai_workflow>
When working with task lists, the AI must:

1. Regularly update the task list file after finishing any significant work.
2. Follow the completion protocol:
   - Mark each finished **sub‑task** `[x]`.
   - Mark the **parent task** `[x]` once **all** its subtasks are `[x]`.
3. Add newly discovered tasks.
4. Keep "Relevant Files" accurate and up to date.
5. Before starting work, check which sub‑task is next.
6. After implementing a sub‑task, update the file and then pause for user approval.
7. **ALWAYS** use the task.mdc file as the primary reference for implementation
8. **NEVER** skip test assertions defined in the task requirements
</ai_workflow>
