import {
  MetadataList,
  MetadataListRow,
  MetadataListTerm,
  MetadataListValue,
  Pill,
} from "@compozy/ui";
import { Clock, Lock, Settings2, ShieldCheck, type LucideIcon } from "lucide-react";
import Link from "next/link";
import {
  installCommand,
  marketplaceSearchCommand,
  type ExtensionEntry,
} from "@/lib/marketplace-catalog";
import { MarketplaceCrumbs } from "./marketplace-crumbs";
import { MarketplaceEntryInputs } from "./marketplace-entry-inputs";
import { MarketplaceEntryLogo } from "./marketplace-entry-logo";
import {
  extensionTierLabel,
  feedDateLine,
  FORMAT_LABELS,
  formatFeedDate,
  TIER_HINTS,
  versionLabel,
} from "./marketplace-entry-meta";
import { MarketplaceInstallCommand } from "./marketplace-install-command";

/**
 * Detail for one v3 extension entry. Everything on the page is a feed field or a command derived
 * from one: the static site has no daemon behind it, so there is no install state, server status,
 * or authorization readiness to show — those live in the runtime's own Marketplace window.
 */

function MarketplaceDetailSectionHead({
  id,
  icon: Icon,
  children,
}: {
  id: string;
  icon: LucideIcon;
  children: string;
}) {
  return (
    <h2
      id={id}
      className="flex items-center gap-2 text-lg font-semibold tracking-[-0.01em] text-fg"
    >
      <Icon aria-hidden className="size-4 text-muted" />
      {children}
    </h2>
  );
}

function ExternalLink({ href, children }: { href: string; children: string }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer noopener"
      className="break-all underline decoration-line-strong underline-offset-[0.22em] transition-colors hover:text-accent"
    >
      {children}
    </a>
  );
}

function Provenance({ entry }: { entry: ExtensionEntry }) {
  const published = formatFeedDate(entry.published_at);
  const updated = formatFeedDate(entry.updated_at);
  return (
    <section aria-labelledby="provenance" className="mt-10">
      <MarketplaceDetailSectionHead id="provenance" icon={ShieldCheck}>
        Provenance
      </MarketplaceDetailSectionHead>
      <MetadataList className="mt-4">
        <MetadataListRow>
          <MetadataListTerm>Tier</MetadataListTerm>
          <MetadataListValue className="flex flex-wrap items-center gap-2">
            <Pill size="sm">
              <ShieldCheck aria-hidden className="size-3" />
              {extensionTierLabel(entry.tier)}
            </Pill>
            <span className="text-subtle">{TIER_HINTS[entry.tier]}</span>
          </MetadataListValue>
        </MetadataListRow>
        {entry.author ? (
          <MetadataListRow>
            <MetadataListTerm>Author</MetadataListTerm>
            <MetadataListValue>{entry.author}</MetadataListValue>
          </MetadataListRow>
        ) : null}
        <MetadataListRow>
          <MetadataListTerm>Version</MetadataListTerm>
          <MetadataListValue className="font-mono">{versionLabel(entry.version)}</MetadataListValue>
        </MetadataListRow>
        {entry.format ? (
          <MetadataListRow>
            <MetadataListTerm>Format</MetadataListTerm>
            <MetadataListValue>{FORMAT_LABELS[entry.format]}</MetadataListValue>
          </MetadataListRow>
        ) : null}
        {published ? (
          <MetadataListRow>
            <MetadataListTerm>Published</MetadataListTerm>
            <MetadataListValue>{published}</MetadataListValue>
          </MetadataListRow>
        ) : null}
        {updated ? (
          <MetadataListRow>
            <MetadataListTerm>Updated</MetadataListTerm>
            <MetadataListValue>{updated}</MetadataListValue>
          </MetadataListRow>
        ) : null}
        <MetadataListRow>
          <MetadataListTerm>Install slug</MetadataListTerm>
          <MetadataListValue className="font-mono">{entry.install_slug}</MetadataListValue>
        </MetadataListRow>
        {entry.repository ? (
          <MetadataListRow>
            <MetadataListTerm>Repository</MetadataListTerm>
            <MetadataListValue className="font-mono">
              <ExternalLink href={entry.repository}>
                {entry.repository.replace(/^https:\/\//, "")}
              </ExternalLink>
            </MetadataListValue>
          </MetadataListRow>
        ) : null}
        <MetadataListRow>
          <MetadataListTerm>Artifact</MetadataListTerm>
          <MetadataListValue className="break-all font-mono">
            <ExternalLink href={entry.artifact_url}>{entry.artifact_url}</ExternalLink>
          </MetadataListValue>
        </MetadataListRow>
        <MetadataListRow>
          <MetadataListTerm>Digest</MetadataListTerm>
          <MetadataListValue className="break-all font-mono">
            sha256:{entry.digest_sha256}
          </MetadataListValue>
        </MetadataListRow>
      </MetadataList>
      <p className="mt-4 text-small-body leading-relaxed text-subtle">
        The runtime verifies this digest against the downloaded artifact before installing — trust
        on this page maps to these fields and nothing else.
      </p>
    </section>
  );
}

export function MarketplaceEntryDetail({ entry }: { entry: ExtensionEntry }) {
  const dateLine = feedDateLine(entry);
  const inputs = entry.inputs ?? [];

  return (
    <main id="main-content" className="mx-auto w-full max-w-3xl px-4 pt-12 pb-20">
      <MarketplaceCrumbs leaf={entry.name} />

      <header className="mt-6 flex items-start gap-4">
        <MarketplaceEntryLogo entry={entry} size="lg" className="mt-1" />
        <div className="min-w-0">
          <h1 className="text-detail-h1 font-semibold tracking-detail-h1 text-fg-strong">
            {entry.name}
          </h1>
          <p className="mt-2.5 text-site-doc-lead text-muted">{entry.description}</p>
        </div>
      </header>

      <div className="mt-6 flex flex-wrap items-center gap-x-4 gap-y-2 text-small-body text-muted">
        <code className="font-mono text-badge text-subtle">{versionLabel(entry.version)}</code>
        <Pill size="sm">
          <ShieldCheck aria-hidden className="size-3" />
          {extensionTierLabel(entry.tier)}
        </Pill>
        {entry.author ? (
          <span>
            By <strong className="font-medium text-fg">{entry.author}</strong>
          </span>
        ) : null}
        {dateLine ? (
          <span className="inline-flex items-center gap-1.5">
            <Clock aria-hidden className="size-3.5 text-subtle" />
            {dateLine.verb} {dateLine.date}
          </span>
        ) : null}
      </div>

      <div className="mt-7 flex flex-col gap-3">
        <MarketplaceInstallCommand
          command={marketplaceSearchCommand(entry)}
          className="border-accent/35 bg-canvas-soft"
        />
        <MarketplaceInstallCommand command={installCommand(entry)} />
        <p className="text-small-body leading-relaxed text-subtle">
          Search confirms the entry exists in your daemon&apos;s active catalog; install fetches the
          artifact, verifies its digest, and enrolls the extension.
          {inputs.length > 0
            ? " This package declares inputs — see below for the values it needs."
            : null}
        </p>
      </div>

      {inputs.length > 0 ? (
        <section aria-labelledby="inputs" className="mt-10">
          <MarketplaceDetailSectionHead id="inputs" icon={Settings2}>
            Inputs
          </MarketplaceDetailSectionHead>
          <MarketplaceEntryInputs inputs={inputs} />
          <p className="mt-3 flex items-start gap-2 text-small-body leading-relaxed text-subtle">
            <Lock aria-hidden className="mt-1 size-3.5 shrink-0" />
            <span>
              Secret values are never stored in the catalog or rendered on this page. Whether the
              server is running or authorized is runtime state — your daemon&apos;s Marketplace
              window shows it after install.
            </span>
          </p>
        </section>
      ) : null}

      <Provenance entry={entry} />

      <p className="mt-12 border-t border-line pt-6 text-small-body leading-relaxed text-subtle">
        New to the marketplace? The{" "}
        <Link
          href="/docs/marketplace"
          className="underline decoration-line-strong underline-offset-[0.22em] transition-colors hover:text-accent"
        >
          concept docs
        </Link>{" "}
        explain how installs, trust tiers, and plugin marketplaces fit the runtime.
      </p>
    </main>
  );
}
