/**
 * Process-singleton Zustand store backing useNavCounts (TechSpec
 * §"Core Interface — useNavCounts()"). One SSE channel + one polling timer
 * lifecycle per process; consumers refcount-retain the store so the SSE socket
 * and polling timers only run while at least one mount is active.
 *
 * - tasks count is driven by the four task SSE event types emitted today by
 *   internal/observe/tasks.go:22-25 and reconciled against the
 *   /api/observe/tasks/dashboard snapshot.
 * - jobs / network / triggers / agents / knowledge / skills / bridges counts
 *   are polled every 5 s via existing list endpoints. No new endpoints.
 * - SSE-drop + concurrent-polling race (N-009): snapshot fetches issued before
 *   the latest applied SSE event are discarded so highest-seq event wins.
 * - stale semantics: tasks stale when SSE drops AND no event in heartbeat
 *   window (default 5 s); other keys stale when last poll older than 2 × poll
 *   interval (10 s default).
 */
import { type StoreApi, createStore } from "zustand/vanilla";

import { createDefaultFetchers, observeStreamUrlForWorkspace } from "./nav-counts-fetchers";

export { createDefaultFetchers, observeStreamUrlForWorkspace } from "./nav-counts-fetchers";

export const NAV_COUNT_KEYS = [
  "tasks",
  "jobs",
  "network",
  "triggers",
  "agents",
  "knowledge",
  "skills",
  "bridges",
] as const;
export type NavCountKey = (typeof NAV_COUNT_KEYS)[number];

export interface NavCount {
  count: number;
  stale: boolean;
}

export type NavCountsStatus = "ready" | "loading" | "error";

export interface NavCountsState {
  counts: Partial<Record<NavCountKey, NavCount>>;
  status: NavCountsStatus;
  /**
   * Refcounted lifecycle handle. The first retainer starts the SSE channel +
   * polling timers; the last release closes them. Returns a release function
   * that the caller must invoke on unmount.
   */
  retainConsumer: () => () => void;
  /** Trigger an immediate snapshot refetch across every nav-count key. */
  refresh: () => void;
}

export const TASK_NAV_COUNT_EVENT_TYPES = [
  "task.run_enqueued",
  "task.canceled",
  "task.run_force_stopped",
  "task.run_recovered",
] as const;
export type TaskNavCountEventType = (typeof TASK_NAV_COUNT_EVENT_TYPES)[number];

/**
 * Per-event delta applied to the local `tasks` count. refresh() reconciles
 * any drift against /api/observe/tasks/dashboard, so the local arithmetic only
 * needs to be directionally correct for the activity indicator. force_stopped
 * and recovered are run-lifecycle events; the task itself remains, so no
 * delta.
 */
const TASK_EVENT_DELTA: Record<TaskNavCountEventType, number> = {
  "task.run_enqueued": 1,
  "task.canceled": -1,
  "task.run_force_stopped": 0,
  "task.run_recovered": 0,
};

const NON_TASK_KEYS: readonly NavCountKey[] = [
  "jobs",
  "network",
  "triggers",
  "agents",
  "knowledge",
  "skills",
  "bridges",
];

export interface NavCountFetchResult {
  count: number;
}

export type NavCountFetcher = (signal: AbortSignal) => Promise<NavCountFetchResult>;
export type NavCountFetchers = Record<NavCountKey, NavCountFetcher>;

export interface NavCountsEventSource {
  addEventListener: (type: string, listener: (event: MessageEvent) => void) => void;
  removeEventListener?: (type: string, listener: (event: MessageEvent) => void) => void;
  close: () => void;
  onopen: ((event: Event) => void) | null;
  onerror: ((event: Event) => void) | null;
}

export type NavCountsEventSourceFactory = (url: string) => NavCountsEventSource;

export interface NavCountsTimerAPI {
  setInterval: (callback: () => void, ms: number) => unknown;
  clearInterval: (handle: unknown) => void;
}

export interface NavCountsStoreOptions {
  eventSourceFactory?: NavCountsEventSourceFactory;
  observeStreamUrl?: string | null;
  workspaceId?: string | null;
  fetchers?: NavCountFetchers;
  now?: () => number;
  pollIntervalMs?: number;
  heartbeatWindowMs?: number;
  staleCheckIntervalMs?: number;
  timer?: NavCountsTimerAPI;
  logger?: (message: string, err: unknown) => void;
}

export type NavCountsStore = StoreApi<NavCountsState>;

const DEFAULT_OBSERVE_STREAM_URL: string | null = null;
const DEFAULT_POLL_INTERVAL_MS = 5_000;
const DEFAULT_HEARTBEAT_WINDOW_MS = 5_000;
const DEFAULT_STALE_CHECK_INTERVAL_MS = 1_000;

export function createNavCountsStore(options: NavCountsStoreOptions = {}): NavCountsStore {
  const eventSourceFactory = options.eventSourceFactory ?? defaultEventSourceFactory;
  const observeStreamUrl =
    options.observeStreamUrl ??
    (options.workspaceId
      ? observeStreamUrlForWorkspace(options.workspaceId)
      : DEFAULT_OBSERVE_STREAM_URL);
  const fetchers = options.fetchers ?? createDefaultFetchers(options.workspaceId);
  const now = options.now ?? defaultNow;
  const pollIntervalMs = options.pollIntervalMs ?? DEFAULT_POLL_INTERVAL_MS;
  const heartbeatWindowMs = options.heartbeatWindowMs ?? DEFAULT_HEARTBEAT_WINDOW_MS;
  const staleCheckIntervalMs = options.staleCheckIntervalMs ?? DEFAULT_STALE_CHECK_INTERVAL_MS;
  const timer = options.timer ?? globalTimer;
  const logger = options.logger ?? defaultLogger;

  let consumerCount = 0;
  let pollTimer: unknown = null;
  let staleTimer: unknown = null;
  let eventSource: NavCountsEventSource | null = null;
  let snapshotAbort: AbortController | null = null;
  let pollAbort: AbortController | null = null;
  let lastTaskEventCursor = "";
  // Counts applied SSE task events; the tasks-snapshot fetch records this
  // counter at issuance and discards itself if any newer SSE event lands while
  // the request was in flight (race rule N-009: highest-seq event wins).
  let taskEventCounter = 0;
  // Tracks any SSE activity (open + events) for stale evaluation when the
  // socket is alive but no business events have flowed yet.
  let sseLastActivityAt = 0;
  let sseConnected = false;
  const lastPolledAt = new Map<NavCountKey, number>();

  const store = createStore<NavCountsState>(() => ({
    counts: {},
    status: "loading",
    retainConsumer: () => {
      consumerCount += 1;
      if (consumerCount === 1) {
        startLifecycle();
      }
      let released = false;
      return () => {
        if (released) return;
        released = true;
        consumerCount = Math.max(0, consumerCount - 1);
        if (consumerCount === 0) {
          stopLifecycle();
        }
      };
    },
    refresh: () => {
      void runSnapshotPass({ markLoading: true });
    },
  }));

  function startLifecycle() {
    store.setState({ status: "loading" });
    openEventSource();
    pollTimer = timer.setInterval(() => {
      void runPollPass();
    }, pollIntervalMs);
    staleTimer = timer.setInterval(() => {
      evaluateStaleness();
    }, staleCheckIntervalMs);
    void runSnapshotPass({ markLoading: false });
  }

  function stopLifecycle() {
    if (eventSource) {
      const source = eventSource;
      eventSource = null;
      try {
        for (const type of TASK_NAV_COUNT_EVENT_TYPES) {
          source.removeEventListener?.(type, handleTaskEvent);
        }
        source.onopen = null;
        source.onerror = null;
        source.close();
      } catch (err) {
        logger("nav-counts: SSE close failed", err);
      }
    }
    sseConnected = false;
    if (snapshotAbort) {
      snapshotAbort.abort();
      snapshotAbort = null;
    }
    if (pollAbort) {
      pollAbort.abort();
      pollAbort = null;
    }
    if (pollTimer != null) {
      timer.clearInterval(pollTimer);
      pollTimer = null;
    }
    if (staleTimer != null) {
      timer.clearInterval(staleTimer);
      staleTimer = null;
    }
  }

  function openEventSource() {
    if (!observeStreamUrl) {
      sseConnected = false;
      return;
    }
    try {
      const source = eventSourceFactory(observeStreamUrl);
      eventSource = source;
      for (const type of TASK_NAV_COUNT_EVENT_TYPES) {
        source.addEventListener(type, handleTaskEvent);
      }
      source.onopen = () => {
        sseConnected = true;
        sseLastActivityAt = now();
        markTasksFresh();
      };
      source.onerror = () => {
        sseConnected = false;
      };
    } catch (err) {
      sseConnected = false;
      logger("nav-counts: failed to open SSE", err);
    }
  }

  function handleTaskEvent(event: MessageEvent) {
    const cursor = event.lastEventId ?? "";
    if (cursor && lastTaskEventCursor && cursor <= lastTaskEventCursor) {
      return;
    }
    const eventType = event.type as TaskNavCountEventType;
    const delta = TASK_EVENT_DELTA[eventType];
    if (delta === undefined) return;
    if (cursor) {
      lastTaskEventCursor = cursor;
    }
    taskEventCounter += 1;
    const stamp = now();
    sseLastActivityAt = stamp;
    sseConnected = true;
    store.setState(state => {
      const previous = state.counts.tasks ?? { count: 0, stale: false };
      const nextCount = Math.max(0, previous.count + delta);
      return {
        counts: {
          ...state.counts,
          tasks: { count: nextCount, stale: false },
        },
      };
    });
  }

  function markTasksFresh(): void {
    store.setState(state => {
      const tasks = state.counts.tasks;
      if (!tasks?.stale) {
        return state;
      }
      return {
        counts: {
          ...state.counts,
          tasks: { ...tasks, stale: false },
        },
      };
    });
  }

  async function runSnapshotPass(opts: { markLoading: boolean }): Promise<void> {
    snapshotAbort?.abort();
    const controller = new AbortController();
    snapshotAbort = controller;
    const { signal } = controller;
    if (opts.markLoading) {
      store.setState({ status: "loading" });
    }
    const issuedTaskCounter = taskEventCounter;
    const results = await Promise.allSettled(
      NAV_COUNT_KEYS.map(key => fetchAndApply(key, signal, issuedTaskCounter))
    );
    if (signal.aborted) return;
    snapshotAbort = null;
    const allOk = results.every(r => r.status === "fulfilled");
    store.setState({ status: allOk ? "ready" : "error" });
  }

  async function runPollPass(): Promise<void> {
    if (snapshotAbort) {
      // The snapshot pass is the broader operation; let it finish.
      return;
    }
    pollAbort?.abort();
    const controller = new AbortController();
    pollAbort = controller;
    const { signal } = controller;
    await Promise.allSettled(
      NON_TASK_KEYS.map(key => fetchAndApply(key, signal, taskEventCounter))
    );
    if (signal.aborted) return;
    pollAbort = null;
  }

  async function fetchAndApply(
    key: NavCountKey,
    signal: AbortSignal,
    issuedTaskCounter: number
  ): Promise<void> {
    const result = await fetchers[key](signal);
    if (signal.aborted) return;
    if (key === "tasks" && taskEventCounter > issuedTaskCounter) {
      // SSE delivered a newer task event while this snapshot was in flight;
      // the higher-seq event wins (N-009).
      return;
    }
    lastPolledAt.set(key, now());
    store.setState(state => ({
      counts: {
        ...state.counts,
        [key]: { count: result.count, stale: false },
      },
    }));
  }

  function evaluateStaleness(): void {
    const current = now();
    store.setState(state => {
      let mutated = false;
      const next: Partial<Record<NavCountKey, NavCount>> = { ...state.counts };
      const taskEntry = next.tasks;
      if (taskEntry) {
        const taskStale =
          !sseConnected && sseLastActivityAt > 0 && current - sseLastActivityAt > heartbeatWindowMs;
        if (taskStale !== taskEntry.stale) {
          next.tasks = { ...taskEntry, stale: taskStale };
          mutated = true;
        }
      }
      for (const key of NON_TASK_KEYS) {
        const entry = next[key];
        if (!entry) continue;
        const polled = lastPolledAt.get(key) ?? 0;
        const stale = polled > 0 && current - polled > pollIntervalMs * 2;
        if (stale !== entry.stale) {
          next[key] = { ...entry, stale };
          mutated = true;
        }
      }
      if (!mutated) return state;
      return { counts: next };
    });
  }

  return store;
}

function defaultEventSourceFactory(url: string): NavCountsEventSource {
  return new EventSource(url);
}

function defaultNow(): number {
  return Date.now();
}

const globalTimer: NavCountsTimerAPI = {
  setInterval: (callback, ms) => globalThis.setInterval(callback, ms),
  clearInterval: handle => {
    globalThis.clearInterval(handle as ReturnType<typeof setInterval>);
  },
};

function defaultLogger(message: string, err: unknown): void {
  console.error(message, err);
}

const processSingletons = new Map<string, NavCountsStore>();

/** Process-wide singleton store used by useNavCounts(). */
export function getNavCountsStore(workspaceId?: string | null): NavCountsStore {
  const key = workspaceId ?? "";
  const existing = processSingletons.get(key);
  if (existing) {
    return existing;
  }
  const store = createNavCountsStore({
    workspaceId,
    observeStreamUrl: workspaceId ? observeStreamUrlForWorkspace(workspaceId) : null,
  });
  processSingletons.set(key, store);
  return store;
}
