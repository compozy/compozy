import type { StateGlyphState } from "@compozy/ui";

import type { SessionPayload } from "@/systems/session";

export interface TabSignal {
  state: StateGlyphState;
  /** Accessible name: the glyph stands alone, without a visible label. */
  label: string;
}

/** Session badge → the tab's signal; accent marks attention only. */
export function sessionSignal(session: SessionPayload | undefined): TabSignal | null {
  switch (session?.badge) {
    case "running":
      return { state: "running", label: "Session running" };
    case "failed":
      return { state: "failed", label: "Session failed" };
    case "waiting-for-auth":
      return { state: "attention", label: "Session needs input" };
    case "hung":
    case "needs-attention":
    case "unhealthy":
      return { state: "attention", label: "Session needs attention" };
    default:
      return null;
  }
}

const SIGNAL_RANK: Record<string, number> = { attention: 0, failed: 1, running: 2 };

/** The loudest signal among a tab's members: needs you, then failed, then working. */
export function tabSignal(signals: readonly (TabSignal | null)[]): TabSignal | null {
  let loudest: TabSignal | null = null;
  for (const signal of signals) {
    if (signal === null) continue;
    if (loudest === null || (SIGNAL_RANK[signal.state] ?? 9) < (SIGNAL_RANK[loudest.state] ?? 9)) {
      loudest = signal;
    }
  }
  return loudest;
}
