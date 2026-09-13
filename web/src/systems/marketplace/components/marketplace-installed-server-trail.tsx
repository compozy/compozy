import { useState } from "react";

import { Button, DropdownMenuItem, Spinner } from "@compozy/ui";

import {
  deriveMCPAuthFilter,
  MCPAuthorizeDialog,
  MCPOverrideEditor,
  useMCPDefinitionAuthorization,
  useMCPOverrideEditor,
} from "@/systems/settings";

import { useMarketplaceExtensionMCPServer } from "../hooks/use-marketplace-detail-mcp-server";
import { installedDisplayName } from "../lib/marketplace-installed-view";
import {
  MarketplaceInstalledTrailControls,
  type MarketplaceInstalledTrailProps,
} from "./marketplace-installed-trail-controls";
import { installedExtensionServerStatus } from "./marketplace-server-status";
import { MarketplaceServerStatusWord } from "./marketplace-server-status-word";

/**
 * Installed trail for an extension that provides an MCP server: the daemon's status word
 * (Needs configuration wins, then the server needing authorization), Authorize targeting exactly
 * that owner-qualified definition, the allocated runtime name when it differs from the logical
 * name, and Edit configuration in the overflow. The live Settings definition is read only when a
 * control needs it — authorization pending, or the overflow opened — never for every row.
 */
function MarketplaceInstalledServerTrail(props: MarketplaceInstalledTrailProps) {
  const { item, pending = false, liveDataEnabled = true } = props;
  const { extension } = item;
  const [menuOpened, setMenuOpened] = useState(false);
  const resolved = installedExtensionServerStatus(extension);
  const target = resolved?.server ?? extension.mcp_servers[0];
  const published = Boolean(target?.runtime_name?.trim());
  const needsAuthorization = resolved?.status.key === "needs_authorization";
  const live = useMarketplaceExtensionMCPServer(
    target,
    liveDataEnabled && published && (needsAuthorization || menuOpened)
  );
  const authorization = useMCPDefinitionAuthorization();
  const override = useMCPOverrideEditor();
  const name = installedDisplayName(item);
  const runtimeName = target?.runtime_name?.trim();
  const allocated = runtimeName && target && runtimeName !== target.name ? runtimeName : null;
  const statusTestId =
    resolved?.status.key === "needs_configuration"
      ? `marketplace-installed-needs-configuration-${extension.name}`
      : `marketplace-installed-server-status-${extension.name}`;

  return (
    <>
      <MarketplaceInstalledTrailControls
        {...props}
        leading={
          <>
            {resolved ? (
              <MarketplaceServerStatusWord data-testid={statusTestId} view={resolved.status} />
            ) : null}
            {allocated ? (
              <span
                className="font-mono text-mono-id text-faint"
                data-testid={`marketplace-installed-runtime-name-${extension.name}`}
                title={`Runtime name ${allocated}`}
              >
                {allocated}
              </span>
            ) : null}
            {needsAuthorization && published && target ? (
              <Button
                aria-label={`Authorize ${target.name} for ${name}`}
                data-testid={`marketplace-installed-authorize-${extension.name}`}
                disabled={pending || !live.server}
                onClick={() => {
                  const entry = live.server;
                  const filter = entry ? deriveMCPAuthFilter(entry) : null;
                  if (entry && filter) authorization.authorize.requestAuthorize(filter, entry);
                }}
                size="sm"
                type="button"
                variant="neutral"
              >
                {live.query.isPending && !live.server ? (
                  <Spinner aria-hidden="true" className="size-3" />
                ) : null}
                Authorize
              </Button>
            ) : null}
          </>
        }
        menuItems={
          published ? (
            <DropdownMenuItem
              data-testid={`marketplace-installed-edit-server-${extension.name}`}
              disabled={!live.server}
              onClick={() => {
                if (live.server) override.openEdit(live.server);
              }}
            >
              Edit server configuration
            </DropdownMenuItem>
          ) : null
        }
        onMenuOpenChange={open => {
          if (open) setMenuOpened(true);
        }}
      />
      <MCPAuthorizeDialog
        authorize={authorization.authorize}
        scope={authorization.scope}
        server={authorization.server}
      />
      {override.editorProps ? <MCPOverrideEditor {...override.editorProps} /> : null}
    </>
  );
}

export { MarketplaceInstalledServerTrail };
