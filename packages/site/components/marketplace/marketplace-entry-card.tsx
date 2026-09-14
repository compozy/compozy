"use client";

import { CatalogCard, Pill } from "@compozy/ui";
import { ArrowRight } from "lucide-react";
import Link from "next/link";
import {
  marketplaceEntryPath,
  marketplaceSearchCommand,
  type ExtensionEntry,
} from "@/lib/marketplace-catalog";
import { MarketplaceEntryLogo } from "./marketplace-entry-logo";
import {
  feedDateLine,
  inputSummary,
  provenanceWord,
  shortDigest,
  versionLabel,
} from "./marketplace-entry-meta";
import { MarketplaceInstallCommand } from "./marketplace-install-command";

/**
 * One row-card per catalog entry, on the board anatomy (logo 40 · name + one-line description ·
 * trail) composed from `CatalogCard`. A client component because the compound parts are runtime
 * properties of a client module; the entry is the parsed feed object, plain JSON across the
 * boundary.
 *
 * What the card shows is what the feed carries: the trail is the copyable daemon search command
 * and the detail link, because a static site cannot install anything. Tier is a word, never a
 * color — the only tone on the row is the hollow warning for an unverified tier.
 */

function MetaSeparator() {
  return <span aria-hidden className="h-0.5 w-0.5 rounded-full bg-subtle" />;
}

export function MarketplaceEntryCard({ entry }: { entry: ExtensionEntry }) {
  const detailHref = marketplaceEntryPath(entry);
  const word = provenanceWord(entry);
  const inputs = inputSummary(entry.inputs);
  const dateLine = feedDateLine(entry);

  return (
    <CatalogCard actionable className="border border-line">
      <div className="flex min-w-0 items-start gap-3">
        <Link href={detailHref} aria-hidden tabIndex={-1} className="shrink-0">
          <MarketplaceEntryLogo entry={entry} size="md" />
        </Link>
        <div className="min-w-0 flex-1">
          <CatalogCard.Title className="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-card-title">
            <Link
              href={detailHref}
              className="truncate transition-colors hover:text-accent focus-visible:text-accent"
            >
              {entry.name}
            </Link>
            <span className="shrink-0 font-mono text-badge font-normal text-subtle">
              {versionLabel(entry.version)}
            </span>
            {word ? (
              <span className="shrink-0 font-mono text-mono-id font-normal text-faint">{word}</span>
            ) : null}
            {entry.tier === "unverified" ? (
              <Pill form="hollow" size="xs" tone="warning">
                unverified
              </Pill>
            ) : null}
          </CatalogCard.Title>
          <CatalogCard.Description className="mt-1 line-clamp-2" title={entry.description}>
            {entry.description}
          </CatalogCard.Description>
        </div>
      </div>
      <CatalogCard.Meta className="flex flex-wrap items-center gap-x-2 gap-y-1.5 text-small-body text-muted">
        {dateLine ? (
          <span>
            {dateLine.verb} {dateLine.date}
          </span>
        ) : null}
        {dateLine && inputs ? <MetaSeparator /> : null}
        {inputs ? <span>{inputs}</span> : null}
        {dateLine || inputs ? <MetaSeparator /> : null}
        <code className="font-mono text-subtle">{shortDigest(entry.digest_sha256)}</code>
      </CatalogCard.Meta>
      <CatalogCard.Actions className="mt-1 flex flex-wrap items-center gap-3">
        <MarketplaceInstallCommand
          command={marketplaceSearchCommand(entry)}
          className="min-w-0 flex-1 basis-64"
        />
        <Link
          href={detailHref}
          className="inline-flex shrink-0 items-center gap-1.5 text-small-body font-medium text-muted transition-colors hover:text-accent"
        >
          View details
          <ArrowRight aria-hidden className="size-3.5" />
        </Link>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}
