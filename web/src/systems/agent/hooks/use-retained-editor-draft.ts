import { useEffect } from "react";

import { retainEditorDraft, takeRetainedEditorDraft } from "../lib/retained-editor-drafts";

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
}

/**
 * Keeps an editor's unsaved edits when it goes away without Save or Discard (#1653). The store
 * must be seeded from `retainedEditorDraft(key)` as it is: the mounted editor takes that draft,
 * so it owns the edits while mounted. When the editor unmounts, or moves to another resource,
 * still holding edits, they are retained under `key` for the next editor of that resource; a
 * save running then settles first, so saved work is not kept as unsaved.
 */
export function useRetainedEditorDraft<TContext>(
  key: string,
  store: DraftStore<TContext>,
  policy: RetainedDraftPolicy<TContext>
): void {
  useEffect(() => {
    // Before any event reaches the new store, its context is the draft it was seeded from.
    takeRetainedEditorDraft(key, store.getSnapshot().context);
    return () => {
      const retain = (context: TContext) => {
        if (policy.isDirty(context)) retainEditorDraft(key, context);
      };
      // Read the store, not the last render: a Discard answered just before unmount counts.
      const context = store.getSnapshot().context;
      if (!policy.isBusy(context)) {
        retain(context);
        return;
      }
      const subscription = store.subscribe(snapshot => {
        if (policy.isBusy(snapshot.context)) return;
        subscription.unsubscribe();
        retain(snapshot.context);
      });
    };
  }, [key, store, policy]);
}
