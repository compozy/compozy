import { createContext, useContext } from "react";

import { LOCAL_COMPUTER_ID, normalizeComputerId, type ComputerId } from "./computer-scope";

/**
 * Ambient computer identity for the active session/surface.
 *
 * Defaults to `"local"` outside a remote session window. Components that hide
 * client-OS actions (Open in editor, Reveal in Finder, native folder picker)
 * or show remote provenance (`ComputerChip`, offline banner) read this context
 * without introducing cross-system circular imports.
 */
export const ComputerScopeContext = createContext<ComputerId>(LOCAL_COMPUTER_ID);

export function useCurrentComputerId(explicitComputerId?: ComputerId): ComputerId {
  const ambient = useContext(ComputerScopeContext);
  return normalizeComputerId(explicitComputerId ?? ambient);
}
