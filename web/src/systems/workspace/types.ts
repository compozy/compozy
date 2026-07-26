import type { OperationResponse } from "@/lib/api-contract";

export type WorkspacesResponse = OperationResponse<"listWorkspaces", 200>;
export type WorkspacePayload = WorkspacesResponse["workspaces"][number];
export type WorkspaceResponse = OperationResponse<"resolveWorkspace", 200>;
export type WorkspaceDetailPayload = OperationResponse<"getWorkspace", 200>;
export type SessionProviderOption = NonNullable<WorkspaceDetailPayload["providers"]>[number];
