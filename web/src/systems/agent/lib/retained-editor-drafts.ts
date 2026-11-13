/**
 * Drafts of agent editors that went away holding unsaved edits without the operator's Save or
 * Discard: a command from outside this client replaced the window, a project or profile switch
 * took the desk away (#1653). They live for the page, keyed by the edited resource; two editors
 * of one resource keep one draft each. The next editor of that resource in this client starts
 * from the newest one. Leaving the page warns while any is held, as a mounted dirty editor does.
 */
const drafts = new Map<string, unknown[]>();
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

/**
 * The newest draft retained for `key`, if any. Reading it changes nothing, so a store may seed
 * from it; the editor that does takes it with `takeRetainedEditorDraft` once mounted.
 */
export function retainedEditorDraft<T>(key: string): T | undefined {
  return drafts.get(key)?.at(-1) as T | undefined;
}

export function retainEditorDraft(key: string, draft: unknown): void {
  drafts.set(key, [...(drafts.get(key) ?? []), draft]);
  syncUnloadWarning();
}

/** Whether `draft` is still held under `key`, unclaimed by an editor. */
export function isRetainedEditorDraft(key: string, draft: unknown): boolean {
  return drafts.get(key)?.includes(draft) ?? false;
}

/** Removes `draft` from `key`'s drafts; another editor's draft of the resource stays. */
export function takeRetainedEditorDraft(key: string, draft: unknown): void {
  const held = drafts.get(key);
  if (!held?.includes(draft)) return;
  const rest = held.filter(candidate => candidate !== draft);
  if (rest.length === 0) drafts.delete(key);
  else drafts.set(key, rest);
  syncUnloadWarning();
}

/** Forgets every retained draft; the page keeps them until it goes, tests reset them per case. */
export function clearRetainedEditorDrafts(): void {
  drafts.clear();
  syncUnloadWarning();
}
