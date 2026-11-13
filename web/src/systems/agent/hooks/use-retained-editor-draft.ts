import { useCallback, useEffect, useRef } from "react";

import {
  isRetainedEditorDraft,
  markDuplicateDraftStore,
  retainEditorDraft,
  takeRetainedEditorDraft,
  wasRetainedEditorDraft,
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
  reseed: () => void;
  /** The resource the editor shows, or showed last. */
  key: string;
  /** What this editor retained per resource it left, until something drops it. */
  retained: Map<string, TContext>;
  /** The context a Discard was answered on; it is not kept even if a running save ignored it. */
  discarded: TContext | null;
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
 * Editors of one resource that mount in one commit all render before any claims a draft, so they
 * may seed from the same one. The first to commit keeps it; any other marks its store a
 * duplicate and calls `reseed`, which replaces the store: the replacement for a duplicate
 * resumes the next draft of the resource, or starts clean (`isDuplicateDraftStore`).
 *
 * Returns `forget`, which drops what this editor retained for the resource it shows, or showed
 * last: a Discard answered for a window that closed after the editor went away still drops its
 * edits, and never another resource's.
 */
export function useRetainedEditorDraft<TContext>(
  key: string,
  store: DraftStore<TContext>,
  policy: RetainedDraftPolicy<TContext>,
  reseed: () => void
): { forget: () => void } {
  const boxRef = useRef<DraftBox<TContext>>({
    store: null,
    reseed,
    key,
    retained: new Map(),
    discarded: null,
  });
  useEffect(() => {
    boxRef.current.reseed = reseed;
  });
  useEffect(() => {
    boxRef.current.store = store;
    boxRef.current.key = key;
    boxRef.current.retained.delete(key);
    // Before any event reaches a new store, its context is the draft it was seeded from, if any.
    const seed = store.getSnapshot().context;
    if (takeRetainedEditorDraft(key, seed) || !wasRetainedEditorDraft(seed)) return;
    // Another editor of the resource claimed this draft in the same commit: start over.
    markDuplicateDraftStore(store);
    boxRef.current.reseed();
  }, [key, store]);
  useEffect(() => {
    // One box for the editor's life: the cleanup reads the store committed last.
    const box = boxRef.current;
    return () => {
      const current = box.store;
      if (current === null) return;
      const retain = (context: TContext): TContext | undefined => {
        if (!policy.isDirty(context)) return undefined;
        const draft = policy.idle(context);
        retainEditorDraft(key, draft);
        box.retained.set(key, draft);
        return draft;
      };
      // Read the store, not the last render: a Discard answered just before unmount counts.
      const context = current.getSnapshot().context;
      // A Discard a running save ignored still holds: the edits it answered for are not kept.
      if (box.discarded === context) return;
      const kept = retain(context);
      if (kept === undefined || !policy.isBusy(context)) return;
      const subscription = current.subscribe(snapshot => {
        if (policy.isBusy(snapshot.context)) return;
        subscription.unsubscribe();
        // Only this cleanup's draft, while still unclaimed: forgotten, taken or replaced by newer
        // edits to the resource, it is done.
        if (box.retained.get(key) !== kept || !isRetainedEditorDraft(key, kept)) return;
        takeRetainedEditorDraft(key, kept);
        box.retained.delete(key);
        retain(snapshot.context);
      });
    };
  }, [key, policy]);
  const forget = useCallback(() => {
    const box = boxRef.current;
    box.discarded = box.store?.getSnapshot().context ?? null;
    const kept = box.retained.get(box.key);
    if (kept === undefined) return;
    box.retained.delete(box.key);
    takeRetainedEditorDraft(box.key, kept);
  }, []);
  return { forget };
}
