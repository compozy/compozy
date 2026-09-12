import type { SessionContextPayload, SessionUsagePayload } from "../types";

export interface SessionContextView extends SessionContextPayload {
  warning: boolean;
  loading: boolean;
  stopped: boolean;
  display?: { compozy: number; agent: number; free: number; total: number };
  estimateExceedsReported: boolean;
}

/** Observation fields are sequenced; attribution and policy may change without a new report. */
export function retainSessionUsage(
  previous: SessionUsagePayload | undefined,
  incoming: SessionUsagePayload | undefined
): SessionUsagePayload | undefined {
  if (!incoming) return previous;
  if (incoming.context.state === "unavailable") {
    return previous ? { ...incoming, context: previous.context } : incoming;
  }
  if (!previous) return incoming;
  const old = previous.context;
  const next = incoming.context;
  if (old.sequence != null && (next.sequence == null || next.sequence <= old.sequence)) {
    return {
      ...incoming,
      context: {
        ...old,
        injected: next.injected,
        pressure_threshold: next.pressure_threshold,
        // Freshness can change when a later turn settles without a new report.
        stale: next.sequence === old.sequence ? (next.stale ?? old.stale) : old.stale,
      },
    };
  }
  return incoming;
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
    warning:
      value.ratio != null &&
      value.pressure_threshold != null &&
      value.size_source === "agent" &&
      value.ratio >= value.pressure_threshold,
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
  | "warning"
  | "estimated"
  | "reported";

/** Shape carries freshness (dotted = stale, dashed = unknown); hue carries pressure only. */
export function sessionContextRingState(context: SessionContextView): SessionContextRingState {
  if (context.loading && context.used == null) return "loading";
  if (context.used == null) return "unknown";
  if (context.ratio == null) return "used-only";
  if (context.stale) return "stale";
  if (context.warning) return "warning";
  if (context.size_source === "catalog") return "estimated";
  return "reported";
}
