import { useEffect } from "react";

import { markAgentInstanceSeen } from "@/systems/agent";

/** Leaving is what marks the visit seen, so the digest stays put while you read it. */
export function useMarkSeenOnLeave(instanceId: string, active: boolean) {
  useEffect(() => {
    if (!active) return undefined;
    const openedAt = Date.now();
    return () => {
      // A remount in the same tick isn't leaving.
      if (Date.now() - openedAt > 1000) {
        void markAgentInstanceSeen(instanceId).catch(() => undefined);
      }
    };
  }, [active, instanceId]);
}
