import { useQuery } from "@tanstack/react-query";

import {
  bundleActivationOptions,
  bundleActivationsOptions,
  extensionProvenanceOptions,
  extensionsListOptions,
} from "../lib/query-options";
import type { InstalledExtensionView } from "../types";

export function useExtensionInventory() {
  const local = useQuery(extensionsListOptions());
  const items: InstalledExtensionView[] = (local.data ?? []).map(extension => {
    const listing = extension.marketplace ?? null;
    return { extension, listing, updateAvailable: listing?.update_available === true };
  });

  return { ...local, data: items };
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
