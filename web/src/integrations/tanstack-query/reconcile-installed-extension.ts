import type { QueryClient } from "@tanstack/react-query";

import { extensionKeys } from "@/systems/extensions/lib/query-keys";
import { marketplaceKeys } from "@/systems/marketplace/lib/query-keys";
import { sessionKeys } from "@/systems/session/lib/query-keys";

export async function reconcileInstalledExtensionCaches(queryClient: QueryClient) {
  const queryKeys = [extensionKeys.all, marketplaceKeys.all, sessionKeys.commandsRoot];
  // Invalidating an initial fetch alone can reuse its pre-mutation response.
  await Promise.all(queryKeys.map(queryKey => queryClient.cancelQueries({ queryKey })));
  return Promise.all(queryKeys.map(queryKey => queryClient.invalidateQueries({ queryKey })));
}
