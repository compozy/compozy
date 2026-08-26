import { useQuery } from "@tanstack/react-query";

import type { SymbolIconOption } from "@compozy/ui";

import { profileIconCatalogOptions } from "../lib/query-options";

const EMPTY_CATALOG: readonly SymbolIconOption[] = [];

/** The lazily loaded Lucide catalog the identity picker offers. */
export function useProfileIconCatalog(): {
  icons: readonly SymbolIconOption[];
  loading: boolean;
} {
  const { data, isPending } = useQuery(profileIconCatalogOptions());
  return { icons: data ?? EMPTY_CATALOG, loading: isPending };
}
