/**
 * The child closed the call. One receipt, no invented inbox row on the parent.
 */
import { CornerDownLeft } from "lucide-react";

import { Receipt } from "@compozy/ui";

export interface AgentCallReturnTurnProps {
  callerName: string;
  verdict?: string | null;
  "data-testid"?: string;
  "data-synthetic-kind"?: string;
}

export function AgentCallReturnTurn({
  callerName,
  verdict = null,
  "data-testid": testId,
  "data-synthetic-kind": syntheticKind = "call-return",
}: AgentCallReturnTurnProps) {
  return (
    <Receipt
      data-synthetic-kind={syntheticKind}
      data-testid={testId}
      icon={<CornerDownLeft aria-hidden="true" />}
      tone="neutral"
    >
      Answer sent back to <b>{callerName}</b>
      {verdict !== null && verdict !== "" ? <> — verdict: {verdict}</> : null}
    </Receipt>
  );
}
