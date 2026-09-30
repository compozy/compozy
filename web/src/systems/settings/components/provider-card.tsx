import { CatalogCard, cn, Pill } from "@compozy/ui";

import { providerListingView } from "../lib/provider-listing-view";
import type { SettingsProviderEntry } from "../types";
import { ProviderLogo } from "./provider-logo";
import { ProviderStatusLabel } from "./provider-status-label";

interface ProviderCardProps {
  provider: SettingsProviderEntry;
  onOpen: (entry: SettingsProviderEntry) => void;
}

/** Provider tile: identity, readiness, and how it signs in. */
export function ProviderCard({ provider, onOpen }: ProviderCardProps) {
  const view = providerListingView(provider);
  const testId = `settings-page-providers-card-${provider.name}`;

  return (
    <CatalogCard actionable className="p-0" data-state={view.state.label}>
      <button
        className="group flex min-h-full w-full flex-col gap-3 px-4 py-3.5 text-left focus-visible:outline-none focus-visible:shadow-focus-inset"
        data-testid={testId}
        onClick={() => onOpen(provider)}
        type="button"
      >
        <span className="flex min-w-0 items-center gap-3">
          <CatalogCard.Logo data-testid={`${testId}-logo`} size="lg">
            <ProviderLogo className="size-4.5" provider={provider.name} />
          </CatalogCard.Logo>
          <span className="flex min-w-0 flex-col">
            <span className="flex min-w-0 items-center gap-2">
              <CatalogCard.Title data-testid={`${testId}-name`}>
                {view.displayName}
              </CatalogCard.Title>
              {provider.default ? (
                <Pill data-testid={`${testId}-default`} tone="accent">
                  Default
                </Pill>
              ) : null}
            </span>
          </span>
        </span>
        <ProviderStatusLabel
          data-state={view.state.label}
          data-testid={`${testId}-status`}
          label={view.status.label}
          ready={view.state.label === "installed"}
          tone={view.status.tone}
        />
        <span className="flex items-center justify-between gap-2 border-t border-line-soft pt-2.5 text-form-label text-muted">
          {view.authSummary}
          <span
            className={cn(
              "text-form-label font-medium text-fg opacity-0 transition-opacity duration-base",
              "group-hover:opacity-100 group-focus-visible:opacity-100"
            )}
          >
            {view.actionLabel}
          </span>
        </span>
      </button>
    </CatalogCard>
  );
}
