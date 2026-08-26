import { useQuery } from "@tanstack/react-query";

import type { SymbolIconOption } from "@compozy/ui";

import { profileIconCatalogOptions } from "../lib/query-options";

const EMPTY_CATALOG: readonly SymbolIconOption[] = [];

export interface ProfileIconCatalogViewModel {
  icons: readonly SymbolIconOption[];
  loading: boolean;
}

/** The lazily loaded Lucide catalog the identity picker offers. */
export function useProfileIconCatalog(enabled = true): ProfileIconCatalogViewModel {
  const { data, isPending } = useQuery({ ...profileIconCatalogOptions(), enabled });
  return { icons: data ?? EMPTY_CATALOG, loading: enabled && isPending };
}
