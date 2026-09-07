import { useEffect, useState } from "react";

import { thinkingGuardRemainingMs } from "@/systems/session";

/**
 * Whether the thinking flicker guard has elapsed since the send (US-023.EC-1).
 * One timer armed at `sentAt + guard`; a first token that lands earlier means
 * the thinking frame never shows. `null` (no send in flight) reads elapsed.
 */
export function useThinkingGuardElapsed(sentAtMs: number | null): boolean {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (sentAtMs === null) return;
    const remaining = thinkingGuardRemainingMs(sentAtMs, Date.now());
    // A delayed effect still has to publish the elapsed clock to the render.
    // A zero-delay timer also keeps that update cancellable during replay.
    const timer = window.setTimeout(() => setNow(Date.now()), remaining);
    return () => window.clearTimeout(timer);
  }, [sentAtMs]);
  if (sentAtMs === null) return true;
  return thinkingGuardRemainingMs(sentAtMs, Math.max(now, sentAtMs)) === 0;
}
