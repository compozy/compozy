import { MonoId, Pill, Section } from "@compozy/ui";

import {
  ExtensionLogPanel,
  ExtensionProvenanceDialog,
  extensionTrustFacts,
  RemoveExtensionDialog,
  useExtensionDetailState,
  VerifiedMark,
  type ExtensionEntry,
  type ExtensionLogEventSource,
} from "@/systems/extensions";
import { formatUptimeSeconds } from "@/lib/format-time";

import {
  ExtensionUpdateConsentDialog,
  MarketplaceDetailExtensionActions,
} from "./marketplace-detail-extension-actions";
import {
  ExtensionBundlesProvided,
  ExtensionDetailBlock,
  ExtensionDiagnostics,
  ExtensionEnvironmentState,
  ExtensionRailBlock,
  ExtensionTokenBlock,
} from "./marketplace-detail-extension-sections";
import { MarketplaceDetailManageState } from "./marketplace-detail-manage-state";
import { marketplaceErrorMessage } from "./marketplace-ui";

interface MarketplaceDetailExtensionManageProps {
  name: string;
  /**
   * The catalog projection also reports a newer published release for an installed extension the
   * curated join cannot key (GitHub and git installs). Both signals come from the daemon.
   */
  catalogUpdateAvailable?: boolean;
  catalogVersion?: string;
  logEventSourceFactory?: (url: string) => ExtensionLogEventSource;
}

function MarketplaceDetailExtensionManage({
  name,
  catalogUpdateAvailable = false,
  catalogVersion,
  logEventSourceFactory,
}: MarketplaceDetailExtensionManageProps) {
  const state = useExtensionDetailState(name, {
    logEventSourceFactory,
    updateVersion: catalogVersion,
  });
  const {
    bundles,
    detail,
    navigate,
    activeDialog,
    dismissDialog,
    logs,
    requestProvenance,
    requestRemoval,
    requestUpdate,
    submitUpdate,
    toggle,
    update,
  } = state;
  const data = detail.data;
  if (!data) {
    return (
      <MarketplaceDetailManageState
        error={detail.error ?? null}
        isLoading={detail.isLoading}
        label="Extension"
        onRetry={() => void detail.refetch()}
        testId={
          detail.isLoading
            ? "marketplace-extension-manage-loading"
            : "marketplace-extension-manage-error"
        }
      />
    );
  }

  const { extension } = data;
  const provenance = extension.provenance;
  const facts = extensionTrustFacts(extension);

  return (
    <>
      <Section label="Manage">
        <MarketplaceDetailExtensionActions
          catalogUpdateAvailable={catalogUpdateAvailable}
          catalogVersion={catalogVersion}
          extension={extension}
          facts={facts}
          isUpdatePending={update.isPending}
          onRequestProvenance={requestProvenance}
          onRequestRemoval={requestRemoval}
          onToggleEnabled={enabled => toggle.mutate({ enabled, name })}
          onUpdate={requestUpdate}
          togglePending={toggle.isPending}
        />
      </Section>
      <ExtensionTokenBlock label="Capabilities" values={extension.capabilities ?? []} />
      <ExtensionTokenBlock label="Permissions" values={extension.permissions ?? []} />
      <ExtensionDetailBlock label="Environment">
        <ExtensionEnvironmentState
          missing={extension.missing_env ?? []}
          required={extension.requires_env ?? []}
        />
      </ExtensionDetailBlock>
      <ExtensionDetailBlock label="Diagnostics">
        <ExtensionDiagnostics
          diagnostics={extension.diagnostics ?? []}
          lastError={extension.last_error}
        />
      </ExtensionDetailBlock>
      <ExtensionLogPanel logs={logs} name={name} />
      <ExtensionRailBlock label="Runtime" rows={runtimeRows(extension)} />
      <ExtensionRailBlock
        label="Provenance"
        rows={[
          {
            term: "Installed from",
            value: (
              <code className="break-all font-mono text-xs text-fg">
                {provenance?.installed_from ?? extension.source}
              </code>
            ),
          },
          {
            term: "Source",
            value: (
              <code className="break-all font-mono text-xs text-fg">
                {provenance?.source_url ?? provenance?.slug ?? extension.source}
              </code>
            ),
          },
          {
            term: "Checksum",
            value: provenance?.checksum_sha256 ? (
              <MonoId value={provenance.checksum_sha256} />
            ) : (
              <code className="font-mono text-xs text-fg">—</code>
            ),
          },
          {
            term: "Archive integrity",
            value: (
              <Pill
                data-testid="extension-provenance-digest"
                mono
                size="xs"
                tone={facts.digestMatched ? "info" : "neutral"}
              >
                {facts.digestMatched ? "digest matched" : "no digest recorded"}
              </Pill>
            ),
          },
          {
            term: "Curated checksum",
            value: (
              <>
                <Pill
                  data-testid="extension-provenance-checksum"
                  mono
                  size="xs"
                  tone={facts.checksumVerified ? "success" : "neutral"}
                >
                  {facts.checksumVerified ? "verified" : "not pinned"}
                </Pill>
                <VerifiedMark verified={facts.checksumVerified} />
              </>
            ),
          },
          {
            term: "Registry tier",
            value: <code className="font-mono text-xs text-fg">{facts.registryTier ?? "—"}</code>,
          },
        ]}
      />
      <ExtensionBundlesProvided
        active={bundles.data}
        error={bundles.error}
        extensionName={extension.name}
        isLoading={bundles.isLoading}
        onRetry={() => void bundles.refetch()}
        provided={extension.bundles ?? []}
      />
      <ExtensionProvenanceDialog
        extension={extension}
        onOpenChange={open => (open ? requestProvenance() : dismissDialog())}
        open={activeDialog === "provenance"}
      />
      <ExtensionUpdateConsentDialog
        error={update.error ? marketplaceErrorMessage(update.error, "Update failed") : undefined}
        extensionName={extension.name}
        onConfirm={submitUpdate}
        onOpenChange={open => (open ? requestUpdate() : dismissDialog())}
        open={activeDialog === "update"}
        pending={update.isPending}
        targetVersion={catalogVersion ?? extension.remote_version ?? ""}
      />
      <RemoveExtensionDialog
        activeBundles={bundles.data}
        dependencyError={bundles.error}
        dependencyLoading={bundles.isLoading}
        extension={extension}
        onOpenChange={open => (open ? requestRemoval() : dismissDialog())}
        onRemoved={() =>
          void navigate({ search: { tab: "installed" }, to: "/marketplace/extensions" })
        }
        onRetryDependencies={() => void bundles.refetch()}
        open={activeDialog === "remove"}
      />
    </>
  );
}

function runtimeRows(extension: ExtensionEntry) {
  const facts = extensionTrustFacts(extension);
  const rows = [
    {
      term: "State",
      value: (
        <Pill mono size="xs" tone={extension.daemon_running ? "success" : "neutral"}>
          {extension.state}
        </Pill>
      ),
    },
    {
      term: "Health",
      value: extension.health ? (
        <>
          <Pill mono size="xs" tone={extension.health === "healthy" ? "success" : "warning"}>
            {extension.health}
          </Pill>
          {extension.health_message ? <span>{extension.health_message}</span> : null}
        </>
      ) : (
        <code className="font-mono text-xs text-fg">—</code>
      ),
    },
    {
      term: "Daemon",
      value: (
        <Pill mono size="xs" tone={extension.daemon_running ? "success" : "neutral"}>
          {extension.daemon_running ? "running" : "stopped"}
        </Pill>
      ),
    },
    {
      term: "Consecutive failures",
      value: (
        <code
          className={`font-mono text-xs ${extension.consecutive_failures > 0 ? "text-danger" : "text-fg"}`}
          data-testid="extension-consecutive-failures"
        >
          {String(extension.consecutive_failures)}
        </code>
      ),
    },
    {
      term: "Restart backoff",
      value: (
        <code className="font-mono text-xs text-fg" data-testid="extension-restart-backoff">
          {extension.restart_backoff_ms > 0 ? `${extension.restart_backoff_ms} ms` : "none"}
        </code>
      ),
    },
    {
      term: "PID",
      value: (
        <code className="font-mono text-xs text-fg">
          {extension.pid ? String(extension.pid) : "—"}
        </code>
      ),
    },
    {
      term: "Uptime",
      value: (
        <code className="font-mono text-xs text-fg">
          {formatUptimeSeconds(extension.uptime_seconds)}
        </code>
      ),
    },
  ];
  if (extension.failure_code) {
    rows.push({
      term: "Failure code",
      value: (
        <Pill data-testid="extension-failure-code" mono size="xs" tone="danger">
          {extension.failure_code}
        </Pill>
      ),
    });
  }
  if (facts.originPath) {
    rows.push({
      term: "Origin path",
      value: (
        <code className="break-all font-mono text-xs text-fg" data-testid="extension-origin-path">
          {facts.originPath}
        </code>
      ),
    });
  }
  if (extension.generation_hash) {
    rows.push({
      term: "Generation",
      value: <MonoId value={extension.generation_hash} />,
    });
  }
  return rows;
}

export { MarketplaceDetailExtensionManage };
export type { MarketplaceDetailExtensionManageProps };
