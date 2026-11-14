import { Pill } from "@compozy/ui";

import type { MCPApprovalReason } from "../lib/mcp-runtime-view-model";

export interface MCPApprovalCellProps {
  reason: MCPApprovalReason;
  /** A refused Allow (the file changed in between, or another failure), shown under the pill. */
  error?: string | null;
}

/**
 * The status cell of a project-file server waiting for the operator's Allow. The Allow button
 * sits in the row's actions; this cell says why it waits.
 */
export function MCPApprovalCell({ reason, error }: MCPApprovalCellProps) {
  return (
    <div className="flex min-w-0 flex-col gap-0.5">
      <Pill form="plain" tone="warning">
        <Pill.Dot />
        Waiting for approval
      </Pill>
      <span className="text-eyebrow text-subtle">
        {reason === "changed" ? "Changed since you allowed it" : "Added by a file in this project"}
      </span>
      {error ? (
        <span className="text-eyebrow text-destructive" role="alert">
          {error}
        </span>
      ) : null}
    </div>
  );
}
