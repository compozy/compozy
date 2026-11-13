"use client";

import { useEffect, useEffectEvent, useRef, useState } from "react";

/**
 * An id-less window that found the project at its cap. `waiting` opens the
 * terminal as soon as a slot frees, `claimed` took that slot and is opening it,
 * and `dismissed` gave up and shows the empty state. Keyed to the arrival it
 * belongs to, so it never leaks across scopes.
 */
interface LimitWait {
  key: string;
  status: "waiting" | "claimed" | "dismissed";
  /** The limit dialog's own Close was used: the dismissal that follows keeps the wait. */
  closeRequested: boolean;
  /** The host's refusal when the slot was claimed; only a later one belongs to this claim. */
  refusalAtClaim?: number | null;
}

export interface UseTerminalLimitWaitOptions {
  /** The window sits on the id-less `/terminal` route and may open one itself. */
  awaitingRoute: boolean;
  /** One arrival at that route in one scope. */
  resolutionKey: string;
  atLimit: boolean;
  setLimitOpen: (open: boolean) => void;
  onOpenTerminal: (() => void) | undefined;
  /** Identifies the host's last refused create for this window; `null` while none is refused. */
  openRefusal: number | null;
  onCloseTerminal: (terminalId: string) => void;
}

export interface TerminalLimitWait {
  /** The window gave up waiting; it shows the empty state instead of resolving. */
  dismissed: boolean;
  /** The resolver met the cap: wait behind the dialog for a slot. */
  wait: () => void;
  /** The limit dialog opened or closed; closing it without freeing a slot dismisses. */
  changeLimitOpen: (open: boolean) => void;
  /** The limit dialog's Close: frees a slot this window may be waiting for. */
  closeLimitTerminal: (terminalId: string) => void;
}

/**
 * Waits for a slot at the terminal cap. The live catalog is the signal: a close
 * from the dialog, another window or the CLI drops the running count, and the
 * waiting window claims the slot and opens its terminal once.
 */
export function useTerminalLimitWait({
  awaitingRoute,
  resolutionKey,
  atLimit,
  setLimitOpen,
  onOpenTerminal,
  openRefusal,
  onCloseTerminal,
}: UseTerminalLimitWaitOptions): TerminalLimitWait {
  const [limitWait, setLimitWait] = useState<LimitWait | null>(null);
  const current = awaitingRoute && limitWait?.key === resolutionKey ? limitWait : null;
  // Adjusted during render: a wait belongs to one arrival, and a freed slot is
  // claimed before the dialog could show over a window that already took it.
  if (limitWait !== null && current === null) setLimitWait(null);
  if (current?.status === "waiting" && !atLimit) {
    setLimitWait({ ...current, status: "claimed", refusalAtClaim: openRefusal });
    setLimitOpen(false);
  }
  const claimed = current?.status === "claimed" ? current : null;

  // Creating the terminal is the daemon synchronization; once per claim.
  const lastClaimRef = useRef<LimitWait | null>(null);
  const openClaimedSlot = useEffectEvent(() => {
    onOpenTerminal?.();
  });
  useEffect(() => {
    if (claimed === null || lastClaimRef.current === claimed) return;
    lastClaimRef.current = claimed;
    openClaimedSlot();
  }, [claimed]);

  return {
    // A refused create (another window took the slot first) ends the wait on the empty state.
    // An earlier refusal still on the host is not this claim's answer.
    dismissed:
      current?.status === "dismissed" ||
      (claimed !== null && openRefusal !== null && openRefusal !== claimed.refusalAtClaim),
    wait: () => {
      if (awaitingRoute) {
        setLimitWait({ key: resolutionKey, status: "waiting", closeRequested: false });
      }
    },
    changeLimitOpen: open => {
      setLimitOpen(open);
      if (open) return;
      // The dialog's own Close keeps the wait once; any other dismissal ends it.
      setLimitWait(wait => {
        if (wait?.status !== "waiting") return wait;
        return wait.closeRequested
          ? { ...wait, closeRequested: false }
          : { ...wait, status: "dismissed" };
      });
    },
    closeLimitTerminal: terminalId => {
      setLimitWait(wait => (wait?.status === "waiting" ? { ...wait, closeRequested: true } : wait));
      onCloseTerminal(terminalId);
    },
  };
}
