import { useQuery } from "@tanstack/react-query";

import { useActiveWorkspace } from "@/systems/workspace";

import {
  bundleActivationOptions,
  bundleActivationsOptions,
  extensionProvenanceOptions,
  extensionsListOptions,
} from "../lib/query-options";
import type { InstalledExtensionView } from "../types";

/**
 * The active workspace selects which daemon instance a name resolves to: the workspace dev overlay
 * when one is linked, the global published row otherwise.
 */
export function useExtensionInstanceScope(): { workspaceId: string | null } {
  const { activeWorkspaceId } = useActiveWorkspace();
  return { workspaceId: activeWorkspaceId ?? null };
}

export function useExtensionInventory() {
  const scope = useExtensionInstanceScope();
  const local = useQuery(extensionsListOptions(scope));
  const items: InstalledExtensionView[] = (local.data ?? []).map(extension => {
    const listing = extension.marketplace ?? null;
    return {
      extension,
      listing,
      updateAvailable: extension.update_available === true || listing?.update_available === true,
    };
  });

  return { ...local, data: items, workspaceId: scope.workspaceId };
}

export function useExtensionDetail(name: string) {
  const inventory = useExtensionInventory();
  return { ...inventory, data: inventory.data.find(item => item.extension.name === name) ?? null };
}

export function useExtensionProvenance(name: string, enabled = true) {
  return useQuery(extensionProvenanceOptions(name, enabled));
}

export function useBundleActivations() {
  return useQuery(bundleActivationsOptions());
}

export function useBundleActivation(id: string) {
  return useQuery(bundleActivationOptions(id));
}
