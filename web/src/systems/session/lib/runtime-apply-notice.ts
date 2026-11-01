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

/** The live session as the composer currently sees it; undefined until it is mounted. */
export interface RuntimeApplyLiveSession {
  badge: SessionPayload["badge"] | undefined;
  runtime: SessionRuntime | undefined;
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
  const { selected, effective, failure } = runtime;
  // A field left empty resolves to a default the payload can't compare, so a recorded
  // failure is the only evidence that clearing it was refused.
  if (selected && (!selected.model || !selected.reasoning_effort) && failure?.trim()) return false;
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
  const { selected, effective, failure, status } = session.runtime;
  if (!selected) return null;
  if (!selectionApplied(session.runtime)) {
    // Only a ready same-provider session_config runtime attempts a live apply; any other
    // failure belongs to an earlier bind, not to this deferred selection.
    const refused =
      !!failure?.trim() &&
      liveApply !== undefined &&
      status === "ready" &&
      effective?.provider === selected.provider;
    return refused
      ? { text: "Couldn't apply this now. It retries with the next message.", tone: "danger" }
      : { text: "Applies from the next message.", tone: "neutral" };
  }
  if (!TURN_ACTIVE_BADGES.has(session.badge)) return null;
  return liveApply === "next_step"
    ? { text: "Applies at the agent's next step.", tone: "neutral" }
    : { text: "Applies from the next message.", tone: "neutral" };
}

/** The session state this save is judged against; null once a newer selection replaced it. */
function reportedSession(
  target: RuntimeApplyTarget,
  live: RuntimeApplyLiveSession
): RuntimeApplySession | null {
  const revision = target.saved.runtime.selection_revision;
  if (live.runtime && live.badge !== undefined) {
    if (live.runtime.selection_revision > revision) return null;
    if (live.runtime.selection_revision === revision) {
      return { badge: live.badge, runtime: live.runtime };
    }
  }
  return target.saved;
}

/**
 * The notice for a saved selection, re-derived from the live session once it reports
 * that save. The "takes effect" note belongs to the turn the change was saved during;
 * a change that lands later (idle save, retried refusal) has nothing left to say.
 */
export function currentRuntimeApplyNotice(
  target: RuntimeApplyTarget,
  live: RuntimeApplyLiveSession
): RuntimeApplyNotice | null {
  const session = reportedSession(target, live);
  if (!session) return null;
  const savedDuringTurn =
    selectionApplied(target.saved.runtime) && TURN_ACTIVE_BADGES.has(target.saved.badge);
  if (selectionApplied(session.runtime) && !savedDuringTurn) return null;
  return runtimeApplyNotice(session, target.liveApply);
}

/** True once the save has nothing left to report, so a later turn cannot revive its note. */
export function runtimeApplyTargetSettled(
  target: RuntimeApplyTarget,
  live: RuntimeApplyLiveSession
): boolean {
  const session = reportedSession(target, live);
  return !session || (selectionApplied(session.runtime) && !TURN_ACTIVE_BADGES.has(session.badge));
}
