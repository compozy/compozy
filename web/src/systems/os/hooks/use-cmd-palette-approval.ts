import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { resolvePendingToolApproval } from "../adapters/cmd-palette-api";
import { cmdPaletteApprovalOptions } from "../lib/cmd-palette-query-options";
import type {
  CmdPaletteApprovalDecision,
  CmdPaletteApprovalStatus,
} from "../lib/cmd-palette-types";
import type { CmdPaletteApprovalIntent } from "../stores/cmd-palette-execution-store";

function approvalMessage(status: CmdPaletteApprovalStatus | undefined): string {
  if (!status) return "Loading approval…";
  switch (status.approval_status) {
    case "denied":
      return "Command denied. The command did not run.";
    case "timeout":
      return "Approval expired. The command did not run.";
    case "canceled":
      return "Approval canceled. The command did not run.";
    case "pending":
      return "This command needs your approval.";
  }
  switch (status.execution_status) {
    case "completed":
      return "Command finished.";
    case "failed":
      return "Command failed.";
    case "uncertain":
      return "The command's result could not be confirmed.";
    default:
      return "Approved. Waiting for the command to finish.";
  }
}

function approvalFailureMessage(error: unknown): string | undefined {
  if (error === null || typeof error !== "object") return undefined;
  const message = Reflect.get(error, "message");
  return typeof message === "string" ? message : undefined;
}

export function useCmdPaletteApproval(approval: CmdPaletteApprovalIntent) {
  const queryClient = useQueryClient();
  const options = cmdPaletteApprovalOptions(approval.profile, approval.id);
  const query = useQuery(options);
  const decision = useMutation({
    mutationFn: (outcome: CmdPaletteApprovalDecision) =>
      resolvePendingToolApproval(approval.profile, approval.id, outcome),
    onSuccess: status => queryClient.setQueryData(options.queryKey, status),
    onSettled: () => queryClient.invalidateQueries({ queryKey: options.queryKey }),
  });
  const status = query.data;
  const canDecide = status?.approval_status === "pending" && !decision.isPending && !query.isError;
  const isRunning =
    status?.approval_status === "approved" && status.execution_status === "dispatching";
  const error = decision.error ?? query.error;
  return {
    canDecide,
    isPending: decision.isPending || query.isPending || isRunning,
    message: approvalMessage(status),
    error:
      error?.message ??
      (status?.execution_status === "failed" ? approvalFailureMessage(status.error) : null),
    decide: (outcome: CmdPaletteApprovalDecision) => {
      if (canDecide) decision.mutate(outcome);
    },
  };
}
