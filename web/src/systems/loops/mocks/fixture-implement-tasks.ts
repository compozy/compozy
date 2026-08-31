import type { LoopDefinitionGraph } from "../types";
import { SPEC_CYCLE_IMPORT_TASKS_KIND } from "./fixture-action-kinds";

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
      kind: SPEC_CYCLE_IMPORT_TASKS_KIND,
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
      id: "select_mode",
      class: "control",
      kind: "route",
      routes: [{ when: "inputs.mode == 'orchestrated'", to: "stage_orchestrated" }],
      default: "select_category",
    },
    {
      id: "select_category",
      class: "control",
      kind: "route",
      routes: [
        { when: "item.type == 'backend'", to: "execute_backend" },
        { when: "item.type == 'frontend'", to: "execute_frontend" },
      ],
      default: "execute_default",
    },
    {
      id: "stage_orchestrated",
      class: "action",
      kind: "transform",
      params: {
        map: {
          task_id: { value: "{{ .item.id }}" },
          status: { value: "staged" },
        },
      },
      produces: { task_id: "string", status: "string" },
    },
    ...[
      ["execute_backend", "backend_runtime"],
      ["execute_frontend", "frontend_runtime"],
      ["execute_default", "default_runtime"],
    ].map(([id, runtimeInput]) => ({
      id,
      class: "action",
      kind: "run-agent",
      params: {
        agent: "{{ .inputs.implementer }}",
        runtime: `{{ .inputs.${runtimeInput} }}`,
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
    })),
    { id: "collect", class: "control", kind: "collect" },
    {
      id: "select_delivery",
      class: "control",
      kind: "route",
      routes: [{ when: "inputs.mode == 'orchestrated'", to: "orchestrate" }],
      default: "per_task_done",
    },
    {
      id: "per_task_done",
      class: "action",
      kind: "transform",
      params: { map: { status: { value: "completed" } } },
      produces: { status: "string" },
    },
    {
      id: "orchestrate",
      class: "action",
      kind: "goal",
      session: { mode: "continuous" },
      params: {
        agent: "{{ .inputs.orchestrator }}",
        runtime: "{{ .inputs.orchestrator_runtime }}",
        objective:
          "Conduct the task graph with one bounded {{ .inputs.implementer }} worker per task and preserve each category runtime.",
        judge: [{ id: "tasks_completed", type: "command", check: "test -d .compozy/tasks" }],
        max_turns: 12,
        output_schema: {
          type: "object",
          required: ["status"],
          properties: {
            status: { enum: ["complete", "blocked"] },
            summary: { type: "string" },
            tasks: { type: "array" },
          },
        },
      },
    },
  ],
  edges: [
    { from: "slug_input", to: "load_tasks" },
    { from: "load_tasks", to: "implement" },
    { from: "implement", to: "select_mode" },
    { from: "select_mode", to: "select_category" },
    { from: "select_mode", to: "stage_orchestrated" },
    { from: "select_category", to: "execute_backend" },
    { from: "select_category", to: "execute_frontend" },
    { from: "select_category", to: "execute_default" },
    { from: "stage_orchestrated", to: "collect" },
    { from: "execute_backend", to: "collect" },
    { from: "execute_frontend", to: "collect" },
    { from: "execute_default", to: "collect" },
    { from: "collect", to: "select_delivery" },
    { from: "select_delivery", to: "per_task_done" },
    { from: "select_delivery", to: "orchestrate" },
  ],
} as unknown as LoopDefinitionGraph;
