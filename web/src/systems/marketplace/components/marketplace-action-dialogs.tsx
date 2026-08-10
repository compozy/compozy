import type { ComponentProps } from "react";

import {
  MCPAuthorizeDialog,
  useMCPAuthorize,
  type SettingsMCPServerEntry,
} from "@/systems/settings";

import type { MarketplaceEntryResponse, MarketplaceListing } from "../types";
import { ExtensionTrustDialog } from "./extension-trust-dialog";
import { MCPInstallDialog } from "./mcp-install-dialog";

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
          workspaceId={workspaceId}
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
