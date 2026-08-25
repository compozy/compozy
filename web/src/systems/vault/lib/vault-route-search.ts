import type { ListingViewMode } from "@compozy/ui";

import { normalizeListingSearchValue, parseListingView } from "@/lib/listing-search";
import { VAULT_NAMESPACES, type VaultNamespace } from "../types";

export type VaultNamespaceFilter = VaultNamespace | "all";

export interface VaultRouteSearch {
  q?: string;
  namespace?: VaultNamespace;
  ref?: string;
  view?: ListingViewMode;
}

export function normalizeVaultPrefixForNamespace(
  value: unknown,
  namespace: VaultNamespace | undefined
): string | undefined {
  const prefix = normalizeListingSearchValue(value);
  if (!prefix || !namespace) return prefix;

  const match = /^vault:([^/]+)\//.exec(prefix);
  return match && match[1] !== namespace ? undefined : prefix;
}

export function parseVaultNamespaceFilter(value: unknown): VaultNamespace | undefined {
  if (typeof value !== "string") return undefined;
  return (VAULT_NAMESPACES as readonly string[]).includes(value)
    ? (value as VaultNamespace)
    : undefined;
}

export function validateVaultSearch(search: Record<string, unknown>): VaultRouteSearch {
  const namespace = parseVaultNamespaceFilter(search.namespace);
  const ref = typeof search.ref === "string" ? search.ref.trim() || undefined : undefined;
  return {
    q: normalizeVaultPrefixForNamespace(search.q, namespace),
    namespace,
    ref,
    view: parseListingView(search.view),
  };
}
