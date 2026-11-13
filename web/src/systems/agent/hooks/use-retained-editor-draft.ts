import { useCallback, useEffect, useRef } from "react";

import {
  isRetainedEditorDraft,
  retainEditorDraft,
  takeRetainedEditorDraft,
} from "../lib/retained-editor-drafts";

interface DraftStore<TContext> {
  getSnapshot(): { context: TContext };
  subscribe(observer: (snapshot: { context: TContext }) => void): { unsubscribe(): void };
}

/** How the hook reads an editor's context; keep it stable (a module constant). */
export interface RetainedDraftPolicy<TContext> {
  /** The context holds edits its baseline does not. */
  isDirty(context: TContext): boolean;
  /** A save or reread is running; its outcome decides what is left to keep. */
  isBusy(context: TContext): boolean;
  /** The context as an editor resumes it: a busy phase becomes idle. */
  idle(context: TContext): TContext;
}

interface DraftBox<TContext> {
  store: DraftStore<TContext> | null;
  /** The resource the editor shows, or showed last. */
  key: string;
  /** What this editor retained per resource it left, until something drops it. */
  retained: Map<string, TContext>;
}

/**
 * Keeps an editor's unsaved edits when it goes away without Save or Discard (#1653). Only the
 * store's first creation seeds from `retainedEditorDraft(key)`, as it is: the mounted editor takes
 * that draft, so it owns the edits while mounted. When the editor unmounts, or moves to another
 * resource, still holding edits, they are retained under `key` for the next editor of that
 * resource. A save still running then (it may wait on a question nobody can answer now) keeps
 * the edits at once and, when it settles, replaces them with what is left, so saved work is not
 * kept as unsaved. A store replaced under the same key (a reload, a clean editor following the
 * server) retains nothing.
 *
 * Returns `forget`, which drops what this editor retained for the resource it shows, or showed
 * last: a Discard answered for a window that closed after the editor went away still drops its
 * edits, and never another resource's.
 */
export function useRetainedEditorDraft<TContext>(
  key: string,
  store: DraftStore<TContext>,
  policy: RetainedDraftPolicy<TContext>
): { forget: () => void } {
  const boxRef = useRef<DraftBox<TContext>>({ store: null, key, retained: new Map() });
  useEffect(() => {
    boxRef.current.store = store;
    boxRef.current.key = key;
    boxRef.current.retained.delete(key);
    // Before any event reaches a new store, its context is the draft it was seeded from, if any.
    takeRetainedEditorDraft(key, store.getSnapshot().context);
  }, [key, store]);
  useEffect(() => {
    // One box for the editor's life: the cleanup reads the store committed last.
    const box = boxRef.current;
    return () => {
      const current = box.store;
      if (current === null) return;
      const retain = (context: TContext) => {
        if (!policy.isDirty(context)) return;
        const draft = policy.idle(context);
        retainEditorDraft(key, draft);
        box.retained.set(key, draft);
      };
      // Read the store, not the last render: a Discard answered just before unmount counts.
      const context = current.getSnapshot().context;
      retain(context);
      if (!policy.isBusy(context)) return;
      const subscription = current.subscribe(snapshot => {
        if (policy.isBusy(snapshot.context)) return;
        subscription.unsubscribe();
        // Only while the draft kept at unmount is still unclaimed: forgotten or taken, it is done.
        const kept = box.retained.get(key);
        if (kept === undefined || !isRetainedEditorDraft(key, kept)) return;
        takeRetainedEditorDraft(key, kept);
        box.retained.delete(key);
        retain(snapshot.context);
      });
    };
  }, [key, policy]);
  const forget = useCallback(() => {
    const box = boxRef.current;
    const kept = box.retained.get(box.key);
    if (kept === undefined) return;
    box.retained.delete(box.key);
    takeRetainedEditorDraft(box.key, kept);
  }, []);
  return { forget };
}
