import { ChevronRight } from "lucide-react";

import { cn, ListingRow, Pill } from "@agh/ui";

import { providerListingView } from "../lib/provider-listing-view";
import type { SettingsProviderEntry } from "../types";
import { ProviderLogo } from "./provider-logo";

interface ProviderRowProps {
  provider: SettingsProviderEntry;
  onOpen: (entry: SettingsProviderEntry) => void;
}

/**
 * Provider listing row on the prototype's dedicated 5-column grid: identity,
 * status, model count, and the drill-in chevron each own a column instead of
 * packing into one trailing cluster.
 */
export function ProviderRow({ provider, onOpen }: ProviderRowProps) {
  const view = providerListingView(provider);
  const testId = `settings-page-providers-row-${provider.name}`;

  return (
    <ListingRow
      className="grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)_auto_14px] gap-3.5 py-[11px]"
      data-state={view.state.label}
    >
      <ListingRow.Link
        aria-label={`Open ${view.displayName}`}
        className="col-span-1"
        data-testid={testId}
        onClick={() => onOpen(provider)}
        render={<button type="button" />}
      >
        <ListingRow.Icon>
          <ProviderLogo className="size-4.5" provider={provider.name} />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>{view.displayName}</ListingRow.Title>
            {provider.default ? <Pill tone="accent">Default</Pill> : null}
          </ListingRow.Name>
          <span className="mt-1 block truncate text-left font-mono text-eyebrow text-subtle">
            {view.command}
          </span>
        </ListingRow.Main>
      </ListingRow.Link>
      <span
        className={cn(
          "inline-flex min-w-0 items-center gap-1.5 justify-self-start truncate text-form-label font-medium",
          view.status.tone === "success"
            ? "text-success"
            : view.status.tone === "danger"
              ? "text-danger"
              : view.status.tone === "info"
                ? "text-info"
                : "text-warning"
        )}
        data-testid={`${testId}-status`}
      >
        <Pill.Dot tone={view.status.tone} />
        {view.status.label}
      </span>
      <span
        className="hidden justify-self-end font-mono text-mono-id tabular-nums whitespace-nowrap text-subtle sm:inline"
        data-testid={`${testId}-models`}
      >
        {view.modelCount || "—"} models
      </span>
      <ChevronRight aria-hidden="true" className="size-3.5 justify-self-end text-faint" />
    </ListingRow>
  );
}
