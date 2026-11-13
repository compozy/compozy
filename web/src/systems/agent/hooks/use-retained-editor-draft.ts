import { useEffect } from "react";

import { dropRetainedEditorDraft, retainEditorDraft } from "../lib/retained-editor-drafts";

interface SnapshotStore<TContext> {
  getSnapshot(): { context: TContext };
}

/**
 * Keeps an editor's unsaved edits when it goes away without Save or Discard (#1653). The mounted
 * editor owns its draft, so the retained copy is dropped while it is mounted; when the editor
 * unmounts, or moves to another resource, still holding edits, they are retained under `key` for
 * the next editor of that resource. The store must be seeded from `retainedEditorDraft(key)`.
 * `isDirty` should be stable; a new one only re-runs the hand-over.
 */
export function useRetainedEditorDraft<TContext>(
  key: string,
  store: SnapshotStore<TContext>,
  isDirty: (context: TContext) => boolean
): void {
  useEffect(() => {
    dropRetainedEditorDraft(key);
    return () => {
      // Read the store, not the last render: a Discard answered just before unmount counts.
      const context = store.getSnapshot().context;
      if (isDirty(context)) retainEditorDraft(key, context);
    };
  }, [key, store, isDirty]);
}
