import type { ComponentProps } from "react";

import { CatalogCard, CatalogCardSkeleton, cn, Pill, Skeleton, SkeletonText } from "@compozy/ui";

import { providerListingView } from "../lib/provider-listing-view";
import type { SettingsProviderEntry } from "../types";
import { ProviderLogo } from "./provider-logo";
import { ProviderStatusLabel } from "./provider-status-label";

const PROVIDER_GRID_CLASS = "grid gap-3 sm:grid-cols-2 lg:grid-cols-3";
const PROVIDER_CARD_BODY_CLASS =
  "flex min-h-full w-full flex-col gap-3 px-4 py-3.5 text-left focus-visible:outline-none focus-visible:shadow-focus-inset";
const PROVIDER_CARD_FOOTER_CLASS =
  "flex items-center justify-between gap-2 border-t border-line-soft pt-2.5 text-form-label text-muted-foreground";

/** The grid every provider card sits in, shared by the loaded list and its loading shape. */
export function ProviderCardGrid({ className, ...props }: ComponentProps<"section">) {
  return <section className={cn(PROVIDER_GRID_CLASS, className)} {...props} />;
}

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
        className={cn("group", PROVIDER_CARD_BODY_CLASS)}
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
                <Pill data-testid={`${testId}-default`} tone="neutral">
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
        <span className={PROVIDER_CARD_FOOTER_CLASS}>
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

/**
 * `count` provider cards in skeleton form: the real card surface with the identity row, readiness
 * line and sign-in footer as placeholders. Siblings, so put them in a `ProviderCardGrid`.
 */
export function ProviderCardSkeleton({ count = 6 }: { count?: number }) {
  return (
    <CatalogCardSkeleton className="p-0" count={count}>
      <div className={PROVIDER_CARD_BODY_CLASS}>
        <span className="flex min-w-0 items-center gap-3">
          <CatalogCard.Logo size="lg">
            <Skeleton className="size-full rounded-sm" />
          </CatalogCard.Logo>
          <CatalogCard.Title>
            <SkeletonText className="w-28" />
          </CatalogCard.Title>
        </span>
        <Skeleton className="h-4 w-28 rounded-pill" />
        <span className={PROVIDER_CARD_FOOTER_CLASS}>
          <SkeletonText className="w-44" />
        </span>
      </div>
    </CatalogCardSkeleton>
  );
}
