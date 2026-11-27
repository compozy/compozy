import { useEffect } from "react";

import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import { markAgentInstanceSeen, type AgentInstanceRef } from "@/systems/agent";

/** Leaving is what marks the visit seen, so the digest stays put while you read it. */
export function useMarkSeenOnLeave(
  instance: AgentInstanceRef | undefined,
  active: boolean,
  computerId?: ComputerId
) {
  const id = instance?.id;
  const profile_name = instance?.profile_name;
  useEffect(() => {
    if (!active || !id || !profile_name) return undefined;
    const openedAt = Date.now();
    return () => {
      // A remount in the same tick isn't leaving.
      if (Date.now() - openedAt > 1000) {
        const target = { id, profile_name };
        const request = isRemoteComputerId(computerId)
          ? markAgentInstanceSeen(target, computerId)
          : markAgentInstanceSeen(target);
        void request.catch(() => undefined);
      }
    };
  }, [active, id, profile_name, computerId]);
}
