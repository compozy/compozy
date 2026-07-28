import { validateVaultSearch, VaultPage } from "@/systems/vault";
import { useDesktop } from "../../hooks/use-desktop";

const EMPTY_SEARCH: Record<string, unknown> = {};

/**
 * Vault app controller driven exclusively by the logical window's WM location.
 * The selector returns the store-owned search reference — validating inside
 * it would mint a fresh snapshot per read and loop `useSyncExternalStore`
 * under write bursts (the 12-window restore storm).
 */
export function VaultWindow({ windowId }: { windowId: string }) {
  const rawSearch = useDesktop(state => state.windows[windowId]?.route.search ?? EMPTY_SEARCH);
  const search = validateVaultSearch(rawSearch);
  return <VaultPage search={search} />;
}
