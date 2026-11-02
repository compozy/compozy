import { useEffect } from "react";

import { markAgentInstanceSeen, type AgentInstanceRef } from "@/systems/agent";

/** Leaving is what marks the visit seen, so the digest stays put while you read it. */
export function useMarkSeenOnLeave(instance: AgentInstanceRef | undefined, active: boolean) {
  const id = instance?.id;
  const profile_name = instance?.profile_name;
  useEffect(() => {
    if (!active || !id || !profile_name) return undefined;
    const openedAt = Date.now();
    return () => {
      // A remount in the same tick isn't leaving.
      if (Date.now() - openedAt > 1000) {
        void markAgentInstanceSeen({ id, profile_name }).catch(() => undefined);
      }
    };
  }, [active, id, profile_name]);
}
