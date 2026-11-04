import type {
  SessionRuntimeEffective,
  SessionRuntimePayload,
  SessionRuntimeSelection,
} from "../types";

type RuntimeIdentity = Pick<
  SessionRuntimeEffective,
  "provider" | "model" | "reasoning_effort" | "speed"
>;

/**
 * The selection already holds on the runtime: same provider, and every model,
 * effort, and speed the selection names. A field the selection leaves empty
 * resolves to a default the payload can't compare, so it never reads as pending.
 */
export function runtimeSelectionMatches(
  selected: Pick<SessionRuntimeSelection, "provider" | "model" | "reasoning_effort" | "speed">,
  runtime: RuntimeIdentity
): boolean {
  return (
    runtime.provider === selected.provider &&
    (!selected.model || runtime.model === selected.model) &&
    (!selected.reasoning_effort || runtime.reasoning_effort === selected.reasoning_effort) &&
    (selected.speed ?? "normal") === (runtime.speed ?? "normal")
  );
}

/**
 * A selection that clears model or effort resolves to a default the payload can't compare,
 * so a recorded runtime failure is the only evidence that the live runtime refused it.
 */
export function selectionResetRefused(
  selected: Pick<SessionRuntimeSelection, "model" | "reasoning_effort">,
  failure: string | undefined
): boolean {
  return (!selected.model || !selected.reasoning_effort) && !!failure?.trim();
}

export interface SessionRuntimeProjection {
  /** What the session runs now, or last ran; absent before the first bind. */
  current: RuntimeIdentity | undefined;
  /** What the next message runs, only while it differs from `current`. */
  next: RuntimeIdentity | undefined;
}

/**
 * One reading of a session's runtime for every surface that names it: the runtime
 * it is bound to (or last recorded) now, and the saved selection the next message
 * switches to while that selection has not reached the runtime yet. A provider
 * switch waits for the next message, so during a turn the two differ (#1290).
 */
export function sessionRuntimeProjection(
  runtime: SessionRuntimePayload | null | undefined
): SessionRuntimeProjection {
  const wasBound = !!runtime?.generation || !!runtime?.acp_session_id;
  const recorded = runtime?.effective ?? (wasBound ? runtime?.last_known : undefined);
  const selected = runtime?.selected ?? undefined;
  if (!recorded) return { current: selected ?? runtime?.last_known ?? undefined, next: undefined };
  const pending =
    !!selected?.provider &&
    (!runtimeSelectionMatches(selected, recorded) ||
      (!!runtime?.effective && selectionResetRefused(selected, runtime.failure)));
  return { current: recorded, next: pending ? selected : undefined };
}
