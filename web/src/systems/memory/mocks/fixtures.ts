import type { MarkdownDocument, WorkflowMemoryIndex } from "../types";
import { workspaceFixture } from "@/systems/app-shell/mocks";
import { workflowAlphaFixture } from "@/systems/workflows/mocks";

export const workflowMemoryIndexFixture: WorkflowMemoryIndex = {
  workspace: workspaceFixture,
  workflow: {
    id: workflowAlphaFixture.id,
    slug: workflowAlphaFixture.slug,
    workspace_id: workflowAlphaFixture.workspace_id,
  },
  entries: [
    {
      file_id: "file-shared",
      display_path: ".compozy/tasks/daemon-web-ui/memory/MEMORY.md",
      kind: "shared",
      size_bytes: 2048,
      title: "MEMORY.md",
      updated_at: "2026-04-20T03:00:00Z",
    },
    {
      file_id: "file-task-13",
      display_path: ".compozy/tasks/daemon-web-ui/memory/task_13.md",
      kind: "task",
      size_bytes: 768,
      title: "task_13.md",
      updated_at: "2026-04-20T03:10:00Z",
    },
  ],
};

export const emptyWorkflowMemoryIndexFixture: WorkflowMemoryIndex = {
  ...workflowMemoryIndexFixture,
  entries: [],
};

export const workflowMemoryDocumentFixture: MarkdownDocument = {
  id: "memory-file-shared",
  kind: "shared",
  title: "MEMORY.md",
  updated_at: "2026-04-20T03:00:00Z",
  markdown: "## Shared workflow memory\nRoute-story harness is pending.",
};
