import { SlidersHorizontal, Wrench } from "lucide-react";
import type { ReactNode } from "react";

import { Eyebrow, MonoId, Pill, PropertyRow } from "@compozy/ui";

import { formatUptimeSeconds } from "@/lib/format-time";

import type { MarketplaceCatalogEntryResponse } from "../types";
import { MarketplaceDetailExtensionActions } from "./marketplace-detail-extension-actions";
import { MarketplaceDetailRailCard } from "./marketplace-detail-shell";
import {
  type ExtensionEntry,
  ExtensionFormatBadge,
  ExtensionTrustBadges,
  extensionTrustFacts,
  VerifiedMark,
} from "@/systems/extensions";

interface MarketplaceExtensionManageCardProps {
  extension: ExtensionEntry;
  facts: ReturnType<typeof extensionTrustFacts>;
  onRequestProvenance: () => void;
  onRequestRemoval: () => void;
  onToggleEnabled: (enabled: boolean) => void;
  togglePending: boolean;
}

function MarketplaceExtensionManageCard({
  extension,
  facts,
  onRequestProvenance,
  onRequestRemoval,
  onToggleEnabled,
  togglePending,
}: MarketplaceExtensionManageCardProps) {
  const status = extensionStatusWord(extension);
  return (
    <MarketplaceDetailRailCard
      icon={SlidersHorizontal}
      summary={<span data-testid="marketplace-extension-status-word">{status}</span>}
      title="Manage"
    >
      <MarketplaceDetailExtensionActions
        extension={extension}
        facts={facts}
        onRequestProvenance={onRequestProvenance}
        onRequestRemoval={onRequestRemoval}
        onToggleEnabled={onToggleEnabled}
        togglePending={togglePending}
      />
      <div className="px-3.5">
        <PropertyRow label="Installed in">{extensionPlacementWord(extension)}</PropertyRow>
      </div>
    </MarketplaceDetailRailCard>
  );
}

/** One plain status word for the Manage summary: Off, Running, On, or Having trouble. */
function extensionStatusWord(extension: ExtensionEntry): string {
  if (!extension.enabled) return "Off";
  const troubled =
    extension.consecutive_failures > 0 ||
    Boolean(extension.failure_code) ||
    (Boolean(extension.health) && extension.health !== "healthy");
  if (troubled)
    return extension.restart_backoff_ms > 0 ? "Having trouble · restarting" : "Having trouble";
  return extension.daemon_running ? "Running" : "On";
}

function extensionPlacementWord(extension: ExtensionEntry): string {
  const profile = extension.profile || "default";
  const where = extension.workspace_id ? "This project" : "All projects";
  return profile === "default" ? where : `${where} · profile ${profile}`;
}

/**
 * Advanced: the operator facts — process status, where the package came from, and the remote
 * access consent — kept one step deeper and closed unless remote access needs confirmation.
 */
function MarketplaceExtensionAdvancedCard({
  data,
  extension,
  facts,
}: {
  data: MarketplaceCatalogEntryResponse;
  extension: ExtensionEntry;
  facts: ReturnType<typeof extensionTrustFacts>;
}) {
  return (
    <MarketplaceDetailRailCard
      data-testid="marketplace-extension-advanced"
      defaultOpen={extension.gateway_confirmation_required === true}
      icon={Wrench}
      title="Advanced"
    >
      <MarketplaceExtensionRuntimeRows extension={extension} facts={facts} />
      <MarketplaceExtensionProvenanceRows extension={extension} facts={facts} />
      <MarketplaceExtensionGatewayRows extension={extension} />
      <MarketplaceRailGroup title="Package">
        <PropertyRow label="Entry ID" mono>
          {data.entry.entry_id}
        </PropertyRow>
        {extension.workspace_id ? (
          <PropertyRow label="Project ID" mono>
            {extension.workspace_id}
          </PropertyRow>
        ) : null}
        <PropertyRow label="Profile" mono>
          {extension.profile || "default"}
        </PropertyRow>
      </MarketplaceRailGroup>
    </MarketplaceDetailRailCard>
  );
}

function MarketplaceRailGroup({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section
      aria-label={title}
      className="border-t border-line-soft px-3.5 pt-2 pb-1 first:border-t-0"
    >
      <Eyebrow className="text-faint">{title}</Eyebrow>
      {children}
    </section>
  );
}

function MarketplaceExtensionRuntimeRows({
  extension,
  facts,
}: {
  extension: ExtensionEntry;
  facts: ReturnType<typeof extensionTrustFacts>;
}) {
  return (
    <MarketplaceRailGroup title="Status">
      <div data-testid="marketplace-extension-runtime">
        <PropertyRow
          editor={
            <Pill mono tone={extension.daemon_running ? "success" : "neutral"}>
              {extension.state}
            </Pill>
          }
          label="State"
        />
        {extension.health ? (
          <PropertyRow
            editor={
              <Pill mono tone={extension.health === "healthy" ? "success" : "warning"}>
                {extension.health}
              </Pill>
            }
            label="Health"
          />
        ) : null}
        {extension.health_message ? (
          <p className="pb-1 text-transcript-caption leading-relaxed text-faint">
            {extension.health_message}
          </p>
        ) : null}
        <PropertyRow label="Process ID" mono>
          {extension.pid ? String(extension.pid) : "—"}
        </PropertyRow>
        <PropertyRow label="Uptime" mono>
          {formatUptimeSeconds(extension.uptime_seconds)}
        </PropertyRow>
        <PropertyRow label="Failures" mono>
          <span
            className={extension.consecutive_failures > 0 ? "text-danger" : undefined}
            data-testid="extension-consecutive-failures"
          >
            {String(extension.consecutive_failures)}
          </span>
        </PropertyRow>
        <PropertyRow label="Backoff" mono valueTitle="Restart backoff">
          <span data-testid="extension-restart-backoff">
            {extension.restart_backoff_ms > 0 ? `${extension.restart_backoff_ms} ms` : "none"}
          </span>
        </PropertyRow>
        {extension.failure_code ? (
          <PropertyRow
            editor={
              <Pill data-testid="extension-failure-code" mono tone="danger">
                {extension.failure_code}
              </Pill>
            }
            label="Failure code"
          />
        ) : null}
        {facts.originPath ? (
          <PropertyRow label="Origin" mono valueTitle={facts.originPath}>
            <span data-testid="extension-origin-path">{facts.originPath}</span>
          </PropertyRow>
        ) : null}
        {extension.generation_hash ? (
          <PropertyRow editor={<MonoId value={extension.generation_hash} />} label="Generation" />
        ) : null}
      </div>
    </MarketplaceRailGroup>
  );
}

function MarketplaceExtensionProvenanceRows({
  extension,
  facts,
}: {
  extension: ExtensionEntry;
  facts: ReturnType<typeof extensionTrustFacts>;
}) {
  const provenance = extension.provenance;
  return (
    <MarketplaceRailGroup title="Provenance">
      <div
        className="flex flex-wrap items-center gap-1.5 py-1.5"
        data-testid="extension-trust-badges"
      >
        <ExtensionFormatBadge format={extension.format} />
        <ExtensionTrustBadges facts={facts} showRegistryTier={false} showSource />
      </div>
      <PropertyRow label="Installed from" mono>
        {provenance?.installed_from ?? extension.source}
      </PropertyRow>
      <PropertyRow
        label="Source"
        mono
        valueTitle={provenance?.source_url ?? provenance?.slug ?? extension.source}
      >
        {provenance?.source_url ?? provenance?.slug ?? extension.source}
      </PropertyRow>
      <PropertyRow
        editor={
          provenance?.checksum_sha256 ? <MonoId value={provenance.checksum_sha256} /> : undefined
        }
        label="Checksum"
      >
        {provenance?.checksum_sha256 ? undefined : "—"}
      </PropertyRow>
      <PropertyRow
        editor={
          <Pill
            data-testid="extension-provenance-digest"
            mono
            size="xs"
            tone={facts.digestMatched ? "info" : "neutral"}
          >
            {facts.digestMatched ? "digest matched" : "no digest recorded"}
          </Pill>
        }
        label="Archive"
      />
      <PropertyRow
        editor={
          <span className="inline-flex items-center gap-1.5">
            <Pill
              data-testid="extension-provenance-checksum"
              mono
              size="xs"
              tone={facts.checksumVerified ? "success" : "neutral"}
            >
              {facts.checksumVerified ? "verified" : "not pinned"}
            </Pill>
            <VerifiedMark verified={facts.checksumVerified} />
          </span>
        }
        label="Curated checksum"
      />
      <PropertyRow label="Registry tier" mono>
        {facts.registryTier ?? "—"}
      </PropertyRow>
    </MarketplaceRailGroup>
  );
}

function MarketplaceExtensionGatewayRows({ extension }: { extension: ExtensionEntry }) {
  if (!extension.gateway_requirement_digest) return null;
  return (
    <MarketplaceRailGroup title="Remote access">
      <PropertyRow
        editor={
          <Pill
            data-testid="extension-gateway-consent"
            mono
            size="xs"
            tone={extension.gateway_confirmation_required ? "warning" : "success"}
          >
            {extension.gateway_confirmation_required ? "confirmation required" : "confirmed"}
          </Pill>
        }
        label="Consent"
      />
      <PropertyRow
        editor={<MonoId value={extension.gateway_requirement_digest} />}
        label="Digest"
      />
    </MarketplaceRailGroup>
  );
}

export { MarketplaceExtensionAdvancedCard, MarketplaceExtensionManageCard };
export type { MarketplaceExtensionManageCardProps };
