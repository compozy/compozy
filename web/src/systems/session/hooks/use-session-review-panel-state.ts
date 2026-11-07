import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

import type { FileViewerTarget } from "@/systems/files";

import type { SessionChangesScope } from "../adapters/session-changes-api";

import { sessionInspectorStore } from "./use-session-inspector-state";

/** The Review panel's tabs. */
export type SessionReviewTab = "changes" | "files";

export type SessionDiffMode = "split" | "unified";

export interface SessionReviewPanelPreference {
  open: boolean;
  tab: SessionReviewTab;
  /** Panel width as a percentage of the session window ("45%"). */
  size?: string;
  /** Changes: split or unified diffs (US-007.AC-5). */
  diffMode?: SessionDiffMode;
  /** Changes: the operator's scope; absent means the daemon default (US-008.AC-3). */
  scope?: SessionChangesScope;
  /** Changes: the operator's base ref; absent means the daemon default. */
  base?: string;
  /** Changes: the changed-files tree beside the diffs; shown unless hidden. */
  changesTree?: boolean;
}

/** Transient: a changed file a click asked Changes to focus. `nonce` re-focuses the same path. */
export interface SessionChangeFocus {
  path: string;
  nonce: number;
}

interface SessionReviewPanelContext {
  bySession: Record<string, SessionReviewPanelPreference>;
  /** Transient: the file a link click asked for, per session. Never persisted. */
  targets: Record<string, FileViewerTarget>;
  /** Transient: the changed file a click asked Changes to show, per session. */
  focusedChanges: Record<string, SessionChangeFocus>;
  /** Transient: bumps when the session's checkout changes, remounting root-bound panel state. */
  rootGenerations: Record<string, number>;
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
  const diffMode =
    record.diffMode === "split" || record.diffMode === "unified" ? record.diffMode : undefined;
  const scope =
    record.scope === "branch" || record.scope === "uncommitted" ? record.scope : undefined;
  const base = typeof record.base === "string" && record.base !== "" ? record.base : undefined;
  return {
    open: record.open,
    tab: record.tab === "changes" ? "changes" : "files",
    ...(size ? { size } : {}),
    ...(diffMode ? { diffMode } : {}),
    ...(scope ? { scope } : {}),
    ...(base ? { base } : {}),
    ...(record.changesTree === false ? { changesTree: false } : {}),
  };
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
  return {
    bySession,
    targets: current.targets,
    focusedChanges: current.focusedChanges,
    rootGenerations: current.rootGenerations,
  };
}

function withPreference(
  context: SessionReviewPanelContext,
  sessionId: string,
  patch: Partial<SessionReviewPanelPreference>
): SessionReviewPanelContext {
  const current = context.bySession[sessionId] ?? CLOSED;
  return { ...context, bySession: { ...context.bySession, [sessionId]: { ...current, ...patch } } };
}

let focusNonce = 0;
function nextFocusNonce(): number {
  focusNonce += 1;
  return focusNonce;
}

export const sessionReviewPanelLogic = createStoreLogic({
  context: (): SessionReviewPanelContext => ({
    bySession: {},
    targets: {},
    focusedChanges: {},
    rootGenerations: {},
  }),
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
    changesPreferenceChanged: (
      context,
      event: {
        sessionId: string;
        patch: Pick<SessionReviewPanelPreference, "diffMode" | "scope" | "base" | "changesTree">;
      }
    ) => (event.sessionId ? withPreference(context, event.sessionId, event.patch) : context),
    /**
     * The session now runs in another checkout: nothing chosen in the old one (the viewer target,
     * a focused change, the Changes base) carries over, and root-bound panel state remounts.
     */
    rootChanged: (context, event: { sessionId: string }) => {
      if (!event.sessionId) return context;
      const { [event.sessionId]: _target, ...targets } = context.targets;
      const { [event.sessionId]: _focus, ...focusedChanges } = context.focusedChanges;
      const current = context.bySession[event.sessionId];
      let bySession = context.bySession;
      if (current) {
        const { base: _base, ...kept } = current;
        bySession = { ...context.bySession, [event.sessionId]: kept };
      }
      const generation = (context.rootGenerations[event.sessionId] ?? 0) + 1;
      return {
        bySession,
        targets,
        focusedChanges,
        rootGenerations: { ...context.rootGenerations, [event.sessionId]: generation },
      };
    },
    changesRequested: (
      context,
      event: { sessionId: string; path?: string; scope?: SessionChangesScope }
    ) => {
      if (!event.sessionId) return context;
      const next = withPreference(context, event.sessionId, {
        open: true,
        tab: "changes",
        ...(event.scope ? { scope: event.scope } : {}),
      });
      if (!event.path) return next;
      const focus = { path: event.path, nonce: nextFocusNonce() };
      return { ...next, focusedChanges: { ...next.focusedChanges, [event.sessionId]: focus } };
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

/** The session's checkout changed; see the `rootChanged` transition. */
export function resetSessionReviewRoot(sessionId: string): void {
  if (sessionId) sessionReviewPanelStore.trigger.rootChanged({ sessionId });
}

/**
 * Opens the session's Review panel on Changes (the PR chip's **Review changes**, a turn's
 * changed-file row). With `path`, the tab focuses that file, or falls back to Files when the
 * summary doesn't list it (US-005.AC-6).
 */
export function requestSessionReviewChanges(
  sessionId: string,
  request: { path?: string; scope?: SessionChangesScope } = {}
): void {
  if (!sessionId) return;
  closeInspector(sessionId);
  sessionReviewPanelStore.trigger.changesRequested({ sessionId, ...request });
}

export interface UseSessionReviewPanelStateResult {
  open: boolean;
  tab: SessionReviewTab;
  size: string;
  target: FileViewerTarget | null;
  diffMode: SessionDiffMode;
  scope: SessionChangesScope | undefined;
  base: string | undefined;
  changesTree: boolean;
  focusedChange: SessionChangeFocus | null;
  /** Changes when the session's checkout changes; keys the panel's root-bound state. */
  rootKey: number;
  setChangesPreference: (
    patch: Pick<SessionReviewPanelPreference, "diffMode" | "scope" | "base" | "changesTree">
  ) => void;
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
  const focusedChange = useSelector(
    sessionReviewPanelStore,
    snapshot => snapshot.context.focusedChanges[sessionId] ?? null
  );
  const rootKey = useSelector(
    sessionReviewPanelStore,
    snapshot => snapshot.context.rootGenerations[sessionId] ?? 0
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
    diffMode: preference.diffMode ?? "unified",
    scope: preference.scope,
    base: preference.base,
    changesTree: preference.changesTree ?? true,
    focusedChange,
    rootKey,
    setChangesPreference: patch =>
      sessionReviewPanelStore.trigger.changesPreferenceChanged({ sessionId, patch }),
    toggle: () => setOpen(!preference.open),
    close: () => setOpen(false),
    setTab: tab => sessionReviewPanelStore.trigger.tabSelected({ sessionId, tab }),
    setSize: size => sessionReviewPanelStore.trigger.resized({ sessionId, size }),
    openFile: next => requestSessionReviewFile(sessionId, next),
  };
}
