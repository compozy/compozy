import type { ComponentProps } from "react";

import {
  MCPAuthorizeDialog,
  useMCPAuthorize,
  type SettingsMCPServerEntry,
} from "@/systems/settings";

import type { MarketplaceEntryResponse, MarketplaceListing } from "../types";
import { BundleActivationDialog } from "./bundle-activation-dialog";
import { ExtensionTrustDialog } from "./extension-trust-dialog";
import { MCPInstallDialog } from "./mcp-install-dialog";

interface MarketplaceActionDialogsProps {
  authorize: ReturnType<typeof useMCPAuthorize>;
  authScope: "global" | "workspace";
  authServer: SettingsMCPServerEntry | null;
  bundleDetail: MarketplaceEntryResponse | null;
  mcpDetail: MarketplaceEntryResponse | null;
  onBundleClose: () => void;
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
  bundleDetail,
  mcpDetail,
  onBundleClose,
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
      {bundleDetail ? (
        <BundleActivationDialog
          data={bundleDetail}
          key={bundleDetail.entry.entry_id}
          onOpenChange={open => {
            if (!open) onBundleClose();
          }}
          open
          workspaceId={workspaceId}
        />
      ) : null}
      {trustEntry ? (
        <ExtensionTrustDialog
          entry={trustEntry}
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
