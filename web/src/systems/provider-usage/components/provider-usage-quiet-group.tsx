import { ChevronRight } from "@compozy/ui/icons";

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  KindIcon,
  providerKindIconRegistry,
} from "@compozy/ui";

import type { ProviderUsageRowAction } from "../lib/status";
import { quietGroupLabel, type ProviderUsageRowModel } from "../lib/view-model";

import { ProviderUsageStatusLine } from "./provider-usage-status-line";

export interface ProviderUsageQuietGroupProps {
  rows: readonly ProviderUsageRowModel[];
  now: number;
  isMacHost: boolean;
  remoteClient?: boolean;
  onAction?: (providerId: string, action: ProviderUsageRowAction) => void;
  pendingProviderIds: ReadonlySet<string>;
}

/**
 * Providers with nothing to measure (unsupported, turned off), folded into
 * one trailing disclosure, closed by default. Opening it lists each name with
 * its reason and, when the daemon names one, the fix.
 */
export function ProviderUsageQuietGroup({
  rows,
  now,
  isMacHost,
  remoteClient,
  onAction,
  pendingProviderIds,
}: ProviderUsageQuietGroupProps) {
  if (rows.length === 0) return null;
  const label = quietGroupLabel(rows.length);
  return (
    <Collapsible data-slot="provider-usage-quiet-group" className="flex min-w-0 flex-col">
      <CollapsibleTrigger className="group/quiet flex w-full items-center gap-1 rounded-lg px-2 py-2 text-left text-meta text-muted transition-colors duration-base hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none">
        <ChevronRight
          aria-hidden="true"
          className="size-3 shrink-0 text-faint transition-transform duration-base group-data-panel-open/quiet:rotate-90 motion-reduce:transition-none"
        />
        {label}
      </CollapsibleTrigger>
      <CollapsibleContent>
        <ul aria-label={label} className="flex flex-col gap-1 pb-1">
          {rows.map(row => (
            <li
              key={row.providerId}
              data-provider={row.providerId}
              data-status={row.target.status}
              className="flex min-w-0 flex-col pr-2 pl-6"
            >
              <span className="flex min-h-6 items-center gap-2">
                <KindIcon
                  aria-hidden="true"
                  kind={row.providerId}
                  registry={providerKindIconRegistry}
                  size="sm"
                  tone="muted"
                />
                <span className="truncate text-small-body text-fg">{row.target.provider_name}</span>
              </span>
              <ProviderUsageStatusLine
                target={row.target}
                now={now}
                isMacHost={isMacHost}
                remoteClient={remoteClient}
                onAction={onAction}
                pending={pendingProviderIds.has(row.providerId)}
                className="pl-6"
              />
            </li>
          ))}
        </ul>
      </CollapsibleContent>
    </Collapsible>
  );
}
