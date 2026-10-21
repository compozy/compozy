import { ChevronRight } from "lucide-react";

import {
  cn,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  KindIcon,
  Progress,
  providerKindIconRegistry,
} from "@compozy/ui";

import { clampPercent, formatPercent, formatPlan, formatWindowReset } from "../lib/format";
import { hasKnownPercent, toneForSeverity } from "../lib/severity";
import type { ProviderUsageRowAction } from "../lib/status";
import type { ProviderUsageRowModel } from "../lib/view-model";

import { ProviderUsageDetail } from "./provider-usage-detail";
import { ProviderUsageStatusLine } from "./provider-usage-status-line";

export interface ProviderUsageRowProps {
  row: ProviderUsageRowModel;
  now: number;
  isMacHost: boolean;
  remoteClient?: boolean;
  /** Start expanded (the Usage app shows every target open). */
  defaultOpen?: boolean;
  onAction?: (providerId: string, action: ProviderUsageRowAction) => void;
  pending?: boolean;
}

function RowSummary({ row, now }: { row: ProviderUsageRowModel; now: number }) {
  const window = row.tightest;
  if (!window) return null;
  const reset = formatWindowReset(window, now);
  if (!hasKnownPercent(window)) {
    return (
      <span className="text-meta text-muted">
        {[window.label, "Usage unknown", reset].filter(Boolean).join(" · ")}
      </span>
    );
  }
  const tone = toneForSeverity(row.severity ?? "neutral");
  return (
    <>
      <Progress
        aria-hidden="true"
        value={clampPercent(window.used_percent)}
        tone={tone}
        className={cn("w-full", row.stale && "opacity-50")}
      />
      <span className="text-meta text-muted">
        {[window.label, reset].filter(Boolean).join(" · ")}
      </span>
    </>
  );
}

function RowFigure({ row }: { row: ProviderUsageRowModel }) {
  const window = row.tightest;
  if (!window || !hasKnownPercent(window)) return null;
  const exhausted = row.severity === "exhausted";
  return (
    <span
      className={cn(
        "shrink-0 text-meta tabular-nums",
        exhausted ? "font-medium text-danger" : "text-fg",
        row.severity === "warning" && "text-warning"
      )}
    >
      {exhausted ? "Limit reached" : `${formatPercent(window.used_percent)} used`}
    </span>
  );
}

/**
 * One target: provider mark and name, plan, the binding window as a bar with
 * its percent and reset, and the status line when the numbers are missing or
 * old. The header is the disclosure button; the status action sits outside it
 * so no button nests in another, which `Disclosure` cannot lay out — hence the
 * `Collapsible` parts it is built from.
 */
export function ProviderUsageRow({
  row,
  now,
  isMacHost,
  remoteClient,
  defaultOpen = false,
  onAction,
  pending,
}: ProviderUsageRowProps) {
  const { target } = row;
  const plan = target.account?.plan ? formatPlan(target.account.plan) : "";
  return (
    <Collapsible
      data-slot="provider-usage-row"
      data-provider={target.provider_id}
      data-status={target.status}
      data-severity={row.severity ?? undefined}
      data-stale={row.stale || undefined}
      defaultOpen={defaultOpen}
      className="flex min-w-0 flex-col rounded-lg"
    >
      <CollapsibleTrigger
        className={cn(
          "group/usage-row grid w-full grid-cols-[auto_minmax(0,1fr)] items-center gap-x-2 gap-y-1.5",
          "rounded-lg px-2 py-2 text-left transition-colors duration-base hover:bg-surface-2",
          "focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
      >
        <span className="flex items-center gap-1">
          <ChevronRight
            aria-hidden="true"
            className={cn(
              "size-3 shrink-0 text-faint transition-transform duration-base motion-reduce:transition-none",
              "group-data-panel-open/usage-row:rotate-90"
            )}
          />
          <KindIcon
            aria-hidden="true"
            kind={target.provider_id}
            registry={providerKindIconRegistry}
            size="sm"
            tone="muted"
          />
        </span>
        <span className="flex min-w-0 items-baseline gap-2">
          <span className="truncate text-small-body font-medium text-fg">
            {target.provider_name}
          </span>
          {plan ? <span className="truncate text-meta text-muted">{plan}</span> : null}
          <span className="flex-1" />
          <RowFigure row={row} />
        </span>
        {row.tightest ? (
          <span className="col-start-2 flex min-w-0 flex-col gap-1">
            <RowSummary row={row} now={now} />
          </span>
        ) : null}
      </CollapsibleTrigger>
      <ProviderUsageStatusLine
        target={target}
        now={now}
        isMacHost={isMacHost}
        remoteClient={remoteClient}
        onAction={onAction}
        pending={pending}
        className="pr-2 pb-1.5 pl-12"
      />
      <CollapsibleContent className="pt-1 pr-2 pb-2 pl-12">
        <ProviderUsageDetail target={target} now={now} />
      </CollapsibleContent>
    </Collapsible>
  );
}
