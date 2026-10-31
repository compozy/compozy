import type { SessionPayload } from "../types";
import type { RuntimeProviderLiveApply } from "@/systems/runtime";

export interface RuntimeApplyNotice {
  text: string;
  tone: "neutral" | "danger";
}

/** Badges that mean the session still has a turn in flight. */
const TURN_ACTIVE_BADGES = new Set<string>([
  "running",
  "waiting-for-input",
  "waiting-for-auth",
  "hung",
  "unhealthy",
]);

type RuntimeApplySession = Pick<SessionPayload, "badge" | "runtime">;

/**
 * When a saved runtime selection reaches the agent, from the daemon's answer:
 * the effective runtime shows whether it was applied to the live process, and the
 * provider's live-apply capability says whether a running turn picks it up.
 * An idle session that applied the change needs no note.
 */
export function runtimeApplyNotice(
  session: RuntimeApplySession,
  liveApply: RuntimeProviderLiveApply | undefined
): RuntimeApplyNotice | null {
  const { selected, effective, failure } = session.runtime;
  if (!selected) return null;
  const applied =
    effective !== undefined &&
    effective !== null &&
    effective.provider === selected.provider &&
    (!selected.model || effective.model === selected.model) &&
    (!selected.reasoning_effort || effective.reasoning_effort === selected.reasoning_effort) &&
    (selected.speed ?? "normal") === (effective.speed ?? "normal");
  if (!applied) {
    return failure?.trim()
      ? { text: "Couldn't apply this now. It retries with the next message.", tone: "danger" }
      : { text: "Applies from the next message.", tone: "neutral" };
  }
  if (!TURN_ACTIVE_BADGES.has(session.badge)) return null;
  return liveApply === "next_step"
    ? { text: "Applies at the agent's next step.", tone: "neutral" }
    : { text: "Applies from the next message.", tone: "neutral" };
}
