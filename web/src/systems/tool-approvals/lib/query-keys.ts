/**
 * Query keys for remembered native-tool approval decisions.
 *
 * The workspace id is a positional key segment so every read and every mutation
 * reconciliation targets exactly one workspace's list — no cross-workspace bleed.
 */
export const toolApprovalKeys = {
  all: ["tool-approval-grants"] as const,
  list: (workspaceId: string) => [...toolApprovalKeys.all, "list", workspaceId] as const,
};
