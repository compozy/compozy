import type { ToolApprovalGrant, ToolApprovalGrantsListResponse } from "@/systems/tool-approvals";

const DEFAULT_PROFILE_OWNER = {
  profile_archived: false,
  profile_color: "#8a8f98",
  profile_id: "00000000000000000000000000",
  profile_name: "default",
} satisfies Pick<
  ToolApprovalGrant,
  "profile_archived" | "profile_color" | "profile_id" | "profile_name"
>;

/**
 * Fixtures cover both explicit wider scopes and one prompt-origin exact decision. They use
 * real catalog tool ids and stay newest-first to match daemon `created_at DESC` ordering.
 */
export const toolApprovalGrantFixtures: ToolApprovalGrant[] = [
  {
    ...DEFAULT_PROFILE_OWNER,
    id: "5f3a1c2e-8b7d-4a6f-9c1e-2d3b4a5c6d7e",
    workspace_id: "ws_default",
    agent_name: "claude-code",
    tool_id: "compozy__config_set",
    decision: "allow",
    created_at: "2026-07-14T09:12:00Z",
    last_used_at: "2026-07-15T08:40:00Z",
  },
  {
    ...DEFAULT_PROFILE_OWNER,
    id: "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
    workspace_id: "ws_default",
    tool_id: "compozy__task_create",
    decision: "reject",
    created_at: "2026-07-13T17:05:00Z",
    last_used_at: "2026-07-14T22:15:00Z",
  },
  {
    ...DEFAULT_PROFILE_OWNER,
    id: "c9d8e7f6-a5b4-4c3d-9e2f-1a0b9c8d7e6f",
    workspace_id: "ws_default",
    agent_name: "openclaw",
    tool_id: "compozy__memory_note",
    input_digest: "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
    decision: "allow",
    created_at: "2026-07-12T11:00:00Z",
    last_used_at: "2026-07-15T07:55:00Z",
  },
];

/**
 * Terminal permissions are remembered decisions like any other.
 *
 * They are stored in the same place, which is why they appear here rather than
 * in a second fixture set — only their reading differs.
 */
export const terminalToolApprovalGrantFixtures: ToolApprovalGrant[] = [
  {
    ...DEFAULT_PROFILE_OWNER,
    id: "b7c8d9e0-f1a2-4b3c-8d4e-5f6a7b8c9d0e",
    workspace_id: "ws_default",
    agent_name: "claude-code",
    tool_id: "compozy__terminal_write",
    // A digest of the exact tool input. The daemon validates the `sha256:`
    // prefix and never returns the input itself, so a terminal id here would be
    // a shape the runtime cannot produce.
    input_digest: "sha256:9f21ac04b7e31d5a8c6f0e2b4d7a19c3e58f6b0d2a4c8e1f3b5d7a9c1e3f5b7d",
    decision: "allow",
    created_at: "2026-07-15T12:44:00Z",
    last_used_at: "2026-07-15T12:46:00Z",
  },
  {
    ...DEFAULT_PROFILE_OWNER,
    id: "d0e1f2a3-b4c5-4d6e-9f0a-1b2c3d4e5f6a",
    workspace_id: "ws_default",
    agent_name: "claude-code",
    tool_id: "compozy__terminal_exec",
    input_digest: "sha256:1e8f7a55c4020b3d6e9a2c5f8b1d4e7a0c3f6b9d2e5a8c1f4b7d0e3a6c9f2b5d",
    decision: "allow",
    created_at: "2026-07-15T12:12:00Z",
    last_used_at: "2026-07-15T12:40:00Z",
  },
];

export const toolApprovalGrantsResponseFixture: ToolApprovalGrantsListResponse = {
  grants: toolApprovalGrantFixtures,
  total: toolApprovalGrantFixtures.length,
};

export const terminalInclusiveToolApprovalGrantsResponseFixture: ToolApprovalGrantsListResponse = {
  grants: [...terminalToolApprovalGrantFixtures, ...toolApprovalGrantFixtures],
  total: terminalToolApprovalGrantFixtures.length + toolApprovalGrantFixtures.length,
};

export const emptyToolApprovalGrantsResponseFixture: ToolApprovalGrantsListResponse = {
  grants: [],
  total: 0,
};
