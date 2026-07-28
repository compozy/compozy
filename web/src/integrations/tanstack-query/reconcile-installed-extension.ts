import type { QueryClient } from "@tanstack/react-query";

import { extensionKeys } from "@/systems/extensions/lib/query-keys";
import { marketplaceKeys } from "@/systems/marketplace/lib/query-keys";

export function reconcileInstalledExtensionCaches(queryClient: QueryClient) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: extensionKeys.all }),
    queryClient.invalidateQueries({ queryKey: marketplaceKeys.all }),
  ]);
}
