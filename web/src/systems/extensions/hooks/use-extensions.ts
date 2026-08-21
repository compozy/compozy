import { useQuery } from "@tanstack/react-query";

import {
  extensionInventoryOptions,
  extensionProvenanceOptions,
  extensionsListOptions,
} from "../lib/query-options";
import type { InstalledExtensionView } from "../types";
import { useActiveWorkspace } from "@/systems/workspace";
import { useProfileReadScope } from "@/systems/profiles";

/**
 * The active workspace selects the dev overlay when one is linked, while the active profile selects
 * the profile-owned inventory within that instance.
 */
export function useExtensionInstanceScope(): {
  workspaceId: string | null;
  profileName: string;
} {
  const { activeWorkspaceId } = useActiveWorkspace();
  const { destination } = useProfileReadScope();
  return { profileName: destination, workspaceId: activeWorkspaceId ?? null };
}

export function useExtensionInventory(enabled = true) {
  const scope = useExtensionInstanceScope();
  const local = useQuery(extensionsListOptions(scope, enabled));
  const items: InstalledExtensionView[] = (local.data ?? []).map(extension => {
    const listing = extension.marketplace ?? null;
    return {
      extension,
      listing,
      updateAvailable: extension.update_available === true || listing?.update_available === true,
    };
  });

  return {
    ...local,
    data: items,
    profileName: scope.profileName,
    workspaceId: scope.workspaceId,
  };
}

export function useExtensionDetail(name: string) {
  const inventory = useExtensionInventory();
  return { ...inventory, data: inventory.data.find(item => item.extension.name === name) ?? null };
}

export function useExtensionProvenance(name: string, enabled = true) {
  return useQuery(extensionProvenanceOptions(name, enabled));
}

/** Shipped-vs-live kit resources for the global published extension instance. */
export function useExtensionKitInventory(name: string, enabled = true) {
  return useQuery(extensionInventoryOptions(name, enabled));
}
