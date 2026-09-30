import {
  BadgeCheck,
  CircleCheck,
  Hand,
  KeyRound,
  Package,
  ScrollText,
  Shield,
  Stethoscope,
} from "lucide-react";

import { MonoId } from "@compozy/ui";

import type { MarketplaceCatalogEntryResponse } from "../types";
import {
  MarketplaceExtensionAdvancedCard,
  MarketplaceExtensionManageCard,
} from "./marketplace-detail-extension-rail";
import { MarketplaceExtensionServerSection } from "./marketplace-detail-extension-server";
import {
  ExtensionDiagnostics,
  ExtensionEnvironmentState,
} from "./marketplace-detail-extension-sections";
import { MarketplaceExtensionAboutCard } from "./marketplace-detail-extension";
import { MarketplaceDetailColumns, MarketplaceDetailSection } from "./marketplace-detail-shell";
import { MarketplaceDetailWarnings } from "./marketplace-detail-warnings";
import { MarketplaceTrustWarningList } from "./marketplace-trust-warning-list";
import { MarketplaceDetailManageFallbackCard } from "./marketplace-detail-manage-state";
import {
  type ExtensionEntry,
  ExtensionDeclaredProfiles,
  ExtensionKitInventoryPanel,
  type ExtensionLogEventSource,
  ExtensionLogPanel,
  ExtensionGatewayConfirmDialog,
  ExtensionProvenanceDialog,
  ExtensionSkippedComponents,
  extensionTrustFacts,
  RemoveExtensionDialog,
  selectExtensionSkippedDiagnostics,
  useExtensionDetailState,
} from "@/systems/extensions";

interface MarketplaceDetailExtensionInstalledProps {
  data: MarketplaceCatalogEntryResponse;
  logEventSourceFactory?: (url: string) => ExtensionLogEventSource;
  /** Server cards poll their live Settings definition only while this is on. */
  liveDataEnabled?: boolean;
}

/**
 * Installed extension detail, calm by default: what's inside and anything that needs the user
 * (setup, problems) stay open; activity, access, and profile placement sit closed below. The rail
 * manages it — Manage, a Server card per provided MCP server, About — and keeps the operator
 * facts (process, provenance, remote access) in one closed Advanced card.
 */
function MarketplaceDetailExtensionInstalled({
  data,
  logEventSourceFactory,
  liveDataEnabled = true,
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

  const { extension } = detailData;
  const facts = extensionTrustFacts(extension);

  return (
    <>
      <MarketplaceDetailColumns
        main={
          <>
            <MarketplaceExtensionKitSection state={state} />
            <MarketplaceExtensionEnvironmentSection extension={extension} />
            <MarketplaceExtensionProblemsSection
              extension={extension}
              warnings={entry.trust?.warnings ?? []}
            />
            <MarketplaceDetailSection defaultOpen={false} icon={ScrollText} title="Activity">
              <ExtensionLogPanel bare logs={state.logs} name={name} />
            </MarketplaceDetailSection>
            <MarketplaceExtensionAccessSection extension={extension} />
            <MarketplaceExtensionProfilesSection extension={extension} />
          </>
        }
        rail={
          <>
            <MarketplaceExtensionManageCard
              extension={extension}
              facts={facts}
              onRequestProvenance={state.requestProvenance}
              onRequestRemoval={state.requestRemoval}
              onToggleEnabled={state.requestToggle}
              togglePending={state.toggle.isPending}
            />
            <MarketplaceExtensionServerSection
              inputs={data.extension?.inputs ?? extension.inputs}
              installed
              liveDataEnabled={liveDataEnabled}
              missingInputs={extension.missing_inputs}
              servers={extension.mcp_servers}
            />
            <MarketplaceExtensionAboutCard data={data} />
            <MarketplaceExtensionAdvancedCard data={data} extension={extension} facts={facts} />
          </>
        }
      />
      {state.gatewayConfirm ? (
        <ExtensionGatewayConfirmDialog
          digest={state.gatewayConfirm.digest}
          extensionName={extension.name}
          onConfirm={state.submitGatewayConfirm}
          onOpenChange={open => {
            if (!open) state.dismissGatewayConfirm();
          }}
          open
          pending={state.update.isPending}
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
        onRemoved={() => void state.navigate({ search: {}, to: "/marketplace/installed" })}
        open={state.activeDialog === "remove"}
      />
    </>
  );
}

type ExtensionDetailState = ReturnType<typeof useExtensionDetailState>;

function MarketplaceExtensionKitSection({ state }: { state: ExtensionDetailState }) {
  const items = state.inventory.data?.items;
  const total = items?.length ?? 0;
  const skippedDiagnostics = selectExtensionSkippedDiagnostics(state.inventory.data?.diagnostics);
  const allLive = total > 0 && items!.every(item => item.live);
  const settled = !state.inventory.isLoading && !state.inventory.error;
  const summary = settled
    ? `${total} ${total === 1 ? "item" : "items"}${allLive ? " · all ready" : ""}`
    : undefined;
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-extension-kit"
      icon={Package}
      summary={summary}
      title="What's inside"
    >
      <ExtensionKitInventoryPanel
        bare
        error={state.inventory.error}
        isLoading={state.inventory.isLoading}
        items={items}
        onRetry={() => void state.inventory.refetch()}
        showEmptyState={skippedDiagnostics.length === 0}
      />
      {settled ? (
        <ExtensionSkippedComponents diagnostics={skippedDiagnostics} ingestedCount={total} />
      ) : null}
    </MarketplaceDetailSection>
  );
}

function MarketplaceExtensionAccessSection({ extension }: { extension: ExtensionEntry }) {
  const capabilities = extension.capabilities ?? [];
  const permissions = extension.permissions ?? [];
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-extension-access"
      defaultOpen={false}
      icon={Shield}
      summary={`${capabilities.length} ${capabilities.length === 1 ? "capability" : "capabilities"} · ${permissions.length} ${permissions.length === 1 ? "permission" : "permissions"}`}
      title="What it can do"
    >
      <MarketplaceExtensionAccessRow
        description={
          capabilities.length ? "Features other extensions and skills can build on." : "None."
        }
        icon={<BadgeCheck aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />}
        title="Capabilities it provides"
        values={capabilities}
      />
      <MarketplaceExtensionAccessRow
        description={
          permissions.length
            ? "Allowed while the extension is on, removed when you turn it off."
            : "None — it doesn't ask for any special access."
        }
        icon={<Hand aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />}
        title="Permissions it needs"
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
        <p className="text-small-body font-medium text-fg">{title}</p>
        <p className="mt-0.5 max-w-prose text-form-label leading-relaxed text-muted">
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
  if (!required.length && !bound.length) return null;
  const summary = missing.length
    ? `${missing.length} missing`
    : required.length
      ? "all set"
      : undefined;
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-extension-environment"
      defaultOpen={missing.length > 0}
      icon={KeyRound}
      summary={summary}
      title="Setup"
    >
      <div className="px-4 py-3">
        <ExtensionEnvironmentState bound={bound} missing={missing} required={required} />
      </div>
    </MarketplaceDetailSection>
  );
}

/** Diagnostics, the last error, and trust warnings in one section — absent when all is well. */
function MarketplaceExtensionProblemsSection({
  extension,
  warnings,
}: {
  extension: ExtensionEntry;
  warnings: NonNullable<NonNullable<MarketplaceCatalogEntryResponse["entry"]["trust"]>["warnings"]>;
}) {
  const diagnostics = extension.diagnostics ?? [];
  const count = diagnostics.length + warnings.length + (extension.last_error ? 1 : 0);
  if (count === 0) return null;
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-extension-diagnostics"
      icon={Stethoscope}
      summary={`${count} ${count === 1 ? "problem" : "problems"}`}
      title="Problems"
    >
      <div className="flex flex-col gap-2 p-3">
        <ExtensionDiagnostics diagnostics={diagnostics} lastError={extension.last_error} />
        <MarketplaceTrustWarningList items={warnings} />
      </div>
    </MarketplaceDetailSection>
  );
}

function MarketplaceExtensionProfilesSection({ extension }: { extension: ExtensionEntry }) {
  const profiles = extension.declared_profiles ?? [];
  const placements = extension.placements ?? [];
  if (profiles.length === 0 && placements.length === 0) return null;
  const needsAttention =
    profiles.some(profile => profile.needs_setup) ||
    placements.some(placement => placement.dormant);
  return (
    <MarketplaceDetailSection
      defaultOpen={needsAttention}
      icon={CircleCheck}
      title="Profiles and placement"
    >
      <ExtensionDeclaredProfiles extension={extension} />
    </MarketplaceDetailSection>
  );
}

export { MarketplaceDetailExtensionInstalled };
export type { MarketplaceDetailExtensionInstalledProps };
