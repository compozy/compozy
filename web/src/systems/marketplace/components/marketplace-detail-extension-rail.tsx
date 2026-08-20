import { Activity, Fingerprint, Network, SlidersHorizontal } from "lucide-react";

import { MonoId, Pill, PropertyRow } from "@compozy/ui";

import { formatUptimeSeconds } from "@/lib/format-time";

import { MarketplaceDetailExtensionActions } from "./marketplace-detail-extension-actions";
import { MarketplaceDetailRailCard, MarketplaceDetailRailNote } from "./marketplace-detail-shell";
import { type ExtensionEntry, extensionTrustFacts, VerifiedMark } from "@/systems/extensions";

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
  return (
    <MarketplaceDetailRailCard
      icon={SlidersHorizontal}
      summary={extension.enabled ? "enabled" : "disabled"}
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
      {extension.workspace_id ? (
        <div className="px-3.5">
          <PropertyRow label="Workspace" mono>
            {extension.workspace_id}
          </PropertyRow>
        </div>
      ) : null}
      <MarketplaceDetailRailNote>
        Disabling unpublishes the kit and stops the subprocess.
      </MarketplaceDetailRailNote>
    </MarketplaceDetailRailCard>
  );
}

function MarketplaceExtensionRuntimeCard({
  extension,
  facts,
}: {
  extension: ExtensionEntry;
  facts: ReturnType<typeof extensionTrustFacts>;
}) {
  const summary = [
    extension.health ?? undefined,
    extension.uptime_seconds ? `up ${formatUptimeSeconds(extension.uptime_seconds)}` : undefined,
  ]
    .filter(Boolean)
    .join(" · ");
  return (
    <MarketplaceDetailRailCard
      data-testid="marketplace-extension-runtime"
      icon={Activity}
      summary={summary || extension.state}
      title="Runtime"
    >
      <div className="px-3.5">
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
        <PropertyRow label="Runtime">
          {extension.daemon_running ? "running" : "stopped"}
        </PropertyRow>
        <PropertyRow label="PID" mono>
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
    </MarketplaceDetailRailCard>
  );
}

function MarketplaceExtensionProvenanceCard({
  extension,
  facts,
}: {
  extension: ExtensionEntry;
  facts: ReturnType<typeof extensionTrustFacts>;
}) {
  const provenance = extension.provenance;
  return (
    <MarketplaceDetailRailCard
      defaultOpen={false}
      icon={Fingerprint}
      summary={facts.digestMatched ? "digest matched" : "no digest recorded"}
      title="Provenance"
    >
      <div className="px-3.5">
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
      </div>
    </MarketplaceDetailRailCard>
  );
}

function MarketplaceExtensionNetworkCard({ extension }: { extension: ExtensionEntry }) {
  if (!extension.network_requirement_digest) return null;
  return (
    <MarketplaceDetailRailCard
      defaultOpen={extension.network_confirmation_required === true}
      icon={Network}
      summary={extension.network_confirmation_required ? "confirmation required" : "confirmed"}
      title="Network"
    >
      <div className="px-3.5">
        <PropertyRow
          editor={
            <Pill
              data-testid="extension-network-consent"
              mono
              size="xs"
              tone={extension.network_confirmation_required ? "warning" : "success"}
            >
              {extension.network_confirmation_required ? "confirmation required" : "confirmed"}
            </Pill>
          }
          label="Consent"
        />
        <PropertyRow
          editor={<MonoId value={extension.network_requirement_digest} />}
          label="Digest"
        />
      </div>
    </MarketplaceDetailRailCard>
  );
}

export {
  MarketplaceExtensionManageCard,
  MarketplaceExtensionNetworkCard,
  MarketplaceExtensionProvenanceCard,
  MarketplaceExtensionRuntimeCard,
};
export type { MarketplaceExtensionManageCardProps };
