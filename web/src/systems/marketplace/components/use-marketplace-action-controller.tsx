import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useSelector } from "@xstate/store-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "@compozy/ui";

import {
  extensionTrustFacts,
  extensionUpdateScope,
  extensionsListOptions,
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
import type {
  MarketplaceCatalogListing,
  MarketplaceEnableFailure,
  MarketplaceEnableTarget,
} from "../types";
import { marketplaceCatalogEntryOptions } from "../lib/query-options";
import { marketplaceOriginKey } from "../lib/marketplace-installed-view";
import { useStoreBinding } from "@/hooks/use-store-binding";
import {
  marketplaceActionControllerLogic,
  marketplaceInstallNeedsReview,
  marketplaceInstallPreviewLogic,
  type MarketplaceInstallPreview,
} from "./marketplace-action-controller-logic";
import { MarketplaceActionDialogs } from "./marketplace-action-dialogs";
import {
  catalogInstallRequest,
  installedName,
  installRequestScope,
  reportFailure,
  updatedToast,
} from "./marketplace-action-requests";
import { marketplaceErrorMessage, marketplaceErrorCode } from "./marketplace-ui";
import { useMarketplaceUpdateRecovery } from "./use-marketplace-update-recovery";
import {
  createExtensionInputDraft,
  prepareExtensionInputs,
  type ExtensionInputDraft,
} from "./extension-install-model";
import { useMarketplacePending } from "./use-marketplace-pending";
import { useMarketplaceEnableRecovery } from "./use-marketplace-enable-recovery";

/**
 * An Add that installed but could not turn on needs the user to act, so its toast outlives the
 * default few seconds that a slow install would otherwise leave unread.
 */
const ENABLE_FAILURE_TOAST_MS = 10_000;

interface MarketplaceActionController {
  dialogs: React.ReactNode;
  /** The latest Add / Ready / Couldn't-turn-on transition, for the page's polite live region. */
  announcement: string;
  /**
   * Catalog row Add: install and enable in one call. Consent comes first for an unverified entry;
   * the review dialog opens only when a required input or the gateway needs a decision.
   */
  install: (entry: MarketplaceCatalogListing) => void;
  /** The enable an Add could not finish, still on the row until Retry or Turn on succeeds. */
  enableFailureFor: (entry: MarketplaceCatalogListing) => MarketplaceEnableFailure | null;
  isEntryEnabling: (entry: MarketplaceCatalogListing) => boolean;
  retryEnable: (entry: MarketplaceCatalogListing) => void;
  turnOn: (entry: MarketplaceCatalogListing, target: MarketplaceEnableTarget) => void;
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

/**
 * Install/update/enable for the one-kind catalog. Every mutation keeps its daemon gates — install
 * preview, unverified consent, gateway confirmation — and rows report pending by origin or by the
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
  const [announcement, setAnnouncement] = useState("");
  const enableRecovery = useMarketplaceEnableRecovery(destination, setAnnouncement);
  const updateFlow = useMarketplaceUpdateRecovery(updateExtension.mutateAsync, updatedToast);
  const { runUpdate } = updateFlow;
  const destinationKey = JSON.stringify([destination.profileName, destination.workspaceId]);
  // The destination now on screen: a preview answered for an older one never installs inline.
  const liveDestination = useRef(destinationKey);
  useEffect(() => {
    liveDestination.current = destinationKey;
  });
  const { store: acquisition } = useStoreBinding(destinationKey, () => ({
    trust: marketplaceActionControllerLogic.createStore(),
    preview: marketplaceInstallPreviewLogic.createStore(),
  }));
  const installPreview = useSelector(acquisition.preview, snapshot => snapshot.context.selected);
  const [installedTrust, setInstalledTrust] = useState<InstalledExtensionView | null>(null);
  const store = acquisition.trust;
  const phase = useSelector(store, snapshot => snapshot.context);
  const trustEntry =
    phase.status === "extensionTrust" || phase.status === "extensionTrustSubmitting"
      ? phase.entry
      : null;

  const viewInstalledToast = (message: string, needsAuthorization: boolean) => {
    // The row itself now reads Set up or Ready; the toast only links the Installed view.
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
    if (liveDestination.current !== destinationKey) return;
    const selected = { entry, preview, request };
    if (marketplaceInstallNeedsReview(preview)) {
      acquisition.preview.trigger.previewLoaded({ selected });
      return;
    }
    await submitInstall(selected, createExtensionInputDraft(preview.inputs));
  };

  const previewInstall = async (entry: MarketplaceCatalogListing, allowUnverified: boolean) => {
    if (!entry.update_available) setAnnouncement(`Adding ${entry.name}…`);
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

  /** Install + enable one previewed selection; a changed package reopens review. */
  const submitInstall = async (selected: MarketplaceInstallPreview, draft: ExtensionInputDraft) => {
    const prepared = prepareExtensionInputs(selected.preview.inputs, draft);
    if (!prepared.valid) return;
    try {
      const { enable_error: enableError, extension } = await installExtension.mutateAsync({
        ...selected.request,
        ...(Object.keys(prepared.inputs).length ? { inputs: prepared.inputs } : {}),
        ...(selected.preview.gateway_requirement_digest
          ? { confirm_gateway_digest: selected.preview.gateway_requirement_digest }
          : {}),
      });
      acquisition.preview.trigger.previewDismissed();
      pending.flashEntry(selected.entry);
      // The destination this request named, even if the window has switched since.
      const scope = installRequestScope(selected.request);
      if (enableError) {
        enableRecovery.record(scope, selected.entry, {
          cause: enableError,
          name: extension.name,
          profileName: extension.profile,
          workspaceId: extension.workspace_id ?? null,
        });
        toast.error(`Added ${selected.entry.name} but couldn't turn it on`, {
          description: enableError.error,
          duration: ENABLE_FAILURE_TOAST_MS,
        });
        setAnnouncement(
          `Added ${selected.entry.name} but couldn't turn it on: ${enableError.error}`
        );
        return;
      }
      enableRecovery.record(scope, selected.entry, null);
      const needsAuthorization =
        extension.mcp_servers?.some(server => server.status === "needs_authorization") ?? false;
      viewInstalledToast(`${selected.entry.name} added`, needsAuthorization);
      setAnnouncement(
        `${selected.entry.name} added · ${needsAuthorization ? "Set up to finish" : "Ready"}`
      );
    } catch (error) {
      acquisition.preview.trigger.previewDismissed();
      if (marketplaceErrorCode(error) !== "extension_source_changed") throw error;
      reportFailure(error, "The extension changed. Review the current package before installing.");
      await reopenCurrentInstall(selected.entry);
    }
  };

  const confirmInstall = (draft: ExtensionInputDraft = {}) => {
    const selected = installPreview;
    if (!selected) return;
    if (!prepareExtensionInputs(selected.preview.inputs, draft).valid) return;
    void withPendingEntry(selected.entry, () => submitInstall(selected, draft));
  };

  const dialogs = (
    <MarketplaceActionDialogs
      install={
        installPreview
          ? {
              selected: installPreview,
              pending: pending.isEntryPending(installPreview.entry),
              onConfirm: confirmInstall,
              onDismiss: () => {
                if (!entryActions.current.has(marketplaceOriginKey(installPreview.entry)))
                  acquisition.preview.trigger.previewDismissed();
              },
            }
          : null
      }
      installedTrust={
        installedTrust
          ? {
              item: installedTrust,
              pending: pending.isItemPending(installedTrust),
              onConfirm: () => runInstalledUpdate(installedTrust, true),
              onDismiss: () => {
                if (!pending.isItemPending(installedTrust)) setInstalledTrust(null);
              },
            }
          : null
      }
      trust={
        trustEntry
          ? {
              entry: trustEntry,
              error: phase.status === "extensionTrust" ? phase.error : null,
              pending: phase.status === "extensionTrustSubmitting",
              onConfirm: confirmTrust,
              onDismiss: () => store.trigger.dialogDismissed(),
            }
          : null
      }
      updateFlow={updateFlow}
    />
  );

  return {
    announcement,
    dialogs,
    enableFailureFor: enableRecovery.failureFor,
    isEntryEnabling: enableRecovery.isEnabling,
    retryEnable: enableRecovery.retry,
    turnOn: (entry, target) => void enableRecovery.turnOn(entry, target),
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

export { useMarketplaceActionController };
export type { MarketplaceActionController };
