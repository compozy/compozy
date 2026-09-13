import { Link, type LinkProps } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { CatalogCard, cn, Pill } from "@compozy/ui";

import { MarketplaceEntryLogo, type MarketplaceEntryLogoEntry } from "./marketplace-entry-logo";

interface MarketplaceEntryCardProps {
  /** Identity the logo ladder and the name resolve from. */
  entry: MarketplaceEntryLogoEntry;
  /** Row title; defaults to the entry name. */
  title?: string;
  description?: string | null;
  /** Faint mono word after the name: "community · author", a marketplace name, a scope. */
  words?: readonly string[];
  /** Quiet 12px summary after the name ("1 MCP server · 2 skills"). */
  contents?: string | null;
  /** Hollow warning word for an unverified registry tier. */
  unverified?: boolean;
  /** Detail link for the name and the logo; the trail is never inside it. */
  link: Pick<LinkProps, "to" | "params" | "search">;
  pending?: boolean;
  flashing?: boolean;
  onFlashEnd?: () => void;
  trail: ReactNode;
  className?: string;
  "data-testid"?: string;
}

/**
 * One row-card for the catalog and the Installed view: logo 40 · name + one-line description ·
 * trail. It never says what an entry contains unless the Installed row passes `contents`, and it
 * never colors by taxonomy — tone lives in the trail only.
 */
function MarketplaceEntryCard({
  entry,
  title,
  description,
  words = [],
  contents,
  unverified = false,
  link,
  pending = false,
  flashing = false,
  onFlashEnd,
  trail,
  className,
  "data-testid": testId,
}: MarketplaceEntryCardProps) {
  const name = title ?? entry.name;
  const text = description?.trim() ?? "";
  return (
    <CatalogCard
      actionable={!pending}
      aria-busy={pending || undefined}
      className={cn(
        "grid min-h-15 grid-cols-[var(--size-provider-logo-well)_minmax(0,1fr)_auto] items-center gap-3 px-3 py-2.5",
        flashing ? "marketplace-card-flash" : pending ? "opacity-55" : undefined,
        className
      )}
      data-testid={testId}
      onAnimationEnd={event => {
        if (event.currentTarget === event.target) onFlashEnd?.();
      }}
    >
      <Link
        aria-hidden="true"
        className="rounded-md focus-visible:outline-none"
        tabIndex={-1}
        {...link}
      >
        <MarketplaceEntryLogo entry={entry} size="md" />
      </Link>
      <div className="flex min-w-0 flex-col gap-0.5">
        <div className="flex min-w-0 items-center gap-1.75">
          <CatalogCard.Title>
            <Link
              aria-disabled={pending || undefined}
              aria-label={`View ${name} details`}
              className="rounded-xs hover:underline hover:underline-offset-2 focus-visible:shadow-focus-ring focus-visible:outline-none"
              onClick={event => {
                if (pending) event.preventDefault();
              }}
              tabIndex={pending ? -1 : undefined}
              {...link}
            >
              {name}
            </Link>
          </CatalogCard.Title>
          {contents ? (
            <span className="shrink-0 text-eyebrow whitespace-nowrap text-subtle">{contents}</span>
          ) : null}
          {words.map(word => (
            <span
              className="shrink-0 font-mono text-mono-id whitespace-nowrap text-faint"
              key={word}
            >
              {word}
            </span>
          ))}
          {unverified ? (
            <Pill form="hollow" size="xs" tone="warning">
              unverified
            </Pill>
          ) : null}
        </div>
        {text ? (
          <CatalogCard.Description
            className="line-clamp-2 text-eyebrow leading-snug @min-[960px]:line-clamp-1"
            title={text}
          >
            {text}
          </CatalogCard.Description>
        ) : (
          <CatalogCard.Description className="text-eyebrow leading-snug text-faint italic">
            No description yet
          </CatalogCard.Description>
        )}
      </div>
      <div className="flex shrink-0 items-center justify-end gap-2" data-slot="marketplace-trail">
        {trail}
      </div>
    </CatalogCard>
  );
}

export { MarketplaceEntryCard };
export type { MarketplaceEntryCardProps };
