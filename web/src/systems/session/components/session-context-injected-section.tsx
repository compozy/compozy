import {
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Empty,
  Eyebrow,
  StatusBreakdown,
} from "@compozy/ui";
import { ChevronDown, Layers } from "lucide-react";
import type { SessionContextPayload } from "../types";
import { formatContextBytes, formatContextTokens, formatContextTurn } from "../lib/context-format";

type InjectedRow = NonNullable<SessionContextPayload["injected"]>["rows"][number];

function SessionContextInjectedRow({
  row,
  total,
  showBar,
}: {
  row: InjectedRow;
  total: number;
  showBar: boolean;
}) {
  const label = row.name || row.label;
  return (
    <li className="flex flex-col gap-1 py-2" data-testid="session-context-injected-row">
      {row.owner_kind === "startup_opaque" || row.tokens == null ? (
        <div className="flex justify-between gap-2 text-form-label">
          <span className="min-w-0 break-words text-muted">{label}</span>
          {row.owner_kind !== "startup_opaque" ? (
            <span className="shrink-0 font-mono tabular-nums">{formatContextBytes(row.bytes)}</span>
          ) : null}
        </div>
      ) : (
        <StatusBreakdown
          className={row.stale ? "[&_[data-slot=status-breakdown-bar]]:bg-accent-dim" : undefined}
          total={total}
          items={[
            {
              label,
              value: row.tokens,
              formattedValue: `≈ ${formatContextTokens(row.tokens)}`,
              tone: "accent",
              showBar,
            },
          ]}
        />
      )}
      <p className="text-eyebrow text-subtle">
        {row.owner_kind === "startup_opaque"
          ? "included in the startup prompt"
          : row.unchanged && row.last_seen_turn_id
            ? `unchanged since turn ${formatContextTurn(row.delivered_turn_id)} · last seen turn ${formatContextTurn(row.last_seen_turn_id)}`
            : `sent on turn ${formatContextTurn(row.delivered_turn_id)}`}
      </p>
      {row.stale ? <p className="text-eyebrow text-warning">may have been summarized</p> : null}
      {row.hook_modified ? <p className="text-eyebrow text-muted">modified by a hook</p> : null}
    </li>
  );
}

export function SessionContextInjectedSection({
  injected,
  defaultOpen = false,
  showBars = true,
}: {
  injected?: SessionContextPayload["injected"];
  defaultOpen?: boolean;
  showBars?: boolean;
}) {
  const rows = injected?.rows ?? [];
  return (
    <section className="flex flex-col gap-3" data-testid="session-context-injected">
      <Collapsible defaultOpen={defaultOpen}>
        <CollapsibleTrigger
          render={<Button variant="ghost" size="sm" className="w-full justify-between px-0" />}
        >
          <Eyebrow>Compozy context ≈</Eyebrow>
          <span className="ml-auto font-mono text-mono-id tabular-nums">
            {injected && rows.some(row => row.tokens != null)
              ? formatContextTokens(injected.tokens)
              : ""}
          </span>
          <ChevronDown className="size-3 text-subtle" />
        </CollapsibleTrigger>
        {rows.length > 0 ? (
          <CollapsibleContent>
            <ul>
              {rows.map(row => (
                <SessionContextInjectedRow
                  key={`${row.key}/${row.delivery_sequence}/${row.name ?? ""}`}
                  row={row}
                  showBar={showBars}
                  total={injected?.tokens ?? 0}
                />
              ))}
            </ul>
            <p className="pt-2 text-eyebrow text-subtle">
              Estimate: bytes ÷ 4 over the text Compozy delivered.
            </p>
          </CollapsibleContent>
        ) : (
          <Empty icon={Layers} title="No Compozy context yet" />
        )}
      </Collapsible>
    </section>
  );
}
