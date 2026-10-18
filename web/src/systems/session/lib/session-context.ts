import type { SessionContextPayload, SessionUsagePayload } from "../types";

export interface SessionContextView extends SessionContextPayload {
  loading: boolean;
  stopped: boolean;
  display?: { compozy: number; agent: number; free: number; total: number };
  estimateExceedsReported: boolean;
}

/**
 * The compaction the daemon says emptied this reading (`context.cleared_by`), present only
 * while the occupancy is unknown and awaiting the agent's next report. A reading that is
 * empty for any other reason (never reported) has no cause.
 */
export function contextClearedBy(
  context: SessionContextPayload
): NonNullable<SessionContextPayload["cleared_by"]> | null {
  return context.state === "unknown" && context.used == null ? (context.cleared_by ?? null) : null;
}

/** The usage reading a window keeps across reads, plus what the daemon invalidated. */
export interface RetainedSessionUsage {
  usage?: SessionUsagePayload;
  /**
   * Highest report sequence the daemon invalidated (a terminal compaction, a rebuilt context).
   * A report at or below it was taken before that boundary, so it is stale, not fresh.
   */
  invalidatedThrough?: number;
}

/**
 * Observation fields are sequenced; attribution and policy may change without a new report.
 *
 * An `unknown` read is the daemon's answer, not a gap: after a terminal compaction it clears
 * the reading until the agent's next usage report, so the previous one is never restored. The
 * boundary becomes the floor below which a late report is stale, so a response that predates
 * it cannot bring the old ratio back either: the daemon names the boundary
 * (`cleared_by.sequence`), and the cleared reading's own sequence covers any other clearing.
 */
export function retainSessionUsage(
  previous: RetainedSessionUsage | undefined,
  incoming: SessionUsagePayload | undefined
): RetainedSessionUsage {
  const kept = previous ?? {};
  if (!incoming) return kept;
  const old = kept.usage?.context;
  if (incoming.context.state === "unavailable") {
    return { ...kept, usage: kept.usage ? { ...incoming, context: kept.usage.context } : incoming };
  }
  if (incoming.context.state === "unknown") {
    const invalidatedThrough = Math.max(
      kept.invalidatedThrough ?? 0,
      old?.sequence ?? 0,
      incoming.context.cleared_by?.sequence ?? 0
    );
    return {
      usage: incoming,
      invalidatedThrough: invalidatedThrough > 0 ? invalidatedThrough : undefined,
    };
  }
  const next = incoming.context;
  const floor = kept.invalidatedThrough;
  if (floor != null && next.sequence != null && next.sequence <= floor) {
    // A report from before the invalidation: refresh attribution, keep the cleared reading.
    return { ...kept, usage: kept.usage ? { ...incoming, context: kept.usage.context } : incoming };
  }
  if (!old) return { usage: incoming };
  if (old.sequence != null && (next.sequence == null || next.sequence <= old.sequence)) {
    return {
      ...kept,
      usage: {
        ...incoming,
        context: {
          ...old,
          injected: next.injected,
          // Freshness can change when a later turn settles without a new report.
          stale: next.sequence === old.sequence ? (next.stale ?? old.stale) : old.stale,
        },
      },
    };
  }
  return { usage: incoming };
}

export function deriveSessionContext(
  context?: SessionContextPayload,
  options: { unavailable?: boolean; loading?: boolean; stopped?: boolean } = {}
): SessionContextView {
  const value = context ?? { state: "unknown" };
  const used = value.used;
  const size = value.size;
  const injected = value.injected?.tokens ?? 0;
  const boundedUsed = used != null && size != null && size > 0 ? Math.min(used, size) : undefined;
  const compozy = boundedUsed == null ? 0 : Math.min(injected, boundedUsed);
  return {
    ...value,
    state: options.unavailable ? "unavailable" : value.state,
    loading: options.loading ?? false,
    stopped: options.stopped ?? false,
    display:
      boundedUsed != null && size != null
        ? { compozy, agent: boundedUsed - compozy, free: size - boundedUsed, total: size }
        : undefined,
    estimateExceedsReported: used != null && injected > used,
  };
}

export type SessionContextRingState =
  | "loading"
  | "unknown"
  | "used-only"
  | "stale"
  | "estimated"
  | "reported";

/** Shape carries freshness: dotted = stale, dashed = unknown. */
export function sessionContextRingState(context: SessionContextView): SessionContextRingState {
  if (context.loading && context.used == null) return "loading";
  if (context.used == null) return "unknown";
  if (context.ratio == null) return "used-only";
  if (context.stale) return "stale";
  if (context.size_source === "catalog") return "estimated";
  return "reported";
}
