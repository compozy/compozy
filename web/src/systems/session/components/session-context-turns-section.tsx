import { describeCost } from "@/lib/cost-provenance";
import { Fragment, useState } from "react";
import { Button, Empty, Eyebrow, Surface } from "@compozy/ui";
import { History, Minimize2 } from "lucide-react";
import type { SessionUsageTurnsResponse } from "../types";
import {
  formatContextPercent,
  formatContextTokens,
  formatContextTurn,
} from "../lib/context-format";

function SessionContextTurnRow({ turn }: { turn: SessionUsageTurnsResponse["turns"][number] }) {
  const usage = turn.usage;
  const context = usage?.context_used;
  // Per-turn ACP cost is an agent report, not a catalog estimate.
  const cost = describeCost({
    status: usage?.cost_amount != null ? "actual" : undefined,
    source: "agent_reported",
    amount: usage?.cost_amount,
    currency: usage?.cost_currency,
  });
  return (
    <li className="flex items-center gap-2 p-2" data-testid="session-context-turn-row">
      <span
        className="w-14 shrink-0 truncate font-mono text-mono-id text-subtle"
        title={turn.turn_id}
      >
        Turn {formatContextTurn(turn.turn_id)}
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        {context != null ? (
          <span className="font-mono text-mono-id tabular-nums">
            {formatContextTokens(context)}
            {usage?.context_size != null
              ? ` / ${formatContextTokens(usage.context_size)}`
              : " used"}
          </span>
        ) : null}
        <div className="flex flex-wrap gap-x-2 gap-y-1 text-eyebrow text-muted">
          {usage?.input_tokens != null ? (
            <span>{formatContextTokens(usage.input_tokens)} in</span>
          ) : null}
          {usage?.output_tokens != null ? (
            <span>{formatContextTokens(usage.output_tokens)} out</span>
          ) : null}
          {usage?.cache_read_tokens != null ? (
            <span>{formatContextTokens(usage.cache_read_tokens)} cache read</span>
          ) : null}
          {usage?.cache_write_tokens != null ? (
            <span>{formatContextTokens(usage.cache_write_tokens)} cache write</span>
          ) : null}
          {usage?.total_tokens != null &&
          usage.input_tokens == null &&
          usage.output_tokens == null ? (
            <span>{formatContextTokens(usage.total_tokens)} tokens</span>
          ) : null}
          {turn.injected ? (
            <span>≈ {formatContextTokens(turn.injected.tokens)} injected</span>
          ) : null}
        </div>
      </div>
      <dl className="shrink-0 font-mono text-mono-id tabular-nums text-muted">
        <dt className="sr-only">Turn cost</dt>
        <dd>{cost.value}</dd>
      </dl>
    </li>
  );
}

export function SessionContextTurnsSection({
  data,
  unavailable = false,
}: {
  data?: SessionUsageTurnsResponse;
  unavailable?: boolean;
}) {
  const [showAll, setShowAll] = useState(false);
  const turns = [...(data?.turns ?? [])].sort((a, b) => b.sequence - a.sequence);
  const visible = showAll ? turns : turns.slice(0, 50);
  const oldest = visible.at(-1)?.sequence ?? 0;
  const rows = [
    ...visible.map(turn => ({
      sequence: turn.sequence,
      key: `turn/${turn.turn_id}`,
      content: <SessionContextTurnRow turn={turn} />,
    })),
    ...(data?.compactions ?? []).flatMap(marker => {
      if (!showAll && turns.length > 50 && marker.sequence < oldest) return [];
      return [
        {
          sequence: marker.sequence,
          key: `compaction/${marker.sequence}`,
          content: (
            <li
              className="flex gap-2 py-2 text-eyebrow text-muted"
              data-testid="session-context-compaction"
            >
              <Minimize2 className="mt-0.5 size-3 shrink-0" />
              <span>
                Compozy compaction · at {formatContextPercent(marker.pressure)} · replay span{" "}
                <span className={marker.span_archived ? undefined : "text-warning"}>
                  {marker.span_archived ? "archived" : "not archived"}
                </span>
              </span>
            </li>
          ),
        },
      ];
    }),
  ].sort((a, b) => b.sequence - a.sequence);
  return (
    <section className="flex flex-col gap-3" data-testid="session-context-turns">
      <Eyebrow>Turns</Eyebrow>
      {unavailable ? <p className="text-eyebrow text-muted">Turn usage unavailable</p> : null}
      {rows.length ? (
        <Surface className="p-0">
          <ul className="divide-y divide-line">
            {rows.map(row => (
              <Fragment key={row.key}>{row.content}</Fragment>
            ))}
          </ul>
        </Surface>
      ) : !unavailable ? (
        <Empty icon={History} title="No turns yet" />
      ) : null}
      {!showAll && turns.length > 50 ? (
        <Button variant="ghost" size="sm" onClick={() => setShowAll(true)}>
          Show earlier turns
        </Button>
      ) : null}
    </section>
  );
}
