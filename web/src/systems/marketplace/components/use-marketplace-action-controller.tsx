import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useSelector, useStore } from "@xstate/store-react";
import { useRef, useState } from "react";
import { toast } from "sonner";

import {
  extensionNetworkConfirmation,
  extensionTrustFacts,
  ExtensionNetworkConfirmDialog,
  previewExtensionInstall,
  type ExtensionInstallPreview,
  type InstalledExtensionView,
  useToggleExtension,
} from "@/systems/extensions";

import {
  useInstallMarketplaceExtension,
  useUpdateMarketplaceExtension,
} from "../hooks/use-marketplace-actions";
import type {
  ExtensionInstallRequest,
  ExtensionUpdateRequest,
  MarketplaceCatalogListing,
} from "../types";
import { marketplaceCatalogEntryOptions } from "../lib/query-options";
import { marketplaceOriginKey } from "../lib/marketplace-installed-view";
import { ExtensionInstallSummaryDialog } from "./extension-install-summary-dialog";
import { ExtensionTrustDialog } from "./extension-trust-dialog";
import { marketplaceActionControllerLogic } from "./marketplace-action-controller-logic";
import {
  formatMarketplaceVersion,
  marketplaceEntrySlug,
  marketplaceErrorMessage,
  marketplaceErrorCode,
} from "./marketplace-ui";
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
  isEntryFlashing: (entry: MarketplaceCatalogListing) => boolean;
  isEntryPending: (entry: MarketplaceCatalogListing) => boolean;
  isItemFlashing: (item: InstalledExtensionView) => boolean;
  isItemPending: (item: InstalledExtensionView) => boolean;
  trackInstalledItems: <T>(
    items: readonly InstalledExtensionView[],
    action: () => Promise<T>
  ) => Promise<T>;
}

interface UpdateRequest {
  body: ExtensionUpdateRequest;
  name: string;
}

interface NetworkConfirm {
  digest: string;
  label: string;
  request: UpdateRequest;
  track: <T>(action: () => Promise<T>) => Promise<T>;
}

interface InstallPreview {
  entry: MarketplaceCatalogListing;
  preview: ExtensionInstallPreview;
  request: ExtensionInstallRequest;
}

function installedName(entry: MarketplaceCatalogListing): string {
  if (!entry.installed_name) {
    throw new Error(`Installed identity is unavailable for ${entry.name}`);
  }
  return entry.installed_name;
}

/** Catalog listings resolve through the curated catalog, so the listing slug is the install ref. */
function curatedInstallRequest(
  entry: MarketplaceCatalogListing,
  allowUnverified: boolean
): ExtensionInstallRequest {
  return {
    allow_unverified: allowUnverified,
    expected_digest: entry.digest_sha256,
    ref: marketplaceEntrySlug(entry),
    source: "curated",
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
function useMarketplaceActionController(): MarketplaceActionController {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const entryActions = useRef(new Set<string>());
  const installExtension = useInstallMarketplaceExtension();
  const updateExtension = useUpdateMarketplaceExtension();
  const toggleExtension = useToggleExtension();
  const pending = useMarketplacePending();
  const [networkConfirm, setNetworkConfirm] = useState<NetworkConfirm | null>(null);
  const [installPreview, setInstallPreview] = useState<InstallPreview | null>(null);
  const [installedTrust, setInstalledTrust] = useState<InstalledExtensionView | null>(null);
  const store = useStore(marketplaceActionControllerLogic);
  const phase = useSelector(store, snapshot => snapshot.context);
  const trustEntry =
    phase.status === "extensionTrust" || phase.status === "extensionTrustSubmitting"
      ? phase.entry
      : null;

  const viewInstalledToast = (message: string) => {
    toast.success(message, {
      action: {
        label: "View installed →",
        onClick: () => void navigate({ search: {}, to: "/marketplace/installed" }),
      },
    });
  };

  /** Returns false when the daemon asked for a network confirmation instead of updating. */
  const runUpdate = async (
    label: string,
    request: UpdateRequest,
    track: NetworkConfirm["track"]
  ): Promise<boolean> => {
    try {
      await updateExtension.mutateAsync(request);
      return true;
    } catch (error) {
      const confirmation = extensionNetworkConfirmation(error);
      if (!confirmation) throw error;
      setNetworkConfirm({ digest: confirmation.digest, label, request, track });
      return false;
    }
  };

  const withPendingEntry = async (
    entry: MarketplaceCatalogListing,
    action: () => Promise<void>
  ) => {
    const key = marketplaceOriginKey(entry);
    if (entryActions.current.has(key)) return;
    entryActions.current.add(key);
    try {
      await pending.trackEntry(entry, action);
    } catch (error) {
      reportFailure(error, `Failed to update ${entry.name}`);
    } finally {
      entryActions.current.delete(key);
    }
  };

  const withPendingItem = async (item: InstalledExtensionView, action: () => Promise<void>) => {
    try {
      await pending.trackItem(item, action);
    } catch (error) {
      toast.error(marketplaceErrorMessage(error, `Failed to update ${item.extension.name}`));
    }
  };

  const loadInstallPreview = async (entry: MarketplaceCatalogListing, allowUnverified: boolean) => {
    const request = curatedInstallRequest(entry, allowUnverified);
    const preview = await previewExtensionInstall(request);
    setInstallPreview({ entry, preview, request });
  };

  const previewInstall = async (entry: MarketplaceCatalogListing, allowUnverified: boolean) => {
    try {
      await loadInstallPreview(entry, allowUnverified);
    } catch (error) {
      if (marketplaceErrorCode(error) !== "extension_source_changed") throw error;
      setInstallPreview(null);
      reportFailure(error, "The extension changed. Review the current package before installing.");
      await reopenCurrentInstall(entry);
    }
  };

  const install = (entry: MarketplaceCatalogListing) => {
    if (entry.trust?.decision === "blocked" || entry.installable === false) return;
    if (entry.trust?.decision === "allowed_unverified") {
      store.trigger.extensionTrustRequested({ entry });
      return;
    }
    void withPendingEntry(entry, () => previewInstall(entry, false));
  };

  const update = (entry: MarketplaceCatalogListing) => {
    if (entry.trust?.decision === "allowed_unverified") {
      store.trigger.extensionTrustRequested({ entry });
      return;
    }
    void withPendingEntry(entry, async () => {
      const done = await runUpdate(
        entry.name,
        { body: { allow_unverified: false, version: entry.version }, name: installedName(entry) },
        action => pending.trackEntry(entry, action)
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
        { body: { allow_unverified: allowUnverified, version }, name },
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
      await toggleExtension.mutateAsync({ enabled, name: item.extension.name });
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
            return runUpdate(
              entry.name,
              {
                body: { allow_unverified: true, version: entry.version },
                name: installedName(entry),
              },
              action => pending.trackEntry(entry, action)
            );
          }
          await previewInstall(entry, true);
          return true;
        }),
    });
  };

  const submitNetworkConfirm = () => {
    const confirm = networkConfirm;
    if (!confirm) return;
    void confirm
      .track(async () => {
        const done = await runUpdate(
          confirm.label,
          {
            ...confirm.request,
            body: { ...confirm.request.body, confirm_network_digest: confirm.digest },
          },
          confirm.track
        );
        if (!done) return;
        setNetworkConfirm(null);
        updatedToast(confirm.label, confirm.request.body.version);
      })
      .catch((error: unknown) => {
        toast.error(marketplaceErrorMessage(error, `Failed to update ${confirm.label}`));
      });
  };

  const reopenCurrentInstall = async (previous: MarketplaceCatalogListing) => {
    const options = marketplaceCatalogEntryOptions({
      entryId: previous.entry_id,
      source: previous.source,
    });
    await queryClient.cancelQueries({ queryKey: options.queryKey, exact: true });
    const { entry } = await queryClient.fetchQuery({ ...options, staleTime: 0 });
    if (marketplaceOriginKey(entry) !== marketplaceOriginKey(previous)) {
      throw new Error(
        "The catalog entry now belongs to another origin. Review it before installing."
      );
    }
    if (entry.trust?.decision === "blocked" || entry.installable === false) {
      throw new Error(entry.install_blocker || "This extension can no longer be installed.");
    }
    if (entry.trust?.decision === "allowed_unverified") {
      store.trigger.extensionTrustRequested({ entry });
      return;
    }
    await loadInstallPreview(entry, false);
  };

  const confirmInstall = () => {
    const selected = installPreview;
    if (!selected) return;
    void withPendingEntry(selected.entry, async () => {
      try {
        await installExtension.mutateAsync({
          ...selected.request,
          ...(selected.preview.network_requirement_digest
            ? { confirm_network_digest: selected.preview.network_requirement_digest }
            : {}),
        });
      } catch (error) {
        setInstallPreview(null);
        if (marketplaceErrorCode(error) !== "extension_source_changed") throw error;
        reportFailure(
          error,
          "The extension changed. Review the current package before installing."
        );
        await reopenCurrentInstall(selected.entry);
        return;
      }
      setInstallPreview(null);
      pending.flashEntry(selected.entry);
      viewInstalledToast(`${selected.entry.name} installed`);
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
      {networkConfirm ? (
        <ExtensionNetworkConfirmDialog
          digest={networkConfirm.digest}
          extensionName={networkConfirm.label}
          onConfirm={submitNetworkConfirm}
          onOpenChange={open => {
            if (!open) setNetworkConfirm(null);
          }}
          open
          pending={updateExtension.isPending}
        />
      ) : null}
      {installPreview ? (
        <ExtensionInstallSummaryDialog
          onConfirm={confirmInstall}
          onOpenChange={open => {
            if (!open && !entryActions.current.has(marketplaceOriginKey(installPreview.entry)))
              setInstallPreview(null);
          }}
          open
          pending={pending.isEntryPending(installPreview.entry)}
          preview={installPreview.preview}
        />
      ) : null}
    </>
  );

  return {
    dialogs,
    endEntryFlash: pending.endEntryFlash,
    endItemFlash: pending.endItemFlash,
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
