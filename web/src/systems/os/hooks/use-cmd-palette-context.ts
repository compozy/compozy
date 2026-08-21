import { useEffect, useEffectEvent, useState } from "react";
import { shallowEqual } from "@xstate/store";

import {
  CMD_PALETTE_CONTEXT_KEYS,
  buildCmdPaletteContextSnapshot,
  sameContextSnapshot,
  type CmdPaletteContextSnapshot,
} from "../lib/cmd-palette-context";
import { useDesktop } from "./use-desktop";

/**
 * A reconnect storm can flip window topology several times inside one burst.
 * Rows must not strobe between enabled and disabled while that settles, so the
 * snapshot the evaluator reads lags the raw state by one quiet window
 * (US-037.EC-2).
 */
export const CMD_PALETTE_CONTEXT_DEBOUNCE_MS = 120;

function contextRevision(snapshot: CmdPaletteContextSnapshot | null): string {
  if (snapshot === null) return "unattached";
  return JSON.stringify(CMD_PALETTE_CONTEXT_KEYS.map(key => snapshot[key]));
}

export interface UseCmdPaletteContextOptions {
  readonly shellDesktop: boolean;
  readonly scopeGlobal: boolean;
  readonly workspaceTrusted: boolean;
  readonly focusedSessionState: string;
  /** Null while no client is attached — the evaluator then refuses, never allows. */
  readonly attached: boolean;
  readonly debounceMs?: number;
}

/** The client's volatile context snapshot, debounced against availability flaps. */
export function useCmdPaletteContext({
  shellDesktop,
  scopeGlobal,
  workspaceTrusted,
  focusedSessionState,
  attached,
  debounceMs = CMD_PALETTE_CONTEXT_DEBOUNCE_MS,
}: UseCmdPaletteContextOptions): CmdPaletteContextSnapshot | null {
  const topology = useDesktop(
    state => ({
      activeDesktopId: state.activeDesktopId,
      focusedId: state.focusedId,
      frames: state.frames,
      windows: state.windows,
    }),
    shallowEqual
  );
  const live = attached
    ? buildCmdPaletteContextSnapshot(topology, {
        shellDesktop,
        scopeGlobal,
        workspaceTrusted,
        focusedSessionState,
      })
    : null;
  const [settled, setSettled] = useState<CmdPaletteContextSnapshot | null>(live);
  const commitLive = useEffectEvent(() => {
    setSettled(current => (sameContextSnapshot(current, live) ? current : live));
  });
  const revision = contextRevision(live);

  useEffect(() => {
    if (!attached) return undefined;
    const timer = setTimeout(commitLive, debounceMs);
    return () => clearTimeout(timer);
  }, [attached, debounceMs, revision]);

  return attached ? settled : null;
}
