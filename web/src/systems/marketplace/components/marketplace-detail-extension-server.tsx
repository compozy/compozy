import { Plug, Settings2 } from "lucide-react";

import { Button, PropertyRow } from "@compozy/ui";

import {
  authorizeLabel,
  deriveMCPAuthFilter,
  MCPAuthorizeDialog,
  MCPOverrideEditor,
  useMCPDefinitionAuthorization,
  useMCPOverrideEditor,
  type SettingsMCPServerEntry,
} from "@/systems/settings";

import { useMarketplaceExtensionMCPServer } from "../hooks/use-marketplace-detail-mcp-server";
import type { MarketplaceExtensionServer } from "../types";
import { MarketplaceDetailRailCard, MarketplaceDetailRailNote } from "./marketplace-detail-shell";
import {
  liveExtensionServerStatus,
  marketplaceServerStatus,
  type MarketplaceServerStatusView,
} from "./marketplace-server-status";
import { MarketplaceServerStatusWord } from "./marketplace-server-status-word";

interface MarketplaceServerInput {
  id: string;
  /** Known on the catalog declaration; the installed inventory reports presence instead. */
  required?: boolean;
}

interface MarketplaceExtensionServerSectionProps {
  servers: readonly MarketplaceExtensionServer[];
  /** The entry's declared inputs; the daemon binds them to the servers at publication. */
  inputs: readonly MarketplaceServerInput[];
  /** Installed only: inputs the daemon reports unresolved (`missing_inputs`). */
  missingInputs?: readonly string[];
  /** Installed entries read live status and gain Authorize / Edit configuration. */
  installed?: boolean;
  liveDataEnabled?: boolean;
}

/**
 * The detail's Server section: one rail card per declared server (stable identity = owner +
 * logical name), with the facts the retired MCP detail showed — Launch · Auth · Inputs · Scope ·
 * Owner — and, once installed, the truthful status, the allocated runtime name and the owner-
 * qualified Authorize / Edit configuration actions. Absent entirely when the entry has no server.
 */
function MarketplaceExtensionServerSection({
  servers,
  inputs,
  missingInputs = [],
  installed = false,
  liveDataEnabled = true,
}: MarketplaceExtensionServerSectionProps) {
  if (servers.length === 0) return null;
  if (!installed) {
    return (
      <>
        {servers.map(server => (
          <MarketplaceExtensionServerSummaryCard
            inputs={inputs}
            key={serverIdentity(server)}
            server={server}
          />
        ))}
      </>
    );
  }
  return (
    <MarketplaceExtensionServerInstalledCards
      inputs={inputs}
      liveDataEnabled={liveDataEnabled}
      missingInputs={missingInputs}
      servers={servers}
    />
  );
}

/** Installed: one authorize flow and one override editor shared by every server card. */
function MarketplaceExtensionServerInstalledCards({
  servers,
  inputs,
  missingInputs,
  liveDataEnabled,
}: {
  servers: readonly MarketplaceExtensionServer[];
  inputs: readonly MarketplaceServerInput[];
  missingInputs: readonly string[];
  liveDataEnabled: boolean;
}) {
  const authorization = useMCPDefinitionAuthorization();
  const override = useMCPOverrideEditor();
  return (
    <>
      {servers.map(server => (
        <MarketplaceExtensionServerLiveCard
          inputs={inputs}
          key={serverIdentity(server)}
          liveDataEnabled={liveDataEnabled}
          missingInputs={missingInputs}
          onAuthorize={entry => {
            const filter = deriveMCPAuthFilter(entry);
            if (filter) authorization.authorize.requestAuthorize(filter, entry);
          }}
          onEdit={override.openEdit}
          server={server}
        />
      ))}
      <MCPAuthorizeDialog
        authorize={authorization.authorize}
        scope={authorization.scope}
        server={authorization.server}
      />
      {override.editorProps ? <MCPOverrideEditor {...override.editorProps} /> : null}
    </>
  );
}

function MarketplaceExtensionServerSummaryCard({
  server,
  inputs,
}: {
  server: MarketplaceExtensionServer;
  inputs: readonly MarketplaceServerInput[];
}) {
  return (
    <MarketplaceDetailRailCard
      data-testid={`marketplace-extension-server-${server.name}`}
      icon={Plug}
      summary={server.name}
      title="Server"
    >
      <div className="px-3.5">
        <MarketplaceServerFactRows inputs={inputs} server={server} />
      </div>
    </MarketplaceDetailRailCard>
  );
}

function MarketplaceExtensionServerLiveCard({
  server,
  inputs,
  missingInputs,
  liveDataEnabled,
  onAuthorize,
  onEdit,
}: {
  server: MarketplaceExtensionServer;
  inputs: readonly MarketplaceServerInput[];
  missingInputs: readonly string[];
  liveDataEnabled: boolean;
  onAuthorize: (entry: SettingsMCPServerEntry) => void;
  onEdit: (entry: SettingsMCPServerEntry) => void;
}) {
  const live = useMarketplaceExtensionMCPServer(server, liveDataEnabled);
  const status: MarketplaceServerStatusView | null =
    missingInputs.length > 0
      ? marketplaceServerStatus("needs_configuration")
      : ((live.server ? liveExtensionServerStatus(live.server) : null) ??
        marketplaceServerStatus(server.status));
  return (
    <MarketplaceDetailRailCard
      data-testid={`marketplace-extension-server-${server.name}`}
      icon={Plug}
      summary={status?.label ?? server.name}
      title="Server"
    >
      <div className="px-3.5">
        {status ? (
          <PropertyRow
            editor={
              <MarketplaceServerStatusWord
                data-testid={`marketplace-extension-server-status-${server.name}`}
                view={status}
              />
            }
            label="Status"
          />
        ) : null}
        <MarketplaceServerFactRows
          inputs={inputs}
          missingInputs={missingInputs}
          server={server}
          showRuntimeName
        />
      </div>
      <MarketplaceExtensionServerActions
        server={server}
        live={live}
        missingInputs={missingInputs}
        onAuthorize={onAuthorize}
        onEdit={onEdit}
      />
    </MarketplaceDetailRailCard>
  );
}

/** Launch · Auth · Inputs · Scope · Owner (· runtime name). Every value is a payload field. */
function MarketplaceServerFactRows({
  server,
  inputs,
  missingInputs = [],
  showRuntimeName = false,
}: {
  server: MarketplaceExtensionServer;
  inputs: readonly MarketplaceServerInput[];
  missingInputs?: readonly string[];
  showRuntimeName?: boolean;
}) {
  const launch = [server.transport, server.launch].filter(value => value?.trim()).join(" · ");
  const auth = formatServerAuth(server.auth);
  const scope = formatServerScope(server);
  const missing = missingInputs.length;
  return (
    <>
      <PropertyRow label="Launch" mono valueTitle={launch || undefined}>
        {launch || "—"}
      </PropertyRow>
      <PropertyRow label="Auth" valueTitle={auth.title}>
        {auth.label}
      </PropertyRow>
      <PropertyRow
        label="Inputs"
        valueTitle={inputs.length ? inputs.map(input => input.id).join(", ") : undefined}
      >
        {inputs.length === 0 ? (
          "none"
        ) : (
          <span className="inline-flex min-w-0 items-center gap-1.5">
            <span className="truncate">{formatInputsCount(inputs)}</span>
            {missing > 0 ? (
              <span
                className="text-eyebrow font-medium text-warning"
                data-testid={`marketplace-extension-server-missing-inputs-${server.name}`}
              >
                {missing} missing
              </span>
            ) : null}
          </span>
        )}
      </PropertyRow>
      {scope ? <PropertyRow label="Scope">{scope}</PropertyRow> : null}
      {server.owner?.trim() ? (
        <PropertyRow label="Owner" mono>
          {server.owner}
        </PropertyRow>
      ) : null}
      {showRuntimeName && server.runtime_name?.trim() ? (
        <PropertyRow label="Runtime name" mono>
          <span data-testid={`marketplace-extension-server-runtime-name-${server.name}`}>
            {server.runtime_name}
          </span>
        </PropertyRow>
      ) : null}
    </>
  );
}

function serverIdentity(server: MarketplaceExtensionServer): string {
  return `${server.owner}:${server.name}`;
}

function formatInputsCount(inputs: readonly MarketplaceServerInput[]): string {
  const total = inputs.length;
  const required = inputs.filter(input => input.required === true).length;
  const noun = total === 1 ? "input" : "inputs";
  if (required === 0) return `${total} ${noun}`;
  return required === total
    ? `${total} ${noun} · required`
    : `${total} ${noun} · ${required} required`;
}

function formatServerAuth(auth: MarketplaceExtensionServer["auth"]): {
  label: string;
  title?: string;
} {
  if (!auth || auth.method === "none") return { label: "None" };
  const parts = [auth.method === "oauth" ? "OAuth" : auth.method];
  if (auth.issuer_url?.trim()) parts.push(issuerHost(auth.issuer_url));
  const scopes = auth.scopes?.filter(scope => scope.trim()) ?? [];
  if (scopes.length) parts.push(`scopes ${scopes.join(", ")}`);
  const label = parts.join(" · ");
  const registration = auth.registration?.trim();
  return { label, title: registration ? `${label} · ${registration} registration` : label };
}

function issuerHost(issuerUrl: string): string {
  try {
    return new URL(issuerUrl).host || issuerUrl;
  } catch {
    return issuerUrl;
  }
}

function formatServerScope(server: MarketplaceExtensionServer): string | null {
  const profile = server.profile?.trim();
  const profileWord = profile && profile !== "default" ? ` · profile ${profile}` : "";
  if (server.scope === "workspace" && server.workspace_id?.trim()) {
    return `Workspace · ${server.workspace_id}${profileWord}`;
  }
  if (server.scope === "global") return `Everywhere (global)${profileWord}`;
  return server.scope?.trim() ? `${server.scope}${profileWord}` : null;
}

export { MarketplaceExtensionServerSection };
export type { MarketplaceExtensionServerSectionProps };

function MarketplaceExtensionServerActions({
  server,
  live,
  missingInputs,
  onAuthorize,
  onEdit,
}: {
  server: MarketplaceExtensionServer;
  live: ReturnType<typeof useMarketplaceExtensionMCPServer>;
  missingInputs: readonly string[];
  onAuthorize: (entry: SettingsMCPServerEntry) => void;
  onEdit: (entry: SettingsMCPServerEntry) => void;
}) {
  const entry = live.server;
  if (!server.runtime_name?.trim())
    return (
      <MarketplaceDetailRailNote>
        Not published yet — no runtime name has been allocated.
      </MarketplaceDetailRailNote>
    );
  const repair = missingInputs.length === 0 && entry ? authorizeLabel(entry) : null;
  return (
    <>
      {entry ? (
        <div
          className="flex flex-wrap items-center gap-1.5 px-3.5 pt-1.5 pb-1"
          data-testid={`marketplace-extension-server-actions-${server.name}`}
        >
          {repair ? (
            <Button
              data-testid={`marketplace-extension-server-authorize-${server.name}`}
              onClick={() => onAuthorize(entry)}
              size="sm"
              type="button"
              variant="neutral"
            >
              {repair}
            </Button>
          ) : null}
          <Button
            data-testid={`marketplace-extension-server-edit-${server.name}`}
            onClick={() => onEdit(entry)}
            size="sm"
            type="button"
            variant="ghost"
          >
            <Settings2 aria-hidden="true" className="size-3" />
            Edit configuration
          </Button>
        </div>
      ) : live.query.error ? (
        <div className="flex items-center gap-2 px-3.5 pt-1.5 pb-1">
          <span className="text-transcript-caption text-faint">Live status unavailable.</span>
          <Button onClick={() => void live.query.refetch()} size="xs" type="button" variant="ghost">
            Retry
          </Button>
        </div>
      ) : null}{" "}
    </>
  );
}
