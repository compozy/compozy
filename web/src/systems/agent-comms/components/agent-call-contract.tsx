/**
 * The result contract on a call record.
 *
 * The wire carries a digest, a budget, and an overflow mode — not the schema
 * body. A disclosure for keys that are not on the record would be invented
 * shape. The digest and the foot are the facts.
 */
import { Eyebrow, MonoId, Panel } from "@compozy/ui";

import { formatAgentCallBytes } from "../lib/format-bytes";

export function AgentCallContract({
  digest,
  budgetBytes,
  overflow,
  "data-testid": testId,
}: {
  digest: string;
  budgetBytes: number;
  overflow: string;
  "data-testid"?: string;
}) {
  return (
    <Panel
      className="border border-line"
      title={<Eyebrow>Result contract</Eyebrow>}
      meta={<MonoId value={digest} copy />}
      bodyClassName="hidden"
      data-testid={testId}
      foot={
        <span className="font-mono text-form text-muted">
          budget {formatAgentCallBytes(budgetBytes)} · overflow {overflow}
        </span>
      }
    />
  );
}
