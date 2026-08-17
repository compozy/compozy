import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

interface SessionSidebarContext {
  /** One preference for every session window; the rail is closed until opened. */
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
  context: (): SessionSidebarContext => ({ open: false, collapsedThreads: {} }),
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

export const sessionSidebarStore = sessionSidebarLogic.createStore().with(
  persist({
    name: SESSION_SIDEBAR_STORAGE_KEY,
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

/** Session-window rail preference, defaulting closed until the operator opens it. */
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
