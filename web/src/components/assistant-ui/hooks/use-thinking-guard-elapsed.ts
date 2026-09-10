import { useEffect, useState } from "react";

import { thinkingGuardRemainingMs } from "@/systems/session";

/**
 * Whether the thinking flicker guard has elapsed since the send (US-023.EC-1).
 * A timer tracks `sentAt + guard`; a first token that lands earlier means
 * the thinking frame never shows. `null` (no send in flight) reads elapsed.
 */
export function useThinkingGuardElapsed(sentAtMs: number | null): boolean {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (sentAtMs === null) return;
    let timer: number;
    const tick = () => {
      const current = Date.now();
      setNow(current);
      const remaining = thinkingGuardRemainingMs(sentAtMs, current);
      // Timer scheduling and the wall clock can round differently. Re-arm an
      // early callback so the pending frame cannot remain hidden indefinitely.
      if (remaining > 0) timer = window.setTimeout(tick, remaining);
    };
    // Publish even when commit work delayed this effect past the deadline.
    timer = window.setTimeout(tick, thinkingGuardRemainingMs(sentAtMs, Date.now()));
    return () => window.clearTimeout(timer);
  }, [sentAtMs]);
  if (sentAtMs === null) return true;
  return thinkingGuardRemainingMs(sentAtMs, Math.max(now, sentAtMs)) === 0;
}
