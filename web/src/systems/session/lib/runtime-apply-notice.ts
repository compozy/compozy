import type { SessionPayload } from "../types";
import { normalizeRuntimeACPSelections, type RuntimeProviderLiveApply } from "@/systems/runtime";

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
type SessionRuntime = SessionPayload["runtime"];

/** One saved selection whose effect the composer reports. */
export interface RuntimeApplyTarget {
  /** The daemon's answer to the save. */
  saved: RuntimeApplySession;
  liveApply: RuntimeProviderLiveApply | undefined;
}

/** Every option the selection names must hold that value on the effective runtime. */
function acpOptionsApplied(runtime: SessionRuntime): boolean {
  const selected = normalizeRuntimeACPSelections(runtime.selected?.acp_options);
  if (!selected) return true;
  const effective = normalizeRuntimeACPSelections(runtime.effective?.acp_options) ?? [];
  return selected.every(option =>
    effective.some(
      candidate =>
        candidate.id === option.id &&
        candidate.value_id === option.value_id &&
        candidate.bool_value === option.bool_value
    )
  );
}

function selectionApplied(runtime: SessionRuntime): boolean {
  const { selected, effective } = runtime;
  return (
    !!selected &&
    !!effective &&
    effective.provider === selected.provider &&
    (!selected.model || effective.model === selected.model) &&
    (!selected.reasoning_effort || effective.reasoning_effort === selected.reasoning_effort) &&
    (selected.speed ?? "normal") === (effective.speed ?? "normal") &&
    acpOptionsApplied(runtime)
  );
}

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
  if (!session.runtime.selected) return null;
  if (!selectionApplied(session.runtime)) {
    return session.runtime.failure?.trim()
      ? { text: "Couldn't apply this now. It retries with the next message.", tone: "danger" }
      : { text: "Applies from the next message.", tone: "neutral" };
  }
  if (!TURN_ACTIVE_BADGES.has(session.badge)) return null;
  return liveApply === "next_step"
    ? { text: "Applies at the agent's next step.", tone: "neutral" }
    : { text: "Applies from the next message.", tone: "neutral" };
}

/**
 * The notice for a saved selection, re-derived from the live session once it reports
 * that save: a finished turn or a later successful retry clears it, and a newer
 * selection from anywhere supersedes it.
 */
export function currentRuntimeApplyNotice(
  target: RuntimeApplyTarget,
  live: { badge: SessionPayload["badge"] | undefined; runtime: SessionRuntime | undefined }
): RuntimeApplyNotice | null {
  const revision = target.saved.runtime.selection_revision;
  if (live.runtime && live.badge !== undefined) {
    if (live.runtime.selection_revision > revision) return null;
    if (live.runtime.selection_revision === revision) {
      return runtimeApplyNotice({ badge: live.badge, runtime: live.runtime }, target.liveApply);
    }
  }
  return runtimeApplyNotice(target.saved, target.liveApply);
}
