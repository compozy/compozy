import { Button } from "@compozy/ui";

import {
  authorizeLabel,
  deriveMCPAuthFilter,
  MCPSignInPopover,
  useMCPDefinitionAuthorization,
  type SettingsMCPServerEntry,
} from "@/systems/settings";

import { useMarketplaceExtensionMCPServer } from "../hooks/use-marketplace-detail-mcp-server";
import type { MarketplaceExtensionServer } from "../types";

interface ExtensionSetupSignInHintProps {
  /** Logical names of the extension's OAuth servers (`mcp_auth_servers`). */
  serverNames: readonly string[];
  servers: readonly MarketplaceExtensionServer[];
}

/**
 * Setup does not sign in itself (D5): it lists the OAuth servers and offers the same Authorize
 * action as the Server card when the live server reports one.
 */
export function ExtensionSetupSignInHint({ serverNames, servers }: ExtensionSetupSignInHintProps) {
  const authorization = useMCPDefinitionAuthorization();
  if (serverNames.length === 0) return null;
  return (
    <div className="flex flex-col gap-2" data-testid="extension-setup-sign-in">
      <p className="text-form-hint text-muted-foreground">
        These servers also need you to sign in.
      </p>
      {serverNames.map(name => (
        <ExtensionSetupSignInRow
          key={name}
          name={name}
          onAuthorize={(entry, trigger) => {
            const filter = deriveMCPAuthFilter(entry);
            if (filter) authorization.authorize.requestAuthorize(filter, entry, trigger);
          }}
          server={servers.find(server => server.name === name)}
        />
      ))}
      <MCPSignInPopover authorize={authorization.authorize} server={authorization.server} />
    </div>
  );
}

function ExtensionSetupSignInRow({
  name,
  onAuthorize,
  server,
}: {
  name: string;
  onAuthorize: (entry: SettingsMCPServerEntry, trigger: HTMLElement) => void;
  server: MarketplaceExtensionServer | undefined;
}) {
  const live = useMarketplaceExtensionMCPServer(server);
  const entry = live.server;
  const label = entry ? authorizeLabel(entry) : null;
  return (
    <div className="flex items-center justify-between gap-3">
      <code className="font-mono text-eyebrow text-fg">{name}</code>
      {entry && label ? (
        <Button
          data-testid={`extension-setup-authorize-${name}`}
          onClick={event => onAuthorize(entry, event.currentTarget)}
          size="sm"
          type="button"
          variant="neutral"
        >
          {label}
        </Button>
      ) : null}
    </div>
  );
}
