import { ListingPage } from "@compozy/ui";

import {
  ToolbarFilterSkeleton,
  ToolbarSearchSkeleton,
  ToolbarSlotSkeleton,
  ToolbarViewToggleSkeleton,
} from "../../components/os-window-toolbar-skeleton";
import type { OsWindowRoute } from "../../lib/os-types";
import { validateVaultSearch, VaultSecretsList, VaultSecurityNote } from "@/systems/vault";

/**
 * The vault window's body while its code loads: the strip its page will publish (search,
 * namespace filter and the cards/rows toggle, which stay put while the list loads), the security
 * note, and the list's loading rows or cards.
 */
export function VaultWindowFallback({ route }: { route: OsWindowRoute }) {
  const { view } = validateVaultSearch(route.search);
  return (
    <>
      <ToolbarSlotSkeleton
        leading={
          <>
            <ToolbarSearchSkeleton />
            <ToolbarFilterSkeleton />
          </>
        }
        trailing={<ToolbarViewToggleSkeleton />}
      />
      <ListingPage data-testid="vault-shell">
        <VaultSecurityNote />
        <VaultSecretsList isLoading secrets={[]} view={view} />
      </ListingPage>
    </>
  );
}
