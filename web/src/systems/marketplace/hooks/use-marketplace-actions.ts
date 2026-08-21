import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { reconcileInstalledExtensionCaches } from "@/integrations/tanstack-query/reconcile-installed-extension";
import { updateExtension } from "@/systems/extensions/adapters/extensions-api";

import {
  installMarketplaceExtension,
  installMarketplaceMCP,
  installMarketplaceSkill,
  refreshMarketplaceCatalog,
  updateMarketplaceSkill,
} from "../adapters/marketplace-actions-api";
import { marketplaceKeys } from "../lib/query-keys";
import type {
  ExtensionInstallRequest,
  ExtensionUpdateRequest,
  MarketplaceKind,
  MCPInstallRequest,
  SkillInstallRequest,
  SkillUpdateRequest,
} from "../types";
import { sessionKeys } from "@/systems/session";
import { settingsKeys } from "@/systems/settings";
import { skillKeys } from "@/systems/skill";

type MCPInstalledPathInput =
  | { entryId: string; scope: "user"; server: string }
  | { entryId: string; scope: "profile"; server: string; profile: string; workspaceId?: string }
  | { entryId: string; scope: "workspace"; server: string; workspaceId: string };

function mcpInstalledPath(input: MCPInstalledPathInput): string {
  const search = new URLSearchParams({ scope: input.scope });
  if (input.server) search.set("installed_name", input.server);
  if (input.scope !== "user" && "workspaceId" in input && input.workspaceId) {
    search.set("workspace_id", input.workspaceId);
  }
  if (input.scope === "profile") {
    search.set("profile", input.profile);
  }
  return `/marketplace/mcp/${encodeURIComponent(input.entryId)}?${search.toString()}`;
}

function invalidateMarketplace(queryClient: ReturnType<typeof useQueryClient>) {
  return queryClient.invalidateQueries({ queryKey: marketplaceKeys.all });
}

function mcpToastAction(label: "Authorize →" | "View installed →", path: string) {
  return {
    action: {
      label,
      onClick: () => globalThis.location.assign(path),
    },
  };
}

export function useRefreshMarketplaceCatalog() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (kind: MarketplaceKind | undefined) => refreshMarketplaceCatalog(kind),
    onSettled: () => invalidateMarketplace(queryClient),
  });
}

export function useInstallMarketplaceMCP() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: MCPInstallRequest) => installMarketplaceMCP(body),
    onSuccess: (result, variables) => {
      const scope = result.mcp_server?.scope ?? variables.scope;
      const server = result.mcp_server?.name ?? variables.name ?? "";
      const workspaceId = result.mcp_server?.workspace_id ?? variables.workspace_id;
      const profile = result.mcp_server?.profile ?? variables.profile;
      const pathInput: MCPInstalledPathInput | null =
        scope === "user"
          ? { entryId: variables.entry_id, scope, server }
          : scope === "profile" && profile
            ? { entryId: variables.entry_id, profile, scope, server, workspaceId }
            : scope === "workspace" && workspaceId
              ? { entryId: variables.entry_id, scope, server, workspaceId }
              : null;
      const installedPath = pathInput ? mcpInstalledPath(pathInput) : null;
      const toastAction = installedPath ? mcpToastAction("Authorize →", installedPath) : undefined;
      if (result.next_step === "authorize") {
        toast.success(
          `${result.mcp_server?.name ?? variables.name ?? "MCP server"} installed · authorization pending`,
          toastAction
        );
        return;
      }
      toast.success(
        `${result.mcp_server?.name ?? variables.name ?? "MCP server"} installed`,
        installedPath ? mcpToastAction("View installed →", installedPath) : undefined
      );
    },
    onSettled: () =>
      Promise.all([
        invalidateMarketplace(queryClient),
        queryClient.invalidateQueries({ queryKey: settingsKeys.mcpRoot() }),
      ]),
  });
}

export function useInstallMarketplaceSkill() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: SkillInstallRequest) => installMarketplaceSkill(body),
    onSettled: () =>
      Promise.all([
        invalidateMarketplace(queryClient),
        queryClient.invalidateQueries({ queryKey: skillKeys.all }),
        queryClient.invalidateQueries({ queryKey: sessionKeys.commandsRoot }),
      ]),
  });
}

export function useUpdateMarketplaceSkill() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: SkillUpdateRequest) => updateMarketplaceSkill(body),
    onSettled: () =>
      Promise.all([
        invalidateMarketplace(queryClient),
        queryClient.invalidateQueries({ queryKey: skillKeys.all }),
        queryClient.invalidateQueries({ queryKey: sessionKeys.commandsRoot }),
      ]),
  });
}

export function useInstallMarketplaceExtension() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: ExtensionInstallRequest) => installMarketplaceExtension(body),
    onSettled: () => reconcileInstalledExtensionCaches(queryClient),
  });
}

export function useUpdateMarketplaceExtension() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, body }: { name: string; body: ExtensionUpdateRequest }) =>
      updateExtension(name, body),
    onSettled: () => reconcileInstalledExtensionCaches(queryClient),
  });
}
