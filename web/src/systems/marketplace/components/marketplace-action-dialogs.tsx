import { ExtensionGatewayConfirmDialog, type InstalledExtensionView } from "@/systems/extensions";

import { MCPAuthorizeDialog, type useMCPDefinitionAuthorization } from "@/systems/settings";

import type { MarketplaceCatalogListing } from "../types";
import { ExtensionInstallSummaryDialog } from "./extension-install-summary-dialog";
import { ExtensionTrustDialog } from "./extension-trust-dialog";
import type { ExtensionInputDraft } from "./extension-install-model";
import type { MarketplaceInstallPreview } from "./marketplace-action-controller-logic";
import type { useMarketplaceUpdateRecovery } from "./use-marketplace-update-recovery";

interface MarketplaceActionDialogsProps {
  authorization: ReturnType<typeof useMCPDefinitionAuthorization>;
  /** Installed-row update waiting for unverified consent. */
  installedTrust: {
    item: InstalledExtensionView;
    pending: boolean;
    onConfirm: () => void;
    onDismiss: () => void;
  } | null;
  /** Catalog entry waiting for unverified consent, with its last failure. */
  trust: {
    entry: MarketplaceCatalogListing;
    error: string | null;
    pending: boolean;
    onConfirm: () => void;
    onDismiss: () => void;
  } | null;
  updateFlow: ReturnType<typeof useMarketplaceUpdateRecovery>;
  /** Install review, open only when a gate has something to decide. */
  install: {
    selected: MarketplaceInstallPreview;
    pending: boolean;
    onConfirm: (draft?: ExtensionInputDraft) => void;
    onDismiss: () => void;
  } | null;
}

/** Page-owned sign-in, consent, gateway and review dialogs, independent of catalog row lifetime. */
function MarketplaceActionDialogs({
  authorization,
  installedTrust,
  trust,
  updateFlow,
  install,
}: MarketplaceActionDialogsProps) {
  return (
    <>
      <MCPAuthorizeDialog
        authorize={authorization.authorize}
        scope={authorization.scope}
        server={authorization.server}
      />
      {installedTrust ? (
        <ExtensionTrustDialog
          action="update"
          name={installedTrust.item.extension.name}
          onConfirm={installedTrust.onConfirm}
          onOpenChange={open => {
            if (!open) installedTrust.onDismiss();
          }}
          open
          pending={installedTrust.pending}
          warnings={installedTrust.item.extension.trust?.warnings}
        />
      ) : null}
      {trust ? (
        <ExtensionTrustDialog
          action={trust.entry.update_available ? "update" : "install"}
          error={trust.error}
          name={trust.entry.name}
          onConfirm={trust.onConfirm}
          onOpenChange={open => {
            if (!open) trust.onDismiss();
          }}
          open
          pending={trust.pending}
          warnings={trust.entry.trust?.warnings}
        />
      ) : null}
      {updateFlow.recovery?.kind === "gateway" ? (
        <ExtensionGatewayConfirmDialog
          digest={updateFlow.recovery.digest}
          extensionName={updateFlow.recovery.label}
          onConfirm={() => updateFlow.confirm()}
          onOpenChange={open => {
            if (!open) updateFlow.dismiss();
          }}
          open
          pending={updateFlow.pending}
        />
      ) : null}
      {updateFlow.recovery?.kind === "inputs" ? (
        <ExtensionInstallSummaryDialog
          action="update"
          name={updateFlow.recovery.label}
          definitions={updateFlow.recovery.definitions}
          key={JSON.stringify(updateFlow.recovery.definitions)}
          onConfirm={updateFlow.confirm}
          onOpenChange={open => {
            if (!open) updateFlow.dismiss();
          }}
          open
          pending={updateFlow.pending}
        />
      ) : null}
      {install ? (
        <ExtensionInstallSummaryDialog
          action="install"
          key={JSON.stringify([install.selected.request, install.selected.preview.inputs])}
          onConfirm={install.onConfirm}
          onOpenChange={open => {
            if (!open) install.onDismiss();
          }}
          open
          pending={install.pending}
          preview={install.selected.preview}
          entry={install.selected.entry}
          destination={install.selected.request}
        />
      ) : null}
    </>
  );
}

export { MarketplaceActionDialogs };
