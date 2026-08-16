import type {
  OperatorNotificationEventPayload,
  SessionAttentionEventPayload,
} from "@/systems/session";

import type { OsSessionAttentionRow } from "./attention-model";

/**
 * The delivery engine: it decides *whether* an attention edge becomes a
 * notification, and *how many* notifications a burst becomes. Rendering
 * (toast, sound, system) is the notifier's job — this module is pure policy
 * with injected timers, which is why every rule below is directly testable.
 *
 * The rules exist to defeat the two documented ways this feature class dies:
 * volume fatigue (a fleet finishing at once) and precision loss (a number that
 * stops meaning "you are blocked"). So completions coalesce, needs-you never
 * does, and the session you are already looking at stays silent.
 */

/** Absorbs any needs-you flap from the same session. */
export const NEEDS_YOU_DEDUP_MS = 5_000;
/** Completions inside one window become a single grouped delivery. */
export const COMPLETION_COALESCE_MS = 5_000;
/**
 * How long an edge waits for the catalog refetch that names its session. An
 * edge that never resolves is dropped rather than rendered with invented copy.
 */
export const RESOLVE_GRACE_MS = 5_000;

export type AttentionDeliveryTarget = Pick<
  OsSessionAttentionRow,
  "id" | "title" | "agentName" | "workspaceId" | "workspaceLabel" | "badge" | "reason"
>;

export interface NeedsYouDelivery {
  kind: "needs-you";
  key: string;
  target: AttentionDeliveryTarget;
}

export interface FinishedDelivery {
  kind: "finished";
  key: string;
  targets: AttentionDeliveryTarget[];
}

export interface AgentDelivery {
  kind: "agent";
  key: string;
  notification: OperatorNotificationEventPayload;
  target: AttentionDeliveryTarget | null;
}

export type AttentionDelivery = NeedsYouDelivery | FinishedDelivery | AgentDelivery;

export interface AttentionDeliveryContext {
  /** Muted workspaces silence delivery — never counts, never bell rows. */
  mutedWorkspaceIds: ReadonlySet<string>;
  /** The session whose window holds focus, when the app is visible. */
  focusedSessionId: string | null;
  appActive: boolean;
  /** Resolves a session id against the latest refetched rows. */
  resolve: (sessionId: string) => AttentionDeliveryTarget | null;
}

export interface AttentionDeliveryPorts {
  now: () => number;
  /**
   * Read at decision time, not at ingest time. Muting a workspace or focusing a
   * session while a coalescing window is open must change what that window
   * delivers (US-015.EC-1).
   */
  getContext: () => AttentionDeliveryContext;
  /** One call per batch — the notifier plays at most one sound per call. */
  deliver: (batch: AttentionDelivery[]) => void;
}

export interface AttentionDeliveryEngine {
  /** Nudges a retry when a refetch may have named a waiting edge. */
  refresh: () => void;
  ingestEdge: (edge: SessionAttentionEventPayload) => void;
  ingestNotification: (notification: OperatorNotificationEventPayload) => void;
  dispose: () => void;
}

interface PendingEdge {
  sessionId: string;
  workspaceId: string;
  at: number;
}

/**
 * Focus suppression (Business Rule 17): the operator is already looking at this
 * session, so a toast would tell them what is on their screen.
 */
export function isFocusSuppressed(sessionId: string, context: AttentionDeliveryContext): boolean {
  return context.appActive && context.focusedSessionId === sessionId;
}

/** A muted workspace delivers nothing, while its rows and counts remain. */
export function isMuted(workspaceId: string, context: AttentionDeliveryContext): boolean {
  return context.mutedWorkspaceIds.has(workspaceId);
}

function suppressed(edge: PendingEdge, context: AttentionDeliveryContext): boolean {
  return isFocusSuppressed(edge.sessionId, context) || isMuted(edge.workspaceId, context);
}

export function createAttentionDeliveryEngine(
  ports: AttentionDeliveryPorts
): AttentionDeliveryEngine {
  let disposed = false;
  const lastDelivered = new Map<string, number>();
  const pendingNeedsYou: Array<PendingEdge & { badge: string }> = [];
  const pendingFinished: PendingEdge[] = [];
  const pendingAgent: OperatorNotificationEventPayload[] = [];
  let flushHandle: ReturnType<typeof setTimeout> | null = null;
  let flushDelay = Number.POSITIVE_INFINITY;
  let coalesceHandle: ReturnType<typeof setTimeout> | null = null;

  // A sooner flush always wins: an edge waiting to be named must deliver the
  // moment the refetch lands, not at the end of the retry poll it happens to
  // share a timer with.
  const scheduleFlush = (delay = 0) => {
    if (disposed || delay >= flushDelay) return;
    if (flushHandle !== null) clearTimeout(flushHandle);
    flushDelay = delay;
    flushHandle = setTimeout(() => {
      flushHandle = null;
      flushDelay = Number.POSITIVE_INFINITY;
      flush();
    }, delay);
  };

  const drainNeedsYou = (
    batch: AttentionDelivery[],
    now: number,
    context: AttentionDeliveryContext
  ) => {
    for (let index = pendingNeedsYou.length - 1; index >= 0; index -= 1) {
      const edge = pendingNeedsYou[index];
      if (edge === undefined) continue;
      const target = context.resolve(edge.sessionId);
      if (target === null) {
        // Give the catalog refetch a moment to name the session, then give up
        // rather than render a toast we cannot describe.
        if (now - edge.at >= RESOLVE_GRACE_MS) pendingNeedsYou.splice(index, 1);
        continue;
      }
      pendingNeedsYou.splice(index, 1);
      if (suppressed(edge, context)) continue;
      batch.push({
        kind: "needs-you",
        key: `needs-you:${edge.sessionId}:${edge.badge}`,
        target,
      });
    }
  };

  const drainAgent = (batch: AttentionDelivery[], context: AttentionDeliveryContext) => {
    while (pendingAgent.length > 0) {
      const notification = pendingAgent.shift();
      if (notification === undefined) continue;
      if (
        isFocusSuppressed(notification.session_id, context) ||
        isMuted(notification.workspace_id, context)
      ) {
        continue;
      }
      batch.push({
        kind: "agent",
        key: `agent:${notification.notification_id}`,
        notification,
        target: context.resolve(notification.session_id),
      });
    }
  };

  const flush = () => {
    if (disposed) return;
    const now = ports.now();
    const context = ports.getContext();
    const batch: AttentionDelivery[] = [];
    drainNeedsYou(batch, now, context);
    drainAgent(batch, context);
    if (batch.length > 0) ports.deliver(batch);
    // Needs-you edges still waiting on a name keep the flush alive so they are
    // delivered as soon as the refetch lands.
    if (pendingNeedsYou.length > 0) scheduleFlush(250);
  };

  const flushFinished = () => {
    coalesceHandle = null;
    if (disposed) return;
    const context = ports.getContext();
    const targets: AttentionDeliveryTarget[] = [];
    while (pendingFinished.length > 0) {
      const edge = pendingFinished.shift();
      if (edge === undefined) continue;
      // A session seen (or restarted) before the window closes drops out of the
      // group: a group of one collapses to the single form, a group of zero
      // fires nothing at all.
      if (suppressed(edge, context)) continue;
      const target = context.resolve(edge.sessionId);
      if (target === null || target.badge !== "done") continue;
      targets.push(target);
    }
    if (targets.length === 0) return;
    ports.deliver([
      {
        kind: "finished",
        key: `finished:${targets.map(target => target.id).join(",")}`,
        targets,
      },
    ]);
  };

  const ingestEdge = (edge: SessionAttentionEventPayload) => {
    if (disposed) return;
    const now = ports.now();
    if (edge.class === "needs-you") {
      const key = edge.session_id;
      const previous = lastDelivered.get(key);
      // A session owns one needs-you notification inside the window even when
      // the requested interaction changes from input to auth (or vice versa).
      if (previous !== undefined && now - previous < NEEDS_YOU_DEDUP_MS) return;
      lastDelivered.set(key, now);
      pendingNeedsYou.push({
        sessionId: edge.session_id,
        workspaceId: edge.workspace_id,
        badge: edge.to,
        at: now,
      });
      scheduleFlush();
      return;
    }
    if (edge.class !== "finished") return;
    pendingFinished.push({
      sessionId: edge.session_id,
      workspaceId: edge.workspace_id,
      at: now,
    });
    // The window never extends itself: a later completion joins the open window
    // rather than pushing its close out, so the signal cannot starve.
    if (coalesceHandle === null) {
      coalesceHandle = setTimeout(flushFinished, COMPLETION_COALESCE_MS);
    }
  };

  return {
    refresh: () => {
      if (pendingNeedsYou.length > 0 || pendingAgent.length > 0) scheduleFlush();
    },
    ingestEdge,
    ingestNotification: notification => {
      if (disposed) return;
      pendingAgent.push(notification);
      scheduleFlush();
    },
    dispose: () => {
      disposed = true;
      if (flushHandle !== null) clearTimeout(flushHandle);
      if (coalesceHandle !== null) clearTimeout(coalesceHandle);
      flushHandle = null;
      flushDelay = Number.POSITIVE_INFINITY;
      coalesceHandle = null;
      pendingNeedsYou.length = 0;
      pendingFinished.length = 0;
      pendingAgent.length = 0;
      lastDelivered.clear();
    },
  };
}
