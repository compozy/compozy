import { useAtom } from "@xstate/store-react";
import { useContext } from "react";

import { useDocumentVisible } from "@/hooks/use-document-visible";

import { WindowLiveDataContext } from "../contexts/window-live-data-context";
import { settledLiveDataOwners } from "../lib/live-data-lease";
import { useOsShell } from "./use-os-shell";

export { LIVE_DATA_SETTLE_MS } from "../lib/live-data-lease";

/** Defaults to live outside the retained-window OS shell. */
export function useCurrentWindowLiveDataEnabled(): boolean {
  return useContext(WindowLiveDataContext);
}

/**
 * Bounds retained-window streams while keeping one visible background window live. The lease
 * follows focus once it has held for `LIVE_DATA_SETTLE_MS` (`settledLiveDataOwners`); a hidden
 * document pauses every window at once.
 */
export function useWindowLiveDataEnabled(windowId: string): boolean {
  const documentVisible = useDocumentVisible();
  const { projection } = useOsShell();
  const windowLive = useAtom(settledLiveDataOwners(projection), owners =>
    owners.includes(windowId)
  );
  return documentVisible && windowLive;
}
