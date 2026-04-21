import type { WorkflowSpecDocument } from "../types";
import { workspaceFixture } from "@/systems/app-shell/mocks";
import { workflowAlphaFixture } from "@/systems/workflows/mocks";

export const workflowSpecFixture: WorkflowSpecDocument = {
  workspace: workspaceFixture,
  workflow: {
    id: workflowAlphaFixture.id,
    slug: workflowAlphaFixture.slug,
    workspace_id: workflowAlphaFixture.workspace_id,
  },
  prd: {
    id: "prd",
    kind: "prd",
    title: "PRD: daemon web UI",
    updated_at: "2026-04-20T01:00:00Z",
    markdown: "# Product requirements\nAdd Storybook coverage.",
  },
  techspec: {
    id: "techspec",
    kind: "techspec",
    title: "TechSpec: daemon web UI",
    updated_at: "2026-04-20T01:30:00Z",
    markdown: "# Technical specification\nMirror AGH's runtime topology.",
  },
  adrs: [
    {
      id: "adr-001",
      kind: "adr",
      title: "ADR-001: topology",
      updated_at: "2026-04-20T00:30:00Z",
      markdown: "Keep `web/` and `packages/ui/`.",
    },
  ],
};

export const partialWorkflowSpecFixture: WorkflowSpecDocument = {
  ...workflowSpecFixture,
  prd: undefined,
  adrs: [],
};
