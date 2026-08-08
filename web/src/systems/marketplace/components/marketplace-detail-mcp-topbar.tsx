import { KeyRound } from "lucide-react";
import { useEffect } from "react";

import { Button, Pill } from "@compozy/ui";

import {
  composeMCPRowStatus,
  deriveMCPAuthFilter,
  isMCPAuthorizeAwaiting,
  MCPAuthorizeDialog,
  SETTINGS_QUERY_INTERVALS,
  useMCPAuthorize,
  useSettingsMCPServers,
} from "@/systems/settings";

import { useMarketplaceDetailMCPServer } from "../hooks/use-marketplace-detail-mcp-server";
import type { MarketplaceEntryResponse, MarketplaceListing } from "../types";
import { MarketplaceEntryAction, MarketplaceEntryStatus } from "./marketplace-entry-actions";

interface MarketplaceMCPDetailTopbarActionsProps {
  entry: MarketplaceEntryResponse["entry"];
  scope?: "global" | "workspace";
  workspaceId?: string;
  onAction: (entry: MarketplaceListing) => void;
  pending: boolean;
}

/**
 * OS-head actions for an installed MCP entry. While the daemon reports the
 * server as not authenticated, the head carries the truthful auth pill and the
 * one repair action; once authorized it falls back to the standard entry
 * status + action pair.
 */
function MarketplaceMCPDetailTopbarActions({
  entry,
  scope,
  workspaceId,
  onAction,
  pending,
}: MarketplaceMCPDetailTopbarActionsProps) {
  const { server, queryEnabled, queryFilter } = useMarketplaceDetailMCPServer(
    entry,
    scope,
    workspaceId
  );
  const authorize = useMCPAuthorize();
  const { acknowledgeStatus } = authorize;
  const awaitingAuthorization = isMCPAuthorizeAwaiting(authorize.phase);
  const authPollQuery = useSettingsMCPServers(queryFilter, {
    enabled: queryEnabled && awaitingAuthorization,
    refetchInterval: SETTINGS_QUERY_INTERVALS.mcpAuthStatusPollInterval,
  });
  const polledServer =
    authPollQuery.data?.mcp_servers.find(candidate => candidate.name === server?.name) ?? server;

  useEffect(() => {
    const status = polledServer?.auth_status;
    if (awaitingAuthorization && status?.status) {
      // react-doctor-disable-next-line react-doctor/no-pass-data-to-parent -- This consumes daemon auth-status updates from the scoped query and advances the local authorization controller; it does not pass child-owned data to a parent.
      acknowledgeStatus(status.status, Boolean(status.token_present));
    }
  }, [acknowledgeStatus, awaitingAuthorization, polledServer?.auth_status]);

  if (!polledServer) {
    return (
      <>
        <MarketplaceEntryStatus entry={entry} />
        <MarketplaceEntryAction
          emphasis="primary"
          entry={entry}
          onAction={onAction}
          pending={pending}
        />
      </>
    );
  }

  const status = composeMCPRowStatus(polledServer);
  const label = status.authorizeLabel;
  const authFilter = deriveMCPAuthFilter(polledServer);

  if (!label) {
    return (
      <>
        <MarketplaceEntryStatus entry={entry} />
        <MarketplaceEntryAction
          emphasis="primary"
          entry={entry}
          onAction={onAction}
          pending={pending}
        />
        <MCPAuthorizeDialog
          authorize={authorize}
          scope={authFilter?.scope ?? "global"}
          server={polledServer}
        />
      </>
    );
  }

  return (
    <>
      <Pill mono tone={status.auth.tone}>
        {polledServer.auth_status?.status === "needs_login"
          ? "needs authorization"
          : status.auth.label.toLowerCase()}
      </Pill>
      <Button
        data-testid="mcp-authorize-btn"
        disabled={!authFilter || awaitingAuthorization}
        onClick={() => {
          if (!authFilter) return;
          authorize.requestAuthorize(authFilter, polledServer);
        }}
        size="sm"
        type="button"
        variant="default"
      >
        <KeyRound aria-hidden="true" className="size-3.5" />
        {label}
      </Button>
      <MCPAuthorizeDialog
        authorize={authorize}
        scope={authFilter?.scope ?? "global"}
        server={polledServer}
      />
    </>
  );
}

export { MarketplaceMCPDetailTopbarActions };
export type { MarketplaceMCPDetailTopbarActionsProps };
