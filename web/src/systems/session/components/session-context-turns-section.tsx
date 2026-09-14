import { describeCost } from "@/lib/cost-provenance";
import { Fragment, useState } from "react";
import { Button } from "@compozy/ui";
import { History, Minimize2 } from "lucide-react";
import type { SessionUsageTurnsResponse } from "../types";
import {
  formatContextPercent,
  formatContextTokens,
  formatContextTurn,
} from "../lib/context-format";
import {
  SessionInspectorEmpty,
  SessionInspectorSection,
  SessionInspectorSectionHead,
} from "./session-inspector-section";

const VISIBLE_TURNS = 50;

type Turn = SessionUsageTurnsResponse["turns"][number];
type Compaction = SessionUsageTurnsResponse["compactions"][number];

/** "in · out · cache" plus what CompozyOS delivered; absent counters have no clause. */
function turnDetails(turn: Turn): string[] {
  const usage = turn.usage;
  const parts: string[] = [];
  if (usage?.input_tokens != null) parts.push(`in ${formatContextTokens(usage.input_tokens)}`);
  if (usage?.output_tokens != null) parts.push(`out ${formatContextTokens(usage.output_tokens)}`);
  if (usage?.cache_read_tokens != null)
    parts.push(`cache ${formatContextTokens(usage.cache_read_tokens)}`);
  if (usage?.cache_write_tokens != null)
    parts.push(`cache write ${formatContextTokens(usage.cache_write_tokens)}`);
  if (usage?.total_tokens != null && usage.input_tokens == null && usage.output_tokens == null)
    parts.push(`${formatContextTokens(usage.total_tokens)} tokens`);
  if (turn.injected) parts.push(`≈ ${formatContextTokens(turn.injected.tokens)} injected`);
  return parts;
}

function SessionContextTurnRow({ turn }: { turn: Turn }) {
  const usage = turn.usage;
  const context = usage?.context_used;
  // Per-turn ACP cost is an agent report, not a catalog estimate.
  const cost = describeCost({
    status: usage?.cost_amount != null ? "actual" : undefined,
    source: "agent_reported",
    amount: usage?.cost_amount,
    currency: usage?.cost_currency,
  });
  const details = turnDetails(turn);
  return (
    <li
      className="grid grid-cols-[3.75rem_minmax(0,1fr)_auto] items-center gap-2 px-2.5 py-1.75 text-form-label text-muted"
      data-testid="session-context-turn-row"
    >
      <span className="truncate font-mono text-mono-id text-subtle" title={turn.turn_id}>
        Turn {formatContextTurn(turn.turn_id)}
      </span>
      <span className="flex min-w-0 flex-col gap-0.5">
        {context != null ? (
          <span className="font-mono text-mono-id tabular-nums text-fg">
            {formatContextTokens(context)}
            <small className="text-faint">
              {usage?.context_size != null
                ? ` / ${formatContextTokens(usage.context_size)}`
                : " used"}
            </small>
          </span>
        ) : null}
        {details.length > 0 ? (
          <span className="text-micro leading-4 text-faint">{details.join(" · ")}</span>
        ) : null}
      </span>
      <dl className="text-right font-mono text-mono-id tabular-nums">
        <dt className="sr-only">Turn cost</dt>
        <dd className={cost.hasCost ? "text-muted" : "text-faint"}>{cost.value}</dd>
      </dl>
    </li>
  );
}

/** A daemon compaction where it fired: pressure, the window then, and whether the span is archived now. */
function SessionContextCompactionMarker({ marker }: { marker: Compaction }) {
  return (
    <li
      className="flex items-start gap-1.75 bg-canvas-soft px-2.5 py-1.5 text-micro leading-4 text-subtle"
      data-testid="session-context-compaction"
    >
      <Minimize2 aria-hidden="true" className="mt-0.75 size-2.75 shrink-0" />
      <span>
        CompozyOS compaction · at {formatContextPercent(marker.pressure)} ·{" "}
        <b className="font-mono font-medium text-muted">
          {formatContextTokens(marker.context_used)}
        </b>{" "}
        · replay span{" "}
        <span className={marker.span_archived ? undefined : "text-warning"}>
          {marker.span_archived ? "archived" : "not archived"}
        </span>
      </span>
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
  const capped = !showAll && turns.length > VISIBLE_TURNS;
  const visible = capped ? turns.slice(0, VISIBLE_TURNS) : turns;
  const oldest = visible.at(-1)?.sequence ?? 0;
  const rows = [
    ...visible.map(turn => ({
      sequence: turn.sequence,
      key: `turn/${turn.turn_id}`,
      content: <SessionContextTurnRow turn={turn} />,
    })),
    ...(data?.compactions ?? []).flatMap(marker => {
      if (capped && marker.sequence < oldest) return [];
      return [
        {
          sequence: marker.sequence,
          key: `compaction/${marker.sequence}`,
          content: <SessionContextCompactionMarker marker={marker} />,
        },
      ];
    }),
  ].sort((a, b) => b.sequence - a.sequence);
  return (
    <SessionInspectorSection data-testid="session-context-turns">
      <SessionInspectorSectionHead
        meta={
          turns.length > 0
            ? capped
              ? `latest ${VISIBLE_TURNS} of ${turns.length.toLocaleString()}`
              : "newest first"
            : undefined
        }
      >
        Turns
      </SessionInspectorSectionHead>
      {unavailable ? (
        <p className="text-micro leading-4 text-faint">Turn usage unavailable</p>
      ) : null}
      {rows.length ? (
        <div className="flex flex-col overflow-hidden rounded-md border border-line-soft bg-canvas">
          <ul className="divide-y divide-line-soft">
            {rows.map(row => (
              <Fragment key={row.key}>{row.content}</Fragment>
            ))}
          </ul>
          {capped ? (
            <div className="flex justify-center border-t border-line-soft p-1.5">
              <Button variant="ghost" size="xs" onClick={() => setShowAll(true)}>
                Show earlier turns
                <span className="font-mono text-mono-id tabular-nums text-faint">
                  {(turns.length - VISIBLE_TURNS).toLocaleString()}
                </span>
              </Button>
            </div>
          ) : null}
        </div>
      ) : !unavailable ? (
        <SessionInspectorEmpty icon={History} title="No turns yet" />
      ) : null}
    </SessionInspectorSection>
  );
}
