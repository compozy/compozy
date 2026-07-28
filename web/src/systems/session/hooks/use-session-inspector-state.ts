import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

interface SessionInspectorContext {
  bySession: Record<string, boolean>;
}

export const SESSION_INSPECTOR_STORAGE_KEY = "compozy:session:inspector:v2";

function normalizedSessionInspectorPreferences(value: unknown): Record<string, boolean> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return {};
  }

  return Object.fromEntries(
    Object.entries(value).filter(
      (entry): entry is [string, boolean] => typeof entry[1] === "boolean"
    )
  );
}

function mergeSessionInspectorPreferences(
  persisted: Partial<SessionInspectorContext>,
  current: SessionInspectorContext
): SessionInspectorContext {
  return {
    bySession: {
      ...current.bySession,
      ...normalizedSessionInspectorPreferences(persisted.bySession),
    },
  };
}

export const sessionInspectorLogic = createStoreLogic({
  context: (): SessionInspectorContext => ({ bySession: {} }),
  on: {
    inspectorToggled: (context, event: { sessionId: string }) => {
      if (!event.sessionId) {
        return;
      }
      return {
        ...context,
        bySession: {
          ...context.bySession,
          [event.sessionId]: !(context.bySession[event.sessionId] ?? false),
        },
      };
    },
    inspectorClosed: (context, event: { sessionId: string }) => {
      if (!event.sessionId) {
        return;
      }
      return {
        ...context,
        bySession: { ...context.bySession, [event.sessionId]: false },
      };
    },
    inspectorVisibilityChanged: (context, event: { sessionId: string; open: boolean }) => {
      if (!event.sessionId) {
        return;
      }
      return {
        ...context,
        bySession: { ...context.bySession, [event.sessionId]: event.open },
      };
    },
  },
});

export const sessionInspectorStore = sessionInspectorLogic.createStore().with(
  persist({
    name: SESSION_INSPECTOR_STORAGE_KEY,
    merge: mergeSessionInspectorPreferences,
  })
);

if (typeof window !== "undefined") {
  window.addEventListener("storage", event => {
    if (event.key === SESSION_INSPECTOR_STORAGE_KEY) {
      void rehydrateStore(sessionInspectorStore);
    }
  });
}

export interface UseSessionInspectorStateResult {
  open: boolean;
  toggle: () => void;
  close: () => void;
  setOpen: (open: boolean) => void;
}

/** Per-session inspector preference, defaulting closed until a named session opens it. */
export function useSessionInspectorState(
  sessionId: string | null | undefined
): UseSessionInspectorStateResult {
  const open = useSelector(sessionInspectorStore, snapshot =>
    sessionId ? (snapshot.context.bySession[sessionId] ?? false) : false
  );

  const toggle = () => {
    if (!sessionId) {
      return;
    }
    sessionInspectorStore.trigger.inspectorToggled({ sessionId });
  };

  const close = () => {
    if (!sessionId) {
      return;
    }
    sessionInspectorStore.trigger.inspectorClosed({ sessionId });
  };

  const setOpen = (nextOpen: boolean) => {
    if (!sessionId) {
      return;
    }
    sessionInspectorStore.trigger.inspectorVisibilityChanged({ open: nextOpen, sessionId });
  };

  return { open, toggle, close, setOpen };
}
