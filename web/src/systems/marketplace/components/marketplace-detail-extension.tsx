import { FileText, Fingerprint, ShieldCheck } from "lucide-react";

import { MonoId, PropertyRow, Time } from "@compozy/ui";

import type { MarketplaceEntryResponse } from "../types";
import { MarketplaceDetailExtensionInstalled } from "./marketplace-detail-extension-installed";
import {
  MarketplaceDetailColumns,
  MarketplaceDetailKvRow,
  MarketplaceDetailRailCard,
  MarketplaceDetailSection,
} from "./marketplace-detail-shell";
import { MarketplaceRepositoryRow } from "./marketplace-detail-skill";
import { MarketplaceDetailWarnings } from "./marketplace-detail-warnings";
import { formatMarketplaceCount, formatMarketplaceVersion } from "./marketplace-ui";

interface MarketplaceDetailExtensionViewProps {
  data: MarketplaceEntryResponse;
}

/**
 * Extension detail: installed entries put the kit and its runtime in the body;
 * browse entries lead with the pinned artifact provenance.
 */
function MarketplaceDetailExtensionView({ data }: MarketplaceDetailExtensionViewProps) {
  if (data.entry.installed) {
    return <MarketplaceDetailExtensionInstalled data={data} />;
  }
  return (
    <MarketplaceDetailColumns
      main={
        <>
          <MarketplaceExtensionArtifactSection extension={data.extension} />
          <MarketplaceDetailWarnings warnings={data.entry.trust?.warnings} />
        </>
      }
      rail={
        <>
          <MarketplaceExtensionDetailsCard data={data} />
          <MarketplaceExtensionTrustCard trust={data.entry.trust} />
        </>
      }
    />
  );
}

function MarketplaceExtensionArtifactSection({
  extension,
}: {
  extension: MarketplaceEntryResponse["extension"];
}) {
  if (!extension) return null;
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-extension-artifact"
      icon={Fingerprint}
      summary="pinned archive"
      title="Provenance"
    >
      {extension.repository ? (
        <MarketplaceDetailKvRow label="Repository">
          <a
            className="font-mono text-form-hint break-all text-muted transition-colors duration-base hover:text-fg-strong"
            href={extension.repository}
            rel="noreferrer"
            target="_blank"
          >
            {extension.repository} ↗
          </a>
        </MarketplaceDetailKvRow>
      ) : null}
      <MarketplaceDetailKvRow
        label="Artifact"
        sub="Install downloads exactly this archive and verifies it against the digest below."
      >
        <a
          className="font-mono text-form-hint break-all text-muted transition-colors duration-base hover:text-fg-strong"
          href={extension.artifact_url}
          rel="noreferrer"
          target="_blank"
        >
          {extension.artifact_url} ↗
        </a>
      </MarketplaceDetailKvRow>
      <MarketplaceDetailKvRow label="SHA-256">
        <MonoId value={extension.digest_sha256} />
      </MarketplaceDetailKvRow>
    </MarketplaceDetailSection>
  );
}

function MarketplaceExtensionDetailsCard({
  data,
  defaultOpen = true,
}: {
  data: MarketplaceEntryResponse;
  defaultOpen?: boolean;
}) {
  const entry = data.entry;
  const version = formatMarketplaceVersion(entry.version);
  return (
    <MarketplaceDetailRailCard
      defaultOpen={defaultOpen}
      icon={FileText}
      summary={[version, entry.source].filter(Boolean).join(" · ") || entry.source}
      title="Details"
    >
      <div className="px-3.5">
        {version ? (
          <PropertyRow label="Version" mono>
            {version}
          </PropertyRow>
        ) : null}
        <PropertyRow label="Entry ID" mono>
          {entry.entry_id}
        </PropertyRow>
        {entry.author ? <PropertyRow label="Author">{entry.author}</PropertyRow> : null}
        <PropertyRow label="Source" mono>
          {entry.source}
        </PropertyRow>
        {entry.downloads !== undefined && entry.downloads !== null ? (
          <PropertyRow label="Downloads" mono>
            {formatMarketplaceCount(entry.downloads)}
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

function MarketplaceExtensionTrustCard({
  trust,
}: {
  trust: MarketplaceEntryResponse["entry"]["trust"];
}) {
  if (!trust) return null;
  return (
    <MarketplaceDetailRailCard
      icon={ShieldCheck}
      summary={[trust.decision.replaceAll("_", " "), trust.registry_tier].join(" · ")}
      title="Trust"
    >
      <div className="px-3.5">
        <PropertyRow label="Decision">{trust.decision.replaceAll("_", " ")}</PropertyRow>
        <PropertyRow label="Registry tier">{trust.registry_tier}</PropertyRow>
        <PropertyRow label="Checksum">
          {trust.checksum_verified ? "verified" : "not yet verified"}
        </PropertyRow>
        <PropertyRow label="Warnings" mono>
          {String(trust.warnings?.length ?? 0)}
        </PropertyRow>
      </div>
    </MarketplaceDetailRailCard>
  );
}

export {
  MarketplaceDetailExtensionView,
  MarketplaceExtensionDetailsCard,
  MarketplaceExtensionTrustCard,
};
export type { MarketplaceDetailExtensionViewProps };
