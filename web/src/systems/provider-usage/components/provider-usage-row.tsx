import { ChevronRight } from "@compozy/ui/icons";

import {
  cn,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  KindIcon,
  providerKindIconRegistry,
  StatusDot,
} from "@compozy/ui";

import { formatPercent, formatPlan, formatWindowReset } from "../lib/format";
import { hasKnownPercent } from "../lib/severity";
import {
  describeProviderUsageStatus,
  type ProviderUsageRowAction,
  type ProviderUsageStatusPresentation,
} from "../lib/status";
import type { ProviderUsageRowModel } from "../lib/view-model";

import { ProviderUsageDetail } from "./provider-usage-detail";
import {
  PROVIDER_USAGE_ROW_FRAME,
  PROVIDER_USAGE_ROW_TRIGGER_FRAME,
} from "./provider-usage-row-frame";
import { ProviderUsageStatusLine } from "./provider-usage-status-line";

export interface ProviderUsageRowProps {
  row: ProviderUsageRowModel;
  now: number;
  isMacHost: boolean;
  remoteClient?: boolean;
  onAction?: (providerId: string, action: ProviderUsageRowAction) => void;
  pending?: boolean;
}

function cueInk(row: ProviderUsageRowModel, known: boolean): string {
  if (!known) return "text-muted-foreground";
  if (row.severity === "exhausted") return "font-medium text-destructive";
  if (row.severity === "warning") return "text-warning";
  return row.stale ? "text-muted-foreground" : "text-fg";
}

/**
 * The collapsed row's trailing cue: the binding window's percent for measured
 * targets, otherwise the status in a word or two. A status dot backs the
 * words, and marks stale numbers by its ring shape.
 */
function RowCue({
  row,
  status,
}: {
  row: ProviderUsageRowModel;
  status: ProviderUsageStatusPresentation | null;
}) {
  const window = row.tightest;
  const known = window !== null && hasKnownPercent(window);
  const exhausted = row.severity === "exhausted";
  const text = known
    ? exhausted
      ? "Limit reached"
      : `${formatPercent(window.used_percent)} used`
    : (status?.label ?? (window ? "Usage unknown" : null));
  if (!text) return null;
  return (
    <span data-slot="provider-usage-row-cue" className="flex shrink-0 items-center gap-1.5">
      {status?.tone ? <StatusDot tone={status.tone} variant={status.dotVariant} size="sm" /> : null}
      <span className={cn("text-meta tabular-nums", cueInk(row, known))}>{text}</span>
    </span>
  );
}

/** Inline spans join without spaces in the computed name, so the trigger names itself. */
function rowAccessibleName(
  row: ProviderUsageRowModel,
  plan: string,
  status: ProviderUsageStatusPresentation | null,
  now: number
): string {
  const window = row.tightest;
  const parts = [row.target.provider_name, plan];
  if (window) {
    const usage = !hasKnownPercent(window)
      ? "usage unknown"
      : row.severity === "exhausted"
        ? "limit reached"
        : `${formatPercent(window.used_percent)} used`;
    parts.push(`${window.label} ${usage}`, formatWindowReset(window, now) ?? "");
  }
  if (status) parts.push(status.label);
  return parts.filter(Boolean).join(", ");
}

/**
 * One target, collapsed to a single line by default: provider mark and name,
 * plan, and a trailing cue (percent used, or the status in a word or two).
 * Expanding shows the status line with its one action, then every window and
 * fact. The header is the disclosure button; the status action sits outside
 * it so no button nests in another, which `Disclosure` cannot lay out — hence
 * the `Collapsible` parts it is built from.
 */
export function ProviderUsageRow({
  row,
  now,
  isMacHost,
  remoteClient,
  onAction,
  pending,
}: ProviderUsageRowProps) {
  const { target } = row;
  const plan = target.account?.plan ? formatPlan(target.account.plan) : "";
  const status = describeProviderUsageStatus(target, { now, isMacHost, remoteClient });
  return (
    <Collapsible
      data-slot="provider-usage-row"
      data-provider={target.provider_id}
      data-status={target.status}
      data-severity={row.severity ?? undefined}
      data-stale={row.stale || undefined}
      className={PROVIDER_USAGE_ROW_FRAME}
    >
      <CollapsibleTrigger
        aria-label={rowAccessibleName(row, plan, status, now)}
        className={cn(
          "group/usage-row",
          PROVIDER_USAGE_ROW_TRIGGER_FRAME,
          "text-left transition-colors duration-base hover:bg-surface-2",
          "focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
      >
        <span className="flex shrink-0 items-center gap-1">
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
        <span className="truncate text-small-body font-medium text-fg">{target.provider_name}</span>
        {plan ? <span className="truncate text-meta text-muted-foreground">{plan}</span> : null}
        <span className="flex-1" />
        <RowCue row={row} status={status} />
      </CollapsibleTrigger>
      <CollapsibleContent className="flex flex-col gap-2 pt-1 pr-2 pb-2 pl-12">
        <ProviderUsageStatusLine
          target={target}
          now={now}
          isMacHost={isMacHost}
          remoteClient={remoteClient}
          onAction={onAction}
          pending={pending}
          dot={false}
        />
        <ProviderUsageDetail target={target} now={now} />
      </CollapsibleContent>
    </Collapsible>
  );
}
