import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { reconcileInstalledExtensionCaches } from "@/integrations/tanstack-query/reconcile-installed-extension";
import { updateExtension } from "@/systems/extensions/adapters/extensions-api";
import { settingsKeys } from "@/systems/settings";
import { skillKeys } from "@/systems/skill";

import {
  activateMarketplaceBundle,
  installMarketplaceExtension,
  installMarketplaceMCP,
  installMarketplaceSkill,
  previewMarketplaceBundle,
  refreshMarketplaceCatalog,
  updateMarketplaceSkill,
} from "../adapters/marketplace-actions-api";
import { marketplaceKeys } from "../lib/query-keys";
import type {
  BundleActivationRequest,
  BundlePreviewRequest,
  ExtensionInstallRequest,
  ExtensionUpdateRequest,
  MarketplaceKind,
  MCPInstallRequest,
  SkillInstallRequest,
  SkillUpdateRequest,
} from "../types";

function mcpInstalledPath(input: {
  entryId: string;
  scope: string;
  server: string;
  workspaceId?: string;
}): string {
  const search = new URLSearchParams({ scope: input.scope });
  if (input.server) search.set("installed_name", input.server);
  if (input.scope === "workspace" && input.workspaceId) {
    search.set("workspace_id", input.workspaceId);
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
    mutationFn: (kind: Exclude<MarketplaceKind, "bundle"> | undefined) =>
      refreshMarketplaceCatalog(kind),
    onSettled: () => invalidateMarketplace(queryClient),
  });
}

export function useInstallMarketplaceMCP() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: MCPInstallRequest) => installMarketplaceMCP(body),
    onSuccess: (result, variables) => {
      const installedPath = mcpInstalledPath({
        entryId: variables.entry_id,
        scope: result.mcp_server?.scope ?? variables.scope,
        server: result.mcp_server?.name ?? variables.name ?? "",
        workspaceId: result.mcp_server?.workspace_id ?? variables.workspace_id,
      });
      if (result.next_step === "authorize") {
        toast.success(
          `${result.mcp_server?.name ?? variables.name ?? "MCP server"} installed · authorization pending`,
          mcpToastAction("Authorize →", installedPath)
        );
        return;
      }
      toast.success(
        `${result.mcp_server?.name ?? variables.name ?? "MCP server"} installed`,
        mcpToastAction("View installed →", installedPath)
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

export function usePreviewMarketplaceBundle() {
  return useMutation({
    mutationFn: (body: BundlePreviewRequest) => previewMarketplaceBundle(body),
  });
}

export function useActivateMarketplaceBundle() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: BundleActivationRequest) => activateMarketplaceBundle(body),
    onSettled: () => invalidateMarketplace(queryClient),
  });
}
