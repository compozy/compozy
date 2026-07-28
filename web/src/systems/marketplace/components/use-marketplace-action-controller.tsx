import { useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import { useMCPAuthorize } from "@/systems/settings";
import {
  useDeactivateBundle,
  useRemoveExtension,
  useToggleExtension,
  useUpdateBundleActivation,
} from "@/systems/extensions";
import {
  deriveMCPAuthFilter,
  deriveMCPManagementFilter,
  useDeleteSettingsMCPServer,
  type SettingsMCPServerEntry,
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
  MarketplaceEntryResponse,
  MarketplaceKind,
  MarketplaceListing,
  MCPInstallRequest,
} from "../types";
import { marketplaceRouteKindFor } from "../types";
import { MarketplaceActionDialogs } from "./marketplace-action-dialogs";
import { marketplaceEntrySlug, marketplaceErrorMessage } from "./marketplace-ui";
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
  handleDeactivate: (item: MarketplaceInstalledItem) => Promise<void>;
  handleUpdateBundle: (item: MarketplaceInstalledItem) => void;
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
  const deactivateBundle = useDeactivateBundle();
  const updateBundle = useUpdateBundleActivation();
  const pending = useMarketplacePending();
  const [mcpDetail, setMCPDetail] = useState<MarketplaceEntryResponse | null>(null);
  const [bundleDetail, setBundleDetail] = useState<MarketplaceEntryResponse | null>(null);
  const [trustEntry, setTrustEntry] = useState<MarketplaceListing | null>(null);
  const [trustError, setTrustError] = useState<string | null>(null);
  const [authServer, setAuthServer] = useState<SettingsMCPServerEntry | null>(null);
  const [authKick, setAuthKick] = useState(0);
  const startedAuthKick = useRef(0);

  const authFilter = authServer ? deriveMCPAuthFilter(authServer) : null;
  const authorize = useMCPAuthorize(authFilter);
  const beginAuthorize = authorize.beginAuthorize;
  const acknowledgeStatus = authorize.acknowledgeStatus;
  const cancelAuthorize = authorize.cancel;

  const authServerEntry = authorize.server
    ? (options.installedItems?.find(item => item.mcpServer?.name === authorize.server)?.mcpServer ??
      authServer)
    : null;

  useEffect(() => {
    if (authKick === 0 || startedAuthKick.current === authKick || !authServer || !authFilter) {
      return;
    }
    startedAuthKick.current = authKick;
    void beginAuthorize(authServer.name, {
      status: authServer.auth_status?.status ?? "needs_login",
      tokenPresent: Boolean(authServer.auth_status?.token_present),
    });
  }, [authKick, authServer, authFilter, beginAuthorize]);

  useEffect(() => {
    const authStatus = authServerEntry?.auth_status;
    if (authorize.isAwaiting && authStatus?.status) {
      acknowledgeStatus(authStatus.status, Boolean(authStatus.token_present));
    }
  }, [authorize.isAwaiting, acknowledgeStatus, authServerEntry?.auth_status]);

  useEffect(() => {
    if (authorize.phase !== "confirmed" || !authorize.server) return;
    toast.success(`${authorize.server} authorized · server running`);
    cancelAuthorize();
  }, [authorize.phase, authorize.server, cancelAuthorize]);

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
    const kind = entry.kind as MarketplaceKind;
    return queryClient.fetchQuery(
      marketplaceEntryOptions(
        kind === "bundle"
          ? { entryId: entry.entry_id, kind, workspaceId }
          : {
              entryId: entry.entry_id,
              installedName: entry.installed_name,
              kind,
              workspaceId,
            }
      )
    );
  };

  const handleAction = (entry: MarketplaceListing) => {
    if (entry.kind === "extension" && entry.trust?.decision === "blocked") return;
    if (entry.kind === "extension" && entry.trust?.decision === "allowed_unverified") {
      setTrustError(null);
      setTrustEntry(entry);
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
          toast.success(version ? `${entry.name} updated to v${version}` : `${entry.name} updated`);
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
          toast.success(
            entry.version ? `${entry.name} updated to v${entry.version}` : `${entry.name} updated`
          );
          return;
        }
        await installExtension.mutateAsync({
          allow_unverified: false,
          slug: marketplaceEntrySlug(entry),
          version: entry.version,
        });
        pending.flash(entry);
        viewInstalledToast(entry, `${entry.name} installed`);
        return;
      }
      const detail = await fetchDetail(entry);
      if (entry.kind === "mcp") setMCPDetail(detail);
      if (entry.kind === "bundle") setBundleDetail(detail);
    });
  };

  const handleAuthorize = (item: MarketplaceInstalledItem) => {
    const server = item.mcpServer;
    if (!server) {
      toast.error(`MCP server identity is unavailable for ${item.entry.name}`);
      return;
    }
    if (!deriveMCPAuthFilter(server)) {
      toast.error(`Workspace scope is required to authorize ${server.name}`);
      return;
    }
    setAuthServer(server);
    setAuthKick(current => current + 1);
  };

  const confirmUnverifiedExtension = async () => {
    if (!trustEntry) return;
    setTrustError(null);
    try {
      await pending.trackEntry(trustEntry, async () => {
        if (trustEntry.update_available) {
          await updateExtension.mutateAsync({
            body: { allow_unverified: true, version: trustEntry.version },
            name: installedName(trustEntry),
          });
          toast.success(`${trustEntry.name} updated`);
          return;
        }
        await installExtension.mutateAsync({
          allow_unverified: true,
          slug: marketplaceEntrySlug(trustEntry),
          version: trustEntry.version,
        });
        pending.flash(trustEntry);
        viewInstalledToast(trustEntry, `${trustEntry.name} installed`);
      });
      setTrustEntry(null);
    } catch (error) {
      setTrustError(marketplaceErrorMessage(error, "Failed to install the extension"));
    }
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
        await removeExtension.mutateAsync(installedName(entry));
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

  const handleDeactivate = async (item: MarketplaceInstalledItem) => {
    const activationId = item.activationId;
    if (!activationId) {
      throw new Error(`Activation id is unavailable for ${item.entry.name}`);
    }
    await pending.trackItem(item, async () => {
      await deactivateBundle.mutateAsync(activationId);
    });
  };

  const handleUpdateBundle = (item: MarketplaceInstalledItem) => {
    void withPendingItem(item, async () => {
      if (!item.activationId || item.activationVersion === undefined) {
        throw new Error(`Activation identity is unavailable for ${item.entry.name}`);
      }
      await updateBundle.mutateAsync({
        body: { expected_version: item.activationVersion },
        id: item.activationId,
      });
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
      authServer={authServer}
      bundleDetail={bundleDetail}
      mcpDetail={mcpDetail}
      onBundleClose={() => setBundleDetail(null)}
      onConfirmTrust={() => void confirmUnverifiedExtension()}
      onInstallMCP={installSelectedMCP}
      onMCPClose={() => setMCPDetail(null)}
      onTrustClose={() => setTrustEntry(null)}
      trustEntry={trustEntry}
      trustError={trustError}
      trustPending={installExtension.isPending || updateExtension.isPending}
      workspaceId={workspaceId}
    />
  );

  return {
    dialogs,
    handleAction,
    handleAuthorize,
    handleDeactivate,
    handleFlashEnd: pending.handleFlashEnd,
    handleRemove,
    handleToggleEnabled,
    handleUpdateBundle,
    isAuthorizing: authorize.isAwaiting,
    isEntryFlashing: pending.isEntryFlashing,
    isEntryPending: pending.isEntryPending,
    isInstalledItemPending: pending.isItemPending,
  };
}

export { useMarketplaceActionController };
export type { MarketplaceActionController, MarketplaceActionControllerOptions };
