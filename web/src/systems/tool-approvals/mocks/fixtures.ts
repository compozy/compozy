import type { ToolApprovalGrant, ToolApprovalGrantsListResponse } from "@/systems/tool-approvals";

/**
 * Fixtures cover both explicit wider scopes and one prompt-origin exact decision. They use
 * real catalog tool ids and stay newest-first to match daemon `created_at DESC` ordering.
 */
export const toolApprovalGrantFixtures: ToolApprovalGrant[] = [
  {
    id: "5f3a1c2e-8b7d-4a6f-9c1e-2d3b4a5c6d7e",
    workspace_id: "ws_default",
    agent_name: "claude-code",
    tool_id: "agh__config_set",
    decision: "allow",
    created_at: "2026-07-14T09:12:00Z",
    last_used_at: "2026-07-15T08:40:00Z",
  },
  {
    id: "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
    workspace_id: "ws_default",
    tool_id: "agh__task_create",
    decision: "reject",
    created_at: "2026-07-13T17:05:00Z",
    last_used_at: "2026-07-14T22:15:00Z",
  },
  {
    id: "c9d8e7f6-a5b4-4c3d-9e2f-1a0b9c8d7e6f",
    workspace_id: "ws_default",
    agent_name: "openclaw",
    tool_id: "agh__memory_note",
    input_digest: "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
    decision: "allow",
    created_at: "2026-07-12T11:00:00Z",
    last_used_at: "2026-07-15T07:55:00Z",
  },
];

export const toolApprovalGrantsResponseFixture: ToolApprovalGrantsListResponse = {
  grants: toolApprovalGrantFixtures,
  total: toolApprovalGrantFixtures.length,
};

export const emptyToolApprovalGrantsResponseFixture: ToolApprovalGrantsListResponse = {
  grants: [],
  total: 0,
};
