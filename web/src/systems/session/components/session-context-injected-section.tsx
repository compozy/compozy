import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  StatusBreakdown,
  cn,
} from "@compozy/ui";
import { ChevronRight } from "lucide-react";
import type { SessionContextPayload } from "../types";
import { formatContextBytes, formatContextTokens, formatContextTurn } from "../lib/context-format";
import { SessionInspectorSection } from "./session-inspector-section";

type InjectedRow = NonNullable<SessionContextPayload["injected"]>["rows"][number];

/** Rows and foot sit under the chevron, aligned with the head label. */
const INDENT = "pl-4.75";

/** One sentence per row: when it was sent, then every caveat that still applies. */
function rowSuffix(row: InjectedRow): string {
  const parts: string[] = [];
  if (row.owner_kind === "startup_opaque") {
    parts.push("included in the startup prompt");
  } else if (row.unchanged && row.last_seen_turn_id) {
    parts.push(
      `unchanged since turn ${formatContextTurn(row.delivered_turn_id)} · last seen turn ${formatContextTurn(row.last_seen_turn_id)}`
    );
  } else {
    parts.push(`sent on turn ${formatContextTurn(row.delivered_turn_id)}`);
  }
  if (row.stale) parts.push("may have been summarized");
  if (row.hook_modified) parts.push("modified by a hook");
  return parts.join(" · ");
}

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
  const plain = row.owner_kind === "startup_opaque" || row.tokens == null;
  return (
    <li
      className="flex flex-col gap-0.5"
      data-testid="session-context-injected-row"
      data-stale={row.stale ? "true" : undefined}
    >
      {plain ? (
        <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2.5 text-form-label">
          <span className="truncate text-muted" title={label}>
            {label}
          </span>
          {row.owner_kind !== "startup_opaque" ? (
            <span className="font-mono text-mono-id tabular-nums text-muted">
              {formatContextBytes(row.bytes)}
            </span>
          ) : null}
        </div>
      ) : (
        <StatusBreakdown
          className={row.stale ? "[&_[data-slot=status-breakdown-bar]]:bg-accent-dim" : undefined}
          total={total}
          items={[
            {
              id: `${row.key}/${row.delivery_sequence}`,
              label,
              value: row.tokens ?? 0,
              formattedValue: `≈ ${formatContextTokens(row.tokens ?? 0)}`,
              tone: "accent",
              showBar,
            },
          ]}
        />
      )}
      <p className={cn("text-micro leading-4", row.stale ? "text-warning" : "text-faint")}>
        {rowSuffix(row)}
      </p>
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
  const total = injected && rows.some(row => row.tokens != null) ? injected.tokens : undefined;
  // Absent means absent: the section keeps its slot in the rail but draws nothing until a delivery lands.
  return (
    <SessionInspectorSection data-testid="session-context-injected" hidden={rows.length === 0}>
      <Collapsible defaultOpen={defaultOpen}>
        <CollapsibleTrigger
          render={
            <button
              type="button"
              className="group flex w-full items-center gap-1.5 rounded-sm text-left text-fg outline-none focus-visible:shadow-focus-ring"
            />
          }
        >
          <ChevronRight
            aria-hidden="true"
            className="size-3.25 shrink-0 text-subtle transition-transform duration-base ease-out group-aria-expanded:rotate-90 motion-reduce:transition-none"
          />
          <span className="text-form-label font-medium">CompozyOS context</span>
          <span className="ml-auto font-mono text-mono-id tabular-nums text-muted">
            {total != null ? (
              <>
                ≈ {formatContextTokens(total)}
                <small className="ml-1 text-faint">bytes/4</small>
              </>
            ) : (
              `${rows.length} ${rows.length === 1 ? "row" : "rows"}`
            )}
          </span>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <ul className={cn("flex flex-col gap-1.75 pt-1.5", INDENT)}>
            {rows.map(row => (
              <SessionContextInjectedRow
                key={`${row.key}/${row.delivery_sequence}/${row.name ?? ""}`}
                row={row}
                showBar={showBars}
                total={total ?? 0}
              />
            ))}
          </ul>
          <p className={cn("pt-1.75 text-micro leading-4 text-faint", INDENT)}>
            {showBars
              ? "Estimate: bytes ÷ 4 over the text CompozyOS delivered. The agent's own prompt and tools are not counted here."
              : "No window reported, so there is nothing to draw the rows against."}
          </p>
        </CollapsibleContent>
      </Collapsible>
    </SessionInspectorSection>
  );
}
