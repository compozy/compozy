import type {
  ArchiveResult,
  SyncResult,
  TaskBoardPayload,
  TaskDetailPayload,
  WorkflowSummary,
} from "../types";
import { workspaceFixture } from "@/systems/app-shell/mocks";

export const workflowAlphaFixture: WorkflowSummary = {
  id: "wf-alpha",
  slug: "alpha",
  workspace_id: workspaceFixture.id,
  last_synced_at: "2026-04-20T01:00:00Z",
};

export const workflowBetaArchivedFixture: WorkflowSummary = {
  id: "wf-beta",
  slug: "beta",
  workspace_id: workspaceFixture.id,
  archived_at: "2026-04-19T14:00:00Z",
};

export const workflowsFixture: WorkflowSummary[] = [
  workflowAlphaFixture,
  workflowBetaArchivedFixture,
];

export const taskBoardFixture: TaskBoardPayload = {
  workspace: workspaceFixture,
  workflow: {
    id: workflowAlphaFixture.id,
    slug: workflowAlphaFixture.slug,
    workspace_id: workflowAlphaFixture.workspace_id,
  },
  task_counts: {
    total: 2,
    completed: 1,
    pending: 1,
  },
  lanes: [
    {
      status: "pending",
      title: "Pending",
      items: [
        {
          task_id: "task_01",
          task_number: 1,
          title: "Bootstrap workspace",
          status: "pending",
          type: "frontend",
          depends_on: [],
          updated_at: "2026-04-20T01:10:00Z",
        },
      ],
    },
    {
      status: "completed",
      title: "Completed",
      items: [
        {
          task_id: "task_00",
          task_number: 0,
          title: "Scaffold repository",
          status: "completed",
          type: "infra",
          depends_on: [],
          updated_at: "2026-04-20T00:30:00Z",
        },
      ],
    },
  ],
};

export const emptyTaskBoardFixture: TaskBoardPayload = {
  ...taskBoardFixture,
  task_counts: {
    total: 0,
    completed: 0,
    pending: 0,
  },
  lanes: [],
};

export const taskDetailFixture: TaskDetailPayload = {
  workspace: workspaceFixture,
  workflow: {
    id: workflowAlphaFixture.id,
    slug: workflowAlphaFixture.slug,
    workspace_id: workflowAlphaFixture.workspace_id,
  },
  task: {
    task_id: "task_02",
    task_number: 2,
    title: "Implement Storybook route harness",
    status: "in_progress",
    type: "frontend",
    depends_on: ["task_01"],
    updated_at: "2026-04-20T03:00:00Z",
  },
  document: {
    id: "task-doc",
    kind: "task",
    title: "Implement Storybook route harness",
    updated_at: "2026-04-20T03:00:00Z",
    markdown: "## Task body\nCreate mocked route states backed by MSW.",
  },
  memory_entries: [
    {
      file_id: "memory-task-02",
      display_path: ".compozy/tasks/daemon-web-ui/memory/task_13.md",
      kind: "task",
      title: "task_13.md",
      size_bytes: 420,
      updated_at: "2026-04-20T03:00:00Z",
    },
  ],
  related_runs: [
    {
      run_id: "run-task-02",
      mode: "task",
      presentation_mode: "text",
      workspace_id: workspaceFixture.id,
      started_at: "2026-04-20T03:10:00Z",
      status: "running",
      workflow_slug: workflowAlphaFixture.slug,
    },
  ],
  live_tail_available: true,
};

export const sparseTaskDetailFixture: TaskDetailPayload = {
  ...taskDetailFixture,
  task: {
    task_id: "task_03",
    task_number: 3,
    title: "Wire regression tests",
    status: "pending",
    type: "frontend",
    updated_at: "2026-04-20T04:00:00Z",
  },
  document: {
    id: "task-doc-empty",
    kind: "task",
    title: "Wire regression tests",
    updated_at: "2026-04-20T04:00:00Z",
    markdown: "",
  },
  memory_entries: [],
  related_runs: [],
  live_tail_available: false,
};

export const workflowSyncResultFixture: SyncResult = {
  workflows_scanned: 1,
  task_items_upserted: 2,
} as SyncResult;

export const workflowArchiveResultFixture: ArchiveResult = {
  archived: true,
} as ArchiveResult;
