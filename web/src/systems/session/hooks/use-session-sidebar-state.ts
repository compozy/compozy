import { useEffect } from "react";
import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { SHELL_WORKTREE_SCOPE } from "@/systems/workspace";

/**
 * One window's sessions-rail presentation. Per-window by design: two session
 * windows side by side open, close, and collapse independently. A rail width
 * (the #836 resizer) belongs here as another per-scope field.
 */
export interface SessionSidebarScopeState {
  /** The rail is open until the operator closes it in this window. */
  open: boolean;
  collapsedThreads: Record<string, boolean>;
  /** Folders start expanded; a collapse is remembered per folder id. */
  collapsedFolders: Record<string, boolean>;
}

interface SessionSidebarContext {
  /**
   * Keyed by window scope (the window id, or `shell` outside a window). The
   * shell entry is the last-used state: every gesture also writes it, and a
   * window with no entry of its own starts from it.
   */
  byScope: Record<string, SessionSidebarScopeState>;
}

export const SESSION_SIDEBAR_STORAGE_KEY = "compozy:session:sidebar:v1";

const DEFAULT_SCOPE_STATE: SessionSidebarScopeState = {
  open: true,
  collapsedThreads: {},
  collapsedFolders: {},
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function normalizedCollapsedFlags(value: unknown): Record<string, boolean> {
  if (!isRecord(value)) {
    return {};
  }
  return Object.fromEntries(
    Object.entries(value).filter(
      (entry): entry is [string, boolean] => typeof entry[1] === "boolean"
    )
  );
}

function normalizedScopeState(value: unknown): SessionSidebarScopeState {
  if (!isRecord(value)) {
    return DEFAULT_SCOPE_STATE;
  }
  return {
    open: typeof value.open === "boolean" ? value.open : DEFAULT_SCOPE_STATE.open,
    collapsedThreads: normalizedCollapsedFlags(value.collapsedThreads),
    collapsedFolders: normalizedCollapsedFlags(value.collapsedFolders),
  };
}

/**
 * A scope with no entry of its own follows the shell (last-used) state, so a
 * new window opens looking like the last one the operator touched.
 */
export function resolveSessionSidebarScope(
  byScope: Record<string, SessionSidebarScopeState>,
  scopeId: string
): SessionSidebarScopeState {
  return byScope[scopeId] ?? byScope[SHELL_WORKTREE_SCOPE] ?? DEFAULT_SCOPE_STATE;
}

function mergeSessionSidebarPreferences(
  persisted: Partial<SessionSidebarContext>,
  current: SessionSidebarContext
): SessionSidebarContext {
  if (!isRecord(persisted.byScope)) {
    return current;
  }
  const byScope = { ...current.byScope };
  for (const [scopeId, state] of Object.entries(persisted.byScope)) {
    byScope[scopeId] = normalizedScopeState(state);
  }
  return { byScope };
}

/** Writes one scope and makes it the last-used state new windows inherit. */
function withScopeState(
  context: SessionSidebarContext,
  scopeId: string,
  update: (state: SessionSidebarScopeState) => SessionSidebarScopeState
): SessionSidebarContext {
  const next = update(resolveSessionSidebarScope(context.byScope, scopeId));
  return {
    byScope: { ...context.byScope, [scopeId]: next, [SHELL_WORKTREE_SCOPE]: next },
  };
}

function toggledFlag(flags: Record<string, boolean>, id: string): Record<string, boolean> {
  return { ...flags, [id]: !(flags[id] ?? false) };
}

export const sessionSidebarLogic = createStoreLogic({
  context: (): SessionSidebarContext => ({ byScope: {} }),
  on: {
    sidebarToggled: (context, event: { scopeId: string }) =>
      withScopeState(context, event.scopeId, state => ({ ...state, open: !state.open })),
    sidebarVisibilityChanged: (context, event: { scopeId: string; open: boolean }) =>
      withScopeState(context, event.scopeId, state => ({ ...state, open: event.open })),
    threadToggled: (context, event: { scopeId: string; sessionId: string }) => {
      if (!event.sessionId) {
        return;
      }
      return withScopeState(context, event.scopeId, state => ({
        ...state,
        collapsedThreads: toggledFlag(state.collapsedThreads, event.sessionId),
      }));
    },
    folderToggled: (context, event: { scopeId: string; folderId: string }) => {
      if (!event.folderId) {
        return;
      }
      return withScopeState(context, event.scopeId, state => ({
        ...state,
        collapsedFolders: toggledFlag(state.collapsedFolders, event.folderId),
      }));
    },
    /**
     * Pins a window to the state it opened with. Without its own entry a window
     * would keep following the shell, i.e. every other window's gestures.
     */
    scopeSeeded: (context, event: { scopeId: string }) => {
      if (Object.hasOwn(context.byScope, event.scopeId)) {
        return;
      }
      return {
        byScope: {
          ...context.byScope,
          [event.scopeId]: resolveSessionSidebarScope(context.byScope, event.scopeId),
        },
      };
    },
    /**
     * Windows close from any client, so no single owner can release a scope
     * deterministically — the shell prunes against the live window set instead.
     */
    scopesPruned: (context, event: { liveScopeIds: readonly string[] }) => {
      const live = new Set<string>([SHELL_WORKTREE_SCOPE, ...event.liveScopeIds]);
      const entries = Object.entries(context.byScope);
      if (entries.every(([scopeId]) => live.has(scopeId))) {
        return;
      }
      return { byScope: Object.fromEntries(entries.filter(([scopeId]) => live.has(scopeId))) };
    },
  },
});

/**
 * Schema 3 keys the rail by window scope. Schema 2 was one global rail for
 * every window: it becomes the shell (last-used) entry, so every existing and
 * new window starts exactly where the operator left the old global rail.
 * Older blobs (schema 0, unversioned) persisted the whole context on every
 * transition — collapsing a thread wrote the old closed default back as
 * `open: false` — so their `open` is not a choice the operator made: drop it
 * and keep the per-thread collapse.
 */
const SESSION_SIDEBAR_SCHEMA_VERSION = 3;

// Runs only for a blob older than the current schema (the persist extension
// skips it on a version match); the merge normalizes every scope entry.
function migrateSessionSidebarPreferences(
  persisted: unknown,
  version: string | number
): SessionSidebarContext {
  const legacy = isRecord(persisted) ? persisted : {};
  const shell =
    version === 2
      ? legacy
      : { collapsedThreads: normalizedCollapsedFlags(legacy.collapsedThreads) };
  return {
    byScope: { [SHELL_WORKTREE_SCOPE]: shell as unknown as SessionSidebarScopeState },
  };
}

export const sessionSidebarStore = sessionSidebarLogic.createStore().with(
  persist({
    name: SESSION_SIDEBAR_STORAGE_KEY,
    version: SESSION_SIDEBAR_SCHEMA_VERSION,
    // Window cleanup is derived from the daemon, not an operator gesture.
    // Persisting it would let a background document overwrite another tab's choice.
    filter: event => event.type !== "scopesPruned",
    migrate: migrateSessionSidebarPreferences,
    merge: mergeSessionSidebarPreferences,
  })
);

/** Drops rail state for windows that no longer exist; the shell entry stays. */
export function pruneSessionSidebarScopes(liveScopeIds: readonly string[]): void {
  sessionSidebarStore.trigger.scopesPruned({ liveScopeIds });
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
  collapsedFolderIds: string[];
  toggleFolder: (folderId: string) => void;
}

/**
 * Sessions-rail state for the window scope rendering this hook: the window
 * itself inside an OS window, the focused window in shell chrome (so ⌘B, the
 * palette toggle, and session cycling act on what the operator is looking at),
 * and the shell scope outside the desktop.
 */
export function useSessionSidebarState(): UseSessionSidebarStateResult {
  const scopeId = useWorktreeScopeId();
  const seeded = useSelector(sessionSidebarStore, snapshot =>
    Object.hasOwn(snapshot.context.byScope, scopeId)
  );
  const state = useSelector(sessionSidebarStore, snapshot =>
    resolveSessionSidebarScope(snapshot.context.byScope, scopeId)
  );

  useEffect(() => {
    if (!seeded && scopeId !== SHELL_WORKTREE_SCOPE) {
      sessionSidebarStore.trigger.scopeSeeded({ scopeId });
    }
  }, [scopeId, seeded]);

  return {
    open: state.open,
    toggle: () => sessionSidebarStore.trigger.sidebarToggled({ scopeId }),
    setOpen: (nextOpen: boolean) =>
      sessionSidebarStore.trigger.sidebarVisibilityChanged({ scopeId, open: nextOpen }),
    collapsedThreadIds: collapsedIds(state.collapsedThreads),
    toggleThread: (sessionId: string) =>
      sessionSidebarStore.trigger.threadToggled({ scopeId, sessionId }),
    collapsedFolderIds: collapsedIds(state.collapsedFolders),
    toggleFolder: (folderId: string) =>
      sessionSidebarStore.trigger.folderToggled({ scopeId, folderId }),
  };
}

function collapsedIds(flags: Record<string, boolean>): string[] {
  return Object.entries(flags).flatMap(([id, collapsed]) => (collapsed ? [id] : []));
}
