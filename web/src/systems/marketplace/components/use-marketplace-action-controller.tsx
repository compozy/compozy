import { useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";
import { useEffect } from "react";
import { toast } from "sonner";

import { isMCPAuthorizeAwaiting, isMCPAuthorizePending, useMCPAuthorize } from "@/systems/settings";
import { useRemoveExtension, useToggleExtension } from "@/systems/extensions";
import {
  deriveMCPAuthFilter,
  deriveMCPManagementFilter,
  useDeleteSettingsMCPServer,
} from "@/systems/settings";
import { useRemoveSkillMarketplace } from "@/systems/skill";

import {
  useInstallMarketplaceExtension,
  useInstallMarketplaceMCP,
  useInstallMarketplaceSkill,
  useUpdateMarketplaceExtension,
  useUpdateMarketplaceSkill,
} from "../hooks/use-marketplace-actions";
import type { MarketplaceInstalledItem } from "../hooks/use-marketplace-kind-page";
import { marketplaceEntryOptions } from "../lib/query-options";
import type {
  ExtensionInstallRequest,
  MarketplaceKind,
  MarketplaceListing,
  MCPInstallRequest,
} from "../types";
import { marketplaceRouteKindFor } from "../types";
import { MarketplaceActionDialogs } from "./marketplace-action-dialogs";
import {
  formatMarketplaceVersion,
  marketplaceEntrySlug,
  marketplaceErrorMessage,
} from "./marketplace-ui";
import {
  marketplaceActionControllerLogic,
  type MarketplaceActionControllerPhase,
  type MarketplaceDialogSelection,
} from "./marketplace-action-controller-logic";
import { useMarketplacePending } from "./use-marketplace-pending";

interface MarketplaceActionControllerOptions {
  onViewInstalled?: () => void;
  installedItems?: readonly MarketplaceInstalledItem[];
}

interface MarketplaceActionController {
  dialogs: React.ReactNode;
  handleAction: (entry: MarketplaceListing) => void;
  handleAuthorize: (item: MarketplaceInstalledItem) => void;
  handleRemove: (item: MarketplaceInstalledItem) => Promise<void>;
  handleToggleEnabled: (item: MarketplaceInstalledItem, enabled: boolean) => void;
  handleFlashEnd: (entry: MarketplaceListing) => void;
  isEntryPending: (entry: MarketplaceListing) => boolean;
  isInstalledItemPending: (item: MarketplaceInstalledItem) => boolean;
  isEntryFlashing: (entry: MarketplaceListing) => boolean;
  isAuthorizing: boolean;
}

function installedName(entry: MarketplaceListing): string {
  if (!entry.installed_name) {
    throw new Error(`Installed identity is unavailable for ${entry.name}`);
  }
  return entry.installed_name;
}

// Marketplace listings resolve through the curated catalog, so the listing slug is the install ref.
function curatedInstallRequest(
  entry: MarketplaceListing,
  allowUnverified: boolean
): ExtensionInstallRequest {
  return {
    allow_unverified: allowUnverified,
    ref: marketplaceEntrySlug(entry),
    source: "curated",
    version: entry.version,
  };
}

function useMarketplaceActionController(
  workspaceId?: string | null,
  options: MarketplaceActionControllerOptions = {}
): MarketplaceActionController {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const installSkill = useInstallMarketplaceSkill();
  const updateSkill = useUpdateMarketplaceSkill();
  const installExtension = useInstallMarketplaceExtension();
  const updateExtension = useUpdateMarketplaceExtension();
  const installMCP = useInstallMarketplaceMCP();
  const removeSkill = useRemoveSkillMarketplace();
  const removeExtension = useRemoveExtension();
  const deleteMCP = useDeleteSettingsMCPServer();
  const toggleExtension = useToggleExtension();
  const pending = useMarketplacePending();
  const controllerStore = useStore(marketplaceActionControllerLogic);
  const phase = useSelector(controllerStore, snapshot => snapshot.context);
  const dialogSelection = marketplaceDialogSelection(phase);
  const dialogDetail =
    useQuery(marketplaceDialogEntryOptions(dialogSelection, workspaceId)).data ?? null;
  const authorize = useMCPAuthorize({
    dismissOnConfirmation: true,
    onConfirmed: server => toast.success(`${server} authorized · server running`),
  });
  const acknowledgeStatus = authorize.acknowledgeStatus;
  const awaitingAuthorization = isMCPAuthorizeAwaiting(authorize.phase);

  const authServerEntry = authorize.server
    ? (options.installedItems?.find(item => item.mcpServer?.name === authorize.server)?.mcpServer ??
      null)
    : null;
  const authFilter = authServerEntry ? deriveMCPAuthFilter(authServerEntry) : null;

  useEffect(() => {
    const authStatus = authServerEntry?.auth_status;
    if (awaitingAuthorization && authStatus?.status) {
      acknowledgeStatus(authStatus.status, Boolean(authStatus.token_present));
    }
  }, [acknowledgeStatus, awaitingAuthorization, authServerEntry?.auth_status]);

  const goToInstalled = (kind: MarketplaceKind) => {
    if (options.onViewInstalled) {
      options.onViewInstalled();
      return;
    }
    void navigate({
      search: { tab: "installed" },
      to: `/marketplace/${marketplaceRouteKindFor(kind)}`,
    });
  };

  const viewInstalledToast = (entry: MarketplaceListing, message: string) => {
    toast.success(message, {
      action: {
        label: "View installed →",
        onClick: () => goToInstalled(entry.kind as MarketplaceKind),
      },
    });
  };

  const handleExtensionTrusted = (entry: MarketplaceListing) => {
    if (entry.update_available) {
      toast.success(`${entry.name} updated`);
      return;
    }
    pending.flash(entry);
    viewInstalledToast(entry, `${entry.name} installed`);
  };

  const withPendingEntry = async (entry: MarketplaceListing, action: () => Promise<void>) => {
    try {
      await pending.trackEntry(entry, action);
    } catch (error) {
      toast.error(marketplaceErrorMessage(error, `Failed to update ${entry.name}`));
    }
  };

  const withPendingItem = async (item: MarketplaceInstalledItem, action: () => Promise<void>) => {
    try {
      await pending.trackItem(item, action);
    } catch (error) {
      toast.error(marketplaceErrorMessage(error, `Failed to update ${item.entry.name}`));
    }
  };

  const fetchDetail = async (entry: MarketplaceListing) => {
    return queryClient.fetchQuery(
      marketplaceEntryOptions({
        entryId: entry.entry_id,
        installedName: entry.installed_name,
        kind: entry.kind as MarketplaceKind,
        workspaceId,
      })
    );
  };

  const handleAction = (entry: MarketplaceListing) => {
    if (entry.kind === "extension" && entry.trust?.decision === "blocked") return;
    if (entry.kind === "extension" && entry.trust?.decision === "allowed_unverified") {
      controllerStore.trigger.extensionTrustRequested({ entry });
      return;
    }
    if (entry.kind === "mcp") {
      controllerStore.trigger.detailRequested({
        describeFailure: error =>
          marketplaceErrorMessage(error, `Failed to load ${entry.name} details`),
        load: () =>
          pending.trackEntry(entry, async () => {
            await fetchDetail(entry);
          }),
        selection: {
          entryId: entry.entry_id,
          installedName: entry.installed_name,
          kind: entry.kind,
        },
      });
      return;
    }
    void withPendingEntry(entry, async () => {
      if (entry.kind === "skill") {
        if (entry.update_available) {
          const result = await updateSkill.mutateAsync({ name: installedName(entry) });
          const version =
            result.skills?.[0]?.latest_version ??
            result.skills?.[0]?.current_version ??
            entry.version;
          const displayVersion = formatMarketplaceVersion(version);
          toast.success(
            displayVersion ? `${entry.name} updated to ${displayVersion}` : `${entry.name} updated`
          );
        } else {
          await installSkill.mutateAsync({
            slug: marketplaceEntrySlug(entry),
            version: entry.version,
          });
          pending.flash(entry);
          viewInstalledToast(entry, `${entry.name} installed`);
        }
        return;
      }
      if (entry.kind === "extension") {
        if (entry.update_available) {
          await updateExtension.mutateAsync({
            body: { allow_unverified: false, version: entry.version },
            name: installedName(entry),
          });
          const displayVersion = formatMarketplaceVersion(entry.version);
          toast.success(
            displayVersion ? `${entry.name} updated to ${displayVersion}` : `${entry.name} updated`
          );
          return;
        }
        await installExtension.mutateAsync(curatedInstallRequest(entry, false));
        pending.flash(entry);
        viewInstalledToast(entry, `${entry.name} installed`);
        return;
      }
    });
  };

  const handleAuthorize = (item: MarketplaceInstalledItem) => {
    const server = item.mcpServer;
    if (!server) {
      toast.error(`MCP server identity is unavailable for ${item.entry.name}`);
      return;
    }
    const filter = deriveMCPAuthFilter(server);
    if (!filter) {
      toast.error(`Workspace scope is required to authorize ${server.name}`);
      return;
    }
    authorize.requestAuthorize(filter, server);
  };

  const confirmUnverifiedExtension = () => {
    controllerStore.trigger.extensionTrustConfirmed({
      describeFailure: error => marketplaceErrorMessage(error, "Failed to install the extension"),
      notifySuccess: handleExtensionTrusted,
      execute: entry =>
        pending.trackEntry(entry, async () => {
          if (entry.update_available) {
            await updateExtension.mutateAsync({
              body: { allow_unverified: true, version: entry.version },
              name: installedName(entry),
            });
            return;
          }
          await installExtension.mutateAsync(curatedInstallRequest(entry, true));
        }),
    });
  };

  const installSelectedMCP = async (request: MCPInstallRequest) => {
    return installMCP.mutateAsync(request);
  };

  const handleRemove = async (item: MarketplaceInstalledItem) => {
    const entry = item.entry;
    await pending.trackItem(item, async () => {
      if (entry.kind === "skill") {
        await removeSkill.mutateAsync({
          name: installedName(entry),
          workspace: workspaceId ?? "",
        });
        toast.success(`${entry.name} removed`);
        return;
      }
      if (entry.kind === "extension") {
        await removeExtension.mutateAsync({
          dev: item.extensionFacts?.dev === true,
          name: installedName(entry),
        });
        return;
      }
      if (entry.kind === "mcp") {
        const server = item.mcpServer;
        if (!server) throw new Error(`MCP server identity is unavailable for ${entry.name}`);
        const filter = deriveMCPManagementFilter(server);
        if (!filter) {
          throw new Error(`MCP source identity is unavailable for ${entry.name}`);
        }
        const result = await deleteMCP.mutateAsync({ name: server.name, filter });
        const lifecycle = result.restart_required ? "restart required" : "applied now";
        toast.success(`${entry.name} removed · ${lifecycle}`);
        return;
      }
      throw new Error(`Remove is not supported for ${entry.kind}`);
    });
  };

  const handleToggleEnabled = (item: MarketplaceInstalledItem, enabled: boolean) => {
    const name = installedName(item.entry);
    void withPendingItem(item, async () => {
      await toggleExtension.mutateAsync({ enabled, name });
    });
  };

  const dialogs = (
    <MarketplaceActionDialogs
      authorize={authorize}
      authScope={authFilter?.scope ?? "global"}
      authServer={authServerEntry}
      mcpDetail={phase.status === "mcpInstall" ? dialogDetail : null}
      onConfirmTrust={confirmUnverifiedExtension}
      onInstallMCP={installSelectedMCP}
      onMCPClose={() => controllerStore.trigger.dialogDismissed()}
      onTrustClose={() => controllerStore.trigger.dialogDismissed()}
      trustEntry={trustEntry(phase)}
      trustError={trustError(phase)}
      trustPending={phase.status === "extensionTrustSubmitting"}
      workspaceId={workspaceId}
    />
  );

  return {
    dialogs,
    handleAction,
    handleAuthorize,
    handleFlashEnd: pending.handleFlashEnd,
    handleRemove,
    handleToggleEnabled,
    isAuthorizing: isMCPAuthorizePending(authorize.phase),
    isEntryFlashing: pending.isEntryFlashing,
    isEntryPending: pending.isEntryPending,
    isInstalledItemPending: pending.isItemPending,
  };
}

function trustEntry(phase: MarketplaceActionControllerPhase): MarketplaceListing | null {
  return phase.status === "extensionTrust" || phase.status === "extensionTrustSubmitting"
    ? phase.entry
    : null;
}

function marketplaceDialogSelection(
  phase: MarketplaceActionControllerPhase
): MarketplaceDialogSelection | null {
  return phase.status === "mcpInstall" ? phase.selection : null;
}

function marketplaceDialogEntryOptions(
  selection: MarketplaceDialogSelection | null,
  workspaceId?: string | null
) {
  return marketplaceEntryOptions({
    entryId: selection?.entryId ?? "",
    installedName: selection?.installedName,
    kind: "mcp",
    workspaceId,
  });
}

function trustError(phase: MarketplaceActionControllerPhase): string | null {
  return phase.status === "extensionTrust" ? phase.error : null;
}

export { useMarketplaceActionController };
export type { MarketplaceActionController, MarketplaceActionControllerOptions };
