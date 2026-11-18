/**
 * Drafts of agent editors that went away holding unsaved edits without the operator's Save or
 * Discard: a command from outside this client replaced the window, a project or profile switch
 * took the desk away (#1653). They live for the page, keyed by the edited resource; two editors
 * of one resource keep one draft each. The next editor of that resource in this client starts
 * from the newest one. Leaving the page warns while any is held, as a mounted dirty editor does.
 */
const drafts = new Map<string, unknown[]>();
/** Every draft ever retained, so an editor can tell its seed was one another editor claimed. */
const everRetained = new WeakSet<object>();
/** Stores seeded from a draft another editor claimed first; they start over (see the hook). */
const duplicateStores = new WeakSet<object>();
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
  if (typeof draft === "object" && draft !== null) everRetained.add(draft);
  syncUnloadWarning();
}

/** Whether `draft` is still held under `key`, unclaimed by an editor. */
export function isRetainedEditorDraft(key: string, draft: unknown): boolean {
  return drafts.get(key)?.includes(draft) ?? false;
}

/**
 * Removes `draft` from `key`'s drafts and tells whether it was there; another editor's draft of
 * the resource stays.
 */
export function takeRetainedEditorDraft(key: string, draft: unknown): boolean {
  const held = drafts.get(key);
  if (!held?.includes(draft)) return false;
  const rest = held.filter(candidate => candidate !== draft);
  if (rest.length === 0) drafts.delete(key);
  else drafts.set(key, rest);
  syncUnloadWarning();
  return true;
}

/** Whether `value` was ever retained as a draft, held now or not. */
export function wasRetainedEditorDraft(value: unknown): boolean {
  return typeof value === "object" && value !== null && everRetained.has(value);
}

/** Marks `store` as seeded from a draft another editor claimed first. */
export function markDuplicateDraftStore(store: object): void {
  duplicateStores.add(store);
}

/**
 * Whether `store` was seeded from a draft another editor claimed first: its replacement resumes
 * the next draft of the resource, or starts clean, instead of showing the same edits twice.
 */
export function isDuplicateDraftStore(store: object): boolean {
  return duplicateStores.has(store);
}

/** Forgets every retained draft; the page keeps them until it goes, tests reset them per case. */
export function clearRetainedEditorDrafts(): void {
  drafts.clear();
  syncUnloadWarning();
}
