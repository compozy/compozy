import { MoreHorizontal } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Eyebrow,
  MonoId,
  Pill,
  Section,
  Switch,
} from "@agh/ui";

import {
  ExtensionProvenanceDialog,
  RemoveExtensionDialog,
  useExtensionDetailState,
  VerifiedMark,
} from "@/systems/extensions";
import { formatUptimeSeconds } from "@/lib/format-time";

import {
  ExtensionBundlesProvided,
  ExtensionDetailBlock,
  ExtensionDiagnostics,
  ExtensionEnvironmentState,
  ExtensionRailBlock,
  ExtensionTokenBlock,
} from "./marketplace-detail-extension-sections";
import { MarketplaceDetailManageState } from "./marketplace-detail-manage-state";

interface MarketplaceDetailExtensionManageProps {
  name: string;
}

function MarketplaceDetailExtensionManage({ name }: MarketplaceDetailExtensionManageProps) {
  const state = useExtensionDetailState(name);
  const {
    bundles,
    detail,
    navigate,
    provenanceOpen,
    removeOpen,
    setProvenanceOpen,
    setRemoveOpen,
    toggle,
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

  return (
    <>
      <Section label="Manage">
        <div className="flex items-center justify-between rounded-lg bg-canvas-soft px-4 py-3">
          <span className="flex items-center gap-2" data-testid="extension-enabled-toggle">
            <Eyebrow className="text-muted" id="marketplace-extension-enabled-label">
              {extension.enabled ? "Enabled" : "Disabled"}
            </Eyebrow>
            <Switch
              aria-labelledby="marketplace-extension-enabled-label"
              checked={extension.enabled}
              data-testid="extension-enabled-switch"
              disabled={toggle.isPending}
              onCheckedChange={enabled => toggle.mutate({ enabled, name })}
            />
          </span>
          <DropdownMenu>
            <DropdownMenuTrigger
              render={<Button aria-label={`Actions for ${name}`} size="icon-sm" variant="ghost" />}
            >
              <MoreHorizontal className="size-4" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => setProvenanceOpen(true)}>
                Provenance
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem className="text-danger" onClick={() => setRemoveOpen(true)}>
                Remove…
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </Section>
      <ExtensionTokenBlock label="Capabilities" values={extension.capabilities ?? []} />
      <ExtensionTokenBlock label="Actions" values={extension.actions ?? []} />
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
      <ExtensionRailBlock
        label="Runtime"
        rows={[
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
        ]}
      />
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
            term: "Verified",
            value: (
              <>
                <Pill mono size="xs" tone={provenance?.checksum_verified ? "success" : "warning"}>
                  {provenance?.checksum_verified ? "verified" : "unverified"}
                </Pill>
                <VerifiedMark verified={Boolean(provenance?.checksum_verified)} />
              </>
            ),
          },
          {
            term: "Registry tier",
            value: (
              <code className="font-mono text-xs text-fg">{provenance?.registry_tier ?? "—"}</code>
            ),
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
        onOpenChange={setProvenanceOpen}
        open={provenanceOpen}
      />
      <RemoveExtensionDialog
        activeBundles={bundles.data}
        dependencyError={bundles.error}
        dependencyLoading={bundles.isLoading}
        extension={extension}
        onOpenChange={setRemoveOpen}
        onRemoved={() =>
          void navigate({ search: { tab: "installed" }, to: "/marketplace/extensions" })
        }
        onRetryDependencies={() => void bundles.refetch()}
        open={removeOpen}
      />
    </>
  );
}

export { MarketplaceDetailExtensionManage };
export type { MarketplaceDetailExtensionManageProps };
