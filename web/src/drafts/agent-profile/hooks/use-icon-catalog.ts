import { useEffect, useState } from "react";

import type { SymbolIconOption } from "@compozy/ui";

import { loadProfileIconCatalog } from "@/systems/profiles/lib/profile-identity";

/** The Lucide catalog, loaded locally because the production hook reads through TanStack Query. */
export function useIconCatalog() {
  const [icons, setIcons] = useState<readonly SymbolIconOption[]>([]);
  useEffect(() => {
    let live = true;
    void loadProfileIconCatalog().then(next => {
      if (live) setIcons(next);
    });
    return () => {
      live = false;
    };
  }, []);
  return icons;
}
