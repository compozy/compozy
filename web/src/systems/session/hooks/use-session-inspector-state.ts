import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { reset } from "@xstate/store/reset";
import { useSelector } from "@xstate/store-react";

import type { InspectorTabId } from "../lib/session-inspector-tabs";

interface SessionInspectorContext {
  bySession: Record<string, boolean>;
  tabBySession: Record<string, InspectorTabId>;
}

export const SESSION_INSPECTOR_STORAGE_KEY = "compozy:session:inspector:v2";

const DEFAULT_INSPECTOR_TAB: InspectorTabId = "usage";

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

function omitSessionTab(
  tabBySession: Record<string, InspectorTabId>,
  sessionId: string
): Record<string, InspectorTabId> {
  if (!(sessionId in tabBySession)) {
    return tabBySession;
  }
  const next = { ...tabBySession };
  delete next[sessionId];
  return next;
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
    tabBySession: current.tabBySession,
  };
}

export const sessionInspectorLogic = createStoreLogic({
  context: (): SessionInspectorContext => ({ bySession: {}, tabBySession: {} }),
  on: {
    inspectorToggled: (context, event: { sessionId: string }) => {
      if (!event.sessionId) {
        return;
      }
      const open = !(context.bySession[event.sessionId] ?? false);
      return {
        ...context,
        bySession: {
          ...context.bySession,
          [event.sessionId]: open,
        },
        tabBySession: open
          ? context.tabBySession
          : omitSessionTab(context.tabBySession, event.sessionId),
      };
    },
    inspectorClosed: (context, event: { sessionId: string }) => {
      if (!event.sessionId) {
        return;
      }
      return {
        ...context,
        bySession: { ...context.bySession, [event.sessionId]: false },
        tabBySession: omitSessionTab(context.tabBySession, event.sessionId),
      };
    },
    inspectorVisibilityChanged: (context, event: { sessionId: string; open: boolean }) => {
      if (!event.sessionId) {
        return;
      }
      return {
        ...context,
        bySession: { ...context.bySession, [event.sessionId]: event.open },
        tabBySession: event.open
          ? context.tabBySession
          : omitSessionTab(context.tabBySession, event.sessionId),
      };
    },
    inspectorTabRequested: (context, event: { sessionId: string; tab: InspectorTabId }) => {
      if (!event.sessionId) {
        return;
      }
      return {
        ...context,
        bySession: { ...context.bySession, [event.sessionId]: true },
        tabBySession: { ...context.tabBySession, [event.sessionId]: event.tab },
      };
    },
    inspectorTabSelected: (context, event: { sessionId: string; tab: InspectorTabId }) => {
      if (!event.sessionId) {
        return;
      }
      return {
        ...context,
        tabBySession: { ...context.tabBySession, [event.sessionId]: event.tab },
      };
    },
  },
});

export const sessionInspectorStore = sessionInspectorLogic
  .createStore()
  .with(
    persist({
      name: SESSION_INSPECTOR_STORAGE_KEY,
      pick: context => ({ bySession: context.bySession }),
      merge: mergeSessionInspectorPreferences,
    })
  )
  .with(reset());

if (typeof window !== "undefined") {
  window.addEventListener("storage", event => {
    if (event.key === SESSION_INSPECTOR_STORAGE_KEY) {
      void rehydrateStore(sessionInspectorStore);
    }
  });
}

/** Open the inspector on a named tab. Safe before the panel has mounted. */
export function requestSessionInspectorTab(sessionId: string, tab: InspectorTabId): void {
  if (!sessionId) {
    return;
  }
  sessionInspectorStore.trigger.inspectorTabRequested({ sessionId, tab });
}

export interface UseSessionInspectorStateResult {
  open: boolean;
  tab: InspectorTabId;
  toggle: () => void;
  close: () => void;
  setOpen: (open: boolean) => void;
  openTab: (tab: InspectorTabId) => void;
  selectTab: (tab: InspectorTabId) => void;
}

/** Per-session inspector preference, defaulting closed until a named session opens it. */
export function useSessionInspectorState(
  sessionId: string | null | undefined
): UseSessionInspectorStateResult {
  const open = useSelector(sessionInspectorStore, snapshot =>
    sessionId ? (snapshot.context.bySession[sessionId] ?? false) : false
  );
  const tab = useSelector(sessionInspectorStore, snapshot =>
    sessionId
      ? (snapshot.context.tabBySession[sessionId] ?? DEFAULT_INSPECTOR_TAB)
      : DEFAULT_INSPECTOR_TAB
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

  const openTab = (nextTab: InspectorTabId) => {
    if (!sessionId) {
      return;
    }
    requestSessionInspectorTab(sessionId, nextTab);
  };

  const selectTab = (nextTab: InspectorTabId) => {
    if (!sessionId) {
      return;
    }
    sessionInspectorStore.trigger.inspectorTabSelected({ sessionId, tab: nextTab });
  };

  return { open, tab, toggle, close, setOpen, openTab, selectTab };
}
