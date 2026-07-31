"use client";

import { CatalogCard, Pill } from "@compozy/ui";
import { ArrowRight, ShieldCheck } from "lucide-react";
import Link from "next/link";
import {
  marketplaceSearchCommand,
  type ExtensionEntry,
  type MarketplaceEntry,
  type MarketplaceKind,
  type MCPEntry,
  type SkillEntry,
} from "@/lib/marketplace-catalog";
import { MarketplaceInstallCommand } from "./marketplace-install-command";
import { extensionTierLabel, formatFeedDate, kindMeta } from "./marketplace-kind-meta";

function shortDigest(digest: string): string {
  return `sha256 · ${digest.slice(0, 8)}…`;
}

function MetaSeparator() {
  return <span aria-hidden className="h-0.5 w-0.5 rounded-full bg-subtle" />;
}

function SkillMeta({ entry }: { entry: SkillEntry }) {
  const updated = formatFeedDate(entry.updated_at ?? entry.published_at);
  return (
    <>
      {entry.author ? <span>By {entry.author}</span> : null}
      {entry.author && updated ? <MetaSeparator /> : null}
      {updated ? (
        <span>
          {entry.updated_at ? "Updated" : "Published"} {updated}
        </span>
      ) : null}
      {entry.tags?.map(tag => (
        <Pill key={tag} size="sm" className="font-mono">
          {tag}
        </Pill>
      ))}
    </>
  );
}

function ExtensionMeta({ entry }: { entry: ExtensionEntry }) {
  const published = formatFeedDate(entry.published_at);
  return (
    <>
      <Pill size="sm">
        <ShieldCheck aria-hidden className="size-3" />
        {extensionTierLabel(entry.tier)}
      </Pill>
      {entry.author ? <span>By {entry.author}</span> : null}
      {published ? (
        <>
          <MetaSeparator />
          <span>Published {published}</span>
        </>
      ) : null}
      <MetaSeparator />
      <code className="font-mono text-subtle">{shortDigest(entry.digest_sha256)}</code>
    </>
  );
}

function inputSummary(entry: MCPEntry): string | null {
  const count = entry.inputs?.length ?? 0;
  if (count === 0) return null;
  const optional = entry.inputs?.every(input => !input.required);
  const plural = count === 1 ? "input" : "inputs";
  return `${count} ${optional ? "optional " : ""}${plural}`;
}

function MCPMeta({ entry }: { entry: MCPEntry }) {
  const published = formatFeedDate(entry.published_at);
  const inputs = inputSummary(entry);
  return (
    <>
      <Pill size="sm" className="font-mono">
        {entry.launch.type}
      </Pill>
      {inputs ? (
        <>
          <MetaSeparator />
          <span>{inputs}</span>
        </>
      ) : null}
      <MetaSeparator />
      <span>
        Installs {entry.default_scope === "global" ? "globally" : "per workspace"} by default
      </span>
      {published ? (
        <>
          <MetaSeparator />
          <span>Published {published}</span>
        </>
      ) : null}
    </>
  );
}

function EntryMeta({ kind, entry }: { kind: MarketplaceKind; entry: MarketplaceEntry }) {
  switch (kind) {
    case "skills":
      return <SkillMeta entry={entry as SkillEntry} />;
    case "extensions":
      return <ExtensionMeta entry={entry as ExtensionEntry} />;
    case "mcp":
      return <MCPMeta entry={entry as MCPEntry} />;
  }
}

export function MarketplaceEntryCard({
  kind,
  entry,
}: {
  kind: MarketplaceKind;
  entry: MarketplaceEntry;
}) {
  const meta = kindMeta(kind);
  const Icon = meta.icon;
  const detailHref = `/marketplace/${kind}/${entry.entry_id}`;

  return (
    <CatalogCard actionable className="border border-line">
      <div className="flex min-w-0 items-start gap-3">
        <CatalogCard.Logo tone="neutral" className="mt-0.5">
          <Icon aria-hidden className="size-4" />
        </CatalogCard.Logo>
        <div className="min-w-0 flex-1">
          <CatalogCard.Title className="flex items-baseline gap-2">
            <Link
              href={detailHref}
              className="truncate transition-colors hover:text-accent focus-visible:text-accent"
            >
              {entry.name}
            </Link>
            {entry.version ? (
              <span className="shrink-0 font-mono text-badge text-subtle">
                {entry.version.startsWith("v") ? entry.version : `v${entry.version}`}
              </span>
            ) : null}
          </CatalogCard.Title>
          <CatalogCard.Description className="mt-1">{entry.description}</CatalogCard.Description>
        </div>
      </div>
      <CatalogCard.Meta className="flex flex-wrap items-center gap-x-2 gap-y-1.5 text-small-body text-muted">
        <EntryMeta kind={kind} entry={entry} />
      </CatalogCard.Meta>
      <CatalogCard.Actions className="mt-1 flex flex-wrap items-center gap-3">
        <MarketplaceInstallCommand
          command={marketplaceSearchCommand(kind, entry)}
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
