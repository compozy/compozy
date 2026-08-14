import type { LoopDefinitionGraph } from "../types";

const executeTaskPrompt = `Kickoff directive:
Begin work on {{ .item.title }} immediately. This run is the operator's authorization
to implement exactly this pending task — do NOT ask for confirmation, do NOT wait for
further instructions, and do NOT reply with a greeting before starting.

Required skills:
- cy-workflow-memory: use before editing code; the memory paths are listed below.
- cy-execute-task: the end-to-end execution workflow for this task.
- cy-final-verify: required before any completion claim or automatic commit; use it to
  identify and run the repository's real verification commands.

Task context:
Task file: {{ .item.path }}
Task id: {{ .item.id }}
Slug: {{ .inputs.slug }}
{{ if .item.blocks }}Depends on: {{ join ", " .item.blocks }}{{ end }}

Workflow memory:
- Memory directory: .compozy/tasks/{{ .inputs.slug }}/memory
- Shared memory: .compozy/tasks/{{ .inputs.slug }}/memory/MEMORY.md
- Task memory: .compozy/tasks/{{ .inputs.slug }}/memory/{{ .item.id }}.md
- Read both memory files before implementation and update them before finishing.
- Keep task-local decisions, learnings, touched surfaces, and corrections in the task
  memory file; promote only durable cross-task context into shared memory.

Scope and tracking:
- Read repository AGENTS.md/CLAUDE.md and surface-specific instructions before editing.
- Read .compozy/tasks/{{ .inputs.slug }}/_spec.md and _tasks.md when present and
  treat them plus the task body below as the source of truth.
- Keep scope tight to this task; record meaningful follow-up work instead of expanding
  scope silently.
- Preserve unrelated worktree changes.
- Fix production code for real; do not weaken tests or add compatibility shims.

Verification:
- Run focused checks for each changed surface.
- Execute every explicit Validation, Test Plan, or Testing item from the task body.
- Report exact commands and outcomes in the structured output.

Tracking and commits:
- Update task checkboxes and status in {{ .item.path }} only after implementation,
  verification evidence, and self-review are complete.
- Update .compozy/tasks/{{ .inputs.slug }}/_tasks.md only when this task is complete.
- Keep tracking-only files out of automatic commits.
{{ if .inputs.auto_commit -}}
- Create exactly one commit for this task after clean verification, self-review, and
  tracking updates. Do not push.
{{ else -}}
- Leave changes uncommitted for manual review. Do not push.
{{ end }}
Task body:
{{ .item.body }}

Closing directive:
You have the full brief above — start work on {{ .item.title }} now instead of
summarizing the plan back. Return \`status\`, \`summary\`, and \`files_changed\`. Use
only \`completed\` after all required implementation and verification work succeeds.`;

export const implementTasksGraph = {
  nodes: [
    { id: "slug_input", class: "source", kind: "input", input_ref: "slug" },
    {
      id: "load_tasks",
      class: "action",
      kind: "ext__dev_cycle__import_tasks",
      params: { pattern: ".compozy/tasks/{{ .inputs.slug }}/task_*.md" },
      produces: { tasks: "array" },
    },
    {
      id: "implement",
      class: "control",
      kind: "fan-out",
      collection: "{{ .nodes.load_tasks.output.tasks }}",
      batch_size: 1,
      max_parallel: 1,
      max_fan_out: 64,
    },
    {
      id: "execute_task",
      class: "action",
      kind: "run-agent",
      params: {
        agent: "{{ .inputs.implementer }}",
        prompt: executeTaskPrompt,
        output_schema: {
          type: "object",
          required: ["status", "summary"],
          properties: {
            status: { enum: ["completed"] },
            summary: { type: "string" },
            files_changed: { type: "array" },
          },
        },
      },
      session: { isolated: true },
      timeout: "45m0s",
      retry: { max_attempts: 2 },
    },
    { id: "collect", class: "control", kind: "collect" },
  ],
  edges: [
    { from: "slug_input", to: "load_tasks" },
    { from: "load_tasks", to: "implement" },
    { from: "implement", to: "execute_task" },
    { from: "execute_task", to: "collect" },
  ],
} as unknown as LoopDefinitionGraph;
