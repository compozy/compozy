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

import { Empty, MonoId } from "@compozy/ui";

import type { MarketplaceEntryResponse } from "../types";
import {
  MarketplaceExtensionManageCard,
  MarketplaceExtensionNetworkCard,
  MarketplaceExtensionProvenanceCard,
  MarketplaceExtensionRuntimeCard,
} from "./marketplace-detail-extension-rail";
import {
  ExtensionDiagnostics,
  ExtensionEnvironmentState,
} from "./marketplace-detail-extension-sections";
import {
  MarketplaceExtensionDetailsCard,
  MarketplaceExtensionTrustCard,
} from "./marketplace-detail-extension";
import { MarketplaceDetailColumns, MarketplaceDetailSection } from "./marketplace-detail-shell";
import { MarketplaceDetailWarnings } from "./marketplace-detail-warnings";
import { MarketplaceDetailManageFallbackCard } from "./marketplace-detail-manage-state";
import {
  type ExtensionEntry,
  ExtensionDeclaredProfiles,
  ExtensionKitInventoryPanel,
  type ExtensionLogEventSource,
  ExtensionLogPanel,
  ExtensionNetworkConfirmDialog,
  ExtensionProvenanceDialog,
  ExtensionSkippedComponents,
  extensionTrustFacts,
  RemoveExtensionDialog,
  selectExtensionSkippedDiagnostics,
  useExtensionDetailState,
} from "@/systems/extensions";

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
            {(extension.declared_profiles?.length ?? 0) > 0 ||
            (extension.placements?.length ?? 0) > 0 ? (
              <MarketplaceDetailSection icon={CircleCheck} title="Profiles and placement">
                <ExtensionDeclaredProfiles extension={extension} />
              </MarketplaceDetailSection>
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
            <MarketplaceExtensionManageCard
              extension={extension}
              facts={facts}
              onRequestProvenance={state.requestProvenance}
              onRequestRemoval={state.requestRemoval}
              onToggleEnabled={state.requestToggle}
              togglePending={state.toggle.isPending}
            />
            <MarketplaceExtensionRuntimeCard extension={extension} facts={facts} />
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
  const skippedDiagnostics = selectExtensionSkippedDiagnostics(state.inventory.data?.diagnostics);
  const allLive = total > 0 && items!.every(item => item.live);
  const settled = !state.inventory.isLoading && !state.inventory.error;
  const summary = settled
    ? `${total} ${total === 1 ? "item" : "items"}${allLive ? " · all live" : ""}`
    : undefined;
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

export { MarketplaceDetailExtensionInstalled };
export type { MarketplaceDetailExtensionInstalledProps };
