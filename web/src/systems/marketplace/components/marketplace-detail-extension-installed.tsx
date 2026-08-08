import {
  Activity,
  BadgeCheck,
  CircleCheck,
  Fingerprint,
  Hand,
  KeyRound,
  Network,
  Package,
  ScrollText,
  Shield,
  SlidersHorizontal,
  Stethoscope,
  Zap,
} from "lucide-react";

import { Empty, MonoId, Pill, PropertyRow } from "@compozy/ui";

import {
  ExtensionKitInventoryPanel,
  ExtensionLogPanel,
  ExtensionNetworkConfirmDialog,
  ExtensionProvenanceDialog,
  extensionTrustFacts,
  RemoveExtensionDialog,
  useExtensionDetailState,
  VerifiedMark,
  type ExtensionEntry,
  type ExtensionLogEventSource,
} from "@/systems/extensions";
import { formatUptimeSeconds } from "@/lib/format-time";

import type { MarketplaceEntryResponse } from "../types";
import { MarketplaceDetailExtensionActions } from "./marketplace-detail-extension-actions";
import {
  ExtensionDiagnostics,
  ExtensionEnvironmentState,
} from "./marketplace-detail-extension-sections";
import {
  MarketplaceExtensionDetailsCard,
  MarketplaceExtensionTrustCard,
} from "./marketplace-detail-extension";
import {
  MarketplaceDetailColumns,
  MarketplaceDetailRailCard,
  MarketplaceDetailRailNote,
  MarketplaceDetailSection,
} from "./marketplace-detail-shell";
import { MarketplaceDetailWarnings } from "./marketplace-detail-warnings";
import { MarketplaceDetailManageFallbackCard } from "./marketplace-detail-manage-state";

interface MarketplaceDetailExtensionInstalledProps {
  data: MarketplaceEntryResponse;
  logEventSourceFactory?: (url: string) => ExtensionLogEventSource;
}

/** Installed extension detail: the kit and its runtime are the body; the rail manages it. */
function MarketplaceDetailExtensionInstalled({
  data,
  logEventSourceFactory,
}: MarketplaceDetailExtensionInstalledProps) {
  const entry = data.entry;
  const name = entry.installed_name?.trim() || entry.name;
  const state = useExtensionDetailState(name, {
    logEventSourceFactory,
    updateVersion: entry.version,
  });
  const detailData = state.detail.data;

  if (!detailData) {
    return (
      <MarketplaceDetailColumns
        main={<MarketplaceDetailWarnings warnings={entry.trust?.warnings} />}
        rail={
          <>
            <MarketplaceDetailManageFallbackCard
              error={state.detail.error ?? null}
              isLoading={state.detail.isLoading}
              label="Extension"
              onRetry={() => void state.detail.refetch()}
              testId={
                state.detail.isLoading
                  ? "marketplace-extension-manage-loading"
                  : "marketplace-extension-manage-error"
              }
            />
            <MarketplaceExtensionDetailsCard data={data} />
            <MarketplaceExtensionTrustCard trust={entry.trust} />
          </>
        }
      />
    );
  }

  const { extension } = detailData;
  const facts = extensionTrustFacts(extension);

  return (
    <>
      <MarketplaceDetailColumns
        main={
          <>
            {state.enableResult?.automation_started.length ? (
              <MarketplaceExtensionAutomationSection
                started={state.enableResult.automation_started}
              />
            ) : null}
            {state.workspaceId === null ? <MarketplaceExtensionKitSection state={state} /> : null}
            <MarketplaceExtensionAccessSection extension={extension} />
            <MarketplaceExtensionEnvironmentSection extension={extension} />
            <MarketplaceExtensionDiagnosticsSection extension={extension} />
            <MarketplaceDetailSection icon={ScrollText} title="Logs">
              <ExtensionLogPanel bare logs={state.logs} name={name} />
            </MarketplaceDetailSection>
            <MarketplaceDetailWarnings warnings={entry.trust?.warnings} />
          </>
        }
        rail={
          <>
            <MarketplaceExtensionManageCard extension={extension} facts={facts} state={state} />
            <MarketplaceExtensionRuntimeCard extension={extension} />
            <MarketplaceExtensionTrustCard trust={entry.trust} />
            <MarketplaceExtensionDetailsCard data={data} defaultOpen={false} />
            <MarketplaceExtensionProvenanceCard extension={extension} facts={facts} />
            <MarketplaceExtensionNetworkCard extension={extension} />
          </>
        }
      />
      {state.networkConfirm ? (
        <ExtensionNetworkConfirmDialog
          action={state.networkConfirm.action}
          digest={state.networkConfirm.digest}
          extensionName={extension.name}
          onConfirm={state.submitNetworkConfirm}
          onOpenChange={open => {
            if (!open) state.dismissNetworkConfirm();
          }}
          open
          pending={
            state.networkConfirm.action === "enable"
              ? state.toggle.isPending
              : state.update.isPending
          }
        />
      ) : null}
      <ExtensionProvenanceDialog
        extension={extension}
        onOpenChange={open => (open ? state.requestProvenance() : state.dismissDialog())}
        open={state.activeDialog === "provenance"}
      />
      <RemoveExtensionDialog
        extension={extension}
        onOpenChange={open => (open ? state.requestRemoval() : state.dismissDialog())}
        onRemoved={() => void state.navigate({ search: {}, to: "/marketplace/extensions" })}
        open={state.activeDialog === "remove"}
      />
    </>
  );
}

type ExtensionDetailState = ReturnType<typeof useExtensionDetailState>;

function MarketplaceExtensionKitSection({ state }: { state: ExtensionDetailState }) {
  const items = state.inventory.data?.items;
  const total = items?.length ?? 0;
  const allLive = total > 0 && items!.every(item => item.live);
  const summary =
    state.inventory.isLoading || state.inventory.error
      ? undefined
      : `${total} ${total === 1 ? "item" : "items"}${allLive ? " · all live" : ""}`;
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-extension-kit"
      icon={Package}
      summary={summary}
      title="Kit inventory"
    >
      <ExtensionKitInventoryPanel
        bare
        error={state.inventory.error}
        isLoading={state.inventory.isLoading}
        items={items}
        onRetry={() => void state.inventory.refetch()}
      />
    </MarketplaceDetailSection>
  );
}

function MarketplaceExtensionAutomationSection({ started }: { started: string[] }) {
  return (
    <MarketplaceDetailSection
      data-testid="extension-automation-started"
      icon={Zap}
      summary={String(started.length)}
      title="Automation started"
    >
      <ul className="divide-y divide-line-soft">
        {[...started].sort().map(startedName => (
          <li className="px-4 py-2.5" key={startedName}>
            <MonoId value={startedName} />
          </li>
        ))}
      </ul>
    </MarketplaceDetailSection>
  );
}

function MarketplaceExtensionAccessSection({ extension }: { extension: ExtensionEntry }) {
  const capabilities = extension.capabilities ?? [];
  const permissions = extension.permissions ?? [];
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-extension-access"
      icon={Shield}
      summary={`${capabilities.length} ${capabilities.length === 1 ? "capability" : "capabilities"} · ${permissions.length} ${permissions.length === 1 ? "permission" : "permissions"}`}
      title="Access"
    >
      <MarketplaceExtensionAccessRow
        description={
          capabilities.length
            ? "Other extensions and skills can depend on these."
            : "None registered."
        }
        icon={<BadgeCheck aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />}
        title="Provides capabilities"
        values={capabilities}
      />
      <MarketplaceExtensionAccessRow
        description={
          permissions.length
            ? "Granted while the extension is enabled, revoked on disable."
            : "None registered — this kit never calls privileged host APIs."
        }
        icon={<Hand aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />}
        title="Requires host permissions"
        values={permissions}
      />
    </MarketplaceDetailSection>
  );
}

function MarketplaceExtensionAccessRow({
  description,
  icon,
  title,
  values,
}: {
  description: string;
  icon: React.ReactNode;
  title: string;
  values: readonly string[];
}) {
  return (
    <div className="flex items-start gap-2.5 border-t border-line-soft px-4 py-3 first:border-t-0">
      {icon}
      <div className="min-w-0">
        <p className="text-small-body font-medium text-fg-strong">{title}</p>
        <p className="mt-0.5 max-w-[64ch] text-form-label leading-relaxed text-muted">
          {description}
        </p>
        {values.length ? (
          <div className="mt-1.5 flex flex-wrap gap-1.5">
            {values.map(value => (
              <MonoId key={value} value={value} />
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}

function MarketplaceExtensionEnvironmentSection({ extension }: { extension: ExtensionEntry }) {
  const required = extension.requires_env ?? [];
  const missing = extension.missing_env ?? [];
  const bound = extension.bound_env_keys ?? [];
  const summary = required.length
    ? missing.length
      ? `${missing.length} missing`
      : `${required.length} required · all resolved`
    : "none required";
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-extension-environment"
      icon={KeyRound}
      summary={summary}
      title="Environment"
    >
      <div className="px-4 py-3">
        <ExtensionEnvironmentState bound={bound} missing={missing} required={required} />
      </div>
    </MarketplaceDetailSection>
  );
}

function MarketplaceExtensionDiagnosticsSection({ extension }: { extension: ExtensionEntry }) {
  const diagnostics = extension.diagnostics ?? [];
  const hasIssues = diagnostics.length > 0 || Boolean(extension.last_error);
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-extension-diagnostics"
      defaultOpen={hasIssues}
      icon={Stethoscope}
      summary={
        hasIssues ? `${diagnostics.length + (extension.last_error ? 1 : 0)} findings` : "no issues"
      }
      title="Diagnostics"
    >
      {hasIssues ? (
        <div className="px-4 py-3">
          <ExtensionDiagnostics diagnostics={diagnostics} lastError={extension.last_error} />
        </div>
      ) : (
        <Empty
          className="py-9"
          description="No diagnostics recorded for this generation. Issues from validation, trust checks, or the process supervisor show up here."
          fill={false}
          icon={CircleCheck}
          title="Nothing to report"
        />
      )}
    </MarketplaceDetailSection>
  );
}

function MarketplaceExtensionManageCard({
  extension,
  facts,
  state,
}: {
  extension: ExtensionEntry;
  facts: ReturnType<typeof extensionTrustFacts>;
  state: ExtensionDetailState;
}) {
  return (
    <MarketplaceDetailRailCard
      icon={SlidersHorizontal}
      summary={extension.enabled ? "enabled" : "disabled"}
      title="Manage"
    >
      <MarketplaceDetailExtensionActions
        extension={extension}
        facts={facts}
        onRequestProvenance={state.requestProvenance}
        onRequestRemoval={state.requestRemoval}
        onToggleEnabled={state.requestToggle}
        togglePending={state.toggle.isPending}
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

function MarketplaceExtensionRuntimeCard({ extension }: { extension: ExtensionEntry }) {
  const facts = extensionTrustFacts(extension);
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
        <PropertyRow label="Daemon">{extension.daemon_running ? "running" : "stopped"}</PropertyRow>
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

export { MarketplaceDetailExtensionInstalled };
export type { MarketplaceDetailExtensionInstalledProps };
