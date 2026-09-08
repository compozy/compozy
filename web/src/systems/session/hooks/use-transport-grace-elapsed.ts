import { useEffect, useState } from "react";

import { SESSION_TRANSPORT_GRACE_MS, transportGraceElapsed } from "../lib/session-transport";

/**
 * Whether the degraded phase has outlasted the grace (US-018.EC-1). One timer
 * armed at `degradedAt + grace`; a blip that heals first never flips it.
 */
export function useTransportGraceElapsed(degradedAt: number | null): boolean {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (degradedAt === null) return;
    const remaining = Math.max(0, degradedAt + SESSION_TRANSPORT_GRACE_MS - Date.now());
    const timer = window.setTimeout(() => setNow(Date.now()), remaining);
    return () => window.clearTimeout(timer);
  }, [degradedAt]);
  return transportGraceElapsed(degradedAt, Math.max(now, degradedAt ?? 0));
}
