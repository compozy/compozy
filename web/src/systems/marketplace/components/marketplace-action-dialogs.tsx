import type { ComponentProps } from "react";

import type { MarketplaceEntryResponse, MarketplaceListing } from "../types";
import { ExtensionTrustDialog } from "./extension-trust-dialog";
import { MCPInstallDialog } from "./mcp-install-dialog";
import {
  MCPAuthorizeDialog,
  type SettingsMCPServerEntry,
  useMCPAuthorize,
} from "@/systems/settings";

interface MarketplaceActionDialogsProps {
  authorize: ReturnType<typeof useMCPAuthorize>;
  authScope: "global" | "workspace";
  authServer: SettingsMCPServerEntry | null;
  mcpDetail: MarketplaceEntryResponse | null;
  onConfirmTrust: () => void;
  onInstallMCP: ComponentProps<typeof MCPInstallDialog>["onInstall"];
  onMCPClose: () => void;
  onTrustClose: () => void;
  trustEntry: MarketplaceListing | null;
  trustError: string | null;
  trustPending: boolean;
  workspaceId?: string | null;
  scope?: "global" | "workspace";
  workspaceName?: string | null;
}

function MarketplaceActionDialogs({
  authorize,
  authScope,
  authServer,
  mcpDetail,
  onConfirmTrust,
  onInstallMCP,
  onMCPClose,
  onTrustClose,
  trustEntry,
  trustError,
  trustPending,
  workspaceId,
  scope,
  workspaceName,
}: MarketplaceActionDialogsProps) {
  return (
    <>
      {mcpDetail ? (
        <MCPInstallDialog
          data={mcpDetail}
          key={mcpDetail.entry.entry_id}
          onInstall={onInstallMCP}
          onOpenChange={open => {
            if (!open) onMCPClose();
          }}
          open
          scope={scope}
          workspaceId={workspaceId}
          workspaceName={workspaceName}
        />
      ) : null}
      {trustEntry ? (
        <ExtensionTrustDialog
          action={trustEntry.update_available ? "update" : "install"}
          name={trustEntry.name}
          warnings={trustEntry.trust?.warnings}
          error={trustError}
          onConfirm={onConfirmTrust}
          onOpenChange={open => {
            if (!open) onTrustClose();
          }}
          open
          pending={trustPending}
        />
      ) : null}
      <MCPAuthorizeDialog authorize={authorize} scope={authScope} server={authServer} />
    </>
  );
}

export { MarketplaceActionDialogs };
