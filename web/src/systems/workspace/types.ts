import type { OperationResponse } from "@/lib/api-contract";

export type WorkspacesResponse = OperationResponse<"listWorkspaces", 200>;
export type WorkspacePayload = WorkspacesResponse["workspaces"][number];
export type WorkspaceResponse = OperationResponse<"resolveWorkspace", 200>;
export type WorkspaceDetailPayload = OperationResponse<"getWorkspace", 200>;
export type SessionProviderOption = NonNullable<WorkspaceDetailPayload["providers"]>[number];

export type WorktreesResponse = OperationResponse<"listWorktrees", 200>;
/** Adopted/registered worktree record. Discovered entries are a separate array. */
export type WorktreePayload = WorktreesResponse["worktrees"][number];
/** Git-known checkout with no Compozy record yet — selecting it is the adoption gesture. */
export type DiscoveredWorktreePayload = WorktreesResponse["discovered"][number];
export type WorktreeRepoState = WorktreesResponse["repo"];
export type WorktreeStatusPayload = OperationResponse<"getWorktreeStatus", 200>;
export type WorktreeCatalogEventPayload = OperationResponse<"streamWorktreeCatalog", 200>;

/**
 * Presentational state vocabulary. `discovered` is derived (no record exists) and
 * `error` presents a status read failure, which the wire keeps distinct from the
 * `failed` lifecycle state (TechSpec N-004).
 */
export type WorktreeDisplayState =
  | "ready"
  | "pending"
  | "discovered"
  | "missing"
  | "error"
  | "failed";
