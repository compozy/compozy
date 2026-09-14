import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useSelector } from "@xstate/store-react";
import { useRef, useState } from "react";
import { toast } from "sonner";

import {
  extensionTrustFacts,
  extensionUpdateScope,
  extensionsListOptions,
  ExtensionNetworkConfirmDialog,
  previewExtensionInstall,
  useExtensionInstanceScope,
  type InstalledExtensionView,
  type ExtensionInstanceScope,
  useToggleExtension,
} from "@/systems/extensions";

import {
  useInstallMarketplaceExtension,
  useUpdateMarketplaceExtension,
} from "../hooks/use-marketplace-actions";
import type { ExtensionInstallRequest, MarketplaceCatalogListing } from "../types";
import { marketplaceCatalogEntryOptions } from "../lib/query-options";
import { marketplaceOriginKey } from "../lib/marketplace-installed-view";
import { ExtensionInstallSummaryDialog } from "./extension-install-summary-dialog";
import { ExtensionTrustDialog } from "./extension-trust-dialog";
import { useStoreBinding } from "@/hooks/use-store-binding";
import {
  marketplaceActionControllerLogic,
  marketplaceInstallPreviewLogic,
} from "./marketplace-action-controller-logic";
import {
  formatMarketplaceVersion,
  marketplaceEntrySlug,
  marketplaceErrorMessage,
  marketplaceErrorCode,
} from "./marketplace-ui";
import { useMarketplaceUpdateRecovery } from "./use-marketplace-update-recovery";
import { prepareExtensionInputs, type ExtensionInputDraft } from "./extension-install-model";
import { useMarketplacePending } from "./use-marketplace-pending";

interface MarketplaceActionController {
  dialogs: React.ReactNode;
  /** Catalog row Install: direct for a verified entry, through the trust dialog when unverified. */
  install: (entry: MarketplaceCatalogListing) => void;
  /** Catalog row Update, addressed by the daemon-joined installed name. */
  update: (entry: MarketplaceCatalogListing) => void;
  /** Installed row Update, addressed by the local installed name. */
  updateInstalled: (item: InstalledExtensionView) => void;
  toggleEnabled: (item: InstalledExtensionView, enabled: boolean) => void;
  endEntryFlash: (entry: MarketplaceCatalogListing) => void;
  endItemFlash: (item: InstalledExtensionView) => void;
  flashItem: (item: InstalledExtensionView) => void;
  isEntryFlashing: (entry: MarketplaceCatalogListing) => boolean;
  isEntryPending: (entry: MarketplaceCatalogListing) => boolean;
  isItemFlashing: (item: InstalledExtensionView) => boolean;
  isItemPending: (item: InstalledExtensionView) => boolean;
  trackInstalledItems: <T>(
    items: readonly InstalledExtensionView[],
    action: () => Promise<T>
  ) => Promise<T>;
}

function installedName(entry: MarketplaceCatalogListing): string {
  if (!entry.installed_name) {
    throw new Error(`Installed identity is unavailable for ${entry.name}`);
  }
  return entry.installed_name;
}

/** Use the listed source and digest for this catalog acquisition. */
function catalogInstallRequest(
  entry: MarketplaceCatalogListing,
  allowUnverified: boolean,
  destination: ReturnType<typeof useExtensionInstanceScope>
): ExtensionInstallRequest {
  return {
    profile: destination.profileName,
    scope: destination.workspaceId ? "workspace" : "global",
    ...(destination.workspaceId ? { workspace_id: destination.workspaceId } : {}),
    allow_unverified: allowUnverified,
    expected_digest: entry.digest_sha256,
    ref: marketplaceEntrySlug(entry),
    source: entry.source_ref === "catalog:compozy" ? "curated" : "marketplace",
    version: entry.version,
  };
}

function updatedToast(name: string, version: string | null | undefined) {
  const display = formatMarketplaceVersion(version);
  toast.success(display ? `${name} updated to ${display}` : `${name} updated`);
}

/**
 * Install/update/enable for the one-kind catalog. Every mutation keeps its daemon gates — install
 * preview, unverified consent, network confirmation — and rows report pending by origin or by the
 * local installed name, never by display name.
 */
function useMarketplaceActionController(
  scope: ExtensionInstanceScope = {}
): MarketplaceActionController {
  const activeScope = useExtensionInstanceScope();
  const destination = {
    profileName: scope.profileName ?? activeScope.profileName,
    workspaceId: scope.workspaceId === undefined ? activeScope.workspaceId : scope.workspaceId,
  };
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const entryActions = useRef(new Set<string>());
  const installExtension = useInstallMarketplaceExtension();
  const updateExtension = useUpdateMarketplaceExtension();
  const toggleExtension = useToggleExtension();
  const pending = useMarketplacePending();
  const updateFlow = useMarketplaceUpdateRecovery(updateExtension.mutateAsync, updatedToast);
  const { runUpdate } = updateFlow;
  const { store: acquisition } = useStoreBinding(
    JSON.stringify([destination.profileName, destination.workspaceId]),
    () => ({
      trust: marketplaceActionControllerLogic.createStore(),
      preview: marketplaceInstallPreviewLogic.createStore(),
    })
  );
  const installPreview = useSelector(acquisition.preview, snapshot => snapshot.context.selected);
  const [installedTrust, setInstalledTrust] = useState<InstalledExtensionView | null>(null);
  const store = acquisition.trust;
  const phase = useSelector(store, snapshot => snapshot.context);
  const trustEntry =
    phase.status === "extensionTrust" || phase.status === "extensionTrustSubmitting"
      ? phase.entry
      : null;

  const viewInstalledToast = (message: string, needsAuthorization: boolean) => {
    toast.success(message, {
      ...(needsAuthorization ? { description: "Needs authorization before it can run" } : {}),
      action: {
        label: "View installed →",
        onClick: () => void navigate({ search: {}, to: "/marketplace/installed" }),
      },
    });
  };

  const withPendingEntry = async (
    entry: MarketplaceCatalogListing,
    action: () => Promise<void>
  ) => {
    const key = marketplaceOriginKey(entry);
    if (entryActions.current.has(key)) return;
    entryActions.current.add(key);
    await pending
      .trackEntry(entry, action)
      .catch(error => reportFailure(error, `Failed to update ${entry.name}`))
      .finally(() => entryActions.current.delete(key));
  };

  const withPendingItem = async (item: InstalledExtensionView, action: () => Promise<void>) => {
    try {
      await pending.trackItem(item, action);
    } catch (error) {
      toast.error(marketplaceErrorMessage(error, `Failed to update ${item.extension.name}`));
    }
  };

  const loadInstallPreview = async (entry: MarketplaceCatalogListing, allowUnverified: boolean) => {
    const request = catalogInstallRequest(entry, allowUnverified, destination);
    const preview = await previewExtensionInstall(request);
    acquisition.preview.trigger.previewLoaded({ selected: { entry, preview, request } });
  };

  const previewInstall = async (entry: MarketplaceCatalogListing, allowUnverified: boolean) => {
    try {
      await loadInstallPreview(entry, allowUnverified);
    } catch (error) {
      if (marketplaceErrorCode(error) !== "extension_source_changed") throw error;
      acquisition.preview.trigger.previewDismissed();
      reportFailure(error, "The extension changed. Review the current package before installing.");
      await reopenCurrentInstall(entry);
    }
  };

  const install = (entry: MarketplaceCatalogListing) => {
    if (entry.name_conflict || entry.trust?.decision === "blocked" || entry.installable === false)
      return;
    if (entry.trust?.decision === "allowed_unverified") {
      store.trigger.extensionTrustRequested({ entry });
      return;
    }
    void withPendingEntry(entry, () => previewInstall(entry, false));
  };

  const catalogUpdateRequest = async (
    entry: MarketplaceCatalogListing,
    allowUnverified: boolean
  ) => {
    const name = installedName(entry);
    const inventory = await queryClient.fetchQuery({
      ...extensionsListOptions(destination),
      staleTime: 0,
    });
    const extension = inventory.find(item => item.name === name);
    if (!extension) throw new Error(`Installed identity is unavailable for ${entry.name}`);
    return {
      name,
      body: {
        ...extensionUpdateScope(extension),
        allow_unverified: allowUnverified,
        version: entry.version,
      },
    };
  };

  const update = (entry: MarketplaceCatalogListing) => {
    if (entry.trust?.decision === "allowed_unverified") {
      store.trigger.extensionTrustRequested({ entry });
      return;
    }
    void withPendingEntry(entry, async () => {
      const done = await runUpdate(entry.name, await catalogUpdateRequest(entry, false), action =>
        pending.trackEntry(entry, action)
      );
      if (done) updatedToast(entry.name, entry.version);
    });
  };

  const runInstalledUpdate = (item: InstalledExtensionView, allowUnverified: boolean) => {
    const name = item.extension.name;
    const version = item.listing?.version ?? item.extension.remote_version;
    void withPendingItem(item, async () => {
      const done = await runUpdate(
        name,
        {
          body: {
            ...extensionUpdateScope(item.extension),
            allow_unverified: allowUnverified,
            version,
          },
          name,
        },
        action => pending.trackItem(item, action)
      );
      setInstalledTrust(current => (current === item ? null : current));
      if (done) {
        pending.flashItem(item);
        updatedToast(name, version);
      }
    });
  };

  const updateInstalled = (item: InstalledExtensionView) => {
    if (!extensionTrustFacts(item.extension).checksumVerified) {
      setInstalledTrust(item);
      return;
    }
    runInstalledUpdate(item, false);
  };

  const toggleEnabled = (item: InstalledExtensionView, enabled: boolean) => {
    void withPendingItem(item, async () => {
      await toggleExtension.mutateAsync({
        enabled,
        name: item.extension.name,
        profileName: item.extension.profile,
        workspaceId: item.extension.workspace_id ?? null,
      });
    });
  };

  const confirmTrust = () => {
    store.trigger.extensionTrustConfirmed({
      describeFailure: error => marketplaceErrorMessage(error, "Failed to install the extension"),
      notifySuccess: entry => {
        if (entry.update_available) updatedToast(entry.name, entry.version);
      },
      execute: entry =>
        pending.trackEntry(entry, async () => {
          if (entry.update_available) {
            return runUpdate(entry.name, await catalogUpdateRequest(entry, true), action =>
              pending.trackEntry(entry, action)
            );
          }
          await previewInstall(entry, true);
          return true;
        }),
    });
  };

  const reopenCurrentInstall = async (previous: MarketplaceCatalogListing) => {
    const options = marketplaceCatalogEntryOptions({
      entryId: previous.entry_id,
      source: previous.source,
      profileName: destination.profileName,
      workspaceId: destination.workspaceId,
    });
    await queryClient.cancelQueries({ queryKey: options.queryKey, exact: true });
    const { entry } = await queryClient.fetchQuery({ ...options, staleTime: 0 });
    if (marketplaceOriginKey(entry) !== marketplaceOriginKey(previous)) {
      throw new Error(
        "The catalog entry now belongs to another origin. Review it before installing."
      );
    }
    if (entry.name_conflict || entry.trust?.decision === "blocked" || entry.installable === false) {
      throw new Error(entry.install_blocker || "This extension can no longer be installed.");
    }
    if (entry.trust?.decision === "allowed_unverified") {
      store.trigger.extensionTrustRequested({ entry });
      return;
    }
    await loadInstallPreview(entry, false);
  };

  const confirmInstall = (draft: ExtensionInputDraft = {}) => {
    const selected = installPreview;
    if (!selected) return;
    const prepared = prepareExtensionInputs(selected.preview.inputs, draft);
    if (!prepared.valid) return;
    void withPendingEntry(selected.entry, async () => {
      try {
        const { extension } = await installExtension.mutateAsync({
          ...selected.request,
          ...(Object.keys(prepared.inputs).length ? { inputs: prepared.inputs } : {}),
          ...(selected.preview.network_requirement_digest
            ? { confirm_network_digest: selected.preview.network_requirement_digest }
            : {}),
        });
        acquisition.preview.trigger.previewDismissed();
        pending.flashEntry(selected.entry);
        viewInstalledToast(
          `${selected.entry.name} installed`,
          extension.mcp_servers?.some(server => server.status === "needs_authorization") ?? false
        );
      } catch (error) {
        acquisition.preview.trigger.previewDismissed();
        if (marketplaceErrorCode(error) !== "extension_source_changed") throw error;
        reportFailure(
          error,
          "The extension changed. Review the current package before installing."
        );
        await reopenCurrentInstall(selected.entry);
        return;
      }
    });
  };

  const dialogs = (
    <>
      {installedTrust ? (
        <ExtensionTrustDialog
          action="update"
          name={installedTrust.extension.name}
          onConfirm={() => runInstalledUpdate(installedTrust, true)}
          onOpenChange={open => {
            if (!open && !pending.isItemPending(installedTrust)) setInstalledTrust(null);
          }}
          open
          pending={pending.isItemPending(installedTrust)}
          warnings={installedTrust.extension.trust?.warnings}
        />
      ) : null}
      {trustEntry ? (
        <ExtensionTrustDialog
          action={trustEntry.update_available ? "update" : "install"}
          error={phase.status === "extensionTrust" ? phase.error : null}
          name={trustEntry.name}
          onConfirm={confirmTrust}
          onOpenChange={open => {
            if (!open) store.trigger.dialogDismissed();
          }}
          open
          pending={phase.status === "extensionTrustSubmitting"}
          warnings={trustEntry.trust?.warnings}
        />
      ) : null}
      {updateFlow.recovery?.kind === "network" ? (
        <ExtensionNetworkConfirmDialog
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
      {installPreview ? (
        <ExtensionInstallSummaryDialog
          action="install"
          key={JSON.stringify([installPreview.request, installPreview.preview.inputs])}
          onConfirm={confirmInstall}
          onOpenChange={open => {
            if (!open && !entryActions.current.has(marketplaceOriginKey(installPreview.entry)))
              acquisition.preview.trigger.previewDismissed();
          }}
          open
          pending={pending.isEntryPending(installPreview.entry)}
          preview={installPreview.preview}
          entry={installPreview.entry}
          destination={installPreview.request}
        />
      ) : null}
    </>
  );

  return {
    dialogs,
    endEntryFlash: pending.endEntryFlash,
    endItemFlash: pending.endItemFlash,
    flashItem: pending.flashItem,
    install,
    isEntryFlashing: pending.isEntryFlashing,
    isEntryPending: pending.isEntryPending,
    isItemFlashing: pending.isItemFlashing,
    isItemPending: pending.isItemPending,
    toggleEnabled,
    trackInstalledItems: pending.trackItems,
    update,
    updateInstalled,
  };
}

function reportFailure(error: unknown, fallback: string) {
  const message = marketplaceErrorMessage(error, fallback);
  const code = marketplaceErrorCode(error);
  if (code) toast.error(message, { description: code });
  else toast.error(message);
}

export { useMarketplaceActionController };
export type { MarketplaceActionController };
