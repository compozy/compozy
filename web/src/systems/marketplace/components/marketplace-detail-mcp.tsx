import {
  Activity,
  Cable,
  FileText,
  Fingerprint,
  KeyRound,
  SlidersHorizontal,
  TriangleAlert,
  Wrench,
} from "lucide-react";

import { cn, Empty, Eyebrow, Pill, PropertyRow } from "@compozy/ui";

import { useMarketplaceDetailMCPServer } from "../hooks/use-marketplace-detail-mcp-server";
import type { MarketplaceEntryResponse } from "../types";
import { MarketplaceDetailManageFallbackCard } from "./marketplace-detail-manage-state";
import {
  MarketplaceDetailColumns,
  MarketplaceDetailKvRow,
  MarketplaceDetailRailCard,
  MarketplaceDetailSection,
} from "./marketplace-detail-shell";
import { MarketplaceDetailWarnings } from "./marketplace-detail-warnings";
import {
  formatMarketplaceMCPLaunch,
  formatMarketplaceVersion,
  stripMarketplaceUrlScheme,
} from "./marketplace-ui";
import {
  composeMCPRowStatus,
  probeToolLabel,
  type SettingsMCPServerEntry,
  type SettingsLayeredScope,
} from "@/systems/settings";

type MCPBlock = NonNullable<MarketplaceEntryResponse["mcp"]>;

interface MarketplaceDetailMCPViewProps {
  data: MarketplaceEntryResponse;
  scope?: SettingsLayeredScope;
  workspaceId?: string;
  profileName?: string;
  liveDataEnabled?: boolean;
}

/**
 * MCP server detail: authorization and connection are the story; the tools
 * section reports only what the daemon's probe actually discovered.
 */
function MarketplaceDetailMCPView({
  data,
  scope,
  workspaceId,
  profileName,
  liveDataEnabled = true,
}: MarketplaceDetailMCPViewProps) {
  const { query, queryEnabled, server } = useMarketplaceDetailMCPServer(
    data.entry,
    scope,
    workspaceId,
    profileName,
    liveDataEnabled
  );
  const mcp = data.mcp;
  const installedUnresolved = data.entry.installed && !server;
  return (
    <MarketplaceDetailColumns
      main={
        <>
          <MarketplaceMCPAuthorizationSection mcp={mcp} server={server} />
          {mcp ? <MarketplaceMCPConnectionSection mcp={mcp} server={server} /> : null}
          <MarketplaceMCPToolsSection installed={data.entry.installed} server={server} />
          <MarketplaceDetailWarnings warnings={data.entry.trust?.warnings} />
        </>
      }
      rail={
        <>
          {installedUnresolved ? (
            queryEnabled ? (
              <MarketplaceDetailManageFallbackCard
                error={query.error ?? null}
                isLoading={query.isLoading}
                label="MCP"
                onRetry={() => void query.refetch()}
                testId={
                  query.isLoading
                    ? "marketplace-mcp-manage-loading"
                    : "marketplace-mcp-manage-error"
                }
              />
            ) : (
              <MarketplaceDetailRailCard icon={SlidersHorizontal} title="Manage">
                <p
                  className="px-3.5 py-1.5 text-small-body text-muted"
                  data-testid="marketplace-mcp-manage-workspace-required"
                >
                  Select a workspace to manage this server.
                </p>
              </MarketplaceDetailRailCard>
            )
          ) : null}
          {server ? <MarketplaceMCPStatusCard server={server} /> : null}
          <MarketplaceMCPDetailsCard data={data} server={server} />
          {server?.auth ? <MarketplaceMCPRegistrationCard server={server} /> : null}
        </>
      }
    />
  );
}

function mcpNeedsAuthorization(server: SettingsMCPServerEntry | undefined): boolean {
  if (!server?.auth_status) return false;
  return server.auth_status.status !== "authenticated";
}

function MarketplaceMCPAuthorizationSection({
  mcp,
  server,
}: {
  mcp: MarketplaceEntryResponse["mcp"];
  server: SettingsMCPServerEntry | undefined;
}) {
  const catalogAuth = mcp?.auth ?? null;
  const serverAuth = server?.auth ?? null;
  if (!catalogAuth && !serverAuth) return null;
  const needsAuth = mcpNeedsAuthorization(server);
  // The banner may only promise the Authorize button the topbar actually renders.
  const authorizeOffered = server ? composeMCPRowStatus(server).authorizeLabel !== null : false;
  const registration = serverAuth?.registration ?? catalogAuth?.registration;
  const automatic = registration === "auto" || registration === "automatic";
  const scopes = serverAuth?.scopes?.length ? serverAuth.scopes : (catalogAuth?.scopes ?? []);
  const tokenPresent = server?.auth_status?.token_present;

  return (
    <MarketplaceDetailSection
      data-testid="marketplace-mcp-authorization"
      icon={KeyRound}
      summary={needsAuth ? "action needed" : "oauth"}
      title="Authorization"
    >
      {needsAuth && authorizeOffered ? (
        <div className="px-3.5 pt-3">
          <div className="flex items-start gap-2.5 rounded-md bg-warning-tint px-3.5 py-3">
            <TriangleAlert aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-warning" />
            <div className="min-w-0">
              <p className="text-small-body font-semibold text-fg-strong">
                This server can't serve tools yet
              </p>
              <p className="mt-0.5 text-form-label leading-relaxed text-muted">
                It's installed and configured, but tools stay unavailable until you authorize. Use
                the Authorize button above — you'll be sent to the provider and back.
              </p>
            </div>
          </div>
        </div>
      ) : null}
      <MarketplaceDetailKvRow
        label="Method"
        sub={
          automatic
            ? "CompozyOS registers itself as a client on first authorize — no client ID or secret to paste."
            : undefined
        }
      >
        {automatic ? "OAuth · automatic registration" : "OAuth"}
      </MarketplaceDetailKvRow>
      {scopes.length > 0 ? (
        <MarketplaceDetailKvRow label="Requested scopes">
          <span className="flex flex-wrap gap-1.5">
            {scopes.map(scopeName => (
              <Pill mono key={scopeName}>
                {scopeName}
              </Pill>
            ))}
          </span>
        </MarketplaceDetailKvRow>
      ) : null}
      {server?.auth_status ? (
        <MarketplaceDetailKvRow label="Token">
          {tokenPresent ? "present" : "not present"}
        </MarketplaceDetailKvRow>
      ) : null}
    </MarketplaceDetailSection>
  );
}

function MarketplaceMCPConnectionSection({
  mcp,
  server,
}: {
  mcp: MCPBlock;
  server: SettingsMCPServerEntry | undefined;
}) {
  const remote = mcp.launch.type === "remote";
  const docker = mcp.launch.type === "docker";
  const launchLine = formatMarketplaceMCPLaunch(mcp.launch);
  const inputs = mcp.inputs ?? [];
  const oauth = Boolean(mcp.auth ?? server?.auth);
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-mcp-connection"
      icon={Cable}
      summary={remote ? "hosted http" : docker ? "docker" : "local process"}
      title="Connection"
    >
      <MarketplaceDetailKvRow
        label="Transport"
        sub={
          remote
            ? "Runs on the provider's infrastructure. Nothing is installed or kept running on this machine."
            : docker
              ? "Runs as a container image pulled onto this machine."
              : "Launched on this machine and managed by CompozyOS."
        }
      >
        {remote ? "Hosted HTTP" : docker ? "Docker container" : "Local process"}
      </MarketplaceDetailKvRow>
      {launchLine ? (
        <MarketplaceDetailKvRow label={remote ? "Endpoint" : "Command"}>
          <code className="block min-w-0 overflow-x-auto rounded-xs border border-line-soft bg-input-fill px-1.5 py-0.5 font-mono text-form-hint whitespace-nowrap text-fg">
            {launchLine}
          </code>
        </MarketplaceDetailKvRow>
      ) : null}
      <MarketplaceDetailKvRow
        label="Default scope"
        sub={
          mcp.default_scope === "workspace"
            ? "Available to sessions in this workspace only. Install at global scope to share it everywhere."
            : "Available to sessions in every workspace."
        }
      >
        {mcp.default_scope}
      </MarketplaceDetailKvRow>
      <MarketplaceDetailKvRow label="Required configuration">
        {inputs.length === 0 ? (
          oauth ? (
            "None — authorize and go."
          ) : (
            "None."
          )
        ) : (
          <span className="flex flex-col gap-2">
            {inputs.map(field => (
              <span className="flex items-start justify-between gap-4" key={field.id}>
                <code className="font-mono text-form-hint text-fg">{field.id}</code>
                <span className="text-right text-form-hint text-muted">{field.prompt}</span>
              </span>
            ))}
          </span>
        )}
      </MarketplaceDetailKvRow>
    </MarketplaceDetailSection>
  );
}

function MarketplaceMCPToolsSection({
  installed,
  server,
}: {
  installed: boolean;
  server: SettingsMCPServerEntry | undefined;
}) {
  const runtime = server?.runtime_status;
  const probed = runtime?.probe === "succeeded";
  const toolLabel = probeToolLabel(runtime?.probe, runtime?.tool_count);
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-mcp-tools"
      icon={Wrench}
      summary={
        probed
          ? toolLabel
          : installed
            ? "discovered after authorization"
            : "discovered after install"
      }
      title="Tools"
    >
      {probed ? (
        <>
          <MarketplaceDetailKvRow label="Discovered tools" mono>
            {toolLabel}
          </MarketplaceDetailKvRow>
          {runtime?.protocol_version ? (
            <MarketplaceDetailKvRow label="Protocol" mono>
              {runtime.protocol_version}
            </MarketplaceDetailKvRow>
          ) : null}
        </>
      ) : (
        <Empty
          className="py-9"
          fill={false}
          icon={Wrench}
          title="No tools discovered yet"
          description={
            installed
              ? "CompozyOS probes the server after it can connect and lists the discovered tools here."
              : "CompozyOS probes the server after install and lists the discovered tools here."
          }
        />
      )}
    </MarketplaceDetailSection>
  );
}

function MarketplaceMCPStatusCard({ server }: { server: SettingsMCPServerEntry }) {
  const status = composeMCPRowStatus(server);
  const needsAuth = mcpNeedsAuthorization(server);
  const runtime = server.runtime_status;
  const toolLabel = probeToolLabel(runtime?.probe, runtime?.tool_count);
  const cells = [
    { key: "config", label: "Config", cell: status.config },
    { key: "auth", label: "Auth", cell: status.auth },
    { key: "runtime", label: "Runtime", cell: status.runtime },
    { key: "probe", label: "Probe", cell: status.probe },
  ];
  return (
    <MarketplaceDetailRailCard
      data-testid="marketplace-mcp-status"
      icon={Activity}
      summary={needsAuth ? "waiting on authorization" : status.runtime.label.toLowerCase()}
      title="Status"
    >
      <div className="grid grid-cols-2">
        {cells.map(({ key, label, cell }, index) => (
          <div
            className={cn(
              "flex flex-col items-start gap-1.5 px-3.5 py-2.5",
              index % 2 === 0 && "border-r border-line-soft",
              index >= 2 && "border-t border-line-soft"
            )}
            key={key}
          >
            <Eyebrow className="text-faint">{label}</Eyebrow>
            <Pill mono tone={cell.tone}>
              {cell.label.toLowerCase()}
            </Pill>
          </div>
        ))}
      </div>
      <div className="border-t border-line-soft px-3.5 pt-1">
        <PropertyRow label="Tools" mono>
          {runtime?.probe === "succeeded" ? (toolLabel ?? "—") : "—"}
        </PropertyRow>
        <PropertyRow label="Protocol" mono>
          {runtime?.protocol_version?.trim() || "—"}
        </PropertyRow>
        {status.runtime.code ? (
          <p
            className="pb-1 font-mono text-transcript-caption leading-relaxed break-words text-faint"
            data-testid="marketplace-mcp-runtime-code"
          >
            {status.runtime.code}
          </p>
        ) : null}
      </div>
    </MarketplaceDetailRailCard>
  );
}

function MarketplaceMCPDetailsCard({
  data,
  server,
}: {
  data: MarketplaceEntryResponse;
  server: SettingsMCPServerEntry | undefined;
}) {
  const entry = data.entry;
  const version = formatMarketplaceVersion(server?.catalog_version ?? entry.version);
  const transport = server?.transport ?? entry.transport;
  const scopeValue = server?.scope ?? data.mcp?.default_scope;
  return (
    <MarketplaceDetailRailCard
      icon={FileText}
      summary={[entry.source, transport].filter(Boolean).join(" · ")}
      title="Details"
    >
      <div className="px-3.5">
        <PropertyRow label="Entry ID" mono>
          {entry.entry_id}
        </PropertyRow>
        {transport ? (
          <PropertyRow label="Transport" mono>
            {transport}
          </PropertyRow>
        ) : null}
        <PropertyRow label="Source" mono>
          {entry.source}
        </PropertyRow>
        {scopeValue ? (
          <PropertyRow label="Scope">
            {server?.scope ? scopeValue : `${scopeValue} · default`}
          </PropertyRow>
        ) : null}
        {version ? (
          <PropertyRow label="Version" mono>
            {version}
          </PropertyRow>
        ) : null}
      </div>
    </MarketplaceDetailRailCard>
  );
}

function MarketplaceMCPRegistrationCard({ server }: { server: SettingsMCPServerEntry }) {
  const auth = server.auth;
  if (!auth) return null;
  const automatic = auth.registration === "auto" || auth.registration === "automatic";
  const refreshable = server.auth_status?.refreshable;
  return (
    <MarketplaceDetailRailCard
      defaultOpen={false}
      icon={Fingerprint}
      summary={automatic ? "automatic" : "manual"}
      title="Registration"
    >
      <div className="px-3.5">
        <PropertyRow label="Strategy">{automatic ? "automatic" : "manual"}</PropertyRow>
        {auth.issuer_url ? (
          <PropertyRow label="Issuer" mono valueTitle={auth.issuer_url}>
            {stripMarketplaceUrlScheme(auth.issuer_url)}
          </PropertyRow>
        ) : null}
        <PropertyRow label="Client ID" mono>
          {auth.client_id?.trim() || (automatic ? "issued at authorize" : "—")}
        </PropertyRow>
        <PropertyRow label="Refreshable">
          {refreshable === undefined || refreshable === null ? "—" : refreshable ? "yes" : "no"}
        </PropertyRow>
      </div>
    </MarketplaceDetailRailCard>
  );
}

export { MarketplaceDetailMCPView };
export type { MarketplaceDetailMCPViewProps };
