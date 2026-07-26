import { CatalogCard, cn, Pill } from "@agh/ui";

import { providerListingView } from "../lib/provider-listing-view";
import type { SettingsProviderEntry } from "../types";
import { ProviderLogo } from "./provider-logo";

interface ProviderCardProps {
  provider: SettingsProviderEntry;
  onOpen: (entry: SettingsProviderEntry) => void;
}

export function ProviderCard({ provider, onOpen }: ProviderCardProps) {
  const view = providerListingView(provider);
  const testId = `settings-page-providers-card-${provider.name}`;

  return (
    <CatalogCard actionable className="border border-line p-0" data-state={view.state.label}>
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
            <span
              className="truncate font-mono text-eyebrow text-subtle"
              data-testid={`${testId}-command`}
            >
              {view.command}
            </span>
          </span>
        </span>
        <span
          className={cn(
            "inline-flex items-center gap-1.5 text-form-label font-medium",
            view.status.tone === "success"
              ? "text-success"
              : view.status.tone === "danger"
                ? "text-danger"
                : view.status.tone === "info"
                  ? "text-info"
                  : "text-warning"
          )}
          data-state={view.state.label}
          data-testid={`${testId}-status`}
        >
          <Pill.Dot tone={view.status.tone} />
          {view.status.label}
        </span>
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
