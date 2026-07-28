import type {
  OperationPath,
  OperationQuery,
  OperationRequestBody,
  OperationResponse,
} from "@/lib/api-contract";

/** Explicit wider decision accepted by the daemon-owned set contract. */
export type ToolApprovalGrantSetRequest = OperationRequestBody<"setToolApprovalGrant">;

/** Stored grant returned after an explicit wider set. */
export type ToolApprovalGrantSetResponse = OperationResponse<"setToolApprovalGrant", 200>;

/** Query params for set (`workspace_id` is required). */
export type SetToolApprovalGrantQuery = OperationQuery<"setToolApprovalGrant">;

/** The full daemon-owned list envelope: exact `grants` for the workspace plus its exact `total`. */
export type ToolApprovalGrantsListResponse = OperationResponse<"listToolApprovalGrants", 200>;

/** One remembered native-tool approval decision for a workspace. */
export type ToolApprovalGrant = ToolApprovalGrantsListResponse["grants"][number];

/** The remembered decision: `allow` or `reject`. */
export type ToolApprovalDecision = ToolApprovalGrant["decision"];

/** Query params for the workspace-scoped list read (`workspace_id` is required). */
export type ListToolApprovalGrantsQuery = OperationQuery<"listToolApprovalGrants">;

/** Query params for revoke (`workspace_id` is required). */
export type RevokeToolApprovalGrantQuery = OperationQuery<"revokeToolApprovalGrant">;

/** Path params for revoke (the grant `id`). */
export type RevokeToolApprovalGrantPath = OperationPath<"revokeToolApprovalGrant">;
