/**
 * Drafts of agent editors that went away holding unsaved edits without the operator's Save or
 * Discard: a command from outside this client replaced the window, a project or profile switch
 * took the desk away (#1653). They live for the page, keyed by the edited resource, and the next
 * editor of that resource in this client starts from them. Leaving the page warns while any is
 * held, as a mounted dirty editor does.
 */
const drafts = new Map<string, unknown>();
let unloadWarningArmed = false;

function warnOnUnload(event: BeforeUnloadEvent): void {
  event.preventDefault();
}

function syncUnloadWarning(): void {
  if (typeof window === "undefined") return;
  const wanted = drafts.size > 0;
  if (wanted === unloadWarningArmed) return;
  unloadWarningArmed = wanted;
  if (wanted) window.addEventListener("beforeunload", warnOnUnload);
  else window.removeEventListener("beforeunload", warnOnUnload);
}

/** One key per editor kind and resource: two editors of one file hold different draft shapes. */
export function retainedDraftKey(editor: string, resourceKey: string): string {
  return JSON.stringify([editor, resourceKey]);
}

/** The draft retained for `key`, if any. Reading it changes nothing, so a store may seed from it. */
export function retainedEditorDraft<T>(key: string): T | undefined {
  return drafts.get(key) as T | undefined;
}

export function retainEditorDraft(key: string, draft: unknown): void {
  drafts.set(key, draft);
  syncUnloadWarning();
}

export function dropRetainedEditorDraft(key: string): void {
  if (!drafts.delete(key)) return;
  syncUnloadWarning();
}

/** Forgets every retained draft; the page keeps them until it goes, tests reset them per case. */
export function clearRetainedEditorDrafts(): void {
  drafts.clear();
  syncUnloadWarning();
}
