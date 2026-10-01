import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

interface SessionSidebarContext {
  /** One preference for every session window; the rail is open until the operator closes it. */
  open: boolean;
  collapsedThreads: Record<string, boolean>;
}

export const SESSION_SIDEBAR_STORAGE_KEY = "compozy:session:sidebar:v1";

function normalizedCollapsedThreads(value: unknown): Record<string, boolean> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return {};
  }
  return Object.fromEntries(
    Object.entries(value).filter(
      (entry): entry is [string, boolean] => typeof entry[1] === "boolean"
    )
  );
}

function mergeSessionSidebarPreferences(
  persisted: Partial<SessionSidebarContext>,
  current: SessionSidebarContext
): SessionSidebarContext {
  return {
    open: typeof persisted.open === "boolean" ? persisted.open : current.open,
    collapsedThreads: {
      ...current.collapsedThreads,
      ...normalizedCollapsedThreads(persisted.collapsedThreads),
    },
  };
}

export const sessionSidebarLogic = createStoreLogic({
  context: (): SessionSidebarContext => ({ open: true, collapsedThreads: {} }),
  on: {
    sidebarToggled: context => ({ ...context, open: !context.open }),
    sidebarVisibilityChanged: (context, event: { open: boolean }) => ({
      ...context,
      open: event.open,
    }),
    threadToggled: (context, event: { sessionId: string }) => {
      if (!event.sessionId) {
        return;
      }
      return {
        ...context,
        collapsedThreads: {
          ...context.collapsedThreads,
          [event.sessionId]: !(context.collapsedThreads[event.sessionId] ?? false),
        },
      };
    },
  },
});

/**
 * Schema 2 is the open-by-default rail. Older blobs (schema 0, unversioned)
 * persisted the whole context on every transition — collapsing a thread wrote
 * the old closed default back as `open: false` — so their `open` is not a
 * choice the operator made: drop it and keep the per-thread collapse.
 */
const SESSION_SIDEBAR_SCHEMA_VERSION = 2;

// Runs only for a blob older than the current schema (the persist extension
// skips it on a version match). Partial by design: the merge fills the default.
function migrateSessionSidebarPreferences(persisted: unknown): SessionSidebarContext {
  const collapsedThreads =
    typeof persisted === "object" && persisted !== null && "collapsedThreads" in persisted
      ? normalizedCollapsedThreads(persisted.collapsedThreads)
      : {};
  return { collapsedThreads } as SessionSidebarContext;
}

export const sessionSidebarStore = sessionSidebarLogic.createStore().with(
  persist({
    name: SESSION_SIDEBAR_STORAGE_KEY,
    version: SESSION_SIDEBAR_SCHEMA_VERSION,
    migrate: migrateSessionSidebarPreferences,
    merge: mergeSessionSidebarPreferences,
  })
);

export function toggleSessionSidebar(): void {
  sessionSidebarStore.trigger.sidebarToggled();
}

if (typeof window !== "undefined") {
  window.addEventListener("storage", event => {
    if (event.key === SESSION_SIDEBAR_STORAGE_KEY) {
      void rehydrateStore(sessionSidebarStore);
    }
  });
}

export interface UseSessionSidebarStateResult {
  open: boolean;
  toggle: () => void;
  setOpen: (open: boolean) => void;
  collapsedThreadIds: string[];
  toggleThread: (sessionId: string) => void;
}

/** Session-window rail preference: open by default; a persisted toggle wins. */
export function useSessionSidebarState(): UseSessionSidebarStateResult {
  const open = useSelector(sessionSidebarStore, snapshot => snapshot.context.open);
  const collapsedThreads = useSelector(
    sessionSidebarStore,
    snapshot => snapshot.context.collapsedThreads
  );
  const collapsedThreadIds: string[] = [];
  for (const [sessionId, collapsed] of Object.entries(collapsedThreads)) {
    if (collapsed) collapsedThreadIds.push(sessionId);
  }

  return {
    open,
    toggle: toggleSessionSidebar,
    setOpen: (nextOpen: boolean) =>
      sessionSidebarStore.trigger.sidebarVisibilityChanged({ open: nextOpen }),
    collapsedThreadIds,
    toggleThread: (sessionId: string) => sessionSidebarStore.trigger.threadToggled({ sessionId }),
  };
}
