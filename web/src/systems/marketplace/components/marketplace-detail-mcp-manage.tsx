import { useEffect } from "react";

import { Button, Pill, Section } from "@agh/ui";

import { useMCPAuthorize } from "@/systems/settings";
import {
  authorizeLabel,
  composeMCPRowStatus,
  deriveMCPAuthFilter,
  MCPAuthorizeDialog,
  SETTINGS_QUERY_INTERVALS,
  useSettingsMCPServers,
  type SettingsMCPServerEntry,
} from "@/systems/settings";
import { useActiveWorkspace } from "@/systems/workspace";

import type { MarketplaceEntryResponse } from "../types";
import { MarketplaceDetailManageState } from "./marketplace-detail-manage-state";

interface MarketplaceDetailMCPManageProps {
  entry: MarketplaceEntryResponse["entry"];
  scope?: "global" | "workspace";
  workspaceId?: string;
}

function findInstalledMCPServer(
  entry: MarketplaceEntryResponse["entry"],
  servers: readonly SettingsMCPServerEntry[]
): SettingsMCPServerEntry | undefined {
  const installedName = entry.installed_name?.trim();
  if (installedName) {
    return servers.find(server => server.name === installedName);
  }
  return servers.find(
    server => server.catalog_entry === entry.entry_id || server.name === entry.name
  );
}

function MarketplaceDetailMCPManage({
  entry,
  scope,
  workspaceId,
}: MarketplaceDetailMCPManageProps) {
  const { activeWorkspaceId } = useActiveWorkspace();
  const resolvedWorkspaceId = workspaceId ?? activeWorkspaceId ?? undefined;
  const resolvedScope = scope ?? (resolvedWorkspaceId ? "workspace" : "global");
  const queryFilter =
    resolvedScope === "workspace"
      ? { scope: "workspace" as const, workspace_id: resolvedWorkspaceId }
      : { scope: "global" as const };
  const queryEnabled = resolvedScope === "global" || Boolean(resolvedWorkspaceId);
  const query = useSettingsMCPServers(queryFilter, {
    enabled: queryEnabled,
    refetchInterval: SETTINGS_QUERY_INTERVALS.collectionRefetchInterval,
  });
  const baseServer = findInstalledMCPServer(entry, query.data?.mcp_servers ?? []);

  const authFilter = baseServer ? deriveMCPAuthFilter(baseServer) : null;
  const authorize = useMCPAuthorize(authFilter);
  const { acknowledgeStatus, isAwaiting } = authorize;
  const authPollQuery = useSettingsMCPServers(queryFilter, {
    enabled: queryEnabled && isAwaiting,
    refetchInterval: SETTINGS_QUERY_INTERVALS.mcpAuthStatusPollInterval,
  });
  const server = findInstalledMCPServer(
    entry,
    authPollQuery.data?.mcp_servers ?? query.data?.mcp_servers ?? []
  );

  useEffect(() => {
    const status = server?.auth_status;
    if (isAwaiting && status?.status) {
      // react-doctor-disable-next-line react-doctor/no-pass-data-to-parent -- This consumes daemon auth-status updates from the scoped query and advances the local authorization controller; it does not pass child-owned data to a parent.
      acknowledgeStatus(status.status, Boolean(status.token_present));
    }
  }, [acknowledgeStatus, isAwaiting, server?.auth_status]);

  if (!server) {
    return (
      <MarketplaceDetailManageState
        error={query.error ?? null}
        isLoading={query.isLoading}
        label="MCP"
        onRetry={() => void query.refetch()}
        testId={query.isLoading ? "marketplace-mcp-manage-loading" : "marketplace-mcp-manage-error"}
      />
    );
  }

  const status = composeMCPRowStatus(server);
  const label = authorizeLabel(server);

  return (
    <>
      <Section label="Manage">
        <div className="flex flex-col gap-3 rounded-lg bg-canvas-soft px-4 py-3">
          <div className="flex flex-wrap items-center gap-1.5">
            <Pill mono tone={status.auth.tone}>
              {status.auth.label}
            </Pill>
            <Pill mono tone={status.runtime.tone}>
              {status.runtime.label}
            </Pill>
            <Pill mono tone={status.probe.tone}>
              {status.probe.label}
            </Pill>
          </div>
          {status.runtime.code ? (
            <p className="font-mono text-xs text-muted" data-testid="marketplace-mcp-runtime-code">
              {status.runtime.code}
            </p>
          ) : null}
          {label ? (
            <div>
              <Button
                data-testid="mcp-authorize-btn"
                disabled={!authFilter || authorize.isAwaiting}
                onClick={() => {
                  if (!authFilter) return;
                  void authorize.beginAuthorize(server.name, {
                    status: server.auth_status?.status ?? "needs_login",
                    tokenPresent: Boolean(server.auth_status?.token_present),
                  });
                }}
                size="sm"
                type="button"
                variant="outline"
              >
                {label}
              </Button>
            </div>
          ) : null}
        </div>
      </Section>
      <MCPAuthorizeDialog
        authorize={authorize}
        scope={authFilter?.scope ?? "global"}
        server={server}
      />
    </>
  );
}

export { MarketplaceDetailMCPManage };
export type { MarketplaceDetailMCPManageProps };
