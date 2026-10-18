import { FileText, Fingerprint } from "lucide-react";

import { MonoId, PropertyRow, Time } from "@compozy/ui";

import { COMPOZY_CATALOG_SOURCE } from "../lib/marketplace-installed-view";
import type { MarketplaceCatalogEntryResponse } from "../types";
import { MarketplaceDetailExtensionInstalled } from "./marketplace-detail-extension-installed";
import { MarketplaceExtensionServerSection } from "./marketplace-detail-extension-server";
import {
  MarketplaceDetailColumns,
  MarketplaceDetailRailCard,
  MarketplaceDetailSection,
  MarketplaceRepositoryRow,
} from "./marketplace-detail-shell";
import { MarketplaceDetailWarnings } from "./marketplace-detail-warnings";
import { formatMarketplaceVersion, marketplaceTrustSentence } from "./marketplace-ui";

interface MarketplaceDetailExtensionViewProps {
  data: MarketplaceCatalogEntryResponse;
  liveDataEnabled?: boolean;
}

/**
 * Extension detail: installed entries put the kit and its runtime in the body;
 * browse entries lead with identity and any warnings, the pinned download source last and closed. An entry whose manifest
 * provides MCP servers opens the rail with a Server card per server (summaries only
 * until installed).
 */
function MarketplaceDetailExtensionView({
  data,
  liveDataEnabled = true,
}: MarketplaceDetailExtensionViewProps) {
  if (data.entry.installed) {
    return <MarketplaceDetailExtensionInstalled data={data} liveDataEnabled={liveDataEnabled} />;
  }
  return (
    <MarketplaceDetailColumns
      main={
        <>
          <MarketplaceDetailWarnings warnings={data.entry.trust?.warnings} />
          <MarketplaceExtensionArtifactSection data={data} />
        </>
      }
      rail={
        <>
          <MarketplaceExtensionServerSection
            inputs={data.extension?.inputs ?? []}
            servers={data.extension?.mcp_servers ?? []}
          />
          <MarketplaceExtensionAboutCard data={data} />
        </>
      }
    />
  );
}

function MarketplaceExtensionArtifactSection({ data }: { data: MarketplaceCatalogEntryResponse }) {
  const extension = data.extension;
  if (!extension) return null;
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-extension-artifact"
      defaultOpen={false}
      icon={Fingerprint}
      summary="Where it's downloaded from"
      title="Source"
    >
      <div className="px-4 py-2">
        {extension.repository ? (
          <PropertyRow label="Repository" valueTitle={extension.repository}>
            <MarketplaceExternalLink href={extension.repository} />
          </PropertyRow>
        ) : null}
        {extension.artifact_url ? (
          <PropertyRow label="Download" valueTitle={extension.artifact_url}>
            <MarketplaceExternalLink href={extension.artifact_url} />
          </PropertyRow>
        ) : null}
        {extension.digest_sha256 ? (
          <PropertyRow editor={<MonoId value={extension.digest_sha256} />} label="SHA-256" />
        ) : null}
        <PropertyRow label="Entry ID" mono>
          {data.entry.entry_id}
        </PropertyRow>
      </div>
      <p className="border-t border-line-soft px-4 py-2.5 text-form-label leading-relaxed text-subtle">
        Installing downloads exactly this file and checks it against the SHA-256 above.
      </p>
    </MarketplaceDetailSection>
  );
}

function MarketplaceExternalLink({ href }: { href: string }) {
  return (
    <a
      className="min-w-0 truncate font-mono text-mono-id text-muted transition-colors duration-base hover:text-fg"
      href={href}
      rel="noreferrer"
      target="_blank"
    >
      {href} ↗
    </a>
  );
}

/**
 * About: identity facts plus the one-sentence trust summary. The catalog's own source is implied,
 * so it only shows for third-party marketplaces.
 */
function MarketplaceExtensionAboutCard({
  data,
  defaultOpen = true,
}: {
  data: MarketplaceCatalogEntryResponse;
  defaultOpen?: boolean;
}) {
  const entry = data.entry;
  const version = formatMarketplaceVersion(entry.version);
  const trust = marketplaceTrustSentence(entry);
  return (
    <MarketplaceDetailRailCard
      data-testid="marketplace-extension-about"
      defaultOpen={defaultOpen}
      icon={FileText}
      summary={version ?? undefined}
      title="About"
    >
      <div className="px-3.5">
        <PropertyRow label="Trust" valueTitle={trust}>
          <span data-testid="marketplace-extension-trust">{trust}</span>
        </PropertyRow>
        {version ? (
          <PropertyRow label="Version" mono>
            {version}
          </PropertyRow>
        ) : null}
        {entry.author ? <PropertyRow label="Author">{entry.author}</PropertyRow> : null}
        {entry.source !== COMPOZY_CATALOG_SOURCE ? (
          <PropertyRow label="Marketplace" mono>
            {entry.source}
          </PropertyRow>
        ) : null}
        {entry.published_at ? (
          <PropertyRow label="Published">
            <Time iso={entry.published_at} />
          </PropertyRow>
        ) : null}
        {entry.updated_at ? (
          <PropertyRow label="Updated">
            <Time iso={entry.updated_at} />
          </PropertyRow>
        ) : null}
        <MarketplaceRepositoryRow repository={data.extension?.repository} />
      </div>
    </MarketplaceDetailRailCard>
  );
}

export { MarketplaceDetailExtensionView, MarketplaceExtensionAboutCard };
export type { MarketplaceDetailExtensionViewProps };
