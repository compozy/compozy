import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

import type { FileViewerTarget } from "@/systems/files";

import { sessionInspectorStore } from "./use-session-inspector-state";

/** The Review panel's tabs; S5 adds `changes` beside `files`. */
export type SessionReviewTab = "files";

export interface SessionReviewPanelPreference {
  open: boolean;
  tab: SessionReviewTab;
  /** Panel width as a percentage of the session window ("45%"). */
  size?: string;
}

interface SessionReviewPanelContext {
  bySession: Record<string, SessionReviewPanelPreference>;
  /** Transient: the file a link click asked for, per session. Never persisted. */
  targets: Record<string, FileViewerTarget>;
}

export const SESSION_REVIEW_PANEL_STORAGE_KEY = "compozy:session:review-panel:v1";
export const SESSION_REVIEW_PANEL_DEFAULT_SIZE = "45%";

const CLOSED: SessionReviewPanelPreference = { open: false, tab: "files" };

function normalizedPreference(value: unknown): SessionReviewPanelPreference | null {
  if (typeof value !== "object" || value === null) return null;
  const record = value as Record<string, unknown>;
  if (typeof record.open !== "boolean") return null;
  const size =
    typeof record.size === "string" && /^\d+(\.\d+)?%$/.test(record.size) ? record.size : undefined;
  return { open: record.open, tab: "files", ...(size ? { size } : {}) };
}

function mergeReviewPanelPreferences(
  persisted: Partial<SessionReviewPanelContext>,
  current: SessionReviewPanelContext
): SessionReviewPanelContext {
  const bySession: Record<string, SessionReviewPanelPreference> = { ...current.bySession };
  const stored = persisted.bySession;
  if (typeof stored === "object" && stored !== null && !Array.isArray(stored)) {
    for (const [sessionId, value] of Object.entries(stored)) {
      const preference = normalizedPreference(value);
      if (preference) bySession[sessionId] = preference;
    }
  }
  return { bySession, targets: current.targets };
}

function withPreference(
  context: SessionReviewPanelContext,
  sessionId: string,
  patch: Partial<SessionReviewPanelPreference>
): SessionReviewPanelContext {
  const current = context.bySession[sessionId] ?? CLOSED;
  return { ...context, bySession: { ...context.bySession, [sessionId]: { ...current, ...patch } } };
}

export const sessionReviewPanelLogic = createStoreLogic({
  context: (): SessionReviewPanelContext => ({ bySession: {}, targets: {} }),
  on: {
    visibilityChanged: (context, event: { sessionId: string; open: boolean }) =>
      event.sessionId ? withPreference(context, event.sessionId, { open: event.open }) : context,
    tabSelected: (context, event: { sessionId: string; tab: SessionReviewTab }) =>
      event.sessionId ? withPreference(context, event.sessionId, { tab: event.tab }) : context,
    resized: (context, event: { sessionId: string; size: string }) =>
      event.sessionId ? withPreference(context, event.sessionId, { size: event.size }) : context,
    fileRequested: (context, event: { sessionId: string; target: FileViewerTarget }) => {
      if (!event.sessionId) return context;
      const next = withPreference(context, event.sessionId, { open: true, tab: "files" });
      return { ...next, targets: { ...next.targets, [event.sessionId]: event.target } };
    },
  },
});

export const sessionReviewPanelStore = sessionReviewPanelLogic.createStore().with(
  persist({
    name: SESSION_REVIEW_PANEL_STORAGE_KEY,
    merge: mergeReviewPanelPreferences,
    pick: context => ({ bySession: context.bySession }),
  })
);

/**
 * The Review panel and the Inspector share the session window's right side (Business Rule 9):
 * opening either one closes the other, whichever surface opened it.
 */
function closeInspector(sessionId: string) {
  if (sessionInspectorStore.getSnapshot().context.bySession[sessionId]) {
    sessionInspectorStore.trigger.inspectorClosed({ sessionId });
  }
}

let inspectorOpen: Record<string, boolean> = {
  ...sessionInspectorStore.getSnapshot().context.bySession,
};
sessionInspectorStore.subscribe(snapshot => {
  const next = snapshot.context.bySession;
  for (const [sessionId, open] of Object.entries(next)) {
    if (open && !inspectorOpen[sessionId]) {
      if (sessionReviewPanelStore.getSnapshot().context.bySession[sessionId]?.open) {
        sessionReviewPanelStore.trigger.visibilityChanged({ sessionId, open: false });
      }
    }
  }
  inspectorOpen = { ...next };
});

if (typeof window !== "undefined") {
  window.addEventListener("storage", event => {
    if (event.key === SESSION_REVIEW_PANEL_STORAGE_KEY) {
      void rehydrateStore(sessionReviewPanelStore);
    }
  });
}

/** Opens the session's Review panel on Files at `target` (a transcript link click). */
export function requestSessionReviewFile(sessionId: string, target: FileViewerTarget): void {
  if (!sessionId) return;
  closeInspector(sessionId);
  sessionReviewPanelStore.trigger.fileRequested({ sessionId, target });
}

export interface UseSessionReviewPanelStateResult {
  open: boolean;
  tab: SessionReviewTab;
  size: string;
  target: FileViewerTarget | null;
  toggle: () => void;
  close: () => void;
  setTab: (tab: SessionReviewTab) => void;
  setSize: (size: string) => void;
  openFile: (target: FileViewerTarget) => void;
}

/** Per-session Review panel preference (open, tab, width) plus the transient viewer target. */
export function useSessionReviewPanelState(sessionId: string): UseSessionReviewPanelStateResult {
  const preference = useSelector(
    sessionReviewPanelStore,
    snapshot => snapshot.context.bySession[sessionId] ?? CLOSED
  );
  const target = useSelector(
    sessionReviewPanelStore,
    snapshot => snapshot.context.targets[sessionId] ?? null
  );
  const setOpen = (open: boolean) => {
    if (!sessionId) return;
    if (open) closeInspector(sessionId);
    sessionReviewPanelStore.trigger.visibilityChanged({ sessionId, open });
  };
  return {
    open: sessionId ? preference.open : false,
    tab: preference.tab,
    size: preference.size ?? SESSION_REVIEW_PANEL_DEFAULT_SIZE,
    target,
    toggle: () => setOpen(!preference.open),
    close: () => setOpen(false),
    setTab: tab => sessionReviewPanelStore.trigger.tabSelected({ sessionId, tab }),
    setSize: size => sessionReviewPanelStore.trigger.resized({ sessionId, size }),
    openFile: next => requestSessionReviewFile(sessionId, next),
  };
}
