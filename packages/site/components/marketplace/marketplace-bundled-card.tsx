"use client";

import { CatalogCard, Pill } from "@compozy/ui";
import { ArrowRight, FileCode, Repeat } from "lucide-react";
import Link from "next/link";
import { MarketplaceInstallCommand } from "./marketplace-install-command";

/**
 * A client component for the same reason `marketplace-entry-card.tsx` is one: `CatalogCard`'s
 * compound parts (`.Logo`, `.Title`, …) are runtime properties of a client module and are not
 * reachable across the server/client boundary. Props stay serializable — the icon is chosen here
 * from `kind` rather than passed in.
 */

export interface BundledExtensionCardProps {
  href: string;
  name: string;
  version: string;
  description: string;
  inventory: string;
  minCompozyVersion: string;
  statusCommand: string;
}

export function BundledExtensionCard({
  href,
  name,
  version,
  description,
  inventory,
  minCompozyVersion,
  statusCommand,
}: BundledExtensionCardProps) {
  return (
    <CatalogCard className="border border-line">
      <div className="flex min-w-0 items-start gap-3">
        <CatalogCard.Logo tone="neutral" className="mt-0.5">
          <Repeat aria-hidden className="size-4" />
        </CatalogCard.Logo>
        <div className="min-w-0 flex-1">
          <CatalogCard.Title className="flex flex-wrap items-baseline gap-2">
            <Link
              href={href}
              className="transition-colors hover:text-accent focus-visible:text-accent"
            >
              {name}
            </Link>
            <span className="shrink-0 font-mono text-badge text-subtle">v{version}</span>
            <Pill size="sm">Bundled</Pill>
          </CatalogCard.Title>
          <CatalogCard.Description className="mt-1">{description}</CatalogCard.Description>
        </div>
      </div>
      <CatalogCard.Meta className="flex flex-wrap items-center gap-x-2 gap-y-1.5 text-small-body text-muted">
        <span className="font-mono text-badge text-subtle">{inventory}</span>
        <span aria-hidden className="h-0.5 w-0.5 rounded-full bg-subtle" />
        <span>Requires Compozy ≥ {minCompozyVersion}</span>
      </CatalogCard.Meta>
      <CatalogCard.Actions className="mt-1 flex flex-wrap items-center gap-3">
        <MarketplaceInstallCommand command={statusCommand} className="min-w-0 flex-1 basis-64" />
        <Link
          href={href}
          className="inline-flex shrink-0 items-center gap-1.5 text-small-body font-medium text-muted transition-colors hover:text-accent"
        >
          View details
          <ArrowRight aria-hidden className="size-3.5" />
        </Link>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

export function BundledSkillCard({ name, description }: { name: string; description: string }) {
  return (
    <CatalogCard className="border border-line">
      <div className="flex min-w-0 items-start gap-3">
        <CatalogCard.Logo tone="neutral" className="mt-0.5">
          <FileCode aria-hidden className="size-4" />
        </CatalogCard.Logo>
        <div className="min-w-0 flex-1">
          <CatalogCard.Title className="flex flex-wrap items-baseline gap-2">
            <span className="font-mono">{name}</span>
            <Pill size="sm">Bundled skill</Pill>
          </CatalogCard.Title>
          <CatalogCard.Description className="mt-1">{description}</CatalogCard.Description>
        </div>
      </div>
      <CatalogCard.Actions className="mt-1 flex flex-wrap items-center gap-3">
        <Link
          href="/docs/skills/bundled"
          className="inline-flex shrink-0 items-center gap-1.5 text-small-body font-medium text-muted transition-colors hover:text-accent"
        >
          How bundled skills load
          <ArrowRight aria-hidden className="size-3.5" />
        </Link>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}
